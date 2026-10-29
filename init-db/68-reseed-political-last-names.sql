-- Re-siembra de los apellidos de las personaspoliticas.
--
-- La migracion 44 inserta estos apellidos con tipo='persona'. Solo se
-- aplicaba en la PRIMERA inicializacion de la base: al vaciar el corpus
-- (TRUNCATE de tags) esos apellidos se perdieron y el worker de NER, que
-- inserta `INSERT INTO tags (valor, tipo) ON CONFLICT DO NOTHING`, volvio a
-- crear las etiquetas sin el campo 'apellido'.
--
-- Consecuencia: `tags.apellido` quedaba NULL en las 208.000 etiquetas, el
-- endpoint /api/last-names devolvia una lista vacia y el filtro por apellido
-- de /api/news no encontraba nada.
--
-- Es idempotente: se puede repetir sin efectos.

INSERT INTO tags (valor, tipo, apellido) VALUES ('Trump', 'persona', 'Trump') ON CONFLICT (valor, tipo) DO UPDATE SET apellido = EXCLUDED.apellido;
INSERT INTO tags (valor, tipo, apellido) VALUES ('Meloni', 'persona', 'Meloni') ON CONFLICT (valor, tipo) DO UPDATE SET apellido = EXCLUDED.apellido;
INSERT INTO tags (valor, tipo, apellido) VALUES ('Biden', 'persona', 'Biden') ON CONFLICT (valor, tipo) DO UPDATE SET apellido = EXCLUDED.apellido;
INSERT INTO tags (valor, tipo, apellido) VALUES ('Macron', 'persona', 'Macron') ON CONFLICT (valor, tipo) DO UPDATE SET apellido = EXCLUDED.apellido;
INSERT INTO tags (valor, tipo, apellido) VALUES ('Scholz', 'persona', 'Scholz') ON CONFLICT (valor, tipo) DO UPDATE SET apellido = EXCLUDED.apellido;
INSERT INTO tags (valor, tipo, apellido) VALUES ('Musk', 'persona', 'Musk') ON CONFLICT (valor, tipo) DO UPDATE SET apellido = EXCLUDED.apellido;
INSERT INTO tags (valor, tipo, apellido) VALUES ('Putin', 'persona', 'Putin') ON CONFLICT (valor, tipo) DO UPDATE SET apellido = EXCLUDED.apellido;
INSERT INTO tags (valor, tipo, apellido) VALUES ('Zelensky', 'persona', 'Zelensky') ON CONFLICT (valor, tipo) DO UPDATE SET apellido = EXCLUDED.apellido;
INSERT INTO tags (valor, tipo, apellido) VALUES ('Netanyahu', 'persona', 'Netanyahu') ON CONFLICT (valor, tipo) DO UPDATE SET apellido = EXCLUDED.apellido;
INSERT INTO tags (valor, tipo, apellido) VALUES ('Erdoğan', 'persona', 'Erdoğan') ON CONFLICT (valor, tipo) DO UPDATE SET apellido = EXCLUDED.apellido;
INSERT INTO tags (valor, tipo, apellido) VALUES ('Erdogan', 'persona', 'Erdogan') ON CONFLICT (valor, tipo) DO UPDATE SET apellido = EXCLUDED.apellido;
INSERT INTO tags (valor, tipo, apellido) VALUES ('Johnson', 'persona', 'Johnson') ON CONFLICT (valor, tipo) DO UPDATE SET apellido = EXCLUDED.apellido;
INSERT INTO tags (valor, tipo, apellido) VALUES ('Trudeau', 'persona', 'Trudeau') ON CONFLICT (valor, tipo) DO UPDATE SET apellido = EXCLUDED.apellido;
INSERT INTO tags (valor, tipo, apellido) VALUES ('Moreno', 'persona', 'Moreno') ON CONFLICT (valor, tipo) DO UPDATE SET apellido = EXCLUDED.apellido;
INSERT INTO tags (valor, tipo, apellido) VALUES ('Lula', 'persona', 'Lula') ON CONFLICT (valor, tipo) DO UPDATE SET apellido = EXCLUDED.apellido;
INSERT INTO tags (valor, tipo, apellido) VALUES ('Milei', 'persona', 'Milei') ON CONFLICT (valor, tipo) DO UPDATE SET apellido = EXCLUDED.apellido;
INSERT INTO tags (valor, tipo, apellido) VALUES ('Albright', 'persona', 'Albright') ON CONFLICT (valor, tipo) DO UPDATE SET apellido = EXCLUDED.apellido;

CREATE INDEX IF NOT EXISTS idx_tags_valor_tipo_apellido ON tags(valor, tipo, apellido);
