# Plan de Integración: Remote Worker para RSS2

## Resumen Ejecutivo

El sistema RSS2 incluye soporte para workers de traducción remotos via WebSocket. Esto permite distribuir la carga de traducción en múltiples equipos, incluyendo máquinas con GPU dedicadas, servidores remotos, o incluso equipos modestos como Raspberry Pi.

---

## Arquitectura del Sistema

```
┌─────────────────────────────────────────────────────────────────┐
│                         SERVIDOR PRINCIPAL                        │
│                                                                   │
│  ┌────────────┐  ┌─────────────┐  ┌──────────────┐               │
│  │  Frontend │  │  Backend    │  │  PostgreSQL  │               │
│  │  (React)  │  │  (Go API)   │  │              │               │
│  └────────────┘  └──────┬──────┘  └──────────────┘               │
│                         │                                         │
│                  ┌──────▼──────┐                                 │
│                  │ /ws/worker  │  ◄── WebSocket endpoint        │
│                  │  Handler     │                                 │
│                  └──────┬──────┘                                 │
│                         │                                         │
│                  ┌──────▼──────┐                                 │
│                  │ Job Assigner│  (cada 5 segundos)              │
│                  └─────────────┘                                 │
└───────────────────────────┬─────────────────────────────────────┘
                            │
            ┌───────────────┼────────────────┐
            │               │                │
    ┌───────▼──────┐ ┌──────▼──────┐ ┌──────▼──────┐
    │ Remote GPU   │ │ Remote GPU   │ │ Remote CPU   │
    │ Worker #1    │ │ Worker #2    │ │ Worker #3    │
    │ (Casa)       │ │ (Oficina)   │ │ (VPS)        │
    └──────────────┘ └─────────────┘ └──────────────┘
```

---

## Componentes Necesarios

### 1. Servidor (ya existente)
- Backend Go con endpoint `/ws/worker`
- Base de datos PostgreSQL con schema de remote workers
- Tabla `remote_workers` para almacenar API keys

### 2. Worker Remoto (nuevo)
- Docker container o Python directo
- Conexión WebSocket al servidor
- Modelo CTranslate2 para traducción

---

## Checklist de Implementación

### Fase 1: Preparación del Servidor

- [ ] **1.1** Ejecutar schema de base de datos
  ```bash
  psql -h localhost -U rss -d rss -f init-db/35-remote-workers.sql
  ```

- [ ] **1.2** Verificar que el endpoint `/ws/worker` existe en el backend
  ```go
  // En main.go o router setup
  r.GET("/ws/worker", handlers.HandleWorkerWS)
  ```

- [ ] **1.3** Verificar variable de entorno del backend
  ```bash
  echo $DATABASE_URL
  # Debe ser: postgres://user:pass@host:5432/rss?sslmode=disable
  ```

- [ ] **1.4** Abrir puerto 8080 para WebSocket (si hay firewall)
  ```bash
  # Ubuntu/Debian
  sudo ufw allow 8080/tcp
  ```

### Fase 2: Construir Imagen del Worker

- [ ] **2.1** Construir imagen Docker
  ```bash
  cd rss2/remote-worker
  docker build -t rss2-remote-worker:latest .
  ```

- [ ] **2.2** (Opcional) Pre-descargar modelo
  ```bash
  mkdir -p ./models
  ct2-transformers-converter \
    --model facebook/nllb-200-1.3B \
    --output_dir ./models/nllb-ct2 \
    --quantization int8
  ```

- [ ] **2.3** (Opcional) Push a registry
  ```bash
  docker tag rss2-remote-worker:latest ghcr.io/user/rss2-remote-worker:latest
  docker push ghcr.io/user/rss2-remote-worker:latest
  ```

### Fase 3: Configurar Worker Remoto

- [ ] **3.1** Acceder al panel de admin: `https://tu-servidor/admin/workers`

- [ ] **3.2** Crear nuevo worker remoto
  - Click "Añadir Worker"
  - Nombre: `gpu-casa` (o nombre descriptivo)
  - Capabilities: `cuda` (o `cpu`)
  - **Copiar la API key generada**

- [ ] **3.3** Anotar la información:
  ```
  API Key: rw_xxxxxxxxxxxxxxxxxxxxx
  Server URL: wss://tu-servidor.com/ws/worker
  ```

### Fase 4: Desplegar Worker

#### Opción A: Docker Standalone (máquina física/VPS)

```bash
# En el equipo remoto
docker run -d \
  --name rss2-worker \
  --gpus all \
  -e WORKER_API_KEY="rw_xxxxxxxxxxxxxxxxxxxxx" \
  -e WORKER_SERVER="wss://tu-servidor.com/ws/worker" \
  -e WORKER_NAME="gpu-casa" \
  -e CT2_DEVICE="cuda" \
  -v /root/rss2-models:/app/models \
  rss2-remote-worker:latest
```

#### Opción B: Docker Compose

```yaml
# docker-compose.yml
version: '3.8'

services:
  rss2-remote-worker:
    image: rss2-remote-worker:latest
    container_name: rss2-worker-gpu
    environment:
      WORKER_API_KEY: "${WORKER_API_KEY}"
      WORKER_SERVER: "${WORKER_SERVER}"
      WORKER_NAME: "${WORKER_NAME}"
      CT2_DEVICE: "cuda"
      CT2_COMPUTE_TYPE: "float16"
    volumes:
      - ./models:/app/models
      - ./hf_cache:/app/hf_cache
    restart: unless-stopped
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]
```

```bash
# Variables de entorno
export WORKER_API_KEY="rw_xxxxxxxxxxxxx"
export WORKER_SERVER="wss://tu-servidor.com/ws/worker"
export WORKER_NAME="gpu-casa"

docker compose up -d
```

#### Opción C: Python Directo (desarrollo/testing)

```bash
cd rss2/remote-worker
pip install -r requirements.txt

WORKER_API_KEY="rw_xxxxxxxxxxxxx" \
WORKER_SERVER="ws://tu-servidor:8080/ws/worker" \
python worker.py
```

### Fase 5: Verificar Conexión

- [ ] **5.1** Ver logs del worker
  ```bash
  docker logs -f rss2-worker
  # Deberías ver:
  # Connected to server
  # Registered as gpu-casa
  ```

- [ ] **5.2** Verificar en panel de admin
  - Ir a Admin → Workers de Traducción
  - Sección "Workers Remotos"
  - El worker debe aparecer con punto verde (online)

- [ ] **5.3** Ver estadísticas
  - Esperar 1-2 minutos
  - Ver "Stats: completed=X, failed=Y" en logs

---

## Escenarios de Despliegue

### Escenario 1: Worker en Casa con GPU

```
┌──────────────────────────────────────────────────────┐
│  Casa                                                 │
│  ┌────────────────────────────────────────────────┐  │
│  │  Worker (Docker)                               │  │
│  │  - NVIDIA RTX 3080                             │  │
│  │  - 10GB VRAM                                   │  │
│  │  - Traduce ~20 notas/min                       │  │
│  └────────────────────────────────────────────────┘  │
│                          │                            │
│                     Internet                          │
│                          │                            │
└──────────────────────────┼────────────────────────────┘
                           │
                    ┌──────▼──────┐
                    │  Servidor   │
                    │  RSS2       │
                    │  (VPS/Cloud)│
                    └─────────────┘
```

**Configuración**:
```bash
docker run -d --gpus all \
  --name rss2-worker \
  -e WORKER_API_KEY="rw_home_gpu" \
  -e WORKER_SERVER="wss://rss2.example.com/ws/worker" \
  -e WORKER_NAME="gpu-casa" \
  -e CT2_DEVICE="cuda" \
  -e CT2_COMPUTE_TYPE="float16" \
  -v ~/rss2-models:/app/models \
  rss2-remote-worker:latest
```

### Escenario 2: Worker en VPS con CPU

```
┌──────────────────────────────────────────────────────┐
│  VPS (DigitalOcean/Rackspace)                        │
│  ┌────────────────────────────────────────────────┐  │
│  │  Worker (Docker)                               │  │
│  │  - 4 vCPUs                                     │  │
│  │  - 8GB RAM                                     │  │
│  │  - Traduce ~5 notas/min                        │  │
│  └────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────┘
```

**Configuración**:
```bash
docker run -d \
  --name rss2-worker-cpu \
  -e WORKER_API_KEY="rw_vps_cpu" \
  -e WORKER_SERVER="wss://rss2.example.com/ws/worker" \
  -e WORKER_NAME="vps-cpu" \
  -e CT2_DEVICE="cpu" \
  -e CT2_COMPUTE_TYPE="int8" \
  -v /opt/rss2-models:/app/models \
  rss2-remote-worker:latest
```

### Escenario 3: Múltiples Workers (Cluster)

```
┌─────────────────────────────────────────────────────┐
│                    SERVIDOR RSS2                      │
└───────────────────────┬─────────────────────────────┘
                        │
        ┌───────────────┼───────────────┐
        │               │               │
   ┌────▼────┐    ┌────▼────┐    ┌────▼────┐
   │ GPU #1  │    │ GPU #2  │    │ CPU #1  │
   │ Casa    │    │ Oficina │    │ VPS     │
   └─────────┘    └─────────┘    └─────────┘
```

```bash
# Worker 1 - Casa
docker run -d --gpus all --name rss2-w1 \
  -e WORKER_API_KEY="rw_key_1" ...

# Worker 2 - Oficina  
docker run -d --gpus all --name rss2-w2 \
  -e WORKER_API_KEY="rw_key_2" ...

# Worker 3 - VPS CPU
docker run -d --name rss2-w3 \
  -e WORKER_API_KEY="rw_key_3" ...
```

---

## Operaciones de Mantenimiento

### Ver Estado de Workers

```bash
# Listar containers
docker ps | grep rss2-worker

# Ver uso de recursos
docker stats

# Ver logs recientes
docker logs rss2-worker --tail 50
```

### Regenerar API Key

Si una API key está comprometida:

1. Admin Panel → Workers → Regenerar Key
2. Actualizar variable `WORKER_API_KEY` en el worker
3. Reiniciar: `docker restart rss2-worker`

### Deshabilitar Worker Temporalmente

```bash
# Opción 1: Desde admin panel
# Admin → Workers → Desactivar

# Opción 2: Detener container (los jobs vuelven a pending)
docker stop rss2-worker

# Opción 3: Marcar como disabled en DB
psql -h localhost -U rss -d rss -c \
  "UPDATE remote_workers SET status='disabled' WHERE name='gpu-casa'"
```

### Actualizar Worker

```bash
# Rebuild imagen
docker build -t rss2-remote-worker:latest .

# Pull de registry
docker pull ghcr.io/user/rss2-remote-worker:latest

# Reiniciar con nueva imagen
docker stop rss2-worker
docker rm rss2-worker
docker run -d ... (mismos parámetros)
```

### Remover Worker

1. **Desde el worker**:
   ```bash
   docker stop rss2-worker
   docker rm rss2-worker
   ```

2. **Desde el admin panel**:
   - Admin → Workers → Eliminar

---

## Monitoreo

### Métricas Disponibles

| Métrica | Descripción | Ubicación |
|---------|-------------|-----------|
| `remote_workers_online` | Workers remotos conectados | Admin panel |
| `rate_per_minute` | Traducciones por minuto | Admin panel |
| `jobs_completed` | Total completadas (por worker) | Logs del worker |
| `jobs_failed` | Total fallidas (por worker) | Logs del worker |

### Alertas Recomendadas

```bash
# Alertar si worker está offline por >5 minutos
#!/bin/bash
WORKER_NAME="gpu-casa"
SERVER="tu-servidor.com"

# Check si el worker está online
STATUS=$(curl -s "https://$SERVER/api/admin/workers/remote" | \
  jq -r ".[] | select(.name==\"$WORKER_NAME\") | .status")

if [ "$STATUS" != "online" ]; then
  echo "ALERTA: Worker $WORKER_NAME está offline (status: $STATUS)"
  # Enviar notificación (email, slack, etc.)
fi
```

---

## Troubleshooting

### Problema: Worker conecta pero no recibe trabajos

**Causas posibles**:
1. No hay trabajos pendientes en `traducciones` con `status='pending'`
2. El scheduler no está creando trabajos

**Verificación**:
```sql
-- Ver jobs pendientes
SELECT COUNT(*) FROM traducciones WHERE status='pending';

-- Ver jobs por idioma
SELECT lang_to, COUNT(*) 
FROM traducciones 
WHERE status='pending' 
GROUP BY lang_to;
```

### Problema: Worker se desconecta frecuentemente

**Causas posibles**:
1. Red inestable
2. Firewall corta conexiones inactivas
3. Servidor sobrecargado

**Solución**:
- El worker tiene reconnect automático con backoff
- Verificar latencia de red: `ping tu-servidor.com`
- Revisar logs del servidor

### Problema: Out of Memory en GPU

```
RuntimeError: CUDA out of memory
```

**Solución**: Reducir compute type o usar GPU con más memoria:
```bash
# Reducir precisión
-e CT2_COMPUTE_TYPE="int8"

# O usar CPU como fallback
-e CT2_DEVICE="cpu"
```

---

## Configuración Avanzada

### Usar Modelo Más Pequeño

```bash
# Usar modelo distilled (600M params vs 1.3B)
-e UNIVERSAL_MODEL="facebook/nllb-200-distilled-600M"

# Rebuild con nuevo modelo
docker exec rss2-worker python -c "
from worker import convert_model
convert_model()
"
```

### Workers con Auto-scaling (Docker Swarm)

```yaml
# docker-compose.yml para Swarm
version: '3.8'

services:
  rss2-remote-worker:
    image: rss2-remote-worker:latest
    environment:
      WORKER_API_KEY: "${WORKER_API_KEY}"
      WORKER_SERVER: "${WORKER_SERVER}"
    volumes:
      - models:/app/models
    deploy:
      replicas: 2
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]
    configs:
      - source: worker_config
        target: /app/config.env
```

### Proxy Inverso para WSS

Si necesitas poner el WebSocket detrás de un proxy:

```nginx
# /etc/nginx/sites-available/rss2
server {
    listen 443 ssl;
    server_name tu-servidor.com;

    location /ws/worker {
        proxy_pass http://localhost:8080/ws/worker;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_read_timeout 86400;
    }
}
```

---

## Seguridad

### Checklist de Seguridad

- [ ] Usar `wss://` en producción (no `ws://`)
- [ ] API keys almacenadas como secrets, no en código
- [ ] Firewall limitando acceso a puertos
- [ ] Workers corriendo con usuario no-root
- [ ] No exponer el puerto de PostgreSQL públicamente

### Buenas Prácticas

```bash
# No guardar API keys en git
echo "WORKER_API_KEY=xxx" >> .env
echo ".env" >> .gitignore

# Usar Docker secrets (Swarm)
echo "mi-api-key" | docker secret create worker_key -

# Rotación de keys periódica
# Cada 3-6 meses regenerar keys de workers
```

---

## Resumen de Comandos

```bash
# ===== CONSTRUCCIÓN =====
docker build -t rss2-remote-worker:latest ./remote-worker

# ===== EJECUCIÓN =====
docker run -d --gpus all \
  --name rss2-worker \
  -e WORKER_API_KEY="tu-key" \
  -e WORKER_SERVER="wss://tu-servidor.com/ws/worker" \
  -e WORKER_NAME="gpu-casa" \
  -v /path/to/models:/app/models \
  rss2-remote-worker:latest

# ===== MONITOREO =====
docker logs -f rss2-worker
docker stats rss2-worker
docker exec rss2-worker ps aux

# ===== MANTENIMIENTO =====
docker restart rss2-worker
docker stop rss2-worker && docker start rss2-worker
docker exec rss2-worker rm -rf /app/models && docker restart rss2-worker

# ===== VERIFICACIÓN =====
# Admin panel: https://tu-servidor.com/admin/workers
# Verificar endpoint:
curl -I https://tu-servidor.com/ws/worker
```

---

## Próximos Pasos

1. [ ] Ejecutar schema de base de datos (`35-remote-workers.sql`)
2. [ ] Verificar que `/ws/worker` funciona en el backend
3. [ ] Construir imagen Docker del worker
4. [ ] Crear primer worker desde admin panel
5. [ ] Desplegar worker en un equipo de prueba
6. [ ] Verificar conexión y primeras traducciones
7. [ ] Configurar monitoreo/alerting
8. [ ] Documentar workers activos en runbook
