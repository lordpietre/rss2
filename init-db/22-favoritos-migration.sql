-- Migración: Actualizar tabla favoritos para soportar usuarios autenticados
-- Añade columna user_id manteniendo retrocompatibilidad con session_id

-- Agregar columna user_id si no existe
DO $$ 
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'favoritos' AND column_name = 'user_id'
    ) THEN
        ALTER TABLE favoritos ADD COLUMN user_id INTEGER REFERENCES usuarios(id) ON DELETE CASCADE;
    END IF;
END $$;

-- Modificar constraint UNIQUE para incluir user_id
-- Primero eliminar constraint existente si existe
ALTER TABLE favoritos DROP CONSTRAINT IF EXISTS favoritos_session_id_noticia_id_key;
ALTER TABLE favoritos DROP CONSTRAINT IF EXISTS favoritos_unique_favorite;

-- Crear nuevo constraint que permite favoritos por user_id O session_id
ALTER TABLE favoritos ADD CONSTRAINT favoritos_unique_favorite 
    UNIQUE NULLS NOT DISTINCT (user_id, session_id, noticia_id);

-- Agregar check constraint: debe tener user_id O session_id (no ambos nulos)
ALTER TABLE favoritos DROP CONSTRAINT IF EXISTS favoritos_user_or_session;
ALTER TABLE favoritos ADD CONSTRAINT favoritos_user_or_session 
    CHECK (user_id IS NOT NULL OR session_id IS NOT NULL);

-- Crear índice en user_id para búsquedas rápidas
CREATE INDEX IF NOT EXISTS idx_favoritos_user_id ON favoritos(user_id);

-- Comentarios
COMMENT ON COLUMN favoritos.user_id IS 'Usuario autenticado (NULL si es favorito anónimo)';
COMMENT ON COLUMN favoritos.session_id IS 'ID de sesión anónima (NULL si usuario autenticado)';
