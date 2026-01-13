#!/bin/bash
# Convertir modelo NLLB de HuggingFace a formato CTranslate2
# Ejecutar una vez antes de usar el translation_worker con CTranslate2

set -e

MODEL=${UNIVERSAL_MODEL:-"facebook/nllb-200-distilled-600M"}
OUTPUT_DIR=${CT2_MODEL_PATH:-"./models/nllb-ct2"}
QUANTIZATION=${CT2_QUANTIZATION:-"int8_float16"}

echo "=== Conversión de modelo NLLB a CTranslate2 ==="
echo "Modelo origen: $MODEL"
echo "Directorio destino: $OUTPUT_DIR"
echo "Quantización: $QUANTIZATION"
echo ""

# Verificar que ctranslate2 está instalado
if ! command -v ct2-transformers-converter &> /dev/null; then
    echo "Error: ct2-transformers-converter no encontrado."
    echo "Instala con: pip install ctranslate2"
    exit 1
fi

# Crear directorio si no existe
mkdir -p "$(dirname "$OUTPUT_DIR")"

# Convertir el modelo
echo "Iniciando conversión (puede tardar 5-10 minutos)..."
ct2-transformers-converter \
    --model "$MODEL" \
    --output_dir "$OUTPUT_DIR" \
    --quantization "$QUANTIZATION" \
    --force

echo ""
echo "✓ Conversión completada: $OUTPUT_DIR"
echo ""
echo "Para usar el modelo, establece:"
echo "  export CT2_MODEL_PATH=$OUTPUT_DIR"
