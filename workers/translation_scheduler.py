#!/usr/bin/env python3
"""
Translation Scheduler Worker
Creates translation jobs for news that need to be translated.
"""

import os
import sys
import time
import logging
from datetime import datetime

import psycopg2
from psycopg2.extras import RealDictCursor
from langdetect import detect, LangDetectException

logging.basicConfig(
    level=logging.INFO,
    format='[%(asctime)s] %(levelname)s - %(message)s',
    datefmt='%Y-%m-%d %H:%M:%S'
)
logger = logging.getLogger(__name__)

DB_CONFIG = {
    'host': os.getenv('DB_HOST', 'db'),
    'port': int(os.getenv('DB_PORT', 5432)),
    'database': os.getenv('DB_NAME', 'rss'),
    'user': os.getenv('DB_USER', 'rss'),
    'password': os.getenv('DB_PASS', 'rss')
}

TARGET_LANGS = os.getenv('TARGET_LANGS', 'es').split(',')
BATCH_SIZE = int(os.getenv('SCHEDULER_BATCH', '2000'))
SLEEP_INTERVAL = int(os.getenv('SCHEDULER_SLEEP', '30'))

# Common source languages to try
SOURCE_LANGS = ['en', 'fr', 'pt', 'de', 'it', 'ru', 'zh', 'ja', 'ar', 'nl', 'pl', 'sv']

def get_db_connection():
    return psycopg2.connect(**DB_CONFIG)

def create_translation_jobs(conn):
    """Create translation jobs for news without translations.
    Relies on langdetect_worker to have set the 'lang' column.
    """
    created = 0
    
    with conn.cursor(cursor_factory=RealDictCursor) as cur:
        for lang in TARGET_LANGS:
            lang = lang.strip()
            if not lang:
                continue
                
            # Insert translation jobs for news that have a detected language
            # but don't have a translation record for the target language.
            cur.execute("""
                INSERT INTO traducciones (noticia_id, lang_from, lang_to, status, created_at)
                SELECT n.id, n.lang, %s, 'pending', NOW()
                FROM noticias n
                WHERE n.lang IS NOT NULL 
                  AND TRIM(n.lang) != ''
                  AND n.lang != %s
                  AND NOT EXISTS (
                      SELECT 1 FROM traducciones t 
                      WHERE t.noticia_id = n.id AND t.lang_to = %s
                  )
                ORDER BY n.fecha DESC
                LIMIT %s
                ON CONFLICT (noticia_id, lang_to) DO NOTHING
                RETURNING noticia_id
            """, (lang, lang, lang, BATCH_SIZE))
            
            rows = cur.fetchall()
            if rows:
                created += len(rows)
                logger.info(f"Created {len(rows)} translation jobs for {lang}")
                
        conn.commit()
    
    return created

def process_translations():
    logger.info("Starting translation scheduler loop...")
    
    while True:
        try:
            conn = get_db_connection()
            created = create_translation_jobs(conn)
            conn.close()
            
            if created == 0:
                logger.info(f"No new news to schedule. Sleeping {SLEEP_INTERVAL}s...")
                time.sleep(SLEEP_INTERVAL)
            else:
                logger.info(f"Total jobs created in this cycle: {created}")
                # Short sleep to avoid hammer but keep momentum
                time.sleep(5)
                
        except Exception as e:
            logger.error(f"Scheduler error: {e}")
            time.sleep(10)

if __name__ == '__main__':
    logger.info("Translation scheduler started")
    process_translations()
