# Spec 02 — Ingesta (feeds → noticias)

## Alcance

`rss-ingestor-go` (servicio `ingestor`): lee `feeds` activos, descarga RSS
(`gofeed`), inserta en `noticias` con `id = md5(url)`, `ON CONFLICT DO NOTHING`.
`getActiveURLs` sin `LIMIT`; concurrencia `RSS_MAX_WORKERS=20` (compose),
revisita cada `RSS_POKE_INTERVAL_MIN=8`, timeout `RSS_FEED_TIMEOUT=60`.

## Env (compose, obligatorias)

`DB_HOST=db DB_PORT=5432 DB_NAME/DB_USER=${POSTGRES_*:-rss}
DB_PASS=${POSTGRES_PASSWORD}` (+ `RSS_MAX_WORKERS`, `RSS_POKE_INTERVAL_MIN`).

## Limpieza de textos (textclean)

Paquete `textclean` **duplicado a propósito** (`backend/internal/textclean` y
`rss-ingestor-go/textclean`; `make parity` verifica que no derivan): ingestor
y backend se construyen con contextos Docker separados (`./rss-ingestor-go`,
`./backend`), así que no pueden compartir módulo. Tests + fuzz en ambos
(`make test`).

Se aplica **al escribir**, nunca al leer: así la API/search/frontend no pagan
coste por request y `tsv`/`search_vector_es`, langdetect, NER y embeddings
consumen texto ya limpio.

- `rss-ingestor-go`: `cleanHTML()` → `textclean.Clean` (resumen) y
  `textclean.CleanTitle` (título). Antes era `goquery.Text()`: pegaba bloques
  (`...na GooglePridať...`), conservaba tabs/espacios (35,7% del texto) y
  dejaba los HTML escapados del feed como texto visible.
- `backend/cmd/scraper`: `extractArticle` y `extractContentFromURL`
  (enriquecimiento) → mismas funciones. De paso se arregla el corte de
  `summary[:500]` por bytes (partía runes UTF-8).

Qué quita: HTML (parseado con `x/net/html`, **nunca regex**), `script/style/
nav/header (salvo dentro de article)/footer/aside/form/aria-hidden`, entidades
HTML, código JS/CSS/JSON embebido, líneas de chrome (menús, cookies,
publicidad, URL/correo sueltos), líneas cortas repetidas ≥4 veces y espacios
sobrantes. Qué no toca: frases normales (las reglas están ancladas a la línea
completa) y entradas tipo `a < b` (RE2 ⇒ tiempo lineal, sin ReDoS). Acotado a
256 KB de entrada y 4000 car. de salida (corte en frontera de frase) e
idempotente (`Clean(Clean(x)) == Clean(x)`), comprobado por fuzz.

Dos casos borde que la validación sobre 4.000 muestras reales dejó al
descubierto, y que la primera versión manejaba mal (borraba texto real):

- **JSON/JSON-LD metido en el campo de contenido** (Yahoo Sports, The Age…):
  si al final no queda nada y el original empieza por `{`/`[`, se devuelve la
  `description` del *primer* valor JSON —aunque venga pegada a CSS, así que no
  sirve `json.Valid`— en vez de vaciar el resumen. Si ese JSON no tiene prosa
  (miga de pan) sí se vacía y el scraper lo re-enriquece con la página.
- **Artículo entero en una sola línea** (CNN Turk, ABC, Katadata…): las reglas
  de ruido son por línea, así que descartar la línea entera habría borrado la
  noticia (1 de cada 13 muestras). Por encima de 400 car. solo se tira si la
  línea **es** basura (cabecera de JSON/CSS/JS o sin espacios de prosa); si es
  prosa se conserva y únicamente se le recortan las llamadas con punto que
  vengan pegadas (`googletag.push(function(){…});`), sin tocar paréntesis de
  verdad (`v2.0 (beta)`).

Validación sobre 4.000 resúmenes reales: −60,9 % de texto (10,7 M → 4,2 M
car.), 50 % de filas modificadas, 0 no idempotentes, 0 etiquetas HTML
sobrevivientes, 125 → 3 filas con `<` (todas legítimas, p. ej. `<예산심의관>`)
y los 69 resúmenes que quedan vacíos (1,7 %) son CSS, menús, JSON sin prosa
(`BreadcrumbList`) o entradas que ya venían vacías.

Backfill de lo ya existente: `make sanitize-dry-run` / `make sanitize`
(`backend/cmd/sanitize`). Crea `titulo_raw`/`resumen_raw`/`cleaned_at` si
faltan, guarda el original **solo donde cambia** (reversible con
`make sanitize-restore`), y con `-requeue` re-encola las traducciones cuyo
texto cambió ≥50 car., borra sus embeddings y tags y pone `lang=NULL` para que
langdetect vuelva a detectar sobre el texto limpio. Idempotente y reanudable
(lotes con Ctrl+C seguro).

Orden de despliegue: primero la imagen nueva (ingestor y scraper con
`textclean`, más el binario `/sanitize`; las escrituras nuevas ya salen
limpias) y después el backfill. Opciones directas si no hay `make`:

```bash
docker compose run --rm --entrypoint /sanitize backend            # dry-run
docker compose run --rm --entrypoint /sanitize backend -apply -requeue
make sanitize-restore                                             # vuelta atrás
```

## Criterios de aceptación

- [x] `SELECT count(*) FROM noticias` crece de forma sostenida.
- [x] Feeds muertos (522/timeout/DNS) no tumban el worker (log + sigue).
- [x] Re-ejecuciones no duplican (`UNIQUE(url)` + md5 idempotente).
- [ ] Métrica de cobertura: % feeds activos con ≥1 noticia de 7 días
      (hoy sin exponer; propuesta en tasks).

## Plan

1. Mantener servicio `ingestor` con `depends_on db healthy` (hecho).
2. Añadir endpoint o log periódico de cobertura por feed (pendiente).
3. Si se quieren semillas nuevas: importar CSV por `/api/feeds/import`
   (requiere multipart con boundary; ver spec 06) o sembrar `fuentes_url`
   para `discovery` (spec 07).

## Tasks

- [x] Servicio `ingestor` en compose con credenciales reales.
- [x] Verificar crecimiento de `noticias` y ausencia de duplicados.
- [x] `GET /api/admin/ingest/stats` implementado (2026-09-14): feeds
      total/activos/con fallos, noticias total/7d, cobertura % 7d.
- [x] Poda 2026-09-14: eliminados 5320 feeds (`activo=false` + `fallos≥30`,
      respaldo CSV en `/tmp/opencode/feeds_muertos_backup.csv`; sin FKs
      hacia `feeds`, borrado seguro). Quedan 1502 (1161 activos).
- [x] Endurecido `ON CONFLICT` 2026-09-14: ingestor `(url)` y scraper
      `(id)` tumbaban el chunk ante duplicados cruzados
      (`duplicate key noticias_pkey` visto en logs). Ahora
      `ON CONFLICT DO NOTHING` sin árbitro en ambos: cualquier duplicado
      salta la fila sin perder el lote.
- [x] Diagnóstico de ruido/HTML 2026-09-30 (6000 traducidas muestreadas vía
      API + conteos globales): 35,7% de los caracteres de `resumen` eran
      espacios/tabuladores; 4733 filas contenían código JS (`function(`) y
      3176 etiquetas HTML (`<`, de las cuales 812 con `<img`); 8,2% de los
      caracteres eran líneas repetidas entre noticias de la misma fuente.
      Peores casos: Folha 38 398 car. de página completa, Index.hr 62 436
      car. con 312 líneas repetidas, `theregister` cuyo resumen ENTERO era
      `(function(){...})()`. Estimación inicial de esa pasada: −30% del texto
      total (9,95M → 7,0M en la muestra); con el `textclean` definitivo la
      medición final dio −60,9% (ver la sección de limpieza).
- [x] Paquete `textclean` + tests/fuzz (2026-09-30) cableado al ingestor
      (`cleanHTML`, títulos) y al scraper (`extractArticle`,
      `extractContentFromURL`), con `make parity` para que las dos copias no
      deriven.
- [x] Backfill `backend/cmd/sanitize` (2026-09-30): dry-run por defecto,
      `-apply`, `-requeue`, `-restore`, columnas `*_raw`/`cleaned_at`
      auto-creadas e idempotente.
- [x] Backfill ejecutado sobre la BD real (2026-09-30, `make sanitize`):
      147.957 filas procesadas, 60.748 cambiadas (41,1 %), −56 % de texto
      (293,3 M → 127,4 M car.), tamaño máx. 312.633 → 4.000, HTML 1.990 → 0,
      33.201 traducciones re-encoladas. Verificación posterior: las 318 filas
      llegadas después del backfill dieron **0 cambios** (el ingestor ya
      escribe limpio), 0 filas con `resumen`/`titulo` >4.000 car. y la API
      devuelve 0/100 resúmenes con etiquetas o código. Reversible con
      `make sanitize-restore`.
