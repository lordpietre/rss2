#!/bin/bash
# =============================================================================
# Remote Worker Deployment Script
# Uso: ./deploy.sh [worker_name] [api_key] [server_url]
# Ejemplo: ./deploy.sh cpu-1 31ec77e7d809a5c38c3f0b3888291f2decb2a442cb41bd11660ddbccee8b46bc ws://192.168.1.193:8080/ws/worker
# =============================================================================

set -e

WORKER_NAME="${1:-cpu-worker}"
API_KEY="${2:-}"
SERVER_URL="${3:-ws://localhost:8080/ws/worker}"
CONTAINER_NAME="rss2-worker-${WORKER_NAME}"
IMAGE_NAME="rss2-remote-worker:latest"

# Validar parámetros
if [ -z "$API_KEY" ]; then
    echo "Error: Falta API_KEY"
    echo "Uso: $0 [worker_name] [api_key] [server_url]"
    echo "Ejemplo: $0 cpu-1 31ec77e7d809a5c38c3f0b3888291f2decb2a442cb41bd11660ddbccee8b46bc ws://192.168.1.193:8080/ws/worker"
    exit 1
fi

echo "=== Deploying Remote Worker ==="
echo "Worker: ${WORKER_NAME}"
echo "Container: ${CONTAINER_NAME}"
echo "Server: ${SERVER_URL}"

# 1. Git pull (si existe .git)
if [ -d ".git" ]; then
    echo ">>> Git pull..."
    git pull
fi

# 2. Docker build
echo ">>> Building Docker image..."
docker build --no-cache -t "${IMAGE_NAME}" .

# 3. Stop and remove old container
echo ">>> Stopping old container..."
docker stop "${CONTAINER_NAME}" 2>/dev/null || true
docker rm "${CONTAINER_NAME}" 2>/dev/null || true

# 4. Create models directory if not exists
mkdir -p models

# 5. Docker run
echo ">>> Starting new container..."
docker run -d \
    --name "${CONTAINER_NAME}" \
    -e WORKER_API_KEY="${API_KEY}" \
    -e WORKER_SERVER="${SERVER_URL}" \
    -e WORKER_NAME="${WORKER_NAME}" \
    -e CT2_DEVICE="cpu" \
    -e CT2_COMPUTE_TYPE="int8" \
    -e MAX_SRC_TOKENS=2048 \
    -e MAX_NEW_TOKENS=2048 \
    -e MAX_BODY_CHARS=80000 \
    -e BODY_CHARS_CHUNK=2000 \
    -e MAX_SEQ_PER_CALL=32 \
    -v $(pwd)/models:/app/models \
    "${IMAGE_NAME}"

echo ">>> Waiting for startup..."
sleep 5

echo ">>> Checking logs..."
docker logs "${CONTAINER_NAME}" --tail 10

echo ""
echo "=== Deployed successfully! ==="
echo "Container: ${CONTAINER_NAME}"
echo ""
echo "To view logs: docker logs -f ${CONTAINER_NAME}"
echo "To stop: docker stop ${CONTAINER_NAME}"
