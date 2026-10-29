"""Normalizacion de guiones para locucion.

Convierte un texto editorial en algo que un motor TTS pueda leer en voz alta:
expande cifras, fechas, porcentajes, abreviaturas y simbolos.

Reglas de diseno:

- No se inventa ni se cambia informacion: solo se reescriben las *formas*
  (un "45%" se lee "cuarenta y cinco por ciento", no "cuarenta y cinco").
- Lo dudoso se deja intacto. Ante la duda es preferible que el motor lea
  "3.14" de forma literal antes que decidir por el redactor que es "3,14".
- El texto original siempre se conserva en el campo ``raw_content`` del
  guion; esta funcion produce una segunda version, la que se locuta.
"""

from __future__ import annotations

import re
from typing import Iterable, List, Optional

# ---------------------------------------------------------------------------
# Numeros en espanol
# ---------------------------------------------------------------------------

_UNITS = (
    "cero", "uno", "dos", "tres", "cuatro",
    "cinco", "seis", "siete", "ocho", "nueve",
)

_TEENS = {
    10: "diez", 11: "once", 12: "doce", 13: "trece", 14: "catorce",
    15: "quince", 16: "dieciséis", 17: "diecisiete", 18: "dieciocho",
    19: "diecinueve",
}

_TENS = {
    20: "veinte", 30: "treinta", 40: "cuarenta", 50: "cincuenta",
    60: "sesenta", 70: "setenta", 80: "ochenta", 90: "noventa",
}

_HUNDREDS = {
    100: "cien", 200: "doscientos", 300: "trescientos", 400: "cuatrocientos",
    500: "quinientos", 600: "seiscientos", 700: "setecientos",
    800: "ochocientos", 900: "novecientos",
}

_MONTHS = (
    "enero", "febrero", "marzo", "abril", "mayo", "junio",
    "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre",
)

_MONTH_INDEX = {name: index + 1 for index, name in enumerate(_MONTHS)}

_ORDINALS = {
    1: "primero", 2: "segundo", 3: "tercero", 4: "cuarto", 5: "quinto",
    6: "sexto", 7: "séptimo", 8: "octavo", 9: "noveno", 10: "décimo",
    11: "undécimo", 12: "duodécimo", 13: "decimotercero", 14: "decimocuarto",
    15: "decimoquinto", 16: "decimosexto", 17: "decimoséptimo",
    18: "decimoctavo", 19: "decimonoveno", 20: "vigésimo",
    21: "vigésimo primero", 22: "vigésimo segundo", 23: "vigésimo tercero",
    24: "vigésimo cuarto", 25: "vigésimo quinto", 26: "vigésimo sexto",
    27: "vigésimo séptimo", 28: "vigésimo octavo", 29: "vigésimo noveno",
    30: "trigésimo",
}


def _under_hundred(value: int) -> str:
    if value < 10:
        return _UNITS[value]
    if value < 20:
        return _TEENS[value]
    tens, unit = divmod(value, 10)
    tens *= 10
    if unit == 0:
        return _TENS[tens]
    return "%s y %s" % (_TENS[tens], _UNITS[unit])


def _under_thousand(value: int) -> str:
    hundreds, rest = divmod(value, 100)
    if hundreds == 0:
        return _under_hundred(rest)
    if rest == 0:
        return _HUNDREDS[hundreds * 100]
    if hundreds == 1:
        return "ciento " + _under_hundred(rest)
    return _HUNDREDS[hundreds * 100] + " " + _under_hundred(rest)


def _under_million(value: int) -> str:
    thousands, rest = divmod(value, 1000)
    if thousands == 0:
        return _under_thousand(rest)
    if thousands == 1:
        head = "mil"
    elif thousands == 21:
        # apocope: "veintiún mil"
        head = "veintiún mil"
    else:
        head = _under_thousand(thousands) + " mil"
    if rest == 0:
        return head
    return head + " " + _under_thousand(rest)


def _under_billion(value: int) -> str:
    millions, rest = divmod(value, 1_000_000)
    if millions == 0:
        return _under_million(rest)
    head = "un millón" if millions == 1 else _under_thousand(millions) + " millones"
    if rest == 0:
        return head
    return head + " " + _under_million(rest)


def int_to_es(value: int) -> str:
    """Convierte un entero no negativo a su lectura en espanol."""
    if value < 0:
        return "menos " + int_to_es(-value)
    if value >= 1_000_000_000:
        head, rest = divmod(value, 1_000_000_000)
        words = "mil millones" if head == 1 else _under_thousand(head) + " mil millones"
        if rest == 0:
            return words
        return words + " " + int_to_es(rest)
    if value >= 1_000_000:
        return _under_billion(value)
    if value >= 1000:
        return _under_million(value)
    return _under_thousand(value)


# ---------------------------------------------------------------------------
# Abreviaturas y unidades
# ---------------------------------------------------------------------------

# Abreviaturas -> lectura. La clave es el texto tal cual aparece (con punto).
ABBREVIATIONS = {
    "ee.uu.a.": "Estados Unidos",
    "ee.uu.": "Estados Unidos",
    "ee.uu": "Estados Unidos",
    "eeuu.": "Estados Unidos",
    "eeuu": "Estados Unidos",
    "sra.": "señora",
    "srta.": "señorita",
    "dña.": "doña",
    "dr.": "doctor",
    "dra.": "doctora",
    "sr.": "señor",
    "lic.": "licenciado",
    "págs.": "páginas",
    "pág.": "página",
    "núm.": "número",
    "n.º": "número",
    "n°": "número",
    "nº": "número",
    "art.": "artículo",
    "cap.": "capítulo",
    "fig.": "figura",
    "vol.": "volumen",
    "aprox.": "aproximadamente",
    "máx.": "máximo",
    "mín.": "mínimo",
    "ej.": "ejemplo",
    "av.": "avenida",
    "c/": "calle",
    "pº": "paseo",
    "fmi": "Fondo Monetario Internacional",
    "oms": "Organización Mundial de la Salud",
    "ong": "Organización No Gubernamental",
    "otan": "Organización del Tratado del Atlántico Norte",
    "ue": "Unión Europea",
    "petro.": "petróleo",
    "s.a.": "sociedad anónima",
    "s.l.": "sociedad limitada",
}

# Unidades: solo se expanden si van detras de una cifra (evita falsos positivos).
UNITS = {
    "km": "kilómetros",
    "kms": "kilómetros",
    "cm": "centímetros",
    "mm": "milímetros",
    "kg": "kilogramos",
    "mg": "miligramos",
    "gr": "gramos",
    "t": "toneladas",
    "l": "litros",
    "ml": "mililitros",
    "h": "horas",
    "min": "minutos",
    "s": "segundos",
    "ha": "hectáreas",
    "mb": "megabytes",
    "gb": "gigabytes",
    "tb": "terabytes",
    "mhz": "megahercios",
    "ghz": "gigahercios",
    "°c": "grados Celsius",
    "ºc": "grados Celsius",
}

CURRENCIES = {
    "€": "euros",
    "$": "dólares",
    "£": "libras",
    "us$": "dólares",
    "eur": "euros",
    "usd": "dólares",
    "gbp": "libras",
    "m€": "millones de euros",
    "m$": "millones de dólares",
    "mm€": "millones de euros",
}

# Numero escrito con separador de miles o decimal. La conversion a palabras
# la hace _number_to_words, que distingue "1.234" (miles) de "3.5" (decimal).
NUMBER_PATTERN = r"\d{1,3}(?:\.\d{3})+(?:,\d+)?|\d+(?:,\d+)?|\d+\.\d{1,2}"

_URL_RE = re.compile(r"(?:https?://|www\.)\S+", re.IGNORECASE)
_CONTROL_RE = re.compile(r"[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]")
_WHITESPACE_RE = re.compile(r"[ \t ]+")


def _match_case(source: str, replacement: str) -> str:
    """Mantiene la capitalizacion del original ('Sr.' -> 'Señor')."""
    if source and source[0].isupper():
        return replacement[0].upper() + replacement[1:]
    return replacement


def expand_abbreviations(text: str) -> str:
    """Expande abreviaturas conocidas conservando la capitalizacion."""
    for abbreviation in sorted(ABBREVIATIONS, key=len, reverse=True):
        # La frontera exige que no haya ni letra ni punto delante: asi "S.A."
        # dentro de "U.S.A." no se toca.
        pattern = re.compile(r"(?<![\w.])" + re.escape(abbreviation) + r"(?![\w])", re.IGNORECASE)
        replacement = ABBREVIATIONS[abbreviation]

        def _sub(match: "re.Match[str]") -> str:
            return _match_case(match.group(0), replacement)

        text = pattern.sub(_sub, text)

    # "D." solo cuando precede a un nombre propio: "D. Juan Perez" -> "Don Juan Perez".
    text = re.sub(
        r"(?<![\w.])D\.(?=\s+[A-ZÁÉÍÓÚÑÜ])",
        "Don ",
        text,
    )
    return text


def expand_dates(text: str) -> str:
    """Convierte fechas numericas y anos en palabras."""
    month_names = "|".join(_MONTHS)

    def _year(value: str) -> str:
        return int_to_es(int(value))

    # 12/03/2024 (convencion espanola: dia/mes/ano)
    def _slash(match: "re.Match[str]") -> str:
        day, month, year = int(match.group(1)), int(match.group(2)), match.group(3)
        if not (1 <= day <= 31 and 1 <= month <= 12):
            return match.group(0)
        return "%d de %s de %s" % (day, _MONTHS[month - 1], _year(year))

    text = re.sub(r"(?<![\w])(\d{1,2})/(\d{1,2})/(\d{4})(?![\w])", _slash, text)

    # 2024-03-12
    def _iso(match: "re.Match[str]") -> str:
        year, month, day = int(match.group(1)), int(match.group(2)), int(match.group(3))
        if not (1 <= day <= 31 and 1 <= month <= 12):
            return match.group(0)
        return "%d de %s de %s" % (day, _MONTHS[month - 1], _year(year))

    text = re.sub(r"(?<![\w])(\d{4})-(\d{1,2})-(\d{1,2})(?![\w])", _iso, text)

    # "12 de marzo de 2024"
    text = re.sub(
        r"(?<![\w])(\d{1,2})\s+de\s+(%s)\s+de\s+(\d{4})(?![\w])" % month_names,
        lambda m: "%s de %s de %s" % (m.group(1), m.group(2).lower(), _year(m.group(3))),
        text,
        flags=re.IGNORECASE,
    )

    # "marzo de 2024"
    text = re.sub(
        r"(?<![\w])(%s)\s+de\s+(\d{4})(?![\w])" % month_names,
        lambda m: "%s de %s" % (m.group(1).lower(), _year(m.group(2))),
        text,
        flags=re.IGNORECASE,
    )
    return text


def expand_percentages(text: str) -> str:
    """'45%' -> 'cuarenta y cinco por ciento'."""
    return re.sub(
        r"(?<![\w])(" + NUMBER_PATTERN + r")\s*%",
        lambda m: _number_to_words(m.group(1)) + " por ciento",
        text,
    )


def expand_currency(text: str) -> str:
    """'1.234,56 EUR' -> 'mil doscientos treinta y cuatro coma cincuenta y seis euros'."""
    symbols = sorted(CURRENCIES, key=len, reverse=True)
    pattern = r"(?<![\w])(" + NUMBER_PATTERN + r")\s*(" + "|".join(re.escape(s) for s in symbols) + r")(?![\w])"
    lookup = {symbol.lower(): name for symbol, name in CURRENCIES.items()}
    return re.sub(
        pattern,
        lambda m: _number_to_words(m.group(1)) + " " + lookup[m.group(2).lower()],
        text,
        flags=re.IGNORECASE,
    )


def expand_units(text: str) -> str:
    """'300 km' -> 'trescientos kilómetros' (solo detras de una cifra)."""
    units = sorted(UNITS, key=len, reverse=True)
    pattern = r"(?<![\w])(" + NUMBER_PATTERN + r")\s*(" + "|".join(re.escape(u) for u in units) + r")(?![\w])"
    return re.sub(
        pattern,
        lambda m: _number_to_words(m.group(1)) + " " + UNITS[m.group(2)],
        text,
    )


def expand_ordinals(text: str) -> str:
    """'1o' / '2a' -> 'primero' / 'segunda'."""
    def _sub(match: "re.Match[str]") -> str:
        value = int(match.group(1))
        word = _ORDINALS.get(value)
        if word is None:
            return match.group(0)
        if match.group(2) == "ª":
            word = word[:-1] + "a" if word.endswith("o") else word
        return word

    # Solo se reconocen los ordinales con indicador explicito (º, °, ª).
    # El "2 o 3" de una enumeracion no se toca.
    text = re.sub(r"(?<![\w])(\d{1,2})\s*([º°ª])\.?(?![\w])", _sub, text)

    # Forma ASCII "1o turno": se acepta solo si lo que sigue es una palabra
    # que no es un numero ("2 o tres" es una enumeracion, no un ordinal).
    not_a_number = (
        r"(?!(?:un|una|dos|tres|cuatro|cinco|seis|siete|ocho|nueve|diez|once|"
        r"doce|trece|catorce|quince|dieciseis|diecisiete|dieciocho|"
        r"diecinueve|veinte|ciento|mil|millon|millones)\b)"
    )
    text = re.sub(
        r"(?<![\w])(\d{1,2})\s*o(?=\s+" + not_a_number + r"[a-záéíóúñü])",
        lambda m: _ORDINALS.get(int(m.group(1)), m.group(0)),
        text,
        flags=re.IGNORECASE,
    )
    return text


def _number_to_words(raw: str) -> str:
    """Lee un numero escrito con separadores de miles o decimales."""
    raw = raw.strip()
    if not raw:
        return raw

    if "," in raw and "." in raw:
        # "1.234,56": miles con punto, decimales con coma.
        integer_part, _, fraction_part = raw.rpartition(",")
        integer_part = integer_part.replace(".", "")
    elif "," in raw:
        # "3,5": decimales con coma.
        integer_part, _, fraction_part = raw.partition(",")
    elif "." in raw:
        chunks = raw.split(".")
        if len(chunks) > 2:
            # "1.000.000" son miles; "1.2.3" no es un numero: se deja intacto.
            if (
                chunks[0].isdigit()
                and len(chunks[0]) <= 3
                and all(len(chunk) == 3 and chunk.isdigit() for chunk in chunks[1:])
            ):
                integer_part, fraction_part = "".join(chunks), ""
            else:
                return raw
        else:
            integer_part, fraction_part = chunks
            # "1.234" son miles; "3.5" es un decimal.
            if (
                len(integer_part) <= 3
                and integer_part.isdigit()
                and len(fraction_part) == 3
                and fraction_part.isdigit()
            ):
                integer_part, fraction_part = integer_part + fraction_part, ""
    else:
        integer_part, fraction_part = raw, ""

    integer_part = integer_part.replace(".", "").replace(",", "")
    if not integer_part.isdigit():
        return raw

    words = int_to_es(int(integer_part))

    fraction_part = fraction_part.strip()
    if fraction_part:
        fraction_part = fraction_part.rstrip("0")
    if fraction_part:
        if not fraction_part.isdigit():
            return raw
        digits = " ".join(_UNITS[int(digit)] for digit in fraction_part)
        words = "%s coma %s" % (words, digits)
    return words


def expand_numbers(text: str) -> str:
    """Convierte los numeros que quedan sueltos."""
    pattern = re.compile(r"(?<![\w.])(\d[\d.,]*)(?![\w])")
    return pattern.sub(lambda m: _number_to_words(m.group(1)), text)


def clean_symbols(text: str) -> str:
    """Limpia simbolos que no aportan a la lectura en voz alta."""
    replacements = (
        ("&", " y "),
        ("+", " más "),
        ("—", ", "),
        ("–", ", "),
        ("…", ", "),
        ("...", ", "),
        ("«", " "),
        ("»", " "),
        ('"', " "),
        ("“", " "),
        ("”", " "),
        ("(", " "),
        (")", " "),
        ("[", " "),
        ("]", " "),
        ("*", " "),
        ("#", " "),
        ("¿", " "),
        ("¡", " "),
        ("@", "arroba "),
        ("/", " "),
    )
    for source, target in replacements:
        text = text.replace(source, target)
    return text


def _tidy(text: str) -> str:
    text = _WHITESPACE_RE.sub(" ", text)
    text = re.sub(r"\s+([,.;:!?])", r"\1", text)
    text = re.sub(r"([,;:])(?=\S)", r"\1 ", text)
    text = re.sub(r"\s{2,}", " ", text)
    return text.strip()


def normalize_for_speech(text: str, language: str = "es") -> str:
    """Devuelve la version de ``text`` preparada para ser locutada.

    Acepta cualquier idioma; la expansion de cifras y abreviaturas esta
    escrita para espanol, que es el idioma objetivo de la primera fase.
    """
    if not text:
        return ""

    text = _CONTROL_RE.sub(" ", text)
    text = text.replace(" ", " ")
    text = _URL_RE.sub(" ", text)
    text = expand_abbreviations(text)
    text = expand_dates(text)
    text = expand_percentages(text)
    text = expand_currency(text)
    text = expand_units(text)
    text = expand_ordinals(text)
    text = expand_numbers(text)
    text = clean_symbols(text)
    text = _tidy(text)

    if text:
        text = text[0].upper() + text[1:]
    return text


def split_sentences(text: str, max_chars: int = 400) -> List[str]:
    """Trocea un guion en frasesiman para el motor de sintesis.

    No parte a mitad de una palabra ni deja fragmentos de menos de 2
    caracteres (que el motor leeria de forma erratica).
    """
    if not text:
        return []

    sentences = re.split(r"(?<=[.!?;:])\s+", text)
    chunks: List[str] = []
    current = ""
    for sentence in sentences:
        sentence = sentence.strip()
        if not sentence:
            continue
        if len(sentence) > max_chars:
            if current:
                chunks.append(current)
                current = ""
            # Corte por palabras, sin partir el guion en mitad de un token.
            words = sentence.split()
            piece = ""
            for word in words:
                if len(piece) + len(word) + 1 > max_chars:
                    if piece:
                        chunks.append(piece.strip())
                    piece = word
                else:
                    piece = (piece + " " + word).strip()
            if piece:
                current = piece
            continue
        if not current:
            current = sentence
        elif len(current) + 1 + len(sentence) <= max_chars:
            current = current + " " + sentence
        else:
            chunks.append(current)
            current = sentence
    if current:
        chunks.append(current)
    return [chunk for chunk in chunks if len(chunk.strip()) >= 2]


def summarize_script(text: str, max_chars: int = 600) -> str:
    """Recorta un guion a un maximo de caracteres sin cortar a mitad de frase."""
    if not text or len(text) <= max_chars:
        return (text or "").strip()

    sentences = re.split(r"(?<=[.!?])\s+", text)
    result: List[str] = []
    length = 0
    for sentence in sentences:
        sentence = sentence.strip()
        if not sentence:
            continue
        if length + len(sentence) + 1 > max_chars:
            break
        result.append(sentence)
        length += len(sentence) + 1
    if result:
        return " ".join(result)
    # Ninguna frase cabia: corte por palabras.
    words = text.split()
    piece = ""
    for word in words:
        if len(piece) + len(word) + 1 > max_chars:
            break
        piece = (piece + " " + word).strip()
    return piece
