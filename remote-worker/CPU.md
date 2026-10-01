# Remote Worker - Ejecución por CPU

Esta guía explica cómo ejecutar el remote worker usando CPU en lugar de GPU.

## Cuándo usar CPU

| Escenario | Recomendación |
|-----------|---------------|
| No tienes GPU NVIDIA | **Usar CPU** |
| Testing/desarrollo | CPU (más económico) |
| Raspberry Pi o ARM | **Solo CPU** |
| Equipo antiguo | CPU |
| Producción con alto volumen | GPU (mucho más rápido) |

## Requisitos

- **CPU**: Con soporte AVX2 (casi todos los CPU Intel/AMD desde 2013+)
- **RAM**: 4GB mínimo (8GB recomendado)
- **Disco**: 5GB para el modelo
- **Sin GPU NVIDIA necesaria**

## Rendimiento Esperado

| Modelo | Traducciones/min | RAM needed |
|--------|------------------|------------|
| `facebook/nllb-200-distilled-600M` | ~3-5/min | ~2GB |
| `facebook/nllb-200-1.3B` | ~1-2/min | ~4GB |

> **Nota**: GPU es ~10-20x más rápido que CPU para esta tarea.

---

## Instalación Rápida

### 1. Configurar entorno

```bash
cd rss2/remote-worker

# Copiar configuración CPU
cp .env.example .env

# Editar con tu API key y servidor
nano .env
```

Contenido del `.env` para CPU:

```bash
# === OBLIGATORIO ===
WORKER_API_KEY=rw_tu_api_key_aqui
WORKER_SERVER=ws://tu-servidor:8080/ws/worker

# === CPU (OBLIGATORIO) ===
CT2_DEVICE=cpu
CT2_COMPUTE_TYPE=int8

# === OPCIONAL ===
WORKER_NAME=cpu-traductor-1
UNIVERSAL_MODEL=facebook/nllb-200-distilled-600M
```

### 2. Construir imagen

```bash
docker build -t rss2-remote-worker:latest .
```

### 3. Ejecutar worker

```bash
# Usando docker-compose (recomendado)
docker compose --profile cpu up -d

# O manualmente
docker run -d \
  --name rss2-worker-cpu \
  -e WORKER_API_KEY="tu-api-key" \
  -e WORKER_SERVER="ws://tu-servidor:8080/ws/worker" \
  -e CT2_DEVICE="cpu" \
  -e CT2_COMPUTE_TYPE="int8" \
  -e UNIVERSAL_MODEL="facebook/nllb-200-distilled-600M" \
  -v $(pwd)/models:/app/models \
  rss2-remote-worker:latest
```

### 4. Verificar

```bash
# Ver logs
docker logs -f rss2-worker-cpu

# Esperar ver algo como:
# Connected to server
# Loading CTranslate2 model from /app/models/nllb-ct2 on cpu
# CTranslate2 model loaded successfully
# Stats: completed=0, jobs_failed=0
```

---

## Docker Compose Específico para CPU

Crea `docker-compose.cpu.yml`:

```yaml
version: '3.8'

services:
  rss2-remote-worker-cpu:
    build:
      context: .
      dockerfile: Dockerfile
    container_name: rss2-worker-cpu
    environment:
      WORKER_API_KEY: "${WORKER_API_KEY}"
      WORKER_SERVER: "${WORKER_SERVER}"
      WORKER_NAME: "cpu-traductor-1"
      CT2_DEVICE: "cpu"
      CT2_COMPUTE_TYPE: "int8"
      UNIVERSAL_MODEL: "facebook/nllb-200-distilled-600M"
    volumes:
      - ./models:/app/models
      - ./hf_cache:/app/hf_cache
    restart: unless-stopped
```

Ejecutar:

```bash
# Con archivo específico
WORKER_API_KEY="tu-key" WORKER_SERVER="ws://servidor:8080/ws/worker" \
docker compose -f docker-compose.cpu.yml up -d

# O exportar variables y usar
export WORKER_API_KEY="tu-key"
export WORKER_SERVER="ws://servidor:8080/ws/worker"
docker compose -f docker-compose.cpu.yml up -d
```

---

## Sin Docker (Python Directo)

Si prefieres ejecutar sin Docker:

### 1. Instalar dependencias

```bash
pip install ctranslate2>=3.24.0 transformers>=4.36.0 websocket-client sentencepiece
```

### 2. Instalar torch (CPU version)

```bash
# Solo CPU, sin CUDA
pip install torch torchvision --index-url https://download.pytorch.org/whl/cpu
```

### 3. Configurar y ejecutar

```bash
export WORKER_API_KEY="tu-api-key"
export WORKER_SERVER="ws://tu-servidor:8080/ws/worker"
export CT2_DEVICE="cpu"
export CT2_COMPUTE_TYPE="int8"

python worker.py
```

---

## Pre-descargar Modelo para CPU

La primera ejecución descarga el modelo (~2GB para 600M, ~5GB para 1.3B). Para pre-descargar:

```bash
# Crear directorio
mkdir -p models

# Descargar y convertir modelo 600M (más rápido para CPU)
ct2-transformers-converter \
  --model facebook/nllb-200-distilled-600M \
  --output_dir ./models/nllb-ct2 \
  --quantization int8

# O modelo completo 1.3B (más preciso, más lento en CPU)
ct2-transformers-converter \
  --model facebook/nllb-200-1.3B \
  --output_dir ./models/nllb-ct2 \
  --quantization int8
```

---

## Troubleshooting CPU

### Error: "CUDA not available"

Esto es **normal** si usas CPU. Verifica que `CT2_DEVICE=cpu` esté configurado.

### Error: "Killed" o Out of Memory

Tu equipo no tiene suficiente RAM. Soluciones:

1. Usar modelo pequeño: `UNIVERSAL_MODEL=facebook/nllb-200-distilled-600M`
2. Reducir swap
3. Cerrar otras aplicaciones

### Traducciones muy lentas

Es normal en CPU. Recomendaciones:

1. Usar modelo 600M en lugar de 1.3B
2. Usar GPU si possible (10-20x más rápido)

### Error: "Illegal instruction"

Tu CPU no soporta AVX2. Opciones:

1. Compilar CTranslate2 desde código fuente
2. Usar una versión más antigua del modelo
3. Cambiar de equipo

---

## Comparación GPU vs CPU

| Aspecto | GPU (cuda) | CPU |
|---------|------------|-----|
| Velocidad | ~20 traducciones/min | ~3-5 traducciones/min |
| RAM | 4-6GB VRAM | 4-8GB RAM |
| Costo | GPU dedicada ~$200+ | Ya tienes CPU |
| Consumo | Alto | Bajo |
| Mejor para | Producción | Testing, equipos modestos |

---

##Ejemplo Completo en VPS (1GB RAM)

Si tienes un VPS con recursos limitados:

```bash
# Usar modelo pequeño y limitar threads
docker run -d \
  --name rss2-worker \
  --cpus="2" \
  --memory="1g" \
  -e WORKER_API_KEY="tu-key" \
  -e WORKER_SERVER="ws://tu-servidor:8080/ws/worker" \
  -e CT2_DEVICE="cpu" \
  -e CT2_COMPUTE_TYPE="int8" \
  -e UNIVERSAL_MODEL="facebook/nllb-200-distilled-600M" \
  -e OMP_NUM_THREADS=2 \
  rss2-remote-worker:latest
```
