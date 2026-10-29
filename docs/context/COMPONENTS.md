# RSS2 — Inventario de Componentes

> Catálogo de módulos, servicios, workers, tareas y responsabilidades.
> Verificado el **2026-10-26**. `L` = líneas del fichero.
> Fuentes: `[CÓDIGO]` = lectura de ficheros; `[DB]` = estado en producción.

---

## 1. Vista de conjunto

| # | Componente | Tipo | Imagen / binario | Puerto | Límites | Estado |
|---|---|---|---|---|---|---|
| 1 | `db` | infra | `pgvector/pgvector:pg16` | interno | 4 CPU / 4 G | ✅ |
| 2 | `redis` | infra | `redis:7-alpine` | interno | 512 MB | ✅ |
| 3 | `ingestor` | worker | `rss-ingestor-go` | — | 2 / 512 M | ✅ |
| 4 | `translation-scheduler` | worker | `rss2-pyworkers` | — | 1 / 256 M | ✅ |
| 5 | `langdetect` | worker | `rss2-pyworkers` | — | 1 / 256 M | ✅ |
| 6 | `translator` | worker | `rss2-translator` | — | 2 / 3 G | ✅ |
| 7 | `translator-2` | worker | `rss2-translator` | — | 2 / 3 G | ✅ |
| 8 | `translator-gpu` | worker (perfil `gpu`) | `rss2-translator` | — | GPU | ⏸ inactivo |
| 9 | `ner` | worker | `rss2-ner` | — | 2 / 2 G | ✅ |
| 10 | `embeddings` | worker + API | `rss2-embeddings` | **6000 (host)** | 2 / 2 G | ✅ |
| 11 | `tts` | servicio | `rss2-tts` | interno :7000 | 3 / 8 G | ✅ |
| 12 | `backend` | API | `rss2-backend` | **8080 (host)** | 4 / 512 M | ✅ |
| 13 | `frontend` | SPA | `rss2-frontend` | interno :80 | — | ✅ |
| 14 | `nginx` | gateway | `nginx:alpine` | **8888 (host)** | — | ✅ |
| 15 | `wiki` | worker | `rss2-backend` | — | 1 / 512 M | ✅ |
| 16 | `topics` | worker | `rss2-backend` | — | 2 / 512 M | ✅ |
| 17 | `related` | worker | `rss2-backend` | — | 2 / 1 G | ✅ |
| 18 | `event-clustering` | worker | `rss2-backend` | — | 2 / 1 G | ✅ |
| 19 | `scraper` | worker | `rss2-backend` | — | 2 / 512 M | ✅ |
| 20 | `discovery` | worker | `rss2-backend` | — | 2 / 512 M | ✅ |
| 21 | `cooccurrences` | worker | `rss2-backend` | — | 0,5 / 256 M | ✅ |
| 22 | `prometheus` | infra (perfil `monitoring`) | upstream | — | — | ⏸ inactivo |
| 23 | `grafana` | infra (perfil `monitoring`) | upstream | — | — | ⏸ inactivo, sin dashboards |
| 24 | `cadvisor` | infra (perfil `monitoring`) | upstream | — | — | ⏸ inactivo |
| — | `sanitize` | herramienta | `/sanitize` en `rss2-backend` | — | — | one-shot, bajo demanda |
| — | `entityscan` | herramienta | `/entityscan` en `rss2-backend` | — | — | one-shot, bajo demanda |
| — | `remote-worker` | kit de despliegue | `remote-worker/` | — | — | ⏸ **fuera de este host** |

**Fuera de compose:** 5 contenedores `rss2-backend-run-*` (creados con `docker run` para escalar el
clustering worker). `docker compose up/down` no los toca. `[DB]`

---

## 2. Backend Go (`backend/`, 59 ficheros, ~20.500 L)

Módulo `github.com/rss2/backend`, Go 1.23.

### 2.1 `cmd/` — binarios

| Binario | Fichero | Función |
|---|---|---|
| `server` | `cmd/server/main.go` (≈700 L) | API REST, scanner de alertas, asignador de trabajos remotos, ficheros estáticos |
| `scraper` | `cmd/scraper/main.go` (552 L) | Enriquece `noticias.contenido` desde `fuentes_url`; extrae `og:*` y contenido principal con goquery |
| `discovery` | `cmd/discovery/main.go` (893 L) | Busca feeds RSS en páginas; gestiona salud de `fuentes_url` (desactivar/reactivar, circuit breaker por dominio) |
| `topics` | `cmd/topics/main.go` (433 L) | Palabras clave → `news_topics`; infiere país; marca `topics_processed` |
| `wiki_worker` | `cmd/wiki_worker/main.go` (525 L) | Wikipedia → `tags.wiki_summary/wiki_url/image_path` |
| `related` | `cmd/related/main.go` (387 L) | Vecinos semánticos → `related_noticias`; crea la función SQL y el índice HNSW |
| `event_clustering` | `cmd/event_clustering/main.go` (650 L) | Agrupación por evento → `event_clusters` + `noticia_clusters` |
| `cooccurrences` | `cmd/cooccurrences/main.go` (120 L) | Recalcula `entity_cooccurrences` (TRUNCATE + re-inserción, 30 días, ≥3 co-apariciones). **Único worker con `sqlx`+`lib/pq`** |
| `sanitize` | `cmd/sanitize/main.go` (488 L) | One-shot: aplica/restaura `textclean`, opcionalmente re-encola traducciones |
| `entityscan` | `cmd/entityscan/main.go` (443 L) | One-shot: clasifica `tags` con `entitycheck`, siembra `entity_blocklist`, poda alertas |

Todos los workers (excepto los one-shot) exponen **`/health` en `:8080`** (interno) y manejan
SIGINT/SIGTERM con cancelación de contexto + 2 s de gracia.

### 2.2 `internal/`

| Paquete | Ficheros | Responsabilidad |
|---|---|---|
| `handlers/` | 15 ficheros, ~9.500 L | Handlers HTTP. `news.go` (1.634 L), `tts.go` (2.173 L), `admin.go` (1.630 L), `favorites.go` (1.069 L), `feed.go` (924 L), `search.go` (693 L), `alerts.go` (478 L), `subscription.go` (428 L), `worker_ws.go` (376 L), `events.go` (280 L), `auth.go` (210 L), `tts_trends.go` (259 L), `remote_worker.go` (178 L), `validation.go` (13 L) |
| `middleware/` | `auth.go` (225 L) | `AuthRequired`, `AdminRequired`, `CORSMiddleware`, `RateLimitMiddleware`, `LoggerMiddleware`, `RequestIDMiddleware` |
| `models/` | `models.go`, `alias.go`, `remote_worker.go`, `tts.go` | DTOs JSON. `tts.go` (169 L) define las 6 estructuras `tts_*` |
| `services/` | `ml.go` (352 L), `tts.go` (330 L) | Cliente HTTP a embeddings/TTS + `SemanticSearch` (pgvector) |
| `cache/` | `redis.go` (200 L) | Caché Redis + `PushTTSJob` (LPUSH) |
| `db/` | `postgres.go` (88 L) | Pool pgx + espera a la BD (30 intentos) |
| `workers/` | `db.go` (93 L) | `LoadDBConfig` / `Connect` / `GetPool` para los workers Go |
| `auth/` | `jwt.go` | Generación/validación JWT HS256 (24 h) |
| `entitycheck/` | `entitycheck.go` (290 L), `alertas.go` (83 L), 4 tests | Clasificador de falsos positivos + umbrales de alerta |
| `textclean/` | `textclean.go` (822 L), tests | Saneado de texto (ver `KNOWN_ISSUES.md` → paridad) |
| `logger/` | `logger.go` | zerolog con nivel por `LOG_LEVEL` |

### 2.3 Handlers sin ruta (código muerto verificado)

- `handlers.GetAdminStats` — `internal/handlers/admin.go:342`
- `handlers.GetModelDownloadURL` — `internal/handlers/worker_ws.go:337`

### 2.4 Configuración (`internal/config/config.go`, 93 L)

| Variable | Default | ¿Se usa? |
|---|---|---|
| `SECRET_KEY` | — | **obligatoria**; `config.Load()` hace `panic` si falta |
| `ALLOWED_ORIGINS` | `http://localhost:5173,http://localhost:3000` | sí (CORS) |
| `SERVER_PORT` | `8080` | sí |
| `DATABASE_URL` | `postgres://rss:rss@localhost:5432/rss` | sí |
| `REDIS_URL` | `redis://localhost:6379` | sí |
| `JWT_EXPIRATION` | `24h` | ❌ **leída pero nunca usada** |
| `RATE_LIMIT_PER_MINUTE` | `300` | sí |
| `TRANSLATION_URL` | `http://libretranslate:7790` | ❌ sólo en `services.Translate`, que **nadie llama** |
| `OLLAMA_URL` | `http://ollama:11434` | ❌ **nunca usada** |
| `EMBEDDINGS_URL` | `http://embeddings:6000` | sí |
| `SPACY_URL` | `http://spacy:8000` | ❌ sólo en `services.ExtractEntities`, que **nadie llama** |
| `DEFAULT_LANG` | `es` | sí |
| `NEWS_PER_PAGE` | `30` | ❌ **nunca leída** (los handlers usan 30 hard-coded) |
| `DOCKER_COMPOSE_DIR` | `/datos/rss2` | ❌ **nunca leída** |
| `WIKI_IMAGES_PATH` | `/app/data/wiki_images` | sí |
| `TTS_URL` | `http://tts:7000` | sí |
| `TTS_API_KEY` | `""` | sí (cabecera `X-API-Key`) |
| `TTS_ENGINE` | `chatterbox` | sí |
| `TTS_AUDIO_PATH` | `/app/data/tts_audio` | sí |
| `TTS_MAX_SCRIPT_CHARS` | `8000` | sí |
| `TTS_MAX_ATTEMPTS` | `2` | sí — ⚠️ **compose no lo pasa al backend**, así que el valor real siempre es 2 |

Variables leídas fuera de `config.go`:
`db/postgres.go`: `DB_MAX_CONNS` (25), `DB_MIN_CONNS` (5), `DB_MAX_CONN_LIFETIME` (1h),
`DB_MAX_CONN_IDLE_TIME` (30m), `DB_HEALTH_CHECK_PERIOD` (1m).
`workers/db.go`: `DB_HOST/PORT/NAME/USER/PASS`, `DATABASE_URL`.
`handlers/alerts.go`: `ALERTS_MIN_HITS`, `ALERTS_MIN_RATIO`, `ALERTS_MIN_BASELINE`,
`ALERTS_MIN_BASELINE_BUCKETS`, `ALERTS_TEMA_*` (4), `ALERTS_LOOKBACK_HOURS` (24),
`ALERTS_COOLDOWN_HOURS` (6), `ALERTS_SCAN_INTERVAL_MIN` (**60 en código, 15 en compose**).

---

## 3. Ingestor Go (`rss-ingestor-go/`, 1.100 L)

Módulo **separado** `rss-ingestor-go` (go 1.25.0). Deps: `gofeed v1.2.1`, `lib/pq v1.10.9`,
`goquery v1.12.0`, `golang.org/x/net v0.52.0`.

| Función | Línea | Responsabilidad |
|---|---|---|
| `main` | 624 | Ticker + ciclo en goroutine, salta ticks si el anterior sigue vivo |
| `ingestCycle` | 569 | Lista feeds → canales → workers → suma insertados |
| `getActiveFeeds` | 197 | `SELECT … WHERE activo AND fallos < 10 ORDER BY id` (**sin LIMIT**) |
| `processFeed` | 333 | HTTP → parse → limpieza → imagen → inserción |
| `normalizeURL` | 240 | Normalización de URL (7 reglas) |
| `generateID` | 276 | `md5(normalizeURL(url))` |
| `cleanHTML` | 284 | Delega en `textclean.Clean` |
| `extractImage` | 288 | 5 estrategias de extracción de imagen |
| `insertNoticias` | 469 | Envuelve a `insertNoticiasWithConflict` |
| `insertNoticiasWithConflict` | 486 | Lotes de 500, `ON CONFLICT DO NOTHING` |
| `isForeignKeyViolation` | 478 | Detecta SQLSTATE **23503** (feed borrado en vuelo) |
| `updateFeedStatus` | 545 | `fallos++`, desactiva a los 10, guarda `last_etag`/`last_modified` |
| `politeDelay` | 94 | 500 ms entre peticiones al mismo host |
| `initHTTPClient` | 136 | Timeout 60 s, pool de conexiones, UA |
| `initDB` | 167 | Pool lib/pq (25 máx / 20 idle / 5 min de vida) + espera 30×2 s |

**Configuración:** `RSS_MAX_WORKERS` (20), `RSS_MAX_FAILURES` (10), `RSS_POKE_INTERVAL_MIN` (60),
`RSS_FEED_TIMEOUT` (60), `RSS_HOST_DELAY_MS` (500), `TARGET_LANGS` (`es`).

**Tests:** `main_test.go` (73 L) — 7 casos de `normalizeURL` + equivalencia de id entre 3 formas de URL.
`textclean/textclean_test.go` (456 L) — 27 tests + `FuzzClean` con 2 semillas.

**Paridad obligatoria:** `diff -r backend/internal/textclean rss-ingestor-go/textclean` → **exit 0**
(verificado byte a byte con `md5sum`). `make parity` lo vigila; **no hay CI que lo ejecute**.

---

## 4. Workers Python

### 4.1 `workers/translation_scheduler.py` (267 L)

| Función | Línea | Responsabilidad |
|---|---|---|
| `main` | 214 | Bucle de 30 s: ruta fast + ruta catch-up + `requeue_incomplete` |
| `get_active_langs` | 95 | `idiomas_disponibles WHERE activo = TRUE` (fallback a `TARGET_LANGS`) |
| `create_translation_jobs` | 109 | El SQL de inserción (anti-join + ventana temporal + `ON CONFLICT`) |
| `requeue_incomplete` | 176 | Devuelve a `pending` las filas `done` con campos traducidos nulos |

Config: `TARGET_LANGS` (`es,en`), `USE_DB_LANGS` (`true`), `SCHEDULER_BATCH` (2000),
`SCHEDULER_SLEEP` (30), `SCHEDULER_LOOKBACK_MINUTES` (60), `SCHEDULER_CATCHUP_INTERVAL_MIN` (360),
`SCHEDULER_CATCHUP_LOOKBACK_HOURS` (168), `SCHEDULER_CATCHUP_BATCH` (20000).

### 4.2 `workers/langdetect_worker.py` (114 L)

`detect_language` (38), `main` (47). Selecciona `lang IS NULL OR TRIM(lang)=''`, `ORDER BY fecha DESC
LIMIT 1000`; **reordena por `id` ascendente** (línea 65) para evitar el deadlock 40P01 con `topics`;
UN UPDATE por fila; un COMMIT por lote; duerme 60 s en vacío, 1 s tras trabajar.
Config: `LANG_DETECT_BATCH` (1000), `LANG_DETECT_SLEEP` (60).

> ⚠️ **No fija `DetectorFactory.seed = 0`** (a diferencia del traductor) → la detección no es
> determinista entre procesos.

### 4.3 `workers/ner_worker.py` (555 L)

| Elemento | Línea | Detalle |
|---|---|---|
| `NER_MODELS` | 50 | `es → es_core_news_lg`, `en → en_core_web_lg` |
| `load_nlp_models` | 396 | `spacy.load(..., disable=["lemmatizer","textcat"])`; si **ningún** modelo carga → el worker sale |
| `clean_tag_text` | 250 | BeautifulSoup + regex de basura HTML + colapso de espacios + reglas de forma |
| `is_blacklisted` | 138 | Blocklist de BD (refrescada cada 10 min) + `entity_config.json` + regex numérica |
| `extract_entities_and_topics` | 348 | `doc.ents` → `persona/organizacion/lugar/tema`; `noun_chunks` → temas (≤15, ≥4 chars, ≤6 palabras) |
| `get_canonical_name` | 101 | Aplica `entity_config.json` (`REVERSE_SYNONYMS`) |
| `main` | 443 | Anti-join con `tags_noticia`; COMMIT por noticia; centinela `_none_` |

Config: `NER_BATCH` (64), `NER_LANG` (`es`, ⚠️ **leída pero nunca usada** — el modelo se elige por `lang_to`).

### 4.4 `workers/embeddings_worker.py` (495 L)

| Elemento | Línea | Detalle |
|---|---|---|
| `EMB_MODEL` | 39 | `paraphrase-multilingual-MiniLM-L12-v2` (384 dims) |
| `ensure_schema` | 68 | ⚠️ declara `traduccion_id INT NOT NULL` (contradice la migración 60) |
| `fetch_batch_pending` | 101 | Traducciones `done` sin embedding |
| `fetch_batch_pending_original` | 130 | Originales en español con **cursor `created_at`** |
| `upsert_embeddings` | 206 | `execute_values` + 2ª pasada con 1 UPDATE por fila |
| `upsert_embeddings_original` | 263 | `traduccion_id = NULL`, `noticia_id` informado, `ON CONFLICT` parcial |
| API Flask | 353 | `GET /health`, `POST /api/embeddings` (máx 100 textos) en `:6000` |

Config: `EMB_BATCH` (128), `EMB_LIMIT` (1000), `EMB_LANGS` (`es`), `DEVICE` (`auto`), `SLEEP_IDLE` (5).
⚠️ `.env` define `EMB_DEVICE` pero el worker lee `DEVICE` → **configuración muerta**.

### 4.5 `translator/workers/ctranslator_worker.py` (745 L)

| Elemento | Línea | Detalle |
|---|---|---|
| `UNIVERSAL_MODEL` | 56 | `facebook/nllb-200-distilled-600M` |
| `ensure_model` / `convert_model` | 230 / 249 | Carga desde `CT2_MODEL_PATH`; si falta `model.bin`, convierte con `ct2-transformers-converter` (timeout 3.600 s) |
| `fetch_pending_translations` | 667 | `FOR UPDATE SKIP LOCKED`, `LIMIT 128`, `ORDER BY n.fecha DESC` |
| `calculate_confidence` | 155 | Ratio de longitud + penalización de bigramas + contenido |
| `split_into_chunks` | 350 | 2.000 caracteres por frontera de frase |
| `process_group` | 456 | Traduce por grupo de idioma; `retry_count < 3` → `pending`, ≥3 → `error` |
| Caché Redis | 85 | `tr:{from}:{to}:{md5}`, TTL 30 días, `MGET` en pipeline |
| `translation_stats` | 702 | `INSERT` de métricas por lote |

Config: `MAX_SEQ_PER_CALL` (**8** en compose; 32 en el código), `MAX_SRC_TOKENS` (2048),
`MAX_NEW_TOKENS` (2048), `MAX_BODY_CHARS` (80.000), `BODY_CHARS_CHUNK` (2000),
`CT2_INTRA_THREADS` (2), `CT2_INTER_THREADS` (1), `CT2_COMPUTE_TYPE` (`int8`), `CT2_DEVICE` (`cpu`).

### 4.6 `remote-worker/worker.py` (409 L)

Cliente WebSocket (`websocket-client`). URL `ws://host:8080/ws/worker?api_key=…`.
Mensajes: `register`, `result` (con reintento ×3), `pong`. Reconexión con backoff 5 s → 60 s.
Mismo modelo NLLB en CTranslate2 int8; `MAX_SEQ_PER_CALL` default 8.
**No está desplegado en esta máquina.** 1 worker registrado (`portatil`) con `status='online'` pero
`last_seen` del 2026-10-09 → **falso positivo** en el panel de administración.

### 4.7 `workers/tts/` (2.600 L) y `workers/tts_worker.py` (55 L)

| Módulo | L | Responsabilidad |
|---|---|---|
| `config.py` | 205 | `TTSConfig.from_env()` — 30+ variables |
| `schema.py` | 157 | DDL idempotente de las 6 tablas `tts_*` (24 sentencias) |
| `script.py` | 557 | Normalización del texto a locutar: `int_to_es`, abreviaturas, fechas, porcentajes, moneda, unidades, ordinales, `split_sentences`, `summarize_script` |
| `engine.py` | 899 | `VoiceEngine` (ABC) + `MeloTTSEngine`, `OpenVoiceEngine`, `ChatterboxEngine`; `probe_audio`; `ENGINE_CLASSES`; `build_engines`; `resolve_engine` |
| `voices.py` | 208 | `slugify`, `safe_join` (anti path traversal), `analyze_sample`, `store_sample`, `delete_sample`, `sha256_file`, `copy_builtin_sample` |
| `db.py` | 863 | Acceso a Postgres: voces, guiones, trabajos, audios, boletines, watchdog |
| `queue.py` | 88 | `TaskQueue` — lista Redis `tts:queue` con fallback a Postgres |
| `service.py` | 911 | `TTSService`: arranque, precarga del motor, bucle del worker, watchdog, procesado, ensamblado, API Flask |
| `bulletin.py` | 170 | `concatenate_wavs` — concatenación por bloques con remuestreo y silencios |

`workers/tts_worker.py` (55 L): construye la config, arranca el servicio, instala SIGTERM/SIGINT y
sirve Flask en `:7000` con `threaded=True`.

---

## 5. Frontend (`frontend/src/`)

- **Stack**: React 18, TypeScript, Vite 5, Tailwind 3, `@tanstack/react-query` v5, `axios`,
  `react-router-dom` v6, `lucide-react`, `date-fns`. **Sin** Redux/Zustand.
- **21 páginas, 15 enrutadas**.

### 5.1 Páginas

| Página | L | Ruta | Propósito | Test |
|---|---|---|---|---|
| `Overview.tsx` | 323 | `/`, `/overview` | KPIs globales + actividad 24 h | — |
| `GlobalMonitor.tsx` | 364 | `/map` | Mapa SVG por país | — |
| `Search.tsx` | 278 | `/search` | Explorador (texto/semántico) | — |
| `Stats.tsx` | 376 | `/stats` | Throughput de traducción, idiomas, actividad horaria | — |
| `Populares.tsx` | 707 | `/populares` | Explorador de entidades + `EntityGraph` | — |
| `Analisis.tsx` | 186 | `/analisis` | Evolución temporal de un tema | — |
| `Alertas.tsx` | 419 | `/alertas` | Centro de alertas + drill-down por `periodo` | ✅ 4 tests |
| **`Noticiario.tsx`** | **1.517** | `/noticiario` | **Constructor del informativo de audio** | ✅ 14 tests |
| `Feeds.tsx` | 596 | `/feeds` | CRUD de fuentes, import/export, borrado masivo | — |
| `News.tsx` | 171 | `/news/:id` | Detalle de artículo | — |
| `Favorites.tsx` | 778 | `/favorites` | Favoritos, listas, búsquedas guardadas | — |
| `Account.tsx` | 159 | `/account` | Perfil y logout | — |
| `Login.tsx` | 129 | `/login` | Login/registro | — |
| `AdminWorkers.tsx` | 482 | `/admin/workers` | Observabilidad de workers y workers remotos | — |
| `AdminUsers.tsx` | 150 | `/admin/users` | Usuarios y roles | — |
| `AdminSettings.tsx` | 231 | `/admin/settings` | Reset, restore, export, info | — |
| `AdminAliases.tsx` | 255 | `/admin/aliases` | Alias de entidades | — |
| `WelcomeWizard.tsx` | 352 | (condicional) | Primer arranque | — |
| `Home.tsx` | 431 | — | ❌ **no enrutada** | — |
| `LoginPage.tsx` | 211 | — | ❌ **no enrutada** (usaría `useApp()` → lanzaría) | — |
| `OnboardingPage.tsx` | 230 | — | ❌ **no enrutada** | — |
| `SettingsPage.tsx` | 324 | — | ❌ **no enrutada** | — |

### 5.2 Servicios

| Fichero | L | Contenido |
|---|---|---|
| `services/api.ts` | 776 | Cliente axios principal. `API_BASE = import.meta.env.VITE_API_URL \|\| '/api'`. Interceptor de `Authorization` y de 401. ~60 métodos tipados. Todos los tipos TS viven aquí (no hay `types/`) |
| `services/apiClient.ts` | 440 | ❌ Cliente **paralelo e inactivo** (modo cloud/local, `DEFAULT_CLOUD_URL = https://api.rss2app.com`). Sin métodos TTS. Sólo lo usa `contexts/AppContext.tsx`, que **nunca se monta** |

### 5.3 Hooks

| Hook | L | Nota |
|---|---|---|
| `useApiFavorites.ts` | 290 | Favoritos, listas, sugerencias, búsquedas guardadas (API real) |
| `useSavedLists.ts` | 129 | ⚠️ lists en `localStorage` (legado) |
| `useSavedSearches.ts` | 78 | ⚠️ búsquedas guardadas en `localStorage` (legado) |
| `useSearchHistory.ts` | 46 | `localStorage['rss2_search_history']` (máx 10) |

### 5.4 Componentes

`IntelligenceLayout.tsx` (391 L, en uso), `Layout.tsx` (276 L, ❌ huérfano),
`ProtectedRoute.tsx`, `ErrorBoundary.tsx`, `CreateListModal.tsx`, `SaveSearchModal.tsx`,
`LineChart.tsx`, y en `ui/`: `NewsCard`, `CompactNewsCard`, `ExpandedNewsModal`,
`SuggestedNewsSidebar`, `TagCloud`, `WikiTooltip`, `WorldMap`, `EntityGraph` (482 L),
`ShareButtons`, `ListShareButtons`, `Skeleton`, `EmptyState`.

> **No existe ningún componente de TTS, ni de boletín, ni de editor, ni de reproductor de audio.**
> Todo vive inline en `Noticiario.tsx`; el reproductor es un `<audio controls>` nativo.

### 5.5 Tests

`vitest.config.ts` (jsdom, `setupFiles: ./src/test/setup.ts`, cobertura v8).
**2 ficheros, 18 casos**: `Noticiario.test.tsx` (14) y `Alertas.test.tsx` (4).
⚠️ `npm test` **no se pudo ejecutar**: Node.js no está instalado en esta máquina.

---

## 6. Scripts y herramientas de administración

| Fichero | L | Función |
|---|---|---|
| `Makefile` | 164 | 25+ targets: `all`, `deps`, `build`, `test`, `parity`, `sanitize-*`, `entities-scan-*`, `alerts-prune-*`, `dedupe-noticias`, `check-schema`, `tts-test`, `tts-test-db` |
| `scripts/check-schema-drift.py` | ~180 | Compara `init-db/` con la BD viva (tablas, columnas, índices, triggers, funciones). Lo que habría detectado las migraciones 51/52 ausentes |
| `scripts/prepare_tts_models.sh` | 242 | Descarga pesos de OpenVoice V2 y Chatterbox; escribe los manifiestos de voces; `--verify` sólo comprueba |
| `scripts/setup_tts_test_db.sh` | 84 | Crea `rss_tts_test` y aplica `migrations/add_tts.sql` |
| `docker-entrypoint-db.sh` | — | Entrypoint de Postgres |
| `backup.sh`, `pre-deploy.sh`, `verify_security.sh`, `migrate_to_secure.sh`, `generate_secure_credentials.sh` | — | Utilidades de despliegue y seguridad |
| `alias.sh` | — | Alias de comandos |
| `init-db/` | 66 ficheros | Esquema + migraciones 00→99 |
| `migrations/` | 9 ficheros | Migraciones para bases ya existentes (incluye `add_tts.sql`) |
| `init-replica/` | — | Configuración de réplica |

---

## 7. Documentación preexistente (no duplicada en `docs/context/`)

| Documento | Fecha | Estado |
|---|---|---|
| `README.md` | 2026-10-24 | Instalación y descripción general. **El diagrama ASCII dice "Cada 8 min" para el ingestor, pero compose usa 60 min** |
| `AGENTS.md` | 2026-10-26 | **Autoritativo**: constitución, quirks operativos, procedimientos de reset |
| `explicacion.md` | 2026-10-04 | Divulgación del proyecto (no técnico) |
| `diagrama.md` | 2026-10-10 | Diagrama de flujo |
| `docs/TTS.md` | 2026-10-24 | Manual de operación del TTS |
| `docs/REVISION_CODIGO.md` | 2026-10-12 | Revisión de código |
| `docs/AUDIT_WORKERS.md` | 2026-10-12 | Auditoría de workers |
| `docs/AUDIT_SUBSCRIPTION.md` | 2026-10-12 | Auditoría de suscripciones |
| `docs/ANDROID_*.md` | 2026-10-12 | Integración Android |
| `docs/MONETIZATION_PLAN.md` | 2026-10-12 | Plan de monetización |
| `docs/REMOTE-WORKER-INTEGRATION.md` | 2026-10-02 | Guía del worker remoto |
| `specs/AUDIT_STATUS.md` | 2026-10-12 | ⚠️ **DESACTUALIZADO**: da 1,28 M noticias y 194 K traducciones; el corpus se truncó el 2026-10-25 |
| `specs/` (13 carpetas) | 2026-09→10 | Especificaciones por área: `00-architecture` … `10-tts` |

---

## 8. Superficie de la API (resumen)

| Grupo | Rutas | Auth |
|---|---|---|
| raíz | `/health`, `/swagger/*any` | pública |
| `auth` | `login`, `register`, `check-first-user` | pública (`/me` exige JWT) |
| `news` | `GET /`, `GET /:id`, `DELETE /:id` | lectura pública; borrado con JWT |
| `feeds` | 13 rutas (CRUD, import/export, `traducido`, bulk-delete, toggle, reactivate) | lectura pública; resto con JWT |
| `search` | `GET /`, `suggestions`, `POST /searchlog` | búsqueda pública |
| `entities` | `entities`, `entities/news`, `entities/mentions`, `entities/graph`, `last-names` | pública |
| `events` | `events`, `events/breaking`, `events/:id` | pública |
| `alerts` | `alerts`, `:id/read`, `:id/dismiss`, `read-all` | **JWT** |
| `stats` | `stats`, `stats/ratelimit`, `categories`, `countries`, `idiomas`, `share/:id` | pública |
| `favorites` / `lists` / `saved-searches` | 4 + 11 + 5 rutas | **JWT** |
| `admin` | 27 rutas (alias, stats, backup, restore, reset-db, usuarios, workers, remotos, scan) | **JWT + admin** |
| `app` | 5 rutas (suscripción, uso, límites) | **JWT** |
| `tts` | 32 rutas (status, voices, speakers, scripts, jobs, audios, bulletins, trends) | **JWT** (+ admin en 5) |
| WebSocket | `/ws/worker` | **API key de worker** |

Total: **~115 rutas**.
