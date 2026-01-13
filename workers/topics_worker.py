import os
import time
import logging
import json
import psycopg2
from psycopg2.extras import execute_values

# Logging
logging.basicConfig(
    level=logging.INFO,
    format='[topics_worker] %(asctime)s %(levelname)s: %(message)s'
)
log = logging.getLogger(__name__)

# Config
DB_CONFIG = {
    "host": os.environ.get("DB_HOST", "localhost"),
    "port": int(os.environ.get("DB_PORT", 5432)),
    "dbname": os.environ.get("DB_NAME", "rss"),
    "user": os.environ.get("DB_USER", "rss"),
    "password": os.environ.get("DB_PASS", "x"),
}

SLEEP_IDLE = 10
BATCH_SIZE = 500

def get_conn():
    return psycopg2.connect(**DB_CONFIG)

def load_topics(conn):
    """
    Load topics and heir keywords.
    Returns list of dicts: [{'id': 1, 'weight': 5, 'keywords': ['foo', 'bar']}]
    """
    with conn.cursor() as cur:
        cur.execute("SELECT id, weight, keywords FROM topics")
        rows = cur.fetchall()
    
    topics = []
    for r in rows:
        tid, weight, kw_str = r
        if not kw_str:
            continue
        # Keywords are comma separated based on insert script
        kws = [k.strip().lower() for k in kw_str.split(",") if k.strip()]
        topics.append({
            "id": tid,
            "weight": weight,
            "keywords": kws
        })
    return topics


def load_countries(conn):
    """
    Load countries.
    Returns list: [{'id': 1, 'name': 'España', 'keywords': ['españa', 'madrid']}]
    """
    with conn.cursor() as cur:
        cur.execute("SELECT id, nombre FROM paises")
        rows = cur.fetchall()
    
    countries = []
    # Hardcoded aliases for simplicity. A separate table would be better.
    ALIASES = {
        "Estados Unidos": ["eeuu", "ee.uu.", "usa", "estadounidense", "washington"],
        "Rusia": ["ruso", "rusa", "moscú", "kremlin"],
        "China": ["chino", "china", "pekin", "beijing"],
        "Ucrania": ["ucraniano", "kiev", "kyiv"],
        "Israel": ["israelí", "tel aviv", "jerusalén"],
        "España": ["español", "madrid"],
        "Reino Unido": ["uk", "londres", "británico"],
        "Francia": ["francés", "parís"],
        "Alemania": ["alemán", "berlín"],
        "Palestina": ["palestino", "gaza", "cisjordania"],
        "Irán": ["iraní", "teherán"],
    }
    
    for r in rows:
        cid, name = r
        kws = [name.lower()]
        if name in ALIASES:
            kws.extend(ALIASES[name])
        countries.append({"id": cid, "name": name, "keywords": kws})
    return countries

def process_batch(conn, topics, countries):
    """
    Fetch batch of processed=False news.
    Match against topics AND countries.
    Insert into news_topics.
    Mark processed.
    """
    with conn.cursor() as cur:
        # Fetch news
        cur.execute("""
            SELECT id, titulo, resumen 
            FROM noticias 
            WHERE topics_processed = FALSE 
            ORDER BY fecha DESC 
            LIMIT %s
        """, (BATCH_SIZE,))
        news_items = cur.fetchall()

    if not news_items:
        return 0

    inserts = [] # (noticia_id, topic_id, score)
    processed_ids = []
    
    # Batch updates for pais_id
    country_updates = [] # (pais_id, noticia_id)

    for item in news_items:
        nid, titulo, resumen = item
        text = (titulo or "") + " " + (resumen or "")
        text_lower = text.lower()
        
        # 1. Match Topics
        for topic in topics:
            matched_count = 0
            for kw in topic["keywords"]:
                if kw in text_lower:
                    matched_count += 1
            
            if matched_count > 0:
                score = topic["weight"] * matched_count
                inserts.append((nid, topic["id"], score))

        # 2. Match Country (Find best match)
        best_country = None
        # Simple heuristic: First found? Or count matches?
        # Let's count matches.
        max_matches = 0
        
        for c in countries:
            matches = 0
            for kw in c["keywords"]:
                # simple word matching. can be improved with regex word boundaries
                if kw in text_lower:
                    matches += 1
            
            if matches > max_matches:
                max_matches = matches
                best_country = c["id"]
        
        if best_country:
            country_updates.append((best_country, nid))

        processed_ids.append(nid)

    with conn.cursor() as cur:
        # Insert relations
        if inserts:
            execute_values(cur, """
                INSERT INTO news_topics (noticia_id, topic_id, score)
                VALUES %s
                ON CONFLICT (noticia_id, topic_id) DO UPDATE SET score = EXCLUDED.score
            """, inserts)
        
        # Update Countries
        if country_updates:
            execute_values(cur, """
                UPDATE noticias AS n
                SET pais_id = v.pais_id
                FROM (VALUES %s) AS v(pais_id, noticia_id)
                WHERE n.id = v.noticia_id
            """, country_updates)
        
        # Mark processed
        cur.execute("""
            UPDATE noticias 
            SET topics_processed = TRUE 
            WHERE id = ANY(%s)
        """, (processed_ids,))
        
    conn.commit()
    return len(news_items)

def initialize_schema(conn):
    """
    Ensure required tables and columns exist.
    """
    log.info("Checking/Initializing schema...")
    with conn.cursor() as cur:
        cur.execute("""
            CREATE TABLE IF NOT EXISTS topics (
                id SERIAL PRIMARY KEY,
                slug VARCHAR(50) UNIQUE NOT NULL,
                name VARCHAR(100) NOT NULL,
                weight INTEGER DEFAULT 1,
                keywords TEXT,
                group_name VARCHAR(50)
            );
            CREATE TABLE IF NOT EXISTS news_topics (
                noticia_id VARCHAR(32) REFERENCES noticias(id) ON DELETE CASCADE,
                topic_id INTEGER REFERENCES topics(id) ON DELETE CASCADE,
                score INTEGER DEFAULT 0,
                created_at TIMESTAMP DEFAULT NOW(),
                PRIMARY KEY (noticia_id, topic_id)
            );
            ALTER TABLE noticias ADD COLUMN IF NOT EXISTS topics_processed BOOLEAN DEFAULT FALSE;
        """)
        conn.commit()
    log.info("Schema OK.")

def main():
    log.info("Starting topics_worker...")
    
    # Run migrations once at startup
    try:
        with get_conn() as conn:
            initialize_schema(conn)
    except Exception as e:
        log.error(f"Error during schema initialization: {e}")
        # We might want to exit here if the schema is crucial
        # sys.exit(1)

    while True:
        try:
            with get_conn() as conn:

                topics = load_topics(conn)
                if not topics:
                    log.warning("No topics found in DB. Sleeping.")
                    time.sleep(SLEEP_IDLE)
                    continue
                
                # Load countries
                countries = load_countries(conn)
                
                count = process_batch(conn, topics, countries)

                if count < BATCH_SIZE:
                     time.sleep(SLEEP_IDLE)
                else:
                    log.info(f"Processed {count} items.")
                    
        except Exception as e:
            log.exception("Error in topics_worker")
            time.sleep(SLEEP_IDLE)

if __name__ == "__main__":
    main()
