# Alerting Proactivo - RSS2

## Reglas de Alertamiento

### Criticidad: CRITICAL

#### Traducciones colgadas
```yaml
alert: TranslationQueueStalled
expr: increase(traducciones_pending[5m]) == 0 AND traducciones_pending > 1000
for: 10m
labels:
  severity: critical
annotations:
  summary: "Cola de traducciones detenida"
  description: "No hay progreso en traducciones por más de 10 minutos con >1000 items pendientes"
```

#### Workers caídos
```yaml
alert: WorkerDown
expr: up{job=~"translator|ner|wiki|scraper"} == 0
for: 2m
labels:
  severity: critical
annotations:
  summary: "Worker caído"
  description: "El worker {{ $labels.instance }} no está respondiendo"
```

#### Base de datos inalcanzable
```yaml
alert: DatabaseDown
expr: pg_up == 0
for: 1m
labels:
  severity: critical
annotations:
  summary: "PostgreSQL no responde"
  description: "La base de datos no responde health checks"
```

### Criticidad: WARNING

#### Memoria alta
```yaml
alert: HighMemoryUsage
expr: (container_memory_usage_bytes / container_spec_memory_limit_bytes) > 0.85
for: 5m
labels:
  severity: warning
annotations:
  summary: "Uso de memoria alto"
  description: "Container {{ $labels.container }} usando >85% de memoria asignada"
```

#### Rate limit activo
```yaml
alert: HighRateLimitErrors
expr: rate(http_requests_total{status="429"}[5m]) > 10
for: 5m
labels:
  severity: warning
annotations:
  summary: "Muchos requests bloqueados por rate limit"
  description: "{{ $value }} requests por segundo están siendo bloqueados"
```

#### Traducciones error rate alto
```yaml
alert: HighTranslationErrorRate
expr: rate(traducciones_error_total[5m]) / rate(traducciones_total[5m]) > 0.1
for: 10m
labels:
  severity: warning
annotations:
  summary: "Alto rate de errores en traducciones"
  description: ">10% de las traducciones están fallando"
```

### Criticidad: INFO

#### News sin traducir por >7 días
```sql
-- Query para verificar noticias sin traducir
SELECT COUNT(*) FROM noticias n
WHERE NOT EXISTS (
  SELECT 1 FROM traducciones t 
  WHERE t.noticia_id = n.id AND t.status = 'done'
)
AND n.fecha < NOW() - INTERVAL '7 days';
```

## Canales de Notificación

| Canal | Uso |
|-------|-----|
| Email | Alertas critical y warning |
| Slack | Alertas critical |
| PagerDuty | Alertas critical de negocio |

## Runbooks

### Worker caído
1. Verificar logs: `docker compose logs <worker>`
2. Restart: `docker compose restart <worker>`
3. Si persiste, verificar recursos del host

### Cola de traducciones detenida
1. Verificar translator workers: `docker compose ps translator`
2. Verificar memoria: `docker stats`
3. Restart translators si necesario: `docker compose restart translator translator-2`
