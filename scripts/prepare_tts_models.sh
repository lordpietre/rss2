#!/usr/bin/env bash
# =============================================================================
# Prepara los modelos del servicio de TTS.
#
# El servicio NO descarga nada al arrancar: los pesos se dejan listos aqui y
# se montan en los volumenes ./models/openvoice y ./models/chatterbox.
#
#   1. Checkpoints de OpenVoice V2 (conversion de timbre).
#   2. Audios de referencia de las voces clonadas preinstaladas + manifiesto.
#   3. Embeddings de timbre de las voces base de OpenVoice.
#   4. Modelo de MeloTTS en espanol (cache de HuggingFace).
#   5. Pesos de Chatterbox Multilingual (motor por defecto).
#
# Uso:
#   ./scripts/prepare_tts_models.sh            # prepara y verifica
#   ./scripts/prepare_tts_models.sh --verify   # solo verifica
#
# Requiere: docker (para la cache de HuggingFace) y curl. Deja ~3,2 GB de
# Chatterbox ademas de los 130 MB de OpenVoice.
# =============================================================================

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODEL_DIR="${ROOT_DIR}/models/openvoice"
CHECKPOINT_DIR="${MODEL_DIR}/checkpoints_v2"
VOICES_DIR="${MODEL_DIR}/voices"
BASE_SPEAKERS_DIR="${MODEL_DIR}/base_speakers"
CHATTERBOX_DIR="${ROOT_DIR}/models/chatterbox"
HF_CACHE_DIR="${ROOT_DIR}/hf_cache"

# El bucket S3 de MyShell (myshell-public-repo-host) ya no existe, asi que los
# checkpoints se toman de HuggingFace (myshell-ai/OpenVoiceV2, licencia MIT).
HF_REPO="https://huggingface.co/myshell-ai/OpenVoiceV2/resolve/main"
# Pesos de Chatterbox (Resemble AI, licencia MIT).
CHATTERBOX_REPO="https://huggingface.co/ResembleAI/chatterbox/resolve/main"
# Audios de referencia de las voces demo (repositorio de OpenVoice).
OPENVOICE_RAW="https://raw.githubusercontent.com/myshell-ai/OpenVoice/main/resources"

# Voces con embedding de timbre precalculado (no necesitan audio de referencia).
BASE_SPEAKERS=(es en-us en-au en-br en-india en-newest fr jp kr zh)

VERIFY_ONLY=0
if [[ "${1:-}" == "--verify" ]]; then
    VERIFY_ONLY=1
fi

log()  { printf '\033[1;34m[TTS]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[TTS]\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31m[TTS]\033[0m %s\n' "$*" >&2; exit 1; }

# Alias del peso T3 de Chatterbox -> nombre real del fichero.
# Debe coincidir con workers/tts/engine.py (CHATTERBOX_T3_FILES).
CHATTERBOX_T3_ALIAS="${TTS_CHATTERBOX_T3_MODEL:-v3}"
case "${CHATTERBOX_T3_ALIAS}" in
    v2|t3_mtl23ls_v2) CHATTERBOX_T3_FILE="t3_mtl23ls_v2.safetensors" ;;
    v3|t3_mtl23ls_v3) CHATTERBOX_T3_FILE="t3_mtl23ls_v3.safetensors" ;;
    *.safetensors)    CHATTERBOX_T3_FILE="${CHATTERBOX_T3_ALIAS}" ;;
    *) die "TTS_CHATTERBOX_T3_MODEL desconocido: '${CHATTERBOX_T3_ALIAS}' (usa v2, v3 o un .safetensors)" ;;
esac

download() {
    local url="$1" target="$2"
    if [[ -s "${target}" ]]; then
        log "Ya existe: $(basename "${target}")"
        return 0
    fi
    log "Descargando $(basename "${target}")..."
    curl --fail --location --silent --show-error --retry 3 --retry-delay 2 \
        --output "${target}.part" "${url}"
    mv "${target}.part" "${target}"
}

verify() {
    local missing=0
    for file in "${CHECKPOINT_DIR}/config.json" "${CHECKPOINT_DIR}/checkpoint.pth"; do
        if [[ ! -s "${file}" ]]; then
            warn "Falta: ${file}"
            missing=1
        fi
    done
    if [[ ! -s "${VOICES_DIR}/manifest.json" ]]; then
        warn "Falta: ${VOICES_DIR}/manifest.json"
        missing=1
    fi
    # Motor por defecto: sin estos ficheros el servicio arranca pero cualquier
    # narracion falla con engine_not_prepared.
    for file in "ve.pt" "s3gen.pt" "conds.pt" "grapheme_mtl_merged_expanded_v1.json" "${CHATTERBOX_T3_FILE}"; do
        if [[ -z "${file}" || ! -s "${CHATTERBOX_DIR}/${file}" ]]; then
            warn "Falta: ${CHATTERBOX_DIR}/${file}"
            missing=1
        fi
    done
    if [[ ${missing} -eq 0 ]]; then
        log "Modelos verificados."
    else
        return 1
    fi
}

if [[ ${VERIFY_ONLY} -eq 1 ]]; then
    verify || die "Los modelos no estan completos. Ejecuta el script sin --verify."
    exit 0
fi

command -v curl >/dev/null 2>&1 || die "Se necesita curl."
command -v docker >/dev/null 2>&1 || warn "Docker no disponible: la cache de HuggingFace no se podra preparar."

mkdir -p "${MODEL_DIR}" "${CHECKPOINT_DIR}" "${VOICES_DIR}" "${BASE_SPEAKERS_DIR}" "${CHATTERBOX_DIR}" "${HF_CACHE_DIR}"

# -----------------------------------------------------------------------------
# 1. Checkpoints de OpenVoice V2 (converter de timbre)
# -----------------------------------------------------------------------------
if [[ ! -s "${CHECKPOINT_DIR}/config.json" || ! -s "${CHECKPOINT_DIR}/checkpoint.pth" ]]; then
    log "Descargando el converter de OpenVoice V2 (131 MB)..."
    download "${HF_REPO}/converter/config.json" "${CHECKPOINT_DIR}/config.json"
    download "${HF_REPO}/converter/checkpoint.pth" "${CHECKPOINT_DIR}/checkpoint.pth"
else
    log "Checkpoints de OpenVoice V2 ya presentes."
fi

# -----------------------------------------------------------------------------
# 2. Voces clonadas preinstaladas
# -----------------------------------------------------------------------------
download "${OPENVOICE_RAW}/example_reference.mp3" "${VOICES_DIR}/example_reference.mp3"
download "${OPENVOICE_RAW}/demo_speaker0.mp3" "${VOICES_DIR}/demo_speaker0.mp3"
download "${OPENVOICE_RAW}/demo_speaker1.mp3" "${VOICES_DIR}/demo_speaker1.mp3"
download "${OPENVOICE_RAW}/demo_speaker2.mp3" "${VOICES_DIR}/demo_speaker2.mp3"

# -----------------------------------------------------------------------------
# 3. Embeddings de timbre precalculados (voces base de OpenVoice V2)
#
# Son 1,7 KB por voz: permiten clonar el timbre sin audio de referencia.
# -----------------------------------------------------------------------------
for speaker in "${BASE_SPEAKERS[@]}"; do
    download "${HF_REPO}/base_speakers/ses/${speaker}.pth" "${BASE_SPEAKERS_DIR}/${speaker}.pth"
done

cat > "${BASE_SPEAKERS_DIR}/manifest.json" <<'JSON'
{
  "voices": [
    { "slug": "ov-es",       "display_name": "OpenVoice · español",      "file": "es.pth",       "language": "es" },
    { "slug": "ov-en-us",    "display_name": "OpenVoice · inglés (EE.UU.)", "file": "en-us.pth",    "language": "en" },
    { "slug": "ov-en-au",    "display_name": "OpenVoice · inglés (AU)",    "file": "en-au.pth",    "language": "en" },
    { "slug": "ov-en-br",    "display_name": "OpenVoice · inglés (UK)",    "file": "en-br.pth",    "language": "en" },
    { "slug": "ov-en-india", "display_name": "OpenVoice · inglés (IN)",    "file": "en-india.pth", "language": "en" },
    { "slug": "ov-en-newest","display_name": "OpenVoice · inglés (nuevo)","file": "en-newest.pth","language": "en" },
    { "slug": "ov-fr",       "display_name": "OpenVoice · francés",       "file": "fr.pth",       "language": "fr" },
    { "slug": "ov-jp",       "display_name": "OpenVoice · japonés",       "file": "jp.pth",       "language": "ja" },
    { "slug": "ov-kr",       "display_name": "OpenVoice · coreano",       "file": "kr.pth",       "language": "ko" },
    { "slug": "ov-zh",       "display_name": "OpenVoice · chino",         "file": "zh.pth",       "language": "zh" }
  ]
}
JSON

cat > "${VOICES_DIR}/manifest.json" <<'JSON'
{
  "voices": [
    {
      "slug": "demo-speaker0",
      "display_name": "Narrador demo 0",
      "file": "demo_speaker0.mp3",
      "engine": "openvoice"
    },
    {
      "slug": "demo-speaker1",
      "display_name": "Narrador demo 1",
      "file": "demo_speaker1.mp3",
      "engine": "openvoice"
    },
    {
      "slug": "demo-speaker2",
      "display_name": "Narrador demo 2",
      "file": "demo_speaker2.mp3",
      "engine": "openvoice"
    },
    {
      "slug": "referencia-ejemplo",
      "display_name": "Referencia de ejemplo",
      "file": "example_reference.mp3",
      "engine": "openvoice"
    }
  ]
}
JSON
log "Manifiesto de voces escrito en ${VOICES_DIR}/manifest.json"

# -----------------------------------------------------------------------------
# 4. Modelo de MeloTTS (cache de HuggingFace)
# -----------------------------------------------------------------------------
if command -v docker >/dev/null 2>&1; then
    log "Descargando el modelo de MeloTTS (ES) a la cache de HuggingFace..."
    docker run --rm \
        -v "${HF_CACHE_DIR}:/cache" \
        -e HF_HOME=/cache \
        --entrypoint python \
        rss2-tts:latest \
        -c "from melo.api import TTS; TTS(language='ES', device='cpu'); print('modelo listo')" \
        || warn "No se pudo pre-descargar el modelo de MeloTTS. El servicio lo bajara en el primer uso."
else
    warn "Sin Docker: el modelo de MeloTTS se descargara en el primer uso."
fi

# -----------------------------------------------------------------------------
# 5. Pesos de Chatterbox Multilingual (motor por defecto)
#
# Son los ficheros planos que lee ChatterboxMultilingualTTS.from_local:
#   ve.pt        voice encoder (clonacion, 5,7 MB)
#   s3gen.pt     decoder de voz (1,06 GB)
#   conds.pt     voz por defecto del modelo
#   t3_*.safetensors  transformador (2,14 GB)
#   grapheme_*.json   lexicono de tokenizacion
# -----------------------------------------------------------------------------
mkdir -p "${CHATTERBOX_DIR}"
log "Descargando los pesos de Chatterbox (~3,2 GB) a ${CHATTERBOX_DIR}..."
download "${CHATTERBOX_REPO}/ve.pt"   "${CHATTERBOX_DIR}/ve.pt"
download "${CHATTERBOX_REPO}/s3gen.pt" "${CHATTERBOX_DIR}/s3gen.pt"
download "${CHATTERBOX_REPO}/conds.pt" "${CHATTERBOX_DIR}/conds.pt"
download "${CHATTERBOX_REPO}/grapheme_mtl_merged_expanded_v1.json" \
         "${CHATTERBOX_DIR}/grapheme_mtl_merged_expanded_v1.json"
download "${CHATTERBOX_REPO}/${CHATTERBOX_T3_FILE}" "${CHATTERBOX_DIR}/${CHATTERBOX_T3_FILE}"

# Cangjie5_TC.json lo pide el tokenizador chino con hf_hub_download(...,
# cache_dir=model_dir): necesita el layout de cache de HuggingFace, no un
# fichero suelto. Se prepara aqui desde el contenedor para que en tiempo de
# ejecucion no haya red.
if command -v docker >/dev/null 2>&1; then
    log "Preparando el cache de HuggingFace de Chatterbox (Cangjie5_TC.json)..."
    docker run --rm \
        -v "${CHATTERBOX_DIR}:/models/chatterbox" \
        --entrypoint python \
        rss2-tts:latest \
        -c "from huggingface_hub import hf_hub_download; \
hf_hub_download(repo_id='ResembleAI/chatterbox', filename='Cangjie5_TC.json', cache_dir='/models/chatterbox'); \
print('cache listo')" \
        || warn "No se pudo preparar el cache de Cangjie. Solo afecta al chino."
else
    warn "Sin Docker: el cache de Cangjie se preparara en el primer uso (solo chino)."
fi

verify || die "La preparacion no ha completado."
log "Listo. Arranca el servicio con: docker compose up -d tts"
