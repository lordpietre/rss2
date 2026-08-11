import os
import time
import logging
from typing import List, Dict, Any, Optional, Tuple

import numpy as np
import psycopg2
import psycopg2.extras
from psycopg2.extras import Json, execute_values


# -------------------------------------------------------------
# LOGGING
# -------------------------------------------------------------
logging.basicConfig(
    level=logging.INFO,
    format='[cluster_worker] %(asctime)s %(levelname)s: %(message)s'
)
log = logging.getLogger(__name__)


# -------------------------------------------------------------
# CONFIG
# -------------------------------------------------------------
DB = dict(
    host=os.environ.get("DB_HOST", "localhost"),
    port=int(os.environ.get("DB_PORT", 5432)),
    dbname=os.environ.get("DB_NAME", "rss"),
    user=os.environ.get("DB_USER", "rss"),
    password=os.environ.get("DB_PASS", "x"),
)

EVENT_LANGS = [
    s.strip().lower()
    for s in os.environ.get("EVENT_LANGS", "es").split(",")
    if s.strip()
]

EVENT_BATCH_IDS = int(os.environ.get("EVENT_BATCH_IDS", "200"))
EVENT_SLEEP_IDLE = float(os.environ.get("EVENT_SLEEP_IDLE", "5.0"))
EVENT_DIST_THRESHOLD = float(os.environ.get("EVENT_DIST_THRESHOLD", "0.25"))

EMB_MODEL = os.environ.get(
    "EMB_MODEL",
    "sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2",
)


# -------------------------------------------------------------
# DB CONNECTION
# -------------------------------------------------------------
def get_conn():
    return psycopg2.connect(**DB)


# -------------------------------------------------------------
# SCHEMA CHECK
# -------------------------------------------------------------
def ensure_schema(conn):
    """Crea índices si no existen (seguro en producción)."""
    with conn.cursor() as cur:
        cur.execute("""
            CREATE INDEX IF NOT EXISTS idx_traducciones_evento
            ON traducciones(evento_id);
        """)
        cur.execute("""
            CREATE INDEX IF NOT EXISTS idx_traducciones_evento_fecha
            ON traducciones(evento_id, noticia_id);
        """)
    conn.commit()


# -------------------------------------------------------------
# FETCH PENDING
# -------------------------------------------------------------
def fetch_pending_traducciones(conn) -> List[int]:
    """Traducciones completadas sin evento asignado pero con embedding."""
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT t.id
            FROM traducciones t
            JOIN traduccion_embeddings e
              ON e.traduccion_id = t.id
             AND e.model = %s
            WHERE t.status = 'done'
              AND t.evento_id IS NULL
              AND t.lang_to = ANY(%s)
            ORDER BY t.id DESC
            LIMIT %s;
            """,
            (EMB_MODEL, EVENT_LANGS, EVENT_BATCH_IDS),
        )
        rows = cur.fetchall()
    return [r[0] for r in rows]


# -------------------------------------------------------------
# FETCH EMBEDDINGS
# -------------------------------------------------------------
def fetch_embeddings_for(conn, tr_ids: List[int]) -> Dict[int, np.ndarray]:
    """Obtiene embeddings como vectores float32, validados y normales."""
    if not tr_ids:
        return {}

    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT traduccion_id, embedding
            FROM traduccion_embeddings
            WHERE traduccion_id = ANY(%s)
              AND model = %s;
            """,
            (tr_ids, EMB_MODEL),
        )
        rows = cur.fetchall()

    out = {}
    for tr_id, emb in rows:
        if not emb:
            continue

        try:
            arr = np.asarray(emb, dtype=np.float32)
            if arr.ndim != 1 or arr.size == 0:
                continue
            if np.isnan(arr).any():
                continue

            norm = np.linalg.norm(arr)
            if norm > 0:
                arr = arr / norm

            out[int(tr_id)] = arr

        except Exception:
            continue

    return out


# -------------------------------------------------------------
# FETCH CENTROIDS (optimized with matrix)
# -------------------------------------------------------------
class CentroidIndex:
    """Índice vectorizado para búsqueda rápida de centroides."""
    
    def __init__(self):
        self.centroids: List[Dict[str, Any]] = []
        self._matrix: Optional[np.ndarray] = None
        self._ids: List[int] = []
    
    def load_from_db(self, conn):
        """Carga centroides de la BD."""
        with conn.cursor(cursor_factory=psycopg2.extras.DictCursor) as cur:
            cur.execute("""
                SELECT id, centroid, total_traducciones
                FROM eventos
                ORDER BY id;
            """)
            rows = cur.fetchall()

        self.centroids = []
        vectors = []
        
        for r in rows:
            raw = r["centroid"]
            if not isinstance(raw, list):
                continue

            try:
                arr = np.asarray(raw, dtype=np.float32)
                if arr.ndim != 1 or arr.size == 0:
                    continue
                if np.isnan(arr).any():
                    continue

                norm = np.linalg.norm(arr)
                if norm > 0:
                    arr = arr / norm

                self.centroids.append({
                    "id": int(r["id"]),
                    "vec": arr,
                    "n": int(r["total_traducciones"] or 1),
                })
                vectors.append(arr)

            except Exception:
                continue
        
        # Build matrix for vectorized search
        if vectors:
            self._matrix = np.vstack(vectors)
            self._ids = [c["id"] for c in self.centroids]
        else:
            self._matrix = None
            self._ids = []
    
    def find_nearest(self, vec: np.ndarray) -> Tuple[Optional[int], float]:
        """Encuentra el centroide más cercano usando operaciones vectorizadas."""
        if self._matrix is None or len(self.centroids) == 0:
            return None, 1.0
        
        # Vectorized cosine similarity: dot product with normalized vectors
        similarities = self._matrix @ vec
        best_idx = int(np.argmax(similarities))
        best_sim = float(similarities[best_idx])
        best_dist = 1.0 - max(-1.0, min(1.0, best_sim))
        
        return best_idx, best_dist
    
    def add_centroid(self, evento_id: int, vec: np.ndarray):
        """Añade un nuevo centroide al índice."""
        self.centroids.append({"id": evento_id, "vec": vec.copy(), "n": 1})
        
        if self._matrix is None:
            self._matrix = vec.reshape(1, -1)
        else:
            self._matrix = np.vstack([self._matrix, vec])
        self._ids.append(evento_id)
    
    def update_centroid(self, idx: int, new_vec: np.ndarray, new_n: int):
        """Actualiza un centroide existente."""
        self.centroids[idx]["vec"] = new_vec
        self.centroids[idx]["n"] = new_n
        if self._matrix is not None:
            self._matrix[idx] = new_vec


# -------------------------------------------------------------
# BATCH FETCH TRADUCCION INFO
# -------------------------------------------------------------
def fetch_traducciones_info_batch(conn, tr_ids: List[int]) -> Dict[int, Dict[str, Any]]:
    """Obtiene info de múltiples traducciones en una sola consulta."""
    if not tr_ids:
        return {}
    
    with conn.cursor(cursor_factory=psycopg2.extras.DictCursor) as cur:
        cur.execute(
            """
            SELECT
                t.id AS traduccion_id,
                t.noticia_id,
                n.fecha,
                COALESCE(NULLIF(t.titulo_trad,''), n.titulo) AS titulo_evento
            FROM traducciones t
            JOIN noticias n ON n.id = t.noticia_id
            WHERE t.id = ANY(%s);
            """,
            (tr_ids,),
        )
        rows = cur.fetchall()

    result = {}
    for row in rows:
        tr_id = int(row["traduccion_id"])
        result[tr_id] = {
            "traduccion_id": tr_id,
            "noticia_id": row["noticia_id"],
            "fecha": row["fecha"],
            "titulo_evento": (row["titulo_evento"] or "")[:255],
        }
    return result


# -------------------------------------------------------------
# BATCH PROCESSING
# -------------------------------------------------------------
def process_batch_optimized(
    conn,
    pending_ids: List[int],
    emb_by_tr: Dict[int, np.ndarray],
    centroid_index: CentroidIndex,
) -> int:
    """Procesa un batch completo con operaciones optimizadas."""
    
    # 1. Fetch all traduccion info in one query
    infos = fetch_traducciones_info_batch(conn, pending_ids)
    
    # Prepare batch operations
    new_eventos = []  # (vec, info) for new eventos
    assign_existing = []  # (tr_id, evento_id, idx, vec, info)
    assign_new = []  # (tr_id, vec, info) - will get evento_id after insert
    
    processed = 0
    
    for tr_id in pending_ids:
        vec = emb_by_tr.get(tr_id)
        if vec is None:
            continue
        
        info = infos.get(tr_id)
        if not info:
            continue
        
        processed += 1
        
        if len(centroid_index.centroids) == 0:
            # First event ever
            assign_new.append((tr_id, vec, info))
        else:
            best_idx, best_dist = centroid_index.find_nearest(vec)
            
            if best_idx is not None and best_dist <= EVENT_DIST_THRESHOLD:
                assign_existing.append((tr_id, centroid_index.centroids[best_idx]["id"], best_idx, vec, info))
            else:
                assign_new.append((tr_id, vec, info))
    
    with conn.cursor() as cur:
        # 2. Insert new eventos in batch
        new_evento_ids = {}
        for tr_id, vec, info in assign_new:
            cur.execute(
                """
                INSERT INTO eventos (centroid, total_traducciones,
                                     fecha_inicio, fecha_fin, n_noticias, titulo)
                VALUES (%s, 1, %s, %s, 1, %s)
                RETURNING id;
                """,
                (
                    Json(vec.tolist()),
                    info["fecha"],
                    info["fecha"],
                    info["titulo_evento"],
                ),
            )
            new_id = cur.fetchone()[0]
            new_evento_ids[tr_id] = new_id
            centroid_index.add_centroid(new_id, vec)
        
        # 3. Update existing eventos and centroids
        for tr_id, evento_id, idx, vec, info in assign_existing:
            c = centroid_index.centroids[idx]
            n_old = c["n"]
            n_new = n_old + 1
            
            new_vec = (c["vec"] * n_old + vec) / float(n_new)
            norm = np.linalg.norm(new_vec)
            if norm > 0:
                new_vec = new_vec / norm
            
            centroid_index.update_centroid(idx, new_vec, n_new)
            
            cur.execute(
                """
                UPDATE eventos
                SET centroid = %s,
                    total_traducciones = total_traducciones + 1,
                    fecha_inicio = LEAST(fecha_inicio, %s),
                    fecha_fin = GREATEST(fecha_fin, %s),
                    n_noticias = n_noticias + 1
                WHERE id = %s;
                """,
                (Json(new_vec.tolist()), info["fecha"], info["fecha"], evento_id),
            )
        
        # 4. Batch update traducciones.evento_id
        trad_updates = []
        for tr_id, evento_id, _, _, _ in assign_existing:
            trad_updates.append((evento_id, tr_id))
        for tr_id, _, _ in assign_new:
            trad_updates.append((new_evento_ids[tr_id], tr_id))
        
        if trad_updates:
            execute_values(
                cur,
                """
                UPDATE traducciones AS t
                SET evento_id = v.evento_id
                FROM (VALUES %s) AS v(evento_id, id)
                WHERE t.id = v.id;
                """,
                trad_updates,
            )
        
        # 5. Batch insert eventos_noticias
        en_inserts = []
        for tr_id, evento_id, _, _, info in assign_existing:
            if info.get("noticia_id"):
                en_inserts.append((evento_id, info["noticia_id"], info["traduccion_id"]))
        for tr_id, _, info in assign_new:
            if info.get("noticia_id"):
                en_inserts.append((new_evento_ids[tr_id], info["noticia_id"], info["traduccion_id"]))
        
        if en_inserts:
            execute_values(
                cur,
                """
                INSERT INTO eventos_noticias (evento_id, noticia_id, traduccion_id)
                VALUES %s
                ON CONFLICT DO NOTHING;
                """,
                en_inserts,
            )
    
    return processed


# -------------------------------------------------------------
# MAIN LOOP
# -------------------------------------------------------------
def main():
    log.info(
        "Iniciando cluster_worker (optimized) langs=%s batch=%d threshold=%.3f emb=%s",
        ",".join(EVENT_LANGS),
        EVENT_BATCH_IDS,
        EVENT_DIST_THRESHOLD,
        EMB_MODEL,
    )

    while True:
        try:
            with get_conn() as conn:
                ensure_schema(conn)

                pending_ids = fetch_pending_traducciones(conn)
                if not pending_ids:
                    time.sleep(EVENT_SLEEP_IDLE)
                    continue

                emb_by_tr = fetch_embeddings_for(conn, pending_ids)
                if not emb_by_tr:
                    time.sleep(EVENT_SLEEP_IDLE)
                    continue

                # Load centroids with vectorized index
                centroid_index = CentroidIndex()
                centroid_index.load_from_db(conn)

                # Process batch with optimizations
                t0 = time.time()
                processed = process_batch_optimized(conn, pending_ids, emb_by_tr, centroid_index)
                dt = time.time() - t0

                conn.commit()
                log.info("Cluster OK: %d procesadas en %.2fs (%.1f/s)", 
                         processed, dt, processed / dt if dt > 0 else 0)

        except Exception:
            log.exception("Error en cluster_worker")
            time.sleep(EVENT_SLEEP_IDLE)


if __name__ == "__main__":
    main()

