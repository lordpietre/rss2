-- Restricción UNIQUE en paises.nombre
--
-- 00-complete-schema.sql crea la tabla paises SIN la restricción (01.schema.sql
-- la tiene, pero al ejecutarse después la tabla ya existe y su
-- 'CREATE TABLE IF NOT EXISTS' no hace nada). Sin UNIQUE, el seed de
-- 04-paises.sql se duplicaba cada vez que se ejecutaba (la	init duplicaba
-- los scripts al reaplicarlos desde 47-pgvector-init.sh): 390 filas para 195
-- países y la asignación de códigos ISO de la migración 99 reventaba con
-- "duplicate key value violates unique constraint paises_iso2_key".
ALTER TABLE paises
    ADD CONSTRAINT paises_nombre_key UNIQUE (nombre);
