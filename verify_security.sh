#!/bin/bash

# ==================================================================================
# Script de Verificación de Seguridad
# ==================================================================================
# 
# Este script verifica que la configuración segura esté correctamente implementada
#
# Uso:
#   ./verify_security.sh
#
# ==================================================================================

set -e

# Colores
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

TOTAL_CHECKS=0
PASSED_CHECKS=0
FAILED_CHECKS=0

echo -e "${BLUE}=================================="
echo -e "🔍 Verificación de Seguridad"
echo -e "==================================${NC}\n"

# Función para verificar
check() {
    local test_name="$1"
    local command="$2"
    local expected="$3"
    
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
    
    echo -n "Verificando: $test_name... "
    
    if eval "$command" > /dev/null 2>&1; then
        if [ "$expected" = "pass" ]; then
            echo -e "${GREEN}✅ PASS${NC}"
            PASSED_CHECKS=$((PASSED_CHECKS + 1))
            return 0
        else
            echo -e "${RED}❌ FAIL${NC}"
            FAILED_CHECKS=$((FAILED_CHECKS + 1))
            return 1
        fi
    else
        if [ "$expected" = "fail" ]; then
            echo -e "${GREEN}✅ PASS${NC}"
            PASSED_CHECKS=$((PASSED_CHECKS + 1))
            return 0
        else
            echo -e "${RED}❌ FAIL${NC}"
            FAILED_CHECKS=$((FAILED_CHECKS + 1))
            return 1
        fi
    fi
}

# Verificar que docker-compose está instalado
if ! command -v docker-compose &> /dev/null; then
    echo -e "${RED}❌ docker-compose no está instalado${NC}"
    exit 1
fi

echo -e "${YELLOW}📋 Verificando configuración de red...${NC}\n"

# Verificar que las redes están creadas
check "Red frontend existe" "docker network inspect rss2_frontend" "pass"
check "Red backend existe" "docker network inspect rss2_backend" "pass"
check "Red monitoring existe" "docker network inspect rss2_monitoring" "pass"

echo ""
echo -e "${YELLOW}📋 Verificando servicios en ejecución...${NC}\n"

# Verificar servicios críticos
check "Contenedor DB corriendo" "docker ps | grep rss2_db" "pass"
check "Contenedor Redis corriendo" "docker ps | grep rss2_redis" "pass"
check "Contenedor Web corriendo" "docker ps | grep rss2_web" "pass"
check "Contenedor Nginx corriendo" "docker ps | grep rss2_nginx" "pass"
check "Contenedor Qdrant corriendo" "docker ps | grep rss2_qdrant" "pass"

echo ""
echo -e "${YELLOW}📋 Verificando exposición de puertos...${NC}\n"

# Verificar que puertos internos NO están expuestos
check "Qdrant NO expuesto públicamente" "! docker ps | grep '0.0.0.0:6333'" "pass"
check "Prometheus NO expuesto públicamente" "! docker ps | grep '0.0.0.0:9090'" "pass"
check "cAdvisor NO expuesto públicamente" "! docker ps | grep '0.0.0.0:8081'" "pass"

# Verificar que puerto web SÍ está expuesto
check "Nginx expuesto en puerto 8001" "docker ps | grep '0.0.0.0:8001->80'" "pass"

echo ""
echo -e "${YELLOW}📋 Verificando variables de entorno...${NC}\n"

# Verificar .env existe
if [ ! -f .env ]; then
    echo -e "${RED}❌ Archivo .env no existe${NC}"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
else
    echo -e "${GREEN}✅ Archivo .env existe${NC}"
    PASSED_CHECKS=$((PASSED_CHECKS + 1))
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
    
    # Verificar que las credenciales han sido cambiadas
    if grep -q "POSTGRES_PASSWORD=x" .env 2>/dev/null; then
        echo -e "${RED}❌ POSTGRES_PASSWORD sigue siendo 'x' (INSEGURO)${NC}"
        FAILED_CHECKS=$((FAILED_CHECKS + 1))
    else
        echo -e "${GREEN}✅ POSTGRES_PASSWORD ha sido cambiado${NC}"
        PASSED_CHECKS=$((PASSED_CHECKS + 1))
    fi
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
    
    if grep -q "SECRET_KEY=secret" .env 2>/dev/null; then
        echo -e "${RED}❌ SECRET_KEY sigue siendo 'secret' (INSEGURO)${NC}"
        FAILED_CHECKS=$((FAILED_CHECKS + 1))
    else
        echo -e "${GREEN}✅ SECRET_KEY ha sido cambiado${NC}"
        PASSED_CHECKS=$((PASSED_CHECKS + 1))
    fi
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
    
    if grep -q "REDIS_PASSWORD" .env 2>/dev/null; then
        echo -e "${GREEN}✅ REDIS_PASSWORD está configurado${NC}"
        PASSED_CHECKS=$((PASSED_CHECKS + 1))
    else
        echo -e "${RED}❌ REDIS_PASSWORD no está configurado${NC}"
        FAILED_CHECKS=$((FAILED_CHECKS + 1))
    fi
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
fi

echo ""
echo -e "${YELLOW}📋 Verificando límites de recursos...${NC}\n"

# Verificar que hay límites de recursos configurados
if docker-compose config 2>/dev/null | grep -q "limits:" ; then
    echo -e "${GREEN}✅ Límites de recursos configurados${NC}"
    PASSED_CHECKS=$((PASSED_CHECKS + 1))
else
    echo -e "${RED}❌ No hay límites de recursos configurados${NC}"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
fi
TOTAL_CHECKS=$((TOTAL_CHECKS + 1))

echo ""
echo -e "${YELLOW}📋 Verificando conectividad de servicios...${NC}\n"

# Verificar que el sitio web responde
if curl -s http://localhost:8001 > /dev/null 2>&1; then
    echo -e "${GREEN}✅ Sitio web responde correctamente${NC}"
    PASSED_CHECKS=$((PASSED_CHECKS + 1))
else
    echo -e "${RED}❌ Sitio web no responde${NC}"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
fi
TOTAL_CHECKS=$((TOTAL_CHECKS + 1))

# Verificar Redis con autenticación (si está configurado)
if docker exec rss2_web python3 -c "
import os
import redis
try:
    password = os.getenv('REDIS_PASSWORD')
    config = {'host': 'redis', 'port': 6379}
    if password:
        config['password'] = password
    r = redis.Redis(**config)
    r.ping()
    print('OK')
except Exception as e:
    print(f'ERROR: {e}')
    exit(1)
" 2>&1 | grep -q "OK"; then
    echo -e "${GREEN}✅ Redis autenticado funciona correctamente${NC}"
    PASSED_CHECKS=$((PASSED_CHECKS + 1))
else
    echo -e "${RED}❌ Redis no responde correctamente${NC}"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
fi
TOTAL_CHECKS=$((TOTAL_CHECKS + 1))

echo ""
echo -e "${BLUE}=================================="
echo -e "📊 Resultados de la Verificación"
echo -e "==================================${NC}\n"

echo -e "Total de verificaciones: $TOTAL_CHECKS"
echo -e "${GREEN}✅ Pasadas: $PASSED_CHECKS${NC}"
echo -e "${RED}❌ Fallidas: $FAILED_CHECKS${NC}"

PERCENTAGE=$((PASSED_CHECKS * 100 / TOTAL_CHECKS))
echo -e "\nPorcentaje de éxito: ${GREEN}${PERCENTAGE}%${NC}\n"

if [ $FAILED_CHECKS -eq 0 ]; then
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}🎉 ¡Todas las verificaciones pasaron correctamente!${NC}"
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "\n${GREEN}✅ El sistema está configurado de forma segura${NC}\n"
    exit 0
else
    echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${RED}⚠️  Algunas verificaciones fallaron${NC}"
    echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "\n${YELLOW}📖 Revisa SECURITY_GUIDE.md para solucionar los problemas${NC}\n"
    exit 1
fi
