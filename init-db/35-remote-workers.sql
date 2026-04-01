-- Tabla de workers remotos
CREATE TABLE IF NOT EXISTS remote_workers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    api_key VARCHAR(64) UNIQUE NOT NULL,
    capabilities VARCHAR(50) DEFAULT 'cpu',
    status VARCHAR(20) DEFAULT 'offline',
    last_seen TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Añadir columnas a traducciones para workers remotos
ALTER TABLE traducciones ADD COLUMN IF NOT EXISTS worker_id INTEGER REFERENCES remote_workers(id);
ALTER TABLE traducciones ADD COLUMN IF NOT EXISTS assigned_at TIMESTAMP;

-- Índices para workers remotos
CREATE INDEX IF NOT EXISTS idx_remote_workers_status ON remote_workers(status);
CREATE INDEX IF NOT EXISTS idx_remote_workers_api_key ON remote_workers(api_key);

-- Índice para buscar jobs disponibles por capabilities
CREATE INDEX IF NOT EXISTS idx_traducciones_pending_worker 
    ON traducciones(lang_to, status, worker_id) 
    WHERE status = 'pending' AND worker_id IS NULL;

-- Índice para jobs asignados (para cleanup de timeout)
CREATE INDEX IF NOT EXISTS idx_traducciones_assigned_timeout 
    ON traducciones(assigned_at) 
    WHERE status = 'assigned' AND assigned_at IS NOT NULL;

-- Tabla de workers remotos conectados (en memoria = no persistir, pero referenciar)
-- El status se actualiza desde el WebSocket handler