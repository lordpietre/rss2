BEGIN;

-- ============================================================================
-- Event Clustering Tables
-- ============================================================================

-- Tabla principal de clusters de eventos
CREATE TABLE IF NOT EXISTS event_clusters (
    id SERIAL PRIMARY KEY,
    cluster_hash VARCHAR(64) NOT NULL,           -- Hash único del cluster para deduplicación
    titulo_template TEXT,                         -- Título genérico del evento (template)
    primer_noticia_id VARCHAR(32),                -- ID de la primera noticia del cluster
    fecha_inicio TIMESTAMP,                       -- Fecha de la primera noticia
    fecha_fin TIMESTAMP,                          -- Fecha de la última noticia
    popularidad INTEGER DEFAULT 0,                -- Número de noticias en el cluster
    importancia VARCHAR(20) DEFAULT 'normal' CHECK (importancia IN ('breaking', 'major', 'normal', 'minor')),
    embedding_vector vector(384),                  -- Centroid del cluster para búsquedas
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Tabla de relación many-to-many entre noticias y clusters
CREATE TABLE IF NOT EXISTS noticia_clusters (
    noticia_id VARCHAR(32) NOT NULL REFERENCES noticias(id) ON DELETE CASCADE,
    cluster_id INTEGER NOT NULL REFERENCES event_clusters(id) ON DELETE CASCADE,
    similitud DOUBLE PRECISION NOT NULL,          -- Similitud coseno con el centroid
    es_principal BOOLEAN DEFAULT FALSE,           -- TRUE si es la noticia principal del cluster
    created_at TIMESTAMP DEFAULT NOW(),
    PRIMARY KEY (noticia_id, cluster_id)
);

-- ============================================================================
-- Índices
-- ============================================================================

CREATE INDEX IF NOT EXISTS idx_event_clusters_hash ON event_clusters(cluster_hash);
CREATE INDEX IF NOT EXISTS idx_event_clusters_fecha_inicio ON event_clusters(fecha_inicio DESC);
CREATE INDEX IF NOT EXISTS idx_event_clusters_importancia ON event_clusters(importancia);
CREATE INDEX IF NOT EXISTS idx_event_clusters_popularidad ON event_clusters(popularidad DESC);
CREATE INDEX IF NOT EXISTS idx_event_clusters_vector ON event_clusters USING hnsw (embedding_vector vector_cosine_ops);

CREATE INDEX IF NOT EXISTS idx_noticia_clusters_noticia_id ON noticia_clusters(noticia_id);
CREATE INDEX IF NOT EXISTS idx_noticia_clusters_cluster_id ON noticia_clusters(cluster_id);
CREATE INDEX IF NOT EXISTS idx_noticia_clusters_similitud ON noticia_clusters(similitud DESC);

-- ============================================================================
-- Funciones auxiliares
-- ============================================================================

-- Función para calcular hash de cluster basándose en entidades compartidas
CREATE OR REPLACE FUNCTION calcular_cluster_hash(
    p_entity_values TEXT[]
) RETURNS VARCHAR(64) AS $$
DECLARE
    v_hash VARCHAR(64);
    v_sorted TEXT;
BEGIN
    -- Ordenar entidades para tener hash consistente
    SELECT string_agg(value, ',' ORDER BY value)
    INTO v_sorted
    FROM unnest(p_entity_values) AS value;

    -- Crear hash SHA256
    v_hash := encode(sha256(v_sorted::bytea), 'hex');
    RETURN v_hash;
END;
$$ LANGUAGE plpgsql STABLE;

-- Función para actualizar importancia del cluster según cantidad de noticias
CREATE OR REPLACE FUNCTION actualizar_importancia_cluster(p_cluster_id INTEGER)
RETURNS VOID AS $$
DECLARE
    v_count INTEGER;
    v_importancia VARCHAR(20);
BEGIN
    SELECT COUNT(*) INTO v_count
    FROM noticia_clusters
    WHERE cluster_id = p_cluster_id;

    -- Clasificar por número de noticias en las últimas 24 horas
    SELECT CASE
        WHEN v_count >= 10 THEN 'breaking'
        WHEN v_count >= 5 THEN 'major'
        WHEN v_count >= 2 THEN 'normal'
        ELSE 'minor'
    END INTO v_importancia;

    UPDATE event_clusters
    SET importancia = v_importancia,
        popularidad = v_count,
        updated_at = NOW()
    WHERE id = p_cluster_id;
END;
$$ LANGUAGE plpgsql;

-- Trigger para actualizar updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS update_event_clusters_updated_at ON event_clusters;
CREATE TRIGGER update_event_clusters_updated_at
    BEFORE UPDATE ON event_clusters
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMIT;
