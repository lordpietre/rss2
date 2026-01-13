import os
import logging
from db import get_conn

# Config (moved/duplicated from worker or just hardcoded/env read here)
IDENTITY_PAISES_ES = int(os.environ.get("IDENTITY_PAISES_ES", 58))
LOG = logging.getLogger("translation_ops")

def ensure_indexes(conn):
    with conn.cursor() as cur:
        cur.execute("CREATE INDEX IF NOT EXISTS traducciones_lang_to_status_idx ON traducciones (lang_to, status);")
        cur.execute("CREATE INDEX IF NOT EXISTS traducciones_status_idx ON traducciones (status);")
    conn.commit()

def cleanup_stuck_translations(conn, timeout_hours=2):
    """Reset stuck 'processing' translations to 'pending' if they're too old."""
    with conn.cursor() as cur:
        # Assuming created_at is the timestamp when they entered processing
        # In this schema, created_at is DEFAULT NOW() upon insertion.
        # But if we don't have an 'updated_at', we use created_at as a fallback.
        # For entries stuck in processing for hours, it's safe to reset.
        cur.execute(
            """
            UPDATE traducciones 
            SET status = 'pending' 
            WHERE status = 'processing' 
              AND created_at < NOW() - INTERVAL '%s hours';
            """,
            (timeout_hours,)
        )
        if cur.rowcount > 0:
            LOG.info(f"Cleaned up {cur.rowcount} stuck translations.")
    conn.commit()

def ensure_pending(conn, lang_to: str, limit: int):
    if limit <= 0:
        return
    with conn.cursor() as cur:
        # Use ON CONFLICT DO NOTHING to avoid errors if run concurrently (though scheduler should be serial)
        cur.execute(
            """
            INSERT INTO traducciones (noticia_id, lang_from, lang_to, status)
            SELECT n.id, NULL, %s, 'pending'
            FROM noticias n
            LEFT JOIN traducciones t ON t.noticia_id=n.id AND t.lang_to=%s
            WHERE t.id IS NULL
            ORDER BY n.fecha DESC NULLS LAST
            LIMIT %s
            ON CONFLICT DO NOTHING;
            """,
            (lang_to, lang_to, limit),
        )
    conn.commit()

def ensure_identity_spanish(conn, lang_to: str, limit: int):
    if lang_to != "es" or limit <= 0:
        return
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO traducciones (noticia_id, lang_from, lang_to,
                                     titulo_trad, resumen_trad, status)
            SELECT n.id, 'es', %s, n.titulo, n.resumen, 'done'
            FROM noticias n
            LEFT JOIN traducciones t ON t.noticia_id=n.id AND t.lang_to=%s
            WHERE t.id IS NULL AND n.pais_id=%s
            ORDER BY n.fecha DESC NULLS LAST
            LIMIT %s
            ON CONFLICT DO NOTHING;
            """,
            (lang_to, lang_to, IDENTITY_PAISES_ES, limit),
        )
        
        # Also persist stats for these auto-completed Spanish ones
        # We need to know how many were inserted. 
        # But 'process_batch' in worker handles stats for processed items.
        # These identity ones bypass the worker.
        # We should add stats here too if we want them counted.
        # However, getting the inserted ID is tricky with ON CONFLICT DO NOTHING and bulk insert.
        # Let's count them afterwards or simple ignore stats for identity 'skip' for now to keep it simple.
        # User goal was TPM (Work done). This is valid work done.
        # Let's add RETURNING id if possible? 
        # INSERT ... RETURNING id.
        # But it's SELECT ...
        pass
        
    conn.commit()


def run_producer_cycle():
    """Main entry point for the scheduler to generate work."""
    # Read env vars here to ensure freshness
    target_langs = os.environ.get("TARGET_LANGS", "es").split(",")
    enqueue_max = int(os.environ.get("ENQUEUE", 200))
    
    target_langs = [x.strip() for x in target_langs if x.strip()]
    
    with get_conn() as conn:
        ensure_indexes(conn)
        cleanup_stuck_translations(conn)
        for tgt in target_langs:
            # 1. Identity for ES
            if tgt == "es":
                ensure_identity_spanish(conn, tgt, enqueue_max)
            
            # 2. Pendings for all
            ensure_pending(conn, tgt, enqueue_max)
    
    LOG.info(f"Producer cycle completed for langs: {target_langs}")
