from typing import List, Dict
from psycopg2 import extras


def get_paises(conn) -> List[Dict]:
    with conn.cursor(cursor_factory=extras.DictCursor) as cur:
        cur.execute("SELECT id, nombre FROM paises ORDER BY nombre;")
        return cur.fetchall()

