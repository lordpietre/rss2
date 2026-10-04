# Cronograma de Mantenimiento RSS2

## Tareas Recurrentes

### Diarias

| Tarea | Descripción | Comando/Script |
|-------|-------------|---------------|
| Verificar healthchecks | Verificar que todos los servicios estén healthy | `docker compose ps` |
| Verificar traducciones pending | Count de traducciones en cola | `SELECT COUNT(*) FROM traducciones WHERE status='pending'` |
| Verificar errores de traducción | Traducciones con status='error' | `SELECT COUNT(*) FROM traducciones WHERE status='error'` |

### Semanales

| Tarea | Descripción | Notas |
|-------|-------------|-------|
| Reiniciar workers colgados | Si CPU ~2% y MEM al límite, restart translator | `docker compose restart translator translator-2` |
| Ver磁盘 espacio | Limpiar Redis si ocupa mucho | `redis-cli FLUSHDB` (si cache es prescindible) |
| Ver logs de errores | Buscar patrones de errores recurrentes | `docker compose logs --since=7d | grep ERROR` |

### Mensuales

| Tarea | Descripción | Notas |
|-------|-------------|-------|
| VACUUM ANALYZE | Reclamar espacio y actualizar estadísticas | `psql -c "VACUUM ANALYZE;"` |
| Verificar índices | Analizar uso de índices con `pg_stat_user_indexes` | Evitar índices no usados |
| Backup verificar | Verificar que backups funcionan | Restaurar en ambiente de test |

## Umbrales de Alerta

| Métrica | Warning | Critical |
|---------|---------|----------|
| traducciones pending | >50k | >100k |
| traducciones error | >100 | >500 |
| Tiempo de traducción avg | >5s/item | >10s/item |
| Memoria translator | >3GB | >3.5GB |

## Troubleshooting Común

### Translator worker colgado
```
Síntoma: CPU ~2%, MEM al límite, no traduce
Solución: docker compose restart translator translator-2
```

### Redis lleno
```
Síntoma: Redis no responde, errores de conexión
Solución: redis-cli FLUSHDB (los datos se reconstruyen de BD)
```

### Deadlock en Postgres
```
Síntoma: ERROR: 40P01 deadlock detected
Solución: Reiniciar el worker que causó el deadlock
Verificación: Los workers deben usar ORDER BY id ASC
```

### 3 réplicas translator saturan memoria
```
Síntoma: Memory cgroup out of memory
Solución: Máximo 2 réplicas de translator por host (22G)
```

## Runbooks

### Reinicio de translator
```bash
docker compose restart translator translator-2
docker compose logs -f translator
```

### Limpieza de traducciones oldas
```sql
-- Traducciones error de hace más de 7 días
DELETE FROM traducciones 
WHERE status = 'error' 
  AND created_at < NOW() - INTERVAL '7 days';
```

### Verificar santé de workers
```bash
docker compose ps
curl http://localhost:8080/health
```
