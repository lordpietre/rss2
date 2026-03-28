import os
import time
import logging
import re
from typing import List, Optional

import psycopg2
import psycopg2.extras
from psycopg2.extras import execute_values
from langdetect import detect, DetectorFactory
import torch
from transformers import AutoTokenizer, AutoModelForSeq2SeqLM, pipeline

DetectorFactory.seed = 0

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s: %(message)s")
LOG = logging.getLogger("translator")

DB_CONFIG = {
    "host": os.environ.get("DB_HOST", "localhost"),
    "port": int(os.environ.get("DB_PORT", 5432)),
    "dbname": os.environ.get("DB_NAME", "rss"),
    "user": os.environ.get("DB_USER", "rss"),
    "password": os.environ.get("DB_PASS", "x"),
}

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

TARGET_LANGS = _env_list("TARGET_LANGS")
BATCH_SIZE = _env_int("TRANSLATOR_BATCH", 8)
ENQUEUE_MAX = _env_int("ENQUEUE", 200)
SLEEP_IDLE = _env_float("TRANSLATOR_SLEEP_IDLE", 5.0)
MAX_SRC_TOKENS = _env_int("MAX_SRC_TOKENS", 512)
MAX_NEW_TOKENS_TITLE = _env_int("MAX_NEW_TOKENS_TITLE", 96)
MAX_NEW_TOKENS_BODY = _env_int("MAX_NEW_TOKENS_BODY", 512)
NUM_BEAMS_TITLE = _env_int("NUM_BEAMS_TITLE", 2)
NUM_BEAMS_BODY = _env_int("NUM_BEAMS_BODY", 2)
UNIVERSAL_MODEL = _env_str("UNIVERSAL_MODEL", "facebook/nllb-200-distilled-600M")
BODY_CHARS_CHUNK = _env_int("BODY_CHARS_CHUNK", 900)

LANG_CODE_MAP = {
    "en": "eng_Latn", "es": "spa_Latn", "fr": "fra_Latn", "de": "deu_Latn",
    "it": "ita_Latn", "pt": "por_Latn", "nl": "nld_Latn", "sv": "swe_Latn",
    "da": "dan_Latn", "fi": "fin_Latn", "no": "nob_Latn",
    "pl": "pol_Latn", "cs": "ces_Latn", "sk": "slk_Latn",
    "sl": "slv_Latn", "hu": "hun_Latn", "ro": "ron_Latn",
    "el": "ell_Grek", "ru": "rus_Cyrl", "uk": "ukr_Cyrl",
    "tr": "tur_Latn", "ar": "arb_Arab", "fa": "pes_Arab",
    "he": "heb_Hebr", "zh": "zho_Hans", "ja": "jpn_Jpan",
    "ko": "kor_Hang", "vi": "vie_Latn",
}

_tokenizer = None
_translator = None
_device = None

def get_translator_components():
    global _tokenizer, _translator, _device
    
    if _translator:
        return _tokenizer, _translator
    
    device = 0 if torch.cuda.is_available() else -1
    LOG.info(f"Loading model {UNIVERSAL_MODEL} on {'cuda' if device == 0 else 'cpu'}")
    
    _tokenizer = AutoTokenizer.from_pretrained(UNIVERSAL_MODEL, src_lang="eng_Latn")
    model = AutoModelForSeq2SeqLM.from_pretrained(UNIVERSAL_MODEL)
    
    if device == 0:
        model = model.to("cuda")
    
    _translator = pipeline(
        "translation",
        model=model,
        tokenizer=_tokenizer,
        device=device,
        max_length=MAX_SRC_TOKENS,
    )
    
    _device = "cuda" if device == 0 else "cpu"
    LOG.info(f"Model loaded on {_device}")
    
    return _tokenizer, _translator

def translate_texts(src: str, tgt: str, texts: List[str]) -> List[str]:
    if not texts:
        return []
    
    clean = [(t or "").strip() for t in texts]
    if all(not t for t in clean):
        return ["" for _ in clean]
    
    tok, translator = get_translator_components()
    
    src_code = LANG_CODE_MAP.get(src, f"{src}_Latn")
    tgt_code = LANG_CODE_MAP.get(tgt, "spa_Latn")
    
    results = []
    for text in clean:
        if not text:
            results.append("")
            continue
        try:
            result = translator(text, src_lang=src_code, tgt_lang=tgt_code)
            results.append(result[0]["translation_text"])
        except Exception as e:
            LOG.warning(f"Translation error: {e}")
            results.append(text)
    
    return results

def split_body_into_chunks(text: str) -> List[str]:
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
    
    return chunks if chunks else [text]

def translate_body_long(src: str, tgt: str, body: str) -> str:
    body = (body or "").strip()
    if not body:
        return ""
    
    chunks = split_body_into_chunks(body)
    if len(chunks) == 1:
        return translate_texts(src, tgt, [body])[0].strip()
    
    translated_chunks = []
    for ch in chunks:
        tr = translate_texts(src, tgt, [ch])[0]
        translated_chunks.append(tr)
    
    return " ".join(translated_chunks)

def normalize_lang(lang: Optional[str], default: str = "es") -> Optional[str]:
    if not lang:
        return default
    lang = lang.strip().lower()[:2]
    return lang if lang else default

def detect_lang(text: str) -> str:
    if not text or len(text) < 10:
        return "en"
    try:
        return detect(text)
    except Exception:
        return "en"

def process_batch(conn, rows):
    todo = []
    
    for r in rows:
        lang_to = normalize_lang(r.get("lang_to"), "es") or "es"
        lang_from = normalize_lang(r.get("lang_from")) or detect_lang(r.get("titulo") or "")
        
        titulo = (r.get("titulo") or "").strip()
        resumen = (r.get("resumen") or "").strip()
        
        if lang_from == lang_to:
            continue
        
        todo.append({
            "tr_id": r.get("tr_id"),
            "lang_from": lang_from,
            "lang_to": lang_to,
            "titulo": titulo,
            "resumen": resumen,
        })
    
    if not todo:
        return
    
    from collections import defaultdict
    groups = defaultdict(list)
    for item in todo:
        key = (item["lang_from"], item["lang_to"])
        groups[key].append(item)
    
    for (lang_from, lang_to), items in groups.items():
        LOG.info(f"Translating {lang_from} -> {lang_to} ({len(items)} items)")
        
        titles = [i["titulo"] for i in items]
        translated_titles = translate_texts(lang_from, lang_to, titles)
        
        translated_bodies = []
        for i in items:
            body = (i["resumen"] or "").strip()
            if body:
                tr = translate_body_long(lang_from, lang_to, body)
                translated_bodies.append(tr)
            else:
                translated_bodies.append("")
        
        cursor = conn.cursor()
        for item, tt, tb in zip(items, translated_titles, translated_bodies):
            tt = (tt or "").strip()
            tb = (tb or "").strip()
            
            if not tt:
                tt = item["titulo"]
            if not tb:
                tb = item["resumen"]
            
            try:
                cursor.execute("""
                    UPDATE traducciones 
                    SET titulo_trad = %s, resumen_trad = %s, lang_to = %s
                    WHERE id = %s
                """, (tt, tb, lang_to, item["tr_id"]))
            except Exception as e:
                LOG.error(f"Update error: {e}")
        
        conn.commit()
        cursor.close()
        LOG.info(f"Translated {len(items)} items")

def fetch_pending_translations(conn):
    cursor = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)
    
    for lang in TARGET_LANGS:
        cursor.execute("""
            SELECT t.id as tr_id, t.lang_from, t.lang_to,
                   n.titulo, n.resumen, n.id as noticia_id
            FROM traducciones t
            JOIN noticias n ON n.id = t.noticia_id
            WHERE t.lang_to = %s 
              AND (t.titulo_trad IS NULL OR t.resumen_trad IS NULL)
            ORDER BY n.fecha DESC
            LIMIT %s
        """, (lang, BATCH_SIZE))
        
        rows = cursor.fetchall()
        if rows:
            LOG.info(f"Found {len(rows)} pending translations for {lang}")
            process_batch(conn, rows)
    
    cursor.close()

def connect_db():
    return psycopg2.connect(**DB_CONFIG)

def main():
    LOG.info("Translation worker started (transformers)")
    get_translator_components()
    
    while True:
        try:
            conn = connect_db()
            fetch_pending_translations(conn)
            conn.close()
        except Exception as e:
            LOG.error(f"Error: {e}")
        
        time.sleep(30)

if __name__ == "__main__":
    main()
