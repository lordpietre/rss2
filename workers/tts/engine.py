"""Interfaz de motor de voz.

RSS2 no habla con un motor concreto: habla con esta interfaz. Anadir un motor
nuevo (una voz base mejor, un clon mas fiel, un TTS remoto) es registrar otra
clase y su configuracion; el pipeline editorial no se toca.

Contrato:

- ``available()``  -> el motor puede usarse ahora mismo.
- ``health()``     -> estado legible para el endpoint de salud.
- ``synthesize()`` -> escribe un WAV en ``output_path`` y devuelve metadatos.
- ``speakers()``   -> hablantes que el motor ofrece de serie.

Los imports de ``torch``/``melo``/``openvoice``/``chatterbox`` son perezosos:
sin el modelo instalado el modulo se importa igual y las pruebas unitarias
funcionan.
"""

from __future__ import annotations

import os
import time
from abc import ABC, abstractmethod
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional

from .config import TTSConfig


class EngineError(Exception):
    """Error del motor, con codigo estable para la API y los registros."""

    def __init__(self, code: str, message: str, retryable: bool = False):
        super().__init__(message)
        self.code = code
        self.message = message
        self.retryable = retryable


@dataclass
class VoiceProfile:
    """Perfil de timbre tal y como lo usa el motor."""

    id: Optional[int] = None
    slug: str = ""
    display_name: str = ""
    engine: str = ""
    language: str = "es"
    speaker_id: Optional[str] = None
    reference_path: Optional[str] = None
    # Embedding de timbre precalculado (.pth). Si existe, no hace falta audio
    # de referencia para clonar la voz.
    embedding_path: Optional[str] = None
    parameters: Dict[str, Any] = field(default_factory=dict)


@dataclass
class SynthesisResult:
    """Resultado de una sintesis."""

    path: str
    duration_seconds: float
    sample_rate: int
    engine: str
    engine_version: str
    parameters: Dict[str, Any] = field(default_factory=dict)
    elapsed_seconds: float = 0.0


def probe_audio(path: str) -> Dict[str, Any]:
    """Lee los metadatos de un archivo de audio.

    Devuelve ``duration_seconds``, ``sample_rate``, ``channels`` y ``frames``.
    Lanza ``EngineError`` si el archivo no existe o no se puede decodificar.
    """
    if not os.path.isfile(path):
        raise EngineError("audio_missing", "El archivo de audio no existe", retryable=False)
    try:
        import soundfile
    except ImportError as exc:  # pragma: no cover - dependencia del contenedor
        raise EngineError("audio_probe_unavailable", str(exc), retryable=False) from exc

    try:
        info = soundfile.info(path)
    except Exception as exc:
        raise EngineError("audio_unreadable", "El archivo de audio no se puede decodificar") from exc

    if info.frames <= 0 or info.samplerate <= 0:
        raise EngineError("audio_empty", "El archivo de audio esta vacio")

    return {
        "duration_seconds": round(float(info.frames) / float(info.samplerate), 3),
        "sample_rate": int(info.samplerate),
        "channels": int(info.channels),
        "frames": int(info.frames),
    }


class VoiceEngine(ABC):
    """Interfaz comun a todos los motores de voz."""

    name = "base"
    version = "unknown"

    def __init__(self, cfg: TTSConfig):
        self.cfg = cfg
        self._loaded = False
        self._load_error: Optional[str] = None
        self._load_seconds: float = 0.0

    # -- estado ------------------------------------------------------------
    def available(self) -> bool:
        """True si el motor esta listo para sintetizar."""
        return self._loaded and self._load_error is None

    def health(self) -> Dict[str, Any]:
        return {
            "engine": self.name,
            "version": self.version,
            "loaded": self._loaded,
            "available": self.available(),
            "load_seconds": round(self._load_seconds, 2),
            "error": self._load_error,
        }

    def speakers(self) -> List[Dict[str, Any]]:
        return []

    # -- sintesis ----------------------------------------------------------
    @abstractmethod
    def synthesize(
        self,
        text: str,
        voice: VoiceProfile,
        output_path: str,
        params: Optional[Dict[str, Any]] = None,
    ) -> SynthesisResult:
        """Sintetiza ``text`` y escribe el WAV en ``output_path``."""

    # -- utilidades --------------------------------------------------------
    @staticmethod
    def _validate_text(text: str) -> None:
        if not text or not text.strip():
            raise EngineError("empty_text", "El guion esta vacio", retryable=False)

    def _prepare_output(self, output_path: str) -> None:
        directory = os.path.dirname(os.path.abspath(output_path))
        if directory:
            os.makedirs(directory, exist_ok=True)

    def _cleanup(self, path: Optional[str]) -> None:
        if path and os.path.exists(path):
            try:
                os.remove(path)
            except OSError:
                pass


class MeloTTSEngine(VoiceEngine):
    """Sintesis estandar con la voz base de MeloTTS para un idioma.

    No clona el timbre: es la voz "de serie" del motor. Es la opcion por
    defecto porque no necesita audio de referencia ni checkpoints extra.
    """

    name = "melotts"
    version = "melotts"

    def __init__(self, cfg: TTSConfig):
        super().__init__(cfg)
        self._model = None

    # -- carga -------------------------------------------------------------
    def _load(self) -> None:
        if self._model is not None:
            return
        started = time.time()
        try:
            import torch
            from melo.api import TTS
        except ImportError as exc:
            self._load_error = "MeloTTS no esta instalado: %s" % exc
            raise EngineError("engine_missing", self._load_error, retryable=False) from exc

        try:
            torch.set_num_threads(max(1, int(self.cfg.torch_threads)))
        except Exception:  # pragma: no cover - depende de la build de torch
            pass

        language = self.cfg.melo_language
        try:
            self._model = TTS(language=language, device=self.cfg.device)
        except Exception as exc:
            self._load_error = "No se pudo cargar el modelo de MeloTTS (%s): %s" % (language, exc)
            raise EngineError("engine_load_failed", self._load_error, retryable=True) from exc

        self._loaded = True
        self._load_error = None
        self._load_seconds = time.time() - started

    def ensure_loaded(self) -> None:
        if not self._loaded:
            self._load()

    # -- API ---------------------------------------------------------------
    def available(self) -> bool:
        return self._model is not None

    def health(self) -> Dict[str, Any]:
        info = super().health()
        info["language"] = self.cfg.melo_language
        info["device"] = self.cfg.device
        return info

    def speakers(self) -> List[Dict[str, Any]]:
        """Hablantes que ofrece el modelo base.

        MeloTTS se entrena con multiples hablantes por idioma: el modelo en
        español tiene 256. Cada uno es una voz distinta (difieren en timbre,
        prosodia y ritmo).
        """
        count = self._speaker_count()
        return [
            {
                "speaker_id": str(index),
                "language": self.cfg.melo_language,
                "engine": self.name,
                "builtin": True,
            }
            for index in range(count)
        ]

    def _speaker_count(self) -> int:
        """Numero de hablantes del modelo (256 en el modelo español).

        Si el modelo todavia no esta cargado, se carga: sin el no se puede
        saber cuantas voces hay. El servicio lo precarga al arrancar, asi que
        esto solo pasa la primera vez.
        """
        if self._model is None:
            try:
                self.ensure_loaded()
            except EngineError:
                return 0
        if self._model is None:
            return 0
        try:
            return int(self._model.hps.data.n_speakers)
        except Exception:
            return 0

    def synthesize(
        self,
        text: str,
        voice: VoiceProfile,
        output_path: str,
        params: Optional[Dict[str, Any]] = None,
    ) -> SynthesisResult:
        params = params or {}
        self._validate_text(text)

        self.ensure_loaded()
        self._prepare_output(output_path)

        speaker_id = params.get("speaker_id") or voice.speaker_id or self.cfg.melo_speaker_id
        try:
            speaker_id = int(str(speaker_id).strip())
        except (TypeError, ValueError):
            speaker_id = int(self.cfg.melo_speaker_id)

        speed = float(params.get("speed", self.cfg.default_speed) or 1.0)
        started = time.time()
        try:
            self._model.tts_to_file(
                text,
                speaker_id=speaker_id,
                output_path=output_path,
                speed=speed,
                quiet=True,
            )
        except Exception as exc:
            self._cleanup(output_path)
            raise EngineError("synthesis_failed", "El motor no pudo sintetizar: %s" % exc, retryable=True) from exc

        meta = probe_audio(output_path)
        return SynthesisResult(
            path=output_path,
            duration_seconds=meta["duration_seconds"],
            sample_rate=meta["sample_rate"],
            engine=self.name,
            engine_version=self.version,
            parameters={"speaker_id": speaker_id, "speed": speed},
            elapsed_seconds=time.time() - started,
        )


class OpenVoiceEngine(VoiceEngine):
    """Conversion de timbre con OpenVoice V2.

    Usa MeloTTS como sintesis base y OpenVoice para convertir el timbre al de
    un audio de referencia. Requiere los checkpoints V2 y una voz con
    ``reference_path``.

    Nota de fidelidad: OpenVoice aproxima el timbre. No reproduce con exactitud
    el acento, la prosodia ni la expresividad de la persona de referencia; eso
    solo puede medirse con pruebas auditivas controladas.
    """

    name = "openvoice"
    version = "openvoice-v2"

    def __init__(self, cfg: TTSConfig):
        super().__init__(cfg)
        self._model = None
        self._converter = None
        self._embeddings: Dict[str, Any] = {}
        self._source_embedding_cache: Optional[Any] = None

    # -- carga -------------------------------------------------------------
    def _checkpoint_dir(self) -> str:
        return os.path.join(self.cfg.openvoice_model_dir, "checkpoints_v2")

    def _build_converter(self, config_path: str):
        """Construye el conversor de timbre de OpenVoice.

        OpenVoice V2 intenta cargar ``wavmark`` (el watermark antiuso) al
        construir el conversor. Si el paquete no esta, la construccion falla;
        si esta, su modelo se descarga en runtime. Para no depender de una
        descarga en tiempo de ejecucion, se construye sin watermark y se
        deja el atributo a None (la conversion no lo necesita).
        """
        import sys
        import types

        from openvoice.api import ToneColorConverter

        class _NullWatermark:
            def to(self, device):  # imita la API de torch
                return self

        if "wavmark" not in sys.modules:
            try:
                import wavmark  # noqa: F401
            except ImportError:
                stub = types.ModuleType("wavmark")
                stub.load_model = lambda *args, **kwargs: _NullWatermark()
                sys.modules["wavmark"] = stub

        converter = ToneColorConverter(config_path, device=self.cfg.device)
        # Sin modelo de watermark: el audio sale sin marca de agua.
        converter.watermark_model = None
        return converter

    def _load(self) -> None:
        if self._converter is not None:
            return
        started = time.time()

        config_path = os.path.join(self._checkpoint_dir(), "config.json")
        checkpoint_path = os.path.join(self._checkpoint_dir(), "checkpoint.pth")
        if not os.path.isfile(config_path) or not os.path.isfile(checkpoint_path):
            self._load_error = (
                "Faltan los checkpoints de OpenVoice en %s (ejecuta scripts/prepare_tts_models.sh)"
                % self._checkpoint_dir()
            )
            raise EngineError("engine_not_prepared", self._load_error, retryable=False)

        try:
            from openvoice.api import ToneColorConverter
        except ImportError as exc:
            self._load_error = "OpenVoice no esta instalado: %s" % exc
            raise EngineError("engine_missing", self._load_error, retryable=False) from exc

        try:
            self._converter = self._build_converter(config_path)
            self._converter.load_ckpt(checkpoint_path)
        except Exception as exc:
            self._load_error = "No se pudo cargar OpenVoice: %s" % exc
            raise EngineError("engine_load_failed", self._load_error, retryable=True) from exc

        self._loaded = True
        self._load_error = None
        self._load_seconds = time.time() - started

    def ensure_loaded(self) -> None:
        if not self._loaded:
            self._load()

    # -- API ---------------------------------------------------------------
    def available(self) -> bool:
        return self._converter is not None

    def health(self) -> Dict[str, Any]:
        info = super().health()
        info["device"] = self.cfg.device
        info["checkpoints"] = self._checkpoint_dir()
        info["checkpoints_present"] = os.path.isfile(os.path.join(self._checkpoint_dir(), "config.json"))
        return info

    def _load_embedding(self, path: str):
        """Carga un embedding de timbre precalculado (.pth)."""
        try:
            import torch
        except ImportError as exc:  # pragma: no cover
            raise EngineError("engine_missing", "PyTorch no esta instalado", retryable=False) from exc
        try:
            return torch.load(path, map_location="cpu")
        except Exception as exc:
            raise EngineError(
                "voice_embedding_invalid",
                "No se pudo cargar el embedding de timbre: %s" % exc,
                retryable=False,
            ) from exc

    def _speaker_embedding(self, voice: VoiceProfile):
        """Embedding de timbre de la voz objetivo.

        Puede venir precalculado (voces base de OpenVoice) o extraerse de un
        audio de referencia (clones propios).
        """
        if voice.embedding_path and os.path.isfile(voice.embedding_path):
            return self._load_embedding(voice.embedding_path)

        if not voice.reference_path:
            raise EngineError(
                "voice_without_reference",
                "La voz '%s' no tiene audio de referencia" % voice.slug,
                retryable=False,
            )
        if not os.path.isfile(voice.reference_path):
            raise EngineError(
                "voice_reference_missing",
                "El audio de referencia de la voz '%s' no existe" % voice.slug,
                retryable=False,
            )

        key = "%s:%s" % (voice.slug, voice.reference_path)
        cached = self._embeddings.get(key)
        if cached is not None:
            return cached

        self.ensure_loaded()
        try:
            embedding = self._converter.extract_se(voice.reference_path)
        except Exception as exc:
            raise EngineError(
                "voice_extract_failed",
                "No se pudo extraer el timbre de la voz '%s': %s" % (voice.slug, exc),
                retryable=False,
            ) from exc

        self._embeddings[key] = embedding
        return embedding

    def _source_embedding(self):
        """Embedding de la voz base que genera el audio (para la conversion).

        OpenVoice V2 publica el embedding de su voz base por idioma; usarlo
        como origen hace que la conversion sea fiel. Si no esta, se extrae del
        propio audio generado.
        """
        if self._source_embedding_cache is not None:
            return self._source_embedding_cache

        candidates = []
        if self.cfg.openvoice_source_embedding:
            candidates.append(self.cfg.openvoice_source_embedding)
        base_dir = os.path.join(self.cfg.openvoice_model_dir, "base_speakers")
        for name in ("es.pth", "en-default.pth"):
            candidates.append(os.path.join(base_dir, name))

        for path in candidates:
            if path and os.path.isfile(path):
                self._source_embedding_cache = self._load_embedding(path)
                return self._source_embedding_cache

        return None

    def synthesize(
        self,
        text: str,
        voice: VoiceProfile,
        output_path: str,
        params: Optional[Dict[str, Any]] = None,
    ) -> SynthesisResult:
        params = params or {}
        self._validate_text(text)

        # La voz se comprueba ANTES de cargar el modelo: una voz sin audio de
        # referencia es un error de configuracion y no debe costar una carga
        # de pesos.
        has_reference = bool(voice.reference_path) and os.path.isfile(voice.reference_path or "")
        has_embedding = bool(voice.embedding_path) and os.path.isfile(voice.embedding_path or "")
        if not has_reference and not has_embedding:
            raise EngineError(
                "voice_without_reference",
                "La voz '%s' no tiene audio de referencia ni embedding de timbre" % voice.slug,
                retryable=False,
            )

        base_engine = MeloTTSEngine(self.cfg)
        base_engine.ensure_loaded()
        # El conversor se carga aqui y no solo al arrancar: el servicio solo
        # precarga el motor por defecto, asi que OpenVoice puede llegar a una
        # sintesis sin haber cargado nunca.
        self.ensure_loaded()

        self._prepare_output(output_path)
        tmp_base = output_path + ".base.wav"
        tau = float(params.get("tau", 0.3) or 0.3)
        speed = float(params.get("speed", self.cfg.default_speed) or 1.0)
        started = time.time()

        try:
            base_engine.synthesize(
                text,
                voice,
                tmp_base,
                {"speed": speed, "speaker_id": voice.speaker_id or self.cfg.melo_speaker_id},
            )
            target_embedding = self._speaker_embedding(voice)

            # Embedding de la voz base que genero el audio. Si la voz objetivo
            # es la propia voz base, la conversion es identidad.
            source_embedding = self._source_embedding()
            if source_embedding is None:
                source_embedding = target_embedding

            self._converter.convert(
                tmp_base,
                source_embedding,
                target_embedding,
                output_path=output_path,
                tau=tau,
            )
        except EngineError:
            self._cleanup(tmp_base)
            self._cleanup(output_path)
            raise
        except Exception as exc:
            self._cleanup(tmp_base)
            self._cleanup(output_path)
            raise EngineError("synthesis_failed", "OpenVoice no pudo sintetizar: %s" % exc, retryable=True) from exc
        finally:
            self._cleanup(tmp_base)

        meta = probe_audio(output_path)
        return SynthesisResult(
            path=output_path,
            duration_seconds=meta["duration_seconds"],
            sample_rate=meta["sample_rate"],
            engine=self.name,
            engine_version=self.version,
            parameters={"tau": tau, "speed": speed, "reference": voice.slug},
            elapsed_seconds=time.time() - started,
        )


# ---------------------------------------------------------------------------
# Chatterbox
# ---------------------------------------------------------------------------

# Idiomas que declara Chatterbox Multilingual (chatterbox.mtl_tts).
# Se mantienen aqui para poder validar el idioma ANTES de importar el paquete:
# importar chatterbox arrastra transformers y cuesta unos segundos.
CHATTERBOX_LANGUAGES = frozenset(
    ("ar", "da", "de", "el", "en", "es", "fi", "fr", "he", "hi", "it", "ja", "ko",
     "ms", "nl", "no", "pl", "pt", "ru", "sv", "sw", "tr", "zh")
)

# Nombre de fichero del peso T3 por alias, por si el paquete aun no esta
# instalado y no se puede consultar su propio resolutor.
CHATTERBOX_T3_FILES = {
    "v2": "t3_mtl23ls_v2.safetensors",
    "t3_mtl23ls_v2": "t3_mtl23ls_v2.safetensors",
    "v3": "t3_mtl23ls_v3.safetensors",
    "t3_mtl23ls_v3": "t3_mtl23ls_v3.safetensors",
}


class ChatterboxEngine(VoiceEngine):
    """Sintesis con Chatterbox Multilingual (Resemble AI, licencia MIT).

    Chatterbox sintetiza y clona en un solo paso: no hay un motor base
    intermedio como en ``openvoice``. La voz sale de un audio de referencia
    (``reference_path``) o, si la voz no lo tiene, de la voz por defecto que
    trae el propio modelo (``conds.pt``).

    Diferencias respecto a los otros motores, todas deliberadas:

    - **Salida a 24 kHz** y con la marca de agua PerTh que Chatterbox aplica
      siempre. No se puede desactivar: es el mecanismo de trazabilidad del
      modelo.
    - **``speed`` no es nativo**. Si la locucion pide otra velocidad se estira
      con el phase vocoder de librosa (conserva el tono).
    - **23 idiomas** seleccionados por ``language_id``; el idioma de la voz
      decide la tokenizacion, no el del guion.

    El modelo es una sola instancia compartida y su estado (``conds``) muta al
    clonar, por lo que la sintesis siempre va bajo el candado del servicio.
    """

    name = "chatterbox"
    version = "chatterbox-multilingual"

    def __init__(self, cfg: TTSConfig):
        super().__init__(cfg)
        self._model = None
        self._builtin_conds = None
        self._languages = set(CHATTERBOX_LANGUAGES)

    # -- carga -------------------------------------------------------------
    def _checkpoint_dir(self) -> str:
        return self.cfg.chatterbox_model_dir

    def _t3_filename(self) -> str:
        """Nombre real del peso T3 elegido en la configuracion."""
        requested = (self.cfg.chatterbox_t3_model or "").strip()
        if requested.endswith(".safetensors"):
            return requested
        # El paquete expone su propio resolutor; el mapa local es el respaldo
        # para cuando chatterbox no esta instalado todavia.
        try:
            from chatterbox import mtl_tts
        except ImportError:
            return CHATTERBOX_T3_FILES.get(requested, "")
        resolver = getattr(mtl_tts, "_resolve_multilingual_t3_model", None)
        if resolver is None:
            return CHATTERBOX_T3_FILES.get(requested, "")
        try:
            return resolver(requested or None)
        except ValueError as exc:
            raise EngineError(
                "engine_config_invalid",
                "TTS_CHATTERBOX_T3_MODEL no es un peso valido (%s): %s" % (requested, exc),
                retryable=False,
            ) from exc

    def _missing_weights(self) -> List[str]:
        """Pesos que ``from_local`` necesita y no estan en disco."""
        directory = self._checkpoint_dir()
        required = ["ve.pt", "s3gen.pt", "conds.pt", "grapheme_mtl_merged_expanded_v1.json"]
        try:
            t3_filename = self._t3_filename()
        except EngineError:
            raise
        if t3_filename:
            required.append(t3_filename)
        else:
            required.append("%s (alias '%s')" % ("t3_mtl23ls_*.safetensors", self.cfg.chatterbox_t3_model))
        return [name for name in required if not os.path.isfile(os.path.join(directory, name))]

    def _load(self) -> None:
        if self._model is not None:
            return
        started = time.time()

        # Se comprueba el disco ANTES de importar el paquete: la ruta habitual
        # de fallo en produccion es "olvide make tts-models".
        missing = self._missing_weights()
        if missing:
            self._load_error = (
                "Faltan los pesos de Chatterbox en %s (%s). Ejecuta scripts/prepare_tts_models.sh"
                % (self._checkpoint_dir(), ", ".join(missing))
            )
            raise EngineError("engine_not_prepared", self._load_error, retryable=False)

        try:
            import torch
            from chatterbox.mtl_tts import ChatterboxMultilingualTTS
        except ImportError as exc:
            self._load_error = "Chatterbox no esta instalado: %s" % exc
            raise EngineError("engine_missing", self._load_error, retryable=False) from exc

        try:
            torch.set_num_threads(max(1, int(self.cfg.torch_threads)))
        except Exception:  # pragma: no cover - depende de la build de torch
            pass

        try:
            self._model = ChatterboxMultilingualTTS.from_local(
                self._checkpoint_dir(),
                self.cfg.device,
                t3_model=self.cfg.chatterbox_t3_model,
            )
        except Exception as exc:
            self._model = None
            self._load_error = "No se pudo cargar Chatterbox: %s" % exc
            raise EngineError("engine_load_failed", self._load_error, retryable=True) from exc

        if self._model.conds is None:
            # generate() sin audio de referencia hace assert sobre conds: sin
            # conds.pt no hay voz por defecto y solo valdria la clonacion.
            self._model = None
            self._load_error = "Falta conds.pt (la voz por defecto de Chatterbox) en %s" % self._checkpoint_dir()
            raise EngineError("engine_not_prepared", self._load_error, retryable=False)

        # prepare_conditionals() sustituye model.conds al clonar una voz. Esta
        # copia de la voz por defecto se guarda para poder volver a ella cuando
        # la siguiente locucion no traiga referencia.
        self._builtin_conds = self._model.conds
        try:
            self._languages = set(ChatterboxMultilingualTTS.get_supported_languages())
        except Exception:  # pragma: no cover - defensa ante cambios de la API
            pass

        self._loaded = True
        self._load_error = None
        self._load_seconds = time.time() - started

    def ensure_loaded(self) -> None:
        if not self._loaded:
            self._load()

    # -- API ---------------------------------------------------------------
    def available(self) -> bool:
        return self._model is not None

    def health(self) -> Dict[str, Any]:
        info = super().health()
        info["device"] = self.cfg.device
        info["checkpoints"] = self._checkpoint_dir()
        info["checkpoints_present"] = not self._missing_weights()
        info["t3_model"] = self.cfg.chatterbox_t3_model
        info["languages"] = len(self._languages)
        info["sample_rate"] = self._model.sr if self._model is not None else None
        return info

    def speakers(self) -> List[Dict[str, Any]]:
        # Chatterbox no expone hablantes: tiene una voz por defecto y clona
        # cualquier referencia. Las voces del motor se declaran en
        # tts_voices, no aqui.
        return []

    # -- validaciones ------------------------------------------------------
    @staticmethod
    def _normalize_language(value: Optional[str]) -> Optional[str]:
        """Reduce un codigo de idioma al formato de Chatterbox ('es', 'en')."""
        if not value:
            return None
        code = str(value).strip().lower().replace("_", "-")
        if not code:
            return None
        base = code.split("-")[0]
        return base if base in CHATTERBOX_LANGUAGES else None

    def _resolve_language(self, voice: VoiceProfile, params: Dict[str, Any]) -> str:
        """Idioma de la locucion, validado antes de cargar los pesos."""
        requested = params.get("language_id") or voice.language or self.cfg.chatterbox_language
        code = self._normalize_language(requested)
        if code is None:
            supported = ", ".join(sorted(CHATTERBOX_LANGUAGES))
            raise EngineError(
                "language_unsupported",
                "Chatterbox no soporta el idioma '%s' de la voz '%s'. Soportados: %s"
                % (requested, voice.slug, supported),
                retryable=False,
            )
        return code

    def _resolve_reference(self, voice: VoiceProfile) -> Optional[str]:
        """Audio de referencia a clonar, o None para usar la voz por defecto."""
        if voice.reference_path:
            if not os.path.isfile(voice.reference_path):
                raise EngineError(
                    "voice_reference_missing",
                    "El audio de referencia de la voz '%s' no existe" % voice.slug,
                    retryable=False,
                )
            return voice.reference_path

        if voice.embedding_path:
            # Un .pth de OpenVoice es un timbre ya calculado; Chatterbox solo
            # sabe trabajar con audio. Fallar en silencio aqui dejaria la
            # narracion con la voz por defecto sin que nadie lo notara.
            raise EngineError(
                "voice_embedding_unsupported",
                "La voz '%s' usa un embedding de timbre de OpenVoice y Chatterbox "
                "solo puede clonar con audio de referencia" % voice.slug,
                retryable=False,
            )

        return None

    # -- sintesis ----------------------------------------------------------
    def synthesize(
        self,
        text: str,
        voice: VoiceProfile,
        output_path: str,
        params: Optional[Dict[str, Any]] = None,
    ) -> SynthesisResult:
        params = params or {}
        self._validate_text(text)

        # Todo lo que puede fallar por configuracion se resuelve ANTES de
        # cargar ~2 GB de pesos.
        language = self._resolve_language(voice, params)
        reference = self._resolve_reference(voice)

        self.ensure_loaded()
        self._prepare_output(output_path)

        speed = float(params.get("speed", self.cfg.default_speed) or 1.0)
        exaggeration = float(params.get("exaggeration", self.cfg.chatterbox_exaggeration) or 0.5)
        cfg_weight = float(params.get("cfg_weight", self.cfg.chatterbox_cfg_weight) or 0.5)
        temperature = float(params.get("temperature", self.cfg.chatterbox_temperature) or 0.8)
        repetition_penalty = float(
            params.get("repetition_penalty", self.cfg.chatterbox_repetition_penalty) or 1.2
        )
        started = time.time()

        try:
            generate_kwargs: Dict[str, Any] = {}
            if reference:
                generate_kwargs["audio_prompt_path"] = reference
            else:
                # Sin referencia se usa la voz por defecto. Si una tarea
                # anterior dejo su clon en model.conds, se restaura aqui: no
                # hay estado que distinga una voz por defecto de un clon.
                self._model.conds = self._builtin_conds

            wav = self._model.generate(
                text,
                language_id=language,
                exaggeration=exaggeration,
                cfg_weight=cfg_weight,
                temperature=temperature,
                repetition_penalty=repetition_penalty,
                **generate_kwargs,
            )
            data = wav.squeeze(0).detach().cpu().numpy()
            sample_rate = int(self._model.sr)

            if abs(speed - 1.0) > 1e-3:
                import librosa

                # Chatterbox no controla la velocidad. El phase vocoder la
                # cambia sin alterar el tono de la voz.
                data = librosa.effects.time_stretch(data.astype("float32", copy=False), rate=speed)

            import soundfile

            soundfile.write(output_path, data, sample_rate, subtype="PCM_16")
        except EngineError:
            self._cleanup(output_path)
            raise
        except Exception as exc:
            self._cleanup(output_path)
            raise EngineError("synthesis_failed", "Chatterbox no pudo sintetizar: %s" % exc, retryable=True) from exc

        meta = probe_audio(output_path)
        return SynthesisResult(
            path=output_path,
            duration_seconds=meta["duration_seconds"],
            sample_rate=meta["sample_rate"],
            engine=self.name,
            engine_version="%s-%s" % (self.version, self.cfg.chatterbox_t3_model),
            parameters={
                "language_id": language,
                "speed": speed,
                "exaggeration": exaggeration,
                "cfg_weight": cfg_weight,
                "reference": voice.slug,
                "cloned": bool(reference),
            },
            elapsed_seconds=time.time() - started,
        )


# ---------------------------------------------------------------------------
# Registro de motores
# ---------------------------------------------------------------------------

ENGINE_CLASSES = {
    MeloTTSEngine.name: MeloTTSEngine,
    OpenVoiceEngine.name: OpenVoiceEngine,
    ChatterboxEngine.name: ChatterboxEngine,
}


def build_engines(cfg: TTSConfig) -> Dict[str, VoiceEngine]:
    """Instancia los motores habilitados en la configuracion."""
    engines: Dict[str, VoiceEngine] = {}
    for name in cfg.engines:
        engine_class = ENGINE_CLASSES.get(name)
        if engine_class is None:
            # Un motor desconocido no se ignora en silencio: el servicio lo
            # reporta en /health para que se note el error de configuracion.
            continue
        engines[name] = engine_class(cfg)
    return engines


def resolve_engine(engines: Dict[str, VoiceEngine], name: str) -> VoiceEngine:
    engine = engines.get(name)
    if engine is None:
        raise EngineError("unknown_engine", "Motor de voz desconocido: %s" % name, retryable=False)
    return engine
