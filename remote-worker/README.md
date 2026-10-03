# Remote Workers - Guía de Despliegue

## Sistema Simplificado

El remote worker:
1. Se descarga el código con `git clone` (o `docker run`)
2. El modelo NLLB se descarga **automáticamente** de HuggingFace en la primera ejecución
3. Se conecta al backend por WebSocket usando una API key

## Despliegue Rápido (2 comandos)

### En el servidor principal - Crear API keys:

```bash
docker exec rss2_db psql -U rss -d rss -c "
INSERT INTO remote_workers (name, api_key, capabilities) 
VALUES ('remote-1', md5(random()::text), 'cpu');
INSERT INTO remote_workers (name, api_key, capabilities) 
VALUES ('remote-2', md5(random()::text), 'cpu');
SELECT id, name, api_key FROM remote_workers;
"
```

Guarda las API keys que te devuelve.

### En el host remoto - Un solo comando:

```bash
# Worker 1
docker run -d \
  --name rss2-remote-worker-1 \
  -e WORKER_API_KEY="AQUI_LA_API_KEY_1" \
  -e WORKER_SERVER="ws://TU_SERVIDOR:8080/ws/worker" \
  -e WORKER_NAME="remote-1" \
  -e CT2_DEVICE="cpu" \
  -e CT2_COMPUTE_TYPE="int8" \
  -e MAX_SRC_TOKENS=2048 \
  -e MAX_NEW_TOKENS=2048 \
  -e MAX_BODY_CHARS=80000 \
  -e BODY_CHARS_CHUNK=2000 \
  rss2-remote-worker:latest

# Worker 2 (cambiar el nombre y API key)
docker run -d \
  --name rss2-remote-worker-2 \
  -e WORKER_API_KEY="AQUI_LA_API_KEY_2" \
  -e WORKER_SERVER="ws://TU_SERVIDOR:8080/ws/worker" \
  -e WORKER_NAME="remote-2" \
  -e CT2_DEVICE="cpu" \
  -e CT2_COMPUTE_TYPE="int8" \
  -e MAX_SRC_TOKENS=2048 \
  -e MAX_NEW_TOKENS=2048 \
  -e MAX_BODY_CHARS=80000 \
  -e BODY_CHARS_CHUNK=2000 \
  rss2-remote-worker:latest
```

## Construcción de la Imagen

```bash
git clone https://github.com/tu-user/rss2.git
cd rss2/remote-worker
docker build -t rss2-remote-worker:latest .
```

## Verificación

```bash
# En el host remoto
docker logs -f rss2-remote-worker-1
# Deberías ver:
# - Conectando al WebSocket
# - Descargando modelo de HuggingFace (primera vez)
# - "CTranslate2 model loaded successfully"
# - Procesando jobs

# En el servidor principal
docker exec rss2_db psql -U rss -d rss -c "SELECT * FROM remote_workers;"
# La columna last_seen debe actualizarse
```

## Solución de Problemas

### "Connection refused"
- Verificar que el backend sea accesible desde el host remoto
- Verificar firewall: puerto 8080 abierto

### "Invalid API key"
- Copiar exactamente la API key de la tabla remote_workers
- Sin espacios extra

### "Model conversion failed"
- Normal en primera ejecución si HuggingFace está lento
- Reintentar más tarde

### Muy lento
- CPU normal: ~3-5 traducciones/minuto
- Para más velocidad, necesitas GPU (10-20x más rápido)

## Arquitectura

```
┌─────────────────┐     WebSocket      ┌─────────────────┐
│ Remote Worker 1 │ ────────────────► │    Backend       │
│ (otro host)    │  ws://host:8080   │  (docker-compose)│
└─────────────────┘                   └─────────────────┘
         │                                     ▲
         │                                     │
         ▼                                     │
┌─────────────────┐                            │
│ Remote Worker 2 │ ─────────────────────────────┘
│ (otro host)    │
└─────────────────┘
```

El modelo NLLB se descarga automáticamente en primera ejecución (~2GB).
