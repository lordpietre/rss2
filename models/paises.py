from typing import List, Dict
from psycopg2 import extras
from cache import cache_get, cache_set


def get_paises(conn) -> List[Dict]:
    # Intentar desde caché primero (datos casi estáticos)
    cached_data = cache_get("paises:all")
    if cached_data:
        return cached_data
    
    with conn.cursor(cursor_factory=extras.DictCursor) as cur:
        cur.execute("SELECT id, nombre FROM paises ORDER BY nombre;")
        result = cur.fetchall()
    
    # Cachear por 1 hora (son datos estáticos)
    cache_set("paises:all", result, ttl_seconds=3600)
    return result

