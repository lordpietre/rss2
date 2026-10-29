-- Correccion de las fechas no creibles y de las URLs rotas de las noticias.
--
-- 1) FECHAS
-- El ingestor copiaba la fecha del feed tal cual. Hay feeds que sirven una
-- fecha absurda en su pagina indice (el 3000-01-01 de un blog japones) y
-- feeds cuyo campo de fecha esta vacio (gofeed lo parsea como epoch o como
-- year 1). Como /api/news ordena por fecha DESC, esas noticias ocupaban la
-- cabecera de la portada: la primera era un indice de blog del ano 3000.
-- Ademas el scraper seleccionaba `contenido IS NULL ORDER BY fecha DESC
-- LIMIT 20` y se quedaba enriching esas mismas 20 noticias para siempre.
--
-- El ingestor ya sanea la fecha al insertar (sanitizeDate). Esta migracion
-- arregla las que ya estaban en la base. Se usa created_at (momento de la
-- ingesta) como sustituto: es lo mas cerca que se puede saber de cuando se
-- publico realmente. Las fechas de archivo (2003, 2010...) NO se tocan: son
-- noticias reales, solo que antiguas.
--
-- 2) URLs
-- Hay feeds que sirven links relativos ("/tour", "//sitio.com/a") o URLs con
-- espacios delante. Guardados tal cual no son descargables ("unsupported
-- protocol scheme") y el frontend los renderiza como enlaces rotos. Se
-- resuelven contra la URL del feed.
--
-- El id NO se toca: cambiarlo borraria en cascada las traducciones, los tags
-- y los embeddings de esas noticias (todas las FKs referencian id con ON
-- DELETE CASCADE). El id sigue siendo coherente porque la deduplicacion
-- usa la columna url, que tiene indice unico.
--
-- El trigger update_noticias_updated_at pone updated_at = NOW() en cada
-- UPDATE. Se desactiva durante la correccion: la ruta de catch-up del
-- translation_scheduler usa ese campo y sin esto se crearian miles de
-- trabajos de traduccion para un lote que no lo necesita.

ALTER TABLE noticias DISABLE TRIGGER update_noticias_updated_at;

-- 1) Fechas en el futuro (hasta el ano 3000).
UPDATE noticias
   SET fecha = created_at
 WHERE fecha > NOW() + INTERVAL '48 hours'
   AND created_at IS NOT NULL;

-- 2) Fechas no creibles: epoch de Unix (1970), year 1 (zero time de Go) y
-- anteriores.
UPDATE noticias
   SET fecha = created_at
 WHERE fecha < '1971-01-01'
   AND created_at IS NOT NULL;

-- 3) URLs rotas: se resuelven contra la URL del feed. El bloque va fila a
-- fila para que un choque con el indice unico de url no aborte la
-- migracion entera (en ese caso la fila duplicada se borra).
DO $$
DECLARE
    r          RECORD;
    nuevo      TEXT;
    corregidas INT := 0;
    descartadas INT := 0;
BEGIN
    FOR r IN
        SELECT n.id, n.url, f.url AS feed_url
          FROM noticias n
          LEFT JOIN feeds f ON f.id = n.feed_id
         WHERE n.url !~ '^https?://'
    LOOP
        nuevo := NULL;

        IF btrim(r.url) ~ '^https?://' THEN
            -- URL absoluta con espacios delante: basta con recortar.
            nuevo := btrim(r.url);
        ELSIF btrim(r.url) LIKE '//%' THEN
            -- Sin esquema (//sitio.com/a): se asume https.
            nuevo := 'https:' || btrim(r.url);
        ELSIF btrim(r.url) LIKE '/%' AND r.feed_url ~ '^https?://' THEN
            -- Ruta absoluta: se resuelve contra el host del feed (la ruta
            -- del feed se descarta, como hace ResolveReference).
            nuevo := regexp_replace(r.feed_url,
                                    '^([a-zA-Z][a-zA-Z0-9+.-]*://[^/]+).*$', '\1')
                     || btrim(r.url);
        END IF;

        IF nuevo IS NULL THEN
            -- Basura (HTML suelto, codigo PHP): no se puede resolver.
            descartadas := descartadas + 1;
            CONTINUE;
        END IF;

        IF nuevo = r.url THEN
            CONTINUE;
        END IF;

        BEGIN
            UPDATE noticias SET url = nuevo WHERE id = r.id;
            corregidas := corregidas + 1;
        EXCEPTION WHEN unique_violation THEN
            -- Ya existe otra noticia con esa URL: es un duplicado real.
            -- Se borra la de URL rota (cascadea sus traducciones).
            DELETE FROM noticias WHERE id = r.id;
            descartadas := descartadas + 1;
        END;
    END LOOP;

    RAISE NOTICE 'Migracion 67: % URLs de noticias corregidas, % descartadas',
        corregidas, descartadas;
END $$;

ALTER TABLE noticias ENABLE TRIGGER update_noticias_updated_at;
