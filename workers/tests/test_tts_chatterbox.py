"""Pruebas del motor Chatterbox.

Ninguna prueba descarga pesos ni necesita el paquete ``chatterbox-tts``: el
modelo se sustituye por un doble que imita la parte de la API que usa el
motor (``generate`` y la voz por defecto de ``conds``).
"""

from __future__ import annotations

import os
import sys
import types

import pytest

from workers.tts.config import TTSConfig
from workers.tts.engine import (
    CHATTERBOX_LANGUAGES,
    ChatterboxEngine,
    EngineError,
    VoiceProfile,
    build_engines,
)


def write_wav(path: str, seconds: float = 0.3, sample_rate: int = 24000) -> str:
    import numpy as np
    import soundfile

    samples = np.zeros(int(seconds * sample_rate), dtype=np.float32)
    os.makedirs(os.path.dirname(path), exist_ok=True)
    soundfile.write(path, samples, sample_rate)
    return path


class FakeChatterbox:
    """Doble del modelo: imita ``generate`` y el estado ``conds``.

    En el modelo real, ``generate(audio_prompt_path=...)`` llama a
    ``prepare_conditionals`` y **sustituye** ``model.conds``. Ese efecto
    secundario es exactamente lo que el motor tiene que dominar.
    """

    sr = 24000

    def __init__(self):
        self.conds = "builtin"
        self.calls = []

    def generate(self, text, language_id, audio_prompt_path=None, **kwargs):
        self.calls.append(
            {
                "text": text,
                "language_id": language_id,
                "audio_prompt_path": audio_prompt_path,
                "conds_at_call": self.conds,
                **kwargs,
            }
        )
        if audio_prompt_path:
            self.conds = "clone"
        torch = pytest.importorskip("torch")
        return torch.zeros(1, int(self.sr))  # 1 s de audio


def make_engine(tmp_path, **overrides) -> ChatterboxEngine:
    cfg = TTSConfig()
    cfg.chatterbox_model_dir = str(tmp_path / "pesos")
    for key, value in overrides.items():
        setattr(cfg, key, value)
    engine = ChatterboxEngine(cfg)
    # Modelo precargado: estas pruebas miden el comportamiento, no la carga.
    engine._model = FakeChatterbox()
    engine._builtin_conds = engine._model.conds
    engine._loaded = True
    return engine


# ---------------------------------------------------------------------------
# Registro y configuracion
# ---------------------------------------------------------------------------


def test_chatterbox_is_registered_as_the_default_engine():
    cfg = TTSConfig()
    assert cfg.default_engine == "chatterbox"
    assert cfg.engines == ["chatterbox", "melotts", "openvoice"]

    engines = build_engines(cfg)
    assert isinstance(engines["chatterbox"], ChatterboxEngine)
    assert set(engines) == {"chatterbox", "melotts", "openvoice"}


def test_chatterbox_has_no_speaker_catalog(tmp_path):
    """Chatterbox no expone hablantes: una voz por defecto y clones."""
    assert make_engine(tmp_path).speakers() == []


def test_health_reports_the_checkpoint_state(tmp_path):
    engine = make_engine(tmp_path)
    info = engine.health()
    assert info["engine"] == "chatterbox"
    assert info["checkpoints"] == str(tmp_path / "pesos")
    assert info["checkpoints_present"] is False  # no hay ficheros reales
    assert info["t3_model"] == "v3"
    assert info["languages"] == 23


def test_missing_weights_are_reported_before_importing_the_package(tmp_path):
    """Sin pesos el codigo es claro y no intenta sintetizar."""
    engine = ChatterboxEngine(TTSConfig())
    engine.cfg.chatterbox_model_dir = str(tmp_path / "que-no-existe")

    with pytest.raises(EngineError) as excinfo:
        engine.ensure_loaded()

    assert excinfo.value.code == "engine_not_prepared"
    assert not excinfo.value.retryable
    assert "prepare_tts_models.sh" in excinfo.value.message
    for name in ("t3_mtl23ls_v3.safetensors", "conds.pt", "ve.pt", "s3gen.pt"):
        assert name in excinfo.value.message
    assert engine._model is None


def test_an_unknown_t3_alias_is_a_configuration_error(monkeypatch):
    """Un TTS_CHATTERBOX_T3_MODEL desconocido no debe intentarse cargar."""
    package = types.ModuleType("chatterbox")
    module = types.ModuleType("chatterbox.mtl_tts")
    module._resolve_multilingual_t3_model = lambda value: (_ for _ in ()).throw(
        ValueError("Unknown multilingual T3 model '%s'" % value)
    )
    package.mtl_tts = module
    monkeypatch.setitem(sys.modules, "chatterbox", package)
    monkeypatch.setitem(sys.modules, "chatterbox.mtl_tts", module)

    engine = ChatterboxEngine(TTSConfig())
    engine.cfg.chatterbox_t3_model = "v9"

    with pytest.raises(EngineError) as excinfo:
        engine._missing_weights()

    assert excinfo.value.code == "engine_config_invalid"
    assert not excinfo.value.retryable


def test_the_t3_alias_resolves_to_the_real_filename():
    """'v3' se convierte en el fichero que ``from_local`` va a buscar."""
    engine = ChatterboxEngine(TTSConfig())
    assert engine._t3_filename() == "t3_mtl23ls_v3.safetensors"

    engine.cfg.chatterbox_t3_model = "t3_mtl23ls_v2.safetensors"
    assert engine._t3_filename() == "t3_mtl23ls_v2.safetensors"


# ---------------------------------------------------------------------------
# Validaciones previas a la carga del modelo
# ---------------------------------------------------------------------------


def test_empty_text_is_rejected(tmp_path):
    engine = make_engine(tmp_path)
    with pytest.raises(EngineError) as excinfo:
        engine.synthesize("   ", VoiceProfile(), str(tmp_path / "x.wav"))
    assert excinfo.value.code == "empty_text"


@pytest.mark.parametrize(
    "raw,expected",
    [
        ("es", "es"),
        ("ES", "es"),
        ("en-us", "en"),
        ("pt_BR", "pt"),
        ("zh", "zh"),
        ("", None),
        ("ca", None),
        ("xx", None),
        (None, None),
    ],
)
def test_language_codes_are_normalised_to_chatterbox(raw, expected):
    assert ChatterboxEngine._normalize_language(raw) == expected


def test_an_unsupported_language_fails_before_loading_the_model(tmp_path):
    cfg = TTSConfig()
    cfg.chatterbox_model_dir = str(tmp_path / "pesos")
    engine = ChatterboxEngine(cfg)  # sin cargar

    with pytest.raises(EngineError) as excinfo:
        engine.synthesize(
            "text",
            VoiceProfile(slug="voz-catalana", language="ca"),
            str(tmp_path / "x.wav"),
        )

    assert excinfo.value.code == "language_unsupported"
    assert not excinfo.value.retryable
    # La validacion ocurre antes de intentar cargar ~2 GB de pesos.
    assert engine._model is None


def test_the_language_comes_from_the_voice_and_reaches_the_model(tmp_path):
    engine = make_engine(tmp_path)
    engine.synthesize(
        "Hola, buenas tardes.",
        VoiceProfile(slug="es-base", language="ES"),
        str(tmp_path / "out.wav"),
    )

    assert engine._model.calls[0]["language_id"] == "es"


def test_a_precomputed_openvoice_embedding_cannot_be_cloned(tmp_path):
    """Un .pth no es audio: fallar en silencio dejaria la voz equivocada."""
    engine = make_engine(tmp_path)
    voice = VoiceProfile(slug="ov-es", language="es", embedding_path=str(tmp_path / "es.pth"))

    with pytest.raises(EngineError) as excinfo:
        engine.synthesize("texto", voice, str(tmp_path / "out.wav"))

    assert excinfo.value.code == "voice_embedding_unsupported"
    assert not excinfo.value.retryable
    assert engine._model.calls == []


def test_a_reference_file_that_disappeared_is_reported(tmp_path):
    engine = make_engine(tmp_path)
    voice = VoiceProfile(
        slug="huerfana",
        language="es",
        reference_path=str(tmp_path / "no-existe.wav"),
    )

    with pytest.raises(EngineError) as excinfo:
        engine.synthesize("texto", voice, str(tmp_path / "out.wav"))

    assert excinfo.value.code == "voice_reference_missing"
    assert engine._model.calls == []


# ---------------------------------------------------------------------------
# La voz por defecto y los clones no se mezclan entre tareas
# ---------------------------------------------------------------------------


def test_a_voice_without_reference_uses_the_builtin_voice(tmp_path):
    engine = make_engine(tmp_path)
    result = engine.synthesize(
        "Buenas noches.",
        VoiceProfile(slug="es-base", language="es"),
        str(tmp_path / "out.wav"),
    )

    call = engine._model.calls[0]
    assert call["audio_prompt_path"] is None
    assert call["conds_at_call"] == "builtin"
    assert result.parameters["cloned"] is False
    assert result.engine_version == "chatterbox-multilingual-v3"
    assert result.sample_rate == 24000
    assert result.parameters["language_id"] == "es"


def test_a_clone_does_not_leak_into_the_next_default_voice(tmp_path):
    """generate() deja su clon en model.conds: la siguiente tarea no lo hereda."""
    engine = make_engine(tmp_path)
    reference = write_wav(str(tmp_path / "ref.wav"))

    engine.synthesize(
        "Primera locucion, clonando.",
        VoiceProfile(slug="mi-voz", language="es", reference_path=reference),
        str(tmp_path / "uno.wav"),
    )
    assert engine._model.conds == "clone"  # el clon queda instalado

    engine.synthesize(
        "Segunda locucion, voz por defecto.",
        VoiceProfile(slug="es-base", language="es"),
        str(tmp_path / "dos.wav"),
    )

    second = engine._model.calls[1]
    assert second["audio_prompt_path"] is None
    assert second["conds_at_call"] == "builtin"
    assert second["language_id"] == "es"


def test_every_clone_passes_the_reference_audio(tmp_path):
    engine = make_engine(tmp_path)
    reference = write_wav(str(tmp_path / "ref.wav"))

    engine.synthesize(
        "Locucion con referencia.",
        VoiceProfile(slug="mi-voz", language="es", reference_path=reference),
        str(tmp_path / "out.wav"),
    )

    call = engine._model.calls[0]
    assert call["audio_prompt_path"] == reference
    assert call["conds_at_call"] == "builtin"  # se parte de la voz por defecto
    assert call["exaggeration"] == 0.5
    assert call["cfg_weight"] == 0.5
    assert engine._model.conds == "clone"


def test_the_expression_controls_can_be_overridden_per_job(tmp_path):
    engine = make_engine(tmp_path)
    engine.synthesize(
        "Texto expresivo.",
        VoiceProfile(slug="es-base", language="es"),
        str(tmp_path / "out.wav"),
        {"exaggeration": 0.7, "cfg_weight": 0.3},
    )

    call = engine._model.calls[0]
    assert call["exaggeration"] == 0.7
    assert call["cfg_weight"] == 0.3


def test_the_speed_is_applied_outside_the_model(tmp_path):
    """Chatterbox no tiene 'speed': el audio se estira despues de generar."""
    engine = make_engine(tmp_path)
    out = str(tmp_path / "lento.wav")

    engine.synthesize(
        "Locucion mas lenta.",
        VoiceProfile(slug="es-base", language="es"),
        out,
        {"speed": 0.5},
    )

    import soundfile

    info = soundfile.info(out)
    # 1 s de audio generado, a la mitad de velocidad => ~2 s.
    assert info.duration == pytest.approx(2.0, abs=0.1)
    assert info.samplerate == 24000


def test_the_supported_language_list_is_the_published_one():
    assert "es" in CHATTERBOX_LANGUAGES
    assert "en" in CHATTERBOX_LANGUAGES
    assert "zh" in CHATTERBOX_LANGUAGES
    assert len(CHATTERBOX_LANGUAGES) == 23
