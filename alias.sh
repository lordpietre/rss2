#!/bin/bash
# Aliases y funciones para despliegue automático de RSS2
# Copia este archivo y ejecútalo: source alias.sh

# ========================================
# FUNCIONES DE VALIDACIÓN AUTOMÁTICA
# ========================================

# Validar credenciales críticas
validate_creds() {
    # Verificar si .env existe
    if [ ! -f .env ]; then
        echo "🔑 Generando credenciales seguras..."
        ./generate_secure_credentials.sh --force
        exit $?
    fi
    
    # Verificar contraseñas vacías
    for var in POSTGRES_PASSWORD REDIS_PASSWORD DB_PASS; do
        val=$(grep "^${var}=" .env 2>/dev/null | cut -d'=' -f2 | tr -d ' ')
        if [ -z "$val" ] || [ "$val" = "change_*" ] || [ "$val" = "" ]; then
            echo "🔑 $var está vacía, generando..."
            ./generate_secure_credentials.sh --force
            exit $?
        fi
    done
    
    echo "✅ Credenciales OK"
}

# ========================================
# ALIASES PRINCIPALES
# ========================================

# Alias principal: docker compose con validación automática
alias dc="docker compose"
alias dcu="docker_compose_up"  # docker compose up
alias dcd="docker_compose_down"  # docker compose down

# Alias con validación automática
docker_compose_up() {
    echo "🔐 Validando credenciales..."
    validate_creds
    
    echo "🚀 docker compose up -d"
    dc up -d
}

docker_compose_down() {
    echo "📦 docker compose down -v"
    dc down -v
}

# Alias para despliegue completo con validación
deploy() {
    echo "================================"
    echo "🚀 DESPLIEGUE AUTOMÁTICO RSS2"
    echo "================================"
    echo ""
    
    echo "1. 🔐 Validando credenciales..."
    validate_creds
    
    echo "2. 📦 Deteniendo y eliminando..."
    dc down -v 2>/dev/null || true
    
    echo "3. 🚀 Iniciando servicios..."
    dc up -d --build
    
    echo "4. ✅ Verificando estado..."
    dc ps
    
    echo ""
    echo "================================"
    echo "✅ ¡Despliegue completado!"
    echo "🌐 App: http://localhost:8888"
    echo "================================"
}

# Alias para despliegue rápido sin rebuild
deploy-quick() {
    echo "🚀 Despliegue rápido (sin rebuild)..."
    dc up -d
    
    echo "📊 Estado:"
    dc ps
}

# Alias para ver logs
tail-logs() {
    echo "📊 Logs en tiempo real..."
    dc logs -f
}

# ========================================
# FUNCIONES DE UTILIDAD
# ========================================

# Generar credenciales
generate-secrets() {
    echo "🔑 Generando credenciales seguras..."
    ./generate_secure_credentials.sh --force
}

# Backup
backup() {
    echo "📦 Creando backup..."
    dc exec db pg_dump -U rss rss > backup_$(date +%Y%m%d).sql
    echo "✅ Backup guardado en ./backup_$(date +%Y%m%d).sql"
}

# ========================================
# MENSAJE DE BIENVENIDA
# ========================================
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    echo "🎉 RSS2 Deployment Aliases cargadas"
    echo ""
    echo "Comandos disponibles:"
    echo "  dc              docker compose"
    echo "  dcu             docker compose up (con validación)"
    echo "  dcd             docker compose down"
    echo "  deploy          Despliegue completo con validación"
    echo "  deploy-quick    Despliegue rápido"
    echo "  tail-logs       Ver logs en tiempo real"
    echo "  generate-secrets Generar credenciales"
    echo "  backup          Crear backup"
    echo ""
fi
