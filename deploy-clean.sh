#!/bin/bash
# Script para despliegue limpio de RSS2

echo "=== RSS2 Clean Deployment Script ==="
echo ""

# Detener contenedores
echo "1. Deteniendo contenedores..."
docker compose down -v 2>/dev/null

# Eliminar volúmenes de datos (si hay permisos)
echo "2. Eliminando volúmenes de datos..."
docker volume rm rss2_db 2>/dev/null || true
docker volume rm rss2_redis 2>/dev/null || true

# Si los volúmenes Docker tienen problemas, intentar con rm
echo "   Intentando limpiar /data/..."
sudo rm -rf /datos/rss2/data/pgdata 2>/dev/null || true
sudo rm -rf /datos/rss2/data/redis-data 2>/dev/null || true

# Iniciar base de datos
echo "3. Iniciando base de datos..."
docker compose up -d db

# Esperar a que esté lista
echo "4. Esperando a que la base de datos esté lista..."
sleep 10

# Verificar estado
if docker compose ps db | grep -q "healthy"; then
    echo "   ✓ Base de datos iniciada correctamente"
    
    # Ejecutar script de schema
    echo "5. Ejecutando script de inicialización..."
    docker compose exec -T db psql -U rss -d rss -f /docker-entrypoint-initdb.d/00-complete-schema.sql 2>&1 | tail -5
    
    # Iniciar demás servicios
    echo "6. Iniciando servicios..."
    docker compose up -d redis backend-go rss2_frontend nginx rss-ingestor-go
    
    echo ""
    echo "=== Despliegue completado ==="
    echo "Accede a: http://localhost:8001"
else
    echo "   ✗ Error: La base de datos no está healthy"
    docker compose logs db
fi
