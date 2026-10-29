-- Correccion del desfase de zona horaria en las fechas de las noticias.
--
-- CONTEXTO
-- El parser RSS de gofeed normaliza a UTC el campo 'pubDate' (rss/parser.go
-- hace date.UTC()), pero NO el campo 'dc:date' del Dublin Core
-- (translator.go se queda con la zona original). La columna 'fecha' es
-- 'timestamp without time zone' y lib/pq manda el offset, que Postgres
-- ignora para ese tipo: se guardaba la HORA DE PARED de la zona del feed.
--
-- Una noticia de un feed japones (dc:date con +09:00) se guardaba 9 horas
-- por delante y salia por delante de todo en la portada (/api/news ordena
-- por fecha DESC). El ingestor ya normaliza a UTC antes de insertar; esta
-- migracion arregla las que ya estaban en la base.
--
-- Tambien hay feeds con el reloj desfasado: publican la hora local
-- etiquetada como UTC ('<pubDate>Tue, 27 Oct 2026 10:45:00 +0000</pubDate>'
-- en un feed japones). A esos feeds se les corrige el desfase deduciendolo
-- de sus propias noticias, que es la misma cuenta.
--
-- COMO SE DETECTA EL DESFASE
-- Una noticia no puede estar fechada mas adelante del momento en que se
-- ingirio: max(fecha - created_at) es, por tanto, 0 o negativo para un feed
-- bien guardado. Si sale positivo, el feed (o nuestra forma de guardar la
-- fecha) va desfasado. Se usa el percentil 75 en vez del maximo para que
-- un item basura con fecha de 3000 no arrastre la correccion, y se limita a
-- 15 horas por si acaso.
--
-- El desfase es constante por feed (es el offset que declara el feed), asi
-- que la correccion se aplica a TODAS sus noticias, no solo a las recientes.

ALTER TABLE noticias DISABLE TRIGGER update_noticias_updated_at;

-- 1) Feeds con desfase positivo detectable.
WITH recent AS (
    SELECT feed_id,
           EXTRACT(EPOCH FROM (fecha - created_at)) / 3600.0 AS h
      FROM noticias
     WHERE created_at > NOW() - INTERVAL '12 hours'
       AND fecha > created_at - INTERVAL '24 hours'
),
per_feed AS (
    SELECT feed_id,
           COUNT(*) AS n,
           percentile_cont(0.75) WITHIN GROUP (ORDER BY h) AS skew
      FROM recent
     GROUP BY feed_id
    HAVING COUNT(*) >= 5
),
corrections AS (
    SELECT feed_id, ROUND(skew * 2) / 2 AS offset_h
      FROM per_feed
     WHERE skew >= 0.9
       AND skew <= 15
)
UPDATE noticias n
   SET fecha = n.fecha - (c.offset_h * INTERVAL '1 hour')
  FROM corrections c
 WHERE c.feed_id = n.feed_id;

-- 2) Feeds con dc:date y offset NEGATIVO (occidental).
--
-- Aqui el desfase es negativo y se confunde con el retardo normal entre la
-- publicacion y la ingesta, asi que no se puede deducir de los datos: se
-- listan los confirmados leyendo el XML del feed.
--
--   8097  tmz.com  <dc:date>...-07:00</dc:date>
--
-- El filtro (el desfase sigue siendo ~-7h) hace que la migracion sea
-- idempotente: al repetirla no corrige dos veces la misma noticia.
UPDATE noticias n
   SET fecha = n.fecha + INTERVAL '7 hours'
 WHERE n.feed_id = 8097
   AND EXISTS (
       SELECT 1
         FROM noticias m
        WHERE m.feed_id = 8097
          AND m.created_at > NOW() - INTERVAL '12 hours'
          AND m.fecha < m.created_at - INTERVAL '6 hours'
   );

-- 3) Red de seguridad: ninguna noticia puede estar fechada en el futuro.
--
-- Tras las correcciones anteriores solo queda el caso de los feeds con el
-- reloj desfasado y de los items sueltos con fecha inventada. Se limita a
-- las noticias ingeridas hace poco: en las anciennes, created_at es la
-- fecha de ingesta (hoy) y no la de publicacion, asi que no serve de tope.
UPDATE noticias
   SET fecha = created_at
 WHERE fecha > created_at + INTERVAL '1 minute'
   AND created_at > NOW() - INTERVAL '30 days';

ALTER TABLE noticias ENABLE TRIGGER update_noticias_updated_at;
