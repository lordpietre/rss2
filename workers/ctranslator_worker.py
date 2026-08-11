import os
import re
import time
import logging
import fcntl
import hashlib
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

CACHE_TTL = int(os.environ.get("TRANSLATION_CACHE_TTL", str(30 * 24 * 3600)))
_redis_client = None


def get_redis():
    """Return a Redis client or None. Cache is best-effort: never blocks translation."""
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
        LOG.warning(f"Redis cache unavailable, translating without cache: {e}")
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


def clean_text(text: str) -> str:
    if not text:
        return ""
    text = re.sub(r"<[^>]+>", "", text)
    text = text.replace("<unk>", "")
    text = text.replace("&nbsp;", " ")
    text = text.replace("&amp;", "&")
    text = text.replace("&lt;", "<")
    text = text.replace("&gt;", ">")
    text = text.replace("&quot;", '"')
    text = re.sub(r"\s+", " ", text)
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
BATCH_SIZE = _env_int("TRANSLATOR_BATCH", 128)
MAX_SRC_TOKENS = _env_int("MAX_SRC_TOKENS", 256)
MAX_NEW_TOKENS = _env_int("MAX_NEW_TOKENS", 256)

CT2_MODEL_PATH = _env_str("CT2_MODEL_PATH", "/app/models/nllb-ct2")
CT2_DEVICE = _env_str("CT2_DEVICE", "cpu")
CT2_COMPUTE_TYPE = _env_str("CT2_COMPUTE_TYPE", "int8")
UNIVERSAL_MODEL = _env_str("UNIVERSAL_MODEL", "facebook/nllb-200-distilled-600M")
CT2_INTRA_THREADS = _env_int("CT2_INTRA_THREADS", 0)
CT2_INTER_THREADS = _env_int("CT2_INTER_THREADS", 1)
BODY_CHARS_CHUNK = _env_int("BODY_CHARS_CHUNK", 900)

LANG_CODE_MAP = {
    "en": "eng_Latn",
    "es": "spa_Latn",
    "fr": "fra_Latn",
    "de": "deu_Latn",
    "it": "ita_Latn",
    "pt": "por_Latn",
    "nl": "nld_Latn",
    "sv": "swe_Latn",
    "da": "dan_Latn",
    "fi": "fin_Latn",
    "no": "nob_Latn",
    "pl": "pol_Latn",
    "cs": "ces_Latn",
    "sk": "slk_Latn",
    "sl": "slv_Latn",
    "hu": "hun_Latn",
    "ro": "ron_Latn",
    "el": "ell_Grek",
    "ru": "rus_Cyrl",
    "uk": "ukr_Cyrl",
    "tr": "tur_Latn",
    "ar": "arb_Arab",
    "fa": "pes_Arab",
    "he": "heb_Hebr",
    "zh": "zho_Hans",
    "ja": "jpn_Jpan",
    "ko": "kor_Hang",
    "vi": "vie_Latn",
}

_tokenizer = None
_translator = None


def ensure_model():
    global _tokenizer, _translator

    if _translator:
        return

    model_path = CT2_MODEL_PATH
    model_bin = os.path.join(model_path, "model.bin")

    # Check if model exists AND is complete (all required files present and non-empty)
    required_files = ["model.bin", "config.json", "shared_vocabulary.json"]
    model_exists = os.path.exists(model_bin)

    if model_exists:
        # Verify all files exist and have reasonable size
        all_files_ok = True
        for f in required_files:
            fpath = os.path.join(model_path, f)
            if not os.path.exists(fpath) or os.path.getsize(fpath) < 100:
                all_files_ok = False
                break

        if not all_files_ok:
            LOG.info(f"Model files incomplete or corrupted, re-converting...")
            # Clean up corrupted files
            for f in required_files:
                try:
                    fpath = os.path.join(model_path, f)
                    if os.path.exists(fpath):
                        os.remove(fpath)
                except:
                    pass
            model_exists = False

    if not model_exists:
        LOG.info(
            f"CTranslate2 model not found at {model_path}, converting from {UNIVERSAL_MODEL}..."
        )
        convert_model()

    device = os.environ.get("CT2_DEVICE", "cpu")
    LOG.info(f"Loading CTranslate2 model from {model_path} on {device}")

    _translator = ctranslate2.Translator(
        model_path,
        device=device,
        compute_type=CT2_COMPUTE_TYPE,
        inter_threads=CT2_INTER_THREADS,
        intra_threads=CT2_INTRA_THREADS,
    )

    _tokenizer = AutoTokenizer.from_pretrained(UNIVERSAL_MODEL)
    LOG.info("CTranslate2 model loaded successfully")


def convert_model():
    import subprocess

    model_path = CT2_MODEL_PATH
    lock_file = os.path.join(model_path, ".converting.lock")

    # Clean up any corrupted files from previous failed conversions
    if os.path.exists(model_path):
        for f in os.listdir(model_path):
            if f.endswith(".lock") or f.startswith("."):
                try:
                    os.remove(os.path.join(model_path, f))
                except:
                    pass

    os.makedirs(model_path, exist_ok=True)

    # Use lock file to prevent multiple workers from converting simultaneously
    lock_fd = os.open(lock_file, os.O_CREAT | os.O_EXCL | os.O_WRONLY)
    try:
        quantization = CT2_COMPUTE_TYPE if CT2_COMPUTE_TYPE != "auto" else "float16"

        cmd = [
            "ct2-transformers-converter",
            "--model",
            UNIVERSAL_MODEL,
            "--output_dir",
            model_path,
            "--quantization",
            quantization,
            "--force",
        ]

        LOG.info(f"Running: {' '.join(cmd)}")
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=3600)

        if result.returncode != 0:
            LOG.error(f"Model conversion failed: {result.stderr}")
            raise RuntimeError("Failed to convert model")

        LOG.info("Model conversion completed")
    finally:
        os.close(lock_fd)
        try:
            os.remove(lock_file)
        except:
            pass


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
        beam_size=1,
        max_decoding_length=MAX_NEW_TOKENS,
        repetition_penalty=1.2,
        no_repeat_ngram_size=2,
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

    parts = re.split(r"(\n\n+|(?<=[\.\!\?؛؟。])\s+)", text)
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

    translated_chunks = translate_texts(src, tgt, chunks)
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
        lang_from = normalize_lang(r.get("lang_from")) or detect_lang(
            r.get("titulo") or ""
        )

        titulo = (r.get("titulo") or "").strip()
        resumen = (r.get("resumen") or "").strip()

        if lang_from == lang_to:
            # Mark as done and copy original text if languages match
            cursor = conn.cursor()
            cursor.execute(
                """
                UPDATE traducciones 
                SET titulo_trad = %s, resumen_trad = %s, status = 'done' 
                WHERE id = %s
            """,
                (titulo, resumen, r.get("tr_id")),
            )
            conn.commit()
            cursor.close()
            continue

        todo.append(
            {
                "tr_id": r.get("tr_id"),
                "lang_from": lang_from,
                "lang_to": lang_to,
                "titulo": titulo,
                "resumen": resumen,
            }
        )

    if not todo:
        return

    # 1. FAST LOCKING: Commit locked_at immediately to inform other workers
    cursor = conn.cursor()
    tr_ids = [item["tr_id"] for item in todo]
    cursor.execute(
        f"""
        UPDATE traducciones 
        SET locked_at = NOW()
        WHERE id = ANY(ARRAY[{",".join(["%s"] * len(tr_ids))}])
    """,
        tr_ids,
    )
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
            # --- TITLES (served from Redis cache when possible) ---
            titles = [i["titulo"] for i in items]
            title_keys = [cache_key(lang_from, lang_to, t) for t in titles]
            title_cache = lookup_cache(title_keys)
            if title_cache:
                LOG.info(f"Title cache hits: {len(title_cache)}/{len(titles)}")

            translated_titles = [title_cache.get(k) for k in title_keys]
            title_miss = [i for i, k in enumerate(title_keys) if k not in title_cache]
            if title_miss:
                miss_texts = [titles[i] for i in title_miss]
                miss_translated = translate_texts(lang_from, lang_to, miss_texts)
                store_cache([(title_keys[i], tr) for i, tr in zip(title_miss, miss_translated)])
                for i, tr in zip(title_miss, miss_translated):
                    translated_titles[i] = tr

            # --- BODY chunks (single batched call for cache misses) ---
            flat_chunks = []
            flat_keys = []
            flat_ck = []
            for item in items:
                body = (item["resumen"] or "").strip()
                if body:
                    chunks = split_body_into_chunks(body)
                    flat_chunks.extend(chunks)
                    flat_keys.extend([(item["tr_id"], i) for i in range(len(chunks))])
                    flat_ck.extend([cache_key(lang_from, lang_to, c) for c in chunks])

            chunk_cache = lookup_cache(flat_ck)
            if chunk_cache:
                LOG.info(f"Body cache hits: {len(chunk_cache)}/{len(flat_ck)}")

            body_parts = defaultdict(list)
            miss_idx = []
            miss_texts = []
            miss_keys = []
            for idx, ck in enumerate(flat_ck):
                tr_id, _ = flat_keys[idx]
                if ck in chunk_cache:
                    body_parts[tr_id].append(chunk_cache[ck])
                else:
                    miss_idx.append(idx)
                    miss_texts.append(flat_chunks[idx])
                    miss_keys.append(ck)

            if miss_texts:
                try:
                    miss_translated = translate_texts(lang_from, lang_to, miss_texts)
                    store_cache(list(zip(miss_keys, miss_translated)))
                except Exception as e:
                    LOG.error(f"Batch body translation error: {e}")
                    miss_translated = miss_texts
                for j, tr in enumerate(miss_translated):
                    if tr:
                        body_parts[flat_keys[miss_idx[j]][0]].append(tr)

            # --- BATCH COMMIT: single transaction per group ---
            updates = []
            for idx, item in enumerate(items):
                tt = clean_text((translated_titles[idx] or "").strip())
                parts = body_parts.get(item["tr_id"])
                tb = clean_text(" ".join(parts).strip()) if parts else ""

                if not tt:
                    tt = item["titulo"]
                if not tb:
                    tb = item["resumen"]

                updates.append((tt, tb, item["tr_id"]))

            if updates:
                try:
                    cursor = conn.cursor()
                    cursor.executemany(
                        """
                        UPDATE traducciones 
                        SET titulo_trad = %s, resumen_trad = %s, status = 'done', locked_at = NULL
                        WHERE id = %s
                    """,
                        updates,
                    )
                    conn.commit()
                    cursor.close()
                except Exception as e:
                    LOG.error(f"Batch update error (items stay pending for retry): {e}")
                    conn.rollback()

            LOG.info(f"Finished group {lang_from} -> {lang_to}")

        except Exception as e:
            LOG.error(f"Batch group error {lang_from} -> {lang_to}: {e}")
            # Mark these as error to avoid infinite loop if it's a model crash
            try:
                cursor = conn.cursor()
                cursor.execute(
                    """
                    UPDATE traducciones SET status = 'error', locked_at = NULL 
                    WHERE id = ANY(ARRAY[{','.join(['%s'] * len(items))}])
                """,
                    [i["tr_id"] for i in items],
                )
                conn.commit()
                cursor.close()
            except:
                conn.rollback()


def fetch_pending_translations(conn):
    cursor = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)

    worker_id = os.environ.get("HOSTNAME", f"worker-{os.getpid()}")
    total_found = 0

    for lang in TARGET_LANGS:
        cursor.execute(
            """
            SELECT t.id as tr_id, t.lang_from, t.lang_to,
                   n.titulo, n.resumen, n.id as noticia_id
            FROM traducciones t
            JOIN noticias n ON n.id = t.noticia_id
            WHERE t.lang_to = %s 
              AND (t.titulo_trad IS NULL OR t.resumen_trad IS NULL)
              AND (t.locked_at IS NULL OR t.locked_at < NOW() - INTERVAL '10 minutes')
            ORDER BY n.fecha ASC
            LIMIT %s
            FOR UPDATE SKIP LOCKED
        """,
            (lang, BATCH_SIZE),
        )

        rows = cursor.fetchall()
        if rows:
            LOG.info(f"Found {len(rows)} pending translations for {lang}")
            start_ts = time.time()
            process_batch(conn, rows)
            elapsed_ms = int((time.time() - start_ts) * 1000)
            try:
                metrics_cur = conn.cursor()
                metrics_cur.execute(
                    """
                    INSERT INTO translation_stats (lang_to, items_translated, elapsed_ms, hostname)
                    VALUES (%s, %s, %s, %s)
                    """,
                    (lang, len(rows), elapsed_ms, worker_id),
                )
                conn.commit()
                metrics_cur.close()
            except Exception as e:
                LOG.error(f"Error recording translation stats: {e}")
                conn.rollback()
            total_found += len(rows)

    cursor.close()
    return total_found


def connect_db():
    return psycopg2.connect(**DB_CONFIG)


def main():
    LOG.info(
        f"CTranslate2 translator worker started (device={CT2_DEVICE}, instances={TRANSLATOR_TOTAL})"
    )
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
