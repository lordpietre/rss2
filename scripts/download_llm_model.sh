#!/bin/bash
# Script para descargar modelo LLM compatible con RTX 3060 12GB

set -e

MODEL_DIR="/home/x/rss2/models/llm"
export PATH="$HOME/.local/bin:$PATH"

echo "=== Descarga de Modelo LLM para Categorización de Noticias ==="
echo ""
echo "Para RTX 3060 12GB, se recomienda un modelo 7B cuantizado."
echo ""
echo "Opciones disponibles:"
echo ""
echo "1) Mistral-7B-Instruct-v0.2 (GPTQ 4-bit) - RECOMENDADO"
echo "   - Tamaño: ~4.5GB"
echo "   - Calidad: Excelente para clasificación"
echo "   - VRAM: ~6-7GB"
echo ""
echo "2) Mistral-7B-Instruct-v0.2 (EXL2 4.0bpw)"
echo "   - Tamaño: ~4.2GB"
echo "   - Calidad: Excelente (optimizado para ExLlamaV2)"
echo "   - VRAM: ~5-6GB"
echo ""
echo "3) OpenHermes-2.5-Mistral-7B (GPTQ 4-bit)"
echo "   - Tamaño: ~4.5GB"
echo "   - Calidad: Muy buena para tareas generales"
echo "   - VRAM: ~6-7GB"
echo ""
echo "4) Neural-Chat-7B-v3-1 (GPTQ 4-bit)"
echo "   - Tamaño: ~4.5GB"
echo "   - Calidad: Buena para español"
echo "   - VRAM: ~6-7GB"
echo ""

read -p "Selecciona una opción (1-4) [1]: " CHOICE
CHOICE=${CHOICE:-1}

case $CHOICE in
    1)
        MODEL_REPO="TheBloke/Mistral-7B-Instruct-v0.2-GPTQ"
        MODEL_FILE="model.safetensors"
        ;;
    2)
        MODEL_REPO="turboderp/Mistral-7B-instruct-exl2"
        MODEL_FILE="4.0bpw"
        ;;
    3)
        MODEL_REPO="TheBloke/OpenHermes-2.5-Mistral-7B-GPTQ"
        MODEL_FILE="model.safetensors"
        ;;
    4)
        MODEL_REPO="TheBloke/neural-chat-7B-v3-1-GPTQ"
        MODEL_FILE="model.safetensors"
        ;;
    *)
        echo "Opción inválida"
        exit 1
        ;;
esac

echo ""
echo "Descargando: $MODEL_REPO"
echo "Destino: $MODEL_DIR"
echo ""

# Crear directorio si no existe
mkdir -p "$MODEL_DIR"

# Verificar si huggingface-cli está instalado
# Verificar si huggingface-cli está instalado o si el modulo existe
# Forzamos actualización a una versión reciente para asegurar soporte de CLI
echo "Actualizando huggingface-hub..."
pip3 install -U "huggingface_hub[cli]>=0.23.0" --break-system-packages

# Descargar modelo usando script de python directo para evitar problemas de CLI
echo "Iniciando descarga..."
python3 -c "
from huggingface_hub import snapshot_download
print(f'Descargando { \"$MODEL_REPO\" } a { \"$MODEL_DIR\" }...')
snapshot_download(repo_id='$MODEL_REPO', local_dir='$MODEL_DIR', local_dir_use_symlinks=False)
"

echo ""
echo "✓ Modelo descargado exitosamente en: $MODEL_DIR"
echo ""
echo "Información del modelo:"
echo "----------------------"
ls -lh "$MODEL_DIR"
echo ""
echo "Para usar este modelo, actualiza docker-compose.yml con:"
echo "  LLM_MODEL_PATH=/app/models/llm"
echo ""
