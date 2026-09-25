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

## Operación

- Backups diarios: `./backup.sh` (retención 30 días, descarta dumps fallidos).
- Rotar password Postgres: `ALTER USER` + recreate (nunca borrar `data/pgdata`).
- Cobertura de ingesta (admin): `GET /api/admin/ingest/stats`.
- Rate limit: `GET /api/stats/ratelimit`.
- Sintéticos: `?semantic=true` requiere embeddings externos (no activo).

## Documentación de diseño

Fuente de verdad, por orden: esquema `init-db/` → `docker-compose.yml` →
contratos HTTP (`backend/`, `frontend/src/services/api.ts`) → `specs/`.

- `specs/constitution.md` — reglas obligatorias de cambio.
- `specs/01-system-overview/` … `specs/08-operations/` — comportamiento verificado.
- `specs/plan.md`, `specs/tasks.md` — backlog y estado.
- `remote-worker/README.md` — worker remoto de traducción (WebSocket).
