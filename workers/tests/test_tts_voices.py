"""Pruebas del registro de voces y del control de calidad de las muestras.

Incluye la proteccion del almacenamiento (9): ninguna operacion puede salir
del directorio de voces.
"""

from __future__ import annotations

import os

import numpy as np
import pytest

from workers.tts.voices import (
    MAX_SAMPLE_BYTES,
    VoiceError,
    analyze_sample,
    copy_builtin_sample,
    delete_sample,
    safe_join,
    sha256_file,
    slugify,
    store_sample,
)


def write_wav(path: str, seconds: float = 5.0, sample_rate: int = 22050, amplitude: float = 0.5) -> str:
    import soundfile

    t = np.linspace(0, seconds, int(seconds * sample_rate), endpoint=False, dtype=np.float32)
    signal = (amplitude * np.sin(2 * np.pi * 220 * t)).astype(np.float32)
    os.makedirs(os.path.dirname(path), exist_ok=True)
    soundfile.write(path, signal, sample_rate)
    return path


# ---------------------------------------------------------------------------
# 9. Proteccion del almacenamiento
# ---------------------------------------------------------------------------


def test_safe_join_blocks_parent_traversal(tmp_path):
    base = str(tmp_path / "voces")
    os.makedirs(base, exist_ok=True)
    with pytest.raises(VoiceError):
        safe_join(base, "../../etc/passwd")


def test_safe_join_blocks_absolute_paths(tmp_path):
    base = str(tmp_path / "voces")
    os.makedirs(base, exist_ok=True)
    with pytest.raises(VoiceError):
        safe_join(base, "/etc/passwd")


def test_safe_join_allows_normal_names(tmp_path):
    base = str(tmp_path / "voces")
    os.makedirs(base, exist_ok=True)
    resolved = safe_join(base, "mi-voz.wav")
    assert resolved.startswith(os.path.realpath(base))


def test_store_sample_writes_inside_the_voices_dir(tmp_dirs):
    path = store_sample(b"contenido de prueba", "mi-voz", tmp_dirs["voices"])
    assert os.path.dirname(os.path.realpath(path)) == os.path.realpath(tmp_dirs["voices"])
    assert os.path.isfile(path)


def test_store_sample_rejects_empty_payload(tmp_dirs):
    with pytest.raises(VoiceError) as excinfo:
        store_sample(b"", "vacia", tmp_dirs["voices"])
    assert excinfo.value.code == "sample_empty"


def test_store_sample_rejects_oversized_payload(tmp_dirs):
    with pytest.raises(VoiceError) as excinfo:
        store_sample(b"0" * (MAX_SAMPLE_BYTES + 1), "grande", tmp_dirs["voices"])
    assert excinfo.value.code == "sample_too_large"


def test_store_sample_sanitises_the_slug(tmp_dirs):
    path = store_sample(b"x", "../../evil name", tmp_dirs["voices"])
    assert os.path.dirname(os.path.realpath(path)) == os.path.realpath(tmp_dirs["voices"])


def test_delete_sample_only_deletes_inside_the_voices_dir(tmp_dirs):
    outside = tmp_dirs["root"] + "/fuera.wav"
    write_wav(outside)
    assert delete_sample(outside, tmp_dirs["voices"]) is False
    assert os.path.isfile(outside)


def test_delete_sample_removes_the_file(tmp_dirs):
    path = store_sample(b"x", "a-borrar", tmp_dirs["voices"])
    assert delete_sample(path, tmp_dirs["voices"]) is True
    assert not os.path.exists(path)


def test_copy_builtin_sample(tmp_dirs):
    source = write_wav(tmp_dirs["root"] + "/demo.wav")
    target = copy_builtin_sample(source, tmp_dirs["voices"], "demo")
    assert target and os.path.isfile(target)
    assert os.path.dirname(os.path.realpath(target)) == os.path.realpath(tmp_dirs["voices"])


# ---------------------------------------------------------------------------
# Calidad de la muestra
# ---------------------------------------------------------------------------


def test_quality_of_a_good_sample(tmp_dirs):
    path = write_wav(tmp_dirs["root"] + "/buena.wav", seconds=6.0)
    report = analyze_sample(path, min_seconds=3.0, max_seconds=60.0)
    assert report.ok
    assert report.issues == []
    assert report.duration_seconds == pytest.approx(6.0, abs=0.1)
    assert report.sample_rate == 22050


def test_quality_flags_a_short_sample(tmp_dirs):
    path = write_wav(tmp_dirs["root"] + "/corta.wav", seconds=1.0)
    report = analyze_sample(path, min_seconds=3.0, max_seconds=60.0)
    assert not report.ok
    assert "too_short" in report.issues


def test_quality_flags_a_long_sample(tmp_dirs):
    path = write_wav(tmp_dirs["root"] + "/larga.wav", seconds=90.0)
    report = analyze_sample(path, min_seconds=3.0, max_seconds=60.0)
    assert not report.ok
    assert "too_long" in report.issues


def test_quality_flags_a_quiet_sample(tmp_dirs):
    path = write_wav(tmp_dirs["root"] + "/baja.wav", seconds=6.0, amplitude=0.01)
    report = analyze_sample(path, min_seconds=3.0, max_seconds=60.0)
    assert not report.ok
    assert "too_quiet" in report.issues


def test_quality_flags_clipping(tmp_dirs):
    path = write_wav(tmp_dirs["root"] + "/saturada.wav", seconds=6.0, amplitude=1.0)
    report = analyze_sample(path, min_seconds=3.0, max_seconds=60.0)
    assert not report.ok
    assert "clipping" in report.issues


def test_quality_rejects_an_undecodable_file(tmp_dirs):
    path = tmp_dirs["root"] + "/roto.wav"
    with open(path, "wb") as handle:
        handle.write(b"no es un wav")
    with pytest.raises(VoiceError) as excinfo:
        analyze_sample(path)
    assert excinfo.value.code == "sample_unreadable"


def test_quality_rejects_a_missing_file(tmp_dirs):
    with pytest.raises(VoiceError) as excinfo:
        analyze_sample(tmp_dirs["root"] + "/no-existe.wav")
    assert excinfo.value.code == "sample_missing"


# ---------------------------------------------------------------------------
# Utilidades
# ---------------------------------------------------------------------------


def test_slugify():
    assert slugify("Voz de Prueba") == "voz-de-prueba"
    assert slugify("  ") == "voz"
    assert slugify("../../etc/passwd") == "etc-passwd"


def test_sha256_file(tmp_dirs):
    path = tmp_dirs["root"] + "/datos.bin"
    with open(path, "wb") as handle:
        handle.write(b"contenido")
    digest = sha256_file(path)
    assert len(digest) == 64
    assert digest == sha256_file(path)
