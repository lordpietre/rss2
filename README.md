# RSS2 — Agregador multilingüe de noticias RSS

Agregador de noticias RSS con traducción automática al español (NLLB),
detección de idioma, NER (spaCy), embeddings locales, búsqueda, alertas
de picos de actividad y frontend React. Un único puerto publicado: **:8888**.

## Arranque rápido

```bash
cp .env.example .env   # y rellena POSTGRES_PASSWORD, REDIS_PASSWORD, SECRET_KEY...
docker compose up -d --build backend   # primero backend (imagen compartida)
docker compose up -d --no-build        # resto de servicios
```

Frontend: http://localhost:8888 · API: http://localhost:8888/api

## Flujo de datos

```
feeds ──ingestor──▶ noticias ──langdetect──▶ lang
  ──translation-scheduler──▶ traducciones(pending) ──translator──▶ done
  ──ner──▶ tags/tags_noticia ──► Populares / Evolución / Alertas
  ──embeddings──▶ traduccion_embeddings ──related──▶ related_noticias
  noticias ──topics──▶ news_topics + pais_id · tags ──wiki──▶ wiki_summary + imagen
  noticias(resumen<200) ──scraper──▶ enriquecimiento · fuentes_url ──discovery──▶ feeds
```

## Arquitectura

### Backend (Go)
- API REST en `backend/cmd/server/main.go` (puerto interno :8080)
- Workers Go compilados como binarios separados en el mismo Dockerfile:
  - `/scraper` — enriquecimiento de resúmenes
  - `/discovery` — descubrimiento de nuevos feeds
  - `/topics` — categorización de noticias
  - `/wiki` — obtención de resúmenes e imágenes de Wikipedia
  - `/related` — cálculo de noticias relacionadas
- Handlers en `backend/internal/handlers/`

### Frontend (React)
- SPA en `frontend/` con Vite + TypeScript + Tailwind
- Páginas principales:
  - `Home.tsx` — portada con carrusel, tendencias, sidebar de sugerencias
  - `Search.tsx` — búsqueda de noticias con filtros
  - `Populares.tsx` — red de entidades populares
  - `Stats.tsx` — Trends Radar con entidades trending
  - `GlobalMonitor.tsx` — Cobertura global con mapa mundial
  - `Analisis.tsx` — Evolución temporal de entidades
  - `Alertas.tsx` — Alertas de picos de actividad

### Workers Python
- `workers/ner/` — spaCy NER (entidades nombradas)
- `workers/langdetect/` — detección de idioma
- `workers/translation-scheduler/` — scheduler de traducciones
- `workers/embeddings/` — sentence-transformers para embeddings
- `translator/` — CTranslate2 con NLLB-200-distilled-600M

## Servicios (`docker-compose.yml`)

| Servicio | Qué hace |
|---|---|
| `db` / `redis` | Postgres 16, caché (sin puertos públicos) |
| `backend` | API Go `:8080` (solo vía gateway) |
| `frontend` + `nginx` | SPA + gateway `:8888` |
| `ingestor` | RSS → `noticias` (20 workers, cada 8 min) |
| `langdetect`, `translation-scheduler`, `translator` | lang → jobs `pending` → NLLB CPU → `done` |
| `ner` | spaCy `es_core_news_lg` → `tags` |
| `embeddings` | sentence-transformers local → `traduccion_embeddings` |
| `related` | relacionadas por coseno SQL sobre embeddings |
| `wiki`, `topics` | resúmenes/imágenes Wikipedia; tópicos + `pais_id` |
| `scraper`, `discovery` | enriquece resúmenes; `fuentes_url` → `feeds`/`feeds_pending` |

Los workers Go (`topics`, `wiki`, `related`, `scraper`,
`discovery`) reutilizan la imagen `backend` con distinto `entrypoint`
(los binarios ya se compilan en `backend/Dockerfile`).

Perfiles opcionales: `gpu` (translator NVIDIA), `monitoring` (prometheus/grafana).

## Entidades y Trends

### Entidades (Populares)

Las entidades se extraen mediante NER (Named Entity Recognition) y se almacenan en las tablas `tags` y `tags_noticia`.

**Parámetros de entidad:**
- `valor` — nombre de la entidad (ej: "Donald Trump", "Barcelona")
- `tipo` — tipo de entidad: `persona`, `organizacion`, `ubicacion`, `tema`

**Endpoints de entidades:**

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/entities` | Lista de entidades con filtros (sort: count, name, recent, last_mentioned) |
| GET | `/api/entities/news?valor=&tipo=` | Noticias que mencionan una entidad |
| GET | `/api/entities/mentions?values=&hours=` | Serie temporal de menciones |

### Trends Radar (Stats.tsx)

La página de Stats muestra las entidades más mencionadas en las últimas 24-72 horas.

**Navegación desde Trends:**
- Al hacer clic en una entidad (ej: "Donald Trump"), navega a `/search?entity=Donald+Trump&tipo=persona`
- La página Search detecta el parámetro `entity` y consulta `/api/entities/news` en lugar del search normal
- El parámetro `tipo` filtra por tipo de entidad

**Timeout de entity news:**
Las consultas de entidades muy populares (ej: "Donald Trump" con miles de menciones) pueden tardar más de 20 segundos. El frontend tiene un timeout de **60 segundos** para estas operaciones.

## Cobertura Global (GlobalMonitor.tsx)

La sección Global Coverage muestra un mapa mundial con la actividad de noticias por país.

### Cómo funciona

1. `GET /api/stats` devuelve `top_countries` con `{ pais_id, pais_nombre, count }`
2. El componente `WorldMap` hace lookup de `pais_nombre` → código ISO usando `COUNTRY_NAME_TO_CODE`
3. Los países se colorean según intensidad: `#72FF8A` (alto), `#36C96B` (medio), `#244C32` (bajo), `#1a2e1f` (ninguno)

### Nombres de países

La tabla `paises` en la base de datos contiene los nombres en español. El lookup soporta:
- Nombres en español: "Estados Unidos", "Reino Unido", "España"
- Nombres en inglés: "United States", "United Kingdom", "Spain"
- Variaciones comunes: "USA", "UK", "méxico", "mexico"

Países soportados: ~40 países principales con paths SVG definidos en `WorldMap.tsx`.

## Alertas de Actividad

Las alertas detectan picos de menciones de entidades usando `ratio` (hits/baseline).

### Endpoint

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/alerts` | Lista de alertas (status: nueva, leida, dismiss) |
| POST | `/api/alerts/:id/read` | Marcar como leída |
| POST | `/api/alerts/:id/dismiss` | Descartar alerta |
| POST | `/api/alerts/read-all` | Marcar todas como leídas |

### Navegación

- Desde Overview: Alerta → `/analisis?entity=...` (Evolución de entidad)
- Desde Stats: Trends → `/search?entity=...` (Noticias de entidad)

## Traducción automática

### Configuración de tokens

El traductor usa CTranslate2 con NLLB-200-distilled-600M. Parámetros configurables:

| Variable | Default | Descripción |
|---|---|---|
| `MAX_SRC_TOKENS` | 1024 | Tokens máximos del texto origen (antes 256) |
| `MAX_NEW_TOKENS` | 1024 | Tokens máximos de la traducción (antes 256) |
| `TRANSLATOR_BATCH` | 128 | Tamaño de lote por idioma |

### Limpieza de texto

Antes de traducir, el texto pasa por `textclean` que elimina:

- **Chrome residual**: `window._taboola`, `window.__taboola`, `googletag.cmd.push`
- **Código JavaScript embebido**: asignaciones pegadas al texto sin separador
- **Puntuación de idiomas asiáticos**: hindi `।`, birmano `။`, guyaratí `॥`

### Chunking y unión de chunks

Textos largos se dividen en chunks usando:
- Puntos y dos puntos como separadores primarios
- Signos de interrogación/exclamación
- Puntuación extendida asiática

Los chunks se unen preservando puntuación final mediante `join_chunks_with_punctuation()`.

### Truncamiento inteligente

En lugar de truncar en bytes, `truncate_at_sentence_boundary()` corta en el último punto completo antes del límite de tokens.

## Operación

- Backups diarios: `./backup.sh` (retención 30 días, descarta dumps fallidos).
- Rotar password Postgres: `ALTER USER` + recreate (nunca borrar `data/pgdata`).
- Cobertura de ingesta (admin): `GET /api/admin/ingest/stats`.
- Rate limit: `GET /api/stats/ratelimit`.
- Sintéticos: `?semantic=true` requiere embeddings externos (no activo).

## Favoritos y Listas

Sistema de favoritos y listas persistentes en Postgres (multidispositivo).

### Tablas

- `user_favorites` — noticias guardadas por usuario
- `user_lists` — listas de noticias creadas por usuario (con campo `keywords`)
- `user_list_items` — items dentro de cada lista
- `user_saved_searches` — búsquedas guardadas (con `last_used` y `use_count`)

### API Endpoints

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/favorites` | Obtener favoritos del usuario |
| POST | `/api/favorites/:noticiaId` | Añadir a favoritos |
| DELETE | `/api/favorites/:noticiaId` | Quitar de favoritos |
| GET | `/api/favorites/:noticiaId/check` | Verificar si es favorito |
| GET | `/api/lists?sort_by=` | Obtener listas (sort: updated_at, created_at, name, item_count) |
| POST | `/api/lists` | Crear nueva lista (con keywords opcionales) |
| PUT | `/api/lists/:id` | Renombrar lista y/o actualizar keywords |
| DELETE | `/api/lists/:id` | Eliminar lista |
| GET | `/api/lists/:id/items?sort_by=` | Obtener noticias (sort: added_at, published_at, title) |
| POST | `/api/lists/:id/items/:noticiaId` | Añadir noticia a lista |
| DELETE | `/api/lists/:id/items/:noticiaId` | Quitar noticia de lista |
| GET | `/api/lists/:id/tags` | Obtener nube de tags de la lista |
| GET | `/api/lists/:id/related` | Buscar noticias relacionadas (auto-actualiza keywords) |
| GET | `/api/lists/suggested` | Noticias sugeridas para sidebar (basadas en keywords) |
| GET | `/api/saved-searches?sort_by=` | Obtener búsquedas (sort: last_used, created_at, use_count, label) |
| POST | `/api/saved-searches` | Guardar búsqueda |
| POST | `/api/saved-searches/:id/use` | Marcar búsqueda como usada (actualiza last_used y use_count) |
| PUT | `/api/saved-searches/:id` | Renombrar búsqueda |
| DELETE | `/api/saved-searches/:id` | Eliminar búsqueda |

### Frontend

- **Favoritos**: marca con ❤️ desde cualquier noticia (carrusel, búsqueda, página individual)
- **Auto-refresh**: los favoritos se actualizan cada 30 segundos y al volver a la pestaña
- **Listas**: pulsa 📁 para añadir a una lista o crear una nueva
- **Ordenamiento**: selector de orden en listas, búsquedas guardadas y entidades populares
- **Keywords**: al crear una lista puedes añadir palabras clave separadas por comas
- **Nube de tags**: en la vista de lista, los tags de las noticias se muestran con interactividad
- **Noticias relacionadas**: el sistema busca automáticamente noticias basadas en los tags más frecuentes
- **Sidebar de sugerencias**: en la portada aparecen noticias sugeridas a los lados (cuando estás logueado)
- **Hover tooltip**: al pasar el ratón 1 segundo sobre una noticia, aparece un bocadillo con el contenido completo
- **Bocadillo de traducciones**: mismo tooltip disponible en resultados de búsqueda
- Requiere autenticación (JWT) para todas las operaciones

### Compartir en Redes Sociales

Botones de compartir disponibles en cada noticia:

- **Facebook**: abre diálogo de compartir con título, resumen y URL
- **LinkedIn**: comparte como actualización de enlace
- **X (Twitter)**: tweet con título y URL
- **Instagram**: copia al portapapeles texto formateado para stories
- **Telegram**: comparte en chats o canales

Para **listas completas**: botón de Telegram que copia todo el contenido de las noticias al portapapeles.

### Algoritmo de Búsqueda Relacionada

1. Se extraen los 10 tags con más menciones de las noticias en la lista
2. Se convierten automáticamente en keywords (ej: "Biden, elecciones, Trump")
3. Se actualiza el campo `keywords` de la lista
4. Se buscan noticias relacionadas usando full-text search de Postgres
5. Las noticias ya presentes en la lista se excluyen

## Re-procesar traducciones

Para re-traducir todas las noticias (ej: tras cambiar límites de tokens):

```sql
-- Resetear todas las traducciones a pending
UPDATE traducciones SET status = 'pending', resumen_trad = NULL WHERE status = 'done';
```

Para limpiar chrome residual de noticias ya ingestadas:

```bash
docker exec rss2_backend sanitize -apply -requeue
```

## Comandos de desarrollo

### Go (backend + rss-ingestor-go)
```bash
make all         # deps + build (full local build)
make deps        # go mod download + tidy (backend/ only)
make build       # build all Go binaries to bin/
make clean       # remove bin/ + go clean
make test        # go test ./... en ambos módulos
make parity      # diff textclean entre backend/ y rss-ingestor-go/
```

### Go workers (run locally with DB env)
```bash
make run-scraper     DB_HOST=localhost DB_PORT=5432 DB_NAME=rss DB_USER=rss DB_PASS=rss
make run-discovery   DB_HOST=localhost DB_PORT=5432 DB_NAME=rss DB_USER=rss DB_PASS=rss
make run-topics      DB_HOST=localhost DB_PORT=5432 DB_NAME=rss DB_USER=rss DB_PASS=rss
make run-related     DB_HOST=localhost DB_PORT=5432 DB_NAME=rss DB_USER=rss DB_PASS=rss RELATED_SLEEP=10
```

### Frontend
```bash
cd frontend
npm test          # vitest
npm run build     # tsc && vite build
```

## Documentación de diseño

Fuente de verdad, por orden: esquema `init-db/` → `docker-compose.yml` →
contratos HTTP (`backend/`, `frontend/src/services/api.ts`) → `specs/`.

- `specs/constitution.md` — reglas obligatorias de cambio.
- `specs/01-system-overview/` … `specs/08-operations/` — comportamiento verificado.
- `specs/plan.md`, `specs/tasks.md` — backlog y estado.
- `remote-worker/README.md` — worker remoto de traducción (WebSocket).
