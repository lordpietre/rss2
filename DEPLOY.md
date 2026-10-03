# RSS2 - Guía de Despliegue Simplificada

## Arquitectura Simplificada

```
RSS Feed → noticias(titulo, resumen, contenido) → scraper(enriquece contenido)
                                                    ↓
                                          traducciones(titulo_trad, resumen_trad, contenido_trad)
                                                    ↓
                                          API → frontend
```

## Archivos Modificados

### Workers de Traducción
- `translator/workers/ctranslator_worker.py` - **Worker principal** (docker-compose)
- `remote-worker/worker.py` - **Worker remoto** (WebSocket)

### Configuración de Límites
| Parámetro | Valor | Descripción |
|-----------|-------|-------------|
| MAX_SRC_TOKENS | 2048 | Tokens origen |
| MAX_NEW_TOKENS | 2048 | Tokens traducción |
| MAX_BODY_CHARS | 80000 | Caracteres max por texto |
| BODY_CHARS_CHUNK | 2000 | Tamaño de chunks |

### Base de Datos
- `init-db/09-contenido.sql` - Migración para columnas contenido y contenido_trad

### Otros
- `workers/translation_scheduler.py` - Crea jobs si hay resumen O contenido
- `backend/cmd/scraper/main.go` - Guarda contenido en columna separada

## Despliegue

### 1. Ejecutar Migración

```bash
psql -h localhost -U rss -d rss -f init-db/09-contenido.sql
```

O si usas docker:
```bash
docker exec -i rss2_db psql -U rss -d rss < init-db/09-contenido.sql
```

### 2. Reiniciar Servicios

```bash
docker-compose restart translator translator-2 scraper translation-scheduler langdetect
```

### 3. Verificar Traducciones

```bash
# Ver que las traducciones se están procesando
docker logs -f rss2_translator

# Ver jobs pendientes
docker exec -i rss2_db psql -U rss -d rss -c "SELECT COUNT(*) FROM traducciones WHERE status='pending';"
```

## Workers

### Local (docker-compose)
- `translator` - Traductor principal
- `translator-2` - Réplica
- `translation-scheduler` - Crea jobs

### Remoto
- `remote-worker/worker.py` - Se conecta por WebSocket al backend

Para usar remote worker:
```bash
cd remote-worker
WORKER_API_KEY=your_key WORKER_SERVER=ws://your-server:8080/ws/worker python worker.py
```

## Flow de Datos

1. **Ingestor** (`rss-ingestor-go`) → Extrae titulo y resumen del feed RSS
2. **Scraper** → Extrae contenido completo de la URL y guarda en columna `contenido`
3. **Langdetect** → Detecta idioma de la noticia
4. **Translation Scheduler** → Crea job de traducción
5. **Translator** → Traduce titulo_trad, resumen_trad, contenido_trad
6. **NER/Embeddings/Related** → Procesos opcionales (lentos, no críticos)

## API Response

La API ahora retorna `content_translated` junto con `title_translated` y `summary_translated`:

```json
{
  "id": "...",
  "titulo": "Original title",
  "title_translated": "Título traducido",
  "summary_translated": "Resumen traducido",
  "content_translated": "Contenido completo traducido...",
  ...
}
```

## Troubleshooting

### Traducciones no se crean
1. Verificar que langdetect haya detectado el idioma: `SELECT lang FROM noticias LIMIT 5;`
2. Verificar que el scheduler esté corriendo: `docker logs rss2_translation_scheduler`

### Contenido no se traduce
1. Verificar que el scraper esté extrayendo contenido: `SELECT length(contenido) FROM noticias LIMIT 5;`
2. Ver logs del translator: `docker logs -f rss2_translator`

### OOM (Out of Memory)
Reducir MAX_SEQ_PER_CALL en docker-compose.yml:
```yaml
environment:
  MAX_SEQ_PER_CALL: 16  # en vez de 32
```
