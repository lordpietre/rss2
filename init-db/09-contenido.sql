-- =============================================================================
-- RSS2 - Migración: Añadir columna contenido para artículos completos
-- =============================================================================
-- Esta migración añade columnas para almacenar y traducir el contenido completo
-- de los artículos, separándolo del resumen.

-- Añadir columna contenido a noticias (contenido completo del artículo extraído por scraper)
ALTER TABLE noticias ADD COLUMN IF NOT EXISTS contenido TEXT;

-- Añadir columna contenido_trad a traducciones (traducción del contenido completo)
ALTER TABLE traducciones ADD COLUMN IF NOT EXISTS contenido_trad TEXT;

-- Actualizar el trigger de tsvector para incluir contenido
CREATE OR REPLACE FUNCTION noticias_tsv_trigger() RETURNS trigger AS $$
BEGIN
  new.tsv := setweight(to_tsvector('spanish', coalesce(new.titulo,'')), 'A') ||
             setweight(to_tsvector('spanish', coalesce(new.resumen,'')), 'B') ||
             setweight(to_tsvector('spanish', coalesce(new.contenido,'')), 'C');
  return new;
END
$$ LANGUAGE plpgsql;

-- Recrear el trigger si existe
DROP TRIGGER IF EXISTS tsvectorupdate ON noticias;
CREATE TRIGGER tsvectorupdate
BEFORE INSERT OR UPDATE ON noticias
FOR EACH ROW EXECUTE PROCEDURE noticias_tsv_trigger();

-- Índices para las nuevas columnas
CREATE INDEX IF NOT EXISTS idx_noticias_contenido ON noticias USING gin(to_tsvector('spanish', coalesce(contenido,'')));
CREATE INDEX IF NOT EXISTS idx_traducciones_contenido_trad ON traducciones(contenido_trad) WHERE contenido_trad IS NOT NULL;

-- Migración de datos: copiar resumen a contenido donde contenido sea NULL
-- Esto es para noticias existentes que tienen el contenido en resumen
UPDATE noticias SET contenido = resumen WHERE contenido IS NULL AND resumen IS NOT NULL AND resumen != '';

DO $$
BEGIN
    RAISE NOTICE 'Migración 09: Columnas contenido y contenido_trad añadidas correctamente';
END $$;
