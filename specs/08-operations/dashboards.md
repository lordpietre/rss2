# Dashboards Recomendados - Grafana

## Dashboard: RSS2 Overview

### Panel 1: Métricas Generales
- Total noticias
- Total feeds activos
- Total usuarios
- Traducciones completadas (24h)

### Panel 2: Pipeline de Traducción
- Traducciones pending (time series)
- Traducciones done (time series)
- Tasa de traducción (items/min)
- Tiempo promedio de traducción

### Panel 3: Workers Health
- **Remote Worker primero** (online/offline status prominently displayed)
- Estado de cada worker (up/down)
- Jobs completados por worker
- Jobs fallidos por worker
- Memoria y CPU por container

### Panel 4: Base de Datos
- Conexiones activas a PostgreSQL
- Queries lentas (>1s)
- Espacio en disco

## Dashboard: Translation Worker

### Panel 1: Throughput
- Items traducidos por minuto
- Items traducidos por hora (12h, 24h, 2d, 3d, 4d, 5d)
- Rate de traducción actual

### Panel 2: Errores
- Errores de traducción por tipo
- Errores HTTP de Wikipedia
- Timeouts

### Panel 3: Queue
- Jobs pending en cola
- Jobs en proceso
- Tiempo en cola promedio

## Dashboard: Idiomas de Noticias Originales

### Panel 1: Distribución de Idiomas
- Top idiomas originales (en, es, pt, fr, de, tr, hr, id, fi, ru...)
- Cantidad de noticias por idioma
- Días activos por idioma

### Panel 2: Tendencia de Idiomas
- Noticias nuevas por idioma por hora/día
- Evolución de la distribución

## Dashboard: Ingestor

### Panel 1: Feeds
- Feeds activos vs inactivos
- Errores de fetch por feed
- Tiempo desde último fetch

### Panel 2: Noticias
- Noticias nuevas por hora
- Noticias por fuente
- Noticias sin traducir

## Datos de Prometheus

Exponer métricas de Prometheus en `/metrics` de cada servicio para scraping.

### Métricas Recomendadas

```
# Backend
http_requests_total{method, status, path}
http_request_duration_seconds{method, path}

# Workers
jobs_processed_total{worker, status}
job_processing_duration_seconds{worker}

# Database
db_connections_active
db_queries_total{query_type}
db_query_duration_seconds{query_type}

# Redis
redis_commands_total{command}
redis_command_duration_seconds{command}
```

## API Stats Endpoint

El endpoint `GET /api/stats` retorna:

```json
{
  "total_news": 29692,
  "total_translated": 10621,
  "translations_pending": 16036,
  "translations_done": 9070,
  "translations_error": 0,
  "top_languages": [
    {"lang": "en", "count": 11096, "dias_activos": 1104},
    {"lang": "es", "count": 2681, "dias_activos": 276},
    ...
  ],
  "translation_stats_12h": [
    {"hour": "2026-10-03 23:00", "total": 3, "items": 384},
    ...
  ],
  "translation_stats_24h": [...],
  "translation_stats_2d": [...],
  "translation_stats_3d": [...],
  "translation_stats_4d": [...],
  "translation_stats_5d": [...]
}
```
