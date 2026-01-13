from psycopg2 import extras
from typing import Optional, Dict


def get_traduccion(conn, traduccion_id: int) -> Optional[Dict]:
    with conn.cursor(cursor_factory=extras.DictCursor) as cur:
        cur.execute(
            """
            SELECT *
            FROM traducciones
            WHERE id = %s;
            """,
            (traduccion_id,),
        )
        return cur.fetchone()

