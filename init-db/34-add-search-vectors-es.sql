-- Add search_vector_es columns for full-text search in Spanish
-- This migration adds missing columns referenced in search.py

-- Add search_vector_es to noticias table
ALTER TABLE noticias ADD COLUMN IF NOT EXISTS search_vector_es tsvector;

-- Add search_vector_es to traducciones table  
ALTER TABLE traducciones ADD COLUMN IF NOT EXISTS search_vector_es tsvector;

-- Create function to update noticias search_vector_es
CREATE OR REPLACE FUNCTION noticias_search_vector_es_trigger() RETURNS trigger AS $$
BEGIN
  new.search_vector_es := setweight(to_tsvector('spanish', coalesce(new.titulo,'')), 'A') ||
                         setweight(to_tsvector('spanish', coalesce(new.resumen,'')), 'B');
  return new;
END
$$ LANGUAGE plpgsql;

-- Create trigger for noticias
DROP TRIGGER IF EXISTS search_vector_es_update_noticias ON noticias;
CREATE TRIGGER search_vector_es_update_noticias
BEFORE INSERT OR UPDATE ON noticias
FOR EACH ROW EXECUTE PROCEDURE noticias_search_vector_es_trigger();

-- Create function to update traducciones search_vector_es
CREATE OR REPLACE FUNCTION traducciones_search_vector_es_trigger() RETURNS trigger AS $$
BEGIN
  new.search_vector_es := setweight(to_tsvector('spanish', coalesce(new.titulo_trad,'')), 'A') ||
                         setweight(to_tsvector('spanish', coalesce(new.resumen_trad,'')), 'B');
  return new;
END
$$ LANGUAGE plpgsql;

-- Create trigger for traducciones
DROP TRIGGER IF EXISTS search_vector_es_update_traducciones ON traducciones;
CREATE TRIGGER search_vector_es_update_traducciones
BEFORE INSERT OR UPDATE ON traducciones
FOR EACH ROW EXECUTE PROCEDURE traducciones_search_vector_es_trigger();

-- Create GIN indexes for fast full-text search
CREATE INDEX IF NOT EXISTS noticias_search_vector_es_idx ON noticias USING gin(search_vector_es);
CREATE INDEX IF NOT EXISTS traducciones_search_vector_es_idx ON traducciones USING gin(search_vector_es);

-- Update existing data
UPDATE noticias SET search_vector_es = to_tsvector('spanish', coalesce(titulo,'') || ' ' || coalesce(resumen,'')) WHERE search_vector_es IS NULL;
UPDATE traducciones SET search_vector_es = to_tsvector('spanish', coalesce(titulo_trad,'') || ' ' || coalesce(resumen_trad,'')) WHERE search_vector_es IS NULL;

-- Add composite index for traducciones search optimization
CREATE INDEX IF NOT EXISTS traducciones_search_composite_idx ON traducciones(lang_to, status) WHERE search_vector_es IS NOT NULL;