import os
import time
import logging
import re
from typing import List, Optional

import psycopg2
import psycopg2.extras
from psycopg2.extras import execute_values

import ctranslate2
from transformers import AutoTokenizer
from langdetect import detect, DetectorFactory

DetectorFactory.seed = 0

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s: %(message)s")
LOG = logging.getLogger("translator")

# =========================
# DB CONFIG
# =========================
DB_CONFIG = {
    "host": os.environ.get("DB_HOST", "localhost"),
    "port": int(os.environ.get("DB_PORT", 5432)),
    "dbname": os.environ.get("DB_NAME", "rss"),
    "user": os.environ.get("DB_USER", "rss"),
    "password": os.environ.get("DB_PASS", "x"),
}

# =========================
# ENV HELPERS
# =========================
def _env_list(name: str, default="es"):
    raw = os.environ.get(name)
    if raw:
        return [s.strip() for s in raw.split(",") if s.strip()]
    return [default]

def _env_int(name: str, default: int = 8):
    v = os.environ.get(name)
    try:
        return int(v)
    except Exception:
        return default

def _env_float(name: str, default: float = 5.0):
    v = os.environ.get(name)
    try:
        return float(v)
    except Exception:
        return default

def _env_str(name: str, default=None):
    v = os.environ.get(name)
    return v if v else default

# =========================
# CONFIG
# =========================
TARGET_LANGS = _env_list("TARGET_LANGS")  # por defecto ["es"]
BATCH_SIZE = _env_int("TRANSLATOR_BATCH", 8)
ENQUEUE_MAX = _env_int("ENQUEUE", 200)
SLEEP_IDLE = _env_float("TRANSLATOR_SLEEP_IDLE", 5.0)

# CTranslate2 Configuration
CT2_MODEL_PATH = _env_str("CT2_MODEL_PATH", "./models/nllb-ct2")
CT2_DEVICE = _env_str("CT2_DEVICE", "auto")  # auto, cpu, cuda
CT2_COMPUTE_TYPE = _env_str("CT2_COMPUTE_TYPE", "auto")  # auto, int8, float16, int8_float16

MAX_SRC_TOKENS = _env_int("MAX_SRC_TOKENS", 512)
MAX_NEW_TOKENS_TITLE = _env_int("MAX_NEW_TOKENS_TITLE", 96)
MAX_NEW_TOKENS_BODY = _env_int("MAX_NEW_TOKENS_BODY", 512)

NUM_BEAMS_TITLE = _env_int("NUM_BEAMS_TITLE", 2)
NUM_BEAMS_BODY = _env_int("NUM_BEAMS_BODY", 2)

# HuggingFace model name (used for tokenizer)
UNIVERSAL_MODEL = _env_str("UNIVERSAL_MODEL", "facebook/nllb-200-distilled-600M")
IDENTITY_PAISES_ES = _env_int("IDENTITY_PAISES_ES", 58)

BODY_CHARS_CHUNK = _env_int("BODY_CHARS_CHUNK", 900)

# =========================
# LANG MAP
# =========================
NLLB_LANG = {
    "es": "spa_Latn", "en": "eng_Latn", "fr": "fra_Latn", "de": "deu_Latn",
    "it": "ita_Latn", "pt": "por_Latn", "nl": "nld_Latn", "sv": "swe_Latn",
    "da": "dan_Latn", "fi": "fin_Latn", "no": "nob_Latn",
    "pl": "pol_Latn", "cs": "ces_Latn", "sk": "slk_Latn",
    "sl": "slv_Latn", "hu": "hun_Latn", "ro": "ron_Latn",
    "el": "ell_Grek", "ru": "rus_Cyrl", "uk": "ukr_Cyrl",
    "tr": "tur_Latn", "ar": "arb_Arab", "fa": "pes_Arab",
    "he": "heb_Hebr", "zh": "zho_Hans", "ja": "jpn_Jpan",
    "ko": "kor_Hang", "vi": "vie_Latn",
}

def map_to_nllb(code: Optional[str]):
    if not code:
        return None
    c = code.strip().lower()
    return NLLB_LANG.get(c, f"{c}_Latn")

def normalize_lang(code: Optional[str], default=None):
    return (code or default).strip().lower() if code else default

def _norm(s: str) -> str:
    return re.sub(r"\W+", "", (s or "").lower()).strip()

# =========================
# DB
# =========================
def get_conn():
    return psycopg2.connect(**DB_CONFIG)

def ensure_indexes(conn):
    with conn.cursor() as cur:
        cur.execute("CREATE INDEX IF NOT EXISTS traducciones_lang_to_status_idx ON traducciones (lang_to, status);")
        cur.execute("CREATE INDEX IF NOT EXISTS traducciones_status_idx ON traducciones (status);")
    conn.commit()

    pass # Moved to translation_ops.py

    pass # Moved to translation_ops.py

def fetch_pending_batch(conn, lang_to: str, batch: int):
    """Fetch pending translations with row locking to support multiple workers."""
    if batch <= 0:
        return []
    
    # Use FOR UPDATE SKIP LOCKED to allow multiple workers
    # Each worker will get different rows without conflicts
    with conn.cursor(cursor_factory=psycopg2.extras.DictCursor) as cur:
        cur.execute(
            """
            SELECT t.id AS tr_id, t.noticia_id, t.lang_from, t.lang_to,
                   n.titulo, n.resumen
            FROM traducciones t
            JOIN noticias n ON n.id=t.noticia_id
            WHERE t.lang_to=%s AND t.status='pending'
            ORDER BY t.id
            LIMIT %s
            FOR UPDATE OF t SKIP LOCKED;
            """,
            (lang_to, batch),
        )
        rows = cur.fetchall()
        
        # Update status within the same transaction while rows are locked
        if rows:
            ids = [r["tr_id"] for r in rows]
            cur.execute("UPDATE traducciones SET status='processing' WHERE id = ANY(%s)", (ids,))
    
    conn.commit()
    return rows

# =========================
# LANGUAGE DETECTION
# =========================
def detect_lang(text1: str, text2: str):
    txt = (text1 or "").strip() or (text2 or "").strip()
    if not txt:
        return None
    try:
        return detect(txt)
    except Exception:
        return None

# =========================
# MODEL LOADING (CTranslate2)
# =========================
_TOKENIZER = None
_TRANSLATOR = None
_DEVICE = None

def _resolve_device():
    if CT2_DEVICE == "cpu":
        return "cpu"
    if CT2_DEVICE == "cuda":
        return "cuda"
    # auto
    return "cuda" if ctranslate2.get_cuda_device_count() > 0 else "cpu"

def _ensure_ct2_model():
    """Convert HuggingFace model to CTranslate2 format if not exists."""
    import os
    import subprocess
    
    model_dir = CT2_MODEL_PATH
    
    # Check if model already exists
    if os.path.isdir(model_dir) and os.path.exists(os.path.join(model_dir, "model.bin")):
        LOG.info("CTranslate2 model already exists at %s", model_dir)
        return True
    
    LOG.info("CTranslate2 model not found, converting from %s...", UNIVERSAL_MODEL)
    LOG.info("This may take 5-10 minutes on first run...")
    
    # Create directory if needed
    os.makedirs(os.path.dirname(model_dir) or ".", exist_ok=True)
    
    # Convert the model
    quantization = CT2_COMPUTE_TYPE if CT2_COMPUTE_TYPE != "auto" else "int8_float16"
    
    cmd = [
        "ct2-transformers-converter",
        "--model", UNIVERSAL_MODEL,
        "--output_dir", model_dir,
        "--quantization", quantization,
        "--force"
    ]
    
    try:
        LOG.info("Running: %s", " ".join(cmd))
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=1800)
        
        if result.returncode != 0:
            LOG.error("Model conversion failed: %s", result.stderr)
            return False
        
        LOG.info("Model conversion completed successfully")
        return True
        
    except subprocess.TimeoutExpired:
        LOG.error("Model conversion timed out after 30 minutes")
        return False
    except Exception as e:
        LOG.error("Model conversion error: %s", e)
        return False

def get_universal_components():
    global _TOKENIZER, _TRANSLATOR, _DEVICE
    if _TRANSLATOR:
        return _TOKENIZER, _TRANSLATOR

    # Ensure CT2 model exists (convert if needed)
    if not _ensure_ct2_model():
        raise RuntimeError(f"Failed to load/convert CTranslate2 model at {CT2_MODEL_PATH}")

    device = _resolve_device()
    
    LOG.info("Loading CTranslate2 model from %s on %s", CT2_MODEL_PATH, device)
    
    _TRANSLATOR = ctranslate2.Translator(
        CT2_MODEL_PATH,
        device=device,
        compute_type=CT2_COMPUTE_TYPE,
    )
    _TOKENIZER = AutoTokenizer.from_pretrained(UNIVERSAL_MODEL)
    _DEVICE = device
    
    LOG.info("CTranslate2 model loaded successfully")
    return _TOKENIZER, _TRANSLATOR

# =========================
# TRANSLATION (CTranslate2)
# =========================
def _safe_src_len(tokenizer):
    max_len = getattr(tokenizer, "model_max_length", 1024) or 1024
    if max_len > 100000:
        max_len = 1024
    return min(MAX_SRC_TOKENS, max_len - 16)

def _translate_texts(src, tgt, texts, beams, max_new_tokens):
    """Translate texts using CTranslate2."""
    if not texts:
        return []
    
    clean = [(t or "").strip() for t in texts]
    if all(not t for t in clean):
        return ["" for _ in clean]

    tok, translator = get_universal_components()
    src_code = map_to_nllb(src)
    tgt_code = map_to_nllb(tgt)

    # Set source language on tokenizer
    try:
        tok.src_lang = src_code
    except Exception:
        pass

    safe_len = _safe_src_len(tok)
    max_new = max(16, min(int(max_new_tokens), MAX_NEW_TOKENS_BODY))

    # Tokenize: convert text to tokens
    sources = []
    for t in clean:
        if t:
            ids = tok.encode(t, truncation=True, max_length=safe_len)
            tokens = tok.convert_ids_to_tokens(ids)
            sources.append(tokens)
        else:
            sources.append([])

    # Target language prefix for NLLB
    target_prefix = [[tgt_code]] * len(sources)

    # Translate with CTranslate2
    start = time.time()
    results = translator.translate_batch(
        sources,
        target_prefix=target_prefix,
        beam_size=beams,
        max_decoding_length=max_new,
        repetition_penalty=1.2,
        no_repeat_ngram_size=4,
    )
    dt = time.time() - start

    # Decode results
    translated = []
    total_tokens = 0
    for result, src_tokens in zip(results, sources):
        if result.hypotheses:
            # Skip the first token (language prefix)
            tokens = result.hypotheses[0][1:]
            total_tokens += len(tokens) + len(src_tokens)
            text = tok.decode(tok.convert_tokens_to_ids(tokens))
            translated.append(text.strip())
        else:
            translated.append("")

    if total_tokens > 0:
        LOG.info("  → tokens=%d tiempo=%.2fs velocidad=%d tok/s", 
                 total_tokens, dt, int(total_tokens / dt) if dt > 0 else 0)

    return translated

def _split_body_into_chunks(text: str) -> List[str]:
    text = (text or "").strip()
    if len(text) <= BODY_CHARS_CHUNK:
        return [text] if text else []

    parts = re.split(r'(\n\n+|(?<=[\.\!\?؛؟。])\s+)', text)
    chunks = []
    current = ""

    for part in parts:
        if not part:
            continue
        if len(current) + len(part) <= BODY_CHARS_CHUNK:
            current += part
        else:
            if current.strip():
                chunks.append(current.strip())
            current = part
    if current.strip():
        chunks.append(current.strip())

    if not chunks:
        return [text]
    return chunks

def translate_body_long(src: str, tgt: str, body: str) -> str:
    body = (body or "").strip()
    if not body:
        return ""

    chunks = _split_body_into_chunks(body)
    if len(chunks) == 1:
        translated = _translate_texts(src, tgt, [body], NUM_BEAMS_BODY, MAX_NEW_TOKENS_BODY)[0]
        return translated.strip()

    translated_chunks = []
    for ch in chunks:
        tr = _translate_texts(src, tgt, [ch], NUM_BEAMS_BODY, MAX_NEW_TOKENS_BODY)[0]
        translated_chunks.append(tr.strip())
    return "\n\n".join(c for c in translated_chunks if c)

# =========================
# BATCH PROCESS
# =========================
def process_batch(conn, rows):
    todo = []
    done = []
    errors = []

    for r in rows:
        lang_to = normalize_lang(r["lang_to"], "es") or "es"
        lang_from = (
            normalize_lang(r["lang_from"])
            or detect_lang(r["titulo"], r["resumen"])
            or "en"
        )

        titulo = (r["titulo"] or "").strip()
        resumen = (r["resumen"] or "").strip()

        if map_to_nllb(lang_from) == map_to_nllb(lang_to):
            done.append((titulo, resumen, lang_from, r["tr_id"]))
        else:
            todo.append({
                "tr_id": r["tr_id"],
                "lang_from": lang_from,
                "lang_to": lang_to,
                "titulo": titulo,
                "resumen": resumen,
            })

    from collections import defaultdict
    groups = defaultdict(list)
    for item in todo:
        key = (item["lang_from"], item["lang_to"])
        groups[key].append(item)

    for (lang_from, lang_to), items in groups.items():
        LOG.info("Translating %s -> %s (%d items)", lang_from, lang_to, len(items))

        titles = [i["titulo"] for i in items]

        try:
            tt = _translate_texts(
                lang_from,
                lang_to,
                titles,
                NUM_BEAMS_TITLE,
                MAX_NEW_TOKENS_TITLE,
            )

            bodies_translated: List[str] = []
            for i in items:
                bodies_translated.append(
                    translate_body_long(lang_from, lang_to, i["resumen"])
                )

            for i, ttr, btr in zip(items, tt, bodies_translated):
                ttr = (ttr or "").strip()
                btr = (btr or "").strip()

                if not ttr or _norm(ttr) == _norm(i["titulo"]):
                    ttr = i["titulo"]
                if not btr or _norm(btr) == _norm(i["resumen"]):
                    btr = i["resumen"]

                # CLEANING: Remove <unk> tokens
                if ttr:
                    ttr = ttr.replace("<unk>", "").replace("  ", " ").strip()
                if btr:
                    btr = btr.replace("<unk>", "").replace("  ", " ").strip()

                done.append((ttr, btr, lang_from, i["tr_id"]))

        except Exception as e:
            err = str(e)[:800]
            LOG.exception("Error translating %s -> %s: %s", lang_from, lang_to, err)
            for i in items:
                errors.append((err, i["tr_id"]))

    with conn.cursor() as cur:
        if done:
            execute_values(
                cur,
                """
                UPDATE traducciones AS t
                SET titulo_trad=v.titulo_trad,
                    resumen_trad=v.resumen_trad,
                    lang_from=COALESCE(t.lang_from, v.lang_from),
                    status='done',
                    error=NULL
                FROM (VALUES %s) AS v(titulo_trad,resumen_trad,lang_from,id)
                WHERE t.id=v.id;
                """,
                done,
            )
            
            # --- NEW: Persist stats ---
            # Insert a record for each translated item into translation_stats
            # We need the language 'lang_to'. In this batch, lang_to is uniform for the group usually,
            # but let's extract it from the 'done' items structure if we had it, or pass it down.
            # In process_batch, we iterate groups. 
            # 'done' list here is flattened from multiple groups? 
            # process_batch logic:
            # 1. 'done' checks map_to_nllb identity (already done?) -> these have lang_to from row?
            # 2. 'groups' loop -> translates -> appends to 'done' with lang_from.
            # 
            # Wait, 'done' list doesn't have lang_to in the tuple: (titulo, resumen, lang_from, tr_id).
            # We need to change the 'done' collection to include lang_to OR we insert based on tr_id.
            
            # Let's verify process_batch logic.
            # rows has all info.
            # define a mapping tr_id -> lang_to
            tr_map = {r["tr_id"]: r["lang_to"] for r in rows}
            
            stats_data = []
            for item in done:
                 # item is (titulo, resumen, lang_from, tr_id)
                 lang_from = item[2]
                 lang_to = tr_map.get(item[3], "es")
                 stats_data.append((lang_from, lang_to))

            execute_values(
                cur,
                "INSERT INTO translation_stats (lang_from, lang_to) VALUES %s",
                stats_data
            )
            # --------------------------

        if errors:
            execute_values(
                cur,
                """
                UPDATE traducciones AS t
                SET status='error', error=v.error
                FROM (VALUES %s) AS v(error,id)
                WHERE t.id=v.id;
                """,
                errors,
            )

    conn.commit()

def process_entity_summaries(conn):
    """Translate pending entity summaries from Wikipedia."""
    from cache import cache_del
    
    LOG.info("DEBUG: Checking for pending entity summaries...")
    
    with conn.cursor(cursor_factory=psycopg2.extras.DictCursor) as cur:
        cur.execute("""
            SELECT id, entity_name, summary, summary_en 
            FROM entity_images 
            WHERE status_es = 'pending' 
            LIMIT 20
            FOR UPDATE SKIP LOCKED;
        """)
        rows = cur.fetchall()
        
        if not rows:
            return False
            
        LOG.info("DEBUG: Found %d pending entity summaries to process", len(rows))
        
        for r in rows:
            entity_id = r["id"]
            name = r["entity_name"]
            text = r["summary_en"] or r["summary"]
            
            if not text:
                cur.execute("UPDATE entity_images SET status_es = 'none' WHERE id = %s", (entity_id,))
                continue
                
            try:
                # English -> Spanish
                translated = translate_body_long('en', 'es', text)
                if translated:
                    cur.execute("""
                        UPDATE entity_images 
                        SET summary_es = %s, status_es = 'done' 
                        WHERE id = %s
                    """, (translated, entity_id))
                    # Invalidate cache
                    cache_del(f"wiki:data:{name.lower()}")
                    LOG.info("  → Translated entity summary: %s", name)
                else:
                     cur.execute("UPDATE entity_images SET status_es = 'error' WHERE id = %s", (entity_id,))
            except Exception as e:
                LOG.error("Error translating entity summary [%s]: %s", name, e)
                cur.execute("UPDATE entity_images SET status_es = 'error' WHERE id = %s", (entity_id,))
        
        conn.commit()
    return True

# =========================
# MAIN LOOP
# =========================
def main():
    LOG.info("Translator worker iniciado (CTranslate2)")
    os.environ.setdefault("TOKENIZERS_PARALLELISM", "false")
    get_universal_components()

    while True:
        any_work = False
        with get_conn() as conn:
            ensure_indexes(conn)
            
            # 1. Process entity summaries (Wikipedia) -> REMOVED per user request
            # Logic moved out to keep translator focused on news ONLY.
            # try:
            #     if process_entity_summaries(conn):
            #         any_work = True
            # except Exception as e:
            #     LOG.error("Error in process_entity_summaries: %s", e)

            # 2. Process news translations
            for tgt in TARGET_LANGS:
                while True:
                    rows = fetch_pending_batch(conn, tgt, BATCH_SIZE)
                    if not rows:
                        break
                    any_work = True
                    LOG.info("[%s] %d elementos", tgt, len(rows))
                    process_batch(conn, rows)

        if not any_work:
            time.sleep(SLEEP_IDLE)

if __name__ == "__main__":
    main()

