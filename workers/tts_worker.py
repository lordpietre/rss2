"""Punto de entrada del servicio de TTS de RSS2.

    python workers/tts_worker.py

Arranca la API interna y el worker de sintesis en el mismo proceso. El
servicio no se expone fuera de la red de Docker: no publica puertos.
"""

from __future__ import annotations

import logging
import os
import signal
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from tts.config import TTSConfig  # noqa: E402
from tts.service import TTSService  # noqa: E402


def main() -> int:
    logging.basicConfig(
        level=os.environ.get("LOG_LEVEL", "INFO").upper(),
        format="[TTS] %(asctime)s %(levelname)s: %(message)s",
    )
    log = logging.getLogger("tts")

    cfg = TTSConfig.from_env()
    if not cfg.api_key:
        log.warning(
            "TTS_API_KEY no esta definido: el servicio acepta peticiones sin clave. "
            "Define la variable en produccion (el backend Go la envia como X-API-Key)."
        )

    service = TTSService(cfg)
    service.start()
    service.start_worker()

    def _shutdown(signum, _frame):
        log.info("Senal %s recibida: parando el servicio", signum)
        service.stop()
        sys.exit(0)

    signal.signal(signal.SIGTERM, _shutdown)
    signal.signal(signal.SIGINT, _shutdown)

    app = service.create_app()
    log.info("Servicio TTS escuchando en %s:%s", cfg.host, cfg.port)
    app.run(host=cfg.host, port=cfg.port, threaded=True, use_reloader=False)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
