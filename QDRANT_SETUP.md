# ✅ Sistema Qdrant - Búsquedas Semánticas

## 🎯 Arquitectura Actual

El sistema vectoriza **directamente** las noticias traducidas y proporciona búsqueda semántica en tiempo real.

```
Noticias Originales (RSS)
        ↓
Traducción (translator workers)
        ↓
PostgreSQL (tabla 'traducciones')
        ↓
Qdrant Worker (vectorización directa)
        ↓
Qdrant (búsquedas semánticas)
        ↓
API de Búsqueda (utils/qdrant_search.py)
        ↓
Buscador General + Monitor de Conflictos
```

## ✅ Servicios y Componentes

| Componente | Puerto/Ubicación | Descripción |
|----------|--------|-------------|
| **Qdrant** | 6333 | Base de datos vectorial |
| **Qdrant Worker** | - | Vectorización continua |
| **Búsqueda Semántica** | `utils/qdrant_search.py` | API de búsqueda vectorial |
| **Buscador General** | `routers/search.py` | Búsqueda con fallback a PostgreSQL |
| **Monitor de Conflictos** | `routers/conflicts.py` | Búsqueda por keywords con vectores |

### Configuración del Worker

- **Origen**: Tabla `traducciones`
- **Modelo**: `paraphrase-multilingual-MiniLM-L12-v2`
- **Dimensiones**: 384
- **Dispositivo**: CPU (GPU disponible)
- **Velocidad**: ~100+ noticias/minuto
- **Total Vectorizado**: ~507,000 noticias

## 🚀 Integración Completa

### Buscador General (`/api/search`)

La búsqueda ahora usa **Qdrant primero** para mayor velocidad y precisión semántica:

1. **Búsqueda Semántica** (por defecto): Usa vectores de Qdrant
2. **Fallback PostgreSQL**: Si falla o no hay resultados
3. **Enriquecimiento**: Combina datos de ambas fuentes

**Ventajas:**
- ✅ Búsquedas 10-100x más rápidas (sin escaneo de 500k filas)
- ✅ Comprende sinónimos y contexto ("protestas" encuentra "manifestaciones")
- ✅ Multilingüe automático
- ✅ Sin dependencia de palabras exactas

**Parámetros:**
- `q`: Texto de búsqueda
- `limit`: Máximo de resultados (default: 10, max: 50)
- `semantic`: `true/false` (default: `true`)

### Monitor de Conflictos (`/conflicts/<id>`)

Ahora usa búsqueda semántica por keywords:

**Antes (ILIKE con PostgreSQL):**
- ❌ "irán protestas" requería coincidencia exacta de toda la frase
- ❌ Lento con 500k noticias
- ❌ No encontraba variaciones ("manifestación", "protesta")

**Ahora (Qdrant):**
- ✅ "irán protestas" busca por ambas palabras independientemente
- ✅ Rápido (búsqueda vectorial)
- ✅ Encuentra contenido semánticamente similar

## 🔧 Comandos Útiles

### Ver Logs
```bash
docker-compose logs -f qdrant-worker
docker-compose logs -f qdrant
docker-compose logs -f rss2_web  # Para ver logs de búsqueda
```

### Estadísticas
```bash
docker exec -it rss2_web python scripts/migrate_to_qdrant.py --stats
```

### Vectorizar Pendientes (Manual)
```bash
docker exec -it rss2_web python scripts/migrate_to_qdrant.py --vectorize --batch-size 200
```

### Reset Completo (⚠️ Destructivo)
```bash
docker exec -it rss2_web python scripts/migrate_to_qdrant.py --reset
```

### Probar Búsqueda Semántica
```bash
# Búsqueda semántica
curl "http://localhost:8001/api/search?q=protestas+en+iran&semantic=true"

# Búsqueda tradicional (fallback)
curl "http://localhost:8001/api/search?q=protestas+en+iran&semantic=false"
```

## 📊 Verificar Estado

### Base de Datos
```sql
-- Progreso de vectorización
SELECT 
    COUNT(*) as total,
    COUNT(*) FILTER (WHERE vectorized = TRUE) as vectorizadas,
    COUNT(*) FILTER (WHERE vectorized = FALSE) as pendientes
FROM traducciones 
WHERE lang_to = 'es' AND status = 'done';
```

### Qdrant API
```bash
# Estado de la colección
curl http://localhost:6333/collections/news_vectors

# Health check
curl http://localhost:6333/healthz

# Conteo de puntos
curl http://localhost:6333/collections/news_vectors | jq '.result.points_count'
```

## 🔍 Variables de Entorno

```bash
# .env
QDRANT_HOST=qdrant
QDRANT_PORT=6333
QDRANT_COLLECTION_NAME=news_vectors
QDRANT_BATCH_SIZE=100
QDRANT_SLEEP_IDLE=30
EMB_MODEL=sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2
```

## 📁 Archivos Relevantes

| Archivo | Función |
|---------|---------|
| `workers/qdrant_worker.py` | Worker de vectorización continua |
| `utils/qdrant_search.py` | **NUEVO**: API de búsqueda semántica |
| `routers/search.py` | **ACTUALIZADO**: Buscador con Qdrant |
| `routers/conflicts.py` | **ACTUALIZADO**: Monitor de conflictos con Qdrant |
| `scripts/migrate_to_qdrant.py` | Migración/estadísticas |
| `docker-compose.yml` | Configuración de servicios + timezone sync |

## ⏰ Sincronización de Hora

**Problema Resuelto:** Todos los contenedores Docker ahora tienen la hora sincronizada (TZ=Europe/Madrid).

**Cambios:**
- ✅ Variable `TZ=Europe/Madrid` en todos los servicios
- ✅ Volúmenes `/etc/timezone` y `/etc/localtime` en servicios clave
- ✅ Logs consistentes entre todos los workers

## 🚀 Despliegue en Nuevas Máquinas

### Requisitos Previos
1. Docker y Docker Compose instalados
2. Al menos 8GB RAM (recomendado 16GB)
3. GPU NVIDIA (opcional, para workers de traducción)

### Pasos de Instalación

```bash
# 1. Clonar repositorio
git clone <repo-url>
cd rss2

# 2. Configurar variables de entorno
cp .env.example .env
# Editar .env con tus credenciales

# 3. Iniciar servicios
docker-compose up -d

# 4. Verificar que Qdrant está funcionando
curl http://localhost:6333/healthz

# 5. Monitorear vectorización
docker-compose logs -f qdrant-worker
```

### Migración de Datos Existentes

Si ya tienes noticias traducidas sin vectorizar:

```bash
# Ver estadísticas
docker exec -it rss2_web python scripts/migrate_to_qdrant.py --stats

# Vectorizar todas las pendientes (puede tardar horas con 500k noticias)
# El worker lo hace automáticamente, pero puedes forzarlo:
docker-compose restart qdrant-worker
```

## 🔍 Troubleshooting

### La búsqueda no usa Qdrant
```bash
# Verificar que Qdrant está corriendo
docker ps | grep qdrant

# Ver logs del worker
docker-compose logs qdrant-worker

# Verificar conexión
curl http://localhost:6333/collections/news_vectors
```

### Búsqueda lenta aún
```bash
# Verificar cuántas noticias están vectorizadas
docker exec -it rss2_web python scripts/migrate_to_qdrant.py --stats

# Si hay muchas pendientes, el worker las procesará automáticamente
# Para acelerar, aumenta el batch size en docker-compose.yml:
# QDRANT_BATCH_SIZE=200
```

### Error "No module named 'qdrant_client'"
```bash
# Reconstruir imagen web
docker-compose build rss2_web
docker-compose restart rss2_web
```

## 📈 Rendimiento

**Antes (PostgreSQL solo):**
- Búsqueda simple: 2-5 segundos (500k filas)
- Búsqueda compleja: 10-30 segundos
- Monitor de conflictos: 5-15 segundos

**Ahora (Qdrant + PostgreSQL):**
- Búsqueda semántica: 50-200ms
- Enriquecimiento PostgreSQL: +50ms
- Monitor de conflictos: 100-300ms
- **Mejora: 10-100x más rápido** ⚡

## 🎯 Próximos Pasos

- [ ] Implementar filtros avanzados (fecha, país, categoría)
- [ ] Cachear resultados frecuentes en Redis
- [ ] Agregar búsqueda híbrida (combinar PostgreSQL FTS + Qdrant)
- [ ] Dashboard de métricas de búsqueda
