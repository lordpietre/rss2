"""El esquema TTS debe ser el mismo en los tres sitios donde vive:

- ``init-db/57-add-tts.sql`` (fuente canonica para bases nuevas)
- ``migrations/add_tts.sql`` (bases existentes)
- ``workers/tts/schema.py`` (el servicio lo asegura al arrancar)

Si divergen, el servicio podria crear tablas distintas de las que espera el
backend. Es el equivalente Python de ``make parity`` para textclean.
"""

from __future__ import annotations

import os
import re

import pytest

from workers.tts.schema import DDL_STATEMENTS

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

SQL_FILES = [
    os.path.join(REPO_ROOT, "init-db", "57-add-tts.sql"),
    os.path.join(REPO_ROOT, "migrations", "add_tts.sql"),
]

CREATE_TABLE_RE = re.compile(
    r"CREATE TABLE IF NOT EXISTS\s+(\w+)\s*\((.*?)\n\s*\)", re.DOTALL | re.IGNORECASE
)

_CONSTRAINT_KEYWORDS = (
    "primary", "unique", "foreign", "constraint", "check", "references", "default", "not", "null",
)


def extract_tables(sql: str) -> dict:
    """Extrae {tabla: {columnas}} de un script SQL."""
    tables = {}
    for match in CREATE_TABLE_RE.finditer(sql):
        table = match.group(1)
        body = match.group(2)
        columns = set()
        for raw_line in body.split("\n"):
            line = raw_line.strip().rstrip(",")
            if not line or line.startswith("--"):
                continue
            first = line.split()[0].lower()
            if first in _CONSTRAINT_KEYWORDS:
                continue
            columns.add(first)
        tables[table] = columns
    return tables


def extract_tables_from_ddl(statements) -> dict:
    return extract_tables(";\n".join(statements))


EXPECTED_TABLES = {
    "tts_voices",
    "tts_scripts",
    "tts_jobs",
    "tts_audios",
    "tts_bulletins",
    "tts_bulletin_items",
}


@pytest.mark.parametrize("path", SQL_FILES, ids=["init-db", "migrations"])
def test_sql_files_have_the_same_tables(path):
    if not os.path.isfile(path):
        pytest.skip("Fichero de esquema no encontrado: %s" % path)
    with open(path, "r", encoding="utf-8") as handle:
        tables = extract_tables(handle.read())
    assert set(tables) == EXPECTED_TABLES


@pytest.mark.parametrize("path", SQL_FILES, ids=["init-db", "migrations"])
def test_python_ddl_matches_the_sql_files(path):
    if not os.path.isfile(path):
        pytest.skip("Fichero de esquema no encontrado: %s" % path)
    with open(path, "r", encoding="utf-8") as handle:
        sql_tables = extract_tables(handle.read())
    python_tables = extract_tables_from_ddl(DDL_STATEMENTS)

    assert set(python_tables) == set(sql_tables), "Las tablas no coinciden"
    for table, columns in sql_tables.items():
        assert python_tables[table] == columns, "Las columnas de %s no coinciden" % table


def test_the_two_sql_files_are_identical():
    if not all(os.path.isfile(path) for path in SQL_FILES):
        pytest.skip("Ficheros de esquema no encontrados")
    with open(SQL_FILES[0], "r", encoding="utf-8") as handle:
        first = handle.read()
    with open(SQL_FILES[1], "r", encoding="utf-8") as handle:
        second = handle.read()
    assert extract_tables(first) == extract_tables(second)
