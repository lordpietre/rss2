"""Cola asincrona de tareas de TTS.

Postgres es la fuente de verdad (durable). Redis actua como canal de
despacho rapido: el backend encola ahi y el worker recoge de inmediato. Si
Redis no esta disponible, el worker sigue funcionando consultando Postgres,
y si el worker muere, las tareas siguen en la tabla y se recuperan al
reiniciar.
"""

from __future__ import annotations

import logging
from typing import Optional

log = logging.getLogger("tts.queue")


class TaskQueue:
    """Cola con Redis y respaldo en Postgres."""

    def __init__(self, cfg):
        self.cfg = cfg
        self._client = None
        self._redis_failed = False

    # -- conexion ----------------------------------------------------------
    def connect(self) -> bool:
        try:
            import redis
        except ImportError:
            log.warning("redis no esta instalado: se usara solo la cola de Postgres")
            self._redis_failed = True
            return False
        try:
            self._client = redis.Redis.from_url(
                self.cfg.redis_url,
                socket_connect_timeout=3,
                socket_timeout=5,
                decode_responses=True,
            )
            self._client.ping()
            log.info("Cola TTS conectada a Redis (%s)", self.cfg.queue_key)
            return True
        except Exception as exc:
            log.warning("Redis no disponible (%s): la cola usara Postgres", exc)
            self._redis_failed = True
            self._client = None
            return False

    @property
    def available(self) -> bool:
        return self._client is not None

    # -- operaciones -------------------------------------------------------
    def push(self, job_id: int) -> bool:
        """Encola una tarea. Devuelve False si Redis no esta disponible."""
        if self._client is None:
            return False
        try:
            self._client.lpush(self.cfg.queue_key, int(job_id))
            return True
        except Exception as exc:
            log.warning("No se pudo encolar la tarea %s en Redis: %s", job_id, exc)
            return False

    def pop(self, timeout: float = 1.0) -> Optional[int]:
        """Saca una tarea de la cola. Devuelve None si no hay ninguna."""
        if self._client is None:
            return None
        try:
            item = self._client.rpop(self.cfg.queue_key)
        except Exception as exc:
            log.warning("No se pudo leer la cola de Redis: %s", exc)
            return None
        if item is None:
            return None
        try:
            return int(item)
        except (TypeError, ValueError):
            return None

    def size(self) -> int:
        if self._client is None:
            return 0
        try:
            return int(self._client.llen(self.cfg.queue_key))
        except Exception:
            return 0
