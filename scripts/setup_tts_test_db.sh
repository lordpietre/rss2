#!/usr/bin/env bash
# =============================================================================
# Crea la base de datos de pruebas del subsistema TTS.
#
#   make test-tts-db
#
# La base es independiente de la de produccion: `rss_tts_test`. Contiene las
# tablas minimas (noticias, traducciones, users) para que las claves
# foraneas del subsistema TTS se puedan probar, y el esquema TTS completo.
#
# Las pruebas de integracion necesitan la URL de conexion:
#
#   export TEST_DATABASE_URL="postgres://rss:<password>@<host>:5432/rss_tts_test"
#   make test-tts
# =============================================================================

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

DB_NAME="${TTS_TEST_DB:-rss_tts_test}"
DB_USER="${POSTGRES_USER:-rss}"

log()  { printf '\033[1;34m[TTS]\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m[TTS]\033[0m %s\n' "$*" >&2; exit 1; }

# Credenciales: del .env si existe.
if [[ -f .env ]]; then
    set -a
    # shellcheck disable=SC1091
    source .env
    set +a
fi

command -v docker >/dev/null 2>&1 || die "Se necesita docker."

# -----------------------------------------------------------------------------
# Base de datos
# -----------------------------------------------------------------------------
if docker compose exec -T db psql -U "${DB_USER}" -d postgres -tAc \
        "SELECT 1 FROM pg_database WHERE datname='${DB_NAME}'" | grep -q 1; then
    log "La base ${DB_NAME} ya existe."
else
    log "Creando la base ${DB_NAME}..."
    docker compose exec -T db psql -U "${DB_USER}" -d postgres \
        -c "CREATE DATABASE ${DB_NAME}" >/dev/null
fi

# -----------------------------------------------------------------------------
# Tablas minimas (referenciadas por el esquema TTS)
# -----------------------------------------------------------------------------
log "Creando las tablas minimas en ${DB_NAME}..."
docker compose exec -T db psql -U "${DB_USER}" -d "${DB_NAME}" <<'SQL'
CREATE TABLE IF NOT EXISTS users (
    id    SERIAL PRIMARY KEY,
    email VARCHAR(255)
);
CREATE TABLE IF NOT EXISTS noticias (
    id  VARCHAR(32) PRIMARY KEY,
    url TEXT
);
CREATE TABLE IF NOT EXISTS traducciones (
    id             SERIAL PRIMARY KEY,
    noticia_id     VARCHAR(32) REFERENCES noticias(id) ON DELETE CASCADE,
    lang_from      CHAR(5),
    lang_to        CHAR(5),
    titulo_trad    TEXT,
    resumen_trad   TEXT,
    contenido_trad TEXT,
    status         VARCHAR(16) DEFAULT 'done'
);
SQL

# -----------------------------------------------------------------------------
# Esquema TTS
# -----------------------------------------------------------------------------
log "Aplicando el esquema TTS..."
docker compose exec -T db psql -U "${DB_USER}" -d "${DB_NAME}" < migrations/add_tts.sql >/dev/null

log "Listo. Para ejecutar las pruebas de integracion:"
printf '  export TEST_DATABASE_URL="postgres://%s:<password>@<host-de-postgres>:5432/%s"\n' \
    "${DB_USER}" "${DB_NAME}"
printf '  make test-tts\n'
