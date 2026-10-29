-- Índices para el pipeline de traducción (scheduler -> translator)
--
-- El scheduler busca "noticias nuevas que aún no tienen job de traducción".
-- Antes lo hacía con un anti-join NOT EXISTS sobre TODO noticias (2,69M filas):
-- como las noticias ya elegibles tienen fila en traducciones, el planner tenía
-- que recorrer la tabla entera para PROBAR que no quedaba ninguna, y el ciclo
-- tardaba ~12 min.
--
-- La ventana de "nuevas" se calcula sobre created_at (fecha de inserción) y NO
-- sobre fecha (fecha de publicación): hay noticias con fecha de1593 y de 2033,
-- así que fecha no sirve como criterio de "lo nuevo".
--
-- created_at se anade en la migracion 99, que se ejecuta DESPUES (orden
-- alfabetico). Se crea aqui tambien (IF NOT EXISTS) para que este indice no
-- dependa del orden: si no, el init aborta con
-- "column created_at does not exist".
ALTER TABLE noticias
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMP DEFAULT NOW();

CREATE INDEX IF NOT EXISTS idx_noticias_created_at
    ON noticias (created_at DESC);

-- Índice parcial: solo las noticias que el scheduler puede encolar
-- (idioma conocido y distinto del destino). Evita visitar en el heap las
-- noticias sin lang (las detecta langdetect) y las que ya están en español.
CREATE INDEX IF NOT EXISTS idx_noticias_trad_elegibles
    ON noticias (created_at DESC)
    WHERE lang IS NOT NULL AND lang <> 'es';

-- Requeue: el translator marca 'done' solo si tiene los tres campos.
-- Las filas 'done' con campos incompletos se quedan atascadas (el translator
-- solo coge status='pending' y el scheduler no reencola porque la fila ya
-- existe). Este índice parcial las encuentra rápido.
CREATE INDEX IF NOT EXISTS idx_traducciones_done_incompletas
    ON traducciones (lang_to)
    WHERE status = 'done'
      AND (titulo_trad IS NULL OR resumen_trad IS NULL OR contenido_trad IS NULL);
