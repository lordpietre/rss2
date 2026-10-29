"""Pruebas del contrato de motor de voz.

No se descarga ningun modelo: el motor real se sustituye por un doble de
prueba que respeta la misma interfaz.
"""

from __future__ import annotations

import os

import numpy as np
import pytest

from workers.tts.config import TTSConfig
from workers.tts.engine import (
    EngineError,
    MeloTTSEngine,
    OpenVoiceEngine,
    SynthesisResult,
    VoiceEngine,
    VoiceProfile,
    build_engines,
    probe_audio,
    resolve_engine,
)


def write_wav(path: str, seconds: float = 1.0, sample_rate: int = 22050) -> str:
    import soundfile

    samples = np.zeros(int(seconds * sample_rate), dtype=np.float32)
    os.makedirs(os.path.dirname(path), exist_ok=True)
    soundfile.write(path, samples, sample_rate)
    return path


# ---------------------------------------------------------------------------
# 7. Archivo WAV corrupto o inexistente
# ---------------------------------------------------------------------------


def test_probe_audio_reads_a_valid_file(tmp_path):
    path = write_wav(str(tmp_path / "ok.wav"), seconds=1.5)
    meta = probe_audio(path)
    assert meta["duration_seconds"] == pytest.approx(1.5, abs=0.05)
    assert meta["sample_rate"] == 22050
    assert meta["channels"] == 1


def test_probe_audio_fails_on_missing_file(tmp_path):
    with pytest.raises(EngineError) as excinfo:
        probe_audio(str(tmp_path / "no-existe.wav"))
    assert excinfo.value.code == "audio_missing"
    assert not excinfo.value.retryable


def test_probe_audio_fails_on_corrupt_file(tmp_path):
    path = tmp_path / "roto.wav"
    path.write_bytes(b"esto no es un wav de verdad")
    with pytest.raises(EngineError) as excinfo:
        probe_audio(str(path))
    assert excinfo.value.code == "audio_unreadable"


def test_probe_audio_fails_on_empty_file(tmp_path):
    path = tmp_path / "vacio.wav"
    path.write_bytes(b"")
    with pytest.raises(EngineError) as excinfo:
        probe_audio(str(path))
    assert excinfo.value.code in ("audio_unreadable", "audio_missing")


# ---------------------------------------------------------------------------
# Contrato de la interfaz
# ---------------------------------------------------------------------------


class FakeEngine(VoiceEngine):
    """Motor de prueba: escribe un WAV valido sin tocar torch."""

    name = "fake"
    version = "fake-1"

    def __init__(self, cfg, fail_with=None):
        super().__init__(cfg)
        self.fail_with = fail_with
        self.calls = []

    def synthesize(self, text, voice, output_path, params=None):
        self.calls.append((text, voice, output_path, params))
        if self.fail_with:
            raise self.fail_with
        write_wav(output_path, seconds=0.5)
        return SynthesisResult(
            path=output_path,
            duration_seconds=0.5,
            sample_rate=22050,
            engine=self.name,
            engine_version=self.version,
            parameters=params or {},
            elapsed_seconds=0.01,
        )


def test_engine_interface_is_abstract():
    with pytest.raises(TypeError):
        VoiceEngine(TTSConfig())


def test_empty_text_is_rejected(tmp_path):
    """El guion vacio se rechaza antes de tocar el motor."""
    engine = MeloTTSEngine(TTSConfig())
    with pytest.raises(EngineError) as excinfo:
        engine.synthesize("   ", VoiceProfile(), str(tmp_path / "x.wav"))
    assert excinfo.value.code == "empty_text"


def test_build_engines_registers_the_configured_ones():
    cfg = TTSConfig()
    cfg.engines = ["melotts", "openvoice"]
    engines = build_engines(cfg)
    assert set(engines) == {"melotts", "openvoice"}
    assert isinstance(engines["melotts"], MeloTTSEngine)
    assert isinstance(engines["openvoice"], OpenVoiceEngine)


def test_unknown_engine_is_ignored_but_resolve_fails():
    cfg = TTSConfig()
    cfg.engines = ["motor-inexistente"]
    engines = build_engines(cfg)
    assert engines == {}
    with pytest.raises(EngineError) as excinfo:
        resolve_engine(engines, "motor-inexistente")
    assert excinfo.value.code == "unknown_engine"


def test_engine_health_is_serializable():
    cfg = TTSConfig()
    cfg.engines = ["melotts"]
    engines = build_engines(cfg)
    health = engines["melotts"].health()
    assert health["engine"] == "melotts"
    assert health["available"] is False
    assert "load_seconds" in health


def test_openvoice_reports_missing_checkpoints():
    """Sin checkpoints, el motor debe fallar con un codigo claro."""
    cfg = TTSConfig()
    cfg.openvoice_model_dir = "/ruta/que/no/existe"
    engine = OpenVoiceEngine(cfg)
    with pytest.raises(EngineError) as excinfo:
        engine.ensure_loaded()
    assert excinfo.value.code == "engine_not_prepared"
    assert not excinfo.value.retryable


def test_openvoice_requires_a_reference_audio(tmp_path):
    """Sin audio de referencia el error es inmediato y no carga el modelo."""
    cfg = TTSConfig()
    cfg.openvoice_model_dir = "/ruta/que/no/existe"
    engine = OpenVoiceEngine(cfg)
    with pytest.raises(EngineError) as excinfo:
        engine.synthesize("texto", VoiceProfile(slug="sin-referencia"), str(tmp_path / "x.wav"))
    assert excinfo.value.code == "voice_without_reference"
    assert not excinfo.value.retryable
