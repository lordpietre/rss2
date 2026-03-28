-- Migración favoritos ya existente en 00-complete-schema.sql con columna user_id
-- Verificar que existe el índice

CREATE INDEX IF NOT EXISTS idx_favoritos_user_id ON favoritos(user_id);
