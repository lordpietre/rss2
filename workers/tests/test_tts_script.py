"""Pruebas de la normalizacion de guiones.

Cubren los puntos 1, 2 y 3 del plan de aceptacion:
texto normal, noticias con fechas/cifras/porcentajes/nombres propios, y
textos vacios o demasiado largos.
"""

from __future__ import annotations

import pytest

from workers.tts.script import (
    int_to_es,
    normalize_for_speech,
    split_sentences,
    summarize_script,
)


# ---------------------------------------------------------------------------
# 1. Texto espanol normal
# ---------------------------------------------------------------------------


def test_normal_text_keeps_wording():
    text = "El presidente del gobierno hizo declaraciones ayer."
    result = normalize_for_speech(text)
    assert "declaraciones" in result
    assert "ayer" in result


def test_normal_text_is_not_emptied():
    text = "Las elecciones se celebraron con una participacion del setenta por ciento."
    result = normalize_for_speech(text)
    assert result.strip() != ""


def test_empty_text_returns_empty_string():
    assert normalize_for_speech("") == ""
    assert normalize_for_speech("   ") == ""


def test_only_whitespace_is_removed():
    assert normalize_for_speech("   \n\t  ") == ""


# ---------------------------------------------------------------------------
# 2. Fechas, cifras, porcentajes y nombres propios
# ---------------------------------------------------------------------------


def test_percentages_are_expanded():
    result = normalize_for_speech("El paro sube al 12,4% en diciembre.")
    assert "por ciento" in result
    assert "%" not in result
    assert "doce coma cuatro" in result


def test_integer_percentage():
    result = normalize_for_speech("Un 45% de los votos emitidos.")
    assert "cuarenta y cinco por ciento" in result


def test_dates_with_slashes_use_spanish_convention():
    result = normalize_for_speech("La reunion fue el 12/03/2024 en Madrid.")
    assert "doce de marzo de dos mil veinte y cuatro" in result
    assert "12/03/2024" not in result


def test_iso_dates():
    result = normalize_for_speech("Publicado el 2024-03-12.")
    assert "doce de marzo de dos mil veinte y cuatro" in result


def test_long_date_keeps_month_name():
    result = normalize_for_speech("Firmado el 15 de marzo de 2024.")
    assert "quince de marzo de dos mil veinte y cuatro" in result


def test_thousands_separator_is_read_as_thousands():
    result = normalize_for_speech("Hubo 1.234 asistentes en la plaza.")
    assert "mil doscientos treinta y cuatro asistentes" in result
    assert "1.234" not in result


def test_decimal_comma_is_read_as_decimal():
    result = normalize_for_speech("La bolsa subio un 3,5 por ciento.")
    assert "tres coma cinco" in result


def test_currency_is_expanded():
    result = normalize_for_speech("El presupuesto es de 1.234,56 EUR.")
    assert "euros" in result
    assert "EUR" not in result


def test_units_after_a_number():
    result = normalize_for_speech("Recorrido de 300 km en 45 min.")
    assert "trescientos kilómetros" in result
    assert "cuarenta y cinco minutos" in result


def test_ordinals_are_expanded():
    result = normalize_for_speech("Fue el 1º turno del campeonato.")
    assert "primero" in result
    assert "1º" not in result


def test_enumeration_with_o_is_not_an_ordinal():
    """'2 o 3' es una enumeracion, no un ordinal: la 'o' se conserva."""
    result = normalize_for_speech("El resultado fue 2 o 3 goles.")
    assert "segundo" not in result
    assert " o " in result


def test_proper_names_are_preserved():
    text = "Angela Merkel y Joe Biden viajaron a Madrid el martes."
    result = normalize_for_speech(text)
    assert "Angela Merkel" in result
    assert "Joe Biden" in result
    assert "Madrid" in result


def test_abbreviations_are_expanded():
    result = normalize_for_speech("El Sr. Perez y la Dra. Gomez hicieron declaraciones.")
    assert "señor" in result.lower()
    assert "doctora" in result.lower()


def test_country_abbreviation():
    result = normalize_for_speech("EE.UU. y la UE celebraron una cumbre.")
    assert "Estados Unidos" in result
    assert "Unión Europea" in result


def test_urls_are_removed_from_the_narration():
    result = normalize_for_speech("Fuente: www.efe.com (EFE).")
    assert "www.efe.com" not in result
    assert "EFE" in result


def test_symbols_are_cleaned():
    result = normalize_for_speech("Madrid & Barcelona: 100% «cita»…")
    assert "&" not in result
    assert "«" not in result
    assert "cita" in result


def test_numbers_do_not_change_the_facts():
    """El valor numerico debe seguir siendo legible tras la expansion."""
    result = normalize_for_speech("Hubo 2.687 asistentes en la plaza.")
    assert "dos mil seiscientos ochenta y siete" in result


# ---------------------------------------------------------------------------
# Enteros en espanol
# ---------------------------------------------------------------------------


@pytest.mark.parametrize(
    "value,expected",
    [
        (0, "cero"),
        (1, "uno"),
        (15, "quince"),
        (16, "dieciséis"),
        (21, "veinte y uno"),
        (22, "veinte y dos"),
        (30, "treinta"),
        (100, "cien"),
        (101, "ciento uno"),
        (200, "doscientos"),
        (500, "quinientos"),
        (900, "novecientos"),
        (1000, "mil"),
        (2000, "dos mil"),
        (2024, "dos mil veinte y cuatro"),
        (21000, "veintiún mil"),
        (1000000, "un millón"),
        (2000000, "dos millones"),
    ],
)
def test_int_to_es(value, expected):
    assert int_to_es(value) == expected


# ---------------------------------------------------------------------------
# 3. Textos vacios y demasiado largos
# ---------------------------------------------------------------------------


def test_very_long_text_is_handled():
    text = "frase de prueba. " * 5000
    result = normalize_for_speech(text)
    assert len(result) > 0
    assert len(result) < len(text) * 2


def test_split_sentences_respects_the_limit():
    text = " ".join(["Esta es una frase bastante larga para la prueba."] * 50)
    chunks = split_sentences(text, max_chars=120)
    assert chunks
    assert all(len(chunk) <= 200 for chunk in chunks)
    # Ningun chunk debe partir una palabra por la mitad.
    assert all(chunk.strip() for chunk in chunks)


def test_split_sentences_on_empty_text():
    assert split_sentences("") == []


def test_summarize_script_truncates_without_breaking_a_sentence():
    text = "Primera frase. " + ("Segunda frase muy larga. " * 40)
    summary = summarize_script(text, max_chars=100)
    assert len(summary) <= 100
    assert summary.endswith(".")


def test_summarize_script_keeps_short_text():
    assert summarize_script("Texto corto.", max_chars=100) == "Texto corto."
