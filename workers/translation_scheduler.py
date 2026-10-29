#!/usr/bin/env python3
"""
Translation Scheduler Worker para RSS2.

Crea jobs de traducción para noticias que necesitan ser traducidas.

El worker tiene dos rutas:

  - RUTA RAPIDA (cada ciclo): solo mira noticias insertadas hace poco
    (created_at >= NOW() - LOOKBACK_MINUTES). Con el indice
    idx_noticias_created_at esto es milisegundos.

  - RUTA DE CATCH-UP (cada CATCHUP_INTERVAL_MIN): mira las noticias
    actualizadas hace poco (updated_at). Es la red de seguridad para lo que
    se queda fuera de la ventana rapida: noticias antiguas que langdetect
    acaba de etiquetar (el trigger update_noticias_updated_at refresca
    updated_at en cada UPDATE), o ingestas que se quedaron sin procesar.

Sin esta division, el anti-join NOT EXISTS obligaba a recorrer TODO el
catalogo de noticias para PROBAR que no quedaba ninguna pendiente: con
2,7M de noticias el ciclo tardaba ~12 minutos y la CPU del Postgres era la
tasa limitante del pipeline entero.
"""

import os
import sys
import time
import logging
from datetime import datetime

import psycopg2
from psycopg2.extras import RealDictCursor

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

# Configurable via env, default: 'es,en'
TARGET_LANGS = os.getenv('TARGET_LANGS', 'es,en').split(',')
BATCH_SIZE = int(os.getenv('SCHEDULER_BATCH', '2000'))
SLEEP_INTERVAL = int(os.getenv('SCHEDULER_SLEEP', '30'))
USE_DB_LANGS = os.getenv('USE_DB_LANGS', 'true').lower() == 'true'

# Ventana de la ruta rapida: noticias insertadas en los ultimos N minutos.
LOOKBACK_MINUTES = int(os.getenv('SCHEDULER_LOOKBACK_MINUTES', '60'))
# Cada cuanto se ejecuta la ruta de catch-up (minutos).
CATCHUP_INTERVAL_MIN = int(os.getenv('SCHEDULER_CATCHUP_INTERVAL_MIN', '360'))
# Ventana de la ruta de catch-up (horas): cubre noticias etiquetadas por
# langdetect o ingestadas mientras el worker estaba caido.
CATCHUP_LOOKBACK_HOURS = int(os.getenv('SCHEDULER_CATCHUP_LOOKBACK_HOURS', '168'))
# Lote maximo de jobs en una pasada de catch-up.
CATCHUP_BATCH = int(os.getenv('SCHEDULER_CATCHUP_BATCH', '20000'))

# Códigos ISO 639-1 válidos para langdetect + comunes
# https://en.wikipedia.org/wiki/List_of_ISO_639-1_codes
VALID_LANGS = {
    'en', 'es', 'fr', 'de', 'it', 'pt', 'nl', 'sv', 'da', 'fi', 'no', 'pl',
    'cs', 'sk', 'sl', 'hu', 'ro', 'el', 'ru', 'uk', 'tr', 'ar', 'fa', 'he',
    'zh', 'ja', 'ko', 'vi', 'th', 'id', 'ms', 'tl', 'hi', 'bn', 'ur', 'ml',
    'ta', 'te', 'uk', 'ru', 'bg', 'hr', 'sr', 'et', 'lv', 'lt', 'mk', 'sq',
    'sh', 'ca', 'eu', 'gl', 'cy', 'ga', 'mt', 'oc', 'la'
}

# Códigos conocidos inválidos o problemáticos que NO deben generar jobs
INVALID_LANGS = {'und', 'unknown', '', 'mul', 'zxx', 'mis', 'sgn'}


def get_db_connection():
    return psycopg2.connect(**DB_CONFIG)


def is_valid_lang(lang):
    """Verifica si un código de idioma es válido para traducción."""
    if not lang:
        return False
    lang = lang.strip().lower()[:2]  # Normaliza a 2 chars
    if lang in INVALID_LANGS:
        return False
    if len(lang) != 2:
        return False
    return True


def get_active_langs(conn):
    """Fetch active languages from idiomas_disponibles table."""
    if not USE_DB_LANGS:
        return [l.strip() for l in TARGET_LANGS if l.strip()]

    with conn.cursor() as cur:
        cur.execute("SELECT codigo FROM idiomas_disponibles WHERE activo = TRUE")
        rows = cur.fetchall()
        if not rows:
            logger.warning("No active languages found in idiomas_disponibles, falling back to env TARGET_LANGS")
            return [l.strip() for l in TARGET_LANGS if l.strip()]
        return [row[0].strip() for row in rows]


def create_translation_jobs(conn, window_col, window_value, window_unit, limit):
    """Crea jobs de traducción para noticias sin job en la ventana indicada.

    window_col: 'created_at' (ruta rápida) o 'updated_at' (catch-up).
    Devuelve (creados, ids_creados).
    """
    created = 0
    created_by_lang = {}

    with conn.cursor(cursor_factory=RealDictCursor) as cur:
        active_langs = get_active_langs(conn)

        for lang in active_langs:
            lang = lang.strip()
            if not lang:
                continue

            cur.execute(f"""
                INSERT INTO traducciones (noticia_id, lang_from, lang_to, status, created_at)
                SELECT n.id, n.lang, %s, 'pending', NOW()
                FROM noticias n
                WHERE n.{window_col} >= NOW() - INTERVAL '%s {window_unit}'
                  AND n.lang IS NOT NULL
                  AND TRIM(n.lang) != ''
                  AND n.lang != %s
                  AND LOWER(n.lang) NOT IN ('und', 'unknown', 'mul', 'zxx', 'mis', 'sgn', '')
                  AND LENGTH(TRIM(n.lang)) <= 5
                  AND (n.resumen IS NOT NULL AND n.resumen != ''
                       OR n.contenido IS NOT NULL AND n.contenido != '')
                  AND NOT EXISTS (
                      SELECT 1 FROM traducciones t
                      WHERE t.noticia_id = n.id AND t.lang_to = %s
                  )
                ORDER BY n.{window_col} DESC
                LIMIT %s
                ON CONFLICT (noticia_id, lang_to) DO NOTHING
                RETURNING noticia_id
            """, (lang, window_value, lang, lang, limit))

            rows = cur.fetchall()
            if rows:
                created += len(rows)
                # Los ids se guardan POR IDIOMA: la tabla traducciones_target
                # lleva (noticia_id, lang_to) y con varios idiomas activos
                # registrarlos cruzados metia filas (noticia, 'en') para
                # noticias cuyo job real es solo de 'es'.
                created_by_lang[lang] = [r['noticia_id'] for r in rows]
                logger.info(f"Created {len(rows)} translation jobs for {lang} (window={window_col} {window_value}{window_unit})")

        # Registro en traducciones_target (tabla pivote) de una sola vez por
        # idioma: hacerlo fila a fila era un INSERT por noticia y dominaba el
        # ciclo.
        for lang, ids in created_by_lang.items():
            if not ids:
                continue
            cur.execute("""
                INSERT INTO traducciones_target (noticia_id, lang_to, created_at)
                SELECT unnest(%s::varchar[]), %s, NOW()
                ON CONFLICT (noticia_id, lang_to) DO NOTHING
            """, (ids, lang))
            logger.info(f"Tracked {len(ids)} requested translations in traducciones_target for {lang}")

        conn.commit()

    return created, created_by_lang


def requeue_incomplete(conn, active_langs):
    """Devuelve a 'pending' las traducciones marcadas como 'done' pero incompletas.

    El translator marca 'done' solo si ha escrito los tres campos, pero
    quedan filas 'done' con campos a NULL (p.ej. por una version anterior del
    worker o por una interruption a mitad). Esas filas no las vuelve a pillar
    nadie: el translator solo coge status='pending' y el scheduler no reencola
    porque la fila ya existe. Quedan atascadas para siempre.

    Solo se reencola si hay texto de origen pendiente de traducir: si el
    contenido de origen esta vacio no hay nada que traducir.
    """
    requeued = 0
    with conn.cursor() as cur:
        for lang in active_langs:
            lang = lang.strip()
            if not lang:
                continue
            cur.execute("""
                UPDATE traducciones t
                SET status = 'pending', worker_id = NULL, locked_at = NULL, retry_count = 0
                FROM noticias n
                WHERE n.id = t.noticia_id
                  AND t.lang_to = %s
                  AND t.status = 'done'
                  AND (
                      t.titulo_trad IS NULL
                      OR t.resumen_trad IS NULL
                      OR (t.contenido_trad IS NULL AND n.contenido IS NOT NULL AND n.contenido != '')
                  )
            """, (lang,))
            if cur.rowcount > 0:
                logger.info(f"Requeued {cur.rowcount} incomplete 'done' translations for {lang}")
                requeued += cur.rowcount
        conn.commit()
    return requeued


def process_translations():
    logger.info("Starting translation scheduler loop...")
    logger.info(
        f"Fast path: created_at >= NOW() - {LOOKBACK_MINUTES} min | "
        f"catch-up every {CATCHUP_INTERVAL_MIN} min: updated_at >= NOW() - {CATCHUP_LOOKBACK_HOURS} h"
    )
    last_catchup = 0.0

    while True:
        try:
            conn = get_db_connection()
            active_langs = get_active_langs(conn)

            # --- Ruta rápida: noticias nuevas ---
            t0 = time.time()
            created, _ = create_translation_jobs(
                conn, 'created_at', LOOKBACK_MINUTES, 'minutes', BATCH_SIZE)
            fast_ms = int((time.time() - t0) * 1000)

            # --- Ruta de catch-up: noticias que se han vuelto elegibles ---
            catchup_created = 0
            catchup_ms = 0
            now = time.time()
            if (now - last_catchup) >= CATCHUP_INTERVAL_MIN * 60:
                last_catchup = now
                t1 = time.time()
                catchup_created, _ = create_translation_jobs(
                    conn, 'updated_at', CATCHUP_LOOKBACK_HOURS, 'hours', CATCHUP_BATCH)
                catchup_ms = int((time.time() - t1) * 1000)

            # --- Requeue de traducciones atascadas ---
            requeue_incomplete(conn, active_langs)

            conn.close()

            total = created + catchup_created
            logger.info(
                f"Cycle: {created} new (fast {fast_ms}ms)"
                + (f", {catchup_created} in catch-up ({catchup_ms}ms)" if catchup_created else "")
            )
            if total == 0:
                logger.info(f"No new news to schedule. Sleeping {SLEEP_INTERVAL}s...")
            else:
                logger.info(f"Total jobs created in this cycle: {total}")

        except Exception as e:
            logger.error(f"Scheduler error: {e}")

        time.sleep(SLEEP_INTERVAL)


if __name__ == '__main__':
    logger.info("Translation scheduler started")
    process_translations()
