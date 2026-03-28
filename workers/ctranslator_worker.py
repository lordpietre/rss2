import os
import time
import logging
import re
from typing import List, Optional

import psycopg2
import psycopg2.extras
from langdetect import detect, DetectorFactory

import ctranslate2
from transformers import AutoTokenizer

DetectorFactory.seed = 0

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s: %(message)s")
LOG = logging.getLogger("translator_ct2")

TRANSLATOR_ID = os.environ.get("TRANSLATOR_ID", "")
TRANSLATOR_TOTAL = int(os.environ.get("TRANSLATOR_TOTAL", "1"))

def clean_text(text: str) -> str:
    if not text:
        return ""
    text = re.sub(r'<[^>]+>', '', text)
    text = text.replace('<unk>', '')
    text = text.replace('&nbsp;', ' ')
    text = text.replace('&amp;', '&')
    text = text.replace('&lt;', '<')
    text = text.replace('&gt;', '>')
    text = text.replace('&quot;', '"')
    text = re.sub(r'\s+', ' ', text)
    return text.strip()

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

def _env_str(name: str, default=None):
    v = os.environ.get(name)
    return v if v else default

TARGET_LANGS = _env_list("TARGET_LANGS")
BATCH_SIZE = _env_int("TRANSLATOR_BATCH", 8)
MAX_SRC_TOKENS = _env_int("MAX_SRC_TOKENS", 512)
MAX_NEW_TOKENS = _env_int("MAX_NEW_TOKENS", 512)

CT2_MODEL_PATH = _env_str("CT2_MODEL_PATH", "/app/models/nllb-ct2")
CT2_DEVICE = _env_str("CT2_DEVICE", "cpu")
CT2_COMPUTE_TYPE = _env_str("CT2_COMPUTE_TYPE", "int8")
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

def ensure_model():
    global _tokenizer, _translator
    
    if _translator:
        return
    
    model_path = CT2_MODEL_PATH
    model_bin = os.path.join(model_path, "model.bin")
    
    if not os.path.exists(model_bin):
        LOG.info(f"CTranslate2 model not found at {model_path}, converting from {UNIVERSAL_MODEL}...")
        convert_model()
    
    LOG.info(f"Loading CTranslate2 model from {model_path} on {CT2_DEVICE}")
    
    _translator = ctranslate2.Translator(
        model_path,
        device=CT2_DEVICE,
        compute_type=CT2_COMPUTE_TYPE,
    )
    
    _tokenizer = AutoTokenizer.from_pretrained(UNIVERSAL_MODEL)
    LOG.info("CTranslate2 model loaded successfully")

def convert_model():
    import subprocess
    
    model_path = CT2_MODEL_PATH
    os.makedirs(model_path, exist_ok=True)
    
    quantization = CT2_COMPUTE_TYPE if CT2_COMPUTE_TYPE != "auto" else "int8"
    
    cmd = [
        "ct2-transformers-converter",
        "--model", UNIVERSAL_MODEL,
        "--output_dir", model_path,
        "--quantization", quantization,
        "--force"
    ]
    
    LOG.info(f"Running: {' '.join(cmd)}")
    result = subprocess.run(cmd, capture_output=True, text=True, timeout=1800)
    
    if result.returncode != 0:
        LOG.error(f"Model conversion failed: {result.stderr}")
        raise RuntimeError("Failed to convert model")
    
    LOG.info("Model conversion completed")

def translate_texts(src: str, tgt: str, texts: List[str]) -> List[str]:
    if not texts:
        return []
    
    ensure_model()
    
    clean = [(t or "").strip() for t in texts]
    if all(not t for t in clean):
        return ["" for _ in clean]
    
    src_code = LANG_CODE_MAP.get(src, f"{src}_Latn")
    tgt_code = LANG_CODE_MAP.get(tgt, "spa_Latn")
    
    try:
        _tokenizer.src_lang = src_code
    except Exception:
        pass
    
    sources = []
    for t in clean:
        if t:
            ids = _tokenizer.encode(t, truncation=True, max_length=MAX_SRC_TOKENS)
            tokens = _tokenizer.convert_ids_to_tokens(ids)
            sources.append(tokens)
        else:
            sources.append([])
    
    target_prefix = [[tgt_code]] * len(sources)
    
    results = _translator.translate_batch(
        sources,
        target_prefix=target_prefix,
        beam_size=2,
        max_decoding_length=MAX_NEW_TOKENS,
        repetition_penalty=2.0,
        no_repeat_ngram_size=3,
    )
    
    translated = []
    for result in results:
        try:
            if result.hypotheses and len(result.hypotheses) > 0:
                hyp = result.hypotheses[0]
                if isinstance(hyp, list) and len(hyp) > 0:
                    first_hyp = hyp[0]
                    if isinstance(first_hyp, dict) and "token_ids" in first_hyp:
                        tokens = first_hyp["token_ids"]
                        text = _tokenizer.decode(tokens)
                        translated.append(text.strip())
                    elif isinstance(first_hyp, str):
                        token_strings = hyp[1:] if len(hyp) > 1 else []
                        if token_strings:
                            text = _tokenizer.convert_tokens_to_string(token_strings)
                            translated.append(text.strip())
                        else:
                            translated.append("")
                    else:
                        translated.append("")
                else:
                    translated.append("")
            else:
                translated.append("")
        except Exception as e:
            LOG.error(f"Error processing result: {e}")
            translated.append("")
    
    return translated

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
        return translate_texts(src, tgt, [body])[0]
    
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
            # Mark as done and copy original text if languages match
            cursor = conn.cursor()
            cursor.execute("""
                UPDATE traducciones 
                SET titulo_trad = %s, resumen_trad = %s, status = 'done' 
                WHERE id = %s
            """, (titulo, resumen, r.get("tr_id")))
            conn.commit()
            cursor.close()
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
    
    # 1. FAST LOCKING: Commit locked_at immediately to inform other workers
    cursor = conn.cursor()
    tr_ids = [item["tr_id"] for item in todo]
    cursor.execute(f"""
        UPDATE traducciones 
        SET locked_at = NOW()
        WHERE id = ANY(ARRAY[{','.join(['%s'] * len(tr_ids))}])
    """, tr_ids)
    conn.commit()
    cursor.close()
    
    from collections import defaultdict
    groups = defaultdict(list)
    for item in todo:
        key = (item["lang_from"], item["lang_to"])
        groups[key].append(item)
    
    for (lang_from, lang_to), items in groups.items():
        LOG.info(f"Translating {lang_from} -> {lang_to} ({len(items)} items)")
        
        try:
            titles = [i["titulo"] for i in items]
            translated_titles = translate_texts(lang_from, lang_to, titles)
            
            for item, tt in zip(items, translated_titles):
                body = (item["resumen"] or "").strip()
                tb = ""
                if body:
                    try:
                        tb = translate_body_long(lang_from, lang_to, body)
                    except Exception as e:
                        LOG.error(f"Body translation error for ID {item['tr_id']}: {e}")
                        tb = item["resumen"]
                
                tt = clean_text((tt or "").strip())
                tb = clean_text((tb or "").strip())
                
                if not tt:
                    tt = item["titulo"]
                if not tb:
                    tb = item["resumen"]
                
                # 2. INDIVIDUAL COMMIT: Save each item as it's done
                try:
                    cursor = conn.cursor()
                    cursor.execute("""
                        UPDATE traducciones 
                        SET titulo_trad = %s, resumen_trad = %s, status = 'done', locked_at = NULL
                        WHERE id = %s
                    """, (tt, tb, item["tr_id"]))
                    conn.commit()
                    cursor.close()
                except Exception as e:
                    LOG.error(f"Update error for ID {item['tr_id']}: {e}")
                    conn.rollback()
            
            LOG.info(f"Finished group {lang_from} -> {lang_to}")
            
        except Exception as e:
            LOG.error(f"Batch group error {lang_from} -> {lang_to}: {e}")
            # Mark these as error to avoid infinite loop if it's a model crash
            try:
                cursor = conn.cursor()
                cursor.execute("""
                    UPDATE traducciones SET status = 'error', locked_at = NULL 
                    WHERE id = ANY(ARRAY[{','.join(['%s'] * len(items))}])
                """, [i["tr_id"] for i in items])
                conn.commit()
                cursor.close()
            except:
                conn.rollback()

def fetch_pending_translations(conn):
    cursor = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)
    
    worker_id = os.environ.get("HOSTNAME", f"worker-{os.getpid()}")
    
    for lang in TARGET_LANGS:
        cursor.execute("""
            SELECT t.id as tr_id, t.lang_from, t.lang_to,
                   n.titulo, n.resumen, n.id as noticia_id
            FROM traducciones t
            JOIN noticias n ON n.id = t.noticia_id
            WHERE t.lang_to = %s 
              AND (t.titulo_trad IS NULL OR t.resumen_trad IS NULL)
              AND (t.locked_at IS NULL OR t.locked_at < NOW() - INTERVAL '10 minutes')
            ORDER BY n.fecha DESC
            LIMIT %s
            FOR UPDATE SKIP LOCKED
        """, (lang, BATCH_SIZE))
        
        rows = cursor.fetchall()
        if rows:
            LOG.info(f"Found {len(rows)} pending translations for {lang}")
            process_batch(conn, rows)
    
    cursor.close()

def connect_db():
    return psycopg2.connect(**DB_CONFIG)

def main():
    LOG.info(f"CTranslate2 translator worker started (device={CT2_DEVICE}, instances={TRANSLATOR_TOTAL})")
    ensure_model()
    
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
