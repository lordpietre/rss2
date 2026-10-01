# Remote Translator Worker

Worker remoto de traducción para RSS2. Se conecta al servidor via WebSocket y procesa trabajos de traducción automáticamente, permitiendo distribuir la carga de traducción en equipos remotos.

## Tabla de Contenidos

1. [Arquitectura](#arquitectura)
2. [Requisitos](#requisitos)
3. [Quick Start GPU](#quick-start-gpu)
4. [Quick Start CPU](./CPU.md) ← Guía específica para CPU
5. [Configuración del Servidor](#configuración-del-servidor)
6. [Instalación del Worker](#instalación-del-worker)
7. [Variables de Entorno](#variables-de-entorno)
8. [Protocolo de Comunicación](#protocolo-de-comunicación)
9. [Monitoreo y Logs](#monitoreo-y-logs)
10. [Solución de Problemas](#solución-de-problemas)
11. [Escalabilidad](#escalabilidad)

---

## Arquitectura

```
┌─────────────────────────────────────────────────────────────────────┐
│                           SERVIDOR RSS2                              │
│                                                                      │
│  ┌──────────────┐    ┌──────────────┐    ┌───────────────────────┐  │
│  │   WebSocket  │    │   Job        │    │   Base de Datos      │  │
│  │   Handler    │◄──►│   Assigner   │◄──►│   (PostgreSQL)        │  │
│  │   /ws/worker │    │   (cada 5s)  │    │                       │  │
│  └──────────────┘    └──────────────┘    │   traducciones        │  │
│         ▲                   ▲            │   remote_workers      │  │
│         │                   │            └───────────────────────┘  │
└─────────┼───────────────────┼──────────────────────────────────────┘
          │                   │
          │   WebSocket       │   Trabajo asignado
          │   (api_key)       │   (JSON con job)
          ▼                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      WORKERS REMOTOS                                │
│                                                                      │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐        │
│  │ Worker GPU #1  │  │ Worker GPU #2  │  │ Worker CPU #3  │  ...   │
│  │ (Casa/Servidor)│  │ (Otro equipo)  │  │ (Raspberry Pi) │        │
│  └────────────────┘  └────────────────┘  └────────────────┘        │
└─────────────────────────────────────────────────────────────────────┘
```

### Flujo de Trabajo

1. **Registro**: El worker se conecta al WebSocket con su API key
2. **Heartbeat**: El servidor envía pings cada 10s, el worker responde con heartbeats
3. **Asignación**: Cada 5s, el servidor asigna trabajos pendientes al worker
4. **Traducción**: El worker procesa el trabajo usando CTranslate2/NLLB-200
5. **Resultado**: El worker envía el resultado al servidor
6. **Timeout**: Si un trabajo no se completa en 10min, vuelve a estar disponible

---

## Requisitos

### Para Workers GPU (Recomendado)
- NVIDIA GPU con CUDA 11.8+ 
- Docker con soporte GPU (`nvidia-container-toolkit`)
- 8GB+ RAM disponible
- Conexión a internet para descargar el modelo (~5GB)

### Para Workers CPU
- CPU con soporte AVX2
- 4GB+ RAM disponible (8GB recomendado)
- ~5GB espacio en disco para el modelo
- **Nota**: CPU es ~10x más lento que GPU. Ver [guía CPU](./CPU.md) para detalles.

### Para el Servidor
- Backend RSS2 corriendo (verificar que `/ws/worker` esté accesible)
- PostgreSQL con tabla `remote_workers` creada (ejecutar `35-remote-workers.sql`)
- API key válida en la tabla `remote_workers`

---

## Quick Start GPU

### 1. Crear Worker desde el Panel de Admin

1. Accede a RSS2 como administrador
2. Ve a **Workers de Traducción** (Admin → Workers)
3. En la sección **Workers Remotos**, haz clic en **Añadir Worker**
4. Ingresa un nombre (ej: `gpu-casa`) y selecciona `GPU`
5. **Copia la API key generada** (solo se muestra una vez)

### 2. Iniciar Worker GPU con Docker

```bash
docker run -d --gpus all --name rss2-worker-gpu \
  -e WORKER_API_KEY="tu_api_key_aqui" \
  -e WORKER_SERVER="wss://tu-servidor.com/ws/worker" \
  -e WORKER_NAME="gpu-casa" \
  -e CT2_DEVICE="cuda" \
  -e CT2_COMPUTE_TYPE="float16" \
  -v ~/rss2-models:/app/models \
  rss2-remote-worker:latest
```

### 3. Verificar la Conexión

```bash
docker logs -f rss2-worker-gpu
# Deberías ver: "Connected to server" y luego "Stats: completed=X, failed=Y"
```

En el panel de admin, el worker debería aparecer con un punto verde (online).

---

## Quick Start CPU

**¿No tienes GPU?** Ver la [guía completa de CPU](./CPU.md) para instrucciones detalladas.

Resumen rápido:

```bash
# 1. Crear worker desde Admin Panel (seleccionar "CPU" como capabilities)

# 2. Ejecutar worker CPU
docker run -d --name rss2-worker-cpu \
  -e WORKER_API_KEY="tu_api_key_aqui" \
  -e WORKER_SERVER="ws://tu-servidor:8080/ws/worker" \
  -e CT2_DEVICE="cpu" \
  -e CT2_COMPUTE_TYPE="int8" \
  -e UNIVERSAL_MODEL="facebook/nllb-200-distilled-600M" \
  -v $(pwd)/models:/app/models \
  rss2-remote-worker:latest

# 3. Ver logs
docker logs -f rss2-worker-cpu
```

---

## Configuración del Servidor

### Verificar que el Endpoint WebSocket Existe

El backend debe tener registrado el handler en `/ws/worker`. Si no está, agrégalo en `main.go`:

```go
// En tu función de setup de rutas
r.GET("/ws/worker", handlers.HandleWorkerWS)
```

### Ejecutar el Schema de Base de Datos

```bash
# Conectar a PostgreSQL y ejecutar
psql -h localhost -U rss -d rss -f init-db/35-remote-workers.sql
```

Esto crea:
- Tabla `remote_workers` para almacenar workers y sus API keys
- Columnas `worker_id` y `assigned_at` en `traducciones`
- Índices para búsqueda eficiente

### Verificar Variables de Entorno del Backend

El backend necesita la URL de la base de datos PostgreSQL:

```bash
DATABASE_URL=postgres://user:pass@localhost:5432/rss?sslmode=disable
```

---

## Instalación del Worker

### Opción A: Docker (Recomendado)

#### Construir la Imagen

```bash
cd rss2/remote-worker
docker build -t rss2-remote-worker:latest .
```

#### Ejecutar el Container

```bash
# GPU Worker
docker run -d --gpus all \
  --name rss2-worker-gpu \
  -e WORKER_API_KEY="your-api-key-here" \
  -e WORKER_SERVER="ws://your-server:8080/ws/worker" \
  -e WORKER_NAME="gpu-worker-1" \
  -e CT2_DEVICE="cuda" \
  -e CT2_COMPUTE_TYPE="float16" \
  -v /path/to/models:/app/models \
  rss2-remote-worker:latest

# CPU Worker
docker run -d \
  --name rss2-worker-cpu \
  -e WORKER_API_KEY="your-api-key-here" \
  -e WORKER_SERVER="ws://your-server:8080/ws/worker" \
  -e WORKER_NAME="cpu-worker-1" \
  -e CT2_DEVICE="cpu" \
  -e CT2_COMPUTE_TYPE="int8" \
  -v /path/to/models:/app/models \
  rss2-remote-worker:latest
```

### Opción B: Sin Docker (Python directo)

```bash
# Instalar dependencias
pip install -r requirements.txt

# O instalar manualmente
pip install ctranslate2 transformers websocket-client sentencepiece

# Ejecutar
WORKER_API_KEY="tu-api-key" \
WORKER_SERVER="ws://tu-servidor:8080/ws/worker" \
python worker.py
```

### Opción C: Docker Compose (Workers externos)

En un servidor externo, crear `docker-compose.yml`:

```yaml
version: '3.8'

services:
  rss2-remote-worker:
    image: ghcr.io/tu-usuario/rss2-remote-worker:latest
    environment:
      WORKER_API_KEY: "${WORKER_API_KEY}"
      WORKER_SERVER: "${WORKER_SERVER}"
      WORKER_NAME: "${WORKER_NAME:-remote-worker}"
      CT2_DEVICE: "${CT2_DEVICE:-cuda}"
      CT2_COMPUTE_TYPE: "${CT2_COMPUTE_TYPE:-float16}"
    volumes:
      - ./models:/app/models
      - ./hf_cache:/app/hf_cache
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]
    restart: unless-stopped
    networks:
      - external

networks:
  external:
    external: true
```

```bash
# Crear red Docker externa (para conectar al servidor)
docker network create rss2_external

# Ejecutar
WORKER_API_KEY="tu-key" \
WORKER_SERVER="ws://ip-del-servidor:8080/ws/worker" \
WORKER_NAME="gpu-casa" \
docker compose up -d
```

---

## Variables de Entorno

| Variable | Requerido | Default | Descripción |
|----------|-----------|---------|-------------|
| `WORKER_API_KEY` | **Sí** | - | API key del worker (generada desde el admin) |
| `WORKER_SERVER` | No | `ws://localhost:8080/ws/worker` | URL WebSocket del servidor |
| `WORKER_NAME` | No | `remote-worker` | Nombre identificativo del worker |
| `CT2_DEVICE` | No | `cuda` | Device: `cuda` o `cpu` |
| `CT2_COMPUTE_TYPE` | No | `float16` (cuda), `int8` (cpu) | Tipo de computación |
| `CT2_MODEL_PATH` | No | `/app/models/nllb-ct2` | Ruta del modelo CTranslate2 |
| `UNIVERSAL_MODEL` | No | `facebook/nllb-200-1.3B` | Modelo base de HuggingFace |

### Valores Recomendados por Dispositivo

| Dispositivo | CT2_DEVICE | CT2_COMPUTE_TYPE | Memoria |
|-------------|-------------|-------------------|---------|
| NVIDIA GPU (8GB+) | cuda | float16 | ~6GB |
| NVIDIA GPU (6GB) | cuda | int8 | ~4GB |
| CPU moderno | cpu | int8 | ~4GB |
| CPU antiguo | cpu | int8 | ~4GB |

---

## Protocolo de Comunicación

### Conexión

El worker se conecta a: `ws://servidor:8080/ws/worker?api_key=TU_API_KEY`

### Mensajes Worker → Servidor

```json
// Registro (al conectar)
{"type": "register", "capabilities": "cuda", "worker_name": "gpu-oficina"}

// Heartbeat (respuesta a ping)
{"type": "heartbeat"}

// Resultado de traducción
{
  "type": "result",
  "result": {
    "job_id": 123,
    "title_trad": "Título traducido",
    "summary_trad": "Resumen traducido...",
    "error": ""
  }
}
```

### Mensajes Servidor → Worker

```json
// Acknowledgment
{"type": "ack", "Job": null}

// Ping (cada 10 segundos)
{"type": "ping", "Job": null}

// Nuevo trabajo
{
  "type": "job",
  "Job": {
    "id": 123,
    "lang_from": "en",
    "lang_to": "es",
    "title": "Original title",
    "summary": "Original summary..."
  }
}
```

### Estados del Worker

| Estado | Significado |
|--------|-------------|
| `offline` | Worker desconectado |
| `online` | Worker conectado y activo |
| `disabled` | Worker deshabilitado desde el admin |

### Estados del Trabajo

| Estado | Significado |
|--------|-------------|
| `pending` | Trabajo disponible para asignar |
| `assigned` | Trabajo asignado a un worker |
| `done` | Traducción completada |
| `error` | Error en la traducción |

---

## Monitoreo y Logs

### Logs del Worker

```bash
# Ver logs
docker logs -f rss2-worker-1

# Filtrar solo traducciones
docker logs rss2-worker-1 | grep "Processing job"

# Ver estadísticas
docker logs rss2-worker-1 | grep "Stats:"
# Ejemplo: Stats: completed=150, failed=2
```

### Panel de Administración

En **Admin → Workers de Traducción** puedes ver:

- **Workers activos**: Número de workers locales corriendo
- **Workers remotos online**: Workers remotos conectados
- **Rendimiento**: Traducciones por segundo/minuto
- **Estado individual**: Online/Offline de cada worker remoto

### Verificar Salud del Worker

```bash
# Ver si el container está corriendo
docker ps | grep rss2-worker

# Ver uso de recursos
docker stats rss2-worker-1

# Reiniciar si hay problemas
docker restart rss2-worker-1
```

---

## Solución de Problemas

### Worker no conecta

```
Error: WebSocket connection failed
```

**Soluciones**:
1. Verificar que `WORKER_API_KEY` es correcta
2. Verificar que el servidor está accesible: `curl -I https://tu-servidor/ws/worker`
3. Verificar que el puerto 8080 está abierto

### Modelo no encontrado

```
CTranslate2 model not found at /app/models/nllb-ct2
Downloading from HuggingFace...
```

**Solución**: La primera ejecución descarga el modelo (~5GB). Esperar o pre-descargar:

```bash
# Pre-descargar modelo
ct2-transformers-converter \
  --model facebook/nllb-200-1.3B \
  --output_dir ./models/nllb-ct2 \
  --quantization int8
```

### GPU no detectada

```
RuntimeError: CUDA not available
```

**Soluciones**:
1. Verificar drivers NVIDIA: `nvidia-smi`
2. Verificar nvidia-container-toolkit instalado
3. Usar `--gpus all` en docker run
4. Si todo falla, usar CPU: `CT2_DEVICE=cpu`

### Traducciones muy lentas

**Optimizaciones**:
1. Usar GPU en lugar de CPU (10-20x más rápido)
2. Reducir `CT2_COMPUTE_TYPE` a `int8` para menor uso de memoria
3. Usar modelo más pequeño: `facebook/nllb-200-distilled-600M`
4. Aumentar número de workers

### Jobs se atascan en "assigned"

El timeout de asignación es 10 minutos. Si un worker se desconecta, los jobs vuelven a `pending` automáticamente.

Para verificar:
```sql
SELECT id, status, worker_id, assigned_at 
FROM traducciones 
WHERE status = 'assigned' 
ORDER BY assigned_at DESC;
```

---

## Escalabilidad

### Múltiples Workers

Puedes ejecutar tantos workers como quieras. El servidor reparte trabajos automáticamente:

```bash
# Worker 1
docker run -d --gpus all --name rss2-worker-1 \
  -e WORKER_API_KEY="key-worker-1" ...

# Worker 2
docker run -d --gpus all --name rss2-worker-2 \
  -e WORKER_API_KEY="key-worker-2" ...
```

### arquitectura Recomendada

```
                    ┌─────────────────┐
                    │   Servidor RSS2  │
                    │  (Backend Go)   │
                    │                 │
                    │ /ws/worker       │
                    └────────┬────────┘
                             │
         ┌───────────────────┼───────────────────┐
         │                   │                   │
    ┌────▼────┐         ┌────▼────┐         ┌────▼────┐
    │ Worker  │         │ Worker  │         │ Worker  │
    │ GPU #1  │         │ GPU #2  │         │ CPU #1  │
    │ Casa    │         │ Oficina │         │ VPS     │
    └─────────┘         └─────────┘         └─────────┘
```

### Límites y Consideraciones

| Aspecto | Límite | Notas |
|---------|--------|-------|
| Workers | Ilimitado | El servidor maneja todos |
| GPU por worker | 1 | Por container |
| Memoria por worker | 6-8GB | Recomendado |
| Timeout de job | 10 min | Configurable en `StartJobAssigner()` |

### Comparación GPU vs CPU

| Característica | GPU (cuda) | CPU |
|----------------|------------|-----|
| **Velocidad** | ~15-25 traducciones/min | ~2-5 traducciones/min |
| **Precisión** | Modelo completo (1.3B) | Modelo completo o distilled (600M) |
| **VRAM/RAM** | 4-6GB VRAM | 4-8GB RAM |
| **Costo** | GPU dedicada ($200+) | Ya tienes CPU |
| **Consumo energía** | Alto (200-400W) | Bajo (50-100W) |
| **Mejor para** | Producción, alto volumen | Testing, equipos modestos, VPS |

**Recomendación**: 
- Para producción con volumen alto → **GPU**
- Para testing, desarrollo, o bajo volumen → **CPU** (ver [guía CPU](./CPU.md))

### Balanceo de Carga

El servidor asigna trabajos así:
1. Cada 5 segundos itera sobre workers online
2. Asigna 1 trabajo por worker (round-robin)
3. Prioriza oldest pending jobs

No hay afinidad de lenguaje - cualquier worker puede procesar cualquier idioma.

---

## API de Administración

### Endpoints

| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/admin/workers/remote` | Listar todos los workers |
| POST | `/admin/workers/remote` | Crear nuevo worker |
| GET | `/admin/workers/remote/:id` | Detalles de un worker |
| DELETE | `/admin/workers/remote/:id` | Eliminar worker |
| POST | `/admin/workers/remote/:id/toggle` | Activar/Desactivar |
| POST | `/admin/workers/remote/:id/regenerate-key` | Nueva API key |

### Crear Worker via API

```bash
curl -X POST http://tu-servidor:8080/admin/workers/remote \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"name": "gpu-oficina", "capabilities": "cuda"}'
```

Respuesta:
```json
{
  "id": 3,
  "name": "gpu-oficina",
  "api_key": "rw_k8s9d7f6h5g4j3...",
  "capabilities": "cuda",
  "status": "offline"
}
```

---

## Seguridad

### Recomendaciones

1. **API Keys**: Tratarlas como contraseñas - no exponer en logs ni repositorios
2. **Usar HTTPS/WSS**: En producción, el WebSocket debe ser `wss://` no `ws://`
3. **Firewall**: Limitar acceso al puerto del worker (solo desde el servidor)
4. **Workers deshabilitados**: Si un worker está comprometido, deshabilítalo desde el admin antes de eliminarlo

### Conectar Worker Externo con TLS

```bash
# Crear red overlay para Docker Swarm
docker network create --driver overlay rss2-external

# O usar reverse proxy con Nginx para WebSocket
# ws:// → wss://
```
