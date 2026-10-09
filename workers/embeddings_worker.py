import os
import time
import logging
from typing import List

import numpy as np
import psycopg2
import psycopg2.extras
from psycopg2.extras import execute_values

import torch
from sentence_transformers import SentenceTransformer

from flask import Flask, request, jsonify


# ================================================================
# Logging
# ================================================================
logging.basicConfig(
    level=logging.INFO,
    format='[EMB] %(asctime)s %(levelname)s: %(message)s'
)
log = logging.getLogger("embeddings_worker")


# ================================================================
# Configuración
# ================================================================
DB = dict(
    host=os.environ.get("DB_HOST", "localhost"),
    port=int(os.environ.get("DB_PORT", 5432)),
    dbname=os.environ.get("DB_NAME", "rss"),
    user=os.environ.get("DB_USER", "rss"),
    password=os.environ.get("DB_PASS", "x"),
)

EMB_MODEL = os.environ.get(
    "EMB_MODEL",
    "sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2",
)

EMB_BATCH = int(os.environ.get("EMB_BATCH", "128"))
SLEEP_IDLE = float(os.environ.get("EMB_SLEEP_IDLE", "5.0"))

# ej: "es,en,fr"
EMB_LANGS = [
    s.strip()
    for s in os.environ.get("EMB_LANGS", "es").split(",")
    if s.strip()
]

DEVICE_ENV = os.environ.get("DEVICE", "auto").lower()
EMB_LIMIT = int(os.environ.get("EMB_LIMIT", "1000"))


# ================================================================
# Conexión
# ================================================================
def get_conn():
    return psycopg2.connect(**DB)


# ================================================================
# Esquema — se asegura que exista
# ================================================================
def ensure_schema(conn):
    """
    Asegura que la tabla de embeddings existe. Idempotente.
    """
    with conn.cursor() as cur:
        cur.execute(
            """
            CREATE TABLE IF NOT EXISTS traduccion_embeddings (
                id SERIAL PRIMARY KEY,
                traduccion_id INT NOT NULL REFERENCES traducciones(id) ON DELETE CASCADE,
                model TEXT NOT NULL,
                dim INT NOT NULL,
                embedding DOUBLE PRECISION[] NOT NULL,
                created_at TIMESTAMP DEFAULT NOW(),
                UNIQUE (traduccion_id, model)
            );
            """
        )
        cur.execute("""
            CREATE INDEX IF NOT EXISTS idx_tr_emb_model
                ON traduccion_embeddings(model);
        """)
        cur.execute("""
            CREATE INDEX IF NOT EXISTS idx_tr_emb_traduccion_id
                ON traduccion_embeddings(traduccion_id);
        """)

    conn.commit()


# ================================================================
# Fetch de trabajos pendientes
# ================================================================
def fetch_batch_pending(conn) -> List[psycopg2.extras.DictRow]:
    """
    Obtiene traducciones en status 'done' que aún no tienen embedding
    para este modelo.
    """
    with conn.cursor(cursor_factory=psycopg2.extras.DictCursor) as cur:
        cur.execute(
            """
            SELECT 
                t.id AS traduccion_id,
                t.lang_to AS lang_to,
                COALESCE(NULLIF(t.titulo_trad,''), '') AS titulo_trad,
                COALESCE(NULLIF(t.resumen_trad,''), '') AS resumen_trad,
                n.id AS noticia_id
            FROM traducciones t
            JOIN noticias n ON n.id = t.noticia_id
            LEFT JOIN traduccion_embeddings e
                   ON e.traduccion_id = t.id AND e.model = %s
            WHERE t.status = 'done'
              AND t.lang_to = ANY(%s)
              AND e.traduccion_id IS NULL
            ORDER BY t.id
            LIMIT %s;
            """,
            (EMB_MODEL, EMB_LANGS, EMB_LIMIT),
        )
        return cur.fetchall()


# ================================================================
# Preparación de textos
# ================================================================
def texts_from_rows(rows: List[psycopg2.extras.DictRow]) -> List[str]:
    """
    Devuelve textos combinados para embeddings.
    Evita pasar texto vacío al modelo.
    """
    texts = []
    for r in rows:
        title = (r["titulo_trad"] or "").strip()
        body = (r["resumen_trad"] or "").strip()

        if title and body:
            texts.append(f"{title}\n{body}")
        else:
            texts.append(title or body or "")

    return texts


# ================================================================
# Upsert
# ================================================================
def upsert_embeddings(conn, rows, embs: np.ndarray, model_name: str):
    """
    Inserta o actualiza embeddings en la base de datos.
    Guarda tanto en embedding (DOUBLE PRECISION[]) como en 
    embedding_vector (vector) para compatibilidad y búsqueda.
    """
    if embs.size == 0 or not rows:
        return

    dim = int(embs.shape[1])

    data = [
        (
            int(r["traduccion_id"]),
            model_name,
            dim,
            embs[i].astype(float).tolist(),
        )
        for i, r in enumerate(rows)
    ]

    with conn.cursor() as cur:
        execute_values(
            cur,
            """
            INSERT INTO traduccion_embeddings 
                (traduccion_id, model, dim, embedding)
            VALUES %s
            ON CONFLICT (traduccion_id, model)
            DO UPDATE SET
                embedding  = EXCLUDED.embedding,
                dim        = EXCLUDED.dim,
                created_at = NOW();
            """,
            data,
        )

    conn.commit()

    # Also update the vector column for pgvector search
    # pgvector accepts arrays in format '[val1, val2, ...]'
    for i, r in enumerate(rows):
        embedding_list = embs[i].astype(float).tolist()
        # Format as pgvector string '[val1,val2,...]'
        vector_str = '[' + ','.join(str(v) for v in embedding_list) + ']'
        
        with conn.cursor() as cur:
            cur.execute("""
                UPDATE traduccion_embeddings 
                SET embedding_vector = %s::vector
                WHERE traduccion_id = %s AND model = %s
            """, (vector_str, int(r["traduccion_id"]), model_name))
        
    conn.commit()
    log.info(f"Upserted {len(rows)} embeddings to pgvector column")


# ================================================================
# Load model
# ================================================================
def resolve_device() -> str:
    """
    Determina el dispositivo a usar.
    """
    if DEVICE_ENV in ("cpu", "cuda"):
        if DEVICE_ENV == "cuda" and not torch.cuda.is_available():
            return "cpu"
        return DEVICE_ENV

    # auto
    return "cuda" if torch.cuda.is_available() else "cpu"


def load_model() -> SentenceTransformer:
    """
    Carga el modelo con fallback CPU si CUDA falla.
    """
    device = resolve_device()
    log.info(f"Cargando modelo {EMB_MODEL} en device={device} …")

    try:
        return SentenceTransformer(EMB_MODEL, device=device)
    except Exception as e:
        log.error(f"Fallo cargando modelo en {device}: {e}")

        if device == "cuda":
            log.warning("→ Reintentando en CPU…")
            return SentenceTransformer(EMB_MODEL, device="cpu")

        raise


# ================================================================
# Flask API Server
# ================================================================
app = Flask(__name__)
model = None

@app.route('/health', methods=['GET'])
def health():
    return jsonify({'status': 'ok', 'model': EMB_MODEL})

@app.route('/api/embeddings', methods=['POST'])
def get_embeddings():
    """
    Genera embeddings para una lista de textos.
    
    Body JSON:
    {
        "texts": ["texto1", "texto2"]  // hasta 100 textos
    }
    
    Returns:
    {
        "embeddings": [[0.1, 0.2, ...], [0.3, 0.4, ...]],
        "model": "sentence-transformers/...",
        "dimension": 384
    }
    """
    global model
    
    data = request.get_json()
    if not data or 'texts' not in data:
        return jsonify({'error': 'Missing texts field'}), 400
    
    texts = data['texts']
    if not isinstance(texts, list):
        return jsonify({'error': 'texts must be a list'}), 400
    
    if len(texts) > 100:
        return jsonify({'error': 'Maximum 100 texts per request'}), 400
    
    if not texts:
        return jsonify({'embeddings': [], 'model': EMB_MODEL, 'dimension': 384})
    
    try:
        # Load model if not loaded
        if model is None:
            model = load_model()
        
        # Generate embeddings
        embs = model.encode(
            texts,
            batch_size=len(texts),
            convert_to_numpy=True,
            show_progress_bar=False,
            normalize_embeddings=True,
        )
        
        return jsonify({
            'embeddings': embs.tolist(),
            'model': EMB_MODEL,
            'dimension': int(embs.shape[1])
        })
    except Exception as e:
        log.exception(f"Error generating embeddings: {e}")
        return jsonify({'error': str(e)}), 500


# ================================================================
# Main Worker
# ================================================================
def run_worker():
    log.info(
        f"Iniciando embeddings_worker | model={EMB_MODEL} | batch={EMB_BATCH} | lang={','.join(EMB_LANGS)} | limit={EMB_LIMIT}"
    )

    worker_model = load_model()

    while True:
        try:
            with get_conn() as conn:
                ensure_schema(conn)

                rows = fetch_batch_pending(conn)
                if not rows:
                    time.sleep(SLEEP_IDLE)
                    continue

                texts = texts_from_rows(rows)

                # Encode
                embs = worker_model.encode(
                    texts,
                    batch_size=EMB_BATCH,
                    convert_to_numpy=True,
                    show_progress_bar=False,
                    normalize_embeddings=True,
                )

                # Upsert
                upsert_embeddings(conn, rows, embs, EMB_MODEL)

                log.info(f"Embeddings generados: {len(rows)}")

        except Exception as e:
            log.exception(f"Error en embeddings_worker: {e}")
            time.sleep(SLEEP_IDLE)

def main():
    import threading
    
    # Start worker in background thread
    worker_thread = threading.Thread(target=run_worker, daemon=True)
    worker_thread.start()
    
    # Start Flask server
    log.info("Iniciando API server en :6000")
    app.run(host='0.0.0.0', port=6000, debug=False)


if __name__ == "__main__":
    main()

