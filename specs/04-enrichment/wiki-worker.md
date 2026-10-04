# Wiki Worker - Retry Logic

## Comportamiento Actual

El worker de wiki (`backend/cmd/wiki_worker/main.go`) procesa tags que no han sido verificados en Wikipedia (`wiki_checked = FALSE`).

### Retry Logic

**Errores transitorios** (se reintentan en siguiente ciclo):
- `context deadline exceeded` (timeout)
- HTTP 429 (Rate Limited)
- HTTP 503 (Service Unavailable)
- HTTP 504 (Gateway Timeout)

**Errores permanentes** (se marcan como `wiki_checked = TRUE` para no reintentar):
- HTTP 400 (Bad Request)
- HTTP 404 (Not Found - incluyendo disambiguation pages)
- Otros errores no transitorios

### Rate Limiting

El worker tiene un delay de **1 segundo** entre cada tag para evitar rate limits de Wikipedia.

### Mejoras Posibles

1. **Tracking de reintentos**: Agregar columna `wiki_retry_count` para limitar reintentos
2. **Backoff exponencial**: Incrementar delay después de errores 429
3. **Circuit breaker**: Detener requests a Wikipedia si hay muchos errores consecutivos

## Configuración

| Variable | Default | Descripción |
|----------|---------|-------------|
| `WIKI_SLEEP` | 30 | Segundos entre ciclos si no hay tags pendientes |
| `WIKI_BATCH` | 40 | Tags a procesar por ciclo |

## Flujo

```
getPendingTags() → processTag() → fetchWikipediaInfo()
                                              ↓
                              ┌────────────────┴────────────────┐
                              ↓                                 ↓
                        Error?                              Success
                              ↓                                 ↓
                    isTransient?                        UPDATE tags
                    (429,503,504,timeout)              SET wiki_summary, wiki_url, 
                     ↓            ↓                      image_path, wiki_checked=TRUE
                   Yes          No
                     ↓            ↓
              No marking         UPDATE tags
              (retry next       SET wiki_checked=TRUE
               cycle)           (permanent failure)
```
