import logging
import sys
import os
from concurrent.futures import ThreadPoolExecutor

# Add app to path
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from db import get_read_conn
from utils.wiki import fetch_wiki_data

logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

def get_top_entities():
    """Get top 100 people, 50 orgs, 50 places from last 30 days."""
    entities = []
    query = """
    SELECT t.valor, COUNT(*) as c
    FROM tags t
    JOIN tags_noticia tn ON t.id = tn.tag_id
    JOIN traducciones tr ON tn.traduccion_id = tr.id
    WHERE tr.created_at > NOW() - INTERVAL '30 days'
      AND t.tipo = %s
    GROUP BY t.valor
    ORDER BY c DESC
    LIMIT %s
    """
    
    try:
        with get_read_conn() as conn:
            with conn.cursor() as cur:
                # People
                cur.execute(query, ('persona', 100))
                entities.extend([row[0] for row in cur.fetchall()])
                
                # Orgs
                cur.execute(query, ('organizacion', 50))
                entities.extend([row[0] for row in cur.fetchall()])
                
                # Places
                cur.execute(query, ('lugar', 50))
                entities.extend([row[0] for row in cur.fetchall()])
    except Exception as e:
        logger.error(f"Error fetching top entities: {e}")
        
    return list(set(entities))

def precache_entity(name):
    try:
        img, summary = fetch_wiki_data(name)
        if img or summary:
            logger.info(f"✓ Cached: {name}")
        else:
            logger.info(f"✗ No data for: {name}")
    except Exception as e:
        logger.error(f"Error caching {name}: {e}")

def run_precache():
    logger.info("Starting entity pre-cache...")
    entities = get_top_entities()
    logger.info(f"Found {len(entities)} unique top entities to cache.")
    
    with ThreadPoolExecutor(max_workers=10) as executor:
        executor.map(precache_entity, entities)
    
    logger.info("Pre-cache complete.")

if __name__ == "__main__":
    run_precache()
