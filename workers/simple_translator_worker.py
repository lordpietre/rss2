#!/usr/bin/env python3
"""
Simple Translation Worker using deep-translator
Uses free translation APIs (Google, LibreTranslate, etc.)
"""

import os
import sys
import time
import logging
from datetime import datetime

import psycopg2
from psycopg2.extras import RealDictCursor
from deep_translator import GoogleTranslator, MyMemoryTranslator, single_detection

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

TARGET_LANG = os.getenv('TARGET_LANGS', 'es').split(',')[0].strip()
BATCH_SIZE = int(os.getenv('TRANSLATOR_BATCH', '32'))
SLEEP_INTERVAL = int(os.getenv('TRANSLATOR_SLEEP', '60'))

def get_db_connection():
    return psycopg2.connect(**DB_CONFIG)

def get_pending_translations(conn):
    with conn.cursor(cursor_factory=RealDictCursor) as cur:
        cur.execute("""
            SELECT n.id, n.feed_id, n.lang, n.titulo, n.resumen
            FROM noticias n
            WHERE NOT EXISTS (
                SELECT 1 FROM traducciones t 
                WHERE t.noticia_id = n.id AND t.lang_to = %s
            )
            AND n.lang IS NOT NULL
            AND n.lang != %s
            ORDER BY n.created_at DESC
            LIMIT %s
        """, (TARGET_LANG, TARGET_LANG, BATCH_SIZE))
        return cur.fetchall()

def detect_language(text):
    """Detect language using MyMemory (free API)"""
    try:
        if text and len(text.strip()) > 10:
            lang = single_detection(text, api_key=None)
            return lang
    except Exception as e:
        logger.debug(f"Language detection failed: {e}")
    return 'en'

def translate_text(text, source_lang, target_lang):
    """Translate text using Google Translator (via deep-translator)"""
    if not text or not text.strip():
        return ""
    
    try:
        translator = GoogleTranslator(source=source_lang, target=target_lang)
        translated = translator.translate(text)
        return translated if translated else text
    except Exception as e:
        logger.warning(f"Google translation failed: {e}")
        
        # Fallback to MyMemory
        try:
            translator = MyMemoryTranslator(source=source_lang, target=target_lang)
            translated = translator.translate(text)
            return translated if translated else text
        except Exception as e2:
            logger.error(f"MyMemory translation also failed: {e2}")
            return text

def save_translation(conn, noticia_id, lang_from, titulo, resumen):
    titulo_trad = translate_text(titulo, lang_from, TARGET_LANG)
    resumen_trad = translate_text(resumen, lang_from, TARGET_LANG) if resumen else ""
    
    with conn.cursor() as cur:
        cur.execute("""
            INSERT INTO traducciones (noticia_id, lang_from, lang_to, titulo_trad, resumen_trad, status, created_at)
            VALUES (%s, %s, %s, %s, %s, 'done', NOW())
            ON CONFLICT (noticia_id, lang_to) DO UPDATE SET
                titulo_trad = EXCLUDED.titulo_trad,
                resumen_trad = EXCLUDED.resumen_trad,
                status = 'done'
        """, (noticia_id, lang_from, TARGET_LANG, titulo_trad, resumen_trad))
    conn.commit()

def process_translations():
    logger.info("Starting translation worker...")
    
    while True:
        conn = get_db_connection()
        try:
            pending = get_pending_translations(conn)
            
            if not pending:
                logger.info(f"No pending translations. Sleeping {SLEEP_INTERVAL}s...")
                time.sleep(SLEEP_INTERVAL)
                continue
            
            logger.info(f"Found {len(pending)} pending translations")
            
            for item in pending:
                try:
                    lang = item['lang']
                    
                    # Auto-detect language if needed
                    if not lang or lang == '':
                        lang = detect_language(item['titulo'] or '')
                        logger.info(f"Detected language: {lang} for news {item['id']}")
                    
                    # Skip if already target language
                    if lang == TARGET_LANG:
                        logger.debug(f"Skipping news {item['id']} - already in target language")
                        continue
                    
                    save_translation(
                        conn, 
                        item['id'], 
                        lang,
                        item['titulo'], 
                        item['resumen']
                    )
                    logger.info(f"Translated news {item['id']}: {item['titulo'][:50]}...")
                    
                except Exception as e:
                    logger.error(f"Error translating news {item['id']}: {e}")
                    continue
                    
        except Exception as e:
            logger.error(f"Database error: {e}")
            time.sleep(5)
        finally:
            conn.close()

if __name__ == '__main__':
    logger.info(f"Translation worker started. Target: {TARGET_LANG}")
    process_translations()
