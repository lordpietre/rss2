# Plan de Migración de RSS2 a Aplicación Multiplataforma

**Versión:** 1.0
**Fecha:** 2026-08-14
**Estado:** Propuesta de arquitectura y plan de migración

---

## Tabla de Contenidos

1. [Resumen Ejecutivo](#1-resumen-ejecutivo)
2. [Análisis Profundo del Código Actual](#2-análisis-profundo-del-código-actual)
3. [Objetivos y Requisitos de la Nueva Aplicación](#3-objetivos-y-requisitos-de-la-nueva-aplicación)
4. [Arquitectura Objetivo (Alternativas)](#4-arquitectura-objetivo-alternativas)
5. [Selección Tecnológica por Plataforma](#5-selección-tecnológica-por-plataforma)
6. [Estrategia de Datos y Sincronización](#6-estrategia-de-datos-y-sincronización)
7. [Estrategia de IA/ML en el Dispositivo](#7-estrategia-de-iaml-en-el-dispositivo)
8. [Núcleo Embebido (Embedded Backend)](#8-núcleo-embebido-embedded-backend)
9. [Rediseño de la API para Clientes Nativos](#9-rediseño-de-la-api-para-clientes-nativos)
10. [Migración del Frontend Paso a Paso](#10-migración-del-frontend-paso-a-paso)
11. [Empaquetado y Distribución por Plataforma](#11-empaquetado-y-distribución-por-plataforma)
12. [Infraestructura de Build y CI/CD](#12-infraestructura-de-build-y-cicd)
13. [Plan de Implementación por Fases](#13-plan-de-implementación-por-fases)
14. [Riesgos y Mitigaciones](#14-riesgos-y-mitigaciones)
15. [Estrategia de Testing y QA](#15-estrategia-de-testing-y-qa)
16. [Referencias y Documentación](#16-referencias-y-documentación)

---

## 1. Resumen Ejecutivo

RSS2 es actualmente una **aplicación web monolítica orquestada con Docker Compose** compuesta por más de 20 servicios interconectados: un frontend React SPA, un backend Go (API REST Gin + 6 workers de ingesta/enriquecimiento), 11 workers Python (traducción neuronal, embeddings, NER, clustering, detección de idioma, categorización LLM) y tres sistemas de almacenamiento (PostgreSQL, Redis, Qdrant).

El objetivo de este plan es transformar este sistema en una **aplicación nativa instalable** en **Windows, Linux, Android e iOS**. Dada la enorme complejidad del stack actual (modelos ML de 600MB–1.3B, vector DB, colas, 20 servicios), la estrategia recomendada es una **arquitectura cliente–servidor con capacidades offline-first**:

- **El "cerebro" del sistema** (ingesta, IA, almacenamiento) sigue corriendo como un servidor (Docker o binario nativo en un equipo potente).
- **Las apps nativas** (desktop y móviles) son clientes ricos con caché local SQLite, modo offline, notificaciones push y, en escritorios con GPU, capacidad de actuar como **workers remotos de traducción** (reutilizando la infraestructura WebSocket ya existente).

La **clave del éxito** es el altísimo grado de reutilización del frontend React actual: la misma base de código puede empaquetarse con **Tauri** (Windows/Linux) y con **Capacitor** (Android/iOS), requiriendo únicamente abstracciones sobre las APIs del navegador.

---

## 2. Análisis Profundo del Código Actual

### 2.1 Arquitectura General

```
                        ┌────────────────────────────────────────────┐
                        │             CLIENTE WEB (Navegador)        │
                        │    React 18 + Vite + Tailwind + React Query │
                        └───────────────────┬────────────────────────┘
                                            │ HTTP/JSON /api + JWT
                                            ▼
                        ┌────────────────────────────────────────────┐
                        │              nginx (puerto 8888)           │
                        │   Gateway + proxy inverso + estáticos SPA  │
                        └───────┬─────────────────────────┬──────────┘
                                │ /api/                    │ / (estáticos)
                                ▼                          ▼
                        ┌────────────────────┐      ┌────────────────────┐
                        │    backend-go      │      │    rss2_frontend   │
                        │  Gin REST API :8080│      │  React compilado   │
                        └───────┬──────┬─────┘      └────────────────────┘
              ┌─────────────────┘      └───────────────┬────────────────────┐
              ▼                                        ▼                    ▼
        PostgreSQL 18                            Redis 7              Qdrant
        (esquema relacional)                  (caché/colas)        (vector DB)
              ▲
              │   Red interna de Docker Compose (backend network)
   ┌──────────┴─────────────────────────────────────────────────────────────┐
   │  Workers Go                    Workers Python                          │
   │  ────────────                  ──────────────                          │
   │  rss-ingestor-go               langdetect_worker.py                    │
   │  scraper (deep scraping)       ctranslator_worker.py (NLLB-200)        │
   │  discovery (descubrir feeds)   embeddings_worker.py (MiniLM)           │
   │  wiki_worker (Wikipedia)       ner_worker.py (spaCy es_core_news_lg)   │
   │  topics (países/temas)         cluster_worker.py                       │
   │  related (similitud coseno)    translation_scheduler.py                │
   │  qdrant (vectorización)        llm_categorizer_worker.py (Ollama)      │
   │                                 remote_translator_worker.py (WS GPU)   │
   └────────────────────────────────────────────────────────────────────────┘
```

**Topología de redes (docker-compose.yml):**
- Red `frontend`: nginx, rss2_frontend, backend-go (puente entre redes).
- Red `backend`: db, redis, ingestor, langdetect, scraper, discovery, wiki-worker, translator, translator-gpu, scheduler, embeddings, topics, related, qdrant, qdrant-worker, ner, cluster.
- Único puerto expuesto al exterior: **nginx :8888**.

### 2.2 Inventario de Servicios (docker-compose.yml, 20 servicios)

| Servicio | Imagen/Dockerfile | Rol | Recursos |
|---|---|---|---|
| `db` | postgres:18-alpine | Base de datos relacional | 2 CPU / 8G |
| `redis` | redis:7-alpine | Caché + colas (workers Python) | 768M |
| `rss-ingestor-go` | rss-ingestor-go/Dockerfile | Crawler RSS (100 workers) | 2 CPU / 2G |
| `langdetect` | Dockerfile | Detección de idioma | 0.5 CPU / 512M |
| `scraper` | Dockerfile.scraper | Extracción profunda de artículos | 1 CPU / 512M |
| `discovery` | Dockerfile.discovery | Descubrimiento de feeds | 1 CPU / 512M |
| `wiki-worker` | Dockerfile.wiki | Enriquecimiento Wikipedia | 0.5 CPU / 256M |
| `backend-go` | backend/Dockerfile | API REST principal | 2 CPU / 512M |
| `rss2_frontend` | frontend/Dockerfile | SPA React | 1 CPU / 512M |
| `nginx` | nginx:alpine | Gateway | — |
| `translator` | Dockerfile.translator | Traducción NLLB CPU | 4 CPU / 6G |
| `translator-gpu` | Dockerfile.translator | 2º worker CPU (nombre heredado) | 4 CPU / 6G |
| `translation-scheduler` | Dockerfile.scheduler | Crea trabajos de traducción | 0.5 CPU / 256M |
| `embeddings` | Dockerfile | Vectores MiniLM | 2 CPU / 6G |
| `topics` | Dockerfile.topics | Matcher países/temas | 1 CPU / 512M |
| `related` | Dockerfile.related | Noticias relacionadas | 1 CPU / 1G |
| `qdrant` | qdrant/qdrant:latest | Base vectorial | 4 CPU / 4G |
| `qdrant-worker` | Dockerfile.qdrant | Vectorización → Qdrant | 1 CPU / 1G |
| `ner` | Dockerfile | NER spaCy | 2 CPU / 2G |
| `cluster` | Dockerfile | Clustering de noticias | 2 CPU / 2G |

> **Observación crítica para la migración:** el servicio `translator-gpu` NO usa CUDA; es un segundo worker CPU. La única capacidad GPU real es vía **workers remotos WebSocket** (`remote_translator_worker.py`). Esto es una ventaja enorme: la infraestructura para que un equipo externo (potencialmente una app de escritorio) traduzca vía WebSocket **ya existe y está probada**.

### 2.3 Frontend (React + TypeScript + Vite)

**Ubicación:** `frontend/src` — 26 archivos, ~4.300 líneas TS/TSX.

**Stack:** React 18.2, TypeScript 5.3 (strict), Vite 5, Tailwind CSS 3.4, React Router 6, TanStack React Query 5, Axios, lucide-react (iconos), date-fns.

**Estructura:**
- `main.tsx` → `QueryClientProvider` → `BrowserRouter` → `App`.
- `App.tsx`: llama a `GET /auth/check-first-user`; si es el primer usuario muestra `WelcomeWizard` (onboarding de 3 pasos), si no, el árbol de rutas envuelto en `Layout`.
- 18 páginas: `Home`, `News`, `Search`, `Feeds`, `Stats`, `Favorites`, `Account`, `Login`, `Populares` (732 líneas, la más compleja), `Analisis`, `Alertas`, `WelcomeWizard`, `AdminAliases`, `AdminUsers`, `AdminSettings`, `AdminWorkers`.
- 4 componentes reutilizables: `Layout`, `ProtectedRoute`, `LineChart` (SVG manual), `WikiTooltip`.
- `services/api.ts` (279 líneas): instancia Axios, interceptor de petición (inyecta JWT de `localStorage`), interceptor de respuesta (401 → redirige a `/login?expired=1`), tipos TS e `apiService` (22 métodos tipados) + exportación del cliente crudo `api`.

**Dependencias de navegador que bloquean una migración nativa directa:**

| API web actual | Uso | Sustitución nativa |
|---|---|---|
| `localStorage` (`token`, `user`, `favorites`) | Almacenar sesión y favoritos | SecureStorage/Keychain/Keyring (Electron/Tauri/Capacitor) + SQLite |
| `window.location.href` (redirect 401) | Forzar logout | Callback de sesión expirada → navegación interna |
| `window.confirm/prompt/alert` | Confirmar borrados, doble confirmación ("type SI"), etc. | Diálogos nativos (capas JS→Rust/ObjC/Swift/Kotlin) o modales React |
| `URL.createObjectURL` + `<a download>` | Descarga de CSV/ZIP/SQL | API de archivos nativa (`saveDialog` en Tauri/Electron, `Share`/filesystem en Capacitor) |
| `window.addEventListener('storage')` | Sync de auth entre pestañas | Eventos de sesión dentro de la misma app |
| `window.history` (BrowserRouter) | Navegación | `HashRouter`/`MemoryRouter` o navegación nativa |
| `window.URLSearchParams` | Parámetros de ruta | React Router compatible |
| `navigator`/Viewport | Responsividad | Adaptación a pantallas táctiles |

**Deudas técnicas detectadas en el frontend (deben corregirse durante la migración):**
1. **Nav móvil rota:** `hidden sm:flex` sin hamburguesa → inutilizable en móvil (< 640px).
2. `WikiTooltip` solo responde a hover → sin soporte táctil.
3. Página `Favorites` usa solo `localStorage` (no sincronizada con backend, pese a existir la tabla `favoritos`).
4. `Account` no persiste cambios (guardado falso).
5. Botón `btn-danger` usado pero **no definido** en `index.css`.
6. Dos patrones de fetch inconsistentes: React Query vs `useEffect`+fetch manual (Populares, Alertas, Analisis, Admin*).
7. Dos clientes de API: `api` crudo vs `apiService` tipado.
8. Filtros de categoría/país en `Search` son "muertos" (no se envían al backend).
9. `clsx` y `tailwind-merge` instalados pero sin uso.
10. **No existe ni un solo test** pese a vitest configurado.
11. Sin code-splitting (`React.lazy` ausente); todo el bundle en un solo archivo.
12. `JWT_EXPIRATION` del backend ignorado; el token expira en 24h fijas sin refresh — **crítico para apps móviles**.

### 2.4 Backend Go (API + Workers)

**Módulo:** `github.com/rss2/backend`, Go 1.23, `gin v1.9.1`, `pgx/v5`, `go-redis/v9`, `golang-jwt/v5`, `gorilla/websocket`, `gofeed`, `goquery`.

**Paquetes internos:**
- `internal/config` — `Load()` lee variables de entorno (sin archivos `.env`; todo lo inyecta Docker).
- `internal/db` — `pgxpool` (25 conns max), solo para la API (`DATABASE_URL`).
- `internal/cache` — Cliente Redis (helpers nunca usados por handlers; solo conexión).
- `internal/auth` — JWT HS256, expiración 24h **hardcodeada**.
- `internal/middleware` — `AuthRequired`, `AdminRequired`, CORS. `RateLimit` es un stub **no conectado**.
- `internal/models` — Tipos de dominio. ⚠️ `News.ID` es `int64` pero la DB usa `VARCHAR(32)` (MD5 de la URL).
- `internal/handlers` — auth, news, feed, search, alerts, admin, remote_worker, worker_ws.
- `internal/services/ml.go` — Clientes HTTP hacia Libretranslate/Ollama/spaCy; `SemanticSearch` es **stub que devuelve vacío**.
- `internal/workers/db.go` — Helper de conexión compartido por los 6 workers (usa `DB_*` env vars).

**Arranque del servidor (`cmd/server/main.go`):**
1. `config.Load()` → 2. `db.Connect()` (fatal si falla) → 3. `initDB()` (migraciones idempotentes: crea `entity_aliases`, `config`, `alertas`, `remote_workers`; añade `role`; seed de `config`) → 4. `cache.Connect()` (no fatal) → 5. `services.Init()` → 6. router Gin → 7. CORS → 8. rutas → 9. `auth.SetJWTSecret()` → 10. `r.Run(":8080")` → 11. goroutines `StartJobAssigner()` (cada 5s) y `StartAlertScanner()` (cada 120 min) → 12. señal SIGINT/SIGTERM para shutdown graceful.

**Workers Go (binarios independientes, todos con bucle `time.Ticker`):**

| Worker | Loop | Función |
|---|---|---|
| `scraper` | 60s | Enriquece `noticias` con resumen corto (<200 chars) vía goquery; extrae artículos de `fuentes_url` (OpenGraph + selectores). |
| `discovery` | 900s | Detecta feeds en `fuentes_url`; crea `feeds` o `feeds_pending` (aprobación manual). |
| `wiki_worker` | 30s | Consulta la API REST de Wikipedia ES; descarga thumbnails (≤2MB) a `data/wiki_images`; rellena `wiki_summary`, `wiki_url`, `image_path`. |
| `topics` | 10s | Matchea keywords de `topics` contra noticias sin procesar; asigna `pais_id`. |
| `related` | 10s | Similitud coseno entre embeddings almacenados; rellena `related_noticias`. |
| `qdrant` | 30s | Toma `traducciones` sin vectorizar, parsea el embedding del array Postgres, hace upsert en colección `news_vectors` de Qdrant. |

**WebSocket `/ws/worker` (`handlers/worker_ws.go`):**
- Protocolo existente y probado para workers remotos GPU.
- Auth por `api_key` (query o header `X-API-Key`) contra tabla `remote_workers`.
- Registro, heartbeat (10s), ping (30s), asignación de jobs con `FOR UPDATE SKIP LOCKED`, límite de 5 min en `assigned`, re-encolado tras desconexión.
- **El endpoint se registra fuera de `/api`** (en `/ws/worker`) y **no está proxied** por el nginx de producción — quirk a documentar.

### 2.5 Workers Python

**Dependencias (`requirements.txt`):** Flask, feedparser, APScheduler, psycopg2, transformers 4.43, ctranslate2, sentence-transformers 3.0.1, spaCy 3.7, sklearn, qdrant-client 1.11, redis, langdetect, newspaper3k, etc.

| Worker | Modelo | Recursos | Comportamiento |
|---|---|---|---|
| `ctranslator_worker.py` | NLLB-200 distilled 600M (CT2, int8) | 4 CPU / 6G | Loop 30s; `FOR UPDATE SKIP LOCKED`; batch 128; cache Redis por título/cuerpo; trocea cuerpos en chunks de 900 chars; `executemany`; registra `translation_stats`. |
| `embeddings_worker.py` | sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2 | 2 CPU / 6G | `model.encode(normalize_embeddings=True)`; upsert en `traduccion_embeddings`; sleep 5s idle. |
| `ner_worker.py` | spaCy `es_core_news_lg` | 2 CPU / 2G | Extrae PER/ORG/LOC → `tags` y `tags_noticia`. |
| `cluster_worker.py` | embeddings MiniLM + sklearn | 2 CPU / 2G | Agrupa noticias en `eventos` (umbral 0.35). |
| `langdetect_worker.py` | langdetect | 0.5 CPU | Detecta idioma → `noticias.lang`. |
| `llm_categorizer_worker.py` | Ollama/Mistral-7B | (externa) | Clasificación LLM (opcional). |
| `translation_scheduler.py` | — | 0.5 CPU | Crea filas `traducciones` pendientes. |
| `remote_translator_worker.py` | NLLB-200 (600M/1.3B) | GPU/CPU | Cliente WebSocket → servidor `/ws/worker`; reconnect exponencial; procesa jobs de traducción. **Este es el prototipo perfecto para el módulo "worker" de la app de escritorio.** |

**Dependencia CTranslate2:** `libctranslate2.so` requiere `patchelf --clear-execstack` en Linux (ver `Dockerfile.remote-worker`), y torch==2.1.0 cu118. Esto condiciona el empaquetado nativo (ver §8 y §11).

### 2.6 Base de Datos PostgreSQL

**Esquema** (fuente: `init-db/00-complete-schema.sql` + 37 migraciones numeradas; se aplica al iniciar el contenedor, sin framework de migraciones Go).

**Tablas principales (28):**
- **Núcleo:** `users`, `config`, `continentes`, `categorias`, `paises`, `feeds`, `fuentes_url`, `noticias`, `traducciones`.
- **Enriquecimiento:** `tags`, `tags_noticia`, `entity_aliases`, `entity_images`, `topics`, `news_topics`, `related_noticias`, `eventos`, `eventos_noticias`.
- **IA:** `traduccion_embeddings` (array `DOUBLE PRECISION[]`), vista `embeddings`.
- **Social:** `favoritos`, `search_history`, `user_search_tags`, `alertas`, `videos`, `video_parrillas`, `remote_workers`, `translation_stats`, `feeds_pending`.

**Características PostgreSQL-específicas (problemas para un embebido SQLite):**
1. **PK de noticias = `VARCHAR(32)`** = MD5 hex de la URL.
2. Full-text en español: columnas `tsvector` (`tsv`, `search_vector_es`) con triggers (`to_tsvector('spanish', ...)`) + índices GIN.
3. `traduccion_embeddings.embedding` es un **array de precisión doble**.
4. Consultas avanzadas: `FOR UPDATE SKIP LOCKED` (workers), `unnest` (bulk inserts), `generate_series` (mentions), CTEs con ventanas (alerta anomalías), `ON CONFLICT DO NOTHING/UPDATE`.
5. El backend llama a `pg_dump`/`psql` desde `cmd/admin.go` para backup/restore (requiere el binario `postgresql-client` en la imagen).

**Fuga de memoria / diferencia de tipos:** `News.ID` en Go models es `int64`; los handlers devuelven el ID string real. Los clientes deben tratar los IDs de noticia como **strings opacos**.

### 2.7 Qdrant (Base Vectorial)

- Colección única `news_vectors` (distancia coseno, dim derivada del modelo MiniLM = 384).
- Payload: `news_id`, `titulo`, `resumen`, `url`, `fuente`, `lang`, `fecha`, `category`, `pais`.
- El worker `qdrant-worker` hace upserts de puntos con UUIDs; marca `traducciones.vectorized=TRUE` y guarda `qdrant_point_id`.
- ⚠️ La búsqueda semántica vía API (`/api/search?semantic=true`) es un **stub vacío**: no consulta Qdrant.

### 2.8 Redis

- 512MB maxmemory, appendonly, requirepass.
- Usado por `ctranslator_worker` como **cache de traducciones** (títulos/cuerpos).
- El backend Go solo establece la conexión; **no usa los helpers de cache**.
- Para la app nativa: Redis es irrelevante (el servidor lo gestiona).

### 2.9 Modelos ML y Pesos

| Modelo | Tamaño | Uso |
|---|---|---|
| `nllb-ct2` (600M, int8) | ~600MB (`models/`) | Traducción CPU |
| `nllb-ct2-1.3b` | ~1.3GB | Traducción GPU/calidad |
| `nllb-200-distilled-600M` (tokenizer) | via HF | Tokenizer |
| `paraphrase-multilingual-MiniLM-L12-v2` | ~470MB | Embeddings |
| `es_core_news_lg` (spaCy) | ~500MB | NER |
| Mistral-7B (Ollama) | ~4GB | Categorización LLM (opcional) |
| `hf_cache/` | **7.9GB** | Caché HuggingFace |

**Conclusión para el dispositivo:** los modelos completos no caben en móvil de forma realista. Ver §7.

### 2.10 Superficie de API Completa

Todas las rutas bajo `/api`, auth JWT Bearer (salvo las públicas).

| Método | Ruta | Auth | Descripción |
|---|---|---|---|
| GET | `/health` | No | Health check |
| GET | `/api/wiki-images/*` | No | Estáticos (imágenes Wikipedia) |
| POST | `/api/auth/login` | No | Login |
| POST | `/api/auth/register` | No | Registro (1er usuario = admin) |
| GET | `/api/auth/check-first-user` | No | ¿Primer usuario? |
| GET | `/api/auth/me` | Sí | Usuario actual |
| GET | `/api/news` | No | Lista paginada con filtros `q, category_id, country_id, translated_only, page, per_page` |
| GET | `/api/news/:id` | No | Detalle + entidades con wiki |
| DELETE | `/api/news/:id` | Sí | Eliminar noticia |
| GET | `/api/feeds` | No | Feeds paginados + categorías/países + conteos |
| GET | `/api/feeds/export` | No | CSV |
| GET | `/api/feeds/:id` | No | Detalle feed |
| POST | `/api/feeds` | Sí | Crear feed |
| POST | `/api/feeds/import` | Sí | Importar CSV |
| PUT | `/api/feeds/:id` | Sí | Actualizar |
| DELETE | `/api/feeds/:id` | Sí | Eliminar |
| POST | `/api/feeds/:id/toggle` | Sí | Activar/desactivar |
| POST | `/api/feeds/:id/reactivate` | Sí | Reset fallos |
| GET | `/api/search` | No | Búsqueda texto (`q, lang`; `semantic=true` = stub) |
| GET | `/api/search/suggestions` | Sí | Términos del usuario |
| POST | `/api/searchlog` | Sí | Registrar búsqueda |
| GET | `/api/entities` | No | Entidades canónicas paginadas |
| GET | `/api/entities/news` | No | Noticias de una entidad |
| GET | `/api/entities/mentions` | No | Serie temporal de menciones |
| GET | `/api/alerts` | No | Alertas de anomalías |
| POST | `/api/alerts/:id/read` | Sí | Marcar leída |
| POST | `/api/alerts/read-all` | Sí | Marcar todas leídas |
| GET | `/api/stats` | No | Estadísticas globales |
| GET | `/api/categories` | No | Categorías |
| GET | `/api/countries` | No | Países |
| POST | `/api/admin/aliases` | Admin | Crear aliases |
| GET | `/api/admin/aliases/export` | Admin | CSV |
| POST | `/api/admin/aliases/import` | Admin | Importar CSV |
| POST | `/api/admin/entities/retype` | Admin | Cambiar tipo entidad |
| GET | `/api/admin/backup` | Admin | pg_dump SQL |
| GET | `/api/admin/backup/news` | Admin | ZIP noticias |
| POST | `/api/admin/restore` | Admin | Restaurar |
| GET | `/api/admin/users` | Admin | Usuarios |
| POST | `/api/admin/users/:id/promote` | Admin | Hacer admin |
| POST | `/api/admin/users/:id/demote` | Admin | Quitar admin |
| POST | `/api/admin/reset-db` | Admin | Vaciar 11 tablas |
| POST | `/api/admin/alerts/scan` | Admin | Ejecutar escáner |
| GET | `/api/admin/workers/status` | Admin | Estado Docker |
| GET | `/api/admin/workers/stats` | Admin | Throughput traducción |
| POST | `/api/admin/workers/config` | Admin | Config traducción |
| POST | `/api/admin/workers/start` | Admin | `docker compose up` |
| POST | `/api/admin/workers/stop` | Admin | `docker compose stop` |
| GET/POST | `/api/admin/workers/remote` | Admin | Listar/crear workers remotos |
| GET/DELETE | `/api/admin/workers/remote/:id` | Admin | Detalle/borrar |
| POST | `/api/admin/workers/remote/:id/toggle` | Admin | Online/offline |
| POST | `/api/admin/workers/remote/:id/regenerate-key` | Admin | Regenerar API key |
| GET | `/ws/worker` | API key | WebSocket workers remotos |

### 2.11 Configuración (Variables de Entorno)

Sin archivo `.env` en Go; todo via env vars inyectadas por Docker Compose. Tabla clave:

| Variable | Default | Uso |
|---|---|---|
| `SERVER_PORT` | 8080 | Puerto API |
| `DATABASE_URL` | postgres://rss:rss@localhost:5432/rss | DSN API |
| `REDIS_URL` | redis://localhost:6379 | DSN cache |
| `SECRET_KEY` | — | Firma JWT |
| `JWT_EXPIRATION` | 24h | **Leído pero ignorado** |
| `TRANSLATION_URL` | http://libretranslate:7790 | Servicio traducción (sin uso real) |
| `OLLAMA_URL` | http://ollama:11434 | Embeddings/categorización |
| `SPACY_URL` | http://spacy:8000 | NER |
| `DOCKER_COMPOSE_DIR` | /datos/rss2 | Directorio para `docker compose` de admin |
| `WIKI_IMAGES_PATH` | /app/data/wiki_images | Imágenes wiki |
| `ALLOWED_ORIGINS` | * | CORS |

Los 6 workers usan `DB_HOST/DB_PORT/DB_NAME/DB_USER/DB_PASS` + variables propias (`SCRAPER_*`, `WIKI_*`, `TOPICS_*`, `RELATED_*`, `QDRANT_*`, `ALERTS_*`).

### 2.12 Limitaciones Relevantes para la Migración

1. **`SemanticSearch` no implementado** en la API Go (stub vacío).
2. **`JWT_EXPIRATION` ignorado** — sin refresh tokens; crítico en móvil.
3. **Admin depende de Docker** (`start/stop/status/backup/restore` hacen `exec.Command`).
4. **Sin framework de migraciones DB** (todo en el init del contenedor PostgreSQL).
5. **Esquema fuertemente PostgreSQL-específico** (tsvector, GIN, arrays, `FOR UPDATE SKIP LOCKED`).
6. **Sin tests** en frontend y sin suite CI visible para la mayoría de servicios.
7. **Sin rate-limiting real** (stub no conectado).
8. **Mensajes y UI 100% en español** hardcodeados (falta i18n).
9. **Favoritos sin backend** aunque existe la tabla `favoritos`.

---

## 3. Objetivos y Requisitos de la Nueva Aplicación

### 3.1 Requisitos Funcionales

1. **RF-1 Lectura de noticias:** listar, filtrar (categoría, país, solo traducidas), ver detalle, entidades con Wikipedia.
2. **RF-2 Búsqueda:** texto (y semántica si se implementa), sugerencias.
3. **RF-3 Favoritos:** guardar/eliminar noticias (sincronizados con servidor).
4. **RF-4 Gestión de feeds:** CRUD, activar/desactivar, reactivar, importar/exportar CSV.
5. **RF-5 Análisis:** entidades populares, evolución temporal de menciones, alertas de anomalías.
6. **RF-6 Estadísticas globales.**
7. **RF-7 Autenticación:** login/registro, sesión persistente, logout.
8. **RF-8 Administración** (solo admin): usuarios, aliases, workers, backup/restore, reset.
9. **RF-9 Notificaciones push** (móvil) y del sistema (desktop) para nuevas alertas.
10. **RF-10 Modo offline:** lectura de noticias cacheadas sin conexión.
11. **RF-11 (Avanzado) Modo worker:** el desktop puede actuar como **worker de traducción remota** (WebSocket GPU/CPU) conectado a un servidor central.
12. **RF-12 (Avanzado) Núcleo local:** el desktop puede ejecutar el **backend completo embebido** (servidor personal local) con la API en `localhost`.

### 3.2 Requisitos No Funcionales

| # | Requisito |
|---|---|
| RNF-1 | Instalación nativa en Windows 10/11 x64, Linux x64 (deb/AppImage/Flatpak), Android 8+, iOS 15+. |
| RNF-2 | Tamaño de descarga: ≤80MB (sin modelos embebidos). |
| RNF-3 | Arranque < 3s; UI fluida (60fps) en dispositivos medios. |
| RNF-4 | Cifrado de credenciales (Keychain/Keyring/Keystore). |
| RNF-5 | Actualizaciones automáticas (auto-update). |
| RNF-6 | Soporte offline-first con reconciliación al reconectar. |
| RNF-7 | Cumplir políticas de tiendas (Google Play, App Store) — sin descargas de modelos gigantes en primer arranque móvil. |
| RNF-8 | Seguridad: JWT con refresh, almacenamiento seguro, `https` obligatorio en producción. |
| RNF-9 | i18n (al menos ES + EN) — aprovechar la migración para introducir `react-i18next`. |

---

## 4. Arquitectura Objetivo (Alternativas)

### 4.1 Opción A — Cliente–Servidor con Offline-First (RECOMENDADA)

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         SERVIDOR RSS2 (central o personal)             │
│   Docker Compose / binario nativo → ingesta, IA, DB, Qdrant, Redis     │
│   nginx :8888 → API REST /api + WebSocket /ws/worker                   │
└───────────────┬─────────────────────────────┬──────────────────────────┘
                │ HTTPS (REST + JWT refresh)  │ WebSocket (trabajos)
                ▼                             ▼
┌───────────────────────────────┐   ┌────────────────────────────────────┐
│  Apps nativas (clientes)      │   │  Desktop con GPU (modo worker)    │
│  Windows / Linux / Android/iOS│   │  NLLB-200 local via WebSocket     │
│                               │   │  (reutiliza remote_translator)    │
│  SQLite local (caché offline) │   └────────────────────────────────────┘
└───────────────────────────────┘
```

**Ventajas:** reutilización total del backend actual (sin reescribir nada de Go/Python); las apps son "clientes ricos"; el modo worker desktop aprovecha la infraestructura WebSocket existente.
**Desventajas:** requiere conexión para ingesta en tiempo real; el modo offline es de lectura/caché.

### 4.2 Opción B — Núcleo Embebido Total (server local en el dispositivo)

Cada desktop ejecuta **todo el stack embebido**: backend Go + PostgreSQL embebido + Qdrant + modelos ML. Móviles se conectan al desktop o usan el servidor central.

**Ventajas:** privacidad total, sin servidor externo.
**Desventajas:** enorme complejidad de empaquetado (Postgres/Qdrant/modelos por plataforma), requisitos de RAM/almacenamiento prohibitivos en móvil, y duplicación de todo el coste de operación. **Descartado como objetivo principal**, solo viable como **modo avanzado desktop** en Fase 5.

### 4.3 Opción C — Híbrida (recomendada como hoja de ruta de 3 etapas)

1. **Etapa 1 (MVP):** Opción A pura — apps nativas como clientes del servidor central.
2. **Etapa 2 (Offline):** Caché SQLite local + sincronización bidireccional + push.
3. **Etapa 3 (Pro):** Modo "worker" desktop (GPU) + modo "núcleo local" desktop (backend embebido) para usuarios avanzados y despliegues domésticos.

**Recomendación:** **Opción C**. Es la que equilibra esfuerzo, valor y viabilidad técnica. El resto del plan se basa en ella.

---

## 5. Selección Tecnológica por Plataforma

### 5.1 Estrategia de Reuso del Frontend

El 95% de la UI actual es React + Tailwind. Se recomienda **una sola base de código React** empaquetada con:

| Plataforma | Tecnología | Razonamiento |
|---|---|---|
| **Windows + Linux** | **Tauri 2** (Rust shell + WebView2/WebKitGTK) | Binarios pequeños (~8-15MB), bajo consumo, API nativa para archivos/notificaciones/servicios, ideal para empaquetar el backend Go como sidecar. |
| **Android + iOS** | **Capacitor 6** (wrapper nativo de la misma SPA React) | Reutiliza la SPA tal cual; acceso a Push, SecureStorage, Filesystem, SplashScreen, autoupdate (capgo). |

**Alternativa evaluada y rechazada:**
- **Electron:** gran ecosistema y autoupdate maduro, pero binarios de 80-120MB y alto consumo RAM; innecesario con WebView del SO moderno. (Puede elegirse si el equipo prefiere JS puro para el shell — coste en tamaño/RAM.)
- **Flutter / React Native:** supondría **reescribir** las ~4.300 líneas TSX. Solo tiene sentido si se quiere UI 100% nativa y se descarta el reuso. **No recomendado** dado el frontend existente.
- **.NET MAUI / Kotlin Multiplatform:** mismo argumento, coste de reescritura alto.

**Arquitectura de paquetes (monorepo):**

```
rss2/
├── app/                          # NUEVO: aplicación multiplataforma
│   ├── packages/
│   │   ├── core-ui/              # Componentes React reutilizables (portados)
│   │   ├── api-client/           # Cliente HTTP + tipos (portados de api.ts)
│   │   ├── storage/              # Abstracción de almacenamiento seguro
│   │   ├── sync/                 # Motor de sincronización offline
│   │   └── platform/             # Bridge nativo (Tauri/Capacitor)
│   ├── src/                      # SPA React compartida
│   ├── desktop/                  # Shell Tauri (Rust)
│   │   └── src-tauri/
│   ├── mobile/                   # Configuración Capacitor (Android/iOS)
│   └── worker/                   # Módulo "worker de traducción" embebido
│       └── (port de remote_translator_worker.py a Rust/embedded)
```

### 5.2 Desktop (Windows/Linux) — Tauri 2

- **Rust shell** con `tauri-plugin-shell`, `tauri-plugin-store`, `tauri-plugin-notification`, `tauri-plugin-autostart`, `tauri-plugin-updater` (v2), `tauri-plugin-sql` (SQLite vía rusqlite), `tauri-plugin-fs`.
- **Comunicación JS↔Rust:** comandos `invoke` (ej. `save_dialog`, `read_file`, `notification_permission`).
- **Sidecar backend Go:** compilado con `go build -ldflags "-s -w"` y registrado como **sidecar binario** de Tauri (`externalBin`). Arrancado/bajo control de Rust (`tauri-plugin-shell`), con auto-restart y registro de logs.
- **Ventana:** aprovecha WebView2 (Windows 10/11 preinstalado) y WebKitGTK (Linux). Tamaño mínimo 1024×700, redimensionable.
- **Bandeja del sistema** (tray) para background-ingesta (opcional en Fase 5).
- **Notificaciones nativas** del sistema.

### 5.3 Android/iOS — Capacitor 6

- **Mismo `dist/` de Vite** sirve para web y para Capacitor (`npx cap add android/ios`).
- **Plugins requeridos:**
  - `@capacitor/preferences` (o `@capacitor/secure-storage-plugin`) → sustituye `localStorage` para token.
  - `@capacitor/push-notifications` → notificaciones push (FCM/APNs).
  - `@capacitor/filesystem` → exportación de CSV/ZIP/SQL a la carpeta de descargas / Share.
  - `@capacitor/share` → compartir noticias.
  - `@capacitor/network` → detección de conectividad para el modo offline.
  - `@capacitor/status-bar`, `@capacitor/splash-screen`.
- **Requiere `HashRouter`** en móvil (sin `window.history` en file://).
- **Actualizaciones:** [Capgo](https://capgo.app) o re-publicación en tiendas para updates OTA; revisión obligatoria para iOS.

### 5.4 Servidor (cerebro)

Dos opciones de despliegue del backend que la app debe soportar:
1. **Servidor central (SaaS/self-hosted):** el `docker-compose.yml` actual sin cambios; la app se conecta a una URL HTTPS.
2. **Núcleo embebido desktop (Fase 5):** el desktop ejecuta un **binario único** que integra la API Go + los workers esenciales (ingesta, wiki, topics, langdetect) sobre **PostgreSQL embebido portable** (ver §8), sin Docker.

### 5.5 Base de Datos Local — SQLite SOLO como caché de cliente (nunca como almacén principal)

**Decisión arquitectónica clave (escala: millones de noticias):** el sistema de registro (system of record) es **PostgreSQL + Qdrant en el servidor**, igual que hoy. **SQLite se usa ÚNICAMENTE como caché acotada y estado de usuario en el dispositivo** — nunca debe contener el dataset completo ni servir como almacén principal.

Razones técnicas (derivadas del código actual):

| Limitación de SQLite | Impacto en RSS2 a escala de millones |
|---|---|
| **Escritura serializada (single-writer)** | El `rss-ingestor-go` corre hasta 100 workers concurrentes y los workers Python usan `FOR UPDATE SKIP LOCKED`. SQLite encolaría todas las escrituras → cuello de botella de ingesta. |
| **Sin `FOR UPDATE SKIP LOCKED` ni `unnest`** | Los workers de traducción/topics dependen de estos patrones; reimplementarlos sobre SQLite degrada el rendimiento y añade bugs. |
| **FTS5 vs tsvector/GIN** | El esquema actual usa `to_tsvector('spanish', ...)` con triggers e índices GIN. FTS5 funciona, pero con millones de filas y escrituras concurrentes el mantenimiento del índice es más débil que Postgres. |
| **Embeddings y vectores** | `traduccion_embeddings` guarda arrays `DOUBLE PRECISION[]` (384 dims ≈ 3KB+/fila). 1 noticia → N traducciones → M embeddings: millones de noticias ⇒ decenas de millones de filas. La búsqueda semántica necesita **Qdrant**, no similitud in-memory de SQLite. |
| **Backups/restore** | El backend ya usa `pg_dump`/`psql`; con SQLite habría que reimplementar y perderías la madurez de esas herramientas. |

**Dónde SÍ es válido SQLite (roles de cliente):**
- Caché de lectura con **evicción forzada** (ver §6.1: presupuesto máx. ~50k noticias y retención de 7–30 días, LRU).
- Estado de usuario: favoritos, leídos, `outbox` de escrituras pendientes, metadatos de feeds.
- Esquema local (ver §6.2) **no replica el esquema PostgreSQL**: es un esquema de aplicación mínimo para lectura rápida.

**Alternativa evaluada:** libSQL/Turso (replicación distribuida) — solo si más adelante se quiere multi-dispositivo con el cliente como fuente. **No recomendado ahora**; el servidor PostgreSQL sigue siendo la fuente de verdad.

### 5.6 Resumen de Decisión

| Componente | Tecnología | Reuso del código actual |
|---|---|---|
| UI | React 18 + Tailwind (misma SPA) | 95% reutilizado |
| Desktop shell | Tauri 2 (Rust) | — |
| Móvil shell | Capacitor 6 | — |
| Cliente HTTP | Axios (portado a `api-client`) | 100% |
| Almacenamiento seguro | Keyring/Keychain/Keystore | nuevo |
| DB local | SQLite | nuevo |
| Motor de traducción (worker) | CTranslate2 (embedded, port a Rust opcional) | reutiliza lógica de `remote_translator_worker.py` |
| Servidor | Docker Compose actual / binario nativo | 100% |

---

## 6. Estrategia de Datos y Sincronización

### 6.1 Modelo Offline-First

**Principios:**
1. **Lectura:** todas las listas de noticias/entidades/feeds se cachean en SQLite local **con presupuesto y evicción forzada**. SQLite es caché, no almacén: la app NUNCA guarda el dataset completo.
2. **Presupuesto de caché (regla dura):** máx. **50.000 noticias** en `news_cache` y retención por TTL (5 min listas, 24h detalle, 30 días fondo). Al superar el tope se aplica **LRU** (borrar por `fetched_at` más antiguo) y se recorta por antigüedad (`fecha < now - 30d`). El servidor conserva el histórico completo; la app solo su ventana de trabajo.
3. **Escritura (favoritos/leídos):** se aplican localmente al instante y se encolan en una tabla `outbox` para sincronizar. El `outbox` se vacía tras confirmación del servidor y no tiene límite práctico (son pocas filas por usuario).
4. **Sincronización:** un motor de sync (background worker) reconcilia `outbox` contra el servidor cuando hay red; conflictos resueltos por "última escritura gana" con timestamp.
5. **Rehidratación:** al reconectar, se invalida la caché y se refresca desde el servidor.

### 6.2 Esquema SQLite Local (caché + estado, con evicción)

```sql
-- Caché de lectura (SIEMPRE acotada: máx. ~50k filas, LRU + TTL)
CREATE TABLE news_cache (
  id TEXT PRIMARY KEY,            -- MD5 de la URL (string opaco)
  payload TEXT NOT NULL,          -- JSON completo de la noticia
  fetched_at INTEGER NOT NULL,    -- epoch ms (usado por el LRU)
  ttl INTEGER NOT NULL DEFAULT 86400000
);
CREATE TABLE list_cache (
  key TEXT PRIMARY KEY,           -- "news:page=1:cat=2" etc.
  payload TEXT NOT NULL,
  fetched_at INTEGER NOT NULL,
  ttl INTEGER NOT NULL DEFAULT 300000
);
CREATE INDEX idx_news_fetched ON news_cache(fetched_at);
-- Trabajo de evicción (job diario):
--   DELETE FROM news_cache WHERE fetched_at < now - 30d;
--   DELETE FROM news_cache WHERE id IN (
--     SELECT id FROM news_cache ORDER BY fetched_at ASC
--     LIMIT (SELECT MAX(0, COUNT(*) - 50000) FROM news_cache));
--   DELETE FROM list_cache WHERE fetched_at < now - ttl;

-- Estado de usuario (sync bidireccional)
CREATE TABLE favorites (
  noticia_id TEXT PRIMARY KEY,
  created_at INTEGER NOT NULL,
  dirty INTEGER NOT NULL DEFAULT 1
);
CREATE TABLE read_states (
  noticia_id TEXT PRIMARY KEY,
  read_at INTEGER NOT NULL,
  dirty INTEGER NOT NULL DEFAULT 1
);

-- Cola de escrituras pendientes de sincronizar (pocas filas; se vacía al sync)
CREATE TABLE outbox (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  op TEXT NOT NULL,               -- 'add_favorite' | 'remove_favorite' | 'read'
  payload TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  synced INTEGER NOT NULL DEFAULT 0
);

-- Metadatos de feeds (para gestión offline)
CREATE TABLE feeds_meta (
  id INTEGER PRIMARY KEY,
  nombre TEXT, url TEXT, activo INTEGER,
  last_error TEXT, updated_at INTEGER
);

-- Config local
CREATE TABLE app_settings (key TEXT PRIMARY KEY, value TEXT);
```

> **Nota de escala:** este esquema NO es una réplica del esquema PostgreSQL (que alberga los millones de noticias, traducciones, embeddings y relaciones). Es una capa de lectura/escritura mínima cuyo tamaño total se mantiene **< 100MB** gracias a la evicción.

### 6.3 Sincronización Bidireccional

**Flujo del motor de sync:**
1. Detectar conectividad (`@capacitor/network` / evento de red en Tauri).
2. Enviar `outbox` pendiente en lotes (con el JWT).
3. Aplicar confirmaciones (borrar filas sincronizadas).
4. Pull incremental: `GET /api/news?since=<cursor>` (requiere añadir filtro `since` al backend, ver §9) → upsert en `news_cache`.
5. Refrescar `favorites` del servidor (una vez `POST /api/favorites` esté implementado, ver §6.5).

### 6.4 Imágenes y Multimedia

- Las imágenes de noticias son **URLs remotas**; se cachean en disco (`~/.cache/rss2` en desktop, app dir en móvil) con límite (ej. 200MB LRU).
- Las imágenes wiki se sirven desde `/api/wiki-images/`; la app debe descargarlas y cachearlas igual.
- **Lazy-loading** ya presente en las tarjetas; optimizar con `srcset`/formatos modernos.
- En móvil: preferir miniatura de 400px para ahorrar datos (sugerir añadir parámetro de tamaño al backend).

### 6.5 Favoritos: de localStorage a sincronizados

El frontend actual guarda favoritos en `localStorage` (objetos completos). Para offline-first real:
1. **Añadir endpoints al backend:** `GET/POST/DELETE /api/favorites` sobre la tabla `favoritos` existente (no utilizada por la UI actual).
2. La app usa la tabla local `favorites` + `outbox`.
3. Migrar los favoritos existentes de `localStorage` al SQLite local en el primer arranque de la app.

### 6.6 Backup y Exportación Local

- Desktop: exportar favoritos/caché a archivo (botón nativo save dialog).
- Sustituir `URL.createObjectURL`+`<a download>` por API nativa:
  - Tauri: `invoke('save_dialog')` + `fs.write_file`.
  - Capacitor: `Filesystem.writeFile` a `DIRECTORY.DOCUMENTS` + `Share.share` con el URI.
- Backup servidor (`/admin/backup`) sigue disponible solo en modo admin.

---

## 7. Estrategia de IA/ML en el Dispositivo

### 7.1 Realidad de los Modelos

| Modelo | Tamaño | Desktop (x64) | Android/iOS |
|---|---|---|---|
| NLLB-200 600M (CT2 int8) | ~600MB | ✅ viable (2-4 GB RAM) | ❌ excesivo para la mayoría |
| NLLB-200 1.3B (float16) | ~2.6GB | ✅ con GPU 8GB+ | ❌ |
| MiniLM embeddings | ~470MB | ✅ | ⚠️ borde |
| spaCy es_core_news_lg | ~500MB | ✅ | ❌ |
| Mistral-7B | ~4GB | ⚠️ solo con GPU | ❌ |

### 7.2 Modelo por Capas (tiered)

**Capa 0 (obligatoria, sin IA local):** la app **solo consume la API**. Toda la IA corre en el servidor. Es el modo por defecto para móvil.

**Capa 1 (desktop con GPU — modo worker):** el desktop instala (bajo demanda, descarga de HuggingFace con opción de "descargar ahora/diferir") el modelo NLLB CTranslate2. Se conecta al servidor por WebSocket (`/ws/worker`) como **worker de traducción remota**. Implementación: **portar `remote_translator_worker.py`** a un servicio interno de la app.

Opciones de implementación del worker desktop:
- **A) Embed Python:** empaquetar el intérprete Python (PyInstaller) con `ctranslate2`+`transformers` como **proceso sidecar** de la app Tauri. Rápido de portar (reutiliza el .py tal cual) pero binarios grandes (300-500MB) y fragilidad de CTranslate2 en empaquetado.
- **B) Port a Rust `ctranslate2-rs`:** binario único pequeño, sin Python. Más trabajo pero mucho más robusto y limpio. **Recomendado a medio plazo.**
- **C) Go + cgo a CTranslate2 C API:** intermedio.

**Recomendación:** empezar con **(A)** (time-to-market) y migrar a **(B)** como mejora de Fase 5.

### 7.3 IA On-Device Móvil (opcional, futuro)

Si se desea búsqueda semántica o traducción offline en móvil:
- **Traducción:** modelos ONNX/ML Kit (NLLB no es práctico; alternativas: `google` ML Kit translate offline ~80 idiomas, pesos menores).
- **Embeddings:** ONNX Runtime Mobile con MiniLM cuantizado (int8, ~40MB).
- **NER:** string matcher local (como el worker `topics` de Go) en lugar de spaCy.

Estos se descargan **bajo demanda** con consentimiento del usuario (política de tiendas).

### 7.4 Categorización LLM y Búsqueda Semántica

- **Categorización LLM** (Ollama/Mistral): permanece en servidor; la app solo muestra resultados.
- **Búsqueda semántica:** el stub de la API deberá implementarse en el servidor consultando Qdrant (Fase 1 de servidor, independiente de la app). La app simplemente usará el endpoint cuando exista.

---

## 8. Núcleo Embebido (Embedded Backend)

### 8.1 Contexto

Para la **Fase 5** (modo avanzado desktop y despliegues domésticos), se propone ejecutar el backend sin Docker: un **binario único** que integre API + workers sobre un almacén local.

**Decisión de escala:** dado que el sistema acumulará millones de noticias, el almacén del núcleo embebido debe ser **PostgreSQL embebido**, NO SQLite. PostgreSQL embebido mantiene paridad total con el servidor (mismo esquema, mismos workers, `FOR UPDATE SKIP LOCKED`, tsvector/GIN, arrays, `pg_dump`). SQLite queda **exclusivamente como caché de cliente** (§6) y solo se ofrece como "modo limitado" para despliegues personales pequeños con techo documentado (§8.5).

### 8.2 Estrategia de Almacén del Núcleo Embebido

**Opción A (RECOMENDADA): PostgreSQL embebido.**

| Aspecto | Detalle |
|---|---|
| Librería | `github.com/fergusstrange/embedded-postgres` (descarga/arranca un binario portable de Postgres) o `zonky`/`otj-pg-embedded`. |
| Compatibilidad | 100% con el esquema `init-db/*.sql` actual; mismo `DATABASE_URL`; los 6 workers Go y los handlers no cambian. |
| Backup/restore | Se reutilizan `pg_dump`/`psql` ya implementados en `internal/handlers/admin.go`. |
| Concurrencia | Soporta los 100 workers del ingestor y `FOR UPDATE SKIP LOCKED` de traducción. |
| Coste de empaquetado | +~50–80MB de binarios Postgres por SO (no es pesado para desktop). |
| Escala | Capaz de albergar millones de noticias sin cambio de diseño. |

El binario de Postgres se registra como **sidecar** adicional de Tauri (junto al núcleo Go) y Rust lo arranca/supervisa antes de lanzar la API. Datos en el directorio de la app (`%APPDATA%/rss2/pgdata`). Puerto loopback (5432 → uno efímero) y credenciales aleatorias generadas en el primer arranque.

**Opción B: "Modo limitado" SQLite (solo personal, con techo).**
- Válido únicamente para el perfil "usuario doméstico con feeds personales".
- **Techo duro:** ≤ 200.000 noticias, ≤ 5 feeds activos, sin búsqueda semántica (solo FTS5 de texto), sin clustering/related.
- Debe existir **ruta de migración** al superar el techo: exportar a `pg_dump`/archivo SQL e importar en un servidor PostgreSQL o en el modo A. Documentar en la UI ("Tu catálogo local ha superado el modo limitado").
- Riesgo asumido: reescritura de consultas Postgres-específicas (`unnest`, `FOR UPDATE SKIP LOCKED`, `generate_series`) para SQLite.

**Recomendación:** implementar **solo la Opción A** en la Fase 5. Mantener SQLite únicamente como caché de cliente. Si el equipo quiere abaratar el modo personal, la Opción B puede añadirse después como mejora, nunca como base.

### 8.3 Componentes del Binario Único

```
rss2-core (un solo binario Go)
├── API REST (Gin, puerto 127.0.0.1:8080) — rutas /api completas para la app
├── Motor PostgreSQL embebido (sidecar portable, arrancado por Rust) o Postgres externo
├── Módulo de ingesta RSS (port del rss-ingestor-go) — goroutine con ticker
├── Módulo wiki (port de wiki_worker) — goroutine
├── Módulo topics (port de topics) — goroutine
├── Módulo langdetect (Go) — goroutine
├── Módulo NER ligero (string matcher) — goroutine
└── Qdrant embebido (sidecar, opcional) o búsqueda vectorial desactivada
```

**Configuración:** archivo `rss2.json` en el directorio de configuración del SO (`%APPDATA%/rss2`, `~/.config/rss2`, `~/Library/Application Support/rss2`). Generado en el primer arranque con secretos aleatorios.

### 8.4 Gestión de Procesos en la App

- **Arranque:** Tauri sidecar arranca el binario con `--serve` y espera el healthcheck (`GET /health`).
- **Supervisión:** Rust monitoriza el proceso; auto-restart tras crash (máx. 3 reintentos, backoff exponencial).
- **Shutdown ordenado:** señal SIGTERM → shutdown graceful del servidor Gin.
- **Logs:** `stderr` redirigido a `rss2.log` rotativo.
- **Seguridad:** bind solo a `127.0.0.1`; token de sesión local; CORS restringido a `tauri://localhost` y `https://app.rss2.local`.

### 8.5 Descarga del Núcleo (Opcional en Desktop)

El paquete del núcleo (binario Go ≈30–50MB + binarios PostgreSQL portable ≈50–80MB) puede **descargarse bajo demanda** desde el servidor de releases (GitHub Releases) con checksum SHA-256 verificado, e instalarse en el directorio de la app. Si el usuario no descarga el núcleo, la app funciona en modo cliente puro (Capa 0).

---

## 9. Rediseño de la API para Clientes Nativos

Cambios mínimos necesarios en el backend Go para soportar clientes nativos de forma robusta.

### 9.1 Autenticación con Refresh Tokens

El JWT actual expira en 24h fijas y la app móvil no puede pedir re-login cada día.

1. Añadir tabla `refresh_tokens (id, user_id, token_hash, expires_at, revoked_at, device_info, created_at)`.
2. Nuevos endpoints:
   - `POST /api/auth/refresh` (con refresh token → nuevo par).
   - `POST /api/auth/logout` (revoca refresh).
3. Usar `JWT_EXPIRATION` config (arreglar el hardcode de `internal/auth/jwt.go`).
4. La app guarda el refresh token en SecureStorage/Keychain y lo renueva de forma transparente.

### 9.2 Endpoints Nuevos Requeridos

| Endpoint | Propósito |
|---|---|
| `GET/POST/DELETE /api/favorites` | Sincronización de favoritos (tabla `favoritos` ya existe) |
| `GET /api/news?since=<ISO>` | Pull incremental para sync offline |
| `GET /api/news/feed` (opcional) | Feed agregado optimizado para móvil (paginado con cursor) |
| `GET /api/push/config` + `POST /api/push/register` | Registrar token FCM/APNs por usuario |
| `POST /api/alerts/:id/read` (ya existe) | — |
| `GET /api/version` | Informa versión de API para compatibilidad de la app |

### 9.3 Notificaciones Push

- **Servidor:** nuevo endpoint para registrar token de push; worker Go que consulta `alertas` nuevas y dispara notificaciones.
- **Android:** FCM (`firebase-messaging`); **iOS:** APNs.
- **Desktop:** notificaciones del sistema vía plugin Tauri (sin infraestructura push; eventos en tiempo real por WebSocket/polling ligero).
- **Canales:** "Nuevas alertas", "Nueva traducción disponible" (opcional), "Estado de workers" (opcional, solo admin).

### 9.4 WebSocket para Tiempo Real

- Actualmente solo `/ws/worker` (workers). Añadir un **canal `/ws/client`** opcional con auth JWT para: notificación de nuevas noticias, actualización de alertas en vivo, estado de traducción. La app puede degradar a polling cada 60s si el WebSocket no está disponible.

### 9.5 Versionado de API

- Montar la API bajo `/api/v1` (manteniendo `/api` como alias por compatibilidad web).
- La app envía `Accept-Version: 1` y `User-Agent: rss2-desktop/1.4.2` para trazabilidad y rate-limiting diferenciado.

### 9.6 CORS y Seguridad

- La app nativa no está sujeta a CORS (sin origin de navegador), pero Tauri WebView sí lo está: incluir `tauri://localhost`, `http://localhost:*` y `https://*.rss2.local` en `ALLOWED_ORIGINS`.
- **HTTPS obligatorio** para conexión a servidores remotos (la app rechaza `http://` salvo `localhost`).
- **Certificado pinning** opcional en móvil para servidores propios.

### 9.7 Rediseño de Funciones Admin Dependientes de Docker

Los endpoints `/admin/workers/start|stop|status` ejecutan `docker compose`. Para la app:
- **Modo cliente:** mostrar solo estado reportado por el servidor; ocultar controles de contenedores en móvil.
- **Modo núcleo embebido:** los controles mapean a APIs de gestión del proceso interno (no Docker).

---

## 10. Migración del Frontend Paso a Paso

### 10.1 Fase 0 — Refactor de base (sin cambios visuales)

1. **Extraer `api-client`:** mover `services/api.ts` a un paquete independiente con inyección de `baseURL`, transporte y almacenamiento.
2. **Crear `storage.ts` (abstracción):** interfaz `SecureStore { get, set, remove }` con implementaciones: `localStorage` (web/dev), `Keyring`/`tauri-plugin-store` (desktop), `Preferences`+`SecureStorage` (móvil).
3. **Sustituir `localStorage`** (token/user/favorites) por la abstracción.
4. **Unificar fetch:** envolver las páginas que usan `api` crudo y `useEffect` para que usen `apiService` + React Query (Populares, Alertas, Analisis, Admin*).
5. **Arreglar deudas menores:** `btn-danger`, filtros muertos de Search, `Account` persistente, limpiar imports muertos (Favorites, clsx), implementar `cn()`.
6. **Añadir i18n:** `react-i18next` con diccionarios ES/EN; marcar todos los strings.
7. **Introducir router portable:** pasar de `BrowserRouter` a un wrapper que use `BrowserRouter` en web y `HashRouter` en móvil (detectable por variable de compilación).

### 10.2 Fase 1 — Abstracción de APIs de navegador

| Función | Abstracción (`platform/`) | Desktop (Tauri) | Móvil (Capacitor) |
|---|---|---|---|
| Descarga de archivos | `saveFile(name, bytes)` | `invoke('save_dialog')` + fs | `Filesystem.writeFile` + `Share` |
| Diálogos | `confirmDialog(msg)` | `invoke('confirm')` (Rust nativo) | modales React (no window.confirm) |
| Notificación | `notify(title, body)` | plugin notification | plugin push/local |
| Conectividad | `onNetworkChange(cb)` | eventos Rust | plugin network |
| Fecha/hora | date-fns (ya portable) | — | — |
| Apertura de enlaces | `openExternal(url)` | plugin opener | `Browser.open` (in-app browser) |

### 10.3 Fase 2 — Adaptaciones móviles

1. **Nav responsive:** sustituir `hidden sm:flex` por un componente de navegación con hamburguesa / bottom-tab en móvil.
2. **WikiTooltip táctil:** añadir soporte de tap (primer tap muestra, segundo navega) — reemplazar hover-only.
3. **Tablas** (Feeds, Alertas, Admin): en móvil, convertir a listas de tarjetas en vez de `overflow-x-auto`.
4. **LineChart:** `min-w-[560px]` → rescalar a viewBox responsivo (eliminar scroll horizontal).
5. **Safe areas:** padding con `env(safe-area-inset-*)`.
6. **Pull-to-refresh** en listas (react-query + plugin de refresco nativo).
7. **Infinite scroll / paginación** para listas de noticias en lugar de paginación numérica.

### 10.4 Fase 3 — Offline y Sincronización

1. Integrar `storage`/SQLite local (`tauri-plugin-sql`, `@capacitor-community/sqlite`).
2. Implementar el motor de sync del §6.3.
3. Convertir las consultas a React Query para servir primero de caché local (`placeholderData`/`initialData` del SQLite) y refrescar del servidor.
4. Estado de conexión: banner "Modo offline" + cola de acciones pendientes.

### 10.5 Fase 4 — Shells Nativos

**Tauri (desktop):**
- `npm create tauri-app` + config en `app/desktop/src-tauri`.
- Plugins: shell (sidecar), sql, store, notification, autostart, updater.
- `tauri.conf.json`: `externalBin` → `binaries/rss2-core-*`, ventana 1024×700, capacidades/perms restringidas.
- Build matrix (ver §12).

**Capacitor (móvil):**
- `npx cap init` + `npx cap add android && npx cap add ios`.
- Plugins listados en §5.3.
- `capacitor.config.ts`: `webDir: dist`, `androidScheme: https`.
- Firma de APK/AAB con keystore; provisioning para iOS.

### 10.6 Gestión de Sesión en la App

- Flujo: login → guardar access token (memoria + SecureStorage) y refresh token (SecureStorage).
- Interceptor: 401 → intentar refresh → reenviar petición → si falla, logout.
- Almacenar `user` (JSON) en SecureStorage y sincronizar estado de sesión por eventos internos (en vez de `storage` event del navegador).

---

## 11. Empaquetado y Distribución por Plataforma

### 11.1 Windows

- **Formato:** NSIS installer (`.exe`) vía Tauri bundler; MSI opcional (WiX).
- **Firma:** certificado Authenticode (EV recomendado) con `signtool` en CI (Secret: `WINDOWS_CERT_BASE64`, `WINDOWS_CERT_PASSWORD`).
- **SmartScreen:** firmar y/o añadir reputación; la primera versión no firmada mostrará aviso.
- **Actualizaciones:** `tauri-plugin-updater` con endpoint de manifest (GitHub Releases).
- **WebView2:** Runtime instalado por defecto en Win10/11; incluir fallback de instalación silenciosa.
- **Instalación por usuario:** sin privilegios admin (per-user), o instalador system-wide opcional.

### 11.2 Linux

- **Formatos:** `.deb` (Debian/Ubuntu), `.rpm` (Fedora), **AppImage** (universal, recomendado para testing), **Flatpak** (distribución a largo plazo), `.tar.gz` portable.
- **WebKitGTK:** dependencia `webkit2gtk-4.1` + `librsvg`; documentar en README del paquete.
- **Firma:** GPG para repos; Flatpak vía Flathub (si se desea).
- **Actualizaciones:** AppImageUpdate (fork de AppImage) o el updater de Tauri con endpoint propio.

### 11.3 Android

- **Formato:** **AAB** (App Bundle) para Google Play; APK firmado para distribución directa.
- **Firma:** keystore `app-release.keystore` (guardar en CI como secret; **nunca en git**).
- **minSdk:** 24 (Android 7.0+) recomendado por Capacitor 6 (minSdk 22+).
- **Play Console:** política de "descarga de modelos on-demand" documentada; contenido de noticias = "News" (requiere moderación de contenido si es social).
- **Permisos:** `INTERNET`, `POST_NOTIFICATIONS` (Android 13+), `VIBRATE`.
- **Distribución alternativa:** F-Droid / directa (APK en el sitio).

### 11.4 iOS

- **Formato:** `.ipa` vía App Store Connect (requiere Mac + Xcode + cuenta Apple Developer $99/año).
- **Firma:** certificado Apple Distribution + provisioning profile (secrets en CI macOS).
- **App Store:** política de contenido de noticias; pantalla de "primeros pasos"; descarga de modelos bajo demanda.
- **Notarización** de macOS (si en el futuro se soporta macOS) — fuera de alcance inicial (solo Win/Linux desktop).
- **Actualización:** OTA solo vía App Store (Capgo no funciona con el App Store para updates; requeriría MDM/supervisión). Descartar OTA en iOS.

### 11.5 Política de Actualizaciones

| Plataforma | Mecanismo |
|---|---|
| Windows | tauri-plugin-updater (manifest JSON en GitHub Releases) |
| Linux | tauri-plugin-updater (AppImage) / repos apt/Flatpak |
| Android | Play Store + directa (APK) + opcional Capgo |
| iOS | App Store (única vía) |

### 11.6 Tamaños Estimados del Instalador

| Plataforma | Sin núcleo | Con núcleo embebido | Con worker Python (GPU) |
|---|---|---|---|
| Windows | ~25MB | ~60MB | ~400MB |
| Linux | ~25MB | ~60MB | ~400MB |
| Android | ~15MB | — | — |
| iOS | ~15MB | — | — |

---

## 12. Infraestructura de Build y CI/CD

### 12.1 Pipeline GitHub Actions (matrix)

```yaml
name: build-app
on: [push, pull_request, workflow_dispatch]

jobs:
  web:
    runs-on: ubuntu-latest
    steps:
      - checkout
      - setup-node, npm ci
      - npm run build            # tsc + vite build (dist/)
      - npm run test             # vitest (nuevos tests)
  desktop-windows:
    needs: web
    runs-on: windows-latest
    steps:
      - npm ci
      - cargo build --release (Tauri)
      - tauri build --bundles nsis
      - firma (signtool) + upload artifact
  desktop-linux:
    needs: web
    runs-on: ubuntu-latest
    steps:
      - apt: libwebkit2gtk-4.1-dev, libgtk-3-dev, libayatana-appindicator3-dev
      - tauri build --bundles deb,appimage
      - upload artifact
  android:
    needs: web
    runs-on: ubuntu-latest
    steps:
      - java 17, Android SDK
      - npx cap sync android
      - gradlew assembleRelease (AAB + APK)
      - upload artifact
  ios:
    needs: web
    runs-on: macos-14
    steps:
      - npx cap sync ios
      - xcodebuild archive + export
      - (opcional) upload to TestFlight via App Store Connect API
  backend-core:
    runs-on: matrix (windows, ubuntu)
    steps:
      - go build -o rss2-core-{os}-{arch} ./cmd/rss2-core  # nuevo binario
      - upload artifact
  release:
    if: startsWith(github.ref, 'refs/tags/v')
    needs: [desktop-windows, desktop-linux, android, backend-core]
    steps:
      - download artifacts
      - create GitHub Release con checksums (sha256sum)
      - publish manifest.json del updater
```

### 12.2 Versionado

- **Semver** estricto (`1.4.2`). El `package.json` (app) y el manifiesto del updater comparten versión.
- Tag `v*` dispara la release. `main` dispara builds de desarrollo (artifacts sin firmar).

### 12.3 Secretos en CI

| Secret | Uso |
|---|---|
| `WINDOWS_CERT_BASE64` / `WINDOWS_CERT_PASSWORD` | Firma Authenticode |
| `ANDROID_KEYSTORE_BASE64` / `ANDROID_KEYSTORE_PASSWORD` / `ANDROID_KEY_PASSWORD` / `ANDROID_KEY_ALIAS` | Firma AAB/APK |
| `APPLE_DIST_CERT_BASE64` / `APPLE_PROVISIONING_BASE64` | Firma iOS |
| `GH_TOKEN` | Publicar releases + manifest |
| `FCM_SERVER_KEY` / `APNS_KEY` | Push (si se testea desde CI) |

---

## 13. Plan de Implementación por Fases

### Fase 0 — Preparación y Refactor (2–3 semanas)
**Objetivo:** base de código portable sin cambios de comportamiento.
- [ ] Extraer `api-client` y `storage` como paquetes.
- [ ] Sustituir `localStorage` por `SecureStore`.
- [ ] Unificar fetch (apiService + React Query) en todas las páginas.
- [ ] Arreglar deudas técnicas §2.3 (btn-danger, Search, Account, Favorites, i18n, router portable).
- [ ] Añadir tests base (vitest + Testing Library) para `api-client`, `storage` y componentes clave.
- **Entregable:** `npm run build` + `npm test` verde; la web sigue funcionando igual en Docker.

### Fase 1 — Backend: Refresh Tokens + API para móvil (2 semanas)
- [ ] Tabla `refresh_tokens` + endpoints `/auth/refresh`, `/auth/logout`.
- [ ] Arreglar `JWT_EXPIRATION` en `internal/auth/jwt.go`.
- [ ] Endpoints `/api/favorites` (GET/POST/DELETE).
- [ ] `GET /api/news?since=` (pull incremental).
- [ ] `/api/version`.
- [ ] Implementar `SemanticSearch` real contra Qdrant (opcional pero recomendado).
- [ ] Tests Go para los nuevos endpoints (patrón existente en `internal/handlers/auth_test.go`).
- **Entregable:** API versionada y consumible por clientes nativos.

### Fase 2 — Shell Desktop Tauri (Windows/Linux) — MVP (3–4 semanas)
- [ ] Proyecto Tauri + integración del `dist/` de Vite.
- [ ] Plugins: store, notification, opener, updater.
- [ ] Abstracciones `platform/` (saveFile, confirmDialog, notify).
- [ ] Login/registro + sesión persistente con refresh.
- [ ] Pantallas principales: Home, News, Search, Favorites, Feeds (read-only), Stats.
- [ ] Instaladores NSIS (Win) y deb/AppImage (Linux).
- **Entregable:** instalador de escritorio funcional conectado a un servidor central.

### Fase 3 — Shell Móvil Capacitor (Android/iOS) (4–5 semanas)
- [ ] Proyecto Capacitor + sync de la SPA.
- [ ] Plugins: preferences, secure-storage, filesystem, share, network, push, splash, status-bar.
- [ ] HashRouter + adaptaciones móviles §10.3 (nav, tooltips, tablas, chart).
- [ ] Push notifications (FCM/APNs) + endpoint `/push/register`.
- [ ] Builds Android (AAB) e iOS (ipa) en CI.
- **Entregable:** apps en Android e iOS instalables (TestFlight/Play internal testing).

### Fase 4 — Offline-First y Sincronización (3–4 semanas)
- [ ] SQLite local (desktop + móvil) con esquema §6.2.
- [ ] Motor de sync (outbox + pull incremental) §6.3.
- [ ] Favoritos sincronizados (backend + app).
- [ ] Caché de imágenes con límite LRU.
- [ ] Modo offline de lectura + banner de estado.
- [ ] Pull-to-refresh e infinite scroll.
- **Entregable:** app funcional offline con reconciliación al reconectar.

### Fase 5 — Núcleo Embebido y Modo Worker (5–8 semanas)
- [ ] Binario `rss2-core` (Go) + **PostgreSQL embebido portable** (Opción A) con el esquema `init-db/*.sql` intacto. **Descartado SQLite como almacén principal** (escala de millones); SQLite queda solo como caché de cliente.
- [ ] Sidecar Postgres (arranque/supervisión/shutdown por Rust) + sidecar del núcleo.
- [ ] Port de ingesta RSS, wiki, topics, langdetect al núcleo.
- [ ] Supervisión de procesos: auto-restart, logs rotativos, healthcheck.
- [ ] Configuración local `rss2.json` + wizard de "Configurar servidor local".
- [ ] Descarga bajo demanda del núcleo y de Postgres portable (checksum verificado).
- [ ] Modo worker: port de `remote_translator_worker.py` (opción A: PyInstaller sidecar; mejora B: Rust).
- [ ] Gestión de workers remotos en la UI (crear API key, ver estado) — ya existe en AdminWorkers.
- **Entregable:** desktop autónomo (sin Docker, a escala de millones vía Postgres embebido) + aporte de GPU a la red de traducción.

### Fase 6 — Distribución y Tiendas (2–3 semanas)
- [ ] Firmas de producción (Windows, Android, iOS).
- [ ] Publicación Play Store + App Store + GitHub Releases.
- [ ] Manifiesto de auto-update y roll-out gradual.
- [ ] Dashboard de métricas de uso (opcional: telemetría anonimizada).
- [ ] Documentación de usuario final por plataforma.
- **Entregable:** releases públicas en las 4 plataformas.

### Cronograma Total Estimado
| Fase | Duración | Acumulado |
|---|---|---|
| 0 | 2–3 sem | 3 sem |
| 1 | 2 sem | 5 sem |
| 2 | 3–4 sem | 9 sem |
| 3 | 4–5 sem | 14 sem |
| 4 | 3–4 sem | 18 sem |
| 5 | 5–8 sem | 26 sem |
| 6 | 2–3 sem | 29 sem |

**~7 meses** con un equipo de 2-3 personas (1 full-stack React/TS, 1 Go/backend, 0.5 Rust/mobile). El MVP (Fases 0-2) se puede alcanzar en **~2 meses**.

---

## 14. Riesgos y Mitigaciones

| # | Riesgo | Probabilidad | Impacto | Mitigación |
|---|---|---|---|---|
| 1 | **Empaquetado de CTranslate2/Python en desktop** (dependencias .so, torch cu118) | Alta | Alto | Empezar con PyInstaller sidecar; roadmap hacia port Rust `ctranslate2-rs`; documentar el `patchelf --clear-execstack` existente. |
| 2 | **Políticas de tiendas (iOS/Android)** por descarga de modelos grandes | Media | Alto | Modelos **on-demand** con consentimiento; nunca en el APK; Capa 0 (sin IA local) como default móvil. |
| 3 | **Coste de mantenimiento de 3 shells** (web, Tauri, Capacitor) | Alta | Medio | Mantener **una sola SPA**; los shells son delgados; abstracciones `platform/` estrictas; tests de contrato. |
| 4 | **JWT/sesión en móvil** (24h fija, sin refresh) | Alta | Medio | Fase 1 implementa refresh tokens; SecureStorage para los tokens. |
| 5 | **SQLite ≠ PostgreSQL** en el núcleo embebido | Baja (decisión: usar PostgreSQL embebido) | Bajo | El núcleo usa **PostgreSQL portable** con el esquema actual intacto; SQLite queda solo como caché de cliente acotada. Si se añade "modo limitado" SQLite, llevar techo duro documentado + ruta de exportación a Postgres. |
| 6 | **Favoritos sin backend** (localStorage hoy) | Alta | Baja | Fase 1 añade endpoints; migración de datos local en primer arranque. |
| 7 | **Notificaciones push requieren backend** nuevo (FCM/APNs) | Media | Medio | Worker Go de push; empezar por alertas; degradar a notificaciones locales en desktop. |
| 8 | **Rendimiento del WebView** en dispositivos Android baratos | Media | Medio | Caché SQLite + infinite scroll; reducir tamaño de bundle (code-splitting); limitar tarjetas cargadas. |
| 9 | **Seguridad** (almacenar refresh token, pinning) | Media | Alta | Keychain/Keystore/Keyring; HTTPS obligatorio; opción de pinning; auditoría de secrets. |
| 10 | **Traducción de toda la UI (i18n)** retrasa fases | Media | Baja | Empezar i18n en Fase 0 (ES como default), EN incremental. |
| 11 | **Escasez de expertise Rust/mobile** | Media | Media | Usar Capacitor (JS puro, sin Swift/Kotlin deep); Rust mínimo en Tauri; plantilla de Tauri como base. |
| 12 | **`SemanticSearch` stub** — expectativa de búsqueda semántica en la app | Media | Medio | Implementarla en servidor (Fase 1 opcional) o desactivar el toggle en la app. |

---

## 15. Estrategia de Testing y QA

### 15.1 Niveles de Testing

| Nivel | Stack | Alcance |
|---|---|---|
| Unit (TS) | vitest + Testing Library | `api-client`, `storage`, `sync`, componentes, hooks |
| Unit (Go) | `go test` | nuevos endpoints (favorites, refresh, push), helpers |
| Integration API | vitest + msw / Go httptest | contratos `api-client` ↔ backend |
| E2E web | Playwright | flujos principales (login, home, search, favorites) |
| E2E desktop | Playwright con WebView (Tauri driver) | humo por plataforma |
| E2E móvil | Detox / Maestro (emulador) | login, offline, push (móvil) |
| Visual | Percy/regression | 4 plataformas, light/dark |
| Rendimiento | Lighthouse (web) / bundle-size budget | CI gate: bundle < 250KB gzip core |

### 15.2 Cobertura Mínima Obligatoria

1. `api-client`: cada endpoint con mock (éxito, 401→refresh→retry, 401→logout, errores de red).
2. `storage`: round-trip, corrupción, rotación de versión.
3. `sync`: outbox vacío, envío, fallo→reintento, conflictos.
4. `favorites`: flujo local→sync→servidor.
5. Componentes clave: `Layout`, `ProtectedRoute`, tarjetas de noticia, `WikiTooltip` táctil.

### 15.3 Testing Manual por Plataforma (checklist)

- [ ] **Windows:** instalación NSIS, auto-update, notificaciones, tray, WebView2 ausente→instalar, SmartScreen.
- [ ] **Linux:** AppImage/deb en Ubuntu+Debian+Fedora, webkit2gtk instalado, escalado HiDPI.
- [ ] **Android:** AAB instalado, push FCM, permiso notificaciones, orientación, modo offline/avión, background.
- [ ] **iOS:** TestFlight, push APNs, permiso notificaciones, Safe Area, Dark Mode.

---

## 16. Referencias y Documentación

- [README.md](./README.md) — Guía de despliegue completo del sistema actual.
- [DEPLOY.md](./DEPLOY.md) — Instrucciones de producción.
- [AGENTS.md](./AGENTS.md) — Guía de desarrollo del repositorio.
- [remote-worker.md](./remote-worker.md) — Protocolo de workers remotos (base para el módulo worker desktop).
- [SECURITY_GUIDE.md](./SECURITY_GUIDE.md) — Guía de seguridad (a revisar para almacenamiento de tokens).
- [QUICKSTART_LLM.md](./QUICKSTART_LLM.md) — Configuración LLM.
- **Ficheros clave del código analizado:**
  - `docker-compose.yml` — 20 servicios, topología de redes, recursos.
  - `backend/cmd/server/main.go` — arranque, rutas, migraciones idempotentes.
  - `backend/internal/handlers/*.go` — toda la superficie de API.
  - `backend/internal/auth/jwt.go` — JWT (24h hardcode).
  - `backend/internal/services/ml.go` — clientes ML (SemanticSearch stub).
  - `frontend/src/services/api.ts` — contrato API + interceptor 401.
  - `frontend/src/App.tsx`, `main.tsx` — boot y routing.
  - `frontend/src/pages/*.tsx` — las 18 páginas.
  - `workers/remote_translator_worker.py` — prototipo del modo worker desktop.
  - `workers/ctranslator_worker.py`, `embeddings_worker.py`, `ner_worker.py` — pipelines IA.
  - `rss-ingestor-go/main.go` — ingesta con politeness y ETag.
  - `init-db/*.sql` — esquema completo (28 tablas).
  - `nginx.conf` — gateway.

---

### Apéndice A — Tabla de Correspondencia Frontend↔App

| Página actual | Estado | Acción en la app |
|---|---|---|
| Home | ✅ | Port directo + infinite scroll |
| News | ✅ | Port + favorito offline |
| Search | ⚠️ | Corregir filtros muertos; semántica condicional |
| Feeds | ✅ | CRUD (solo desktop/admin) |
| Favorites | ⚠️ | Migrar de localStorage a sync |
| Account | ⚠️ | Implementar persistencia real |
| Login | ✅ | Añadir refresh tokens |
| Populares | ✅ | Port |
| Analisis | ✅ | Chart responsivo |
| Alertas | ✅ | + push notifications |
| WelcomeWizard | ✅ | + wizard "configurar servidor local" (Fase 5) |
| AdminAliases/Users/Settings/Workers | ⚠️ | Ocultar/adaptar acciones dependientes de Docker en móvil |

### Apéndice B — Glosario

- **Sidecar:** binario adicional empaquetado junto a la app y lanzado por el shell (ej. el backend Go embebido).
- **Offline-first:** diseño donde la app funciona sin red usando datos locales y sincroniza después.
- **Outbox:** cola local de escrituras pendientes de confirmar por el servidor.
- **Capa 0/1/2 de IA:** modelo de capacidades de IA según el dispositivo (cliente puro / worker GPU / núcleo local).
- **FTS5:** módulo de búsqueda full-text de SQLite (equivalente al tsvector de Postgres).
