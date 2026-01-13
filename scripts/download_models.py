import logging
import ssl
import nltk
import os
import urllib.request
from transformers import AutoTokenizer, AutoModelForSeq2SeqLM

# ================================================================
# Logging
# ================================================================
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(levelname)s - %(message)s"
)
LOG = logging.getLogger("download_models")

# ================================================================
# SSL FIX
# ================================================================
try:
    _create_unverified_https_context = ssl._create_unverified_context
except AttributeError:
    pass
else:
    ssl._create_default_https_context = _create_unverified_https_context

# ================================================================
# Paths y modelos
# ================================================================
NLTK_PACKAGES = ["punkt", "punkt_tab", "stopwords"]

NLLB_MODEL = "facebook/nllb-200-distilled-600M"

FASTTEXT_URL = "https://dl.fbaipublicfiles.com/fasttext/supervised-models/lid.218.bin"
FASTTEXT_DEST = "/app/models/lid.218.bin"  # donde lo espera tu worker


# ================================================================
# Descargar NLTK
# ================================================================
def download_nltk():
    for pkg in NLTK_PACKAGES:
        try:
            path = f"tokenizers/{pkg}" if pkg.startswith("punkt") else f"corpora/{pkg}"
            nltk.data.find(path)
            LOG.info(f"NLTK '{pkg}' already installed")
        except LookupError:
            LOG.info(f"Downloading NLTK '{pkg}'...")
            nltk.download(pkg, quiet=True)
            LOG.info(f"Downloaded OK: {pkg}")

# ================================================================
# Descargar NLLB
# ================================================================
def download_nllb(model_name: str):
    LOG.info(f"Downloading NLLB model: {model_name}")
    try:
        AutoTokenizer.from_pretrained(model_name)
        AutoModelForSeq2SeqLM.from_pretrained(model_name)
        LOG.info(f"Downloaded OK: {model_name}")
    except Exception as e:
        LOG.error(f"Failed downloading NLLB model {model_name}: {e}")

# ================================================================
# Descargar fastText LID.218
# ================================================================
def download_fasttext():
    # Crear carpeta /app/models si no existe
    dest_dir = os.path.dirname(FASTTEXT_DEST)
    os.makedirs(dest_dir, exist_ok=True)

    # Si ya existe, no lo descargamos
    if os.path.exists(FASTTEXT_DEST):
        LOG.info(f"fastText LID already exists at {FASTTEXT_DEST}")
        return

    LOG.info(f"Downloading fastText LID model from {FASTTEXT_URL}")

    try:
        urllib.request.urlretrieve(FASTTEXT_URL, FASTTEXT_DEST)
        LOG.info(f"Downloaded fastText LID model to {FASTTEXT_DEST}")
    except Exception as e:
        LOG.error(f"Failed to download fastText LID model: {e}")

# ================================================================
# Main
# ================================================================
if __name__ == "__main__":
    LOG.info("Downloading NLTK data...")
    download_nltk()

    LOG.info("Downloading NLLB model...")
    download_nllb(NLLB_MODEL)

    LOG.info("Downloading fastText LID model...")
    download_fasttext()

    LOG.info("All downloads completed successfully.")

