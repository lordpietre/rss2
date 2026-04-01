import os
import sys
import time
import json
import logging
import re
import threading
from typing import List, Optional

import websocket

import ctranslate2
from transformers import AutoTokenizer

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s: %(message)s",
    handlers=[logging.StreamHandler(sys.stdout)]
)
LOG = logging.getLogger("remote-translator")

WORKER_NAME = os.environ.get("WORKER_NAME", "remote-worker")
WORKER_API_KEY = os.environ.get("WORKER_API_KEY", "")
WORKER_SERVER = os.environ.get("WORKER_SERVER", "ws://localhost:8080/ws/worker")
DEVICE = os.environ.get("CT2_DEVICE", "cpu")
MODEL_PATH = os.environ.get("CT2_MODEL_PATH", "/app/models/nllb-ct2")
COMPUTE_TYPE = os.environ.get("CT2_COMPUTE_TYPE", "int8")
UNIVERSAL_MODEL = os.environ.get("UNIVERSAL_MODEL", "facebook/nllb-200-distilled-600M")

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

MAX_SRC_TOKENS = 512
MAX_NEW_TOKENS = 512
BODY_CHARS_CHUNK = 900

_tokenizer = None
_translator = None
_ws = None
_reconnect_delay = 5
_running = True
_stats = {"jobs_completed": 0, "jobs_failed": 0}


def ensure_model():
    global _tokenizer, _translator
    
    if _translator:
        return
    
    model_bin = os.path.join(MODEL_PATH, "model.bin")
    
    if not os.path.exists(model_bin):
        LOG.info(f"CTranslate2 model not found at {MODEL_PATH}, converting...")
        convert_model()
    
    LOG.info(f"Loading CTranslate2 model from {MODEL_PATH} on {DEVICE}")
    
    _translator = ctranslate2.Translator(
        MODEL_PATH,
        device=DEVICE,
        compute_type=COMPUTE_TYPE,
    )
    
    _tokenizer = AutoTokenizer.from_pretrained(UNIVERSAL_MODEL)
    LOG.info("CTranslate2 model loaded successfully")


def convert_model():
    import subprocess
    
    os.makedirs(MODEL_PATH, exist_ok=True)
    
    quantization = COMPUTE_TYPE if COMPUTE_TYPE != "auto" else "int8"
    
    cmd = [
        "ct2-transformers-converter",
        "--model", UNIVERSAL_MODEL,
        "--output_dir", MODEL_PATH,
        "--quantization", quantization,
        "--force"
    ]
    
    LOG.info(f"Running: {' '.join(cmd)}")
    result = subprocess.run(cmd, capture_output=True, text=True, timeout=1800)
    
    if result.returncode != 0:
        LOG.error(f"Model conversion failed: {result.stderr}")
        raise RuntimeError("Failed to convert model")
    
    LOG.info("Model conversion completed")


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


def process_job(job: dict) -> dict:
    job_id = job.get("id")
    lang_from = job.get("lang_from", "en")
    lang_to = job.get("lang_to", "es")
    title = job.get("title", "")
    summary = job.get("summary", "")
    
    LOG.info(f"Processing job {job_id}: {lang_from} -> {lang_to}")
    
    if lang_from == lang_to:
        return {
            "job_id": job_id,
            "title_trad": title,
            "summary_trad": summary,
            "error": ""
        }
    
    try:
        title_tr = translate_texts(lang_from, lang_to, [title])[0]
        title_tr = clean_text(title_tr) or title
        
        summary_tr = ""
        if summary:
            summary_tr = translate_body_long(lang_from, lang_to, summary)
            summary_tr = clean_text(summary_tr) or summary
        
        _stats["jobs_completed"] += 1
        
        return {
            "job_id": job_id,
            "title_trad": title_tr,
            "summary_trad": summary_tr,
            "error": ""
        }
    except Exception as e:
        LOG.error(f"Translation error for job {job_id}: {e}")
        _stats["jobs_failed"] += 1
        return {
            "job_id": job_id,
            "title_trad": "",
            "summary_trad": "",
            "error": str(e)
        }


def send_message(msg: dict):
    if _ws and _ws.sock and _ws.sock.connected:
        try:
            _ws.send(json.dumps(msg))
        except Exception as e:
            LOG.error(f"Send error: {e}")


def on_message(ws, message):
    try:
        msg = json.loads(message)
    except:
        LOG.error(f"Invalid JSON: {message}")
        return
    
    msg_type = msg.get("type")
    
    if msg_type == "job":
        job = msg.get("job", {})
        result = process_job(job)
        send_message({
            "type": "result",
            "result": result
        })
        LOG.info(f"Sent result for job {result['job_id']}")
    
    elif msg_type == "ping":
        send_message({"type": "heartbeat"})
    
    elif msg_type == "ack":
        LOG.debug(f"Server ACK: {msg.get('message', '')}")
    
    elif msg_type == "error":
        LOG.error(f"Server error: {msg.get('message', '')}")


def on_error(ws, error):
    LOG.error(f"WebSocket error: {error}")


def on_close(ws, close_status_code, close_msg):
    global _reconnect_delay
    LOG.warning(f"WebSocket closed: {close_status_code} - {close_msg}")
    LOG.info(f"Reconnecting in {_reconnect_delay}s...")


def on_open(ws):
    global _reconnect_delay
    LOG.info("Connected to server")
    _reconnect_delay = 5
    
    send_message({
        "type": "register",
        "capabilities": DEVICE,
        "worker_name": WORKER_NAME
    })


def stats_reporter():
    while _running:
        time.sleep(60)
        LOG.info(f"Stats: completed={_stats['jobs_completed']}, failed={_stats['jobs_failed']}")


def connect():
    global _ws, _reconnect_delay
    
    while _running:
        try:
            if not WORKER_API_KEY:
                LOG.error("WORKER_API_KEY not set")
                time.sleep(60)
                continue
            
            ws_url = f"{WORKER_SERVER}?api_key={WORKER_API_KEY}"
            
            _ws = websocket.WebSocketApp(
                ws_url,
                on_open=on_open,
                on_message=on_message,
                on_error=on_error,
                on_close=on_close,
                header={"X-API-Key": WORKER_API_KEY}
            )
            
            LOG.info(f"Connecting to {WORKER_SERVER}...")
            _ws.run_forever(ping_interval=30, ping_timeout=10)
            
        except Exception as e:
            LOG.error(f"Connection error: {e}")
        
        LOG.info(f"Waiting {_reconnect_delay}s before reconnect...")
        time.sleep(_reconnect_delay)
        _reconnect_delay = min(_reconnect_delay * 2, 60)


def main():
    global _running
    
    if not WORKER_API_KEY:
        LOG.error("WORKER_API_KEY environment variable is required")
        sys.exit(1)
    
    LOG.info(f"Remote translator worker starting...")
    LOG.info(f"Server: {WORKER_SERVER}")
    LOG.info(f"Device: {DEVICE}")
    LOG.info(f"Model: {MODEL_PATH}")
    
    ensure_model()
    
    stats_thread = threading.Thread(target=stats_reporter, daemon=True)
    stats_thread.start()
    
    connect()


if __name__ == "__main__":
    main()