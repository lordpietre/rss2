#!/usr/bin/env python3
"""
Comprueba que la base de datos tiene TODO lo que declaran los scripts de
init-db/.

Motivo: init-db/ solo se ejecuta en la PRIMERA inicializacion de
data/pgdata. Una migracion anadida despues (o un trigger que alguien creo a
mano) no se aplica nunca, y el sistema se rompe en silencio: asi se
perdieron las migraciones 51 y 52 (traducciones.confidence/retry_count y las
tablas idiomas_disponibles/traducciones_target) y el trigger
update_noticias_updated_at, que hacia que la ruta de catch-up del scheduler
no encontrara nada.

Uso:
    scripts/check-schema-drift.sh            # contra la BD por defecto
    DB_NAME=... DB_USER=... scripts/check-schema-drift.sh

Sale con codigo 1 si falta algo.
"""
import os
import re
import shutil
import subprocess
import sys
import glob

DB_NAME = os.environ.get("DB_NAME", "rss")
DB_USER = os.environ.get("DB_USER", "rss")
DB_HOST = os.environ.get("DB_HOST", "db")
DB_PASS = os.environ.get("DB_PASS", "rss")

INIT_DIR = os.environ.get("INIT_DB_DIR", "init-db")


def q(sql):
    """Ejecuta una consulta y devuelve las filas.

    Usa el cliente psql local si existe; si no, entra en el contenedor de la
    base de datos con 'docker compose exec' (el contenedor de postgres no
    lleva python, asi que el script se ejecuta en el host).
    """
    args = ["-U", DB_USER, "-d", DB_NAME, "-t", "-A", "-F", "\t", "-c", sql]
    env = dict(os.environ, PGPASSWORD=DB_PASS)
    port = os.environ.get("DB_PORT", "5432")
    if DB_HOST == "db":
        # "db" es un nombre interno de Docker: desde el host no resuelve.
        # La base no publica puerto, asi que se entra por el contenedor.
        cmd = ["docker", "compose", "exec", "-T", "db", "psql"] + args
    else:
        cmd = ["psql", "-h", DB_HOST, "-p", port] + args
    p = subprocess.run(cmd, capture_output=True, text=True, env=env)
    if p.returncode != 0:
        sys.exit(f"Error de Postgres: {p.stderr.strip()}")
    return [line.split("\t") for line in p.stdout.splitlines() if line != ""]


def strip_comments(txt):
    """Quita los comentarios: hay prosa en ingles ('Create trigger for ...')
    que el parser tomaria por declaraciones reales."""
    txt = re.sub(r"/\*.*?\*/", " ", txt, flags=re.S)
    txt = re.sub(r"--[^\n]*", " ", txt)
    return txt


def declared():
    """Objetos declarados en los scripts de init-db."""
    tables, columns, indexes, triggers, functions = set(), set(), set(), set(), set()
    for path in sorted(glob.glob(os.path.join(INIT_DIR, "*.sql"))):
        txt = strip_comments(open(path, encoding="utf-8", errors="replace").read())
        for m in re.finditer(
            r"CREATE\s+(?:TABLE|UNIQUE\s+TABLE|UNLOGGED\s+TABLE)\s+(?:IF\s+NOT\s+EXISTS\s+)?([A-Za-z_][\w.]*)",
            txt, re.I):
            tables.add(m.group(1).split(".")[-1].strip('"'))
        for m in re.finditer(
            r"CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:CONCURRENTLY\s+)?(?:IF\s+NOT\s+EXISTS\s+)?([A-Za-z_][\w.]*)",
            txt, re.I):
            indexes.add(m.group(1).split(".")[-1].strip('"'))
        for m in re.finditer(
            r"CREATE\s+(?:OR\s+REPLACE\s+)?FUNCTION\s+([A-Za-z_][\w.]*)", txt, re.I):
            functions.add(m.group(1).split(".")[-1].strip('"'))
        # ALTER TABLE <t> ADD COLUMN [IF NOT EXISTS] <c>
        for m in re.finditer(
            r"ALTER\s+TABLE\s+([A-Za-z_][\w.]*)\s+ADD\s+COLUMN\s+(?:IF\s+NOT\s+EXISTS\s+)?([A-Za-z_][\w]*)",
            txt, re.I):
            tbl = m.group(1).split(".")[-1].strip('"')
            col = m.group(2).strip('"')
            columns.add((tbl, col))
            tables.add(tbl)
        # CREATE [CONSTRAINT] TRIGGER <n> ... ON <t>
        for m in re.finditer(
            r"CREATE\s+(?:CONSTRAINT\s+)?TRIGGER\s+([A-Za-z_][\w]*)\s+[\s\S]{0,120}?\sON\s+([A-Za-z_][\w.]*)",
            txt, re.I):
            triggers.add((m.group(2).split(".")[-1].strip('"'), m.group(1)))
    return tables, columns, indexes, triggers, functions


def main():
    tables, columns, indexes, triggers, functions = declared()
    problems = []

    have_tables = {r[0] for r in q(
        "SELECT tablename FROM pg_tables WHERE schemaname='public'")}
    for t in sorted(tables):
        if t not in have_tables:
            problems.append(f"tabla ausente: {t}")

    have_cols = {(r[0], r[1]) for r in q(
        "SELECT table_name, column_name FROM information_schema.columns WHERE table_schema='public'")}
    for tbl, col in sorted(columns):
        if (tbl, col) not in have_cols:
            problems.append(f"columna ausente: {tbl}.{col}")

    have_idx = {r[0] for r in q(
        "SELECT indexname FROM pg_indexes WHERE schemaname='public'")}
    for i in sorted(indexes):
        if i not in have_idx:
            problems.append(f"indice ausente: {i}")

    have_trg = {(r[0], r[1]) for r in q(
        "SELECT c.relname, t.tgname FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid "
        "WHERE NOT t.tgisinternal AND c.relnamespace='public'::regnamespace")}
    for tbl, trg in sorted(triggers):
        if (tbl, trg) not in have_trg:
            problems.append(f"trigger ausente: {trg} ON {tbl}")

    have_fn = {r[0] for r in q(
        "SELECT p.proname FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace "
        "WHERE n.nspname='public'")}
    for f in sorted(functions):
        if f not in have_fn:
            problems.append(f"funcion ausente: {f}")

    print(f"Comprobados: {len(tables)} tablas, {len(columns)} columnas, "
          f"{len(indexes)} indices, {len(triggers)} triggers, {len(functions)} funciones")
    if problems:
        print(f"\nDERIVA DE ESQUEMA ({len(problems)}):")
        for p in problems:
            print(f"  - {p}")
        return 1
    print("Sin deriva: la base tiene todo lo que declaran los scripts de init-db/.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
