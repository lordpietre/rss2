# Spec 03 — Detección de idioma + traducción (noticias → español)

## Alcance

1. `langdetect` (`workers/langdetect_worker.py`, imagen `rss2-pyworkers`):
   `noticias.lang NULL → detect()` (solo `psycopg2+langdetect`).
2. `translation-scheduler` (`workers/translation_scheduler.py`, misma imagen):
   cada 30 s crea `traducciones(..., lang_to='es', status='pending')` para
   noticias con `lang` conocido y distinto de `es` (`TARGET_LANGS`,
   batch 2000). Precondición documentada: sin `lang` no hay jobs.
3. `translator` (`translator/Dockerfile.cpu`, CTranslate2 NLLB-200 int8 CPU):
   `ctranslator_worker.py` reclama por polling SQL (`SKIP LOCKED`,
   `locked_at` >10 min reintenta), traduce, `status='done'`, `locked_at=NULL`,
   inserta en `translation_stats`, caché Redis best-effort (`tr:{de}:{a}:{md5}`,
   TTL 30 d). Escalado 2026-09-15: `translator` + `translator-2`
   (misma imagen, `CT2_INTRA_THREADS=2`, reparto por `SKIP LOCKED`).
   OJO: 3 réplicas saturan los 22G y entran en swap (5.7G) dejando los
   workers colgados (CPU ~2%, MEM al límite); 2 es el techo de este host.

## Env (compose, obligatorias)

DB_* reales en los tres servicios; `TARGET_LANGS=es`, `SCHEDULER_BATCH/SLEEP`,
`REDIS_URL` con password en translator; `CT2_MODEL_PATH=/models/nllb-ct2:ro`.

## Criterios de aceptación

- [x] `noticias.lang` se rellena (27k+).
- [x] `traducciones pending` se crean en lotes y `done` crece.
- [x] `titulo_trad/resumen_trad` con español real (verificado ja/tr→es).
- [x] `/api/news?translated_only=true` devuelve traducidas (tras fix
      `n.contenido`/args del spec 05).
- [ ] Drenar backlog (~27k) — tarda horas en CPU; alternativa GPU documentada.

## Plan

1. Servicios `langdetect`, `translation-scheduler` (imagen compartida
   `rss2-pyworkers` construida una vez) y `translator` (hecho).
2. No construir dos servicios a la misma imagen en un solo comando (compose
   falla con `AlreadyExists`); construir `translation-scheduler` y arrancar
   el resto con `--no-build`.
3. `Dockerfile.scheduler` copia `workers/` completo (no un solo script).

## Tasks

- [x] Env DB/Redis en translator (+gpu), scheduler, langdetect.
- [x] Servicio scheduler + langdetect + imagen compartida.
- [x] `Dockerfile.scheduler` con `COPY workers/`.
- [x] Verificar ciclo lang → pending → done con contenido real.
- [x] **Orden de bloqueo** (2026-09-25): `process_batch` hacía los
      `UPDATE noticias SET lang=...` en orden `fecha DESC`, solapándose
      con el UPDATE masivo de `topics` → Postgres abortaba uno con
      `deadlock detected (SQLSTATE 40P01)`. Ahora `rows.sort(key=id)`
      antes del bucle (id ascendente, igual que topics).
- [x] **OOM de los traductores** (2026-09-26): 41 `Memory cgroup out of
      memory` en el kernel, solo en `translator`/`translator_2` (límite
      4G), acelerando de 5/h a 14/h. Causa medida con test empírico, no
      supuesta: el modelo NLLB son 916 MB fijos y `translate_batch` añade
      ~28 MB por secuencia, así que `MAX_SEQ_PER_CALL=128` picaba en
      **4383 MB** y volaba el límite; con **32** el pico baja a
      **1940 MB**. Fijado en `docker-compose.yml` (los tres bloques
      `translator*`), `.env` y `.env.example`; límites de memoria
      ajustados a lo medido (`translator*` 4G→3G, `embeddings` 4G→2G,
      `backend` 4G→512M). Verificado: recreados 01:06 UTC,
      `restarts=0` y ningún OOM posterior. Ojo: `docker inspect
      .State.OOMKilled` se resetea a `false` al reiniciar, la evidencia
      autoritativa es `dmesg -T | grep "Memory cgroup out of memory"`.
- [ ] Decidir GPU (`--profile gpu` en máquina NVIDIA) o más réplicas CPU si
      el backlog no drena.
- [ ] Añadir `TRANSLATOR_*`/`SCHEDULER_*` a `.env.example` si se estabilizan.
