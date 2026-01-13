import os
import psycopg2
import logging
from datetime import datetime

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s: %(message)s")
logger = logging.getLogger("recover_system")

DB_CONFIG = {
    "host": os.environ.get("DB_HOST", "localhost"),
    "port": int(os.environ.get("DB_PORT", 5432)),
    "dbname": os.environ.get("DB_NAME", "rss"),
    "user": os.environ.get("DB_USER", "rss"),
    "password": os.environ.get("DB_PASS", "x"),
}

def recover():
    try:
        conn = psycopg2.connect(**DB_CONFIG)
        conn.autocommit = True
        with conn.cursor() as cur:
            # 1. Reset stuck translations
            logger.info("Resetting stuck 'processing' translations to 'pending'...")
            cur.execute("UPDATE traducciones SET status = 'pending' WHERE status = 'processing';")
            logger.info(f"Reset {cur.rowcount} translations.")

            # 2. Correct future-dated news
            logger.info("Correcting future-dated news...")
            now = datetime.utcnow()
            cur.execute("UPDATE noticias SET fecha = %s WHERE fecha > %s;", (now, now))
            logger.info(f"Corrected {cur.rowcount} news items.")

            # 3. Reactivate feeds (Optional - only those with few failures)
            logger.info("Reactivating feeds with 10-29 failures (giving them another chance)...")
            cur.execute("UPDATE feeds SET activo = TRUE, fallos = 0 WHERE activo = FALSE AND fallos < 30;")
            logger.info(f"Reactivated {cur.rowcount} feeds.")

        conn.close()
        logger.info("Recovery complete!")
    except Exception as e:
        logger.error(f"Error during recovery: {e}")

if __name__ == "__main__":
    recover()
