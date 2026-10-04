# Audit de Dependencias - RSS2

Fecha: 2026-10-03

## Go Backend

### Runtime
- **Go**: 1.23.12

### Dependencias Principales

| Paquete | Versión | Uso | Notas |
|---------|---------|-----|-------|
| github.com/gin-gonic/gin | v1.9.1 | HTTP framework | |
| github.com/golang-jwt/jwt/v5 | v5.0.0 | Autenticación JWT | |
| github.com/jackc/pgx/v5 | v5.4.3 | Driver PostgreSQL | |
| github.com/redis/go-redis/v9 | v9.0.5 | Cliente Redis | |
| github.com/rs/zerolog | v1.32.0 | Logging | |
| github.com/gorilla/websocket | v1.5.1 | WebSocket | |
| golang.org/x/crypto | v0.28.0 | Criptografía | |
| golang.org/x/net | v0.30.0 | Networking | |
| github.com/mmcdole/gofeed | v1.2.1 | Parser RSS/Atom | |
| github.com/PuerkitoBio/goquery | v1.9.2 | HTML parsing | |

### Recomendaciones

1. **Actualizar Gin**: v1.9.1 tiene versiones más nuevas disponibles (v1.10.x)
2. **Actualizar go-redis**: v9.0.5 tiene v9.5+ disponible
3. **Vulnerabilidades**: Ejecutar `go audit` periódicamente

## Python Workers

### Runtime
- **Python**: 3.11+

### Dependencias Principales (requirements.txt o setup.py)

| Paquete | Uso | Notas |
|---------|-----|-------|
| psycopg2-binary | PostgreSQL driver | |
| redis | Cliente Redis | |
| ctranslate2 | Traducción NLLB | |
| transformers | Modelos HuggingFace | |
| langdetect | Detección de idioma | |
| feedparser | Parser RSS | |

## Acciones Recomendadas

1. **Semanal**: `go mod tidy && go vet`
2. **Mensual**: `go list -m all | grep -v indirect`
3. **Trimestral**: Revisar actualizaciones de dependencias mayores

## Seguridad

- No usar dependencias con CVEs conocidos
- Preferir dependencias mantenidas activamente
- Verificar licencias (MIT, BSD, Apache preferidas)
