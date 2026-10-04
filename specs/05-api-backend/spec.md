# Spec 05 — API backend (Go)

## Alcance

`backend/cmd/server/main.go` (imagen backend, `command: server`, `:8080`):
Gin + pgx + go-redis + zerolog. Rutas públicas: `/health`, `/api/news[/:id]`,
`/api/feeds[/export]`, `/api/search`, `/api/entities[/news|/mentions]`,
`/api/last-names`, `/api/alerts`, `/api/stats`, `/api/categories`,
`/api/countries`, `/api/auth/*`. Con auth: CRUD feeds, import, searchlog,
suggestions, alertas read, `/api/auth/me`, `DELETE /api/news/:id`.
`GET /api/stats/ratelimit` (público, sin caché: 429s totales y por
configuración desde el arranque; sin IPs). Admin:
aliases (POST, GET list con `?tipo`, PUT/DELETE `/:id`, export/import),
entities/retype, backup/restore,
users, reset-db, alerts/scan, workers/* (+ remote), WS `/ws/worker`
(api_key por query/`X-API-Key`, mensajes register/heartbeat/result/job).

## Reglas verificadas (fixes aplicados, no regressar)

1. **Esquema real**: `noticias.id VARCHAR(32)` → `NewsWithTranslations.ID
   string` (igual que el frontend); no existe `n.contenido` → usar
   `COALESCE(n.resumen,'')`; `titulo/fecha/fuente/lang` nulables → temps
   (`*time.Time`→RFC3339, `TrimSpace` en `CHAR(5)`); `COALESCE(n.titulo,'')`.
   Afecta a `GetNews`, `GetEntityNews`, `GetNewsByID`, `SearchNews`,
   `GetEntities` (orden `valor,tipo,apellido,cnt,...`).
2. **Args SQL**: no duplicar `targetLang` entre count y select (bug
   `expected N arguments`).
3. **Caché**: `cache.Set` serializa; pasar structs (categories, countries,
   search response, stats, feeds). Purgar Redis tras cambios de formato.
4. **Rate limit**: `SetTrustedProxies([127/8,10/8,172.16/12,192.168/16])` para
   bucket por IP real (X-Forwarded-For de nginx). Límites: 10/min en auth,
   `RATE_LIMIT_PER_MINUTE` (def. 60) en `/api`.
5. **Auth UX**: duplicado en register → `409 Email or username already
   registered` (no 500). No hay reset de password en la app.
6. **Logger**: package `internal/logger` expone `Trace/Debug/Info/Warn/Error/
   Fatal/Panic/Log` sobre el `Logger` global (post-`Init`); cada worker
   `cmd/*` debe tener `func main(){ Main() }`.
7. **Búsqueda full-text** (2026-09-14): `SearchNews` usaba
   `ILIKE %..%` (seq scan, 8.3 s). Ahora
   `search_vector_es @@ plainto_tsquery('spanish', q)` (GIN, 100%
   poblado): 0.35 s, con stemming. `total` verificado contra SQL.
8. **Populares sin duplicados de case** (2026-09-14): `GetEntities`
   agrupa por `LOWER(...)`, suma counts, muestra la variante exacta más
   frecuente (`mode()`) y `MAX(wiki_*)`. "Donald Trump/TRUMP/trump" →
   una fila (1945). `GetEntityNews` ya era case-insensitive (1772).
9. **`paises` no tiene `flag_emoji`** (2026-09-25): `GetStats` seleccionaba
   `p.flag_emoji` (columna inexistente; `init-db/` manda). El `err` se
   tragaba un `if err == nil` → `top_countries` salía **null** sin log.
   Query reducida a `p.id, p.nombre, COUNT(n.id)`, `rows.Scan` con error
   logueado y también el de la query. `flag_emoji` sigue en el contrato
   del JSON (string vacío, igual que `GET /api/countries`, que nunca lo
   selecciona). Tras un fix que toca respuestas cacheadas: `FLUSHDB`
   (la clave `stats` TTL 30 min devolvía el null viejo).
10. **`lang_to` es `CHAR(5)`** (2026-09-25): Postgres rellena con
    espacios → `lang_translated` salía `"es   "` y el frontend lo pinta
    en `Search.tsx` (`lang_translated || fuente_nombre`). `Lang` ya hacía
    `TrimSpace` pero `LangTranslated` no. Ahora en los 3 scan que lo
    traen (`GetNews`, `GetNewsByID`, `SearchNews`); `GetEntityNews` no lo
    selecciona. Verificado en vivo: `len=2`.

## Auth (favoritos y listas)

Todos los endpoints de `/api/favorites/*`, `/api/lists/*`, `/api/saved-searches/*`
requieren autenticación JWT válida via `middleware.AuthRequired()`. El `user_id`
se extrae del claims JWT y se pone en el contexto Gin (`c.Set("user_id", ...)`).
Los handlers acceden via `c.GetInt("user_id")`.

El middleware verifica:
1. Presencia de header `Authorization: Bearer <token>`
2. Validez y expiración del JWT
3. Extrae `UserID` del claims y lo pone en contexto

Si la auth falla, retorna 401 Unauthorized.

## Criterios de aceptación

- [x] `GET /api/news`, `translated_only`, `/api/news/:id`, `/api/search`,
      `/api/entities*`, `/api/alerts`, `/api/feeds` → 200 con datos reales.
- [x] Register→login→JWT→`/me` funciona; duplicado → 409.
- [x] Gap AdminAliases cerrado (2026-09-14): `GET /admin/aliases`,
  `PUT/DELETE /admin/aliases/:id` implementados en `handlers/admin.go`
  (verificados: 401 sin auth, `vet`+`build` limpios). `cmd/main.go` roto
  eliminado (importaba paquete main).

## Tasks

- [x] Todos los fixes listados + rebuild + verificación en vivo.
- [x] `GET /api/admin/ingest/stats` implementado (feeds total/activos con
  fallos, noticias total/7d, cobertura % por `fuente_nombre = feeds.nombre`).
