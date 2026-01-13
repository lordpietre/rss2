from __future__ import annotations

import time
from datetime import datetime
from typing import Optional

import bleach
from markupsafe import Markup


def safe_html(texto: Optional[str]) -> str:
    if not texto:
        return ""
    
    # Sanitize content to prevent layout breakage (e.g. unclosed divs)
    allowed_tags = ['b', 'i', 'strong', 'em', 'p', 'br', 'span', 'a']
    allowed_attrs = {'a': ['href', 'target', 'rel']}
    
    cleaned = bleach.clean(texto, tags=allowed_tags, attributes=allowed_attrs, strip=True)
    return Markup(cleaned)


def normalize_url_py(u: Optional[str]) -> Optional[str]:
    if not u:
        return None

    u = u.strip()
    if not u:
        return None

    if "://" not in u:
        u = "http://" + u

    u = u.split("#", 1)[0]

    try:
        from urllib.parse import (
            urlsplit,
            urlunsplit,
            parse_qsl,
            urlencode,
        )
    except ImportError:
        return u

    try:
        parts = urlsplit(u)
    except Exception:
        return u

    scheme = parts.scheme.lower()
    netloc = parts.netloc.lower()

    if "@" in netloc:
        auth, host = netloc.rsplit("@", 1)
    else:
        auth, host = None, netloc

    if ":" in host:
        hostname, port = host.split(":", 1)
    else:
        hostname, port = host, None

    hostname = hostname.strip()
    if port:
        port = port.strip()

    if (scheme == "http" and port == "80") or (scheme == "https" and port == "443"):
        port = None

    if port:
        host = f"{hostname}:{port}"
    else:
        host = hostname

    if auth:
        host = f"{auth}@{host}"

    query_list = parse_qsl(parts.query, keep_blank_values=True)
    query_filtered = [
        (k, v)
        for (k, v) in query_list
        if not (k.startswith("utm_") or k in ("gclid", "fbclid"))
    ]
    query = urlencode(query_filtered)

    path = parts.path
    while "//" in path:
        path = path.replace("//", "/")

    cleaned = urlunsplit((scheme, host, path, query, ""))

    return cleaned


def parse_rss_datetime(s: Optional[str]) -> Optional[datetime]:
    if not s:
        return None

    s = s.strip()
    if not s:
        return None

    formats = [
        "%a, %d %b %Y %H:%M:%S %z",
        "%Y-%m-%dT%H:%M:%S%z",
        "%Y-%m-%dT%H:%M:%S.%f%z",
        "%a, %d %b %Y %H:%M:%S GMT",
        "%Y-%m-%d %H:%M:%S",
    ]

    for fmt in formats:
        try:
            return datetime.strptime(s, fmt)
        except Exception:
            pass

    try:
        from email.utils import parsedate_to_datetime

        dt = parsedate_to_datetime(s)
        return dt
    except Exception:
        return None


def unix_time() -> int:
    return int(time.time())


def format_date(value, format="%Y-%m-%d %H:%M"):
    if value is None:
        return ""
    if not isinstance(value, datetime):
        return str(value)
    return value.strftime(format)


# Country name (Spanish) to ISO 3166-1 alpha-2 code mapping
COUNTRY_ISO = {
    "afganistán": "AF", "albania": "AL", "alemania": "DE", "andorra": "AD",
    "angola": "AO", "antigua y barbuda": "AG", "arabia saudita": "SA",
    "argelia": "DZ", "argentina": "AR", "armenia": "AM", "australia": "AU",
    "austria": "AT", "azerbaiyán": "AZ", "bahamas": "BS", "bangladés": "BD",
    "barbados": "BB", "baréin": "BH", "bélgica": "BE", "belice": "BZ",
    "benín": "BJ", "bielorrusia": "BY", "birmania": "MM", "bolivia": "BO",
    "bosnia y herzegovina": "BA", "botsuana": "BW", "brasil": "BR",
    "brunéi": "BN", "bulgaria": "BG", "burkina faso": "BF", "burundi": "BI",
    "bután": "BT", "cabo verde": "CV", "camboya": "KH", "camerún": "CM",
    "canadá": "CA", "catar": "QA", "chad": "TD", "chile": "CL", "china": "CN",
    "chipre": "CY", "colombia": "CO", "comoras": "KM", "corea del norte": "KP",
    "corea del sur": "KR", "costa de marfil": "CI", "costa rica": "CR",
    "croacia": "HR", "cuba": "CU", "dinamarca": "DK", "dominica": "DM",
    "ecuador": "EC", "egipto": "EG", "el salvador": "SV",
    "emiratos árabes unidos": "AE", "eritrea": "ER", "eslovaquia": "SK",
    "eslovenia": "SI", "españa": "ES", "estados unidos": "US", "estonia": "EE",
    "esuatini": "SZ", "etiopía": "ET", "filipinas": "PH", "finlandia": "FI",
    "fiyi": "FJ", "francia": "FR", "gabón": "GA", "gambia": "GM",
    "georgia": "GE", "ghana": "GH", "granada": "GD", "grecia": "GR",
    "guatemala": "GT", "guinea": "GN", "guinea-bisáu": "GW",
    "guinea ecuatorial": "GQ", "guyana": "GY", "haití": "HT", "honduras": "HN",
    "hungría": "HU", "india": "IN", "indonesia": "ID", "irak": "IQ",
    "irán": "IR", "irlanda": "IE", "islandia": "IS", "islas marshall": "MH",
    "islas salomón": "SB", "israel": "IL", "italia": "IT", "jamaica": "JM",
    "japón": "JP", "jordania": "JO", "kazajistán": "KZ", "kenia": "KE",
    "kirguistán": "KG", "kiribati": "KI", "kuwait": "KW", "laos": "LA",
    "lesoto": "LS", "letonia": "LV", "líbano": "LB", "liberia": "LR",
    "libia": "LY", "liechtenstein": "LI", "lituania": "LT", "luxemburgo": "LU",
    "macedonia del norte": "MK", "madagascar": "MG", "malasia": "MY",
    "malaui": "MW", "maldivas": "MV", "malí": "ML", "malta": "MT",
    "marruecos": "MA", "mauricio": "MU", "mauritania": "MR", "méxico": "MX",
    "micronesia": "FM", "moldavia": "MD", "mónaco": "MC", "mongolia": "MN",
    "montenegro": "ME", "mozambique": "MZ", "namibia": "NA", "nauru": "NR",
    "nepal": "NP", "nicaragua": "NI", "níger": "NE", "nigeria": "NG",
    "noruega": "NO", "nueva zelanda": "NZ", "omán": "OM", "países bajos": "NL",
    "pakistán": "PK", "palaos": "PW", "palestina": "PS", "panamá": "PA",
    "papúa nueva guinea": "PG", "paraguay": "PY", "perú": "PE", "polonia": "PL",
    "portugal": "PT", "reino unido": "GB", "república centroafricana": "CF",
    "república checa": "CZ", "república del congo": "CG",
    "república democrática del congo": "CD", "república dominicana": "DO",
    "ruanda": "RW", "rumanía": "RO", "rusia": "RU", "samoa": "WS",
    "san cristóbal y nieves": "KN", "san marino": "SM",
    "san vicente y las granadinas": "VC", "santa lucía": "LC",
    "santo tomé y príncipe": "ST", "senegal": "SN", "serbia": "RS",
    "seychelles": "SC", "sierra leona": "SL", "singapur": "SG", "siria": "SY",
    "somalia": "SO", "sri lanka": "LK", "sudáfrica": "ZA", "sudán": "SD",
    "sudán del sur": "SS", "suecia": "SE", "suiza": "CH", "surinam": "SR",
    "tailandia": "TH", "tanzania": "TZ", "tayikistán": "TJ",
    "timor oriental": "TL", "togo": "TG", "tonga": "TO",
    "trinidad y tobago": "TT", "túnez": "TN", "turkmenistán": "TM",
    "turquía": "TR", "tuvalu": "TV", "ucrania": "UA", "uganda": "UG",
    "uruguay": "UY", "uzbekistán": "UZ", "vanuatu": "VU", "vaticano": "VA",
    "venezuela": "VE", "vietnam": "VN", "yemen": "YE", "yibuti": "DJ",
    "zambia": "ZM", "zimbabue": "ZW",
}


def country_flag(country_name: Optional[str]) -> str:
    """Convert country name to flag emoji using regional indicator symbols."""
    if not country_name:
        return ""
    
    name = country_name.strip().lower()
    iso_code = COUNTRY_ISO.get(name)
    
    if not iso_code:
        return ""
    
    # Convert ISO code to flag emoji using regional indicator symbols
    # A=🇦 is U+1F1E6, B=🇧 is U+1F1E7, etc.
    return "".join(chr(0x1F1E6 + ord(c) - ord('A')) for c in iso_code.upper())

