# Remote Translator Worker

Worker remoto de traducción para RSS2. Se conecta al servidor via WebSocket y procesa trabajos de traducción automáticamente.

## Requisitos

- Docker con soporte GPU (NVIDIA)
- O Python 3.11+ con CUDA

## Uso con Docker

```bash
docker run -d --gpus all --name remote-worker \
  -e WORKER_API_KEY="tu_api_key" \
  -e WORKER_SERVER="ws://tu-servidor:8888/ws/worker" \
  -v ~/models:/app/models \
  tu-usuario/rss2-remote-worker
```

## Variables de Entorno

| Variable | Default | Descripción |
|----------|---------|-------------|
| WORKER_API_KEY | (requerido) | API key del worker |
| WORKER_SERVER | ws://localhost:8080/ws/worker | URL del servidor WebSocket |
| CT2_DEVICE | cuda | Device: cuda o cpu |
| CT2_COMPUTE_TYPE | float16 | Tipo de computación |
| CT2_MODEL_PATH | /app/models/nllb-ct2 | Ruta del modelo |
| UNIVERSAL_MODEL | facebook/nllb-200-1.3B | Modelo de HuggingFace |
| WORKER_NAME | remote-worker | Nombre del worker |

## Sin Docker

```bash
pip install -r requirements.txt

python worker.py --api-key tu_api_key --server ws://tu-servidor:8888/ws/worker
```

## Modelo

La primera vez que ejecutas el worker, descargará automáticamente el modelo desde HuggingFace y lo convertirá a formato CTranslate2 (~5-10 min).

Para acelerar instalaciones posteriores, puedes pre-descargar el modelo:
```bash
ct2-transformers-converter --model facebook/nllb-200-1.3B --output_dir ./models/nllb-ct2 --quantization int8
```

## Construir imagen

```bash
docker build -t rss2-remote-worker .
```