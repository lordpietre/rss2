-- Limpieza de noticias duplicadas.
--
-- El ingestor deriva el id de la URL normalizada (sin esquema, sin www, sin
-- barra final y sin parametros de tracking), asi que un mismo articulo
-- publicado con variantes de URL genera filas distintas. Esta operacion
-- deja UNA fila por articulo.
--
-- Se agrupa por (feed, URL normalizada) y NO por titulo: dos articulos
-- distintos con el mismo titular NO son duplicados (agrupar por titulo
-- contaba 24.275 filas de las cuales solo 1.098 eran duplicados reales).
--
-- De cada grupo se conserva la fila MAS NUEVA (created_at): es la que genera
-- el ingestor actual con su id normalizado, de modo que al volver a
-- publicarse el articulo el INSERT con ON CONFLICT no crea otra copia.
--
-- Es idempotente: se puede repetir sin efectos.

-- 1) Duplicados por URL normalizada (esquema/www/barra/tracking).
WITH norm AS (
    SELECT n.id,
           n.feed_id,
           n.created_at,
           lower(regexp_replace(
               regexp_replace(
                   regexp_replace(url, '^https?://(www\.)?', '', 'i'),
                   '[?&](utm_[^&]*|fbclid|gclid|dclid|msclkid|yclid|mc_cid|mc_eid|ref|ref_src|spm)=[^&]*',
                   '', 'g'),
               '/$', '')) AS u
    FROM noticias n
),
dup AS (
    SELECT id, feed_id, created_at,
           row_number() OVER (PARTITION BY feed_id, u ORDER BY created_at DESC, id) AS rn
    FROM norm
)
DELETE FROM noticias n
USING dup d
WHERE n.id = d.id AND d.rn > 1;

-- 2) Duplicados exactos: misma URL (bit a bit) en el mismo feed.
WITH dup AS (
    SELECT id,
           row_number() OVER (PARTITION BY feed_id, url ORDER BY created_at DESC, id) AS rn
    FROM noticias
)
DELETE FROM noticias n
USING dup d
WHERE n.id = d.id AND d.rn > 1;

-- 3) Contadores de noticias por fuente (el trigger no se dispara con DELETE
--    en bloque de forma fiable y el TRUNCATE del reset los dejo obsoletos).
--    ON CONFLICT es necesario: el ingestor esta vivo e inserta en feed_stats
--    a la vez (su trigger), y sin el la carga puede petar con un
--    "duplicate key" en plena carrera.
TRUNCATE feed_stats;
INSERT INTO feed_stats (fuente_nombre, noticias_count)
SELECT fuente_nombre, count(*)
FROM noticias
WHERE fuente_nombre IS NOT NULL AND fuente_nombre <> ''
GROUP BY fuente_nombre
ON CONFLICT (fuente_nombre) DO UPDATE
SET noticias_count = EXCLUDED.noticias_count;
