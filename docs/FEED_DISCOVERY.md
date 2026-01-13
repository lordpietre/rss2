# Sistema de Descubrimiento y Gestión de Feeds RSS

Este documento describe el sistema mejorado de descubrimiento automático y gestión de feeds RSS implementado en RSS2.

## 📋 Visión General

El sistema ahora incluye dos mecanismos para gestionar feeds RSS:

1. **Gestión Manual Mejorada**: Interfaz web para descubrir y añadir feeds desde cualquier URL
2. **Worker Automático**: Proceso en segundo plano que descubre feeds desde URLs almacenadas

## 🎯 Componentes del Sistema

### 1. Utilidad de Descubrimiento (`utils/feed_discovery.py`)

Módulo Python que proporciona funciones para:

- **`discover_feeds(url)`**: Descubre automáticamente todos los feeds RSS/Atom desde una URL
- **`validate_feed(feed_url)`**: Valida un feed y extrae su información básica
- **`get_feed_metadata(feed_url)`**: Obtiene metadatos detallados de un feed

```python
from utils.feed_discovery import discover_feeds

# Descubrir feeds desde una URL
feeds = discover_feeds('https://elpais.com')
# Retorna: [{'url': '...', 'title': '...', 'valid': True, ...}, ...]
```

### 2. Router de Feeds Mejorado (`routers/feeds.py`)

Nuevos endpoints añadidos:

#### Interfaz Web
- **`GET/POST /feeds/discover`**: Interfaz para descubrir feeds desde una URL
  - Muestra todos los feeds encontrados
  - Permite seleccionar cuáles añadir
  - Aplica configuración global (categoría, país, idioma)

- **`POST /feeds/discover_and_add`**: Añade múltiples feeds seleccionados
  - Extrae automáticamente título y descripción
  - Evita duplicados
  - Muestra estadísticas de feeds añadidos

#### API JSON
- **`POST /feeds/api/discover`**: API para descubrir feeds
  ```json
  {
    "url": "https://example.com"
  }
  ```
  Retorna:
  ```json
  {
    "feeds": [...],
    "count": 5
  }
  ```

- **`POST /feeds/api/validate`**: API para validar un feed específico
  ```json
  {
    "url": "https://example.com/rss"
  }
  ```

### 3. Worker de Descubrimiento (`workers/url_discovery_worker.py`)

Worker automático que:

1. **Procesa URLs de la tabla `fuentes_url`**
   - Prioriza URLs nunca procesadas
   - Reintenta URLs con errores
   - Actualiza URLs antiguas

2. **Descubre y Crea Feeds Automáticamente**
   - Encuentra todos los feeds RSS en cada URL
   - Valida cada feed encontrado
   - Crea entradas en la tabla `feeds`
   - Evita duplicados

3. **Gestión de Estado**
   - Actualiza `last_check`, `last_status`, `status_message`
   - Maneja errores gracefully
   - Registra estadísticas detalladas

#### Configuración del Worker

Variables de entorno:

```bash
# Intervalo de ejecución (minutos)
URL_DISCOVERY_INTERVAL_MIN=15

# Número de URLs a procesar por ciclo
URL_DISCOVERY_BATCH_SIZE=10

# Máximo de feeds a crear por URL
MAX_FEEDS_PER_URL=5
```

#### Estados de URLs en `fuentes_url`

| Estado | Descripción |
|--------|-------------|
| `success` | Feeds creados exitosamente |
| `existing` | Feeds encontrados pero ya existían |
| `no_feeds` | No se encontraron feeds RSS |
| `no_valid_feeds` | Se encontraron feeds pero ninguno válido |
| `error` | Error al procesar la URL |

## 🚀 Uso del Sistema

### Método 1: Interfaz Web Manual

1. **Navega a `/feeds/discover`**
2. **Ingresa una URL** (ej: `https://elpais.com`)
3. **Haz clic en "Buscar Feeds"**
4. El sistema mostrará todos los feeds encontrados con:
   - Estado de validación
   - Título y descripción
   - Número de entradas
   - Tipo de feed (RSS/Atom)
5. **Configura opciones globales**:
   - Categoría
   - País
   - Idioma
6. **Selecciona los feeds deseados** y haz clic en "Añadir Feeds Seleccionados"

### Método 2: Worker Automático

1. **Añade URLs a la tabla `fuentes_url`**:
   ```sql
   INSERT INTO fuentes_url (nombre, url, categoria_id, pais_id, idioma, active)
   VALUES ('El País', 'https://elpais.com', 1, 1, 'es', TRUE);
   ```

2. **El worker procesará automáticamente**:
   - Cada 15 minutos (configurable)
   - Descubrirá todos los feeds
   - Creará entradas en `feeds`
   - Actualizará el estado

3. **Monitorea el progreso**:
   ```sql
   SELECT nombre, url, last_check, last_status, status_message
   FROM fuentes_url
   ORDER BY last_check DESC;
   ```

### Método 3: Interfaz de URLs (Existente)

Usa la interfaz web existente en `/urls/add_source` para añadir URLs que serán procesadas por el worker.

## 🔄 Flujo de Trabajo Completo

```
┌─────────────────┐
│  Usuario añade  │
│  URL del sitio  │
└────────┬────────┘
         │
         v
┌─────────────────────────┐
│  URL guardada en        │
│  tabla fuentes_url      │
└────────┬────────────────┘
         │
         v
┌─────────────────────────┐
│  Worker ejecuta cada    │
│  15 minutos             │
└────────┬────────────────┘
         │
         v
┌─────────────────────────┐
│  Descubre feeds RSS     │
│  usando feedfinder2     │
└────────┬────────────────┘
         │
         v
┌─────────────────────────┐
│  Valida cada feed       │
│  encontrado             │
└────────┬────────────────┘
         │
         v
┌─────────────────────────┐
│  Crea entradas en       │
│  tabla feeds            │
└────────┬────────────────┘
         │
         v
┌─────────────────────────┐
│  Ingestor Go procesa    │
│  feeds cada 15 minutos  │
└────────┬────────────────┘
         │
         v
┌─────────────────────────┐
│  Noticias descargadas   │
│  y procesadas           │
└─────────────────────────┘
```

## 📊 Tablas de Base de Datos

### `fuentes_url`
Almacena URLs de sitios web para descubrimiento automático:

```sql
CREATE TABLE fuentes_url (
    id SERIAL PRIMARY KEY,
    nombre VARCHAR(255) NOT NULL,
    url TEXT NOT NULL UNIQUE,
    categoria_id INTEGER REFERENCES categorias(id),
    pais_id INTEGER REFERENCES paises(id),
    idioma CHAR(2) DEFAULT 'es',
    last_check TIMESTAMP,
    last_status VARCHAR(50),
    status_message TEXT,
    last_http_code INTEGER,
    active BOOLEAN DEFAULT TRUE
);
```

### `feeds`
Almacena feeds RSS descubiertos y validados:

```sql
CREATE TABLE feeds (
    id SERIAL PRIMARY KEY,
    nombre VARCHAR(255),
    descripcion TEXT,
    url TEXT NOT NULL UNIQUE,
    categoria_id INTEGER REFERENCES categorias(id),
    pais_id INTEGER REFERENCES paises(id),
    idioma CHAR(2),
    activo BOOLEAN DEFAULT TRUE,
    fallos INTEGER DEFAULT 0,
    last_etag TEXT,
    last_modified TEXT,
    last_error TEXT
);
```

## ⚙️ Configuración del Sistema

### Variables de Entorno

Añade al archivo `.env`:

```bash
# RSS Ingestor
RSS_POKE_INTERVAL_MIN=15        # Intervalo de ingesta (minutos)
RSS_MAX_FAILURES=10             # Fallos máximos antes de desactivar feed
RSS_FEED_TIMEOUT=60             # Timeout para descargar feeds (segundos)

# URL Discovery Worker
URL_DISCOVERY_INTERVAL_MIN=15   # Intervalo de descubrimiento (minutos)
URL_DISCOVERY_BATCH_SIZE=10     # URLs a procesar por ciclo
MAX_FEEDS_PER_URL=5             # Máximo de feeds por URL
```

### Docker Compose

El worker ya está configurado en `docker-compose.yml`:

```yaml
url-discovery-worker:
  build: .
  container_name: rss2_url_discovery
  command: bash -lc "python -m workers.url_discovery_worker"
  environment:
    DB_HOST: db
    URL_DISCOVERY_INTERVAL_MIN: 15
    URL_DISCOVERY_BATCH_SIZE: 10
    MAX_FEEDS_PER_URL: 5
  depends_on:
    db:
      condition: service_healthy
  restart: unless-stopped
```

## 🔧 Comandos Útiles

### Ver logs del worker de descubrimiento
```bash
docker logs -f rss2_url_discovery
```

### Reiniciar el worker
```bash
docker restart rss2_url_discovery
```

### Ejecutar manualmente el worker (testing)
```bash
docker exec -it rss2_url_discovery python -m workers.url_discovery_worker
```

### Ver estadísticas de descubrimiento
```sql
-- URLs procesadas recientemente
SELECT nombre, url, last_check, last_status, status_message
FROM fuentes_url
WHERE last_check > NOW() - INTERVAL '1 day'
ORDER BY last_check DESC;

-- Feeds creados recientemente
SELECT nombre, url, fecha_creacion
FROM feeds
WHERE fecha_creacion > NOW() - INTERVAL '1 day'
ORDER BY fecha_creacion DESC;
```

## 🛠️ Troubleshooting

### El worker no encuentra feeds

1. Verifica que la URL sea accesible:
   ```bash
   curl -I https://example.com
   ```

2. Verifica los logs del worker:
   ```bash
   docker logs rss2_url_discovery
   ```

3. Prueba manualmente el descubrimiento:
   ```python
   from utils.feed_discovery import discover_feeds
   feeds = discover_feeds('https://example.com')
   print(feeds)
   ```

### Feeds duplicados

El sistema previene duplicados usando `ON CONFLICT (url) DO NOTHING`. Si un feed ya existe, simplemente se omite.

### Worker consume muchos recursos

Ajusta las configuraciones:

```bash
# Reduce el batch size
URL_DISCOVERY_BATCH_SIZE=5

# Aumenta el intervalo
URL_DISCOVERY_INTERVAL_MIN=30

# Reduce feeds por URL
MAX_FEEDS_PER_URL=3
```

## 📝 Mejores Prácticas

1. **Añade URLs de sitios, no feeds directos**
   - ✅ `https://elpais.com`
   - ❌ `https://elpais.com/rss/feed.xml`

2. **Configura categoría y país correctamente**
   - Facilita la organización
   - Mejora la experiencia del usuario

3. **Monitorea el estado de las URLs**
   - Revisa periódicamente `fuentes_url`
   - Desactiva URLs que fallan consistentemente

4. **Limita el número de feeds por URL**
   - Evita sobrecarga de feeds similares
   - Mantén `MAX_FEEDS_PER_URL` entre 3-5

## 🎉 Ventajas del Sistema

✅ **Automatización completa**: Solo añade URLs, el sistema hace el resto
✅ **Descubrimiento inteligente**: Encuentra todos los feeds disponibles
✅ **Validación automática**: Solo crea feeds válidos y funcionales
✅ **Sin duplicados**: Gestión inteligente de feeds existentes
✅ **Escalable**: Procesa múltiples URLs en lotes
✅ **Resiliente**: Manejo robusto de errores y reintentos
✅ **Monitoreable**: Logs detallados y estados claros

## 📚 Referencias

- **feedfinder2**: https://github.com/dfm/feedfinder2
- **feedparser**: https://feedparser.readthedocs.io/
- **Tabla fuentes_url**: `/init-db/01.schema.sql`
- **Worker**: `/workers/url_discovery_worker.py`
- **Utilidades**: `/utils/feed_discovery.py`
