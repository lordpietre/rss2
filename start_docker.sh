#!/bin/bash
# Script para iniciar los servicios de Docker
# Ejecutar con: sudo ./start_docker.sh

set -e
cd "$(dirname "$0")"

echo "=== RSS2 Docker Services ==="

# Verificar si el modelo CTranslate2 existe
CT2_MODEL="./models/nllb-ct2"
if [ ! -d "$CT2_MODEL" ]; then
    echo ""
    echo "⚠️  Modelo CTranslate2 no encontrado en $CT2_MODEL"
    echo "   Convirtiendo modelo (esto puede tardar 5-10 minutos)..."
    echo ""
    
    # Verificar si ctranslate2 está instalado
    if ! python3 -c "import ctranslate2" 2>/dev/null; then
        echo "Instalando ctranslate2..."
        pip install ctranslate2
    fi
    
    # Convertir el modelo
    ./convert_model.sh
fi

echo ""
echo "Iniciando servicios Docker..."
docker compose up -d --build

echo ""
echo "✓ Servicios iniciados"
echo ""
echo "Para ver los logs:"
echo "  docker compose logs -f translator"
echo ""
echo "Para verificar el estado:"
echo "  docker compose ps"
