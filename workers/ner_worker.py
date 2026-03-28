import os
import time
import logging
import re
import string
import json
from typing import List, Tuple, Set, Dict
from collections import Counter

import psycopg2
import psycopg2.extras
import spacy
from bs4 import BeautifulSoup

# ==========================================================
# Logging
# ==========================================================
logging.basicConfig(
    level=logging.INFO,
    format='[NER] %(asctime)s %(levelname)s: %(message)s'
)
log = logging.getLogger("ner_worker")

# ==========================================================
# Config DB
# ==========================================================
DB = dict(
    host=os.environ.get("DB_HOST", "localhost"),
    port=int(os.environ.get("DB_PORT", 5432)),
    dbname=os.environ.get("DB_NAME", "rss"),
    user=os.environ.get("DB_USER", "rss"),
    password=os.environ.get("DB_PASS", "x"),
)

NER_LANG = os.environ.get("NER_LANG", "es").strip().lower()
BATCH = int(os.environ.get("NER_BATCH", 64))

# ==========================================================
# Mapeo de entidades spaCy → nuestro modelo SQL
# ==========================================================
ENT_LABELS = {
    "PERSON": "persona",
    "PER": "persona",
    "ORG": "organizacion",
    "GPE": "lugar",
    "LOC": "lugar",
    "MISC": "tema",
}

# ==========================================================
# Configuración global de entidades (Synonyms / Blacklist)
# ==========================================================
ENTITY_CONFIG = {"blacklist": [], "synonyms": {}}
REVERSE_SYNONYMS = {}

def load_entity_config():
    global ENTITY_CONFIG, REVERSE_SYNONYMS
    path = "entity_config.json"
    if os.path.exists(path):
        try:
            with open(path, "r", encoding="utf-8") as f:
                ENTITY_CONFIG = json.load(f)
            
            # Construir mapa inverso para búsqueda rápida de sinónimos
            REVERSE_SYNONYMS = {}
            for canonical, aliases in ENTITY_CONFIG.get("synonyms", {}).items():
                for alias in aliases:
                    REVERSE_SYNONYMS[alias.lower()] = canonical
                REVERSE_SYNONYMS[canonical.lower()] = canonical
            
            log.info(f"Loaded entity_config.json: {len(ENTITY_CONFIG.get('blacklist', []))} blacklisted, {len(ENTITY_CONFIG.get('synonyms', {}))} synonym groups")
        except Exception as e:
            log.error(f"Error loading entity_config.json: {e}")

def get_canonical_name(text: str) -> str:
    if not text:
        return text
    lower = text.lower()
    return REVERSE_SYNONYMS.get(lower, text)

def is_blacklisted(text: str) -> bool:
    if not text:
        return True
    lower = text.lower()
    # Check full match
    if lower in [item.lower() for item in ENTITY_CONFIG.get("blacklist", [])]:
        return True
    # Check if it's just a number
    if re.fullmatch(r"[0-9\s\.,\-:/]+", lower):
        return True
    return False

# ==========================================================
# Limpieza avanzada
# ==========================================================
_ws_re = re.compile(r"\s+")

HTML_TRASH_PATTERNS = [
    r"<[^>]+>",
    r"&[a-z]+;",
    r"&#\d+;?",
    r'width="\d+"',
    r'height="\d+"',
]

GENERIC_BAD_TAGS = {
    "república", "estado", "centro", "gobierno", "el gobierno",
    "gobiernos", "report", "sp", "unión", "union", "dólares",
    "dolar", "dólar", "the post", "post", "artículo", "el artículo",
    "la ciudad", "mundo", "país", "pais", "países", "paises",
    "la noche", "la publicación", "este miércoles", "el miércoles",
    "hoy", "ayer", "mañana", "servicio", "servicios", "el presidente",
    "presidente", "el ministro", "ministro", "la guerra", "guerra",
    "seguridad", "wp-content", "internal_photos", "/internal_photos",
    "https", "http", "src"
}

STOPWORDS = set()

ARTICLES = {
    "el", "la", "los", "las", "un", "una", "uno", "al", "del"
}

# Límites
TOPIC_MIN_CHARS = 4
TOPIC_MAX_WORDS = 6
TOPIC_MAX_PER_DOC = 15


# ==========================================================
# Helpers
# ==========================================================
def get_conn():
    return psycopg2.connect(**DB)


def _looks_like_attr_or_path(text_lower: str) -> bool:
    """Filtra basura tipo rutas, html, atributos, URLs, etc."""
    if text_lower.startswith("/"):
        return True
    if "http://" in text_lower or "https://" in text_lower:
        return True
    if any(ext in text_lower for ext in (".jpg", ".jpeg", ".png", ".gif", ".webp")):
        return True
    if re.search(r"\b(src|alt|style|class)\s*=", text_lower):
        return True
    if "data-" in text_lower:
        return True
    if re.search(r"&#\d+;?", text_lower):
        return True
    if "=" in text_lower and " " not in text_lower.strip():
        return True

    # tokens-hash tipo "ajsdh7287sdhjshd8" (solo si no tiene espacios)
    if " " not in text_lower and re.fullmatch(r"[a-z0-9_]{15,}", text_lower.replace("-", "")):
        return True

    # palabras sin espacios largas con guiones
    if "-" in text_lower and " " not in text_lower:
        return True

    return False


# ==========================================================
# Limpieza de entidades
# ==========================================================
def clean_tag_text(text: str) -> str | None:
    if not text:
        return None

    try:
        text = BeautifulSoup(text, "html.parser").get_text()
    except Exception:
        pass

    for pat in HTML_TRASH_PATTERNS:
        text = re.sub(pat, "", text)

    text = _ws_re.sub(" ", text).strip()
    text = text.strip(string.punctuation + " ")

    if len(text) < 3:
        return None
    if re.search(r"[<>/\\]", text):
        return None

    if is_blacklisted(text):
        return None

    lower = text.lower()
    if lower.startswith("href="):
        return None
    if _looks_like_attr_or_path(lower):
        return None
    if lower in GENERIC_BAD_TAGS:
        return None

    # Normalización vía entity_config
    canonical = get_canonical_name(text)
    
    return canonical


# ==========================================================
# Limpieza de topics (noun-chunks)
# ==========================================================
def clean_topic_text(text: str) -> str | None:
    if not text:
        return None

    try:
        text = BeautifulSoup(text, "html.parser").get_text()
    except Exception:
        pass

    for pat in HTML_TRASH_PATTERNS:
        text = re.sub(pat, "", text)

    text = _ws_re.sub(" ", text).strip()
    text = text.strip(string.punctuation + " ")

    if len(text) < TOPIC_MIN_CHARS:
        return None

    if is_blacklisted(text):
        return None

    lower = text.lower()
    if _looks_like_attr_or_path(lower):
        return None

    tokens = [
        t.strip(string.punctuation)
        for t in lower.split()
        if t.strip(string.punctuation)
    ]
    if not tokens:
        return None

    # remover artículos iniciales
    if tokens[0] in ARTICLES:
        tokens = tokens[1:]
        if not tokens:
            return None

    norm = " ".join(tokens).strip()

    if len(norm) < TOPIC_MIN_CHARS:
        return None
    if norm in GENERIC_BAD_TAGS:
        return None
    if len(tokens) > TOPIC_MAX_WORDS:
        return None
    if all(t in STOPWORDS for t in tokens):
        return None

    return norm


# ==========================================================
# Extracción NER + Topics
# ==========================================================
def extract_entities_and_topics(nlp, text: str) -> Tuple[List[Tuple[str, str]], List[Tuple[str, str]]]:
    ents = []
    topics = []

    if not text:
        return ents, topics

    doc = nlp(text)

    # --- ENTIDADES ---
    for ent in doc.ents:
        tipo = ENT_LABELS.get(ent.label_)
        if not tipo:
            continue

        cleaned = clean_tag_text(ent.text)
        if not cleaned:
            continue
            
        ents.append((cleaned, tipo))

    # --- TOPICS ---
    topic_counter = Counter()
    for chunk in doc.noun_chunks:
        cleaned = clean_topic_text(chunk.text)
        if cleaned:
            topic_counter[cleaned] += 1

    ent_values = {v.lower() for (v, _) in ents}

    for val, _count in topic_counter.most_common(TOPIC_MAX_PER_DOC):
        if val.lower() in ent_values:
            continue
        topics.append((val, "tema"))

    return list(set(ents)), list(set(topics))


# ==========================================================
# Worker principal
# ==========================================================
def main():
    global STOPWORDS

    # Cargar spaCy
    log.info("Cargando modelo spaCy es_core_news_lg...")
    nlp = spacy.load("es_core_news_lg", disable=["lemmatizer", "textcat"])
    STOPWORDS = set(nlp.Defaults.stop_words)
    log.info("Modelo spaCy cargado correctamente.")

    # Cargar configuración de entidades
    load_entity_config()

    while True:
        try:
            with get_conn() as conn:
                with conn.cursor(cursor_factory=psycopg2.extras.DictCursor) as cur:
                    cur.execute(
                        """
                        SELECT t.id, t.titulo_trad, t.resumen_trad, t.noticia_id
                        FROM traducciones t
                        WHERE t.status = 'done'
                          AND t.lang_to = %s
                          AND NOT EXISTS (
                              SELECT 1 FROM tags_noticia tn WHERE tn.traduccion_id = t.id
                          )
                        ORDER BY t.id DESC
                        LIMIT %s;
                        """,
                        (NER_LANG, BATCH),
                    )

                    rows = cur.fetchall()

                    if not rows:
                        time.sleep(10)
                        continue

                    log.info(f"Procesando {len(rows)} traducciones para NER/temas...")

                    inserted_links = 0

                    for r in rows:
                        noticia_id = r["noticia_id"]
                        traduccion_id = r["id"]
                        
                        text = f"{r['titulo_trad'] or ''}\n{r['resumen_trad'] or ''}".strip()
                        if not text:
                            # Para evitar re-procesar, insertamos un tag especial '_none_'
                            tags = [("_none_", "sistema")]
                        else:
                            ents, topics = extract_entities_and_topics(nlp, text)
                            tags = ents + topics
                            if not tags:
                                tags = [("_none_", "sistema")]

                        for valor, tipo in tags:
                            try:
                                # Usar commit parcial por noticia para evitar abortar todo el batch
                                cur.execute(
                                    """
                                    INSERT INTO tags (valor, tipo)
                                    VALUES (%s, %s)
                                    ON CONFLICT (valor, tipo)
                                    DO UPDATE SET valor = EXCLUDED.valor
                                    RETURNING id;
                                    """,
                                    (valor, tipo),
                                )
                                tag_id = cur.fetchone()[0]

                                cur.execute(
                                    """
                                    INSERT INTO tags_noticia (traduccion_id, noticia_id, tag_id)
                                    VALUES (%s, %s, %s)
                                    ON CONFLICT DO NOTHING;
                                    """,
                                    (traduccion_id, noticia_id, tag_id),
                                )

                                if cur.rowcount > 0:
                                    inserted_links += 1
                            except Exception as e:
                                log.error(f"Error insertando tag '{valor}': {e}")
                                conn.rollback()
                                # Volvemos a empezar el loop de tags para esta noticia no es buena idea,
                                # pero el rollback abortó la transacción del cursor.
                                # En psycopg2, tras rollback hay que seguir o cerrar.
                                pass
                        
                        conn.commit()

                    log.info(f"Lote NER OK. Nuevas relaciones tag_noticia: {inserted_links}")

        except Exception as e:
            log.exception(f"Error general en NER loop: {e}")
            time.sleep(10)


if __name__ == "__main__":
    main()

