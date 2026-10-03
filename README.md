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
- `user_lists` — listas de noticias creadas por usuario
- `user_list_items` — items dentro de cada lista
- `user_saved_searches` — búsquedas guardadas

### API Endpoints

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/favorites` | Obtener favoritos del usuario |
| POST | `/api/favorites/:noticiaId` | Añadir a favoritos |
| DELETE | `/api/favorites/:noticiaId` | Quitar de favoritos |
| GET | `/api/favorites/:noticiaId/check` | Verificar si es favorito |
| GET | `/api/lists` | Obtener listas del usuario |
| POST | `/api/lists` | Crear nueva lista |
| PUT | `/api/lists/:id` | Renombrar lista |
| DELETE | `/api/lists/:id` | Eliminar lista |
| GET | `/api/lists/:id/items` | Obtener noticias de una lista |
| POST | `/api/lists/:id/items/:noticiaId` | Añadir noticia a lista |
| DELETE | `/api/lists/:id/items/:noticiaId` | Quitar noticia de lista |
| GET | `/api/saved-searches` | Obtener búsquedas guardadas |
| POST | `/api/saved-searches` | Guardar búsqueda |
| PUT | `/api/saved-searches/:id` | Renombrar búsqueda |
| DELETE | `/api/saved-searches/:id` | Eliminar búsqueda |

### Frontend

- **Favoritos**: marca con ❤️ desde el carrusel de noticias
- **Listas**: pulsa 📁 para añadir a una lista o crear una nueva
- **Búsquedas guardadas**: guarda búsquedas frecuentes en la pestaña Favoritos → Búsquedas
- Requiere autenticación (JWT) para todas las operaciones

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

## Documentación de diseño

Fuente de verdad, por orden: esquema `init-db/` → `docker-compose.yml` →
contratos HTTP (`backend/`, `frontend/src/services/api.ts`) → `specs/`.

- `specs/constitution.md` — reglas obligatorias de cambio.
- `specs/01-system-overview/` … `specs/08-operations/` — comportamiento verificado.
- `specs/plan.md`, `specs/tasks.md` — backlog y estado.
- `remote-worker/README.md` — worker remoto de traducción (WebSocket).
