# Spec 04 — Enriquecimiento: NER, wiki, alertas

## Alcance

1. `ner` (`workers/ner_worker.py`, imagen `Dockerfile.ner`: spacy +
   `es_core_news_lg` ~560 MB + bs4 + psycopg2): lee traducciones `done`
   (`lang_to='es'`) en lotes de 64, escribe `tags` + `tags_noticia`
   (UNIQUE tag/noticia). Sin Redis.
2. `wiki-worker` (binario en imagen backend, **sin servicio**): tags
   persona/organización → `es.wikipedia` summary + thumbnail en
   `/app/data/wiki_images`, marca `wiki_checked`. Servido en
   `/api/wiki-images/*`.
3. `AlertScanner` (goroutine del server, 45 s + cada 120 min,
    `ALERTS_*` env): picos **por hora** (`hits≥3`, `baseline≥0.5`,
    `ratio≥5`, `ALERTS_LOOKBACK_HOURS=24`, `≥2` buckets base)
    → `alertas(status='nueva')`. Referencia = última hora COMPLETA con
    datos (`n.fecha < date_trunc('hour', CURRENT_TIMESTAMP)`): la hora en
    curso está incompleta y comparar contra ella nunca dispara.
    `alertas.periodo` es **TIMESTAMP** (la migración DATE→TIMESTAMP la
    hace el server al arrancar, idempotente, porque `alertas` no vive en
    `init-db/` sino en el DDL de `cmd/server/main.go`).
    Verificado 2026-09-26: 29 alertas de la hora `00:00` (ref, 9 buckets
    activos, 8 de baseline).

## Env (compose)

`ner`: DB_* reales, `NER_LANG=es`, `NER_BATCH=64`.

## Criterios de aceptación

- [x] `tags`/`tags_noticia` crecen (31k/55k).
- [x] Populares (`/api/entities`) con counts; noticias por entidad
      (`valor`+`tipo`); Evolución (`/api/entities/mentions`) con series.
- [x] Scanner corre y genera alertas (fix día completo, 1044 el
      2026-09-14). Umbrales reescalados a hora el 2026-09-26.
- [x] Falsos positivos en Populares = 0 en el top-50 de España y global
      (`go test ./internal/entitycheck`, dentro de `make test`); 7.248
      valores fuera de la API tras `make entities-scan-apply`.
- [x] Fix alertas 2026-09-26 — granularidad horaria + baseline activo:
      * **Síntoma**: `GET /api/alerts` → `alertas:[]` y el log decía
        `scan finished, 0 new alerts` aunque el scan corría cada 120 min.
      * **Causa raíz medida** (no deducida): con la serie diaria,
        `dias_con_datos=1` frente a 8 huecos → `AVG(COALESCE(cnt,0))`
        daba `baseline=0` en las 67 430 entidades y `baseline≥2` las
        descartaba todas (`pasan_min_baseline=0`).
      * **Agravante**: `COALESCE(cnt,0)` sobre la serie de calendario
        confunde «ese tramo no había datos etiquetados» con «0
        menciones»; con historia parcial rebañaba el baseline y
        falseaba el ratio hacia arriba. Ahora el baseline solo promedia
        buckets con datos.
      * **Granularidad**: por hora había 9 buckets activos (8 de
        baseline) donde por día solo había 1, así que la detección pasa
        a `date_trunc('hour', n.fecha)` y la ventana a `ALERTS_LOOKBACK_HOURS`.
      * **Ajuste por volumen**: la ingesta es irregular y el NER va por
        detrás, así que las horas tienen distinto nº de noticias. Sin
        ajustar, el ratio máximo medido era **1.8** y ninguna alerta
        alcanzaba umbral; con `baseline = menciones/noticia de la base
        × noticias de la hora ref`, `ratio = hits/baseline` sigue
        coherente con la UI y se degrada a la comparación absoluta de
        siempre cuando las horas igualan volumen.
      * **Umbrales horarios**: `hits≥3`, `baseline≥0.5`, `ratio≥5`,
        `buckets≥2` (los diarios `hits≥5/baseline≥2` exigirían `hits≥10`
        por hora, insalvable con ~200 noticias/hora).
      * **Verificado por ejecución**: `alerts: ref=2026-09-26 00:00:00
        buckets_activos=9 baseline=8 candidatos=29 nuevas=29`;
        `periodo` migrado a `timestamp`; `GET /api/alerts` devuelve
        `periodo:"2026-09-26 00:00"` y el tipo `Alerta` del frontend
        encaja sin cambios.
- [x] `wiki` activo como servicio (imagen `rss2-backend`,
  `entrypoint: /wiki-worker`, `DB_*`, volumen `wiki_images`):
  `tags.wiki_summary/url` + thumbs servidos en `/api/wiki-images/*`.
  Verificado: `Putin` con summary real.
- [x] Fix wiki 2026-09-14: ante error de la API (403/red) el tag NO se
  marcaba `wiki_checked` y se reintentaba cada ciclo eternamente (storm
  sobre tags basura del NER). Ahora se marca revisado también en error.
- [x] Filtro anti-basura NER 2026-09-14 (`clean_tag_text`): rechaza
  `[]{}"<>\/` + backtick, >80 chars, días de semana sueltos y
  `este/el + día` (`GENERIC_BAD_TAGS`). Purgados 1746+46 tags basura
  (respaldo en `/tmp/opencode/tags_basura_backup.csv`; cascada limpia
  `tags_noticia`). Verificado: 0 tags basura tras 2 min procesando.
  Alertas históricas con day-words se conservan (auditoría).
- [x] Filtro NER vía topics 2026-09-14: la basura reapareció (98 tags)
  porque `clean_topic_text` (noun-chunks) no aplicaba la regla — solo
  `clean_tag_text` (entidades). Regla extraída a helper compartido
  `_is_markup_garbage` usado en ambas. Purgados los 98; 0 rebrotes.

## Falsos positivos: plan global (2026-09-30)

Diagnóstico sobre **todos** los países y tipos: 304.718 valores distintos en
`tags` (202.208 tema, 40.349 persona, 37.101 lugar, 24.090 organización).
Aplicando las reglas de `entitycheck`: **7.248 FP fuertes (2,4 %)** y
12.857 débiles (4,2 %). La basura es siempre de la misma familia: horas y
fechas pegadas (`BILD28.09.2026`), marcadores (`Italia 0-2 Bélgica`),
fragmentos de titular cortados (`Consejo de Administración de la`), urls,
palabras sueltas (`Fuente`, `Más`, `Leer`) y texto pegado.

La limpieza es **global por diseño**: no filtra por país ni por tipo, así que
cualquier fuente de cualquier idioma entra por el mismo sitio.

### Las cuatro capas

1. **Reglas únicas** — `backend/internal/entitycheck` (Go, RE2):
   `Classify(tipo, valor)` → `fuerte` (basura mecánica de alta precisión, se
   bloquea) o `débil` (sospechoso, solo se informa; la revisión es humana).
   Ningún consumidor reimplementa reglas.
2. **Blocklist en BD** — `entity_blocklist(tipo, valor, motivo, menciones)`
   la llena `backend/cmd/entityscan` (dry-run por defecto, `-apply` para
   escribir, `-unblock` para deshacer un valor). La leen:
   * `handlers.GetEntities` (lista **y** `total`, así la paginación cuadra)
     y `handlers.GetEntityNews` → Populares deja de mostrarlo a cualquiera;
   * `workers/ner_worker.py` → el valor ya no se vuelve a insertar aunque
     el texto pase otra vez por el NER (se recarga cada 10 min, con
     tolerancia a que la tabla no exista).
3. **Detección continua** — `TestPopularesEspana` y `TestPopularesGlobal`
   (mismo paquete) consultan la API real: lo más popular de España
   (persona/organización) y los 4 tipos a nivel global. Si un FP fuerte
   vuelve a ser visible, el test falla y dice qué ejecutar. Se saltan solos
   si la API no responde, así `make test` sirve sin levantar el stack
   (`API_URL` para apuntar a otra instancia).
4. **Prevención en origen** — el `textclean` ya desplegado (spec 02) limpia
   el texto del que el NER extrae entidades: sin `script`/CSS/chrome hay
   muchos menos FP nuevos que antes.

### Mantenimiento (cuando falle el test de la capa 3)

```bash
make entities-scan           # informe: qué FP hay ahora mismo
make entities-scan-apply     # los añade a entity_blocklist y todo vuelve a verde
# un valor legítimo bloqueado por error:
docker compose run --rm --entrypoint /entityscan backend -unblock "X" -tipo persona
```

`init-db/46-entity-blocklist.sql` (instalaciones nuevas) y
`migrations/add_entity_blocklist.sql` (las existentes) crean la tabla; el
binario también la crea si falta.

### Fases

- [x] F1 reglas + tests unitarios (casos reales legítimos y basura en ambos
      sentidos: `Pará` no se bloquea, `Según la` sí).
- [x] F2 blocklist + filtro en API + semilla global (7.248 valores;
      totals de `/entities` bajan en esa misma cantidad).
- [x] F3 `ner_worker` no reinserta valores de la blocklist.
- [x] F4 tests de API dentro de `make test` (España + global).
- [ ] F5 **fragmentación** (no es FP: es agrupación) — `Mourinho`/`José
      Mourinho`, `Real`/`Real Madrid`/`Los Blancos` conviven porque no hay
      alias. Vía `entity_aliases` (ya con export/import en Populares);
      candidato: proponer alias automáticamente desde `wiki_url` idéntico.
- [ ] F6 FP **débiles** (12.857: nombres pegados, frases de 7-8 palabras):
      informe humano con `/entityscan -v -tipo X`; nunca se bloquean solos.
- [ ] F7 `wiki-worker` que ignore la blocklist (hoy sigue llamando a
      Wikipedia por valores que ya no se muestran).

## Alertas: revisión del algoritmo (2026-09-30)

Todo lo de esta sección está **medido** contra la serie real (export
`/tmp/opencode/hourly.csv`, 604.115 pares tag-hora, 118 horas con datos
del 25-09 07:00 al 30-09 04:00, y `vol.csv` con las noticias por hora),
no deducido. El backtest en Python reproduce la SQL de producción:
para la hora fijada `2026-09-30 04:00` la consulta original da
`2539` candidatos con baseline y `0` alertas, y el backtest `2531/0`
(misma snapshot, mismos umbrales), así que las comparaciones de abajo
son fiables.

### Diagnóstico

* **Ruido**: 843 alertas en 5 días (26-09→30-09) ≈ **169/día** y
  **todas con `status='nueva'`** (0 leídas): nadie las consume.
* **Reparto por tipo**: 548 `tema` (65 %), 151 `lugar`, 85
  `organizacion`, 59 `persona`.
* **Baseline diminuto**: el **74 %** tiene `baseline < 1` (mediana 0,8),
  es decir la entidad esperaba menos de una mención por hora; con
  `hits≥3` el **19 %** de las alertas no es significativo
  (cola de Poisson `P(X≥hits | λ=baseline) ≥ 0,01`).
* **Calidad de los valores**: temas genéricos (`clic`, `debate`,
  `campaña`, `encuesta`, `diario`, `reglas`), artefactos de traducción
  concatenados (`OficiaisTestimonización`, `De trabajoPorte`,
  `El ensayarAcaracóferos de gasLámpadas`) y CTA de página web
  (`Haga clic`, `Reklamwindow._taboola`, `helpertext.html Publicidad
  Anunciar`); 26/843 (3 %) llevan marcas mecánicas evidentes y solo
  **4/843 están en `entity_blocklist`**, que además `GET /api/alerts`
  no filtra.
* **Volumen inestable por scan**: 11-13 horas ref cubiertas al día
  (scan cada 120 min) con entre **2 y 60 alertas por scan** (media 18).
  La serie solo existe desde el 25-09 (base reconstruida): para la hora
  `2026-09-30 04:00` el scan insertó 7 alertas y con **los mismos
  parámetros** sobre el snapshot posterior la misma consulta da **0**,
  porque los baselines siguen rellenándose. Es decir, durante los
  primeros días tras un rebuild el detector dispara con baselines
  artificialmente bajos.
* **Ranking sesgado**: la UI ordena por `ratio`, que con `baseline`
  minúsculo se dispara (`×30` sobre `baseline=0,3`) y favorece
  entidades de poco volumen frente a las relevantes.

### Cuellos de botella del algoritmo actual

1. `baseline≥0.5` deja pasar picos sobre entidades que casi nunca
   aparecen (mediana 0,8) — el 74 % del ruido.
2. `hits≥3` con una base de 135-520 noticias/hora es demasiado barato.
3. No hay filtro anti-FP: ni `entity_blocklist` ni `entitycheck` se
   consultan al insertar ni al servir `/api/alerts`.
4. Sin cooldown: la única amortiguación es implícita (la hora del pico
   entra en su propio baseline y el ratio se autocontiene la hora
   siguiente), medida pero insuficiente como garantía.
5. `GetAlertas` devuelve `total = len(alertas)` (no es el total real),
   no acepta filtro por `tipo` y no existe estado de falsa alerta.

### Backtest de umbrales (118 h, cooldown 6 h, misma snapshot)

| conjunto | reglas (hits/bl/ratio/buckets) | alertas | /día | % p≥0,01 | % tema | bl mediana |
|---|---|---|---|---|---|---|
| **actual** | `3/0,5/5/nb2` (todos) | 540 | ~135 (observado 169) | **19 %** | 64 % | 0,8 |
| C uniforme medio | `5/1/5/nb2` (todos) | 197 | 49 | 0 % | 60 % | 1,4 |
| **A propuesto** | no-tema `5/1/5/nb2` · tema `6/2/6/nb3` | 86 | **21,5** | 0 % | 9 % | 1,5 |
| D sin temas | `5/1/5/nb2` solo persona/lugar/org | 78 | 19,5 | 0 % | 0 % | 1,4 |
| B uniforme estricto | `5/2/5/nb3` (todos) | 29 | 7,2 | 0 % | 45 % | 2,6 |

Notas del backtest:

* Con `baseline≥1`, `ratio≥5` y `hits≥5` el **peor caso** es
  `P(X≥5 | λ=1) = 0,0037 < 0,01`: la significación estadística queda
  garantizada por construcción (el 19 % actual desaparece), y es lo que
  mide la columna `% p≥0,01`.
* Un criterio puramente estadístico no basta: aplicar solo
  Benjamini-Hochberg al 5 % sobre los candidatos da **6,5 alertas/hora**
  (más que hoy). El freno tiene que ser práctico: `baseline≥1` = entidad
  realmente recurrente (≈24 menciones/día esperadas).
* Las 86 alertas de la propuesta A son entidades legítimas en su
  práctica totalidad (`Serbia`, `Manchester City`, `Luiz Inácio Lula da
  Silva`, `Kylian Mbappé`, `España`, `Corea del Sur`, `Seúl`), con solo
  8 temas (1,6/día) y la basura restante (`Haga clic`) eliminable por
  blocklist.
* El cooldown apenas recorta (86→85 con 24 h) porque el baseline se
  autocorrige; se mantiene de todos modos como garantía.

### Valores propuestos (los «correctos»)

| parámetro | hoy | propuesto | por qué |
|---|---|---|---|
| `ALERTS_MIN_HITS` | 3 | **5** | con base de 200-500 noticias/hora, 3 menciones no es un pico |
| `ALERTS_MIN_BASELINE` | 0,5 | **1** | exige que la entidad sea recurrente; elimina el 74 % del ruido y el 19 % no significativo |
| `ALERTS_MIN_RATIO` | 5 | **5** | ya es correcto; con bl≥1 implica `p < 0,01` |
| `ALERTS_MIN_BASELINE_BUCKETS` | 2 | **2** (tema: 3) | evita derivar el ratio de un solo bucket |
| regla para `tema` | idéntica a resto | **`6/2/6/nb3`** | los temas son palabras genéricas: exigen doble exigencia (solo 8 alertas en 5 días) |
| cooldown | — | **6 h** por `(valor, tipo)` | garantiza que una historia no se repite en horas consecutivas |
| filtro anti-FP | — | **`entity_blocklist` + `entitycheck.Classify`** al insertar y al servir | misma capa 2/1 del plan de FP global |
| scan | 120 min | **60 min** (`ALERTS_SCAN_INTERVAL_MIN`) | hoy solo se cubren ~12 de cada 24 horas ref |
| estado | `nueva`/`leida` | **+ `descartada`** | feedback del usuario = futuro filtrado/semilla |

Con `5/1/5/nb2` uniforme (solo env, sin tocar código) se pasaría de
~169/día a ~49/día de inmediato; el filtro por tipo, el anti-FP y el
cooldown requieren código.

### Fases

- [ ] A1 **Umbrales + anti-FP en `RunAlertScan`**: defaults nuevos
      (`minHits=5`, `minBaseline=1`, `minRatio=5`, `minBuckets=2`),
      regla más exigente para `tipo='tema'` (`6/2/6/3`), `NOT EXISTS
      entity_blocklist` + `entitycheck.Classify(...) != fuerte` antes de
      insertar y cooldown de 6 h (`NOT EXISTS alertas` con `periodo ≥
      ref−6h`). El log pasa a informar `candidatos/omitidas_fp/omitidas_cooldown/nuevas`.
- [ ] A2 **Endpoint**: `GetAlertas` con el mismo filtro anti-FP, `total`
      real (`COUNT(*)` con los filtros), query param `tipo` y `nuevas`
      acotado; `POST /api/alerts/:id/dismiss` → `status='descartada'`
      (y filtro `?status=` ya soportado).
- [ ] A3 **Frontend `Alertas.tsx`**: botón «Descartar» (junto a
      «Marcar leída»), filtro por tipo, y ranking por significación en
      lugar de `ratio` crudo (mostrar `baseline` junto a `hits` para que
      el ×N sea interpretable).
- [ ] A4 **Podado + semilla**: recalcular las 843 existentes con los
      nuevos umbrales (`status='descartada'` las que no pasen) y añadir a
      `entitycheck` la regla fuerte de CTA/plantilla web (`haga clic`,
      `haz clic`, `leer más`, `suscríbete`, `política de privacidad`,
      `términos de uso`, `publicidad`) + `make entities-scan-apply`.
- [ ] A5 **Vigilancia en `make test`** (mismo patrón que
      `TestPopulares*`): últimas 24 h de alertas ≤ 40, 0 % con
      `p ≥ 0,01`, 0 valores en `entity_blocklist` y ninguna con
      `hits < 5`; se salta si la API no responde.
- [ ] A6 **Estabilidad**: `ALERTS_SCAN_INTERVAL_MIN=60` en compose y
      aviso en log si la hora ref lleva >3 h de retraso (NER parado →
      alertas sobre datos viejos).

## Plan / Tasks

- [x] `Dockerfile.ner` + servicio `ner` con DB real.
- [x] Fix orden de columnas en `GetEntities` (`apellido`→ Count).
- [x] Servicio `wiki` (ver arriba) — tooltips con imagen en Populares.
- [x] `alertas` > 0 verificado (1044 del 2026-09-13 tras el fix).
- [x] Alertas con foto+resumen (2026-09-14): `GET /api/alerts` hace
  `LEFT JOIN tags` (`wiki_summary/url/image_path`, nulls si el tag no
  tiene wiki); `Alertas.tsx` muestra thumbnail + snippet + `WikiTooltip`
  (mismo componente que Populares). Imagen servida 200 en
  `/api/wiki-images/*`.
