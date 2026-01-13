import os
import time
import logging
from typing import List, Tuple

import numpy as np
import psycopg2
import psycopg2.extras

logging.basicConfig(
    level=logging.INFO,
    format='[related] %(asctime)s %(levelname)s: %(message)s'
)

DB = dict(
    host=os.environ.get("DB_HOST", "localhost"),
    port=int(os.environ.get("DB_PORT", 5432)),
    dbname=os.environ.get("DB_NAME", "rss"),
    user=os.environ.get("DB_USER", "rss"),
    password=os.environ.get("DB_PASS", "x"),
)

EMB_MODEL = os.environ.get(
    "EMB_MODEL",
    "sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2"
)

TOPK = int(os.environ.get("RELATED_TOPK", 10))
BATCH_IDS = int(os.environ.get("RELATED_BATCH_IDS", 200))
SLEEP_IDLE = float(os.environ.get("RELATED_SLEEP", 10))
MIN_SCORE = float(os.environ.get("RELATED_MIN_SCORE", 0.0))
WINDOW_HOURS = int(os.environ.get("RELATED_WINDOW_H", 0))


def get_conn():
    return psycopg2.connect(**DB)


def fetch_all_embeddings(cur) -> Tuple[List[int], np.ndarray]:
    sql = """
        SELECT e.traduccion_id, e.embedding, n.fecha
        FROM traduccion_embeddings e
        JOIN traducciones t ON t.id = e.traduccion_id
        JOIN noticias n ON n.id = t.noticia_id
        WHERE e.model = %s
          AND t.status = 'done'
          AND t.lang_to = 'es'
    """
    params = [EMB_MODEL]

    if WINDOW_HOURS > 0:
        sql += " AND n.fecha >= NOW() - INTERVAL %s"
        params.append(f"{WINDOW_HOURS} hours")

    cur.execute(sql, params)
    rows = cur.fetchall()

    if not rows:
        return [], None

    ids = []
    vecs = []

    for tr_id, emb, _ in rows:
        if not emb:
            continue
        arr = np.asarray(emb, dtype=np.float32)
        if arr.ndim != 1 or arr.size == 0:
            continue
        ids.append(tr_id)
        vecs.append(arr)

    if not ids:
        return [], None

    mat = np.vstack(vecs)
    norms = np.linalg.norm(mat, axis=1, keepdims=True)
    norms[norms == 0] = 1e-8
    mat = mat / norms

    return ids, mat


def fetch_pending_ids(cur, limit) -> List[int]:
    cur.execute(
        """
        SELECT t.id
        FROM traducciones t
        JOIN traduccion_embeddings e
             ON e.traduccion_id = t.id AND e.model = %s
        LEFT JOIN related_noticias r
               ON r.traduccion_id = t.id
        WHERE t.lang_to = 'es'
          AND t.status = 'done'
        GROUP BY t.id
        HAVING COUNT(r.related_traduccion_id) = 0
        ORDER BY t.id DESC
        LIMIT %s;
        """,
        (EMB_MODEL, limit),
    )
    return [r[0] for r in cur.fetchall()]


def topk(idx: int, ids_all: List[int], mat: np.ndarray, K: int) -> List[Tuple[int, float]]:
    q = mat[idx]
    sims = np.dot(mat, q)
    sims[idx] = -999.0

    if MIN_SCORE > 0:
        mask = sims >= MIN_SCORE
        sims = np.where(mask, sims, -999.0)

    if K >= len(sims):
        top_idx = np.argsort(-sims)
    else:
        part = np.argpartition(-sims, K)[:K]
        top_idx = part[np.argsort(-sims[part])]

    return [(ids_all[j], float(sims[j])) for j in top_idx[:K]]


def insert_related(cur, tr_id: int, pairs):
    clean = []
    for rid, score in pairs:
        if rid == tr_id:
            continue
        s = float(score)
        if s <= 0:
            continue
        clean.append((tr_id, rid, s))

    if not clean:
        return

    psycopg2.extras.execute_values(
        cur,
        """
        INSERT INTO related_noticias (traduccion_id, related_traduccion_id, score)
        VALUES %s
        ON CONFLICT (traduccion_id, related_traduccion_id)
        DO UPDATE SET score = EXCLUDED.score;
        """,
        clean,
    )


def build_for_ids(conn, target_ids: List[int]) -> int:
    with conn.cursor() as cur:
        ids_all, mat = fetch_all_embeddings(cur)

    if not ids_all or mat is None:
        return 0

    pos = {tid: i for i, tid in enumerate(ids_all)}
    processed = 0

    with conn.cursor() as cur:
        for tr_id in target_ids:
            if tr_id not in pos:
                continue
            idx = pos[tr_id]
            pairs = topk(idx, ids_all, mat, TOPK)
            insert_related(cur, tr_id, pairs)
            processed += 1

        conn.commit()

    return processed


def main():
    logging.info(
        "Iniciando related_worker (EMB=%s TOPK=%s BATCH=%s MIN=%.3f WINDOW_H=%s)",
        EMB_MODEL,
        TOPK,
        BATCH_IDS,
        MIN_SCORE,
        WINDOW_HOURS,
    )

    while True:
        try:
            with get_conn() as conn, conn.cursor() as cur:
                todo = fetch_pending_ids(cur, BATCH_IDS)

            if not todo:
                time.sleep(SLEEP_IDLE)
                continue

            with get_conn() as conn:
                done = build_for_ids(conn, todo)
                logging.info("Relacionadas generadas/actualizadas para %d traducciones.", done)

        except Exception:
            logging.exception("Error en related_worker")
            time.sleep(SLEEP_IDLE)


if __name__ == "__main__":
    main()

