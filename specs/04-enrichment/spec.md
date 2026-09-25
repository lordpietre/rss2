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
    `ALERTS_*` env): picos (`hits≥5`, `baseline≥2`, `ratio≥5`, lookback 8 d)
    → `alertas(status='nueva')`. Referencia = último día COMPLETO
    (`n.fecha < CURRENT_DATE`): comparar contra el día en curso nunca
    dispara (día parcial). Verificado 2026-09-14: 1044 alertas del
    periodo 2026-09-13.

## Env (compose)

`ner`: DB_* reales, `NER_LANG=es`, `NER_BATCH=64`.

## Criterios de aceptación

- [x] `tags`/`tags_noticia` crecen (31k/55k).
- [x] Populares (`/api/entities`) con counts; noticias por entidad
      (`valor`+`tipo`); Evolución (`/api/entities/mentions`) con series.
- [x] Scanner corre y genera alertas (fix día completo, 1044 el
      2026-09-14). Umbrales vigentes sin cambios.
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
