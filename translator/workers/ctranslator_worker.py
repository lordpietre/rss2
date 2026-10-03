#!/usr/bin/env python3
"""
CTranslate2 Translation Worker para RSS2.

Traduce títulos, resúmenes y contenido completo de noticias.
Soporta ejecución local (docker-compose) y remota (WebSocket).

Límites configurables via environment:
- MAX_SRC_TOKENS: Tokens máximos del texto origen (default: 2048)
- MAX_NEW_TOKENS: Tokens máximos de la traducción (default: 2048)
- MAX_BODY_CHARS: Caracteres máximos del cuerpo a traducir (default: 80000)
- BODY_CHARS_CHUNK: Tamaño de chunks para contenido largo (default: 2000)
"""

import os
import re
import time
import logging
import hashlib
from typing import List, defaultdict

import psycopg2
import psycopg2.extras
from langdetect import detect, DetectorFactory

import ctranslate2
from transformers import AutoTokenizer

DetectorFactory.seed = 0

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s: %(message)s")
LOG = logging.getLogger("translator_ct2")

# =============================================================================
# CONFIGURACIÓN
# =============================================================================

TRANSLATOR_ID = os.environ.get("TRANSLATOR_ID", "")
TRANSLATOR_TOTAL = int(os.environ.get("TRANSLATOR_TOTAL", "1"))
CACHE_TTL = int(os.environ.get("TRANSLATION_CACHE_TTL", str(30 * 24 * 3600)))

# Límites de traducción - aumentados para contenido completo
MAX_SRC_TOKENS = int(os.environ.get("MAX_SRC_TOKENS", "2048"))
MAX_NEW_TOKENS = int(os.environ.get("MAX_NEW_TOKENS", "2048"))
MAX_BODY_CHARS = int(os.environ.get("MAX_BODY_CHARS", "80000"))  # 80k para artículos completos
BODY_CHARS_CHUNK = int(os.environ.get("BODY_CHARS_CHUNK", "2000"))  # 2k por chunk

# CTranslate2
CT2_MODEL_PATH = os.environ.get("CT2_MODEL_PATH", "/app/models/nllb-ct2")
CT2_DEVICE = os.environ.get("CT2_DEVICE", "cpu")
CT2_COMPUTE_TYPE = os.environ.get("CT2_COMPUTE_TYPE", "int8")
CT2_INTRA_THREADS = int(os.environ.get("CT2_INTRA_THREADS", "2"))
CT2_INTER_THREADS = int(os.environ.get("CT2_INTER_THREADS", "1"))
MAX_SEQ_PER_CALL = int(os.environ.get("MAX_SEQ_PER_CALL", "32"))

UNIVERSAL_MODEL = os.environ.get("UNIVERSAL_MODEL", "facebook/nllb-200-distilled-600M")

# Base de datos
DB_CONFIG = {
    "host": os.environ.get("DB_HOST", "localhost"),
    "port": int(os.environ.get("DB_PORT", 5432)),
    "dbname": os.environ.get("DB_NAME", "rss"),
    "user": os.environ.get("DB_USER", "rss"),
    "password": os.environ.get("DB_PASS", "x"),
}

# Idiomas soportados
LANG_CODE_MAP = {
    "en": "eng_Latn", "es": "spa_Latn", "fr": "fra_Latn", "de": "deu_Latn",
    "it": "ita_Latn", "pt": "por_Latn", "nl": "nld_Latn", "sv": "swe_Latn",
    "da": "dan_Latn", "fi": "fin_Latn", "no": "nob_Latn", "pl": "pol_Latn",
    "cs": "ces_Latn", "sk": "slk_Latn", "sl": "slv_Latn", "hu": "hun_Latn",
    "ro": "ron_Latn", "el": "ell_Grek", "ru": "rus_Cyrl", "uk": "ukr_Cyrl",
    "tr": "tur_Latn", "ar": "arb_Arab", "fa": "pes_Arab", "he": "heb_Hebr",
    "zh": "zho_Hans", "ja": "jpn_Jpan", "ko": "kor_Hang", "vi": "vie_Latn",
}

# Puntuación de fin de oración
SENTENCE_END_CHARS = set(".!?;؟؛।။॥।")

# =============================================================================
# REDIS CACHE
# =============================================================================

_redis_client = None


def get_redis():
    global _redis_client
    if _redis_client is not None:
        return _redis_client if _redis_client is not False else None
    url = os.environ.get("REDIS_URL", "redis://localhost:6379")
    try:
        import redis
        client = redis.Redis.from_url(
            url, decode_responses=True,
            socket_connect_timeout=2, socket_timeout=2,
        )
        client.ping()
        _redis_client = client
        LOG.info("Redis translation cache enabled")
        return client
    except Exception as e:
        LOG.warning(f"Redis cache unavailable: {e}")
        _redis_client = False
        return None


def cache_key(lang_from: str, lang_to: str, text: str) -> str:
    digest = hashlib.md5((text or "").encode("utf-8", "ignore")).hexdigest()
    return f"tr:{lang_from}:{lang_to}:{digest}"


def lookup_cache(keys: List[str]) -> dict:
    r = get_redis()
    if not r or not keys:
        return {}
    try:
        vals = r.mget(keys)
        return {k: v for k, v in zip(keys, vals) if v}
    except Exception as e:
        LOG.warning(f"Redis mget error: {e}")
        return {}


def store_cache(pairs, ttl: int = CACHE_TTL):
    r = get_redis()
    if not r or not pairs:
        return
    try:
        pipe = r.pipeline(transaction=False)
        for k, v in pairs:
            if v:
                pipe.setex(k, ttl, v)
        pipe.execute()
    except Exception as e:
        LOG.warning(f"Redis cache store error: {e}")


# =============================================================================
# LIMPIEZA DE TEXTO
# =============================================================================

def clean_text(text: str) -> str:
    if not text:
        return ""
    text = re.sub(r"<[^>]+>", "", text)
    text = text.replace("<unk>", "").replace("&nbsp;", " ")
    text = text.replace("&amp;", "&").replace("&lt;", "<").replace("&gt;", ">")
    text = text.replace("&quot;", '"')
    text = re.sub(r"\s+", " ", text)
    return text.strip()


# =============================================================================
# MODELO CTRANSLATE2
# =============================================================================

_tokenizer = None
_translator = None


def ensure_model():
    global _tokenizer, _translator
    if _translator:
        return

    model_bin = os.path.join(CT2_MODEL_PATH, "model.bin")
    if not os.path.exists(model_bin):
        LOG.info(f"Model not found at {CT2_MODEL_PATH}, converting...")
        convert_model()

    LOG.info(f"Loading CTranslate2 model from {CT2_MODEL_PATH} on {CT2_DEVICE}")
    _translator = ctranslate2.Translator(
        CT2_MODEL_PATH, device=CT2_DEVICE, compute_type=CT2_COMPUTE_TYPE,
        inter_threads=CT2_INTER_THREADS, intra_threads=CT2_INTRA_THREADS,
    )
    _tokenizer = AutoTokenizer.from_pretrained(UNIVERSAL_MODEL)
    LOG.info("CTranslate2 model loaded successfully")


def convert_model():
    import subprocess
    os.makedirs(CT2_MODEL_PATH, exist_ok=True)
    cmd = [
        "ct2-transformers-converter", "--model", UNIVERSAL_MODEL,
        "--output_dir", CT2_MODEL_PATH, "--quantization", CT2_COMPUTE_TYPE, "--force",
    ]
    LOG.info(f"Running: {' '.join(cmd)}")
    result = subprocess.run(cmd, capture_output=True, text=True, timeout=3600)
    if result.returncode != 0:
        LOG.error(f"Model conversion failed: {result.stderr}")
        raise RuntimeError("Failed to convert model")


# =============================================================================
# TRADUCCIÓN
# =============================================================================

def translate_texts(src: str, tgt: str, texts: List[str]) -> List[str]:
    """Traduce una lista de textos."""
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
    translated = []

    for i in range(0, len(sources), MAX_SEQ_PER_CALL):
        results = _translator.translate_batch(
            sources[i:i + MAX_SEQ_PER_CALL],
            target_prefix=target_prefix[i:i + MAX_SEQ_PER_CALL],
            beam_size=1, max_decoding_length=MAX_NEW_TOKENS,
            repetition_penalty=1.2, no_repeat_ngram_size=2,
        )
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
            except Exception as e:
                LOG.error(f"Error processing result: {e}")
                translated.append("")
    return translated


# =============================================================================
# TRUNCAMIENTO Y CHUNKING
# =============================================================================

def truncate_at_sentence_boundary(text: str, max_len: int) -> str:
    """Trunca texto en frontera de oración."""
    if len(text) <= max_len:
        return text
    search_end = min(len(text), max_len)
    for i in range(search_end - 1, max(0, search_end - 200), -1):
        if text[i] in SENTENCE_END_CHARS:
            return text[:i + 1]
    cutoff = int(max_len * 0.9)
    last_space = text.rfind(" ", cutoff, max_len)
    if last_space > 0:
        return text[:last_space]
    return text[:max_len]


def split_into_chunks(text: str) -> List[str]:
    """Divide texto en chunks por fronteras de oración."""
    text = (text or "").strip()
    if len(text) <= BODY_CHARS_CHUNK:
        return [text] if text else []

    parts = re.split(r"(\n\n+|(?<=[\.\!\?؛؟।။॥。])\s+)", text)
    chunks, current = [], ""

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


def translate_long_text(src: str, tgt: str, body: str) -> str:
    """Traduce texto largo dividiendo en chunks."""
    body = (body or "").strip()
    if not body:
        return ""

    # Truncar si es muy largo
    if len(body) > MAX_BODY_CHARS:
        body = truncate_at_sentence_boundary(body, MAX_BODY_CHARS)

    chunks = split_into_chunks(body)
    if len(chunks) == 1:
        return translate_texts(src, tgt, [body])[0]

    translated_chunks = translate_texts(src, tgt, chunks)
    return join_chunks(translated_chunks)


def join_chunks(chunks: List[str]) -> str:
    """Une chunks traducidos."""
    if not chunks:
        return ""
    if len(chunks) == 1:
        return chunks[0]

    result = chunks[0]
    for chunk in chunks[1:]:
        if not chunk:
            continue
        starts_lower = chunk and chunk[0].islower()
        prev_ends_lower = result and result[-1].islower() if result else False

        if prev_ends_lower and starts_lower:
            result += " " + chunk
        elif result and result[-1] in SENTENCE_END_CHARS:
            result += " " + chunk
        else:
            result += " " + chunk
    return result


# =============================================================================
# UTILIDADES
# =============================================================================

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


# =============================================================================
# PROCESAMIENTO DE BATCH
# =============================================================================

def process_batch(conn, rows):
    """Procesa un batch de traducciones."""
    todo = []

    for r in rows:
        lang_to = normalize_lang(r.get("lang_to"), "es") or "es"
        lang_from = normalize_lang(r.get("lang_from"), default=None) or detect_lang(
            f"{r.get('titulo') or ''} {r.get('resumen') or ''} {r.get('contenido') or ''}"[:1000]
        ) or "es"

        titulo = (r.get("titulo") or "").strip()
        resumen = (r.get("resumen") or "").strip()
        contenido = (r.get("contenido") or "").strip()

        if lang_from == lang_to:
            cursor = conn.cursor()
            cursor.execute("""
                UPDATE traducciones 
                SET titulo_trad = %s, resumen_trad = %s, contenido_trad = %s, status = 'done' 
                WHERE id = %s
            """, (titulo, resumen, contenido, r.get("tr_id")))
            conn.commit()
            cursor.close()
            continue

        todo.append({
            "tr_id": r.get("tr_id"),
            "lang_from": lang_from,
            "lang_to": lang_to,
            "titulo": titulo,
            "resumen": resumen,
            "contenido": contenido,
        })

    if not todo:
        return

    # Bloquear registros
    cursor = conn.cursor()
    tr_ids = [item["tr_id"] for item in todo]
    cursor.execute(f"""
        UPDATE traducciones SET locked_at = NOW()
        WHERE id = ANY(ARRAY[{",".join(["%s"] * len(tr_ids))}])
    """, tr_ids)
    conn.commit()
    cursor.close()

    # Agrupar por idioma
    groups = defaultdict(list)
    for item in todo:
        key = (item["lang_from"], item["lang_to"])
        groups[key].append(item)

    for (lang_from, lang_to), items in groups.items():
        LOG.info(f"Translating {lang_from} -> {lang_to} ({len(items)} items)")
        process_group(conn, lang_from, lang_to, items)


def process_group(conn, lang_from: str, lang_to: str, items: List[dict]):
    """Procesa un grupo de items del mismo par de idiomas."""
    try:
        # --- TÍTULOS ---
        titles = [i["titulo"] for i in items]
        title_keys = [cache_key(lang_from, lang_to, t) for t in titles]
        title_cache = lookup_cache(title_keys)

        translated_titles = [title_cache.get(k) for k in title_keys]
        title_miss = [i for i, k in enumerate(title_keys) if k not in title_cache]

        if title_miss:
            miss_texts = [titles[i] for i in title_miss]
            miss_translated = translate_texts(lang_from, lang_to, miss_texts)
            store_cache([(title_keys[i], tr) for i, tr in zip(title_miss, miss_translated)])
            for i, tr in zip(title_miss, miss_translated):
                translated_titles[i] = tr

        # --- RESÚMENES ---
        resumen_parts = translate_field(items, "resumen", lang_from, lang_to)

        # --- CONTENIDOS ---
        contenido_parts = translate_field(items, "contenido", lang_from, lang_to)

        # --- GUARDAR ---
        updates = []
        for idx, item in enumerate(items):
            tt = clean_text((translated_titles[idx] or "").strip())
            rp = resumen_parts.get(item["tr_id"])
            tb = clean_text(" ".join(rp).strip()) if rp else ""
            cp = contenido_parts.get(item["tr_id"])
            tc = clean_text(" ".join(cp).strip()) if cp else ""

            if not tt:
                tt = item["titulo"]
            if not tb:
                tb = item["resumen"]

            updates.append((tt, tb, tc, item["tr_id"]))

        if updates:
            cursor = conn.cursor()
            cursor.executemany("""
                UPDATE traducciones 
                SET titulo_trad = %s, resumen_trad = %s, contenido_trad = %s, 
                    status = 'done', locked_at = NULL
                WHERE id = %s
            """, updates)
            conn.commit()
            cursor.close()

        LOG.info(f"Finished group {lang_from} -> {lang_to}")

    except Exception as e:
        LOG.error(f"Batch group error {lang_from} -> {lang_to}: {e}")
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


def translate_field(items: List[dict], field: str, lang_from: str, lang_to: str) -> dict:
    """Traduce un campo (resumen o contenido) para todos los items."""
    flat_chunks, flat_keys, flat_ck = [], [], []
    item_chunks = {}

    for item in items:
        body = (item[field] or "").strip()
        if len(body) > MAX_BODY_CHARS:
            body = truncate_at_sentence_boundary(body, MAX_BODY_CHARS)
        if body:
            chunks = split_into_chunks(body)
            flat_chunks.extend(chunks)
            flat_keys.extend([(item["tr_id"], i) for i in range(len(chunks))])
            flat_ck.extend([cache_key(lang_from, lang_to, c) for c in chunks])
            item_chunks[item["tr_id"]] = chunks

    if not flat_chunks:
        return {}

    chunk_cache = lookup_cache(flat_ck)

    result_parts = defaultdict(list)
    miss_idx, miss_texts, miss_keys = [], [], []

    for idx, ck in enumerate(flat_ck):
        tr_id, _ = flat_keys[idx]
        if ck in chunk_cache:
            result_parts[tr_id].append(chunk_cache[ck])
        else:
            miss_idx.append(idx)
            miss_texts.append(flat_chunks[idx])
            miss_keys.append(ck)

    if miss_texts:
        try:
            miss_translated = translate_texts(lang_from, lang_to, miss_texts)
            store_cache(list(zip(miss_keys, miss_translated)))
        except Exception as e:
            LOG.error(f"Batch {field} translation error: {e}")
            miss_translated = miss_texts
        for j, tr in enumerate(miss_translated):
            if tr:
                result_parts[flat_keys[miss_idx[j]][0]].append(tr)

    return result_parts


# =============================================================================
# FETCH Y MAIN
# =============================================================================

def fetch_pending_translations(conn):
    """Obtiene traducciones pendientes y las procesa."""
    cursor = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)
    worker_id = os.environ.get("HOSTNAME", f"worker-{os.getpid()}")
    total_found = 0

    target_langs = os.environ.get("TARGET_LANGS", "es").split(",")

    for lang in target_langs:
        lang = lang.strip()
        if not lang:
            continue

        cursor.execute("""
            SELECT t.id as tr_id, t.lang_from, t.lang_to,
                   n.titulo, n.resumen, n.contenido, n.id as noticia_id
            FROM traducciones t
            JOIN noticias n ON n.id = t.noticia_id
            WHERE t.lang_to = %s
              AND t.status = 'pending'
              AND t.worker_id IS NULL
              AND (t.titulo_trad IS NULL OR t.resumen_trad IS NULL OR t.contenido_trad IS NULL)
              AND (t.locked_at IS NULL OR t.locked_at < NOW() - INTERVAL '10 minutes')
            ORDER BY n.fecha DESC
            LIMIT 128
            FOR UPDATE SKIP LOCKED
        """, (lang,))

        rows = cursor.fetchall()
        if rows:
            LOG.info(f"Found {len(rows)} pending translations for {lang}")
            start_ts = time.time()
            process_batch(conn, rows)
            elapsed_ms = int((time.time() - start_ts) * 1000)

            try:
                metrics_cur = conn.cursor()
                metrics_cur.execute("""
                    INSERT INTO translation_stats (lang_to, items_translated, elapsed_ms, hostname)
                    VALUES (%s, %s, %s, %s)
                """, (lang, len(rows), elapsed_ms, worker_id))
                conn.commit()
                metrics_cur.close()
            except Exception as e:
                LOG.error(f"Error recording stats: {e}")
                conn.rollback()

            total_found += len(rows)

    cursor.close()
    return total_found


def connect_db():
    return psycopg2.connect(**DB_CONFIG)


def main():
    LOG.info(f"CTranslate2 translator worker started (id={TRANSLATOR_ID}, total={TRANSLATOR_TOTAL})")
    ensure_model()

    while True:
        try:
            conn = connect_db()
            total = fetch_pending_translations(conn)
            conn.close()

            if total == 0:
                LOG.info("No pending translations, sleeping...")
            else:
                LOG.info(f"Processed {total} translations, sleeping...")
        except Exception as e:
            LOG.error(f"Error: {e}")

        time.sleep(30)


if __name__ == "__main__":
    main()
