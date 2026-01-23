-- Índices de optimización de rendimiento
-- Creados: 2025-12-16
-- Propósito: Acelerar consultas frecuentes de conteo y filtrado

-- Índices parciales para estados de traducciones
CREATE INDEX IF NOT EXISTS idx_traducciones_status_partial_done 
  ON traducciones(status) WHERE status = 'done';

CREATE INDEX IF NOT EXISTS idx_traducciones_status_partial_pending  
  ON traducciones(status) WHERE status = 'pending';

-- Índice compuesto para feeds activos/inactivos con fallos
CREATE INDEX IF NOT EXISTS idx_feeds_activo_fallos 
  ON feeds(activo, fallos);

-- Índice compuesto para páginas de noticias con filtros comunes
CREATE INDEX IF NOT EXISTS idx_noticias_fecha_pais_categoria  
  ON noticias(fecha DESC, pais_id, categoria_id);
