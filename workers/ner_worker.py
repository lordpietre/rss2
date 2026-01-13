import os
import time
import logging
import re
import string
from typing import List, Tuple
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

    text = BeautifulSoup(text, "html.parser").get_text()
    for pat in HTML_TRASH_PATTERNS:
        text = re.sub(pat, "", text)

    text = _ws_re.sub(" ", text).strip()
    text = text.strip(string.punctuation + " ")

    if len(text) < 3:
        log.debug(f"Clean reject (too short): {text}")
        return None
    if re.search(r"[<>/\\]", text):
        log.debug(f"Clean reject (bad chars): {text}")
        return None

    lower = text.lower()
    if lower.startswith("href="):
        log.debug(f"Clean reject (href): {text}")
        return None
    if _looks_like_attr_or_path(lower):
        log.debug(f"Clean reject (attr/path): {text}")
        return None
    if lower in GENERIC_BAD_TAGS:
        log.debug(f"Clean reject (generic bad): {text}")
        return None

    replacements = {
        "ee.uu.": "Estados Unidos",
        "los estados unidos": "Estados Unidos",
        "eeuu": "Estados Unidos",
        "eu": "Unión Europea",
        "ue": "Unión Europea",
        "kosova": "Kosovo",
        # Specific User Requests
        "trump": "Donald Trump",
        "mr. trump": "Donald Trump",
        "mr trump": "Donald Trump",
        "doland trump": "Donald Trump",
        "el presidente trump": "Donald Trump",
        "president trump": "Donald Trump",
        "ex-president trump": "Donald Trump",
        "expresidente trump": "Donald Trump",
        "putin": "Vladimir Putin",
        "vladimir putin": "Vladimir Putin",
        "v. putin": "Vladimir Putin",
        "presidente putin": "Vladimir Putin",
        # New requests
        "sanchez": "Pedro Sánchez",
        "pedro sanchez": "Pedro Sánchez",
        "p. sanchez": "Pedro Sánchez",
        "mr. sanchez": "Pedro Sánchez",
        "sánchez": "Pedro Sánchez", # explicit match just in case
        "pedro sánchez": "Pedro Sánchez",
        "maduro": "Nicolás Maduro",
        "nicolas maduro": "Nicolás Maduro",
        "mr. maduro": "Nicolás Maduro",
        "lula": "Lula da Silva",
        "lula da silva": "Lula da Silva",
        "luiz inácio lula da silva": "Lula da Silva",
    }
    if lower in replacements:
        return replacements[lower]

    # Blacklist (explicit removals requested)
    blacklist = {
        "getty images", "netflix", "fiscalia", "segun", "estoy", # People blacklist
        "and more", "app", "estamos", "ultra", # Orgs blacklist
        "hacienda", "fiscalía" 
    }
    if lower in blacklist:
        log.debug(f"Clean reject (blacklist): {text}")
        return None

    return text


# ==========================================================
# Limpieza de topics (noun-chunks)
# ==========================================================
def clean_topic_text(text: str) -> str | None:
    if not text:
        return None

    text = BeautifulSoup(text, "html.parser").get_text()
    for pat in HTML_TRASH_PATTERNS:
        text = re.sub(pat, "", text)

    text = _ws_re.sub(" ", text).strip()
    text = text.strip(string.punctuation + " ")

    if len(text) < TOPIC_MIN_CHARS:
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
    if re.fullmatch(r"[0-9\s\.,\-:/]+", norm):
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
    # log.debug(f"Processing text ({len(text)} chars): {text[:30]}...")
    # log.debug(f"Entities found: {len(doc.ents)}")

    # --- ENTIDADES ---
    for ent in doc.ents:
        tipo = ENT_LABELS.get(ent.label_)
        if not tipo:
            continue

        cleaned = clean_tag_text(ent.text)
        if not cleaned:
            # log.debug(f"Rejected entity: {ent.text} ({ent.label_})")
            continue
            
        if tipo == "persona":
            lower_cleaned = cleaned.lower()
            # Aggressive normalization rules for VIPs
            # Use token checks or substring checks carefully
            if "trump" in lower_cleaned.split(): 
                # Token 'trump' exists? e.g. "donald trump", "trump", "mr. trump"
                # Exclude family members
                family = ["ivanka", "melania", "eric", "tiffany", "barron", "lara", "mary", "fred"]
                if not any(f in lower_cleaned for f in family):
                    cleaned = "Donald Trump"
            
            elif "sanchez" in lower_cleaned or "sánchez" in lower_cleaned:
                # Be careful of other Sanchez? But user context implies Pedro.
                if "pedro" in lower_cleaned or "presidente" in lower_cleaned or lower_cleaned in ["sanchez", "sánchez"]:
                    cleaned = "Pedro Sánchez"
            
            elif "maduro" in lower_cleaned:
                cleaned = "Nicolás Maduro"
            
            elif "lula" in lower_cleaned:
                cleaned = "Lula da Silva"
            
            elif "putin" in lower_cleaned:
                cleaned = "Vladimir Putin"

        # log.debug(f"Accepted entity: {cleaned} ({tipo})")
        ents.append((cleaned, tipo))

    # --- TOPICS ---
    topic_counter = Counter()
    for chunk in doc.noun_chunks:
        cleaned = clean_topic_text(chunk.text)
        if cleaned:
            topic_counter[cleaned] += 1

    ent_values = {v for (v, _) in ents}

    for val, _count in topic_counter.most_common(TOPIC_MAX_PER_DOC):
        if val in ent_values:
            continue
        topics.append((val, "tema"))

    return list(set(ents)), list(set(topics))


# ==========================================================
# Worker principal
# ==========================================================
def main():
    global STOPWORDS

    # Cargar spaCy
    log.info("Cargando modelo spaCy es_core_news_md...")
    nlp = spacy.load("es_core_news_md", disable=["lemmatizer", "textcat"])
    STOPWORDS = set(nlp.Defaults.stop_words)
    log.info("Modelo spaCy cargado correctamente.")

    while True:
        try:
            with get_conn() as conn, conn.cursor(cursor_factory=psycopg2.extras.DictCursor) as cur:
                cur.execute(
                    """
                    SELECT t.id, t.titulo_trad, t.resumen_trad
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
                    time.sleep(5)
                    continue

                log.info(f"Procesando {len(rows)} traducciones para NER/temas...")

                inserted_links = 0

                for r in rows:
                    text = f"{r['titulo_trad'] or ''}\n{r['resumen_trad'] or ''}".strip()
                    if not text:
                        continue

                    ents, topics = extract_entities_and_topics(nlp, text)
                    tags = ents + topics
                    if not tags:
                        continue

                    for valor, tipo in tags:
                        try:
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
                                INSERT INTO tags_noticia (traduccion_id, tag_id)
                                VALUES (%s, %s)
                                ON CONFLICT DO NOTHING;
                                """,
                                (r["id"], tag_id),
                            )

                            if cur.rowcount > 0:
                                inserted_links += 1

                        except Exception:
                            log.exception("Error insertando tag/relación")

                conn.commit()
                log.info(f"Lote NER OK. Nuevas relaciones tag_noticia: {inserted_links}")

        except Exception:
            log.exception("Error general en NER loop")
            time.sleep(5)


if __name__ == "__main__":
    main()

