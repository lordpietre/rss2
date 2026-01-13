#!/bin/bash
# Script para iniciar los servicios de Docker
# Ejecutar con: sudo ./start_docker.sh

set -e
cd "$(dirname "$0")"

echo "=== RSS2 Docker Services ==="

# Verificación de modelo eliminada (script de conversión no disponible)

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
