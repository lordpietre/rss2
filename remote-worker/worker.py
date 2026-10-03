#!/usr/bin/env python3
"""
Remote Translation Worker para RSS2.
Se conecta al backend por WebSocket y traduce noticias.

Traduce títulos, resúmenes y contenido completo.
"""

import os
import sys
import time
import json
import logging
import re
import hashlib
from typing import List, Dict, Optional, defaultdict

import websocket

import ctranslate2
from transformers import AutoTokenizer

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s: %(message)s",
    handlers=[logging.StreamHandler(sys.stdout)]
)
LOG = logging.getLogger("remote-translator")

# =============================================================================
# CONFIGURACIÓN
# =============================================================================

WORKER_NAME = os.environ.get("WORKER_NAME", "remote-worker")
WORKER_API_KEY = os.environ.get("WORKER_API_KEY", "")
WORKER_SERVER = os.environ.get("WORKER_SERVER", "ws://localhost:8080/ws/worker")

DEVICE = os.environ.get("CT2_DEVICE", "cpu")
MODEL_PATH = os.environ.get("CT2_MODEL_PATH", "/app/models/nllb-ct2")
COMPUTE_TYPE = os.environ.get("CT2_COMPUTE_TYPE", "int8")
UNIVERSAL_MODEL = os.environ.get("UNIVERSAL_MODEL", "facebook/nllb-200-distilled-600M")

# Límites
MAX_SRC_TOKENS = int(os.environ.get("MAX_SRC_TOKENS", "2048"))
MAX_NEW_TOKENS = int(os.environ.get("MAX_NEW_TOKENS", "2048"))
MAX_BODY_CHARS = int(os.environ.get("MAX_BODY_CHARS", "80000"))
BODY_CHARS_CHUNK = int(os.environ.get("BODY_CHARS_CHUNK", "2000"))

LANG_CODE_MAP = {
    "en": "eng_Latn", "es": "spa_Latn", "fr": "fra_Latn", "de": "deu_Latn",
    "it": "ita_Latn", "pt": "por_Latn", "nl": "nld_Latn", "sv": "swe_Latn",
    "da": "dan_Latn", "fi": "fin_Latn", "no": "nob_Latn", "pl": "pol_Latn",
    "cs": "ces_Latn", "sk": "slk_Latn", "sl": "slv_Latn", "hu": "hun_Latn",
    "ro": "ron_Latn", "el": "ell_Grek", "ru": "rus_Cyrl", "uk": "ukr_Cyrl",
    "tr": "tur_Latn", "ar": "arb_Arab", "fa": "pes_Arab", "he": "heb_Hebr",
    "zh": "zho_Hans", "ja": "jpn_Jpan", "ko": "kor_Hang", "vi": "vie_Latn",
}

SENTENCE_END_CHARS = set(".!?;؟؛।။॥।")

_tokenizer = None
_translator = None
_ws = None
_reconnect_delay = 5
_running = True
_stats = {"jobs_completed": 0, "jobs_failed": 0}


# =============================================================================
# MODELO
# =============================================================================

def ensure_model():
    global _tokenizer, _translator

    if _translator:
        return

    model_bin = os.path.join(MODEL_PATH, "model.bin")
    if not os.path.exists(model_bin):
        LOG.info(f"Model not found at {MODEL_PATH}, converting...")
        convert_model()

    LOG.info(f"Loading CTranslate2 model from {MODEL_PATH} on {DEVICE}")
    _translator = ctranslate2.Translator(
        MODEL_PATH, device=DEVICE, compute_type=COMPUTE_TYPE,
    )
    _tokenizer = AutoTokenizer.from_pretrained(UNIVERSAL_MODEL)
    LOG.info("Model loaded successfully")


def convert_model():
    import subprocess
    os.makedirs(MODEL_PATH, exist_ok=True)
    cmd = [
        "ct2-transformers-converter", "--model", UNIVERSAL_MODEL,
        "--output_dir", MODEL_PATH, "--quantization", COMPUTE_TYPE,
    ]
    LOG.info(f"Converting {UNIVERSAL_MODEL} to CTranslate2...")
    result = subprocess.run(cmd, capture_output=True, text=True, timeout=3600)
    if result.returncode != 0:
        LOG.error(f"Conversion failed: {result.stderr}")
        raise RuntimeError("Model conversion failed")


# =============================================================================
# TRADUCCIÓN
# =============================================================================

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
    translated = []

    for i in range(0, len(sources), 32):
        results = _translator.translate_batch(
            sources[i:i+32], target_prefix=target_prefix[i:i+32],
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
    text = (text or "").strip()
    if len(text) <= BODY_CHARS_CHUNK:
        return [text] if text else []

    parts = re.split(r"(\n\n+|(?<=[\.\!\?؛؟।။॥।])\s+)", text)
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
    body = (body or "").strip()
    if not body:
        return ""

    if len(body) > MAX_BODY_CHARS:
        body = truncate_at_sentence_boundary(body, MAX_BODY_CHARS)

    chunks = split_into_chunks(body)
    if len(chunks) == 1:
        return translate_texts(src, tgt, [body])[0]

    translated_chunks = []
    for ch in chunks:
        tr = translate_texts(src, tgt, [ch])[0]
        translated_chunks.append(tr)

    return join_chunks(translated_chunks)


def join_chunks(chunks: List[str]) -> str:
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
# PROCESAMIENTO DE JOBS
# =============================================================================

def process_job(job: dict) -> dict:
    job_id = job.get("id")
    lang_from = job.get("lang_from", "en")
    lang_to = job.get("lang_to", "es")
    title = job.get("title", "")
    summary = job.get("summary", "")
    content = job.get("content", "")

    LOG.info(f"Processing job {job_id}: {lang_from} -> {lang_to}")

    if lang_from == lang_to:
        return {
            "job_id": job_id,
            "title_trad": title,
            "summary_trad": summary,
            "contenido_trad": content,
        }

    try:
        title_tr = translate_texts(lang_from, lang_to, [title])[0] if title else title
        summary_tr = translate_long_text(lang_from, lang_to, summary) if summary else summary
        content_tr = translate_long_text(lang_from, lang_to, content) if content else content

        return {
            "job_id": job_id,
            "title_trad": title_tr,
            "summary_trad": summary_tr,
            "contenido_trad": content_tr,
        }
    except Exception as e:
        LOG.error(f"Job {job_id} failed: {e}")
        return {"job_id": job_id, "error": str(e)}


# =============================================================================
# WEBSOCKET
# =============================================================================

def connect_ws():
    global _ws, _reconnect_delay

    try:
        ws_url = f"{WORKER_SERVER}?api_key={WORKER_API_KEY}"
        _ws = websocket.WebSocketApp(
            ws_url,
            on_open=on_open,
            on_message=on_message,
            on_error=on_error,
            on_close=on_close,
        )
        LOG.info(f"Connecting to {ws_url}")
        _ws.run_forever()
    except Exception as e:
        LOG.error(f"WebSocket error: {e}")

    _reconnect_delay = min(_reconnect_delay * 2, 60)
    LOG.info(f"Reconnecting in {_reconnect_delay}s...")
    time.sleep(_reconnect_delay)
    _reconnect_delay = 5
    connect_ws()


def on_open(ws):
    LOG.info("WebSocket connected")
    register()


def on_message(ws, message):
    global _running
    try:
        msg = json.loads(message)
        msg_type = msg.get("type")

        if msg_type == "ping":
            ws.send(json.dumps({"type": "pong"}))
        elif msg_type == "ack":
            LOG.debug(f"Job ack: {msg.get('job_id')}")
        elif msg_type == "error":
            LOG.error(f"Server error: {msg.get('error')}")
        elif msg_type == "job":
            job = msg.get("job", {})
            result = process_job(job)
            ws.send(json.dumps({"type": "result", **result}))
            _stats["jobs_completed"] += 1
        elif msg_type == "stop":
            LOG.info("Received stop command")
            _running = False
    except Exception as e:
        LOG.error(f"Error processing message: {e}")


def on_error(ws, error):
    LOG.error(f"WebSocket error: {error}")


def on_close(ws, close_status_code, close_msg):
    LOG.info(f"WebSocket closed: {close_status_code} - {close_msg}")


def register():
    if _ws:
        _ws.send(json.dumps({
            "type": "register",
            "worker_name": WORKER_NAME,
            "capabilities": "cpu",
        }))
        LOG.info(f"Registered as {WORKER_NAME}")


# =============================================================================
# MAIN
# =============================================================================

def main():
    LOG.info(f"Starting remote translation worker: {WORKER_NAME}")
    ensure_model()
    connect_ws()


if __name__ == "__main__":
    main()
