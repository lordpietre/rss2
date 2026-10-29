# RSS2 — Base de Datos

> Análisis de PostgreSQL 16 + pgvector y Redis 7.
> Mediciones tomadas el **2026-10-26 ~18:40 UTC** contra la instancia de producción.
> Fuentes: `[DB]` = consulta directa; `[CÓDIGO]` = lectura de ficheros; `[DOC]` = `AGENTS.md` / `init-db/`.

---

## 1. Visión general

| Aspecto | Valor | Fuente |
|---|---|---|
| Motor | PostgreSQL 16.15 (Debian) sobre `pgvector/pgvector:pg16` | `[DB]` |
| Tamaño de la base | **3.592 MB** | `[DB]` |
| Tablas en el esquema `public` | **50** (de las cuales 41 tienen datos) | `[DB]` |
| Índices | **212** | `[DB]` |
| Claves foráneas | **61** | `[DB]` |
| Extensiones | `vector` (pgvector) | `[DB]` |
| `shared_buffers` | 1 GB | `[CÓDIGO: docker-compose.yml:21]` |
| `effective_cache_size` | 2 GB | `[CÓDIGO]` |
| `work_mem` | 64 MB | `[CÓDIGO]` |
| `maintenance_work_mem` | 512 MB | `[CÓDIGO]` |
| `max_connections` | 200 | `[CÓDIGO]` |
| `wal_level` | `replica` | `[CÓDIGO]` |
| `autovacuum` | on, `vacuum_scale_factor=0.2`, `analyze_scale_factor=0.1` | `[DB]` |
| `shm_size` | **256 MB** (obligatorio para la agregación hash paralela) | `[DOC: AGENTS.md]` |

---

## 2. Volumen de datos

### 2.1 Tablas principales (contenido real)

| Tabla | Filas | Tamaño total | Heap | Índices | Escritor principal |
|---|---|---|---|---|---|
| `noticias` | **814.290** | **2.412 MB** | 969 MB | 694 MB | `ingestor` |
| `traduccion_embeddings` | 58.160 | 404 MB | 117 MB | 131 MB | `embeddings` |
| `traducciones` | 453.724 | 279 MB | 74 MB | 202 MB | `scheduler` (crece), `translator` (actualiza) |
| `tags_noticia` | 330.282 | 116 MB | 27 MB | 88 MB | `ner` |
| `event_clusters` | 21.886 | 97 MB | 56 MB | 41 MB | `event-clustering` |
| `traducciones_target` | 358.226 | 82 MB | 26 MB | 55 MB | `scheduler` |
| `entity_stats` | 150.352 | 72 MB | 38 MB | 34 MB | `backend` (refresh) |
| `tags` | 165.665 | 71 MB | 20 MB | 51 MB | `ner` |
| `news_topics` | 183.023 | 41 MB | 16 MB | 24 MB | `topics` |
| `feeds` | 33.528 | 17 MB | 9,9 MB | 7,8 MB | API / `discovery` |
| `noticia_clusters` | 31.319 | 9,9 MB | 3,6 MB | 6,3 MB | `event-clustering` |
| `entity_cooccurrences` | 36.675 | 9,1 MB | 3,7 MB | 5,3 MB | `cooccurrences` |
| `entity_blocklist` | — | 3,6 MB | 2,3 MB | 1,2 MB | `entityscan` |
| `feed_stats` | 22.283 | 3,2 MB | 2,0 MB | 1,2 MB | trigger |
| `alertas` | 85 | — | — | — | `backend` (scanner) |
| `tts_voices` | 23 | — | — | — | `tts` |
| `tts_scripts` / `tts_jobs` / `tts_audios` | 6 / 6 / 6 | — | — | — | Go (editorial) + `tts` |
| `tts_bulletins` / `tts_bulletin_items` | 1 / 6 | — | — | — | Go + `tts` |
| `related_noticias` | **0** | — | — | — | `related` (no ha producido nada aún) |
| `topics` | **0** | — | — | — | Semilla vacía; `topics` clasifica sin filas de referencia |

**Concentración:** `noticias` es el **53 %** de la base (2,4 de 3,6 GB) y `traduccion_embeddings`
el 11 % (404 MB para 58 K vectores de 384 dims → ~7 KB por fila, con HNSW ocupando 131 MB).

### 2.2 Distribución temporal de la ingesta

| Día | Noticias creadas |
|---|---|
| 2026-10-25 | 635.681 |
| 2026-10-26 (parcial) | 174.727 |

El corpus se truncó el 2026-10-25 (ver `AGENTS.md` → "Resetting the news corpus"), por lo que
**sólo hay 2 días de histórico**. No hay datos para medir seasonalidad ni para validar
retenciones.

### 2.3 Distribución de idiomas (`noticias.lang`)

| `lang` | Filas | % |
|---|---|---|
| `NULL` | 294.796 | 36,2 % |
| `en` | 269.963 | 33,2 % |
| `fr` | 49.101 | 6,0 % |
| `es` | 33.160 | 4,1 % |
| `pt` | 22.296 | 2,7 % |
| `de` | 20.362 | 2,5 % |
| `ru` | 19.850 | 2,4 % |
| `ja` | 18.380 | 2,3 % |
| `ko` | 8.687 | 1,1 % |
| `nl` | 7.251 | 0,9 % |
| `it` | 6.816 | 0,8 % |
| `tr` | 6.630 | 0,8 % |

> El 36 % sin idioma es la **cola de `langdetect`**, no un fallo: el worker procesa 1.000 filas por
> ciclo y la ingesta las supera con creces.

### 2.4 Estado de las traducciones

| `status` | Filas | % |
|---|---|---|
| `pending` | **427.200** | 94,2 % |
| `done` | 26.524 | 5,8 % |

**Traducciones reales disponibles: 26.524 de 814.290 noticias (3,3 %).**

---

## 3. Modelo de datos

### 3.1 Núcleo de noticias

```
categorias ──┐                     paises ──┐                     feeds ─┐
             ├──────► noticias ◄──────────────┤                          │
             │         │  │                   │                          │
             │         │  │  id = md5(url normalizada)                    │
             │         │  │  feed_id ────────────────────────────────────┘
             │         │  │  lang, fecha, created_at, updated_at
             │         │  │  titulo, resumen, contenido, titulo_raw, resumen_raw
             │         │  │  tsv, search_vector_es, topics_processed
             │         │  │
             │         │  ├──► traducciones (UNIQUE noticia_id, lang_to)
             │         │  │        titulo_trad, resumen_trad, contenido_trad
             │         │  │        status, confidence, retry_count, worker_id, locked_at
             │         │  │
             │         │  ├──► traduccion_embeddings
             │         │  │        embedding_vector vector(384), model
             │         │  │        traduccion_id NULLABLE | noticia_id NULLABLE
             │         │  │
             │         │  ├──► tags_noticia ────► tags (valor, tipo)
             │         │  │
             │         │  ├──► news_topics ─────► topics
             │         │  ├──► related_noticias
             │         │  ├──► noticia_clusters ─► event_clusters
             │         │  └──► traducciones_target (pivote "traducción solicitada")
             │
             ├──► tts_scripts ─► tts_jobs ─► tts_audios
             └──► tts_bulletins ─► tts_bulletin_items
```

### 3.2 Claves primarias

| Tabla | PK | Notas |
|---|---|---|
| `noticias` | `id VARCHAR(32)` | md5 de la URL normalizada |
| `feeds` | `id` (serial) | |
| `traducciones` | `id` | `UNIQUE (noticia_id, lang_to)` |
| `traduccion_embeddings` | `id` | `UNIQUE (traduccion_id, model)` **parcial**; `UNIQUE (noticia_id, model)` parcial `WHERE noticia_id IS NOT NULL` |
| `tags` | `id` | `UNIQUE (valor, tipo)` |
| `tags_noticia` | `id` | `UNIQUE (tag_id, noticia_id)` |
| `alertas` | `id` | `UNIQUE (valor, tipo, periodo)` — estado agregado |
| `tts_*` | `id` (serial/BIGSERIAL) | |
| `users` | `id` | `bcrypt` para contraseñas |

### 3.3 Claves foráneas (61) y su efecto

Las FK con `ON DELETE CASCADE` más importantes:

| FK | Cascada | Consecuencia |
|---|---|---|
| `noticias.feed_id → feeds.id` | CASCADE | **Borrar un feed borra todas sus noticias** y, en cascada, traducciones, tags, embeddings y clusters. Ver `KNOWN_ISSUES.md` / protección 428 |
| `traducciones.noticia_id → noticias.id` | CASCADE | — |
| `tags_noticia.traduccion_id → traducciones.id` | CASCADE | — |
| `traduccion_embeddings.traduccion_id → traducciones.id` | CASCADE | — |
| `tts_scripts.noticia_id → noticias.id` | CASCADE | **Borrar la noticia borra sus guiones** (y con ellos los trabajos y audios) |
| `tts_jobs.script_id → tts_scripts.id` | CASCADE | — |
| `tts_audios.job_id → tts_jobs.id` | CASCADE | — |

`traducciones.lang_to → idiomas_disponibles.codigo` y `traducciones.worker_id → remote_workers.id`
también son FK → **no se puede añadir un idioma destino ni un worker sin tocar antes la tabla de
referencia**.

### 3.4 Índices notables

**`noticias`** — 18 índices:

| Índice | Tipo | Uso |
|---|---|---|
| `noticias_pkey (id)` | btree | PK |
| `noticias_url_key (url)` | btree UNIQUE | Duplicados exactos de URL |
| `idx_noticias_created_at` | btree (created_at DESC) | Ruta *fast* del scheduler |
| `idx_noticias_updated_at` | btree (updated_at DESC) | Ruta *catch-up* del scheduler |
| `idx_noticias_fecha` / `idx_noticias_fecha_pais_categoria` | btree | Listados y filtros |
| `idx_noticias_lang` | btree **parcial** (`lang IS NOT NULL AND lang <> ''`) | Filtro de idioma |
| `idx_noticias_trad_elegibles` | btree **parcial** (`lang IS NOT NULL AND lang <> 'es'`) | Candidatas a traducción |
| `idx_noticias_topics_pending` | btree **parcial** (`topics_processed = false`) | Lote del worker `topics` |
| `idx_noticias_cleaned_null` | btree **parcial** (`cleaned_at IS NULL`) | Pendientes de `textclean` |
| `noticias_tsv_idx`, `noticias_search_vector_es_idx` | **GIN** | Búsqueda full-text |
| `idx_noticias_contenido` | **GIN** | `to_tsvector('spanish', contenido)` |
| `idx_noticias_categoria` / `_categoria_id` | btree | **Duplicados** |
| `idx_noticias_pais` / `_pais_id` | btree | **Duplicados** |

**`traducciones`** — 21 índices, incluidos **cinco índices parciales solapados** sobre `status`:
`idx_traducciones_status_partial_pending`, `_partial_done`, `idx_traducciones_status`,
`_status_lang`, `_pending_lookup`, `_pending_worker`, `_status_retry`. El índice
`idx_traducciones_pending_worker (lang_to, status, worker_id) WHERE status='pending' AND worker_id IS NULL`
es el que usa el traductor; `idx_traducciones_pending_lookup (status, lang_to, created_at) WHERE status='pending'`
lo solapa.

**`traduccion_embeddings`**:
`idx_tr_emb_vector_hnsw` — **HNSW** con `vector_cosine_ops` (`m=16`, `ef_construction=64`).
Es el índice que hace posible la búsqueda semántica.

**`tags_noticia`** — 7 índices, incluidos **tres pares solapados**:
`idx_tags_noticia_tag_id_include (tag_id)`, `idx_tags_noticia_tag (tag_id)`,
`idx_tags_noticia_noticia_id_tag_id (noticia_id, tag_id)` y el UNIQUE `(tag_id, noticia_id)`.

---

## 4. Consultas costosas y accesos

### 4.1 Consultas medidas como costosas

| Consulta | Plan / coste | Observación |
|---|---|---|
| `GetFeeds` (`handlers/feed.go:102-115`) | `COUNT(*)` sobre `feeds` + `LEFT JOIN categorias/paises/feed_stats` + `ORDER BY p.nombre, f.activo, f.fallos, c.nombre, f.nombre` | El `ORDER BY` sobre columnas de tablas unidas obliga a leer y ordenar **toda** la tabla de feeds + los catálogos. Con 33 K feeds y 212 páginas, es la consulta más cara de la API |
| `GetEntities` ruta lenta (`handlers/news.go:820+`) | `tags_noticia ⋈ tags ⋈ traducciones ⋈ noticias` + `mode() WITHIN GROUP` | Devuelve `total = 0` a propósito (documentado) para no pagar un `COUNT DISTINCT` carísimo |
| `RunAlertScan` (`handlers/alerts.go:273-286`) | CTE horaria sobre 30 días + `SUM/SUM` ponderado | **Se ejecuta cada 15 min** sobre `tags_noticia` (330 K filas) |
| `find_related_translations` (`cmd/related`) | Búsqueda HNSW + filtro por `traduccion_id` | Usa el índice vectorial |
| `SemanticSearch` (`services/ml.go:161-341`) | UNION de dos fuentes + `ORDER BY 1 - (v <=> $1)` | `LIMIT`/`OFFSET` al final; el vector va como literal |
| `getActiveFeeds` (`rss-ingestor-go/main.go:197`) | `WHERE activo = TRUE AND (fallos IS NULL OR fallos < 10) ORDER BY id` | **Sin índice** en `activo`/`fallos` → seq scan + sort de 33 K filas, una vez por ciclo de ingesta |

### 4.2 Lecturas secuenciales acumuladas (confirmado)

`pg_stat_user_tables` en el uptime actual del contenedor:

| Tabla | `seq_scan` | `seq_tup_read` | Filas |
|---|---|---|---|
| `feeds` | **4.615.698** | **81.173.258.205** | 33.528 |
| `paises` | 1.040.318 | 121.965.973 | 195 |
| `categorias` | 923.441 | 8.836.692 | 15 |

**81.173 millones de tuplas leídas** en la tabla `feeds` desde el último arranque de Postgres.
Una lectura media de 17.591 filas por scan → ~2,4 M de lecturas completas de la tabla.

**Estado en el momento de la medición:** el contador **no crecía** (delta 0 en una ventana de 20 s) y
el log de nginx no muestra peticiones a `/api/feeds` en los últimos 30 min → **la tormenta fue
histórica** (probablemente la sesión de frontend o el reset del corpus) y **no está activa ahora**.
No se ha identificado la causa exacta. `[HIPÓTESIS]`

`paises` y `categorias` con `n_live_tup = 0` indican que **nunca se han ejecutado VACUUM/ANALYZE**
sobre ellas (viven en el mismo grupo de lectura que `feeds`).

### 4.3 Operaciones de escritura intensivas

| Productor | Escrituras | Frecuencia | Modo |
|---|---|---|---|
| `ingestor` | `INSERT` en lotes de 500 filas × 11 columnas | ~1.500/min (pico) | `ON CONFLICT DO NOTHING` |
| `translator` | `UPDATE traducciones` por lote (128 filas) + `INSERT translation_stats` | ~31/min | `executemany` en una transacción |
| `langdetect` | `UPDATE noticias SET lang` por fila | 1.000 por ciclo | Un COMMIT por lote |
| `topics` | `INSERT news_topics` por lotes de 500 + `UPDATE noticias` masivo con `unnest` | cada 10 s | Ordena ids para evitar deadlock |
| `ner` | `INSERT tags` + `INSERT tags_noticia` por noticia | 64 por ciclo | COMMIT por noticia |
| `embeddings` | `execute_values` + **un UPDATE por fila** para `embedding_vector` | 128 por ciclo | 2 pasadas |
| `cooccurrences` | `TRUNCATE` + re-inserción completa | cada 24 h | 36.675 filas |
| `event-clustering` | `UPDATE event_clusters` por cluster | cada 15 min | Con CTE `cnt` |

**Punto de fricción:** `embeddings` hace **un UPDATE por fila** para escribir el vector pgvector
(`workers/embeddings_worker.py:245-259`). Con 128 filas por ciclo son 128 idas y vueltas a la BD
por lote. Podría resolverse con un `COPY` a una tabla temporal o con un `unnest` de arrays.

### 4.4 Concurrencia y bloqueos

| Mecanismo | Uso | Riesgo |
|---|---|---|
| `FOR UPDATE SKIP LOCKED` | `translator`, `tts` (Postgres fallback), `worker_ws` | Correcto, pero el `translator` hace `JOIN` **sin `OF t`** → **bloquea filas de `noticias` también** → contención con `langdetect` y `topics` |
| Orden de bloqueo ascendente por `id` | `langdetect` y `topics` | Ambos ordenan por `id` antes del UPDATE masivo para evitar el deadlock 40P01 |
| Early commit | `translator` (mismo idioma) | Compromete la transacción **antes** de marcar `locked_at` → ventana estrecha de doble reclamo |
| `CREATE INDEX` al arrancar | `related`, `event-clustering` | `ensureSchema` crea índices HNSW si no existen — **bloqueos largos** en tablas grandes |
| `TRUNCATE` | `cooccurrences` | `ACCESS EXCLUSIVE` bloqueante; Mitad de la tabla `noticia_clusters` se lee a la vez |

**Deadlocks conocidos y mitigados:** 40P01 entre `langdetect` y `topics` (resuelto ordenando por id).

### 4.5 Paginación

| Endpoint | Estrategia | Nota |
|---|---|---|
| `GET /api/news` | `LIMIT/OFFSET` + `per_page` clampado a 100 | `total` con `COUNT(*)` cuando hay filtros; `reltuples` cuando no |
| `GET /api/search` | `LIMIT/OFFSET`, `per_page` 20 en la UI | `total` con `COUNT(*)` y el mismo WHERE |
| `GET /api/feeds` | `LIMIT/OFFSET`, `per_page` ≤ 100 | `COUNT(*)` sin filtros |
| `GET /api/entities` | `LIMIT/OFFSET` | `total = 0` en la ruta lenta (a propósito) |
| `GET /api/alerts` | `LIMIT` clampado a [1, 500] | `total` con `COUNT(*)` |

**Riesgo:** `OFFSET` profundo degrada O(n). Con 814 K noticias, la página 400 yaOffset es
insostenible. No hay paginación por cursor (keyset) en ninguna lista.

---

## 5. Redis

### 5.1 Configuración y uso real

| Aspecto | Valor | Fuente |
|---|---|---|
| Imagen | `redis:7-alpine` | `[CÓDIGO]` |
| Persistencia | `--appendonly yes` (`./data/redis-data`) | `[CÓDIGO]` |
| Memoria máxima | **512 MB** | `[CÓDIGO]` |
| Política | **`allkeys-lru`** | `[CÓDIGO]` |
| Autenticación | `--requirepass ${REDIS_PASSWORD}` | `[CÓDIGO]` |
| Claves vivas | **413.732** | `[DB]` |
| Memoria usada | **137 MB** (27 % del máximo; pico 143,9 MB) | `[DB]` |

### 5.2 Qué almacena

| Espacio de claves | Contenido | TTL | Productor |
|---|---|---|---|
| `tr:{from}:{to}:{md5(texto)}` | **Caché de traducción** (el 99 % de las claves) | 30 días | `translator` |
| `search:<q>:<lang>:<page>:<perPage>[:filtros]` | Respuesta de `/api/search` | 5 min | `backend` |
| `feedlist:<filtros>:<page>:<perPage>` | Listado de feeds | 1 h | `backend` |
| `stats`, `categories`, `countries` | Catálogos y estadísticas | 1 h / 4 h | `backend` |
| `wiki:summary:<lang>:<título>` | Resumen de Wikipedia | 48 h | `wiki` |
| `tts:queue` | **Lista** de trabajos TTS (LPUSH/RPOP) | sin TTL | `backend` → `tts` |

### 5.3 Familias de claves muertas (verificado por grep de todos los call sites)

`news:*`, `feed:<id>`, `entities:*`, `statstop:*` se **invalidan** pero **nunca se escriben**.
`cache.NewsKey`, `cache.EntityKey`, `cache.StatsTopKey` no tienen ningún escritor.
`InvalidateCategories`, `InvalidateCountries` y `InvalidateEntities` nunca se llaman.

### 5.4 Comportamiento ante caída de Redis

**Fail-open, confirmado:**

- `cache.Connect` devuelve error si el ping falla, **pero `Client` ya está asignado**; el backend
  hace `log.Warn` y **sigue arrancando**.
- Todos los sitios de lectura usan `if cached, err := cache.Get(...); err == nil && cached != ""`
  → en error se cae a la base de datos.
- Todos los sitios de escritura **descartan el error** de `cache.Set`.
- `PushTTSJob` es *fire-and-forget*: el trabajo ya está confirmado en `tts_jobs`; el worker lo
  recupera de Postgres. El comentario del código lo dice explícitamente.
- El worker TTS captura el fallo de conexión a Redis y usa Postgres (`FOR UPDATE SKIP LOCKED`).

**Estado observado:** el servicio TTS reporta `"queue": {"pending": 0, "redis": false}` → **la cola
Redis del TTS no está conectada** y todo el trabajo se recupera de Postgres (funcionalmente
correcto, con algo más de latencia). `[DB] GET /health del servicio tts]`

**Riesgo residual:** si `REDIS_URL` fuera **malformado** (no simplemente inalcanzable),
`redis.ParseURL` falla *antes* de asignar `Client` → `cache.Client` queda `nil` → los endpoints con
caché harían *nil-pointer dereference*. No es alcanzable con el valor de compose actual. `[CÓDIGO]`

### 5.5 Requisitos de persistencia

| Dato | ¿Necesita persistencia? | Razón |
|---|---|---|
| Caché de traducción | **No** | Regenerable; el coste es re-traducir |
| Caché de API | **No** | Regenerable |
| Resúmenes de Wikipedia | **No** | Regenerable |
| **Cola `tts:queue`** | **No** (por diseño) | Postgres es la fuente de verdad |
| **AOF** | Sí, por robustez | `appendonly yes` está activo; con `allkeys-lru` Redis puede vaciar claves, y AOF permite recuperar el estado si el proceso se reinicia |

**Si Redis se reiniciara:** se perdería la caché (recoverable) y la cola TTS en vuelo (recoverable
desde Postgres). **Ninguna pérdida de datos.**

---

## 6. Cuellos de botella y oportunidades

### 6.1 Confirmados (medidos)

| # | Cuello de botella | Evidencia | Impacto |
|---|---|---|---|
| B1 | `noticias` ocupa 2,4 GB sin particionado | `[DB]` | VACUUM, backups y `COUNT(*)` crecen de forma lineal con el histórico |
| B2 | `feeds` con 81 M tuplas leídas por seq scan | `[DB] pg_stat_user_tables` | I/O desperdiciado (histórico, no activo) |
| B3 | `getActiveFeeds()` sin índice ni paginación | `[CÓDIGO: rss-ingestor-go/main.go:197]` | Seq scan de 33 K filas por ciclo de ingesta |
| B4 | `GetFeeds` ordena por columnas de tablas unidas | `[CÓDIGO: handlers/feed.go:113]` | La consulta más cara de la API |
| B5 | `embeddings` hace 1 UPDATE por fila para el vector | `[CÓDIGO: workers/embeddings_worker.py:245-259]` | 128 idas y vueltas por lote |
| B6 | El traductor bloquea filas de `noticias` (JOIN sin `OF t`) | `[CÓDIGO: ctranslator_worker.py:680-693]` | Contención con `langdetect`/`topics` |
| B7 | `embeddings` reinicia el cursor a `None` cuando no hay trabajo | `[CÓDIGO: workers/embeddings_worker.py:471]` | Re-escaneo de `noticias` cada 5 s en reposo |
| B8 | Caché de traducción: 413 K claves con TTL 30 días y `allkeys-lru` | `[DB]` | Con `maxmemory 512mb`, la política puede **evictar claves de la cola TTS** o entradas de la caché de traducción |
| B9 | `noticias.contenido` casi siempre vacío (668 de 814.290) | `[DB]` | El `scraper` no está rellenando; el `min_chars` del noticiario depende de `resumen` |
| B10 | 817.301 filas con `cleaned_at IS NULL` | `[DB]` | El corpus mayoritario no ha pasado por `textclean` |

### 6.2 Sospechas (no confirmadas)

| # | Sospecha | Cómo confirmarla |
|---|---|---|
| S1 | La tormenta de seq scans en `feeds` la causó el reset del corpus o una sesión del frontend | Revisar el log de Postgres con `log_min_duration_statement` durante un periodo comparable |
| S2 | `autovacuum` no mantiene al día `noticias` (factor 0,2 sobre 814 K filas → umbral de ~163 K filas por pasada) | `pg_stat_user_tables.last_autovacuum` y `n_dead_tup` |
| S3 | El `DELETE` (no `TRUNCATE`) de `POST /api/admin/reset-db` no dispara el trigger de `feed_stats` | Revisar la definición del trigger `trigger_update_feed_stats` |
| S4 | `cmd/scraper` genera ids `md5(url)` **sin** `normalizeURL` → puede colisionar con el id del ingestor o duplicar | Comparar `SELECT count(*), count(DISTINCT url) FROM noticias WHERE contenido IS NOT NULL` |
| S5 | `noticias.tsv` y `noticias.search_vector_es` son índices GIN sobre columnas calculadas → coste de escritura alto en el ingestor | Medir el tiempo del `INSERT` en lotes con y sin esos índices |

### 6.3 Oportunidades de optimización (priorizadas, **no aplicadas**)

1. **Particionar `noticias` por rango de `created_at`** (mensual). Aporta: `VACUUM` y `DROP` de
   histórico por tramo, y `COUNT(*)` por rango más barato. Coste: la PK y los FKWould have que
   incluir la clave de partición, lo que obliga a revisar `noticias.id` (md5 de URL → **no
   monotónico**, por lo que **no es viable particionar por `created_at` sin cambiar la PK**).
   → **Conclusión: el particionado por fecha no es aplicable al diseño actual del id.**
2. **Índice parcial en `feeds (activo)` o en `(activo, fallos)`** → elimina el seq scan del ingestor (B3).
3. **`INSERT ... SELECT` o `COPY` para `embedding_vector`** → elimina N idas y vueltas (B5).
4. **Revisar los 5 índices solapados de `traducciones` y los 3 de `tags_noticia`** → escribir menos
   = insert más rápido. `[CÓDIGO]`
5. **`OFFSET` → keyset pagination** en las listas grandes (`/api/news`, `/api/search`).
6. **Fijar `maxmemory-policy` a `volatile-lru`** en vez de `allkeys-lru`, para que la política nunca
   evicte la cola `tts:queue` (que no tiene TTL). Alternativa: TTL a la cola.
7. **Rebuild de `feed_stats`** tras un reset (ya documentado en `AGENTS.md`).

---

## 7. Retención y eliminación

| Mecanismo | Estado |
|---|---|
| Retención automática de noticias | **No existe.** `noticias` crece sin límite |
| Retención de traducciones | **No existe** |
| Retención de embeddings | **No existe** |
| Retención de alertas | **No existe** (85 filas) |
| Retención de `tags` | **No existe** (165.665 filas) |
| `TRUNCATE` de co-ocurrencias | Sí, pero sólo se **recalcula** (no es retención) |
| Borrado por feed | Sí: `DELETE FROM feeds` en cascada (con confirmación 428) |
| Limpieza de texto | `make sanitize-restore` lee de `*_raw` |
| Deduplicación | `make dedupe-noticias` (migración 63), idempotente |
| **Vacuum de la base** | Automático (autovacuum), sin `VACUUM FULL` programados |

**Ausencia crítica:** no hay política de retención. Al ritmo actual (635 K/día en el pico), la base
crece ~1 GB/día sólo en `noticias`. A 30 días el tamaño de la base sería ~30 GB, y a 90 días ~90 GB,
sin que ninguna consulta mejore (el histórico no se consulta: el `desde` por defecto es `7d`).

---

## 8. Procedimientos operativos verificados

| Procedimiento | Comando | Nota |
|---|---|---|
| Reiniciar la base sin perder datos | `docker compose exec -T db psql -f /docker-entrypoint-initdb.d/63-dedupe-noticias.sql` | Idempotente |
| Rebuild de `feed_stats` | `TRUNCATE feed_stats; INSERT INTO feed_stats SELECT fuente_nombre, count(*) FROM noticias WHERE fuente_nombre <> '' GROUP BY 1;` | Necesario tras un reset (el trigger no dispara en `TRUNCATE`) |
| Limpiar la caché tras un cambio de formato | `docker compose exec redis redis-cli FLUSHDB` | `allkeys-lru` + 413 K claves: `FLUSHDB` es el mecanismo real |
| Backup | `pg_dump -Fc -Z 6` a `data/backup/` **y verificar la restauración** | `GET /api/admin/backup` hace `pg_dump` en streaming |
| Comprobar deriva de esquema | `make check-schema` (`scripts/check-schema-drift.py`) | Compara `init-db/` con la BD viva |

> **Advertencia:** `init-db/*.sql` **sólo se ejecuta en la primera inicialización** de `data/pgdata`.
> Las migraciones añadidas después nunca se aplican solas. Antes de reconstruir cualquier imagen hay
> que comprobar que faltan columnas/tablas (p. ej. `traducciones.confidence` / `retry_count` de la
> migración 51 ya faltaron una vez). `[DOC: AGENTS.md]`
