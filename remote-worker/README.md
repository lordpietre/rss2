# Remote Workers - Guía de Despliegue

## Sistema Simplificado

El remote worker:
1. Se conecta al backend por WebSocket usando una API key
2. El modelo NLLB se descarga **automáticamente** de HuggingFace en la primera ejecución (~2GB)
3. Compite por jobs de traducción con los workers locales usando `FOR UPDATE SKIP LOCKED`

## Requisitos

- Docker
- CPU con soporte AVX2 (casi todos desde 2013)
- ~4GB RAM
- ~5GB disco para el modelo

## Despliegue Rápido (2 pasos)

### 1. Crear API Key en el servidor principal

```bash
docker exec rss2_db psql -U rss -d rss -c "
INSERT INTO remote_workers (name, api_key, capabilities) 
VALUES ('mi-worker', md5(random()::text), 'cpu')
RETURNING id, name, api_key;
"
```

### 2. Ejecutar deploy.sh en el host remoto

```bash
cd ~/rss2/remote-worker
./deploy.sh
```

El script es **interactivo** y te preguntará:
- Nombre del worker
- API Key
- URL del backend
- Nombre del contenedor

## Opciones de Despliegue

### Opción 1: Script interactivo (recomendado)

```bash
./deploy.sh
```

El script:
- Hace git pull automáticamente
- Construye la imagen Docker
- Maneja contenedores existentes
- Espera a que el modelo se descargue
- Verifica la conexión al backend

### Opción 2: Docker Compose (múltiples workers)

```bash
# Crear archivo .env
cat > .env << EOF
BACKEND_HOST=192.168.1.193
WORKER_API_KEY_1=tu-api-key-1
WORKER_API_KEY_2=tu-api-key-2
EOF

# Iniciar
docker compose up -d
```

### Opción 3: Docker Run directo

```bash
docker run -d \
  --name rss2-worker \
  -e WORKER_API_KEY="tu-api-key" \
  -e WORKER_SERVER="ws://tu-servidor:8080/ws/worker" \
  -e WORKER_NAME="mi-worker" \
  -e CT2_DEVICE="cpu" \
  -e CT2_COMPUTE_TYPE="int8" \
  -e MAX_SRC_TOKENS=2048 \
  -e MAX_NEW_TOKENS=2048 \
  -e MAX_BODY_CHARS=80000 \
  -e BODY_CHARS_CHUNK=2000 \
  -e MAX_SEQ_PER_CALL=32 \
  -v $(pwd)/models:/app/models \
  rss2-remote-worker:latest
```

## Verificación

### En el servidor principal

```bash
# Ver workers remotos
docker exec rss2_db psql -U rss -d rss -c "SELECT * FROM remote_workers;"

# Ver traducciones por worker
docker exec rss2_db psql -U rss -d rss -c "
SELECT 
    t.worker_id,
    COUNT(*) as total,
    SUM(CASE WHEN t.status='done' THEN 1 ELSE 0 END) as done
FROM traducciones t
WHERE t.worker_id IS NOT NULL
GROUP BY t.worker_id;
"
```

### En el host remoto

```bash
# Ver logs
docker logs -f rss2-worker

# Ver modelo descargado
ls -la models/nllb-ct2/
```

## Parámetros del Worker

| Variable | Default | Descripción |
|----------|---------|-------------|
| `WORKER_API_KEY` | (requerido) | API key de la tabla `remote_workers` |
| `WORKER_SERVER` | `ws://localhost:8080/ws/worker` | URL del backend WebSocket |
| `WORKER_NAME` | `remote-worker` | Nombre identificativo |
| `CT2_DEVICE` | `cpu` | `cpu` o `cuda` |
| `CT2_COMPUTE_TYPE` | `int8` | `int8` para CPU, `float16` para GPU |
| `MAX_SRC_TOKENS` | `2048` | Tokens máximos origen |
| `MAX_NEW_TOKENS` | `2048` | Tokens máximos traducción |
| `MAX_BODY_CHARS` | `80000` | Caracteres máximos por texto |
| `BODY_CHARS_CHUNK` | `2000` | Tamaño de chunks |
| `MAX_SEQ_PER_CALL` | `32` | Secuencias por llamada |

## Arquitectura

```
┌─────────────────┐     WebSocket      ┌─────────────────┐
│ Remote Worker 1 │ ────────────────► │    Backend       │
│ (host externo)  │  ws://host:8080  │  (docker-compose)│
└─────────────────┘                   └─────────────────┘
         │                                     ▲
         │                                     │
         ▼                                     │
┌─────────────────┐                           │
│ Remote Worker 2  │ ────────────────────────────┘
│ (host externo)  │
└─────────────────┘
```

## Troubleshooting

### "Connection refused"
- Verificar que el backend sea accesible
- Verificar firewall: puerto 8080 abierto

### "Invalid API key"
- Copiar exactamente la API key de la tabla `remote_workers`
- Sin espacios extra

### "Model conversion failed"
- Normal en primera ejecución si HuggingFace está lento
- El script espera y reintenta automáticamente

### "Model not found at /app/models/nllb-ct2"
- El modelo no se ha descargado aún
- Esperar ~5 minutos o pre-descargar manualmente:
```bash
mkdir -p models
ct2-transformers-converter \
  --model facebook/nllb-200-distilled-600M \
  --output_dir ./models/nllb-ct2 \
  --quantization int8 --force
```

### Worker muy lento
- CPU normal: ~3-5 traducciones/minuto
- Es normal, la traducción en CPU es lenta
- Considerar GPU para mayor velocidad (10-20x más rápido)

## Actualización

Para actualizar el worker:

```bash
cd ~/rss2/remote-worker
./deploy.sh
```

El script automáticamente:
1. Hace `git pull` para obtener el código actualizado
2. Reconstruye la imagen Docker
3. Reinicia el contenedor con la nueva versión
