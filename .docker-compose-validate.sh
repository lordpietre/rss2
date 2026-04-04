#!/bin/bash
# Validador automático de credenciales para docker compose
# Se ejecuta automáticamente antes de iniciar los servicios

set -e

# Colores
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}🔐 Validando credenciales antes de iniciar...${NC}"

# Verificar archivo .env
if [ ! -f .env ]; then
    echo -e "${RED}❌ ERROR: Archivo .env no encontrado${NC}"
    echo -e "${YELLOW}💡 Generando credenciales...${NC}"
    ./generate_secure_credentials.sh --force
    exit $?
fi

# Verificar credenciales CRÍTICAS
for var in POSTGRES_PASSWORD REDIS_PASSWORD DB_PASS; do
    value=$(grep "^${var}=" .env 2>/dev/null | cut -d'=' -f2 | tr -d ' ')
    
    if [ -z "$value" ] || [ "$value" = "change_this_to_a_long_random_string" ] || [ "$value" = "change_this_password" ]; then
        echo -e "${RED}❌ ERROR: $var está vacía o tiene valor por defecto${NC}"
        echo -e "${YELLOW}💡 Generando credenciales seguras...${NC}"
        ./generate_secure_credentials.sh --force
        exit $?
    fi
done

echo -e "${GREEN}✅ Todas las credenciales están definidas${NC}"
exit 0
