"""Pruebas del noticiario: concatenacion de varias narraciones.

Comprueban que la pieza final respeta el orden, los silencios y el manifiesto
(offsets) que permite mapear el audio sobre las noticias de origen.
"""

from __future__ import annotations

import os

import numpy as np
import pytest
import soundfile

from workers.tts.bulletin import DEFAULT_SILENCE_MS, concatenate_wavs
from workers.tts.engine import EngineError


def write_tone(path: str, seconds: float, value: float, sample_rate: int = 22050) -> str:
    """Escribe un WAV con un tono constante (para poder identificarlo)."""
    samples = np.full(int(seconds * sample_rate), value, dtype=np.float32)
    os.makedirs(os.path.dirname(path), exist_ok=True)
    soundfile.write(path, samples, sample_rate)
    return path


def test_concatenate_two_files(tmp_path):
    first = write_tone(str(tmp_path / "a.wav"), 1.0, 0.5)
    second = write_tone(str(tmp_path / "b.wav"), 2.0, -0.5)
    output = str(tmp_path / "out" / "bulletin.wav")

    result = concatenate_wavs(
        [{"path": first, "script_id": 1}, {"path": second, "script_id": 2}],
        output,
        silence_ms=0,
    )

    assert os.path.isfile(output)
    assert result["duration_seconds"] == pytest.approx(3.0, abs=0.05)
    assert result["size_bytes"] > 0
    assert len(result["items"]) == 2


def test_silence_is_inserted_between_items(tmp_path):
    first = write_tone(str(tmp_path / "a.wav"), 1.0, 0.5)
    second = write_tone(str(tmp_path / "b.wav"), 1.0, -0.5)
    output = str(tmp_path / "bulletin.wav")

    result = concatenate_wavs(
        [{"path": first, "script_id": 1}, {"path": second, "script_id": 2}],
        output,
        silence_ms=1000,
    )

    # 1 s + 1 s de silencio + 1 s
    assert result["duration_seconds"] == pytest.approx(3.0, abs=0.05)


def test_no_silence_after_the_last_item(tmp_path):
    first = write_tone(str(tmp_path / "a.wav"), 1.0, 0.5)
    second = write_tone(str(tmp_path / "b.wav"), 1.0, -0.5)
    output = str(tmp_path / "bulletin.wav")

    result = concatenate_wavs(
        [{"path": first, "script_id": 1}, {"path": second, "script_id": 2}],
        output,
        silence_ms=2000,
    )

    assert result["duration_seconds"] == pytest.approx(4.0, abs=0.05)


def test_manifest_offsets_point_to_each_item(tmp_path):
    first = write_tone(str(tmp_path / "a.wav"), 1.0, 0.5)
    second = write_tone(str(tmp_path / "b.wav"), 2.0, -0.5)
    output = str(tmp_path / "bulletin.wav")

    result = concatenate_wavs(
        [{"path": first, "script_id": 11}, {"path": second, "script_id": 22}],
        output,
        silence_ms=500,
    )

    items = result["items"]
    assert items[0]["script_id"] == 11
    assert items[0]["start_offset"] == pytest.approx(0.0, abs=0.01)
    assert items[0]["duration_seconds"] == pytest.approx(1.0, abs=0.05)

    # El segundo empieza tras el primero mas la pausa.
    assert items[1]["start_offset"] == pytest.approx(1.5, abs=0.05)
    assert items[1]["script_id"] == 22

    # El offset permite leer el tramo correcto del audio final.
    audio, rate = soundfile.read(output)
    sample = audio[int(items[1]["start_offset"] * rate)]
    assert sample < 0  # el segundo item es el tono negativo


def test_empty_bulletin_is_rejected(tmp_path):
    with pytest.raises(EngineError) as excinfo:
        concatenate_wavs([], str(tmp_path / "out.wav"))
    assert excinfo.value.code == "empty_bulletin"


def test_missing_item_file_is_rejected(tmp_path):
    output = str(tmp_path / "out.wav")
    with pytest.raises(EngineError) as excinfo:
        concatenate_wavs([{"path": str(tmp_path / "no-existe.wav")}], output)
    assert excinfo.value.code == "bulletin_item_missing"


def test_output_directory_is_created(tmp_path):
    first = write_tone(str(tmp_path / "a.wav"), 0.5, 0.5)
    output = str(tmp_path / "deep" / "dir" / "bulletin.wav")
    concatenate_wavs([{"path": first}], output)
    assert os.path.isfile(output)


def test_default_silence_is_reasonable():
    assert 0 < DEFAULT_SILENCE_MS <= 2000


def test_items_with_different_sample_rates_are_resampled(tmp_path):
    """Chatterbox sale a 24 kHz y MeloTTS/OpenVoice a 22,05 kHz.

    Un noticiario que mezcle voces de dos motores no puede escribir los
    frames sin remuestrear: saldria a trompicones.
    """
    chatterbox = write_tone(str(tmp_path / "a.wav"), 1.0, 0.5, sample_rate=24000)
    melo = write_tone(str(tmp_path / "b.wav"), 1.0, -0.5, sample_rate=22050)
    output = str(tmp_path / "bulletin.wav")

    result = concatenate_wavs(
        [{"path": chatterbox, "script_id": 1}, {"path": melo, "script_id": 2}],
        output,
        silence_ms=500,
    )

    # La frecuencia la manda el primer item; el segundo se remuestrea.
    assert result["sample_rate"] == 24000
    # 1 s + 0,5 s de silencio + 1 s remuestreado.
    assert result["duration_seconds"] == pytest.approx(2.5, abs=0.1)

    audio, sample_rate = soundfile.read(output)
    assert sample_rate == 24000
    # El item remuestreado conserva su amplitud (no se corrompe).
    assert np.abs(audio).max() == pytest.approx(0.5, abs=0.05)
