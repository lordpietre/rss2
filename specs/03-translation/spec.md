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
- [ ] Decidir GPU (`--profile gpu` en máquina NVIDIA) o más réplicas CPU si
      el backlog no drena.
- [ ] Añadir `TRANSLATOR_*`/`SCHEDULER_*` a `.env.example` si se estabilizan.
