#!/bin/bash
# ==============================================================================
# Pre-deployment model preparation for translator-gpu
# Ensures the NLLB-1.3B model is properly converted before starting workers
# ==============================================================================

set -e

MODEL_DIR="./models/nllb-ct2-1.3b"
REQUIRED_FILES=("model.bin" "config.json" "shared_vocabulary.json")
MIN_SIZES=(1000000 100 1000000)  # Minimum sizes in bytes

echo "========================================"
echo "  Pre-Deploy Model Preparation"
echo "========================================"

# Check if model directory exists
if [ ! -d "$MODEL_DIR" ]; then
    echo "Creating model directory..."
    mkdir -p "$MODEL_DIR"
fi

# Check if all required files exist and have minimum size
all_present=true
for i in "${!REQUIRED_FILES[@]}"; do
    file="${REQUIRED_FILES[$i]}"
    min_size="${MIN_SIZES[$i]}"
    
    if [ -f "$MODEL_DIR/$file" ]; then
        size=$(stat -c%s "$MODEL_DIR/$file" 2>/dev/null || stat -f%z "$MODEL_DIR/$file" 2>/dev/null || echo 0)
        if [ "$size" -lt "$min_size" ]; then
            echo "  ✗ $file is too small ($size bytes), needs re-conversion"
            all_present=false
            break
        else
            echo "  ✓ $file OK ($(numfmt --to=iec-i --suffix=B $size))"
        fi
    else
        echo "  ✗ $file missing"
        all_present=false
    fi
done

if [ "$all_present" = true ]; then
    echo ""
    echo "✓ Model files verified - workers can start immediately"
    exit 0
fi

echo ""
echo "⚠ Model incomplete or missing - will be converted by first worker"
echo "  This may take 5-10 minutes on first deployment"
echo ""
echo "To pre-convert the model manually (optional):"
echo "  docker run --rm -v \$(pwd)/models:/app/models rss2-translator-gpu:latest \\"
echo "    ct2-transformers-converter --model facebook/nllb-200-1.3B \\"
echo "    --output_dir /app/models/nllb-ct2-1.3b --quantization float16 --force"
echo ""

exit 0