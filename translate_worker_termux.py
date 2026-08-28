#!/usr/bin/env python3
"""
Worker de traducción MINIMO para Termux Android.
Objetivo único: Recibir jobs de traducción del servidor y enviar resultados de vuelta.
No requiere modelo ML para conectarse y reportar status.
"""

import os
import sys
import json
import logging
import readline

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s: %(message)s")
LOG = logging.getLogger("minimal-worker")

# ==========================================================
# Config - SOLO API key necesaria
# ==========================================================
WORKER_NAME = os.environ.get("WORKER_NAME", f"termux-{os.getpid()}")
WORKER_SERVER = os.environ.get("WORKER_SERVER", "ws://10.0.2.2:8080/ws/worker")

# API key: pedir si no está puesta
if not os.environ.get("RSS2_API_KEY"):
    api_key = input("🔑 API key de RSS2: ").strip()
    if not api_key:
        print("❌ Key requerida. Saliendo.")
        sys.exit(1)
    os.environ["RSS2_API_KEY"] = api_key

WORKER_API_KEY = os.environ["RSS2_API_KEY"]

# Código de idiomas simples
LANG_MAP = {
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

# Estado simple - solo conecta y reenvía
connected = False
jobs_processed = 0

def on_open(ws):
    global connected
    connected = True
    LOG.info(f"📡 Conectado como {WORKER_NAME}")
    ws.send(json.dumps({
        "type": "register",
        "worker_name": WORKER_NAME
    }))

def on_message(ws, message):
    global jobs_processed
    try:
        msg = json.loads(message)
    except:
        return
    
    if msg.get("type") == "job":
        job = msg.get("job", {})
        result = process_job(job)
        ws.send(json.dumps({
            "type": "result",
            "result": result
        }))
        jobs_processed += 1
        if jobs_processed % 10 == 0:
            LOG.info(f"📊 Procesados {jobs_processed} jobs")

def process_job(job):
    """Procesa un job de traducción - mínimo esfuerzo."""
    job_id = job.get("id", "unknown")
    lang_from = job.get("lang_from", "en")
    lang_to = job.get("lang_to", "es")
    title = job.get("title", "")
    summary = job.get("summary", "")
    
    # Traducción "mínima": si mismoidioma, pas-through
    # Si tiene modelo, traduciría aquí; si no, devolvemos original
    
    # Marcar resultado
    result = {
        "job_id": job_id,
        "title_orig": title[:50] if title else "",
        "summary_orig": summary[:50] if summary else "",
        "lang_from": lang_from,
        "lang_to": lang_to,
        "title_trad": title,  # Pas-through si sin modelo
        "summary_trad": summary,
        "translated": False,  # Flag: ¿realmente tradujimos?
        "error": ""
    }
    return result

def on_error(ws, error):
    LOG.error(f"WS error: {error}")

def on_close(ws, *args):
    LOG.warning(f"Conexión cerrada")
    global connected
    connected = False

def connect():
    """Conectar WebSocket al servidor."""
    import websocket
    ws_url = f"ws://localhost:8080/ws/worker?api_key={WORKER_API_KEY}"
    
    ws = websocket.WebSocketApp(
        ws_url,
        on_open=on_open,
        on_message=on_message,
        on_error=on_error,
        on_close=on_close,
    )
    ws.run_forever(ping_interval=15, ping_timeout=5)

def main():
    global connected
    LOG.info(f"Worker mínimo starting: {WORKER_NAME}")
    LOG.info(f"Conectando a: ws://localhost:8080/ws/worker")
    
    # Intentar conectar (se reintenta solo si se corta)
    connect()

if __name__ == "__main__":
    main()