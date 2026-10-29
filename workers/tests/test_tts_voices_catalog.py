"""Pruebas del catalogo de voces y de los hablantes del modelo.

Cubren:
- El numero de hablantes del modelo (256 en el modelo español).
- Las voces con embedding de timbre precalculado (no necesitan audio).
- El registro bajo demanda de un hablante del modelo.
"""

from __future__ import annotations

import os

import pytest

from workers.tts.config import TTSConfig
from workers.tts.engine import MeloTTSEngine, OpenVoiceEngine, VoiceProfile


class _FakeHps:
    class data:  # noqa: N801 - imita la estructura de los hparams
        n_speakers = 256
        sampling_rate = 22050


class _FakeModel:
    hps = _FakeHps()


def test_model_speaker_count_is_the_real_one():
    """El modelo en espanol tiene 256 hablantes: cada uno es una voz."""
    engine = MeloTTSEngine(TTSConfig())
    engine._model = _FakeModel()

    speakers = engine.speakers()
    assert len(speakers) == 256
    assert speakers[0]["speaker_id"] == "0"
    assert speakers[255]["speaker_id"] == "255"
    assert all(s["builtin"] for s in speakers)


def test_speaker_count_is_zero_when_the_model_is_not_available():
    """Si el modelo no se puede cargar, no hay hablantes que ofrecer."""
    from workers.tts.engine import EngineError

    engine = MeloTTSEngine(TTSConfig())

    def _fail():
        raise EngineError("engine_load_failed", "sin modelo", retryable=True)

    engine._load = _fail
    assert engine.speakers() == []


def test_voice_with_precomputed_embedding_needs_no_reference_audio(tmp_path):
    """Una voz con .pth no necesita audio de referencia: se carga el embedding."""
    torch = pytest.importorskip("torch")

    embedding_file = tmp_path / "es.pth"
    expected = torch.ones(1, 256, 1)
    torch.save(expected, embedding_file)

    engine = OpenVoiceEngine(TTSConfig())
    voice = VoiceProfile(slug="ov-es", embedding_path=str(embedding_file))

    # No hay converter cargado: si intentara extraer el timbre de un audio
    # fallaria. Con el embedding precalculado, funciona.
    loaded = engine._speaker_embedding(voice)
    assert loaded is not None
    assert tuple(loaded.shape) == (1, 256, 1)


def test_voice_without_reference_or_embedding_fails_fast(tmp_path):
    engine = OpenVoiceEngine(TTSConfig())
    voice = VoiceProfile(slug="sin-nada")

    from workers.tts.engine import EngineError

    with pytest.raises(EngineError) as excinfo:
        engine._speaker_embedding(voice)
    assert excinfo.value.code == "voice_without_reference"


def test_missing_embedding_file_is_reported(tmp_path):
    engine = OpenVoiceEngine(TTSConfig())
    voice = VoiceProfile(slug="ov-fantasma", embedding_path=str(tmp_path / "no-existe.pth"))

    from workers.tts.engine import EngineError

    with pytest.raises(EngineError) as excinfo:
        engine._speaker_embedding(voice)
    assert excinfo.value.code == "voice_without_reference"


def test_openvoice_needs_the_checkpoints():
    """Sin checkpoints el motor lo dice con un codigo claro."""
    cfg = TTSConfig()
    cfg.openvoice_model_dir = "/ruta/inexistente"
    engine = OpenVoiceEngine(cfg)

    from workers.tts.engine import EngineError

    with pytest.raises(EngineError) as excinfo:
        engine.ensure_loaded()
    assert excinfo.value.code == "engine_not_prepared"
    assert not excinfo.value.retryable


def test_synthetize_loads_the_converter_on_demand(monkeypatch, tmp_path):
    """El conversor se carga en la propia sintetizacion, no solo al arrancar.

    El servicio solo precarga el motor por defecto: si OpenVoice no pidiera
    la carga aqui, la primera narracion fallaria con
    'NoneType' object has no attribute 'convert'.
    """
    torch = pytest.importorskip("torch")

    import soundfile

    from workers.tts.engine import MeloTTSEngine

    cfg = TTSConfig()
    cfg.openvoice_model_dir = str(tmp_path)
    engine = OpenVoiceEngine(cfg)

    # Embedding de la voz que simula la conversion.
    embedding_file = tmp_path / "es.pth"
    torch.save(torch.ones(1, 256, 1), embedding_file)

    # El modelo base no hace falta: se sustituye por un doble que escribe un
    # WAV valido en la ruta que le pidan.
    def _fake_base(self, text, voice, output_path, params=None, **kwargs):
        soundfile.write(output_path, [0.0] * 100, 22050)

    monkeypatch.setattr(MeloTTSEngine, "ensure_loaded", lambda self: None)
    monkeypatch.setattr(MeloTTSEngine, "synthesize", _fake_base)

    # El conversor tambien es un doble: lo que se comprueba es que el motor
    # lo carga (o intenta) antes de usarlo.
    class _FakeConverter:
        def __init__(self):
            self.converted = False

        def convert(self, source_path, source_embedding, target_embedding,
                    output_path=None, tau=0.3):
            self.converted = True
            soundfile.write(output_path, [0.0] * 100, 22050)

    fake = _FakeConverter()
    monkeypatch.setattr(engine, "_load", lambda: (setattr(engine, "_converter", fake),
                                                  setattr(engine, "_loaded", True)))

    voice = VoiceProfile(slug="ov-es", embedding_path=str(embedding_file))
    result = engine.synthesize("hola", voice, str(tmp_path / "out.wav"))

    assert engine._converter is fake
    assert fake.converted
    assert result.sample_rate == 22050
