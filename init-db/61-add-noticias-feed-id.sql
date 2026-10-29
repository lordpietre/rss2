-- Enlace real entre noticias y feed.
--
-- Hasta ahora la relacion era por 'noticias.fuente_nombre' (texto), que no es
-- una clave: 26.947 nombres distintos para 33.672 feeds, o sea que muchos
-- feeds comparten nombre. Eso hacia imposible borrar un feed con precision
-- (habia que adivinar por nombre) y dejaba noticias huerfanas.
--
-- Con la FK, DELETE FROM feeds borra exactamente sus noticias y, en cascada,
-- todo el estado derivado (traducciones, tags_noticia, embeddings, clusters).
ALTER TABLE noticias
    ADD COLUMN IF NOT EXISTS feed_id INTEGER REFERENCES feeds(id) ON DELETE CASCADE;

-- Relleno por nombre. Con nombres duplicados la fila se asigna a uno de los
-- feeds que lo comparten; es la mejor aproximacion posible sin haber guardado
-- el id en el momento de la ingesta.
UPDATE noticias n
SET feed_id = f.id
FROM feeds f
WHERE n.feed_id IS NULL AND f.nombre = n.fuente_nombre;

CREATE INDEX IF NOT EXISTS idx_noticias_feed_id ON noticias (feed_id);

-- El ingestor escribe ya el id del feed, de modo que las noticias nuevas
-- quedan enlazadas en el momento de insertarse.
