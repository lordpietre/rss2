# Spec 04 — Enriquecimiento: NER, wiki, alertas

## Alcance

1. `ner` (`workers/ner_worker.py`, imagen `Dockerfile.ner`: spacy +
   `es_core_news_lg` ~560 MB + bs4 + psycopg2): lee traducciones `done`
   (`lang_to='es'`) en lotes de 64, escribe `tags` + `tags_noticia`
   (UNIQUE tag/noticia). Sin Redis.
2. `wiki-worker` (binario en imagen backend, **sin servicio**): tags
   persona/organización → `es.wikipedia` summary + thumbnail en
   `/app/data/wiki_images`, marca `wiki_checked`. Servido en
   `/api/wiki-images/*`.
3. `AlertScanner` (goroutine del server, 45 s + cada 120 min,
    `ALERTS_*` env): picos **por hora** (`hits≥3`, `baseline≥0.5`,
    `ratio≥5`, `ALERTS_LOOKBACK_HOURS=24`, `≥2` buckets base)
    → `alertas(status='nueva')`. Referencia = última hora COMPLETA con
    datos (`n.fecha < date_trunc('hour', CURRENT_TIMESTAMP)`): la hora en
    curso está incompleta y comparar contra ella nunca dispara.
    `alertas.periodo` es **TIMESTAMP** (la migración DATE→TIMESTAMP la
    hace el server al arrancar, idempotente, porque `alertas` no vive en
    `init-db/` sino en el DDL de `cmd/server/main.go`).
    Verificado 2026-09-26: 29 alertas de la hora `00:00` (ref, 9 buckets
    activos, 8 de baseline).

## Env (compose)

`ner`: DB_* reales, `NER_LANG=es`, `NER_BATCH=64`.

## Criterios de aceptación

- [x] `tags`/`tags_noticia` crecen (31k/55k).
- [x] Populares (`/api/entities`) con counts; noticias por entidad
      (`valor`+`tipo`); Evolución (`/api/entities/mentions`) con series.
- [x] Scanner corre y genera alertas (fix día completo, 1044 el
      2026-09-14). Umbrales reescalados a hora el 2026-09-26.
- [x] Fix alertas 2026-09-26 — granularidad horaria + baseline activo:
      * **Síntoma**: `GET /api/alerts` → `alertas:[]` y el log decía
        `scan finished, 0 new alerts` aunque el scan corría cada 120 min.
      * **Causa raíz medida** (no deducida): con la serie diaria,
        `dias_con_datos=1` frente a 8 huecos → `AVG(COALESCE(cnt,0))`
        daba `baseline=0` en las 67 430 entidades y `baseline≥2` las
        descartaba todas (`pasan_min_baseline=0`).
      * **Agravante**: `COALESCE(cnt,0)` sobre la serie de calendario
        confunde «ese tramo no había datos etiquetados» con «0
        menciones»; con historia parcial rebañaba el baseline y
        falseaba el ratio hacia arriba. Ahora el baseline solo promedia
        buckets con datos.
      * **Granularidad**: por hora había 9 buckets activos (8 de
        baseline) donde por día solo había 1, así que la detección pasa
        a `date_trunc('hour', n.fecha)` y la ventana a `ALERTS_LOOKBACK_HOURS`.
      * **Ajuste por volumen**: la ingesta es irregular y el NER va por
        detrás, así que las horas tienen distinto nº de noticias. Sin
        ajustar, el ratio máximo medido era **1.8** y ninguna alerta
        alcanzaba umbral; con `baseline = menciones/noticia de la base
        × noticias de la hora ref`, `ratio = hits/baseline` sigue
        coherente con la UI y se degrada a la comparación absoluta de
        siempre cuando las horas igualan volumen.
      * **Umbrales horarios**: `hits≥3`, `baseline≥0.5`, `ratio≥5`,
        `buckets≥2` (los diarios `hits≥5/baseline≥2` exigirían `hits≥10`
        por hora, insalvable con ~200 noticias/hora).
      * **Verificado por ejecución**: `alerts: ref=2026-09-26 00:00:00
        buckets_activos=9 baseline=8 candidatos=29 nuevas=29`;
        `periodo` migrado a `timestamp`; `GET /api/alerts` devuelve
        `periodo:"2026-09-26 00:00"` y el tipo `Alerta` del frontend
        encaja sin cambios.
- [x] `wiki` activo como servicio (imagen `rss2-backend`,
  `entrypoint: /wiki-worker`, `DB_*`, volumen `wiki_images`):
  `tags.wiki_summary/url` + thumbs servidos en `/api/wiki-images/*`.
  Verificado: `Putin` con summary real.
- [x] Fix wiki 2026-09-14: ante error de la API (403/red) el tag NO se
  marcaba `wiki_checked` y se reintentaba cada ciclo eternamente (storm
  sobre tags basura del NER). Ahora se marca revisado también en error.
- [x] Filtro anti-basura NER 2026-09-14 (`clean_tag_text`): rechaza
  `[]{}"<>\/` + backtick, >80 chars, días de semana sueltos y
  `este/el + día` (`GENERIC_BAD_TAGS`). Purgados 1746+46 tags basura
  (respaldo en `/tmp/opencode/tags_basura_backup.csv`; cascada limpia
  `tags_noticia`). Verificado: 0 tags basura tras 2 min procesando.
  Alertas históricas con day-words se conservan (auditoría).
- [x] Filtro NER vía topics 2026-09-14: la basura reapareció (98 tags)
  porque `clean_topic_text` (noun-chunks) no aplicaba la regla — solo
  `clean_tag_text` (entidades). Regla extraída a helper compartido
  `_is_markup_garbage` usado en ambas. Purgados los 98; 0 rebrotes.

## Plan / Tasks

- [x] `Dockerfile.ner` + servicio `ner` con DB real.
- [x] Fix orden de columnas en `GetEntities` (`apellido`→ Count).
- [x] Servicio `wiki` (ver arriba) — tooltips con imagen en Populares.
- [x] `alertas` > 0 verificado (1044 del 2026-09-13 tras el fix).
- [x] Alertas con foto+resumen (2026-09-14): `GET /api/alerts` hace
  `LEFT JOIN tags` (`wiki_summary/url/image_path`, nulls si el tag no
  tiene wiki); `Alertas.tsx` muestra thumbnail + snippet + `WikiTooltip`
  (mismo componente que Populares). Imagen servida 200 en
  `/api/wiki-images/*`.
