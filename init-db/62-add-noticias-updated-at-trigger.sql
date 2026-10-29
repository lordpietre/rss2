-- Trigger de updated_at en 'noticias'.
--
-- La funcion update_updated_at_column() ya se creaba en la migracion 53
-- (para event_clusters), pero NINGUN script de init-db creaba el trigger
-- sobre la tabla noticias. Sin el, noticias.updated_at se quedaba congelado
-- en el valor de la insercion.
--
-- Consecuencia: la ruta de catch-up del translation_scheduler, que busca las
-- noticias actualizadas en las ultimas horas (para pillar las que acaba de
-- etiquetar langdetect), no encontraba nada y esas noticias no se
-- translateban nunca, sin ningun error visible.
--
-- La funcion se crea aqui tambien (IF NOT EXISTS no existe para funciones,
-- asi que se usa DO $$) para que la migracion sea autonoma.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
        WHERE n.nspname = 'public' AND p.proname = 'update_updated_at_column'
    ) THEN
        EXECUTE $fn$
            CREATE OR REPLACE FUNCTION public.update_updated_at_column()
            RETURNS trigger
            LANGUAGE plpgsql
            AS $body$
            BEGIN
                NEW.updated_at = NOW();
                RETURN NEW;
            END;
            $body$;
        $fn$;
    END IF;
END $$;

DROP TRIGGER IF EXISTS update_noticias_updated_at ON noticias;
CREATE TRIGGER update_noticias_updated_at
    BEFORE UPDATE ON noticias
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
