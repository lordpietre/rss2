from psycopg2 import extras
from typing import List, Dict, Optional


def get_feed_by_id(conn, feed_id: int) -> Optional[Dict]:
    with conn.cursor(cursor_factory=extras.DictCursor) as cur:
        cur.execute("SELECT * FROM feeds WHERE id = %s;", (feed_id,))
        return cur.fetchone()


def get_feeds_activos(conn) -> List[Dict]:
    """Feeds activos y no caídos, usados por el ingestor RSS."""
    with conn.cursor(cursor_factory=extras.DictCursor) as cur:
        cur.execute(
            """
            SELECT id, nombre, url, categoria_id, pais_id, fallos, activo
            FROM feeds
            WHERE activo = TRUE
              AND (fallos IS NULL OR fallos < 5)
            ORDER BY id;
            """
        )
        return cur.fetchall()

