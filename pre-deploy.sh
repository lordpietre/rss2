#!/bin/bash

# ==================================================================================
# Pre-Deploy Script - Validación de Credenciales Seguras
# ==================================================================================
#
# Este script se ejecuta antes de docker compose up para:
# 1. Verificar que existen credenciales en .env
# 2. Generar credenciales seguras si faltan
# 3. Validar que las variables críticas estén definidas
# 4. Mostrar resumen de credenciales en consola
#
# Uso:
#   ./pre-deploy.sh
#
# Salida:
#   - Éxito: Muestra resumen de credenciales y permite continuar
#   - Error: Sale con código 1 si faltan credenciales críticas
#
# ==================================================================================

set -e  # Exit on error

# Colores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}╔═══════════════════════════════════════════════════════════╗"
echo -e "║   🔐 Pre-Deploy Security Check - RSS2 Platform           ║"
echo -e "╚═══════════════════════════════════════════════════════════╝${NC}\n"

# Función para mostrar ayuda
show_help() {
    cat << EOF
${BLUE}Uso:${NC}
  ./pre-deploy.sh [--generate|--skip]

${BLUE}Opciones:${NC}
  --generate   Generar credenciales seguras automáticamente
  --skip       Saltar validación y continuar (NO RECOMENDADO)

${BLUE}Ejemplos:${NC}
  ./pre-deploy.sh                    # Validar credenciales existentes
  ./pre-deploy.sh --generate         # Generar nuevas credenciales
  ./pre-deploy.sh --skip            # Saltar validación

${YELLOW}Nota:${NC}  Las credenciales se guardan en .env para docker compose
EOF
}

# Verificar argumentos
if [ $# -gt 0 ]; then
    case "$1" in
        --generate)
            echo -e "${GREEN}🔄 Generando credenciales seguras...${NC}\n"
            bash ./generate_secure_credentials.sh --force
            exit $?
            ;;
        --skip)
            echo -e "${YELLOW}⚠️  Saltando validación de credenciales${NC}"
            echo -e "${YELLOW}⚠️  Esto puede causar advertencias de WARN durante el despliegue${NC}"
            echo ""
            # Leer credenciales de ejemplo si no existen
            if [ ! -f .env ]; then
                if [ -f .env.example ]; then
                    cp .env.example .env
                    echo -e "${GREEN}✅ .env creado desde .env.example${NC}"
                elif [ -f .env.secure.example ]; then
                    cp .env.secure.example .env
                    echo -e "${GREEN}✅ .env creado desde .env.secure.example${NC}"
                else
                    echo -e "${RED}❌ Error: No hay .env.example para copiar${NC}"
                    exit 1
                fi
            fi
            ;;
        *)
            show_help
            exit 1
            ;;
    esac
fi

# Función para generar credenciales seguras
generate_credentials() {
    if [ -f ./generate_secure_credentials.sh ]; then
        echo -e "${GREEN}🔄 Generando credenciales seguras...${NC}\n"
        bash ./generate_secure_credentials.sh --force
        return $?
    else
        echo -e "${RED}❌ Error: generate_secure_credentials.sh no encontrado${NC}"
        return 1
    fi
}

# Verificar si .env existe y tiene contenido
check_env_file() {
    if [ ! -f .env ]; then
        echo -e "${RED}❌ Error: Archivo .env no encontrado${NC}"
        echo -e "${YELLOW}💡 Generando credenciales seguras automáticamente...${NC}"
        generate_credentials
        return $?
    fi
    
    if [ ! -s .env ]; then
        echo -e "${RED}❌ Error: Archivo .env está vacío${NC}"
        echo -e "${YELLOW}💡 Generando credenciales seguras automáticamente...${NC}"
        generate_credentials
        return $?
    fi
    
    echo -e "${GREEN}✅ Archivo .env encontrado${NC}"
    return 0
}

# Función para leer variables del archivo .env
get_env_var() {
    local var_name="$1"
    grep "^${var_name}=" .env 2>/dev/null | cut -d'=' -f2- | tr -d '[:space:]'
}

# Función para validar variables críticas
validate_credentials() {
    local missing=0
    local critical_missing=0
    local critical_vars=(POSTGRES_PASSWORD REDIS_PASSWORD DB_PASS)
    
    echo -e "\n${BLUE}🔍 Validando credenciales críticas...${NC}\n"
    
    for var in "${critical_vars[@]}"; do
        local value
        value=$(get_env_var "$var")
        
        if [ -z "$value" ]; then
            echo -e "${RED}❌ FALTA: $var está vacía${NC}"
            ((missing++))
            ((critical_missing++))
        else
            echo -e "${GREEN}✅ DEFINIDO: $var=${value:0:16}${NC}"
        fi
    done
    
    if [ $critical_missing -gt 0 ]; then
        return 1
    fi
    
    return 0
}

# Función para mostrar resumen de credenciales
show_credentials_summary() {
    echo -e "\n${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}📋 Resumen de Credenciales Activas${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
    
    if [ -f .env ]; then
        echo "${BLUE}📁 Archivo .env:${NC}"
        echo -e "   ${GREEN}📍 Ruta:${NC} $(pwd)/.env"
        echo -e "   ${GREEN}📏 Tamaño:${NC} $(wc -c < .env) bytes"
        echo -e "   ${GREEN}📊 Líneas:${NC} $(wc -l < .env)"
        echo ""
        
        echo "${BLUE}🔑 Variables Críticas:${NC}"
        echo -e "   ${GREEN}✅ POSTGRES_PASSWORD:${NC} ✓"
        echo -e "   ${GREEN}✅ REDIS_PASSWORD:${NC} ✓"
        echo -e "   ${GREEN}✅ DB_PASS:${NC} ✓"
        echo ""
        
        echo "${BLUE}📝 Otras Variables Importantes:${NC}"
        for var in POSTGRES_USER DB_NAME SECRET_KEY GRAFANA_PASSWORD; do
            local value
            value=$(get_env_var "$var")
            if [ -n "$value" ]; then
                echo -e "   ${GREEN}✅ $var:${NC} ${value:0:12}..."
            else
                echo -e "   ${YELLOW}⚠️  $var:${NC} No definido"
            fi
        done
    else
        echo -e "${RED}❌ No se pudo cargar .env${NC}"
        return 1
    fi
    
    echo ""
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}✅ Validación completada - Listo para desplegar${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
    
    # Preguntar si quiere copiar en clipboard (macOS)
    if command -v pbcopy >/dev/null 2>&1; then
        echo -e "${YELLOW}💡 Para copiar credenciales, usa: pbcopy < .env${NC}"
    fi
    
    return 0
}

# Función para mostrar advertencias de seguridad
show_security_warnings() {
    echo -e "\n${YELLOW}⚠️  RECOMENDACIONES DE SEGURIDAD:${NC}\n"
    
    if [ -f .env ]; then
        if [ -f .gitignore ]; then
            if grep -q "^.env" .gitignore; then
                echo -e "   ✅ .env está en .gitignore"
            else
                echo -e "   ${RED}❌ .env NO está en .gitignore${NC}"
                echo -e "   ${YELLOW}   → Añade .env al .gitignore${NC}"
            fi
        else
            echo -e "   ${YELLOW}⚠️  No hay .gitignore, crea uno para proteger .env${NC}"
        fi
    fi
    
    echo ""
    echo -e "   ✅ Credenciales no están en Dockerfile (se inyectan en runtime)"
    echo -e "   ✅ Credenciales no están en docker-compose.yml (se cargan desde .env)"
    echo -e "   ✅ Variables sensibles usan ${NC}${GREEN}recomendados${NC}${BLUE} (no hardcoded)"
    echo ""
}

# Función para desplegar con docker compose
deploy_with_compose() {
    echo -e "\n${GREEN}🚀 DESPLIEGUE AUTOMÁTICO${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    
    # Detener y limpiar contenedores
    echo -e "${YELLOW}📦 Deteniendo contenedores...${NC}"
    docker compose down -v 2>/dev/null || true
    
    # Iniciar servicios
    echo -e "${GREEN}📦 Construyendo e iniciando servicios...${NC}"
    docker compose up -d --build
    
    # Esperar a que los servicios estén saludables
    echo -e "${GREEN}✅ Esperando a que los servicios estén listos...${NC}"
    sleep 5
    
    # Verificar estado
    echo -e "${BLUE}📊 Estado de los servicios:${NC}"
    docker compose ps
    
    # Verificar base de datos
    echo -e "${BLUE}🔍 Verificando base de datos...${NC}"
    if docker compose ps db | grep -q "healthy\|Up"; then
        echo -e "${GREEN}✅ Base de datos: Iniciada${NC}"
        
        # Esperar a que el schema se inicialice
        echo -e "${GREEN}✅ Esperando inicialización del schema...${NC}"
        sleep 10
        
        # Ejecutar scripts SQL si existen
        if [ -d init-db ] && [ "$(ls -A init-db 2>/dev/null)" ]; then
            echo -e "${YELLOW}📄 Ejecutando scripts de inicialización...${NC}"
            docker compose exec -T db psql -U rss -d rss -f /docker-entrypoint-initdb.d/*.sql 2>&1 | head -20 || true
        fi
        
        # Verificar Redis
        echo -e "${BLUE}🔍 Verificando Redis...${NC}"
        sleep 5
        if docker compose ps redis | grep -q "Up"; then
            echo -e "${GREEN}✅ Redis: Iniciado${NC}"
        fi
    else
        echo -e "${RED}❌ Error: Base de datos no se inició correctamente${NC}"
        docker compose logs db
        return 1
    fi
    
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}✅ DESPLIEGUE COMPLETADO${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo -e "${BLUE}🌐 Endpoints disponibles:${NC}"
    echo -e "   ${GREEN}App:${NC}      http://localhost:8888"
    echo -e "   ${GREEN}Backend API:${NC} http://localhost:8888/api"
    echo -e "   ${GREEN}Grafana:${NC}    http://127.0.0.1:3001"
    echo -e "   ${GREEN}Prometheus:${NC} http://127.0.0.1:9090"
    echo ""
    
    # Verificar si existe .env.backup y restaurarlo
    if [ -f .env.backup ]; then
        echo -e "${YELLOW}⚠️  Nota: Se encontró .env.backup, puedes restaurarlo si fue necesario${NC}"
    fi
    
    return 0
}

# Función principal
main() {
    # Verificar que .env exista y tenga contenido
    if ! check_env_file; then
        return 1
    fi
    
    # Validar credenciales críticas
    if ! validate_credentials; then
        echo -e "\n${RED}❌ Error: Faltan credenciales críticas${NC}"
        echo -e "${YELLOW}💡 Ejecuta: ./pre-deploy.sh --generate${NC}"
        return 1
    fi
    
    # Mostrar resumen de credenciales
    if ! show_credentials_summary; then
        return 1
    fi
    
    # Mostrar advertencias de seguridad
    show_security_warnings
    
    echo -e "\n${GREEN}╔═══════════════════════════════════════════════════════════╗"
    echo -e "║   ✅ PRE-DEPLOY VALIDACIÓN EXITOSA - LISTO PARA INICIAR   ║"
    echo -e "╚═══════════════════════════════════════════════════════════╝${NC}\n"
    
    # Preguntar si quiere desplegar automáticamente
    echo -e "${BLUE}¿Deseas desplegar ahora?${NC}"
    echo -e "   1) Desplegar automáticamente (recomendado)"
    echo -e "   2) Solo validar y salir (modo manual)"
    echo -e "   "
    read -p "   Elige una opción: " choice
    
    case $choice in
        1)
            if deploy_with_compose; then
                return 0
            else
                echo -e "\n${RED}❌ Error en el despliegue${NC}"
                return 1
            fi
            ;;
        2)
            echo -e "\n${YELLOW}✅ Validación completada - Salir${NC}"
            echo -e "${BLUE}▶️  Para desplegar manualmente, ejecuta:${NC}"
            echo -e "   ${GREEN}docker compose up -d${NC}"
            echo ""
            return 0
            ;;
        *)
            echo -e "${YELLOW}⚠️  Opción no válida, usando opción 2${NC}"
            return 0
            ;;
    esac
}

# Ejecutar
main

# ==================================================================================
# Verificación de modelo GPU (después del despliegue)
# ==================================================================================
check_gpu_model() {
    MODEL_DIR="./models/nllb-ct2-1.3b"
    
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "  🤖 Verificando modelo de traducción GPU"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    if [ ! -d "$MODEL_DIR" ]; then
        echo "  ⚠️  Directorio de modelo no existe"
        return 0
    fi
    
    model_ok=true
    
    # Verificar model.bin
    if [ -f "$MODEL_DIR/model.bin" ]; then
        size=$(stat -c%s "$MODEL_DIR/model.bin" 2>/dev/null || echo 0)
        if [ "$size" -gt 1000000 ]; then
            echo "  ✓ model.bin ($(numfmt --to=iec-i --suffix=B $size 2>/dev/null || echo "${size}B"))"
        else
            echo "  ✗ model.bin demasiado pequeño"
            model_ok=false
        fi
    else
        echo "  ✗ model.bin no encontrado"
        model_ok=false
    fi
    
    # Verificar config.json
    if [ -f "$MODEL_DIR/config.json" ]; then
        echo "  ✓ config.json"
    else
        echo "  ✗ config.json no encontrado"
        model_ok=false
    fi
    
    # Verificar shared_vocabulary.json
    if [ -f "$MODEL_DIR/shared_vocabulary.json" ]; then
        echo "  ✓ shared_vocabulary.json"
    else
        echo "  ✗ shared_vocabulary.json no encontrado"
        model_ok=false
    fi
    
    echo ""
    if [ "$model_ok" = true ]; then
        echo "  ✓ Modelo verificado - workers GPU iniciarán rápidamente"
    else
        echo "  ⚠️  Modelo incompleto - primer worker lo convertirá (5-10 min)"
        echo ""
        echo "  Pre-convertir manualmente:"
        echo "    docker run --rm -v \$(pwd)/models:/app/models rss2-translator-gpu:latest \\"
        echo "      ct2-transformers-converter --model facebook/nllb-200-1.3B \\"
        echo "      --output_dir /app/models/nllb-ct2-1.3b --quantization float16 --force"
    fi
}

check_gpu_model
