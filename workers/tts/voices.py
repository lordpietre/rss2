"""Voces: registro de perfiles de timbre y control de calidad de las muestras.

El servicio ofrece dos familias de voces:

- Voces del motor (integradas): no requieren audio de referencia.
- Clones propios: el usuario registra una muestra y se comprueba su calidad
  antes de poder usarla. Guardar y borrar muestras es siempre una accion
  explicita; el servicio nunca procesa grabaciones por su cuenta.
"""

from __future__ import annotations

import hashlib
import os
import re
import shutil
import tempfile
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional

_SLUG_RE = re.compile(r"[^a-z0-9_-]+")

MAX_SAMPLE_BYTES = 50 * 1024 * 1024  # 50 MB: suficiente para varios minutos de audio

# Formatos de muestra que el servicio sabe decodificar (libsndfile >= 1.1 los
# lee todos: WAV, MP3, OGG/Vorbis-Opus, FLAC, AIFF y AU). La extension manda
# porque soundfile detecta el formato a partir de ella.
SAMPLE_EXTENSIONS = (".wav", ".mp3", ".ogg", ".oga", ".opus", ".flac", ".aif", ".aiff", ".au")


class VoiceError(Exception):
    """Error de gestion de voces, con codigo estable."""

    def __init__(self, code: str, message: str):
        super().__init__(message)
        self.code = code
        self.message = message


@dataclass
class QualityReport:
    """Resultado del analisis de una muestra de voz."""

    ok: bool
    duration_seconds: float = 0.0
    sample_rate: int = 0
    channels: int = 0
    peak: float = 0.0
    clipping_ratio: float = 0.0
    issues: List[str] = field(default_factory=list)

    def to_dict(self) -> Dict[str, Any]:
        return {
            "ok": self.ok,
            "duration_seconds": round(self.duration_seconds, 3),
            "sample_rate": self.sample_rate,
            "channels": self.channels,
            "peak": round(self.peak, 4),
            "clipping_ratio": round(self.clipping_ratio, 6),
            "issues": list(self.issues),
        }


def slugify(value: str, fallback: str = "voz") -> str:
    """Normaliza un nombre a un slug seguro para nombres de fichero."""
    value = _SLUG_RE.sub("-", (value or "").strip().lower()).strip("-_")
    return value[:48] or fallback


def safe_join(base_dir: str, *parts: str) -> str:
    """Une ``parts`` dentro de ``base_dir`` garantizando que no se escapa.

    Es la proteccion contra path traversal: cualquier intento de salirse del
    directorio base con ``..`` o una ruta absoluta se resuelve dentro.
    """
    base = os.path.realpath(base_dir)
    target = os.path.realpath(os.path.join(base, *parts))
    if target != base and not target.startswith(base + os.sep):
        raise VoiceError("invalid_path", "Ruta fuera del directorio permitido")
    return target


def analyze_sample(
    path: str,
    min_seconds: float = 3.0,
    max_seconds: float = 60.0,
) -> QualityReport:
    """Comprueba que una muestra sea utilizable para clonar timbre.

    No juzga el timbre: solo comprueba que el archivo sea utilizable
    (decodificable, con duracion razonable, sin saturacion).
    """
    if not os.path.isfile(path):
        raise VoiceError("sample_missing", "El archivo de muestra no existe")

    try:
        import numpy as np
        import soundfile
    except ImportError as exc:  # pragma: no cover - dependencia del contenedor
        raise VoiceError("audio_tooling_missing", str(exc)) from exc

    try:
        data, sample_rate = soundfile.read(path, always_2d=True)
    except Exception as exc:
        raise VoiceError("sample_unreadable", "La muestra no se puede decodificar: %s" % exc) from exc

    frames = data.shape[0]
    channels = data.shape[1] if data.ndim > 1 else 1
    duration = frames / float(sample_rate) if sample_rate else 0.0

    mono = data.mean(axis=1) if channels > 1 else data.reshape(-1)
    peak = float(np.max(np.abs(mono))) if mono.size else 0.0
    clipping_ratio = float(np.mean(np.abs(mono) >= 0.99)) if mono.size else 0.0

    issues: List[str] = []
    if duration < min_seconds:
        issues.append("too_short")
    if duration > max_seconds:
        issues.append("too_long")
    if sample_rate < 22050:
        issues.append("low_sample_rate")
    if channels > 1:
        issues.append("not_mono")
    if peak < 0.05:
        issues.append("too_quiet")
    if clipping_ratio > 0.01:
        issues.append("clipping")

    return QualityReport(
        ok=not issues,
        duration_seconds=round(duration, 3),
        sample_rate=int(sample_rate),
        channels=int(channels),
        peak=peak,
        clipping_ratio=clipping_ratio,
        issues=issues,
    )


def sample_suffix(filename: Optional[str]) -> str:
    """Extension de la muestra, validada contra los formatos soportados.

    La extension es la que decide como la decodifica soundfile: un MP3
    guardado como ``.wav`` no se puede leer. Si el formato no es soportado
    se rechaza aqui, antes de escribir nada en disco.
    """
    ext = os.path.splitext(filename or "")[1].lower()
    if ext not in SAMPLE_EXTENSIONS:
        raise VoiceError(
            "unsupported_format",
            "Formato no soportado: %s. Usa WAV, MP3 u OGG (tambien FLAC, AIFF y AU)."
            % (ext or "sin extension"),
        )
    return ext


def store_sample(data: bytes, slug: str, voices_dir: str, suffix: str = ".wav") -> str:
    """Guarda una muestra en el directorio de voces y devuelve su ruta."""
    if not data:
        raise VoiceError("sample_empty", "La muestra esta vacia")
    if len(data) > MAX_SAMPLE_BYTES:
        raise VoiceError("sample_too_large", "La muestra supera el tamano maximo permitido")

    os.makedirs(voices_dir, exist_ok=True)
    filename = "%s%s" % (slugify(slug), suffix)
    target = safe_join(voices_dir, filename)

    # Se escribe primero en un temporal y se renombra: si el proceso muere a
    # mitad, no queda una muestra a medio escribir.
    handle, tmp_path = tempfile.mkstemp(dir=voices_dir, suffix=".part")
    try:
        with os.fdopen(handle, "wb") as tmp_file:
            tmp_file.write(data)
        os.replace(tmp_path, target)
    except Exception:
        if os.path.exists(tmp_path):
            os.remove(tmp_path)
        raise
    return target


def delete_sample(path: str, voices_dir: str) -> bool:
    """Borra una muestra. Solo dentro del directorio de voces."""
    if not path:
        return False
    target = safe_join(voices_dir, os.path.basename(path))
    if os.path.isfile(target):
        os.remove(target)
        return True
    return False


def sha256_file(path: str) -> str:
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for block in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def copy_builtin_sample(source: str, voices_dir: str, slug: str) -> Optional[str]:
    """Copia una muestra de referencia de las preinstaladas al directorio de voces."""
    if not source or not os.path.isfile(source):
        return None
    os.makedirs(voices_dir, exist_ok=True)
    target = safe_join(voices_dir, "%s%s" % (slugify(slug), os.path.splitext(source)[1] or ".wav"))
    shutil.copyfile(source, target)
    return target
