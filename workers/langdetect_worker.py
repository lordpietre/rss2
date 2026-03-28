#!/usr/bin/env python3
"""
Language Detection Worker
Detects and updates the language of news items in the database.
"""

import os
import sys
import time
import logging
from collections import Counter

import psycopg2
from psycopg2.extras import RealDictCursor
from langdetect import detect, LangDetectException

logging.basicConfig(
    level=logging.INFO,
    format='[%(asctime)s] %(levelname)s - %(message)s',
    datefmt='%Y-%m-%d %H:%M:%S'
)
LOG = logging.getLogger(__name__)

DB_CONFIG = {
    'host': os.getenv('DB_HOST', 'db'),
    'port': int(os.getenv('DB_PORT', 5432)),
    'database': os.getenv('DB_NAME', 'rss'),
    'user': os.getenv('DB_USER', 'rss'),
    'password': os.getenv('DB_PASS', 'rss')
}

BATCH_SIZE = int(os.getenv('LANG_DETECT_BATCH', '1000'))
SLEEP_INTERVAL = int(os.getenv('LANG_DETECT_SLEEP', '60'))

def get_db_connection():
    return psycopg2.connect(**DB_CONFIG)

def detect_language(text):
    if not text or len(text.strip()) < 10:
        return None
    try:
        return detect(text)
    except LangDetectException:
        return None

def process_batch(conn):
    cursor = conn.cursor(cursor_factory=RealDictCursor)
    
    # ONLY pick items where lang is NULL or empty
    cursor.execute("""
        SELECT id, titulo, resumen
        FROM noticias
        WHERE lang IS NULL OR TRIM(lang) = ''
        ORDER BY fecha DESC
        LIMIT %s
    """, (BATCH_SIZE,))
    
    rows = cursor.fetchall()
    if not rows:
        return 0
    
    updated = 0
    lang_stats = Counter()
    
    for row in rows:
        news_id = row['id']
        titulo = (row['titulo'] or "").strip()
        resumen = (row['resumen'] or "").strip()
        
        combined = f"{titulo} {resumen}".strip()
        
        lang = detect_language(combined)
        
        if lang:
            cursor.execute("""
                UPDATE noticias SET lang = %s WHERE id = %s
            """, (lang, news_id))
            lang_stats[lang] += 1
            updated += 1
    
    conn.commit()
    cursor.close()
    
    if updated > 0:
        LOG.info(f"Updated {updated} news languages: {dict(lang_stats)}")
    
    return updated

def main():
    LOG.info("Language detection worker started")
    
    while True:
        try:
            conn = get_db_connection()
            processed = process_batch(conn)
            conn.close()
            
            if processed == 0:
                LOG.info("No more news to process, sleeping...")
                time.sleep(SLEEP_INTERVAL)
            else:
                time.sleep(1)
                
        except Exception as e:
            LOG.error(f"Error: {e}")
            time.sleep(10)

if __name__ == "__main__":
    main()
