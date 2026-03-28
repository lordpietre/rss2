-- init-db/07-tags-views.sql
-- Vista de Top tags (24h) para el esquema existente:
--   tags(id, nombre)
--   tags_noticia(id, traduccion_id, tag_id)
--   traducciones(id, noticia_id, lang_to, status, ...)
--   noticias(id, fecha, ...)

CREATE OR REPLACE VIEW public.v_tag_counts_24h AS
SELECT
  tg.id,
  tg.valor AS tag_name,
  COUNT(*) AS apariciones
FROM public.tags tg
JOIN public.tags_noticia tn ON tn.tag_id = tg.id
JOIN public.traducciones t   ON t.id = tn.traduccion_id
JOIN public.noticias n       ON n.id = t.noticia_id
WHERE t.status = 'done'
  AND t.lang_to = 'es'
  AND n.fecha >= now() - INTERVAL '24 hours'
GROUP BY tg.id, tg.valor
ORDER BY apariciones DESC, tg.valor;

-- Índices recomendados para acelerar la vista (idempotentes)
CREATE INDEX IF NOT EXISTS idx_noticias_fecha
  ON public.noticias (fecha);

CREATE INDEX IF NOT EXISTS idx_traducciones_noticia_lang_status
  ON public.traducciones (noticia_id, lang_to, status);

CREATE INDEX IF NOT EXISTS idx_tags_noticia_traduccion
  ON public.tags_noticia (traduccion_id);

CREATE INDEX IF NOT EXISTS idx_tags_noticia_tag
  ON public.tags_noticia (tag_id);

