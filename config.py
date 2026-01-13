import os
from dotenv import load_dotenv

load_dotenv()

DB_CONFIG = {
    "dbname": os.getenv("DB_NAME", "rss"),
    "user": os.getenv("DB_USER", "rss"),
    "password": os.getenv("DB_PASS", ""),
    "host": os.getenv("DB_HOST", "localhost"),
    "port": int(os.getenv("DB_PORT", 5432)),
}

# Write DB (primary) - for workers/ingestion
DB_WRITE_CONFIG = {
    "dbname": os.getenv("DB_NAME", "rss"),
    "user": os.getenv("DB_USER", "rss"),
    "password": os.getenv("DB_PASS", ""),
    "host": os.getenv("DB_WRITE_HOST", os.getenv("DB_HOST", "localhost")),
    "port": int(os.getenv("DB_PORT", 5432)),
}

# Read DB (replica) - for web queries
DB_READ_CONFIG = {
    "dbname": os.getenv("DB_NAME", "rss"),
    "user": os.getenv("DB_USER", "rss"),
    "password": os.getenv("DB_PASS", ""),
    "host": os.getenv("DB_READ_HOST", os.getenv("DB_HOST", "localhost")),
    "port": int(os.getenv("DB_PORT", 5432)),
}

# Redis Cache
REDIS_HOST = os.getenv("REDIS_HOST", "localhost")
REDIS_PORT = int(os.getenv("REDIS_PORT", 6379))
REDIS_PASSWORD = os.getenv("REDIS_PASSWORD", None)  # None = sin autenticación (para compatibilidad)
REDIS_TTL_DEFAULT = int(os.getenv("REDIS_TTL_DEFAULT", 60))

SECRET_KEY = os.getenv("SECRET_KEY", "CAMBIA_ESTA_CLAVE_POR_ALGO_LARGO_Y_ALEATORIO")

DEFAULT_LANG = os.getenv("DEFAULT_LANG", "es")
DEFAULT_TRANSLATION_LANG = os.getenv("DEFAULT_TRANSLATION_LANG", "es")

WEB_TRANSLATED_DEFAULT = int(os.getenv("WEB_TRANSLATED_DEFAULT", "1"))
# Configuración de paginación
NEWS_PER_PAGE_DEFAULT = 30  # Reducido de 50 para mejor rendimiento

RSS_MAX_WORKERS = int(os.getenv("RSS_MAX_WORKERS", "3"))  # Reducido de 10 a 3
RSS_FEED_TIMEOUT = int(os.getenv("RSS_FEED_TIMEOUT", "60")) # Aumentado timeout
RSS_MAX_FAILURES = int(os.getenv("RSS_MAX_FAILURES", "5"))

TARGET_LANGS = os.getenv("TARGET_LANGS", "es")

TRANSLATOR_BATCH = int(os.getenv("TRANSLATOR_BATCH", "2"))  # Reducido de 4 a 2
ENQUEUE = int(os.getenv("ENQUEUE", "50")) # Reducido de 200 a 50
TRANSLATOR_SLEEP_IDLE = float(os.getenv("TRANSLATOR_SLEEP_IDLE", "10")) # Aumentado de 5 a 10

MAX_SRC_TOKENS = int(os.getenv("MAX_SRC_TOKENS", "512"))
MAX_NEW_TOKENS = int(os.getenv("MAX_NEW_TOKENS", "256"))

NUM_BEAMS_TITLE = int(os.getenv("NUM_BEAMS_TITLE", "1")) # Reducido beams para menos CPU
NUM_BEAMS_BODY = int(os.getenv("NUM_BEAMS_BODY", "1"))

UNIVERSAL_MODEL = os.getenv("UNIVERSAL_MODEL", "facebook/nllb-200-1.3B")
DEVICE = os.getenv("DEVICE", "cpu")

TOKENIZERS_PARALLELISM = os.getenv("TOKENIZERS_PARALLELISM", "false")
PYTHONUNBUFFERED = os.getenv("PYTHONUNBUFFERED", "1")
PYTORCH_CUDA_ALLOC_CONF = os.getenv("PYTORCH_CUDA_ALLOC_CONF", "")

