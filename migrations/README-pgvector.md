# Migración a pgvector

## Resumen de Cambios

### Archivos Modificados/Creados:

1. **`migrations/47-pgvector-embeddings.sql`** - Migration SQL para agregar pgvector
2. **`workers/embeddings_worker.py`** - Actualizado para guardar en columna `embedding_vector`
3. **`backend/cmd/related/main.go`** - Reescrito para usar búsqueda vectorial de pgvector
4. **`docker-compose.yml`** - Cambiado a imagen `pgvector/pgvector:pg16`

## Pasos para Migrar

### 1. Detener servicios
```bash
docker-compose down
```

### 2. Respaldar base de datos
```bash
docker-compose up -d db
docker exec rss2_db pg_dump -U rss rss > backup_rss_$(date +%Y%m%d).sql
docker-compose stop db
```

### 3. Ejecutar la migración
```bash
# Opción A: Aplicar migración manualmente
docker exec -i rss2_db psql -U rss -d rss < migrations/47-pgvector-embeddings.sql

# Opción B: Iniciar servicios normalmente (la migración se aplica automáticamente)
docker-compose up -d
```

### 4. Verificar instalación de pgvector
```bash
docker exec rss2_db psql -U rss -d rss -c "SELECT extname, extversion FROM pg_extension WHERE extname = 'vector';"
```

### 5. Verificar índice HNSW
```bash
docker exec rss2_db psql -U rss -d rss -c "SELECT indexname, indexdef FROM pg_indexes WHERE tablename = 'traduccion_embeddings';"
```

### 6. Verificar función
```bash
docker exec rss2_db psql -U rss -d rss -c "SELECT proname FROM pg_proc WHERE proname = 'find_related_translations';"
```

## Cambios en el Comportamiento

### Antes (Memoria):
- `fetchAllEmbeddings()` cargaba TODOS los embeddings en memoria
- Cálculo O(n²) de similaridad coseno
- Límite práctico: ~100,000 noticias

### Después (pgvector):
- Búsqueda vectorial nativa en PostgreSQL
- Índice HNSW para búsqueda aproximada de vecinos más cercanos
- Escalabilidad: millones de noticias
- Memoria constante (no carga todos los vectores)

## Verificación de Funcionamiento

```bash
# Ver que hay embeddings en la nueva columna
docker exec rss2_db psql -U rss -d rss -c "SELECT COUNT(*) FROM traduccion_embeddings WHERE embedding_vector IS NOT NULL;"

# Ver que related_worker usa pgvector
docker logs rss2_related --tail=20
```

## Rollback (si es necesario)

```sql
-- Desactivar uso de la nueva columna
ALTER TABLE traduccion_embeddings DROP COLUMN IF EXISTS embedding_vector;
DROP INDEX IF EXISTS idx_tr_emb_vector_hnsw;
DROP FUNCTION IF EXISTS find_related_translations;

-- Restaurar código anterior del related worker si es necesario
```
