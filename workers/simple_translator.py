#!/usr/bin/env python3
import os
import time
import logging
import re
from typing import List, Optional

import psycopg2
import psycopg2.extras
from psycopg2.extras import execute_values
from transformers import pipeline, AutoTokenizer, AutoModelForSeq2SeqLM
import torch

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s: %(message)s")
LOG = logging.getLogger("translator_simple")

DB_CONFIG = {
    "host": os.environ.get("DB_HOST", "localhost"),
    "port": int(os.environ.get("DB_PORT", 5432)),
    "dbname": os.environ.get("DB_NAME", "rss"),
    "user": os.environ.get("DB_USER", "rss"),
    "password": os.environ.get("DB_PASS", "x"),
}

TARGET_LANGS = os.environ.get("TARGET_LANGS", "es").split(",")
BATCH_SIZE = int(os.environ.get("TRANSLATOR_BATCH", 32))
MAX_SRC_TOKENS = 512

TRANSLATORS = {}

LANG_MAP = {
    "en": "en-ES",
    "es": "es-ES", 
    "fr": "fr-ES",
    "de": "de-ES",
    "pt": "pt-ES",
    "it": "it-ES",
    "ru": "ru-ES",
    "ar": "ar-ES",
    "fa": "fa-ES",
    "ps": "ps-ES",
    "zh": "zh-ES",
    "ja": "ja-ES",
    "ko": "ko-ES",
}

def get_translator(source_lang: str, target_lang: str = "es"):
    key = f"{source_lang}_{target_lang}"
    if key in TRANSLATORS:
        return TRANSLATORS[key]
    
    model_name = f"Helsinki-NLP/opus-mt-{source_lang}-{target_lang}"
    if source_lang == target_lang:
        model_name = f"Helsinki-NLP/opus-mt-{source_lang}-es"
    
    LOG.info(f"Loading translator: {model_name}")
    
    try:
        device = 0 if torch.cuda.is_available() else -1
        translator = pipeline("translation", model=model_name, device=device)
        TRANSLATORS[key] = translator
        LOG.info(f"Translator loaded: {key}")
        return translator
    except Exception as e:
        LOG.error(f"Failed to load translator {model_name}: {e}")
        return None

def normalize_lang(lang: Optional[str], default: str = "es") -> Optional[str]:
    if not lang:
        return default
    lang = lang.strip().lower()[:2]
    return lang if lang else default

def translate_text(source_lang: str, target_lang: str, texts: List[str]) -> List[str]:
    if not texts:
        return []
    
    if source_lang == target_lang:
        return texts
    
    translator = get_translator(source_lang, target_lang)
    if not translator:
        return texts
    
    results = []
    for text in texts:
        if not text or not text.strip():
            results.append(text)
            continue
        try:
            result = translator(text[:MAX_SRC_TOKENS], max_length=MAX_SRC_TOKENS)
            translated = result[0]['translation_text']
            results.append(translated)
        except Exception as e:
            LOG.warning(f"Translation error: {e}")
            results.append(text)
    
    return results

def connect_db():
    return psycopg2.connect(**DB_CONFIG)

def process_batch(conn, rows):
    todo = []
    
    for r in rows:
        lang_to = normalize_lang(r.get("lang_to"), "es") or "es"
        lang_from = normalize_lang(r.get("lang_from")) or "en"
        
        titulo = (r.get("titulo") or "").strip()
        resumen = (r.get("resumen") or "").strip()
        
        if lang_from == lang_to:
            continue
            
        todo.append({
            "tr_id": r.get("tr_id"),
            "lang_from": lang_from,
            "lang_to": lang_to,
            "titulo": titulo,
            "resumen": resumen,
        })
    
    if not todo:
        return
    
    from collections import defaultdict
    groups = defaultdict(list)
    for item in todo:
        key = (item["lang_from"], item["lang_to"])
        groups[key].append(item)
    
    for (lang_from, lang_to), items in groups.items():
        LOG.info(f"Translating {lang_from} -> {lang_to} ({len(items)} items)")
        
        titles = [i["titulo"] for i in items]
        translated_titles = translate_text(lang_from, lang_to, titles)
        
        translated_bodies = []
        for i in items:
            body = (i["resumen"] or "").strip()
            if body:
                tr = translate_text(lang_from, lang_to, [body])
                translated_bodies.append(tr[0] if tr else body)
            else:
                translated_bodies.append("")
        
        cursor = conn.cursor()
        for i, (item, tt, tb) in enumerate(zip(items, translated_titles, translated_bodies)):
            tt = (tt or "").strip()
            tb = (tb or "").strip()
            
            if not tt:
                tt = item["titulo"]
            if not tb:
                tb = item["resumen"]
            
            try:
                cursor.execute("""
                    UPDATE traducciones 
                    SET titulo_trad = %s, resumen_trad = %s, lang_to = %s
                    WHERE id = %s
                """, (tt, tb, lang_to, item["tr_id"]))
            except Exception as e:
                LOG.error(f"Update error: {e}")
        
        conn.commit()
        cursor.close()
        LOG.info(f"Translated {len(items)} items")

def fetch_pending_translations(conn):
    cursor = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)
    
    for lang in TARGET_LANGS:
        cursor.execute("""
            SELECT t.id as tr_id, t.lang_from, t.lang_to,
                   n.titulo, n.resumen, n.id as noticia_id
            FROM traducciones t
            JOIN noticias n ON n.id = t.noticia_id
            WHERE t.lang_to = %s 
              AND (t.titulo_trad IS NULL OR t.resumen_trad IS NULL)
            ORDER BY n.fecha DESC
            LIMIT %s
        """, (lang, BATCH_SIZE))
        
        rows = cursor.fetchall()
        if rows:
            LOG.info(f"Found {len(rows)} pending translations for {lang}")
            process_batch(conn, rows)
    
    cursor.close()

def main():
    LOG.info("Simple translator worker started")
    
    while True:
        try:
            conn = connect_db()
            fetch_pending_translations(conn)
            conn.close()
        except Exception as e:
            LOG.error(f"Error: {e}")
        
        time.sleep(30)

if __name__ == "__main__":
    main()
