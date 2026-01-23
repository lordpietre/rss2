CREATE TABLE IF NOT EXISTS continentes (id SERIAL PRIMARY KEY, nombre VARCHAR(50) NOT NULL UNIQUE);
CREATE TABLE IF NOT EXISTS categorias (id SERIAL PRIMARY KEY, nombre VARCHAR(100) NOT NULL UNIQUE);
CREATE TABLE IF NOT EXISTS paises (id SERIAL PRIMARY KEY, nombre VARCHAR(100) NOT NULL UNIQUE, continente_id INTEGER REFERENCES continentes(id) ON DELETE SET NULL);
CREATE TABLE IF NOT EXISTS feeds (id SERIAL PRIMARY KEY, nombre VARCHAR(255), descripcion TEXT, url TEXT NOT NULL UNIQUE, categoria_id INTEGER REFERENCES categorias(id) ON DELETE SET NULL, pais_id INTEGER REFERENCES paises(id) ON DELETE SET NULL, idioma CHAR(2), activo BOOLEAN DEFAULT TRUE, fallos INTEGER DEFAULT 0, last_etag TEXT, last_modified TEXT);
CREATE TABLE IF NOT EXISTS fuentes_url (
    id SERIAL PRIMARY KEY, 
    nombre VARCHAR(255) NOT NULL, 
    url TEXT NOT NULL UNIQUE, 
    categoria_id INTEGER REFERENCES categorias(id) ON DELETE SET NULL, 
    pais_id INTEGER REFERENCES paises(id) ON DELETE SET NULL, 
    idioma CHAR(2) DEFAULT 'es',
    last_check TIMESTAMP WITHOUT TIME ZONE,
    last_status VARCHAR(50),
    status_message TEXT,
    last_http_code INTEGER,
    active BOOLEAN DEFAULT TRUE
);
CREATE TABLE IF NOT EXISTS noticias (id VARCHAR(32) PRIMARY KEY, titulo TEXT, resumen TEXT, url TEXT NOT NULL UNIQUE, fecha TIMESTAMP, imagen_url TEXT, fuente_nombre VARCHAR(255), categoria_id INTEGER REFERENCES categorias(id) ON DELETE SET NULL, pais_id INTEGER REFERENCES paises(id) ON DELETE SET NULL, tsv tsvector);


ALTER TABLE noticias ADD COLUMN IF NOT EXISTS tsv tsvector;
ALTER TABLE noticias ADD COLUMN IF NOT EXISTS topics_processed BOOLEAN DEFAULT FALSE;


CREATE OR REPLACE FUNCTION noticias_tsv_trigger() RETURNS trigger AS $$
BEGIN
  new.tsv := setweight(to_tsvector('spanish', coalesce(new.titulo,'')), 'A') ||
             setweight(to_tsvector('spanish', coalesce(new.resumen,'')), 'B');
  return new;
END
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS tsvectorupdate ON noticias;

CREATE TRIGGER tsvectorupdate
BEFORE INSERT OR UPDATE ON noticias
FOR EACH ROW EXECUTE PROCEDURE noticias_tsv_trigger();

CREATE INDEX IF NOT EXISTS noticias_tsv_idx ON noticias USING gin(tsv);
