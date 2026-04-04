#!/bin/bash

# ==================================================================================
# Script de Generación de Credenciales Seguras
# ==================================================================================
# 
# Este script genera credenciales aleatorias seguras para todos los servicios
# y crea un archivo .env con las configuraciones necesarias.
#
# Uso:
#   ./generate_secure_credentials.sh
#
# El script creará:
#   - .env.generated (con las credenciales nuevas)
#   - .env.backup (backup de .env actual si existe)
#
# ==================================================================================

set -e  # Exit on error

# Argumentos para modo automático
FORCE="false"

# Verificar argumentos
while [[ $# -gt 0 ]]; do
    case $1 in
        --force)
            FORCE="true"
            shift
            ;;
        -h|--help)
            echo "Generador de Credenciales Seguras"
            echo ""
            echo "Uso: ./generate_secure_credentials.sh [--force]"
            echo ""
            echo "Opciones:"
            echo "  --force   Forzar reemplazo de .env sin preguntar"
            echo "  --help    Mostrar esta ayuda"
            exit 0
            ;;
        *)
            echo "Opción desconocida: $1"
            exit 1
            ;;
    esac
done

# Colores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=================================="
echo -e "🔒 Generador de Credenciales Seguras"
echo -e "==================================${NC}\n"

# Verificar dependencias
command -v openssl >/dev/null 2>&1 || { echo -e "${RED}❌ Error: openssl no está instalado${NC}"; exit 1; }
command -v python3 >/dev/null 2>&1 || { echo -e "${RED}❌ Error: python3 no está instalado${NC}"; exit 1; }

# Backup del .env actual si existe
if [ -f .env ]; then
    echo -e "${YELLOW}⚠️  Encontrado archivo .env existente${NC}"
    BACKUP_FILE=".env.backup.$(date +%Y%m%d_%H%M%S)"
    cp .env "$BACKUP_FILE"
    echo -e "${GREEN}✅ Backup creado: $BACKUP_FILE${NC}\n"
fi

echo -e "${GREEN}🔑 Generando credenciales seguras...${NC}\n"

# Generar credenciales
POSTGRES_PASSWORD=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-32)
REDIS_PASSWORD=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-32)
SECRET_KEY=$(python3 -c "import secrets; print(secrets.token_hex(32))")
GRAFANA_PASSWORD=$(openssl rand -base64 24 | tr -d "=+/" | cut -c1-24)

# Mostrar credenciales generadas (para que el usuario las guarde)
echo -e "${YELLOW}⚠️  IMPORTANTE: Guarda estas credenciales en un lugar seguro${NC}\n"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${GREEN}POSTGRES_PASSWORD:${NC} $POSTGRES_PASSWORD"
echo -e "${GREEN}REDIS_PASSWORD:${NC}    $REDIS_PASSWORD"
echo -e "${GREEN}SECRET_KEY:${NC}         $SECRET_KEY"
echo -e "${GREEN}GRAFANA_PASSWORD:${NC}  $GRAFANA_PASSWORD"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Crear archivo .env.generated
ENV_FILE=".env.generated"
cat > "$ENV_FILE" << EOF
# ==================================================================================
# CONFIGURACIÓN SEGURA - Generado automáticamente
# Fecha: $(date +"%Y-%m-%d %H:%M:%S")
# ==================================================================================
# 
# IMPORTANTE: 
# - NO compartas este archivo
# - Guarda las credenciales en un gestor de contraseñas
# - Añade .env al .gitignore
#
# ==================================================================================

# ==================================================================================
# DATABASE CONFIGURATION - PostgreSQL
# ==================================================================================
POSTGRES_DB=rss
POSTGRES_USER=rss
POSTGRES_PASSWORD=$POSTGRES_PASSWORD

DB_NAME=rss
DB_USER=rss
DB_PASS=$POSTGRES_PASSWORD
DB_HOST=db
DB_PORT=5432
DB_WRITE_HOST=db
DB_READ_HOST=db-replica

# ==================================================================================
# REDIS CONFIGURATION - Con autenticación
# ==================================================================================
REDIS_HOST=redis
REDIS_PORT=6379
REDIS_PASSWORD=$REDIS_PASSWORD

# ==================================================================================
# APPLICATION SECRETS
# ==================================================================================
SECRET_KEY=$SECRET_KEY

# ==================================================================================
# MONITORING - Grafana
# ==================================================================================
GRAFANA_PASSWORD=$GRAFANA_PASSWORD

# ==================================================================================
# EXTERNAL SERVICES
# ==================================================================================
ALLTALK_URL=http://host.docker.internal:7851

# ==================================================================================
# AI MODELS & WORKERS
# ==================================================================================
RSS_MAX_WORKERS=3
TARGET_LANGS=es
TRANSLATOR_BATCH=128
ENQUEUE=300

# RSS Ingestor Configuration
RSS_POKE_INTERVAL_MIN=15
RSS_MAX_FAILURES=10
RSS_FEED_TIMEOUT=60

# URL Feed Discovery Worker
URL_DISCOVERY_INTERVAL_MIN=15
URL_DISCOVERY_BATCH_SIZE=10
MAX_FEEDS_PER_URL=5

# CTranslate2 / AI Model Paths
CT2_MODEL_PATH=/app/models/nllb-ct2
CT2_DEVICE=cuda
CT2_COMPUTE_TYPE=int8_float16
UNIVERSAL_MODEL=facebook/nllb-200-distilled-600M

# Embeddings
EMB_MODEL=sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2
EMB_BATCH=64
EMB_DEVICE=cuda

# NER
NER_LANG=es
NER_BATCH=64

# Flask / Gunicorn
GUNICORN_WORKERS=8
FLASK_DEBUG=0

# Qdrant Configuration
QDRANT_HOST=qdrant
QDRANT_PORT=6333
QDRANT_COLLECTION_NAME=news_vectors
QDRANT_BATCH_SIZE=100
QDRANT_SLEEP_IDLE=30
EOF

echo -e "${GREEN}✅ Archivo generado: $ENV_FILE${NC}\n"

# Preguntar si quiere reemplazar .env (si no se usa --force)
if [ "$FORCE" = "true" ]; then
    mv "$ENV_FILE" .env
    echo -e "${GREEN}✅ Archivo .env actualizado automáticamente${NC}"
else
    echo -e "${YELLOW}¿Deseas reemplazar el archivo .env actual con el generado?${NC}"
    echo -e "${YELLOW}(Recomendado: revisa $ENV_FILE primero)${NC}"
    read -p "¿Continuar? (s/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[SsYy]$ ]]; then
        mv "$ENV_FILE" .env
        echo -e "${GREEN}✅ Archivo .env actualizado${NC}"
    else
        echo -e "${YELLOW}⚠️  Archivo guardado como: $ENV_FILE${NC}"
        echo -e "${YELLOW}   Para usarlo: mv $ENV_FILE .env${NC}"
    fi
fi

echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}✅ ¡Credenciales generadas exitosamente!${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${YELLOW}📋 PRÓXIMOS PASOS:${NC}"
echo ""
echo -e "  1. Revisa las credenciales generadas arriba"
echo -e "  2. Guárdalas en un gestor de contraseñas seguro"
echo -e "  3. Migra a docker-compose.secure.yml:"
echo -e "     ${GREEN}cp docker-compose.secure.yml docker-compose.yml${NC}"
echo -e "  4. Haz backup de tus datos (ver SECURITY_GUIDE.md)"
echo -e "  5. Reinicia los servicios:"
echo -e "     ${GREEN}docker-compose down && docker-compose up -d${NC}"
echo -e "  6. Verifica que todo funciona correctamente"
echo ""
echo -e "${YELLOW}📖 Para más detalles, revisa: SECURITY_GUIDE.md${NC}"
echo ""
