"""Servicio de TTS: API HTTP interna + bucle del worker.

Proceso unico con dos funciones:

1. API interna (Flask) en la red de Docker: estado del motor, normalizacion
   de guiones, consulta de tareas y cancelacion. No se publica ningun puerto
   al exterior.
2. Worker: consume la cola (Redis con respaldo en Postgres), invoca el motor,
   valida el WAV generado y registra el resultado.

El audio se escribe en el volumen persistente; en Postgres solo queda la ruta
relativa y los metadatos. Nada se publica automaticamente: el audio queda
``pending_review`` hasta que un editor lo aprueba.
"""

from __future__ import annotations

import json
import logging
import os
import shutil
import threading
import time
import uuid
from typing import Any, Dict, Optional

from . import db as ttsdb
from .bulletin import concatenate_wavs
from .config import TTSConfig
from .engine import EngineError, VoiceEngine, VoiceProfile, build_engines, resolve_engine
from .queue import TaskQueue
from .script import normalize_for_speech, split_sentences
from .voices import (
    MAX_SAMPLE_BYTES,
    VoiceError,
    analyze_sample,
    copy_builtin_sample,
    delete_sample,
    safe_join,
    sample_suffix,
    sha256_file,
    slugify,
    store_sample,
)

log = logging.getLogger("tts.service")


class TTSService:
    """Estado y logica del servicio."""

    def __init__(self, cfg: TTSConfig):
        self.cfg = cfg
        self.engines: Dict[str, VoiceEngine] = build_engines(cfg)
        self.queue = TaskQueue(cfg)
        self.conn = None
        self._stop = threading.Event()
        self._worker_thread: Optional[threading.Thread] = None
        # Una sola instancia de inferencia por modelo: el acceso al motor se
        # serializa para no duplicar memoria ni corromper el estado interno.
        self._engine_lock = threading.Lock()
        self._stats = {"jobs_done": 0, "jobs_failed": 0, "last_job_seconds": 0.0}

    # ------------------------------------------------------------------
    # Arranque
    # ------------------------------------------------------------------
    def start(self) -> None:
        for directory in (self.cfg.audio_dir, self.cfg.voices_dir, self.cfg.tmp_dir):
            os.makedirs(directory, exist_ok=True)

        self.conn = ttsdb.connect(self.cfg)
        ttsdb.ensure_schema(self.conn)
        log.info("Esquema TTS listo en Postgres")

        self.queue.connect()
        self._seed_builtin_voices()

        # Precarga del modelo en segundo plano: el catalogo de voces y la
        # primera sintesis no pagan la carga.
        threading.Thread(target=self._preload_engines, name="tts-preload", daemon=True).start()

        # Recuperacion: tareas que quedaron a medias por un reinicio.
        recovered = ttsdb.recover_stale_jobs(self.conn, self.cfg.job_timeout_seconds)
        if recovered["requeued"] or recovered["failed"]:
            log.warning(
                "Tareas recuperadas al arrancar: %s reencoladas, %s fallidas",
                recovered["requeued"],
                recovered["failed"],
            )

    def _preload_engines(self) -> None:
        """Precarga el motor por defecto; el resto se cargan en su primer uso.

        Precargarlos todos a la vez no cabe en la memoria asignada al
        contenedor: solo Chatterbox ocupa ~2 GB y sumado a MeloTTS y OpenVoice
        se pasa del limite y el contenedor muere. Un motor no precargado paga
        la carga en su primer trabajo (serializada por el mismo candado de
        sintesis) y a partir de ahi se queda en memoria.
        """
        for name, engine in self.engines.items():
            if name != self.cfg.default_engine:
                log.info("Motor '%s' se cargara en el primer uso", name)
                continue
            try:
                engine.ensure_loaded()
                log.info("Motor '%s' cargado", name)
            except EngineError as exc:
                # El servicio sigue arrancando: si falta un peso, el error se
                # ve en /health y en el error de la tarea.
                log.warning("El motor '%s' no se pudo cargar: %s", name, exc.message)

    def stop(self) -> None:
        self._stop.set()
        if self._worker_thread:
            self._worker_thread.join(timeout=10)
        if self.conn:
            try:
                self.conn.close()
            except Exception:
                pass

    # ------------------------------------------------------------------
    # Voces
    # ------------------------------------------------------------------
    def _seed_builtin_voices(self) -> None:
        """Registra las voces del motor que no requieren audio de referencia.

        La voz base (``es-base``) es la del MOTOR POR DEFECTO: si cambia
        ``TTS_ENGINE``, la voz base cambia con ella. Las voces que declaran
        otro motor (hablantes de MeloTTS, voces ``ov-*`` de OpenVoice) no se
        tocan: solo cambia la base.
        """
        for slug in self.cfg.builtin_voices:
            existing = ttsdb.get_voice_by_slug(self.conn, slug)
            if existing is None:
                ttsdb.insert_voice(
                    self.conn,
                    slug=slug,
                    display_name="Voz base en español",
                    engine=self.cfg.default_engine,
                    language="es",
                    speaker_id=self.cfg.melo_speaker_id,
                    quality_status="ok",
                    is_builtin=True,
                )
                log.info("Voz integrada registrada: %s", slug)
            elif existing.get("engine") != self.cfg.default_engine:
                # La voz base sigue al motor por defecto. Solo se actualiza
                # esta voz: el resto de las integradas declaran su motor.
                ttsdb.update_voice_engine(self.conn, int(existing["id"]), self.cfg.default_engine)
                log.info(
                    "Voz integrada '%s': motor %s -> %s (el del sistema)",
                    slug,
                    existing.get("engine"),
                    self.cfg.default_engine,
                )

        self._seed_builtin_clones()

    def _seed_builtin_clones(self) -> None:
        """Registra las voces preinstaladas.

        Hay dos clases:

        - **Clones con audio de referencia** (voces demo): un .mp3 del que se
          extrae el timbre.
        - **Voces con embedding precalculado** (voces base de OpenVoice V2):
          un .pth que ya contiene el timbre, sin necesidad de audio.
        """
        self._seed_voices_from_manifest(
            os.path.join(self.cfg.openvoice_model_dir, "voices", "manifest.json"),
            os.path.join(self.cfg.openvoice_model_dir, "voices"),
            engine="openvoice",
        )
        self._seed_voices_from_manifest(
            self.cfg.openvoice_base_speakers_manifest,
            os.path.dirname(self.cfg.openvoice_base_speakers_manifest),
            engine="openvoice",
        )

    def _seed_voices_from_manifest(self, manifest_path: str, base_dir: str, engine: str) -> None:
        """Registra las voces de un manifiesto (copiando el fichero al volumen)."""
        if not manifest_path or not os.path.isfile(manifest_path):
            return

        try:
            with open(manifest_path, "r", encoding="utf-8") as handle:
                manifest = json.load(handle)
        except (OSError, ValueError) as exc:
            log.warning("No se pudo leer el manifiesto de voces (%s): %s", manifest_path, exc)
            return

        for entry in manifest.get("voices", []):
            slug = entry.get("slug")
            source = entry.get("file")
            if not slug or not source:
                continue
            if ttsdb.get_voice_by_slug(self.conn, slug):
                continue

            source_path = os.path.join(base_dir, os.path.basename(source))
            if not os.path.isfile(source_path):
                log.warning("Falta el audio de referencia de la voz %s: %s", slug, source_path)
                continue

            try:
                target = copy_builtin_sample(source_path, self.cfg.voices_dir, slug)
                if not target:
                    continue
                voice_id = ttsdb.insert_voice(
                    self.conn,
                    slug=slug,
                    display_name=entry.get("display_name") or slug,
                    engine=entry.get("engine") or engine,
                    language=entry.get("language") or "es",
                    reference_file=os.path.basename(target),
                    quality_status="ok",
                    is_builtin=True,
                )
                log.info("Voz preinstalada registrada: %s (id %d)", slug, voice_id)
            except (VoiceError, OSError) as exc:
                log.warning("No se pudo registrar la voz %s: %s", slug, exc)

    def _voice_profile(self, voice_row: Dict[str, Any]) -> VoiceProfile:
        reference_path = None
        embedding_path = None
        reference_file = voice_row.get("reference_file")
        if reference_file:
            safe = safe_join(self.cfg.voices_dir, os.path.basename(reference_file))
            # Un .pth es un embedding de timbre precalculado; el resto, un
            # audio de referencia del que se extrae el timbre.
            if safe.endswith(".pth"):
                embedding_path = safe
            else:
                reference_path = safe
        return VoiceProfile(
            id=voice_row.get("id"),
            slug=voice_row.get("slug", ""),
            display_name=voice_row.get("display_name", ""),
            engine=voice_row.get("engine", self.cfg.default_engine),
            language=(voice_row.get("language") or "es").strip(),
            speaker_id=voice_row.get("speaker_id"),
            reference_path=reference_path,
            embedding_path=embedding_path,
        )

    # ------------------------------------------------------------------
    # Worker
    # ------------------------------------------------------------------
    def start_worker(self) -> None:
        if self._worker_thread and self._worker_thread.is_alive():
            return
        self._worker_thread = threading.Thread(target=self._worker_loop, name="tts-worker", daemon=True)
        self._worker_thread.start()
        log.info("Worker de TTS en marcha")

    def _worker_loop(self) -> None:
        last_watchdog = 0.0
        while not self._stop.is_set():
            try:
                # Vigilante: revisa periodicamente las tareas atascadas. No
                # basta con hacerlo al arrancar: el worker puede colgarse
                # con el servicio en marcha.
                now = time.time()
                if now - last_watchdog >= self.cfg.watchdog_interval:
                    last_watchdog = now
                    self._run_watchdog()

                job = self._next_job()
                if job is None:
                    self._stop.wait(self.cfg.sleep_idle)
                    continue
                self._process_job(job)
            except Exception:
                log.exception("Error inesperado en el worker de TTS")
                self._stop.wait(self.cfg.sleep_idle)

    def _run_watchdog(self) -> None:
        """Reencola o falla las tareas que llevan demasiado tiempo en marcha."""
        try:
            result = ttsdb.recover_stale_jobs(self.conn, self.cfg.job_timeout_seconds)
        except Exception as exc:
            log.warning("El vigilante no pudo revisar las tareas: %s", exc)
            return
        if result["requeued"] or result["failed"]:
            log.warning(
                "Vigilante: %s tareas reencoladas y %s fallidas por exceder el tiempo maximo",
                result["requeued"],
                result["failed"],
            )

    def _next_job(self) -> Optional[Dict[str, Any]]:
        """Saca la siguiente tarea de la cola (Redis) o de Postgres."""
        job_id = self.queue.pop(timeout=0)
        if job_id is not None:
            job = ttsdb.claim_job(self.conn, job_id)
            if job:
                return job
            # La tarea ya no estaba en cola (otro worker la cogio): se sigue
            # mirando en Postgres para no dejar trabajo parado.

        return ttsdb.claim_next_job(self.conn)

    def _process_job(self, job: Dict[str, Any]) -> None:
        job_id = int(job["id"])
        started = time.time()
        log.info("Procesando tarea TTS %s (script=%s, motor=%s)", job_id, job["script_id"], job["engine"])

        script = ttsdb.get_script(self.conn, int(job["script_id"]))
        if not script:
            ttsdb.mark_job_failed(self.conn, job_id, "script_missing", "El guion ya no existe")
            self._stats["jobs_failed"] += 1
            return

        if script["status"] != ttsdb.SCRIPT_APPROVED:
            ttsdb.mark_job_failed(
                self.conn,
                job_id,
                "script_not_approved",
                "El guion %s no esta aprobado por un editor" % script["id"],
            )
            self._stats["jobs_failed"] += 1
            return

        voice_row = None
        if job.get("voice_id"):
            voice_row = ttsdb.get_voice(self.conn, int(job["voice_id"]))
        if voice_row is None:
            voice_row = ttsdb.get_voice_by_slug(self.conn, self.cfg.builtin_voices[0])
        if voice_row is None:
            ttsdb.mark_job_failed(self.conn, job_id, "voice_missing", "No hay ninguna voz disponible")
            self._stats["jobs_failed"] += 1
            return

        voice = self._voice_profile(voice_row)

        try:
            engine = resolve_engine(self.engines, job["engine"])
        except EngineError as exc:
            ttsdb.mark_job_failed(self.conn, job_id, exc.code, exc.message)
            self._stats["jobs_failed"] += 1
            return

        text = script["content"] or ""
        if not text.strip():
            ttsdb.mark_job_failed(self.conn, job_id, "empty_script", "El guion esta vacio")
            self._stats["jobs_failed"] += 1
            return

        try:
            result = self._synthesize_to_disk(
                engine,
                text,
                voice,
                job_id,
                int(script["id"]),
                speed=float(job.get("speed") or 1.0),
            )
        except EngineError as exc:
            attempts = int(job.get("attempts") or 1)
            max_attempts = int(job.get("max_attempts") or self.cfg.max_attempts)
            retryable = exc.retryable and attempts < max_attempts
            log.error("Tarea TTS %s fallida (%s): %s", job_id, exc.code, exc.message)
            ttsdb.mark_job_failed(self.conn, job_id, exc.code, exc.message, requeue=retryable)
            if not retryable:
                self._stats["jobs_failed"] += 1
            return
        except Exception as exc:  # pragma: no cover - defensivo
            log.exception("Tarea TTS %s: error inesperado", job_id)
            ttsdb.mark_job_failed(self.conn, job_id, "unexpected_error", str(exc))
            self._stats["jobs_failed"] += 1
            return

        relative_path = result["relative_path"]
        try:
            audio_id = ttsdb.insert_audio(
                self.conn,
                job_id=job_id,
                script_id=int(script["id"]),
                voice_id=voice_row.get("id"),
                engine=result["engine"],
                engine_version=result["engine_version"],
                storage_path=relative_path,
                duration_seconds=result["duration_seconds"],
                sample_rate=result["sample_rate"],
                size_bytes=result["size_bytes"],
                sha256=result["sha256"],
                parameters=result["parameters"],
            )
        except Exception:
            log.exception("No se pudo registrar el audio de la tarea %s", job_id)
            ttsdb.mark_job_failed(self.conn, job_id, "audio_register_failed", "No se pudo registrar el audio")
            self._stats["jobs_failed"] += 1
            return

        ttsdb.mark_job_succeeded(self.conn, job_id)
        elapsed = time.time() - started
        self._stats["jobs_done"] += 1
        self._stats["last_job_seconds"] = round(elapsed, 2)
        log.info(
            "Tarea TTS %s completada en %.1fs (audio %s, %.1fs de audio)",
            job_id,
            elapsed,
            audio_id,
            result["duration_seconds"],
        )

        # Si este guion forma parte de un noticiario a la espera, se comprueba
        # si ya estan todas las narraciones y se monta el noticiario.
        try:
            self._maybe_build_bulletins(int(script["id"]))
        except Exception:
            log.exception("No se pudo actualizar el noticiario tras la tarea %s", job_id)

    # ------------------------------------------------------------------
    # Noticiario
    # ------------------------------------------------------------------
    def _maybe_build_bulletins(self, script_id: int) -> None:
        """Monta los noticiarios a la espera en cuanto estan todas sus piezas."""
        for bulletin in ttsdb.get_pending_bulletins_for_script(self.conn, script_id):
            self.build_bulletin(int(bulletin["id"]))

    def build_bulletin(self, bulletin_id: int) -> Optional[Dict[str, Any]]:
        """Concatena las narraciones de un noticiario, si estan todas listas.

        Devuelve el estado del noticiario. Si faltan piezas, queda en
        'pending' y no se toca nada.
        """
        bulletin = ttsdb.get_bulletin(self.conn, bulletin_id)
        if not bulletin:
            return None
        if bulletin["status"] not in (ttsdb.BULLETIN_PENDING, ttsdb.BULLETIN_FAILED):
            return bulletin

        items = ttsdb.get_bulletin_items(self.conn, bulletin_id)
        if not items:
            return bulletin

        ready: List[Dict[str, Any]] = []
        pending = 0
        for item in items:
            audio = ttsdb.get_latest_audio_for_script(self.conn, int(item["script_id"]))
            if not audio:
                pending += 1
                continue

            audio_path = os.path.join(self.cfg.audio_dir, audio["storage_path"])
            if not os.path.isfile(audio_path):
                pending += 1
                continue

            ttsdb.set_bulletin_item_audio(
                self.conn,
                bulletin_id,
                int(item["script_id"]),
                int(audio["id"]),
                audio["storage_path"],
                float(audio["duration_seconds"] or 0.0),
            )
            ready.append(
                {
                    "script_id": int(item["script_id"]),
                    "audio_id": int(audio["id"]),
                    "path": audio_path,
                    "duration_seconds": audio["duration_seconds"],
                }
            )

        if pending:
            log.info(
                "Noticiario %s a la espera: %s de %s narraciones listas",
                bulletin_id,
                len(ready),
                len(items),
            )
            return bulletin

        relative_dir = "bulletins"
        target_dir = os.path.join(self.cfg.audio_dir, relative_dir)
        os.makedirs(target_dir, exist_ok=True)
        target_path = os.path.join(target_dir, "bulletin-%s.wav" % bulletin_id)

        try:
            result = concatenate_wavs(
                ready,
                target_path,
                silence_ms=int(bulletin.get("silence_ms") or 0),
            )
        except EngineError as exc:
            log.error("No se pudo montar el noticiario %s: %s", bulletin_id, exc.message)
            ttsdb.update_bulletin_audio(self.conn, bulletin_id, ttsdb.BULLETIN_FAILED)
            return ttsdb.get_bulletin(self.conn, bulletin_id)

        # Cada item guarda su audio y el offset de inicio dentro del
        # noticiario: el manifiesto permite mapear el audio sobre las noticias.
        for entry in result["items"]:
            ttsdb.set_bulletin_item_audio(
                self.conn,
                bulletin_id,
                int(entry["script_id"]),
                int(entry.get("audio_id") or 0),
                "",
                float(entry["duration_seconds"]),
                float(entry["start_offset"]),
            )

        ttsdb.update_bulletin_audio(
            self.conn,
            bulletin_id,
            ttsdb.BULLETIN_READY,
            audio_path=os.path.join(relative_dir, "bulletin-%s.wav" % bulletin_id),
            duration_seconds=result["duration_seconds"],
            size_bytes=result["size_bytes"],
            sha256=sha256_file(target_path),
        )

        log.info(
            "Noticiario %s montado: %s noticias, %.1fs de audio",
            bulletin_id,
            len(result["items"]),
            result["duration_seconds"],
        )
        return ttsdb.get_bulletin(self.conn, bulletin_id)

    def _synthesize_to_disk(
        self,
        engine: VoiceEngine,
        text: str,
        voice: VoiceProfile,
        job_id: int,
        script_id: int,
        speed: float = 1.0,
    ) -> Dict[str, Any]:
        """Sintetiza a un temporal, valida el WAV y lo mueve a su ruta final."""
        os.makedirs(self.cfg.tmp_dir, exist_ok=True)
        tmp_path = os.path.join(self.cfg.tmp_dir, "job-%s-%s.wav" % (job_id, uuid.uuid4().hex[:8]))

        with self._engine_lock:
            result = engine.synthesize(text, voice, tmp_path, {"speed": speed})

        # Comprobacion explicita del archivo: existe, se decodifica y tiene
        # una duracion valida.
        from .engine import probe_audio

        meta = probe_audio(tmp_path)
        if meta["duration_seconds"] <= 0:
            raise EngineError("audio_invalid", "El audio generado no tiene duracion")

        relative_dir = os.path.join(str(script_id))
        target_dir = os.path.join(self.cfg.audio_dir, relative_dir)
        os.makedirs(target_dir, exist_ok=True)
        filename = "job-%s.wav" % job_id
        target_path = os.path.join(target_dir, filename)
        shutil.move(tmp_path, target_path)

        return {
            "relative_path": os.path.join(relative_dir, filename),
            "duration_seconds": meta["duration_seconds"],
            "sample_rate": meta["sample_rate"],
            "size_bytes": os.path.getsize(target_path),
            "sha256": sha256_file(target_path),
            "engine": result.engine,
            "engine_version": result.engine_version,
            "parameters": result.parameters,
        }

    # ------------------------------------------------------------------
    # API
    # ------------------------------------------------------------------
    def create_app(self):
        from flask import Flask, g, jsonify, request

        app = Flask("tts")
        app.config["MAX_CONTENT_LENGTH"] = MAX_SAMPLE_BYTES + (1024 * 1024)

        @app.before_request
        def _authenticate():
            if request.path == "/health":
                return None
            key = self.cfg.api_key
            if not key:
                return None
            provided = request.headers.get("X-API-Key", "")
            if provided != key:
                return jsonify({"error": "unauthorized", "message": "Clave de API invalida"}), 401
            return None

        @app.after_request
        def _no_store(response):
            response.headers["Cache-Control"] = "no-store"
            return response

        @app.get("/health")
        def health():
            engines = {name: engine.health() for name, engine in self.engines.items()}
            return jsonify(
                {
                    "status": "ok",
                    "service": "tts",
                    "device": self.cfg.device,
                    "engines": engines,
                    "queue": {
                        "redis": self.queue.available,
                        "pending": self.queue.size(),
                    },
                    "stats": self._stats,
                }
            )

        @app.get("/api/voices")
        def list_voices():
            voices = ttsdb.list_voices(self.conn)
            for voice in voices:
                # La ruta interna del audio de referencia no se expone.
                voice.pop("reference_file", None)
            return jsonify({"voices": voices})

        @app.post("/api/voices")
        def create_voice():
            """Registra una muestra de voz propia.

            La accion es siempre explicita: el servicio no procesa ninguna
            grabacion por su cuenta. La muestra se guarda en el volumen de
            voces, que no publica ningun servidor web.
            """
            if not self.cfg.allow_voice_upload:
                return (
                    jsonify(
                        {
                            "error": "voice_upload_disabled",
                            "message": "El registro de muestras de voz esta deshabilitado",
                        }
                    ),
                    403,
                )

            upload = request.files.get("sample")
            if upload is None or not upload.filename:
                return jsonify({"error": "missing_sample", "message": "Falta el archivo de audio"}), 400

            data = upload.read()
            if len(data) > MAX_SAMPLE_BYTES:
                return (
                    jsonify({"error": "sample_too_large", "message": "La muestra supera el tamano maximo"}),
                    413,
                )

            slug = slugify(request.form.get("slug") or upload.filename)
            display_name = (request.form.get("display_name") or slug).strip()[:128]
            language = (request.form.get("language") or "es").strip().lower()[:5]

            # Motor de la voz: por defecto el del sistema (Chatterbox clona
            # timbre y acento con el audio). Se puede pedir "openvoice" para
            # la conversion de timbre clasica sobre la voz de MeloTTS.
            engine = (request.form.get("engine") or self.cfg.default_engine).strip().lower()
            if engine not in self.cfg.engines:
                return (
                    jsonify(
                        {
                            "error": "unknown_engine",
                            "message": "Motor desconocido: %s" % engine,
                        }
                    ),
                    400,
                )

            try:
                # La extension del fichero es la que decide como se decodifica
                # (un MP3 guardado como .wav no se puede leer): se respeta la
                # del archivo subido y se valida aqui mismo.
                suffix = sample_suffix(upload.filename)
                path = store_sample(data, slug, self.cfg.voices_dir, suffix=suffix)
            except VoiceError as exc:
                return jsonify({"error": exc.code, "message": exc.message}), 400

            try:
                report = analyze_sample(
                    path,
                    self.cfg.min_sample_seconds,
                    self.cfg.max_sample_seconds,
                )
            except VoiceError as exc:
                delete_sample(path, self.cfg.voices_dir)
                return jsonify({"error": exc.code, "message": exc.message}), 400

            voice_id = ttsdb.insert_voice(
                self.conn,
                slug=slug,
                display_name=display_name,
                engine=engine,
                language=language,
                reference_file=os.path.basename(path),
                duration_seconds=report.duration_seconds,
                sample_rate=report.sample_rate,
                quality_status="ok" if report.ok else "poor",
                quality_report=report.to_dict(),
                is_builtin=False,
            )

            voice = ttsdb.get_voice(self.conn, voice_id)
            if voice:
                voice.pop("reference_file", None)
            return jsonify({"voice": voice, "quality": report.to_dict()}), 201

        @app.delete("/api/voices/<int:voice_id>")
        def delete_voice(voice_id: int):
            voice = ttsdb.get_voice(self.conn, voice_id)
            if not voice:
                return jsonify({"error": "not_found", "message": "La voz no existe"}), 404

            reference_file = voice.get("reference_file")
            ttsdb.delete_voice(self.conn, voice_id)
            if reference_file:
                delete_sample(
                    os.path.join(self.cfg.voices_dir, os.path.basename(reference_file)),
                    self.cfg.voices_dir,
                )
            return jsonify({"deleted": True})

        @app.get("/api/voices/<int:voice_id>/quality")
        def voice_quality(voice_id: int):
            voice = ttsdb.get_voice(self.conn, voice_id)
            if not voice:
                return jsonify({"error": "not_found", "message": "La voz no existe"}), 404
            report = voice.get("quality_report") or {}
            return jsonify(
                {
                    "voice_id": voice_id,
                    "slug": voice.get("slug"),
                    "quality_status": voice.get("quality_status"),
                    "report": report,
                }
            )

        @app.post("/api/scripts/normalize")
        def normalize_script():
            payload = request.get_json(silent=True) or {}
            text = payload.get("text") or ""
            if not isinstance(text, str):
                return jsonify({"error": "invalid_request", "message": "El texto debe ser una cadena"}), 400
            if len(text) > self.cfg.max_script_chars * 2:
                return jsonify({"error": "text_too_long", "message": "El texto supera el limite permitido"}), 400
            language = payload.get("language") or "es"
            if self.cfg.normalize_script:
                normalized = normalize_for_speech(text, language)
            else:
                normalized = text.strip()
            return jsonify(
                {
                    "text": normalized,
                    "sentences": len(split_sentences(normalized)),
                    "length": len(normalized),
                }
            )

        @app.get("/api/jobs/<int:job_id>")
        def get_job(job_id: int):
            job = ttsdb.get_job(self.conn, job_id)
            if not job:
                return jsonify({"error": "not_found", "message": "La tarea no existe"}), 404
            audio = ttsdb.get_audio_by_job(self.conn, job_id)
            job["audio"] = audio
            return jsonify({"job": job})

        @app.post("/api/jobs/<int:job_id>/cancel")
        def cancel_job(job_id: int):
            job = ttsdb.get_job(self.conn, job_id)
            if not job:
                return jsonify({"error": "not_found", "message": "La tarea no existe"}), 404
            if job["status"] == ttsdb.JOB_RUNNING:
                return (
                    jsonify(
                        {
                            "error": "job_running",
                            "message": "La tarea ya esta en ejecucion y no se puede cancelar de forma segura",
                        }
                    ),
                    409,
                )
            if not ttsdb.cancel_job(self.conn, job_id):
                return (
                    jsonify({"error": "not_cancellable", "message": "La tarea no se puede cancelar en su estado actual"}),
                    409,
                )
            return jsonify({"job": ttsdb.get_job(self.conn, job_id)})

        @app.get("/api/speakers")
        def model_speakers():
            """Hablantes que ofrece el modelo base (256 en el modelo español)."""
            engine_name = request.args.get("engine") or self.cfg.default_engine
            engine = self.engines.get(engine_name)
            if engine is None:
                return jsonify({"error": "unknown_engine", "message": "Motor desconocido"}), 400
            try:
                speakers = engine.speakers()
            except Exception as exc:
                return jsonify({"error": "speakers_unavailable", "message": str(exc)}), 503
            return jsonify({"engine": engine_name, "speakers": speakers})

        @app.post("/api/speakers")
        def register_model_speaker():
            """Registra un hablante del modelo como voz utilizable."""
            payload = request.get_json(silent=True) or {}
            try:
                speaker_id = int(payload.get("speaker_id"))
            except (TypeError, ValueError):
                return jsonify({"error": "invalid_request", "message": "speaker_id debe ser un numero"}), 400
            if speaker_id < 0:
                return jsonify({"error": "invalid_request", "message": "speaker_id invalido"}), 400

            engine_name = payload.get("engine") or self.cfg.default_engine
            engine = self.engines.get(engine_name)
            if engine is None:
                return jsonify({"error": "unknown_engine", "message": "Motor desconocido"}), 400

            try:
                available = len(engine.speakers())
            except Exception:
                available = 0
            if available and speaker_id >= available:
                return (
                    jsonify(
                        {
                            "error": "speaker_out_of_range",
                            "message": "El modelo tiene %s hablantes" % available,
                        }
                    ),
                    400,
                )

            slug = slugify(payload.get("slug") or ("%s-speaker-%d" % (engine_name, speaker_id)))
            voice_id = ttsdb.insert_voice(
                self.conn,
                slug=slug,
                display_name=(payload.get("display_name") or "Hablante %d" % speaker_id)[:128],
                engine=engine_name,
                language=self.cfg.melo_language.lower(),
                speaker_id=str(speaker_id),
                quality_status="ok",
                is_builtin=True,
            )
            voice = ttsdb.get_voice(self.conn, voice_id)
            if voice:
                voice.pop("reference_file", None)
            return jsonify({"voice": voice}), 201

        @app.get("/api/bulletins/<int:bulletin_id>")
        def get_bulletin(bulletin_id: int):
            bulletin = ttsdb.get_bulletin(self.conn, bulletin_id)
            if not bulletin:
                return jsonify({"error": "not_found", "message": "El noticiario no existe"}), 404
            bulletin["items"] = ttsdb.get_bulletin_items(self.conn, bulletin_id)
            return jsonify({"bulletin": bulletin})

        @app.post("/api/bulletins/<int:bulletin_id>/build")
        def build_bulletin(bulletin_id: int):
            """Fuerza el montaje del noticiario (reintento manual)."""
            bulletin = self.build_bulletin(bulletin_id)
            if not bulletin:
                return jsonify({"error": "not_found", "message": "El noticiario no existe"}), 404
            bulletin["items"] = ttsdb.get_bulletin_items(self.conn, bulletin_id)
            return jsonify({"bulletin": bulletin})

        @app.post("/api/synthesize")
        def synthesize():
            """Sintesis sincrona (uso interno y pruebas)."""
            payload = request.get_json(silent=True) or {}
            text = payload.get("text") or ""
            voice_slug = payload.get("voice") or (self.cfg.builtin_voices[0] if self.cfg.builtin_voices else "")
            if not text.strip():
                return jsonify({"error": "empty_text", "message": "El texto esta vacio"}), 400
            if len(text) > self.cfg.max_script_chars:
                return (
                    jsonify({"error": "text_too_long", "message": "El texto supera el limite permitido"}),
                    413,
                )

            voice_row = ttsdb.get_voice_by_slug(self.conn, voice_slug)
            if not voice_row:
                return jsonify({"error": "voice_not_found", "message": "La voz no existe"}), 404

            engine_name = payload.get("engine") or voice_row["engine"] or self.cfg.default_engine
            try:
                engine = resolve_engine(self.engines, engine_name)
            except EngineError as exc:
                return jsonify({"error": exc.code, "message": exc.message}), 400

            voice = self._voice_profile(voice_row)
            os.makedirs(self.cfg.tmp_dir, exist_ok=True)
            tmp_path = os.path.join(self.cfg.tmp_dir, "sync-%s.wav" % uuid.uuid4().hex[:10])
            try:
                with self._engine_lock:
                    result = engine.synthesize(text, voice, tmp_path, {})
            except EngineError as exc:
                return jsonify({"error": exc.code, "message": exc.message}), 502

            from .engine import probe_audio

            meta = probe_audio(tmp_path)
            return jsonify(
                {
                    "path": tmp_path,
                    "duration_seconds": meta["duration_seconds"],
                    "sample_rate": meta["sample_rate"],
                    "engine": result.engine,
                    "engine_version": result.engine_version,
                    "parameters": result.parameters,
                }
            )

        return app
