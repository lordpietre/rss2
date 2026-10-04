-- Migration: Add updated_at to noticias
-- Date: 2026-10-04
-- Reason: El modelo Go News y NewsWithTranslations tienen UpdatedAt, pero la columna no existe en BD.

ALTER TABLE noticias ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT NOW();

-- Crear índice para queries que filtren por updated_at
CREATE INDEX IF NOT EXISTS idx_noticias_updated_at ON noticias(updated_at DESC);

-- Nota: El campo no se actualiza automáticamente en updates.
-- Para actualizarlo, se puede usar un trigger o actualizar manualmente.
