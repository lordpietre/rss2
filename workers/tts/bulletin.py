"""Noticiario: concatenacion ordenada de varias narraciones.

Cada item es el WAV de una noticia ya narrada. El noticiario las une en un
unico archivo, con una pausa configurable entre bloques, y devuelve el
manifiesto (offset de inicio y duracion de cada item) para que el audio
pueda mapearse sobre las noticias de origen.

La concatenacion se hace por bloques: no se carga el noticiario entero en
memoria (un bloque de 5 noticias puede pesar cientos de MB).
"""

from __future__ import annotations

import logging
import os
from typing import Any, Dict, List

from .engine import EngineError

log = logging.getLogger("tts.bulletin")

# Silencio por defecto entre noticias (ms).
DEFAULT_SILENCE_MS = 800
# Bloque de lectura del WAV (frames). 44100 frames ~ 1 s a 44,1 kHz.
BLOCK_FRAMES = 44100


def _resample(block: Any, source_rate: int, target_rate: int) -> Any:
    """Remuestrea un bloque de audio (frames, canales) a otra frecuencia.

    Los motores no salen todos a la misma frecuencia: Chatterbox genera a
    24 kHz y MeloTTS/OpenVoice a 22,05 kHz. Sin remuestrear, un noticiario
    que mezcle voces de dos motores saldria a trompicones.
    """
    import numpy as np

    try:
        import librosa
    except Exception as exc:  # pragma: no cover - defensivo
        raise EngineError(
            "bulletin_resample_failed",
            "No se pudo remuestrear una pieza del noticiario: %s" % exc,
            retryable=False,
        ) from exc

    # librosa trabaja en mono o en 2D (canales, frames): se transpone.
    resampled = librosa.resample(
        block.T, orig_sr=source_rate, target_sr=target_rate
    )
    return np.ascontiguousarray(resampled.T, dtype="float32")


def concatenate_wavs(
    items: List[Dict[str, Any]],
    output_path: str,
    silence_ms: int = DEFAULT_SILENCE_MS,
) -> Dict[str, Any]:
    """Une varios WAV en uno solo.

    ``items`` es una lista de diccionarios con ``path`` y, opcionalmente,
    ``duration_seconds`` y ``script_id``.

    Devuelve ``duration_seconds``, ``sample_rate``, ``size_bytes`` y la lista
    de items con su offset de inicio.
    """
    import numpy as np
    import soundfile

    if not items:
        raise EngineError("empty_bulletin", "El noticiario no tiene narraciones", retryable=False)

    directory = os.path.dirname(os.path.abspath(output_path))
    if directory:
        os.makedirs(directory, exist_ok=True)

    sample_rate = None
    channels = None
    manifest: List[Dict[str, Any]] = []
    total_frames = 0

    # La frecuencia de muestreo se toma del primer item: hay que conocerla
    # antes de abrir el fichero de salida.
    for position, item in enumerate(items):
        path = item.get("path")
        if not path or not os.path.isfile(path):
            raise EngineError(
                "bulletin_item_missing",
                "Falta el audio del item %s: %s" % (position + 1, path),
                retryable=False,
            )
        with soundfile.SoundFile(path, mode="r") as source:
            sample_rate = int(source.samplerate)
            channels = int(source.channels)
        break

    with soundfile.SoundFile(
        output_path,
        mode="w",
        format="WAV",
        subtype="PCM_16",
        samplerate=sample_rate,
        channels=channels,
    ) as out:

        for position, item in enumerate(items):
            path = item.get("path")
            if not path or not os.path.isfile(path):
                raise EngineError(
                    "bulletin_item_missing",
                    "Falta el audio del item %s: %s" % (position + 1, path),
                    retryable=False,
                )

            with soundfile.SoundFile(path, mode="r") as source:
                item_rate = int(source.samplerate)
                start_offset = total_frames / float(sample_rate)

                if item_rate != sample_rate:
                    # Los motores no salen todos a la misma frecuencia
                    # (Chatterbox 24 kHz, MeloTTS/OpenVoice 22,05 kHz). Un
                    # noticiario que mezcle voces de dos motores debe
                    # remuestrear, o las piezas salen a trompicones.
                    log.info(
                        "El item %s tiene %d Hz y el noticiario %d Hz: se remuestrea",
                        position + 1,
                        item_rate,
                        sample_rate,
                    )
                    block = source.read(dtype="float32", always_2d=True)
                    block = _resample(block, item_rate, sample_rate)
                    out.write(block)
                    frames_read = block.shape[0]
                else:
                    frames_read = 0
                    while True:
                        block = source.read(BLOCK_FRAMES, dtype="float32", always_2d=True)
                        if len(block) == 0:
                            break
                        out.write(block)
                        frames_read += block.shape[0]

                manifest.append(
                    {
                        "script_id": item.get("script_id"),
                        "audio_id": item.get("audio_id"),
                        "position": position + 1,
                        "path": path,
                        "start_offset": round(start_offset, 3),
                        "duration_seconds": round(frames_read / float(sample_rate), 3),
                    }
                )
                total_frames += frames_read

            # Pausa entre noticias (no al final: el noticiario acaba con la
            # ultima noticia).
            if position < len(items) - 1 and silence_ms > 0:
                silence_frames = int(round(sample_rate * silence_ms / 1000.0))
                if silence_frames > 0:
                    silence = np.zeros((silence_frames, channels), dtype="float32")
                    out.write(silence)
                    total_frames += silence_frames

    size_bytes = os.path.getsize(output_path)
    return {
        "duration_seconds": round(total_frames / float(sample_rate), 3),
        "sample_rate": sample_rate,
        "channels": channels,
        "size_bytes": size_bytes,
        "items": manifest,
    }
