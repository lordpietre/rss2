#!/bin/bash
# =============================================================================
# Remote Worker Deployment Script - Interactivo y Seguro
# =============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMAGE_NAME="rss2-remote-worker:latest"
DEFAULT_CONTAINER_NAME="rss2-worker"

# Colores
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Trap para cleanup
cleanup() {
    echo -e "\n${YELLOW}Script interrupted${NC}"
    exit 130
}
trap cleanup SIGINT SIGTERM

echo -e "${GREEN}=== Remote Worker Deployment Script ===${NC}"
echo ""

# =============================================================================
# Funciones auxiliares
# =============================================================================

confirm() {
    local prompt="$1"
    local default="${2:-n}"
    local yn
    
    if [ "$default" = "y" ]; then
        prompt="$prompt [Y/n]: "
    else
        prompt="$prompt [y/N]: "
    fi
    
    read -p "$prompt" yn
    case $yn in
        [Yy]*) return 0 ;;
        [Nn]*) return 1 ;;
        "") return $([ "$default" = "y" ] && echo 0 || echo 1) ;;
        *) echo "Invalid input"; return 1 ;;
    esac
}

wait_for_model() {
    echo -e "${YELLOW}Model not found. It will be downloaded (~2GB). This may take several minutes...${NC}"
    echo "To pre-download the model, run:"
    echo "  mkdir -p models && ct2-transformers-converter --model facebook/nllb-200-distilled-600M --output_dir ./models/nllb-ct2 --quantization int8"
    echo ""
    
    local max_wait=600  # 10 minutos max
    local waited=0
    local interval=15
    
    while [ $waited -lt $max_wait ]; do
        # Check on HOST, not in container (because of bind mount)
        if [ -f "${MODELS_DIR}/nllb-ct2/model.bin" ]; then
            MODEL_SIZE=$(du -h "${MODELS_DIR}/nllb-ct2/model.bin" | cut -f1)
            echo -e "${GREEN}Model downloaded successfully! (${MODEL_SIZE})${NC}"
            return 0
        fi
        
        local container_status=$(docker inspect -f '{{.State.Status}}' "${CONTAINER_NAME}" 2>/dev/null || echo "not_found")
        
        if [ "$container_status" = "exited" ]; then
            echo -e "${RED}Container exited! Checking logs:${NC}"
            docker logs "${CONTAINER_NAME}" --tail 30
            return 1
        fi
        
        echo -e "${BLUE}Waiting for model download... ($((max_wait - waited))s remaining)${NC}"
        sleep $interval
        waited=$((waited + interval))
        
        # Show download progress from container logs
        docker logs "${CONTAINER_NAME}" --tail 5 2>/dev/null | grep -E "(Downloading|Downloaded|Converting|Converting model)" || true
    done
    
    echo -e "${RED}Timeout waiting for model download${NC}"
    return 1
}

# =============================================================================
# 1. Pedir parámetros
# =============================================================================

echo -e "${BLUE}--- Configuration ---${NC}"

# Worker name
read -p "Worker name [cpu-worker-1]: " WORKER_NAME
WORKER_NAME="${WORKER_NAME:-cpu-worker-1}"

# API Key (obligatoria)
while [ -z "$API_KEY" ]; do
    read -p "API Key (from remote_workers table): " API_KEY
    if [ -z "$API_KEY" ]; then
        echo -e "${RED}Error: API Key is required${NC}"
    fi
done

# Server URL
read -p "Backend URL [ws://localhost:8080/ws/worker]: " SERVER_URL
SERVER_URL="${SERVER_URL:-ws://localhost:8080/ws/worker}"

# Container name
read -p "Container name [rss2-worker]: " CONTAINER_NAME
CONTAINER_NAME="${CONTAINER_NAME:-${DEFAULT_CONTAINER_NAME}}"

echo ""
echo -e "${BLUE}--- Summary ---${NC}"
echo "Worker Name: ${WORKER_NAME}"
echo "Container:   ${CONTAINER_NAME}"
echo "Server:      ${SERVER_URL}"
echo "API Key:     ${API_KEY:0:8}..."
echo ""

if ! confirm "Continue with these settings?"; then
    echo "Cancelled."
    exit 0
fi

# =============================================================================
# 2. Verificar que docker está corriendo
# =============================================================================

if ! docker info >/dev/null 2>&1; then
    echo -e "${RED}Error: Docker is not running or not accessible${NC}"
    exit 1
fi

# =============================================================================
# 3. Git pull si existe .git
# =============================================================================

echo ""
echo -e "${GREEN}--- Updating code ---${NC}"

if [ -d "${SCRIPT_DIR}/.git" ]; then
    echo "Git pull..."
    if git -C "${SCRIPT_DIR}" pull --ff-only 2>/dev/null; then
        echo -e "${GREEN}Updated successfully${NC}"
    else
        echo -e "${YELLOW}Git pull failed or nothing to update, continuing...${NC}"
    fi
else
    echo -e "${YELLOW}No .git found, skipping update${NC}"
fi

# =============================================================================
# 4. Docker build
# =============================================================================

echo ""
echo -e "${GREEN}--- Building Docker image ---${NC}"

if docker build --no-cache -t "${IMAGE_NAME}" "${SCRIPT_DIR}"; then
    echo -e "${GREEN}Build successful${NC}"
else
    echo -e "${RED}Build failed!${NC}"
    exit 1
fi

# =============================================================================
# 5. Manejo del contenedor existente
# =============================================================================

echo ""
echo -e "${GREEN}--- Managing existing container ---${NC}"

EXISTING=$(docker ps -a --format '{{.Names}}' | grep "^${CONTAINER_NAME}$" || true)

if [ -n "$EXISTING" ]; then
    echo "Container '${CONTAINER_NAME}' exists"
    
    RUNNING=$(docker ps --format '{{.Names}}' | grep "^${CONTAINER_NAME}$" || true)
    if [ -n "$RUNNING" ]; then
        echo "Stopping..."
        docker stop "${CONTAINER_NAME}" || true
    fi
    
    if confirm "Remove old container?" "y"; then
        echo "Removing..."
        docker rm "${CONTAINER_NAME}" || true
    else
        echo -e "${RED}Cannot continue without removing old container${NC}"
        exit 1
    fi
fi

# =============================================================================
# 6. Preparar volumen de modelos
# =============================================================================

echo ""
echo -e "${GREEN}--- Preparing models directory ---${NC}"

MODELS_DIR="${SCRIPT_DIR}/models"
mkdir -p "${MODELS_DIR}"

MODEL_PATH="${MODELS_DIR}/nllb-ct2/model.bin"
if [ -f "${MODEL_PATH}" ]; then
    MODEL_SIZE=$(du -h "${MODEL_PATH}" | cut -f1)
    echo -e "${GREEN}Model found (${MODEL_SIZE}): ${MODEL_PATH}${NC}"
    echo "Note: Model will be re-converted with --force on first run"
else
    echo -e "${YELLOW}Model not found - will be downloaded on first run (~2GB)${NC}"
fi

# =============================================================================
# 7. Docker run
# =============================================================================

echo ""
echo -e "${GREEN}--- Starting container ---${NC}"

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
    -v "${MODELS_DIR}:/app/models" \
    "${IMAGE_NAME}"

if [ $? -ne 0 ]; then
    echo -e "${RED}Failed to start container!${NC}"
    exit 1
fi

echo -e "${GREEN}Container started${NC}"

# =============================================================================
# 8. Verificar startup
# =============================================================================

echo ""
echo -e "${GREEN}--- Verifying startup ---${NC}"

sleep 5

# Check if container is still running
STATUS=$(docker inspect -f '{{.State.Status}}' "${CONTAINER_NAME}" 2>/dev/null || echo "not_found")

if [ "$STATUS" != "running" ]; then
    echo -e "${RED}Container is not running (status: ${STATUS})${NC}"
    echo ""
    echo "Logs:"
    docker logs "${CONTAINER_NAME}" --tail 50
    exit 1
fi

# Show initial logs
echo ""
echo -e "${BLUE}Initial logs:${NC}"
docker logs "${CONTAINER_NAME}" --tail 30

# =============================================================================
# 9. Verificar modelo (si no existía)
# =============================================================================

# Check on HOST (because of bind mount)
if [ ! -f "${MODELS_DIR}/nllb-ct2/model.bin" ]; then
    echo ""
    echo -e "${YELLOW}Model not found locally, waiting for container to download...${NC}"
    
    if ! wait_for_model; then
        echo -e "${RED}Model download failed or timed out${NC}"
        echo ""
        echo "Check logs: docker logs -f ${CONTAINER_NAME}"
        exit 1
    fi
else
    echo -e "${GREEN}Model ready locally${NC}"
fi

# =============================================================================
# 10. Verificar conexión al backend
# =============================================================================

echo ""
echo -e "${GREEN}--- Checking backend connection ---${NC}"

# Wait a bit more for connection attempt
sleep 5

LOGS=$(docker logs "${CONTAINER_NAME}" --tail 50 2>&1)

if echo "$LOGS" | grep -q "connected\|Connected\|WebSocket\|online"; then
    echo -e "${GREEN}Worker appears to be connecting to backend${NC}"
elif echo "$LOGS" | grep -q "error\|Error\|failed\|Failed"; then
    echo -e "${YELLOW}There may be connection issues. Check logs.${NC}"
fi

# =============================================================================
# Final
# =============================================================================

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}=== Deployment Complete! ===${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "Container: ${CONTAINER_NAME}"
echo "API Key:   ${API_KEY:0:8}..."
echo "Server:    ${SERVER_URL}"
echo ""
echo -e "View logs:  ${BLUE}docker logs -f ${CONTAINER_NAME}${NC}"
echo -e "View shell: ${BLUE}docker exec -it ${CONTAINER_NAME} /bin/sh${NC}"
echo -e "Stop:       ${BLUE}docker stop ${CONTAINER_NAME}${NC}"
echo -e "Remove:     ${BLUE}docker rm ${CONTAINER_NAME}${NC}"
echo ""
echo -e "To check backend sees this worker:"
echo -e "  docker exec rss2_db psql -U rss -d rss -c \"SELECT * FROM remote_workers WHERE name='${WORKER_NAME}';\""
echo ""
