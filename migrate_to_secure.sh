#!/bin/bash

# ==================================================================================
# Script de Migración a Configuración Segura - TODO EN UNO
# ==================================================================================

set -e

# Colores
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}🔒 Migración a Configuración Segura - TODO EN UNO${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"

echo -e "${YELLOW}⚠️  Este script hará lo siguiente:${NC}"
echo "  1. Detener los servicios actuales"
echo "  2. Iniciar con la configuración segura"
echo "  3. Verificar que todo funciona"
echo ""
echo -e "${YELLOW}📊 Tiempo estimado: 3-5 minutos${NC}\n"

read -p "¿Deseas continuar? (s/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[SsYy]$ ]]; then
    echo -e "${RED}❌ Operación cancelada${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}PASO 1: Deteniendo servicios actuales...${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"

docker-compose down

echo ""
echo -e "${GREEN}✅ Servicios detenidos${NC}\n"

echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}PASO 2: Iniciando con configuración segura...${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"

docker-compose up -d

echo ""
echo -e "${GREEN}✅ Servicios iniciados${NC}\n"

echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}PASO 3: Esperando que los servicios se inicialicen...${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"

echo -n "Esperando 30 segundos"
for i in {1..30}; do
    echo -n "."
    sleep 1
done
echo ""

echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}PASO 4: Verificando servicios...${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"

docker-compose ps

echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}PASO 5: Ejecutando verificación de seguridad...${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"

./verify_security.sh

echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}PASO 6: Verificando web app...${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"

if curl -s http://localhost:8001 > /dev/null 2>&1; then
    echo -e "${GREEN}✅ Web app responde correctamente${NC}"
else
    echo -e "${RED}❌ Web app no responde - revisar logs:${NC}"
    echo "   docker-compose logs nginx"
    echo "   docker-compose logs rss2_web"
fi

echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}🎉 ¡Migración completada!${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"

echo -e "${GREEN}✅ Tu sistema ahora está configurado de forma segura:${NC}\n"
echo "  🔒 Credenciales fuertes configuradas"
echo "  🌐 Redes segmentadas (frontend, backend, monitoring)"
echo "  🚪 Solo puerto 8001 expuesto públicamente"
echo "  🔐 Redis con autenticación"
echo "  📊 Límites de recursos configurados"
echo ""

echo -e "${YELLOW}📋 PRÓXIMOS PASOS:${NC}\n"
echo "  1. Verifica que puedes acceder a: http://localhost:8001"
echo "  2. Prueba búsqueda y funcionalidades principales"
echo "  3. Para Grafana (monitoring):"
echo "     - Acceso local: http://localhost:3001"
echo "     - Usuario: admin"
echo "     - Password: Ver EJECUTAR_AHORA.md"
echo ""

echo -e "${YELLOW}📖 Documentación:${NC}"
echo "  - EJECUTAR_AHORA.md    → Instrucciones detalladas"
echo "  - SECURITY_GUIDE.md    → Guía completa de seguridad"
echo "  - SECURITY_AUDIT.md    → Resumen de auditoría"
echo ""

echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
