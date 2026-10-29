# RSS2 — Registro de Decisiones

> **Separación estricta**: la Parte A contiene decisiones **verificadas en el código** (tomadas y
> aplicadas). La Parte B contiene **recomendaciones nuevas** (NO son decisiones: nadie las ha
> tomado). La Parte C documenta las **preguntas abiertas**.
>
> Formato de la Parte A: *Contexto → Decisión → Consecuencias*. Verificado el 2026-10-26.

---

# Parte A — Decisiones verificadas

## A-01 · El id de una noticia es el md5 de su URL normalizada

**Contexto.** El mismo artículo se sirve como `https://www.x.com/a`, `http://x.com/a` y
`https://www.x.com/a?utm_source=rss`. Con un id aleatorio o con la URL cruda, cada variante
producía una noticia y una traducción distintas (22.000 duplicados en 4.700 grupos tras el reset).

**Decisión.** `generateID(url) = md5(normalizeURL(url))`, con `normalizeURL` quitando esquema,
`www.`, puerto implícito, barra final, parámetros de tracking y ordenando la query.
`rss-ingestor-go/main.go:240-279`.

**Consecuencias.**
- `noticias.id` es `VARCHAR(32)` (16 bytes de md5 en hex) → **no es monotónico ni ordenable**.
  Esto **descarta el particionado de `noticias` por fecha** (ver `DATABASE.md` §6.3).
- La deduplicación es por URL normalizada, **no por título** (agrupar por título daba 24.275 filas
  frente a 1.098 duplicados reales).
- El mismo artículo con URLs distintas en dos fuentes **sigue** siendo dos noticias.

---

## A-02 · PostgreSQL es la única fuente de verdad

**Contexto.** Varios workers necesitan coordinarse (traductores locales, workers remotos, TTS).
Un broker externo añadiría una dependencia y un modo de fallo más.

**Decisión.** Todos los estados de trabajo viven en tablas Postgres. Redis es **caché** y
**canal de despacho**, nunca fuente de verdad. El patrón es `FOR UPDATE SKIP LOCKED`.

**Consecuencias.**
- Redis es **fail-open**: si cae, los endpoints siguen sirviendo desde la BD y los workers
  recuperan el trabajo consultando Postgres. `backend/internal/cache/redis.go`, `workers/tts/queue.py:27-48`.
- El `LPUSH` a `tts:queue` es *fire-and-forget*: su error se descarta y el trabajo ya está
  confirmado en `tts_jobs`. El comentario del código lo dice explícitamente.
- `alertas` es **estado agregado** (entidad + hora), no derivado de `noticias` por FK → tras un
  reset del corpus hay que truncarla explícitamente.

---

## A-03 · El filtro de "traducido" debe usar `LEFT JOIN`

**Contexto.** "Traducido" significa "el texto ya está disponible en español", lo que ocurre si
(a) hay una traducción `done` con `titulo_trad` distinta del original, **o** (b) la noticia ya
nació en español (canales marcados `feeds.traducido`).

**Decisión.** La condición es
`(t.status='done' AND t.titulo_trad IS NOT NULL AND t.titulo_trad != n.titulo) OR n.lang = <idioma destino>`
y el `JOIN` con `traducciones` es **siempre `LEFT`**. `handlers/search.go:301-331`,
`handlers/news.go:339-388`.

**Consecuencias.**
- Con `INNER JOIN` la rama `n.lang` es **inalcanzable** → desaparecerían justo las noticias que ya
  están en español y son narrables (los canales `traducido` nunca generan fila en `traducciones`).
- La misma condición debe usarse en `/api/search` y `/api/news` — si divergen, el Noticiario
  (que usa `/api/search`) y la lista de noticias offering divergen.

---

## A-04 · `feeds.traducido` es una lista curada, no `feeds.idioma`

**Contexto.** `feeds.idioma` es el idioma que declara el canal y a menudo es incorrecto.
`feeds.traducido` (migración 59) es una lista curada a mano.

**Decisión.** El ingestor inserta las noticias de los feeds marcados `traducido` con
`lang = TARGET_LANGS[0]` directamente, de modo que `langdetect` las ignora y el scheduler nunca las
traduce. `rss-ingestor-go/main.go:447-451`.

**Consecuencias.**
- Endpoints de gestión: `POST /api/feeds/:id/traducido` y `POST /api/feeds/traducido/bulk`.
- Sólo el **primer** idioma de `TARGET_LANGS` se usa → con `TARGET_LANGS=es,en` un feed "traducido"
  se marcaría sólo como `es`.
- Hoy hay **5 feeds** marcados. `idiomas_disponibles` tiene sólo `es` activo.

---

## A-05 · El baseline de las alertas se pondera por volumen

**Contexto.** El estimador original era `AVG(cnt/noticias)` por bucket horario. Un bucket con 9
noticias y 1 mención (ratio 11 %) dominaba la media e inflaba el baseline de **todas** las
entidades presentes en esa hora → el detector **no encontraba ninguna alerta nunca**.

**Decisión.** Baseline **agrupado (pooled)**: `SUM(menciones) / SUM(noticias de esas horas) ×
noticias de la hora de referencia`. `handlers/alerts.go:316-340`.

**Consecuencias.**
- Es el estimador correcto de "menciones esperadas en la hora de referencia".
- El scanner necesita ≥2 buckets previos por entidad (`MinBuckets`) → no produce nada hasta que hay
  unas horas de noticias etiquetadas.
- El bucket es `date_trunc('hour', n.fecha)` → `GET /api/entities/news?periodo=…` filtra a
  `[periodo, periodo+1h)`, que es exactamente la granularidad del detector.

---

## A-06 · `textclean` está duplicado a propósito, con verificación de paridad

**Contexto.** `backend/internal/textclean` y `rss-ingestor-go/textclean` se construyen en contextos
Docker distintos → divergirían sin control. El código que limpia el texto **tiene** que ser
idéntico en ambos, porque de él depende el id implícito y la calidad de la traducción.

**Decisión.** Duplicación deliberada + `make parity` (`diff -r`) como verificación. Sin CI.

**Consecuencias.**
- **Verificado hoy**: `diff -r` → exit 0, `md5sum` idéntico.
- ⚠️ **No hay ningún hook ni CI que lo ejecute** → la divergencia sólo se detecta si alguien se
  acuerda. Es el punto más frágil de la cadena.

---

## A-07 · El servicio TTS es interno y separado del backend Go

**Contexto.** Los motores de TTS son pesados (Chatterbox ≈ 2–3 GB de RAM) y su API es estable;
el backend Go debe ser ligero y rápido de compilar/desplegar.

**Decisión.** Servicio Python (Flask) en el puerto 7000, **sin puertos publicados**, sólo en la red
`backend`, con autenticación por `X-API-Key`. El audio se sirve al usuario **a través** del backend
Go (que exige JWT). `docker-compose.yml:518-588`, `backend/internal/services/tts.go`.

**Consecuencias.**
- El backend Go **no hace inferencia**: normaliza el texto delegando en el servicio
  (`POST /api/scripts/normalize`) para que exista **una sola implementación**.
- El audio **nunca** se guarda en Postgres: sólo la ruta relativa, duración, frecuencia y sha256.
- El path traversal está bloqueado por `resolveTTSAudioPath` (con test, incluido el caso symlink).

---

## A-08 · El audio se ensambla en el servidor, nunca en el cliente

**Contexto.** El boletín puede tener hasta 50 items, cada uno con su propia frecuencia de muestreo.

**Decisión.** `concatenate_wavs` (`workers/tts/bulletin.py`) procesa **en bloques** de 44.100 frames
(sin cargar el boletín entero en memoria), remuestrea los items con frecuencia distinta usando la
del **primer** item, e inserta `silence_ms` (800 ms por defecto) entre items pero no después del
último. El manifiesto (`position`, `start_offset`, `duration_seconds`) se escribe en
`tts_bulletin_items` para poder mapear el audio sobre las noticias.

**Consecuencias.**
- El cliente recibe **un** WAV reproducible con `sha256` y duración exacta.
- La reproducción en el navegador usa `<audio controls>` nativo; la descarga usa `blob` + `<a download>`.

---

## A-09 · El motor TTS se resuelve por prioridad: petición → voz → default global

**Contexto.** Una voz puede exigir un motor concreto (p. ej. un embedding `.pth` de OpenVoice no
funciona con Chatterbox).

**Decisión.** `resolveTTSEngine` (`handlers/tts.go:2132-2149`): si la petición indica motor, se usa;
si no, el de la voz (`tts_voices.engine`); si no, `config.TTSEngine` (global, `chatterbox`).

**Consecuencias.**
- Chatterbox **rechaza explícitamente** los embeddings `.pth` de OpenVoice (`voice_embedding_unsupported`)
  en vez de narrar con la voz equivocada.
- La voz `es-base` se sincroniza con `TTS_ENGINE` al arrancar (`_seed_builtin_voices`).
- `tts_audios.engine` y `engine_version` persisten el motor usado → **no hay que tocar el esquema**
  para añadir motores.

---

## A-10 · La idempotencia se garantiza con `ON CONFLICT`, no con lógica de aplicación

**Contexto.** Todos los workers son re-ejecutables (reinicios, reintentos, re-ejecución manual).

**Decisión.**
- Ingestor: id direccionable por contenido + `ON CONFLICT DO NOTHING`.
- Scheduler: `NOT EXISTS` + `ON CONFLICT (noticia_id, lang_to) DO NOTHING`.
- NER: anti-join con `tags_noticia` + `ON CONFLICT DO NOTHING`.
- Embeddings: `ON CONFLICT (traduccion_id, model) DO UPDATE` y `ON CONFLICT (noticia_id, model)`.
- TTS: `ON CONFLICT (idempotency_key) DO NOTHING`.

**Consecuencias.** Todos los workers son **idempotentes por construcción**: relanzarlos no duplica
trabajo. El `idempotency_key` del TTS la recalcula el servidor (ver ISS-007).

---

## A-11 · Se evita `pq.CopyIn` en el ingestor

**Contexto.** `CopyIn` es más rápido para lotes grandes.

**Decisión.** Se usan lotes de 500 filas con `INSERT … ON CONFLICT DO NOTHING` en vez de `COPY`.
`rss-ingestor-go/main.go:469-473` (el comentario explica el porqué).

**Consecuencias.** Un `CopyIn` preparado deja el backend en modo COPY con un lock
`ACCESS EXCLUSIVE`; si el flujo se interrumpe, bloquea toda la base. El coste en rendimiento es
aceptable para el volumen actual.

---

## A-12 · La ingesta prioriza lo reciente; el corpus no se puede drenar por completo

**Contexto.** La ingesta genera ~331/min de media (477.659/día medidos) y los 2 traductores
drenan ~31/min.

**Decisión.** El traductor ordena `ORDER BY n.fecha DESC`: **lo más reciente por fecha de
publicación primero**, el backlog antiguo no se prioriza. **Reconfirmado por el propietario el
2026-10-26** (ver A-26).

**Consecuencias.** Es la decisión correcta para el valor (un noticiario necesita noticias de hoy),
pero significa que **el backlog nunca se vacía** y que `translated_only` sólo devuelve material
reciente. Es la raíz de ISS-001. La consecuencia se acepta conscientemente: para que además
"se traduzca todo", hace falta **capacidad** (W7.2/W7.3), no un cambio de orden (A-31).

---

## A-13 · El scheduler usa dos rutas temporales, no un anti-join global

**Contexto.** El anti-join original escaneaba todo el catálogo: ~12 min por ciclo con 2,7 M de filas.

**Decisión.** Dos rutas acotadas por ventana:
- **fast** sobre `noticias.created_at` (índice `idx_noticias_created_at`, ~2 ms);
- **catch-up** sobre `noticias.updated_at` (índice `idx_noticias_updated_at`), porque el trigger
  `update_noticias_updated_at` refresca `updated_at` en cada UPDATE → así se capturan las noticias
  que `langdetect` etiquetó minutos después de insertarse.

**Consecuencias.** El ciclo pasó de ~12 min a ~30 s. La ruta catch-up corre cada 6 h
(`SCHEDULER_CATCHUP_INTERVAL_MIN=360`).

---

## A-14 · Se protege el borrado masivo de feeds con una precondición de servidor

**Contexto.** Un `confirm()` en el navegador no protege a quien llama a la API directamente, y el
borrado en cascada destruye miles de canales con sus noticias.

**Decisión.** `POST /api/feeds/bulk-delete` responde **428 Precondition Required** con la
previsualización (`feeds_affected`, `noticias_affected`) salvo que el cuerpo incluya
`confirmar: true`. `handlers/feed.go:334-398`.

**Consecuencias.** Es la única protección real. El frontend **siempre** envía `confirmar: true`
(`api.ts:407`) → la protección sólo existe para el cliente API.

---

## A-15 · La API cuenta con el mismo filtro que devuelve, en todas las páginas

**Contexto.** `total` se calculaba con `reltuples` de `pg_class` (aproximación global) y sólo en la
primera página → la paginación se rompía con filtros y desde la página 2 salía `total=0`.

**Decisión.** Si hay cualquier filtro → `COUNT(*)` con el **mismo WHERE** y los **mismos argumentos**;
el conteo se ejecuta en **todas** las páginas. Sin filtros se acepta `reltuples` como aproximación.
`handlers/news.go:345-374`, `handlers/search.go:394`.

**Consecuencias.** Correcto pero caro en tablas grandes sin filtro (no es el caso).

---

## A-16 · Los parámetros posicionales en SQL siguen una regla fija

**Contexto.** La búsqueda semántica calculaba el placeholder del idioma destino como
`len(extraArgs)+1`, pero el idioma es **siempre el primer argumento** (`$1`). Con cualquier filtro,
el placeholder colisionaba y la consulta fallaba con *"operator does not exist: integer >=
character"*. El Noticiario siempre envía `min_chars` y `desde` → **la búsqueda semántica estaba
completamente rota**.

**Decisión.** Regla: el idioma destino es `$1`; los filtros van después; `LIMIT`/`OFFSET` son los
dos últimos. `backend/internal/services/ml.go:252-293`.

**Consecuencias.** La regla está documentada en el propio código como invariante. **No hay test
automático que la verifique** → conviene añadir uno.

---

## A-17 · `nginx` resuelve el upstream dinámicamente

**Contexto.** Un `upstream backend:8080` estático se resuelve **una vez al arrancar**. Si el
contenedor del backend se recrea (o tras un reinicio del host) cambia su IP → nginx sigue
apuntando a la antigua → **404 desde un servicio ajeno**.

**Decisión.** `resolver 127.0.0.11 valid=10s ipv6=off` + `proxy_pass http://$api_upstream$request_uri`
(con variables). Con variables, la URI debe pasarse explícitamente como `$request_uri`, o nginx
envía sólo el prefijo y todo da 404. `nginx.conf:36-44, 75-91`.

**Consecuencias.** Es obligatorio reiniciar nginx después de recrear el contenedor `backend`
(es decir: `docker compose restart nginx`).

> El `frontend/nginx.conf:37` usa un `proxy_pass` **estático** — mismo anti-patrón, pero es código
> muerto en esta topología (nada alcanza `rss2_frontend:80` directamente).

---

## A-18 · El grupo de APIs tiene su propio rate limit, con buckets por IP

**Contexto.** Un único bucket global hacía que el límite estricto de `/auth` (10/min) consumiera el cupo de toda la API.

**Decisión.** Cubos separados por `(IP, límite)`: `RateLimitMiddleware(10)` para login/register y
`RateLimitMiddleware(300)` para el resto de `/api`. `middleware/auth.go:182-213`.

**Consecuencias.** `SetTrustedProxies` es **obligatorio** para que `ClientIP()` respete
`X-Forwarded-For` de nginx; sin él, todos los clientes comparten un cubo.

---

## A-19 · `preshed` está fijado a 3.0.13

**Contexto.** `preshed` 3.0.14 rompe la carga de **cualquier** modelo spaCy con
`OverflowError: Python int too large to convert to C ssize_t` (`explosion/spaCy#14047`), porque
`PreshMap` itera claves uint64 ≥ 2^63. El síntoma es que el worker NER muere al arrancar con
"No se pudo cargar ningún modelo spaCy".

**Decisión.** `preshed==3.0.13` fijado en `Dockerfile.ner:8-12` con el enlace al issue upstream.

**Consecuencias.** Cualquier actualización de spaCy debe volver a fijar `preshed`.

---

## A-20 · `MAX_SEQ_PER_CALL=8` por el límite de cgroup, no por gusto

**Contexto.** El pico de memoria del KV cache de CTranslate2 es de ~24 KB por token por secuencia
(12 capas × 1024 dims, int8), reservado hasta `MAX_SRC_TOKENS + MAX_NEW_TOKENS`.

**Decisión.** Valor 8. Mediciones registradas en `docker-compose.yml:97-99`:
8 seq ≈ 1,8 GiB · 16 → ~2,6 GiB · 32 → ~3,1 GiB · 128 → ~6,3 GiB — **todos matados por el cgroup de 3 GB**.

**Consecuencias.** El límite de cgroup **cuenta las páginas swapeadas**: ampliar el swap del host
**no** ayuda; sólo reduce el pico del contenedor. Esto es lo que limita el throughput del traductor
(ISS-001).

---

## A-21 · Las réplicas `rss2-backend-run-*` se gestionan a mano

**Contexto.** Se crearon con `docker run` para escalar el clustering worker.

**Decisión.** No están en `docker-compose.yml`. Docker las reinicia al arrancar (tienen
`unless-stopped`), pero `docker compose up/down` no las toca.

**Consecuencias.** Si cambia la imagen del backend, hay que recrearlas a mano; si no, siguen
ejecutando el binario viejo. Estado actual: **5 réplicas** vivas.

---

## A-22 · El DDL de TTS vive en tres sitios, con un test de paridad

**Contexto.** `init-db/57-add-tts.sql` (canónico), `migrations/add_tts.sql` (bases existentes) y
`workers/tts/schema.py` (el worker se auto-abastece).

**Decisión.** Los tres se mantienen sincronizados y `workers/tests/test_tts_schema_parity.py`
comprueba que los tres declaran las mismas 6 tablas con las mismas columnas.

**Consecuencias.** Es el patrón correcto para el caso "el DDL está en tres sitios". **No hay un
equivalente para `textclean`** (que es código, no DDL) — ver A-06.

---

## A-23 · El audio se sirve sólo a usuarios autenticados, con path traversal bloqueado

**Contexto.** El audio es el contenido más sensible (síntesis de voz) y el directorio está en disco.

**Decisión.** `GET /api/tts/audios/:id/file` y `GET /api/tts/bulletins/:id/file` exigen JWT; el
contenedor `backend` monta `./data/tts_audio` en **solo lectura**; `resolveTTSAudioPath` rechaza
rutas absolutas y cualquier ruta que escape del directorio base.

**Consecuencias.** Hay test unitario para el escape por symlink. El directorio de muestras de voz
(`TTS_VOICES_DIR`) **no** lo sirve ningún servidor web → las muestras de voz quedan sin exponer.

---

## A-24 · El servicio TTS precarga sólo el motor por defecto

**Contexto.** Cada motor ocupa ~2 GB de RAM y la carga tarda (Chatterbox: 91 s medidos).

**Decisión.** Se precarga **sólo** `TTS_ENGINE` en un hilo de fondo al arrancar; los demás se cargan
en el primer uso.

**Consecuencias.** El healthcheck del contenedor TTS necesita `start_period: 600s`. Con el límite
de 8 GB del contenedor, tener los tres motores cargados a la vez (6 GB) es arriesgado.

---

## A-25 · La cola de traducción se cachea en Redis con TTL de 30 días

**Contexto.** NLLB es determinista: el mismo texto → la misma traducción. Re-traducir es caro.

**Decisión.** Redis `tr:{from}:{to}:{md5(texto)}`, TTL 30 días, lectura por lotes con `MGET`/pipeline,
degradación limpia si Redis no está.

**Consecuencias.** Es la razón de que Redis tenga 413.732 claves (el 99 % es esta caché). Con
`allkeys-lru` y 512 MB, hay presión de memoria (ver ISS-024).

---

## A-26 · Se traduce todo, y **primero lo más reciente**

**Contexto.** El traductor elige el siguiente lote de 128 trabajos con un `ORDER BY`. La dirección
de ese orden **es una política de producto**, no un detalle de rendimiento.

**Decisión (propietario, 2026-10-26).** *"Se traduce todo"* **y** *"necesito que las noticias
traducidas sean las primeras en traducir"*.

**Orden elegido: `ORDER BY n.fecha DESC`** — el más reciente **por fecha de publicación** primero.
Es decir, **se mantiene el comportamiento actual**. Se descarta la propuesta de orden FIFO.

**Consecuencias.**
- Coherencia con el resto del producto: la app, las listas, la búsqueda y el noticiario ordenan
  todo por `n.fecha`. Traducir en el mismo orden hace que lo que aparece arriba sea, además,
  lo ya traducido.
- ⚠️ **La consecuencia hay que aceptarla explícitamente**: con capacidad insuficiente
  (31/min frente a 331/min de ingesta, **9,4 %** de cobertura), el orden "más reciente primero"
  significa que **la cola antigua nunca se alcanza**. Reparto medido del backlog:

  | Antigüedad de publicación | Pendientes |
  |---|---|
  | Últimas 24 h | 126.148 |
  | 1–3 días | 80.115 |
  | 3–7 días | 69.253 |
  | **Más de 7 días** | **155.071** |

  Bajo newest-first puro, esas 155.071 (y las que sigan llegando de la cola) esperarían
  indefinidamente. **"Se traduce todo" sólo es cierto si la capacidad sube** (W7.2/W7.3).
- Para que "se traduce todo" sea compatible con "primero lo más reciente", la opción es un
  **reparto justo**: una fracción de los lotes va al más reciente y el resto a la cola antigua
  (ver W7.1b). El porcentaje concreto lo decide el propietario (P-02/P-09).

---

## A-27 · El motor de TTS de producción es Chatterbox

**Contexto.** Hay tres motores instalados (Chatterbox, MeloTTS, OpenVoice) y sólo Chatterbox se ha
cargado alguna vez (`/health`: `melotts.loaded=false`, `openvoice.loaded=false`).

**Decisión (propietario, 2026-10-26).** *"El motor de TTS es Chatterbox."*

**Consecuencias.**
- Chatterbox es además el **único** que hace clonación en un paso (audio de referencia → voz).
  La decisión de A-28 es coherente con ella y **no requiere ningún motor adicional**.
- MeloTTS y OpenVoice pasan a **no-production**. No hace falta desinstalarlos: `TTS_ENGINES` es
  una lista y `TTS_ENGINE` el default (`workers/tts/config.py:82-83`). Se pueden dejar instalados
  (carga diferida, 0 RAM en reposo) para comparar, pero **deben salir de `TTS_ENGINES` por defecto**.
- ⚠️ **Consecuencia a vigilar**: `tts_voices.engine` se persiste por voz y
  `resolveTTSEngine` (`handlers/tts.go:2132-2149`) prioriza **la voz** sobre el default global.
  Hay **14 voces con `engine='openvoice'`** (10 `ov-*`, 3 `demo-speaker*`, 1 `referencia-ejemplo`)
  y **3 con `engine='melotts'`** (`melotts-speaker-42/-200/-0`) en la base. Si se desactiva
  OpenVoice sin migrar esas filas, esas voces fallarán con `unknown_engine`.
  ⇒ Acciones: (a) migrar las voces `ov-*`/`melotts-*` a `chatterbox` conservando su referencia,
  o (b) marcarlas como deshabilitadas y que la UI las oculte.
- La ruta de clonación de OpenVoice (`wavmark` stubbeado → salida sin marca de agua) queda
  **descartada**, lo que elimina de paso ese riesgo.

---

## A-28 · Se admiten voces estándar y clonadas, con el modelo actual

**Contexto.** El propietario valoraba poder clonar su propia voz y confirmar que no habría fricción
legal.

**Decisión (propietario, 2026-10-26).** *"Se puede usar voces estándar y clonada como funciona
ahora."*

**Estado actual que esto ratifica.**
- **Voces estándar**: `es-base` es la voz por defecto, respaldada por los `conds` guardados del
  modelo (`workers/tts/engine.py:700-701, 821`). No necesita referencia.
- **Voces clonadas**: `POST /api/tts/voices` (sólo admin) sube un audio de referencia
  (≤ 50 MB, extensiones whitelistadas), que Chatterbox usa como `audio_prompt_path`.
  El control de calidad (`analyze_sample`) marca muestras demasiado cortas, largas, de baja
  frecuencia, estéreo, silenciosas o saturadas.
- El registro de voces y el motor se persisten en `tts_voices.engine` / `tts_audios.engine`
  → **no hace falta tocar el esquema** para esta decisión.

**Consecuencias.**
- **B-06 (registro de consentimiento) deja de ser bloqueante** y pasa a "higiene recomendada".
  Se mantiene como recomendación, no como requisito previo: el modelo actual (subida sólo por
  admin, con control de calidad) es el que el propietario ha elegido.
- El alcance del trabajo TTS se reduce a **rendimiento** (W8.1/W8.2) y a la evaluación de motores
  **alternativos** (W8.3), no a añadir clonación.
- Sigue siendo recomendable dejar constancia de la autorización de cada voz clonada (una columna y
  un campo de formulario). Coste bajo, riesgo legal evitado.

---

## A-29 · El presupuesto de tokens del traductor se ajusta al tamaño real de los textos

**Contexto.** El traductor reservaba `MAX_SRC_TOKENS + MAX_NEW_TOKENS = 4096` tokens por
secuencia. Medido sobre el corpus real (`[DB]`, textos con `lang` conocido ≠ `es`):

| Métrica | Valor |
|---|---|
| Título | 62 caracteres de media |
| Título + resumen | 620 de media · **p50 = 305** · **p95 = 2.970** |
| Fragmento real enviado al modelo | ≤ 2.000 caracteres (`BODY_CHARS_CHUNK`) + título |

Es decir: **ninguna llamada real al modelo se acerca a 2.048 tokens** (~525 tokens como máximo
real), pero la reserva de memoria es ~4× mayor de lo necesario.

**Decisión.** Ajustar `MAX_SRC_TOKENS` y `MAX_NEW_TOKENS` al tamaño real observado y subir
`MAX_SEQ_PER_CALL` en la misma proporción, manteniendo el límite de cgroup de 3 GB.

**Consecuencias.**
- 4.096 → 2.048 tokens por secuencia = **2× menos memoria reservada** → `MAX_SEQ_PER_CALL`
  podría pasar de 8 a 16 con el mismo límite.
- Teóricamente **~2× de rendimiento** en la misma máquina. **Es una hipótesis a medir**: el plan
  W7.2 incluye el experimento controlado (ventana de mantenimiento, medir lotes y pico de RSS).
- ⚠️ El recorte es real si se baja de 768 tokens: `translate_texts` trunca el origen con
  `truncation=True, max_length=MAX_SRC_TOKENS` (línea 288). Con 1.024 tokens el corte no afecta
  a ningún caso real (los fragmentos ya están troceados a 2.000 caracteres).
- La regla de `AGENTS.md` se mantiene: **el cgroup cuenta las páginas swapeadas**, así que el
  pico de memoria del contenedor es el límite, no el tamaño de lote nominal.

---

## A-30 · La calidad de la traducción se evalúa contra el corpus real, no contra un promedio

**Contexto.** La detección de idioma y la traducción se medían sobre promedios que no describen
la distribución real.

**Decisión.** Cualquier cambio de calidad o de rendimiento se evalúa sobre las métricas reales
medidas: p50/p95 de longitud de texto, distribución de idiomas, y **tasa de duplicados por título
(1,05 — es decir, no hay duplicación sistemática)**.

**Consecuencias.**
- Se descarta la hipótesis de "muchas noticias duplicadas" (factor 1,05 medido sobre 477.659
  noticias de 24 h): el volumen es real.
- La cifra real de ingesta es **477.659 noticias/día desde 17.090 feeds (28,0 por feed/día)**,
  entre 2,8× y 4× la estimación del propietario (7–10 por feed/día). Cualquier dimensionado de
  la traducción debe usar **477 K/día**, no 200 K/día.

---

## A-31 · El orden FIFO por `t.created_at ASC` queda **descartado** (medido)

**Contexto.** Se propuso cambiar el reclamo a `ORDER BY t.created_at ASC` (el trabajo más antiguo
primero) por dos motivos: (a) "traducir todo" sugiere drenar la cola en orden, y (b) la consulta
resultaba mucho más barata.

**Medición** (`[DB]`, `EXPLAIN ANALYZE` sobre la consulta real del traductor):

| Variante | Tiempo | Buffers | Filas de `noticias` sondeadas |
|---|---|---|---|
| `ORDER BY n.fecha DESC` (actual) | **93,2 ms** | 17.938 | 3.551 |
| `ORDER BY t.created_at ASC` (FIFO) | **6,6 ms** | 1.012 | 0 |
| `ORDER BY t.created_at DESC` (LIFO por llegada) | **4,9 ms** | 945 | 0 |

**Por qué se descarta.** Los dos órdenes **no son equivalentes**: el solape entre el top-128 por
`n.fecha DESC` y el top-128 por `t.created_at DESC` es de **1 noticia de 128**. `fecha` es la fecha
**de publicación** (puede ser muy antigua: hay noticias de abril de 2025) y `created_at` es el
momento en que **entró en el sistema**. Cambiar a FIFO traduciría primero las noticias más
antiguas, que es exactamente lo contrario de lo pedido.

**Dos matices honestos sobre la medida.**
1. La ventaja de rendimiento **no es relevante para el caudal**: 93 ms sobre un lote de ~500.000 ms
   de inferencia es **0,02 %**. El `ORDER BY` actual **no** es un cuello de botella.
2. El `ORDER BY` sí tiene un coste **real, pero distinto**: al hacer `JOIN` sin `OF t`,
   PostgreSQL bloquea filas de `noticias` además de `traducciones` (ver ISS-022) → contención con
   `langdetect` y `topics`. Eso sí conviene arreglarlo, y **sin cambiar el orden**.

**Consecuencias.**
- **No se cambia el `ORDER BY`.** Se conserva `n.fecha DESC`.
- La corrección de ISS-022 (`OF t`) se aplica **conservando el orden actual**.
- La reducción del coste de la consulta queda como **mejora opcional**, no como palanca de caudal.

---

---

# Parte B — Recomendaciones nuevas (NO son decisiones)

> Nada de esta parte está implementado. Cada punto indica su dependencia y su riesgo.

## B-01 · Desplegar los `remote-worker` en hosts adicionales

> **Estado tras A-26: PALANCA PRINCIPAL.** Con "se traduce todo", la capacidad de traducción debe
> pasar de ~31/min a ~331–380/min. Esta es la vía de escalado horizontal que el diseño ya soporta.

**Motivación.** El protocolo WebSocket existe, el backend asigna trabajos (`StartJobAssigner`, 5 s),
la recuperación de trabajos atascados existe (`StartStaleJobCleanup`), y el cliente tiene reintentos
y backoff. A ~15/min por worker, **hacen falta 20–23 workers remotos** para cerrar la brecha
(tras aplicar A-29, que multiplica por ~2 el rendimiento por core).
**Estado real:** 0 hosts operativos; 1 worker registrado con `status='online'` **falso**
(`last_seen` del 2026-10-09) → hay que arreglar también el contador del panel de admin.
**Depende de:** resolver el estado `online` obsoleto (`backend/internal/handlers/admin.go:548` cuenta
`status='online'` desde la BD sin comprobar `last_seen`).
**Riesgo:** bajo — el camino ya está probado en el código.

## B-02 · Correr el traductor en GPU si aparece hardware NVIDIA

**Motivación.** El perfil `gpu` ya existe (`translator-gpu`, `Dockerfile.gpu`, `float16`/cuda).
`MAX_SEQ_PER_CALL` subiría de 8 a 128 y el throughput se multiplicaría.
**Depende de:** hardware + que el `models/nllb-ct2` se convierta a `float16`.
**Riesgo:** medio — hay que medir la calidad de la conversión y el consumo.

## B-03 · Mover el TTS a GPU

**Motivación.** Chatterbox en CPU es 18,6× tiempo real (ISS-002); en CUDA sería 10–50× más rápido.
**Depende de:** `Dockerfile.tts` con torch CUDA (ahora es CPU-only), perfil de compose, y un
cuidadoso análisis de concurrencia (Chatterbox es stateful: el clonado muta `model.conds`, y el
servicio mantiene un lock → **no** se puede paralelizar trivialmente).
**Riesgo:** medio-alto.

## B-04 · Persistir la parrilla del noticiario

**Motivación.** Hoy se pierde al recargar (ISS-006).
**Diseño mínimo:** un estado `lineup` en `tts_bulletins` (o una tabla `tts_lineups`) con el orden
de los `script_ids`, guardado al reorder y recuperado al abrir la página.
**Depende de:** ISS-006 también exige derivar `script_ids` de la selección vigente.
**Riesgo:** bajo — sólo afecta al subsistema TTS.

## B-05 · Añadir notas personales a los guiones

**Motivación.** El propietario quiere escribir comentarios, explicaciones, ironía o contexto que
**no** se locutan.
**Diseño mínimo:** columna `notes TEXT` en `tts_scripts` + campo en `TtsScript` + un `<textarea>`
secundario en el editor. La nota **no** entra en el texto que se sintetiza.
**Depende de:** migración (nº 66) → **hay que actualizar `init-db/`, `migrations/` y
`workers/tts/schema.py`** y volver a pasar `make tts-test` (paridad del DDL).
**Riesgo:** bajo.

## B-06 · Registrar el consentimiento de las voces clonadas

> **Estado tras A-28: NO BLOQUEANTE.** El propietario ha autorizado el modelo actual (subida sólo
> por admin + control de calidad). Pasa de "requisito previo a habilitar la clonación" a
> "higiene recomendada".

**Motivación.** `POST /api/tts/voices` sube cualquier audio y lo usa para clonar. Sin registro de
autorización, clonar la voz de un tercero sigue siendo un riesgo legal para el propietario, aunque
la política de producto sea la actual.
**Diseño mínimo:** columnas `consent_*` en `tts_voices` + verificación en el handler de alta +
una marca visible en la UI.
**Depende de:** B-05 (misma migración) + decidir si las voces propias quedan exentas.
**Riesgo:** bajo.

## B-07 · Instrumentar la aplicación con métricas Prometheus

**Motivación.** ISS-015: Prometheus no scrapea nada de la aplicación y no hay dashboards.
**Diseño mínimo:** `promhttp` en el backend Go (contadores ya existen: `rateLimitHitsTotal`,
`rateLimitHitsByCfg`), un endpoint `/metrics` en los workers Python (los `translation_stats` ya
existen en BD), y 2–3 dashboards.
**Riesgo:** bajo.

## B-08 · Crear un índice parcial en `feeds` para la ingesta

**Motivación.** ISS-003: `getActiveFeeds()` hace un seq scan de 33 K filas por ciclo.
**Diseño:** `CREATE INDEX CONCURRENTLY idx_feeds_activo ON feeds (id) WHERE activo = TRUE` — o
`(activo, fallos)`.
**Depende de:** ventana de mantenimiento (es un `CREATE INDEX`, no `CONCURRENTLY` bloquea escrituras).
**Riesgo:** bajo.

## B-09 · Sustituir el `ORDER BY` multi-tabla de `GetFeeds`

**Motivación.** ISS-003/B4: la consulta más cara de la API ordena por columnas de `paises` y
`categorias`, lo que obliga a leer y ordenar toda la tabla.
**Diseño:** ordenar por `feeds` (que ya tiene índices) y enrich en Go, o precomputar
`categoria_nombre`/`pais_nombre` en la fila de `feeds`.
**Riesgo:** medio — cambia el orden visible de la tabla de feeds (tests de UI podrían depender).

## B-10 · Escribir `embedding_vector` por lotes

**Motivación.** ISS-022: un UPDATE por fila.
**Diseño:** `execute_values` con la expresión vectorial, o `COPY` a una tabla temporal +
`INSERT … SELECT`.
**Riesgo:** bajo.

## B-11 · Reducir el número de feeds activos

**Motivación.** ISS-001: 28.250 feeds producen 635 K noticias/día y la traducción no llega.
**Diseño:** usar el bulk-delete existente (con su 428 de confirmación) para retirar los feeds de
menor valor; o un límite por categoría.
**Depende de:** decisión del propietario sobre qué fuentes valen.
**Riesgo:** medio — se pierde cobertura.

## B-12 · Traducir por niveles: título + resumen primero, contenido bajo demanda

> **Estado tras A-26: sigue siendo válida, pero cambia de objetivo.** Ya no es "traducir menos",
> es "**traducir antes**": el objetivo sigue siendo traducirlo todo, pero el texto que el usuario
> ve (noticiario, búsqueda) está disponible mucho antes.
> ⚠️ **Con el orden "más reciente primero" (A-26), esta optimización es todavía más valiosa**:
> es la única forma de que la noticia recién ingerida esté disponible en español en minutos en
> lugar de horas.

**Motivación.** Con A-26, la cobertura crecerá muy despacio si cada noticia exige su contenido
completo. El `min_chars` del Noticiario (150 caracteres) se satisface con el resumen, y `search.go`
ya acepta `COALESCE(contenido_trad, contenido, resumen_trad, resumen)` como texto narrable.
**Diseño:** el traductor llena `titulo_trad` y `resumen_trad` y deja `contenido_trad` para una
segunda pasada; el noticiario trabaja con el resumen.
**Nota:** el contenido (`noticias.contenido`) está vacío en el 99,9 % de las filas (668 de 814.290),
así que **hoy el nivel 2 es casi vacío** — el beneficio real está en el resumen, no en el contenido.
**Riesgo:** medio-alto — cambia la semántica de `status='done'`.

## B-13 · Unificar la configuración en `.env` (eliminar valores fijos en compose)

**Motivación.** ISS-010: hay al menos 8 variables cuyo valor en `.env` no tiene efecto.
**Diseño:** `${VAR:-default}` en compose para todas las variables que hoy están fijas
(`RSS_MAX_WORKERS`, `RSS_POKE_INTERVAL_MIN`, `EMB_BATCH`, `ALERTS_SCAN_INTERVAL_MIN`, …).
**Riesgo:** bajo.

## B-14 · Eliminar el código muerto

**Motivación.** ISS-010: 2 handlers sin ruta, 2 clientes API en el frontend, 5 páginas no enrutadas,
4 variables de config sin uso, 2 funciones de `services/ml.go` sin llamantes, 4 familias de claves
Redis sin escritor, el `Dockerfile` de la raíz sin usar, un binario de 12,7 MB versionado en git.
**Riesgo:** bajo — pero conviene hacerlo en commits separados y verificables.

## B-16 · Tier frío en otra base de datos, con ventana de ~10 días

> Requisito del propietario (2026-10-26): *"que el archivo de la base de datos sea otra base de
> datos que no le afecte las consultas, para aliviar la carga"*. **No es una decisión: es una
> propuesta con las mediciones que la sostienen** (ver `ARCHITECTURE.md` §13).
>
> ⏸️ **APLAZADO por el propietario el 2026-10-26**: *"vamos a dejar de lado este tema de la 2ª base
> de datos y nos centramos en los demás problemas iniciales"*. No se implementa nada de esto ahora.
> **Se reactiva cuando:** la latencia de las consultas con filtros se degrade de forma medible
> (hoy `/api/news` = 0,16 s y `/api/search` = 0,22 s), o el corpus supere ~5 M filas
> (≈10 días al ritmo real de 477.659 not/día), o la RAM del host deje de alojar
> cómodamente el índice HNSW (>20 GB).

**Motivación (medida).** El corpus crece **9,75 KB por noticia** (heap 1,8 + índices 0,85 + vector
y HNSW 7,1). A 477.659 noticias/día son **44 GB a 10 días, 400 GB a 90 días y 1,6 TB a un año**.
El `COUNT(*) FROM noticias` —que el backend ejecuta en cada página de `/api/news` y `/api/search`
cuando hay filtros— ya cuesta **635 ms** con 814 K filas y escala lineal: **33,5 s a 90 días**.

**Propuesta.** Dos niveles: el Postgres actual conserva una **ventana de ~10 días** y el histórico
va a **otra instancia de base de datos**, con `postgres_fdw` para que el backend siga haciendo
**una sola consulta** (UNION) a través de la frontera.

**Regla imprescindible (compatibilidad con A-26 "se traduce todo").** El archivador mueve una
noticia al nivel frío **sólo si ya está traducida** (o su idioma es el destino, o está marcada
como no traducible). Las noticias antiguas sin traducir **siguen en caliente** hasta que se
traduzcan. Si no, la ventana de 10 días cortaría la cola de traducción y "se traduce todo" sería
falso.

**Motor recomendado.** **PostgreSQL (2ª instancia)**: cero tecnología nueva, mismo SQL y driver,
`make check-schema` sigue sirviendo, y el repo ya tiene `wal_level=replica` + `replica_slot` +
`init-replica/` (aunque **no hay réplica en marcha**: el slot está `active = f`).
**ClickHouse** es mejor para analítica pura pero exige reescribir cada consulta del archivo.

**Riesgo:** medio-alto — toca el esquema de flujo de datos (qué pasa con `traducciones`,
`tags_noticia` y `traduccion_embeddings` al mover una noticia) y la capa de consultas.
**Depende de:** W1 (métricas) y de P-11/P-12.

> ⏸️ **Estado: APLAZADO** (2026-10-26). La documentación se conserva para no perder el trabajo de
> medición y diseño, pero **no entra en el plan de trabajo actual**.

---

## B-15 · Fijar la semilla de `langdetect` y añadir `OF t` al `FOR UPDATE` del traductor

**Motivación.** ISS-019, ISS-020.
**Riesgo:** bajo — cambios de una línea, con efecto medible en contención.

---

# Parte C — Preguntas abiertas

> **Actualizado 2026-10-26.** P-01, P-03 y P-04 están **resueltas** por el propietario y se han
> convertido en decisiones verificadas (A-26, A-27, A-28).

| # | Pregunta | Estado | Resolución / nota |
|---|---|---|---|
| ~~P-01~~ | ¿El corpus debe traducirse completo o sólo "lo relevante"? | ✅ **Resuelta 2026-10-26** | *"Se traduce todo."* → **A-26**. Implica cerrar la brecha de ~11× (W7) |
| ~~P-03~~ | ¿Qué motor de TTS se quiere para producción? | ✅ **Resuelta 2026-10-26** | *"Chatterbox."* → **A-27**. MeloTTS y OpenVoice salen de producción |
| ~~P-04~~ | ¿Se puede usar voz clonada de terceros? | ✅ **Resuelta 2026-10-26** | *"Voces estándar y clonada como funciona ahora."* → **A-28**. B-06 deja de ser bloqueante |
| **P-02** | ¿Se acepta que la traducción se retrase? | 🟡 **Abierta** | Con A-26 (más reciente primero) el retraso es estructural. **Reformulada:** ¿cuánto retraso es aceptable? Si es "horas", la capacidad mínima es la de la ingesta (331/min). Si es "días", hace falta menos hardware pero el histórico tarda |
| **P-05** | ¿El servicio sigue siendo monohuérfano o baja a un host de producción? | 🟡 Abierta | Determina orquestación, backups y CI. **Afecta a W7:** los workers remotos asumen hosts separados |
| **P-06** | ¿Se acepta perder el histórico (`noticias` sin retención)? | 🟡 Abierta | Sin retención, la base crece ~1 GB/día. **Afecta a W7:** con "se traduce todo", el histórico sin traducir sigue creciendo |
| **P-07** | ¿El TTS sigue en el mismo host que el resto? | 🟡 Abierta | Con 8 GB de TTS + 4 GB de Postgres + CPU, hay contención. Ahora es más relevante: el TTS es el único consumidor **controlable por configuración** |
| **P-08** | ¿Se quiere un modo "sin traducción" para consultas de volumen? | 🟡 Abierta | Hoy todo el pipeline asume traducción. Con A-26, convivirán noticias traducidas y sin traducir durante meses |

### Preguntas nuevas que surgen de A-26

| # | Pregunta | Por qué importa |
|---|---|---|
| **P-09** | ¿Con qué horizonte se cierra el backlog de 427.200 traducciones pendientes? | A 331/min (ritmo de la ingesta) el backlog no crece pero no baja. A 380/min baja en ~30 días. La respuesta determina cuántos workers remotos o GPUs hacen falta |
| **P-10** | ¿Se acepta recortar los 477.659 artículos/día (28 por feed) reduciendo feeds o items? | Es la palanca más barata: reduce proporcionalmente la demanda. El factor de duplicación por título es **1,05**, así que **no hay duplicación que eliminar**: el volumen es real |

---

## Referencia rápida

| Decisión | Fichero clave |
|---|---|
| A-01 id por URL normalizada | `rss-ingestor-go/main.go:240-279` |
| A-02 Postgres como fuente de verdad | `backend/internal/cache/redis.go`, `workers/tts/queue.py` |
| A-03 `LEFT JOIN` para "traducido" | `backend/internal/handlers/search.go:301-331` |
| A-04 `feeds.traducido` | `rss-ingestor-go/main.go:447-451` |
| A-05 baseline ponderado | `backend/internal/handlers/alerts.go:316-340` |
| A-06 paridad de `textclean` | `Makefile:62-64` |
| A-07 TTS interno | `docker-compose.yml:518-588` |
| A-08 ensamblado en servidor | `workers/tts/bulletin.py` |
| A-09 prioridad del motor | `backend/internal/handlers/tts.go:2132-2149` |
| A-10 idempotencia por `ON CONFLICT` | varios |
| A-11 sin `pq.CopyIn` | `rss-ingestor-go/main.go:469-473` |
| A-12 priorizar lo reciente | `translator/workers/ctranslator_worker.py:692` |
| A-13 dos rutas del scheduler | `workers/translation_scheduler.py:229-242` |
| A-14 precondición 428 | `backend/internal/handlers/feed.go:389-398` |
| A-15 `total` con el mismo WHERE | `backend/internal/handlers/news.go:345-374` |
| A-16 `$1` = idioma destino | `backend/internal/services/ml.go:252-293` |
| A-17 nginx con resolver | `nginx.conf:36-44, 75-91` |
| A-18 buckets de rate limit | `backend/internal/middleware/auth.go:182-213` |
| A-19 `preshed==3.0.13` | `Dockerfile.ner:8-12` |
| A-20 `MAX_SEQ_PER_CALL=8` | `docker-compose.yml:97-99` |
| A-21 réplicas a mano | `docker-compose.yml` (ausentes) |
| A-22 paridad del DDL de TTS | `workers/tests/test_tts_schema_parity.py` |
| A-23 audio sólo autenticado | `backend/internal/handlers/tts.go:1258, 2104-2124` |
| A-24 precarga sólo el default | `workers/tts/service.py:91-110` |
| A-25 caché de traducción | `translator/workers/ctranslator_worker.py:85-137` |
| A-26 se traduce todo, primero lo más reciente | `translator/workers/ctranslator_worker.py:692` |
| A-31 el orden FIFO por `created_at ASC` queda descartado | `translator/workers/ctranslator_worker.py:692` (medición con `EXPLAIN ANALYZE`) |
| A-27 motor de TTS = Chatterbox | `workers/tts/config.py:82-83`, `backend/internal/handlers/tts.go:2132-2149` |
| A-28 voces estándar y clonadas | `workers/tts/voices.py`, `workers/tts/engine.py:761-831` |
| A-29 presupuesto de tokens ajustado | `translator/workers/ctranslator_worker.py:43-46` |
| A-30 evaluación sobre el corpus real | `[DB]` métricas de longitud y duplicados |
