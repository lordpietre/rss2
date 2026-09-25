# Spec 07 — Workers pendientes de activar

Binarios ya compilados en la imagen backend sin servicio en compose.
Activar solo con necesidad + env `DB_*` reales (y resto de su tabla).

| Worker (`command`) | Lee | Escribe | Env clave | Estado / nota |
|---|---|---|---|---|
| `scraper` | `fuentes_url` active + `noticias` resumen<200 | `noticias`, `fuentes_url.last_*` | `SCRAPER_INTERVAL=60`, `SCRAPER_WORKERS=5` | [x] ACTIVO 2026-09-14 (`/scraper`): enriquece resúmenes (verificado en logs); sin `fuentes_url` no hay ingesta nueva por esta vía |
| `discovery` | `fuentes_url` | `feeds_pending`/`feeds`, `fuentes_url.last_*` | `DISCOVERY_INTERVAL=900` (sin run inicial), `MAX_FEEDS_PER_URL=5` | [x] ACTIVO 2026-09-14 (`/discovery`): en espera (`fuentes_url` vacía). `findFeedLinksInHTML` sigue stub: solo detecta URLs que YA son feeds |
| `wiki-worker` | `tags` no checked | `tags.wiki_*`, thumbs en `/app/data/wiki_images` | `WIKI_SLEEP=30`, `WIKI_BATCH=20` | [x] ACTIVO 2026-09-14 como servicio `wiki` (imagen `rss2-backend`, `entrypoint: /wiki-worker`; `Dockerfile.wiki` legacy: contexto raíz demasiado pesado) |
| `topics` | `noticias` no procesadas, `topics`, `paises` | `news_topics`, `noticias.pais_id` | `TOPICS_SLEEP=10`, `TOPICS_BATCH=500` | [x] ACTIVO 2026-09-14 como servicio `topics` (imagen `rss2-backend`, `entrypoint: /topics`; procesa 500/batch) |
| `related` | `traduccion_embeddings` | `related_noticias` (coseno CPU) | `RELATED_*`, `EMB_MODEL` = mismo que embeddings | [x] ACTIVO 2026-09-14 (`/related`, verificado: 6000 filas y creciendo) |
| ~~`qdrant-worker`~~ | — | — | — | [x] ELIMINADO 2026-09-15: nadie leía Qdrant (`SemanticSearch` es stub que devuelve vacío; frontend sin entrada semántica; related usa coseno SQL). Verificación exhaustiva previa en specs/tasks. |

## Plan de activación sugerido

Completado 2026-09-14: los 6 workers están activos (ver tabla).

Decisiones tomadas (ya no bloquean):

- Embeddings: sentence-transformers local
  `paraphrase-multilingual-MiniLM-L12-v2` (CPU, modelo ya en `hf_cache`,
  servicio `embeddings` con `Dockerfile.embeddings`). `EMB_MODEL` idéntico
  en `embeddings` y `related` (el default `mxbai-embed-large` era de Ollama,
  retirado). `qdrant-worker` lee `dim` de la tabla: agnóstico al modelo.
- `fuentes_url`: servicios activos sin semillas (tabla vacía). Scraper
  igual hace trabajo real (enriquecimiento); discovery queda en espera.
  Sembrar URLs es operativo (requieren revisión vía `feeds_pending` salvo
  que la fuente traiga categoria+país).

## Notas de activación (2026-09-14, no regressar)

- `workers.Connect` (`internal/workers/db.go`) ahora crea el pool desde
  `DB_*` (password URL-escaped). Antes solo verificaba `db.GetPool()`,
  que siempre era nil fuera de `server` → los 6 workers morían con
  "database pool not initialized" al activarlos.
- Servicios nuevos reutilizan `image: rss2-backend` SIN `build` (el
  binario ya se compila en `backend/Dockerfile`): evita el conflicto
  `AlreadyExists` de construir dos servicios contra el mismo tag.
- Dockerfiles legacy de raíz (`Dockerfile.wiki/related/qdrant/scraper/
  discovery/topics/translator*/remote-worker`) eliminados 2026-09-14
  (nadie los referenciaba; el target `docker-build` del Makefile, roto,
  ahora usa compose).

## Tasks

- [x] Servicio `wiki` + verificado `tags.wiki_summary` e imágenes.
- [x] Servicio `topics` + verificado (`news_topics` creciendo).
- [x] Decisión embeddings: local sentence-transformers (servicio
  `embeddings` activo; `related` 6000+ filas; `qdrant-worker` subiendo).
- [x] Decisión `fuentes_url`: activos sin semillas (scraper enriquece,
  discovery en espera); sembrar es operativo.
