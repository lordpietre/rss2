# Spec 01 — Visión del sistema (as-built)

## Objetivo

Describir la arquitectura real y verificada de RSS2: agregador de noticias RSS
multilingüe con traducción automática (NLLB), NER, búsqueda y detección de
picos de actividad. Todo lo afirmado aquí está comprobado contra código,
compose y contenedores en ejecución.

## Topología (docker-compose.yml)

```
                        :8888
                          |
                    [nginx]  (único puerto publicado)
                    /       \
            [frontend]   [backend:8080] (cmd: server)
            (nginx+SPA)        |
              red frontend     +-- [db] postgres:16 (sin puertos)
                                +-- [redis] :6379 --requirepass (sin puertos)
                                +-- [translator] :8000 CPU (sin puertos)
                                +-- [ingestor] rss-ingestor-go
                                +-- [translation-scheduler]
                                +-- [langdetect]
                                +-- [ner] (spaCy)
   (2026-09-15: Qdrant + qdrant-worker eliminados — nadie los leía.)
  Perfiles opcionales: translator-gpu (profile gpu, requiere NVIDIA),
                       prometheus/grafana/cadvisor (profile monitoring)
```

Redes: `frontend` (nginx, frontend, backend) y `backend` (todo lo demás).
`backend` está en ambas. DNS entre servicios por nombre de servicio.

## Pipeline de datos (flujo completo verificado)

```
feeds (6822, 1460 activos)
  │  rss-ingestor-go (poll 8 min, 20 workers) → INSERT noticias (id=md5(url))
  ▼
noticias (id VARCHAR(32), titulo, resumen, url, fecha, lang NULL)
  │  langdetect (spaCy? no: lib langdetect) → SET lang
  ▼
translation-scheduler (cada 30 s, batch 2000)
  │  INSERT traducciones (noticia_id, lang_from, lang_to='es', status='pending')
  │  solo si lang IS NOT NULL y lang != 'es'
  ▼
translator local ctranslator_worker.py (polling SQL SKIP LOCKED, 30 s)
  │  NLLB-200 CTranslate2 CPU int8 → titulo_trad/resumen_trad, status='done'
  │  (alternativa: remote workers por websocket /ws/worker → status assigned)
  ▼
ner (spaCy es_core_news_lg sobre traducciones done)
  │  → tags (valor, tipo, apellido) + tags_noticia (pivote)
  ▼
Consumidores de lectura:
  - API news/search/entities (JOIN noticias+traducciones+tags)
  - AlertScanner (en proceso server, cada 120 min): picos vs media 8 días
    → alertas (hace falta >1 día de historia para disparar)
  - Pendientes de activar: topics, related, qdrant-worker, wiki-worker,
    scraper/discovery (ver spec 07)
```

Volúmenes/estado observados: ~31k noticias, ~27k traducciones pending,
~900+ done creciendo, ~31k tags, ~55k tags_noticia.

## Contratos y convenciones

- API base `/api` (gateway `:8888` → `backend:8080`; frontend sirve `/` y
  proxya `/api` a `backend:8080`). OpenAPI parcial vía gin-swagger.
- Auth JWT (HS256, `SECRET_KEY`): login/register abiertos; resto con
  `AuthRequired`; admin con `AdminRequired`. Rotar `SECRET_KEY` invalida
  sesiones (hay que re-loguear; no hay reset de password en la app).
- Rate limit por IP de cliente (requiere trusted proxies Docker; ver spec 05).
- Cache Redis:solo caché reconstruible (`cache.Set` serializa una vez).
- Traducciones `status`: `pending → assigned → done | error` (sin CHECK SQL).
- `noticias.lang CHAR(5)` con padding: comparar con `TRIM`.
- Remote workers: WS `/ws/worker`, `api_key` por query/`X-API-Key`,
  mensajes `register`/`heartbeat`/`result`/`job`, estados
  `assigned` (ver spec 05). Uso documentado en `remote-worker/README.md`.

## Gaps conocidos (no son bugs abiertos, son decisiones pendientes)

1. `AdminAliases.tsx` llama a `GET/PUT/DELETE /admin/aliases/:id` que no
   existen en el backend (solo POST/export/import).
2. `WORKER_*_ENABLED` en compose no las consume ningún código.
3. `?semantic=true` requiere Ollama (quitado del compose a petición).
4. `fuentes_url` vacía → scraper/discovery sin trabajo hasta sembrar URLs.
5. `translator-gpu` solo con hardware NVIDIA + profile `gpu`.
