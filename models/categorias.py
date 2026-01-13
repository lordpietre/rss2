from psycopg2 import extras
from typing import List, Dict


def get_categorias(conn) -> List[Dict]:
    with conn.cursor(cursor_factory=extras.DictCursor) as cur:
        cur.execute("SELECT id, nombre FROM categorias ORDER BY nombre;")
        return cur.fetchall()

