#!/bin/bash
# Hook para docker compose - Se ejecuta automáticamente
# Copia este archivo al inicio de tu .bashrc o .bash_profile

# Función de validación automática
validate_credentials() {
    # Verificar .env
    if [ ! -f .env ]; then
        echo "🔑 Generando credenciales..."
        ./generate_secure_credentials.sh --force
        return $?
    fi
    
    # Verificar contraseñas vacías
    for var in POSTGRES_PASSWORD REDIS_PASSWORD DB_PASS; do
        val=$(grep "^${var}=" .env | cut -d'=' -f2 | tr -d ' ')
        if [ -z "$val" ] || [ "$val" = "change_*" ]; then
            echo "🔑 Generando credenciales para $var..."
            ./generate_secure_credentials.sh --force
            return $?
        fi
    done
}

# Alias para docker compose con validación automática
alias docker="docker"
alias "docker compose"="docker compose --env-file .env"

# Función helper: docker compose up con validación
docker_compose_up() {
    echo "🔐 Validando credenciales..."
    validate_credentials
    
    echo "🚀 Iniciando servicios..."
    docker compose up -d
}

# Exportar alias
alias docker_compose_up="docker_compose_up"
