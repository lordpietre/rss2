-- Script SQL para crear tablas de parrillas de noticias para videos

-- Tabla principal de parrillas/programaciones
CREATE TABLE IF NOT EXISTS video_parrillas (
    id SERIAL PRIMARY KEY,
    nombre VARCHAR(255) NOT NULL UNIQUE,
    descripcion TEXT,
    tipo_filtro VARCHAR(50) NOT NULL, -- 'pais', 'categoria', 'entidad', 'continente', 'custom'
    
    -- Filtros
    pais_id INTEGER REFERENCES paises(id),
    categoria_id INTEGER REFERENCES categorias(id),
    continente_id INTEGER REFERENCES continentes(id),
    entidad_nombre VARCHAR(255), -- Para filtrar por persona/organización específica
    entidad_tipo VARCHAR(50), -- 'persona', 'organizacion'
    
    -- Configuración de generación
    max_noticias INTEGER DEFAULT 5, -- Número máximo de noticias por video
    duracion_maxima INTEGER DEFAULT 180, -- Duración máxima en segundos
    idioma_voz VARCHAR(10) DEFAULT 'es', -- Idioma del TTS
    voz_modelo VARCHAR(100), -- Modelo de voz específico a usar
    
    -- Configuración de diseño
    template VARCHAR(50) DEFAULT 'standard', -- 'standard', 'modern', 'minimal'
    include_images BOOLEAN DEFAULT true,
    include_subtitles BOOLEAN DEFAULT true,
    
    -- Programación
    frecuencia VARCHAR(20), -- 'daily', 'weekly', 'manual'
    ultima_generacion TIMESTAMP,
    proxima_generacion TIMESTAMP,
    
    -- Estado
    activo BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Tabla de videos generados
CREATE TABLE IF NOT EXISTS video_generados (
    id SERIAL PRIMARY KEY,
    parrilla_id INTEGER REFERENCES video_parrillas(id) ON DELETE CASCADE,
    titulo VARCHAR(500) NOT NULL,
    descripcion TEXT,
    fecha_generacion TIMESTAMP DEFAULT NOW(),
    
    -- Archivos
    video_path VARCHAR(500),
    audio_path VARCHAR(500),
    subtitles_path VARCHAR(500),
    thumbnail_path VARCHAR(500),
    
    -- Metadata
    duracion INTEGER, -- en segundos
    num_noticias INTEGER,
    noticias_ids TEXT[], -- Array de IDs de noticias incluidas
    
    -- Estado de procesamiento
    status VARCHAR(20) DEFAULT 'pending', -- 'pending', 'processing', 'completed', 'error'
    error_message TEXT,
    
    -- Estadísticas
    views INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Tabla de noticias en videos (relación muchos a muchos)
CREATE TABLE IF NOT EXISTS video_noticias (
    id SERIAL PRIMARY KEY,
    video_id INTEGER REFERENCES video_generados(id) ON DELETE CASCADE,
    noticia_id VARCHAR(100) NOT NULL,
    traduccion_id INTEGER REFERENCES traducciones(id),
    orden INTEGER NOT NULL, -- Orden de aparición en el video
    timestamp_inicio FLOAT, -- Segundo donde comienza esta noticia
    timestamp_fin FLOAT, -- Segundo donde termina esta noticia
    created_at TIMESTAMP DEFAULT NOW()
);

-- Índices para mejorar performance
CREATE INDEX IF NOT EXISTS idx_parrillas_tipo ON video_parrillas(tipo_filtro);
CREATE INDEX IF NOT EXISTS idx_parrillas_activo ON video_parrillas(activo);
CREATE INDEX IF NOT EXISTS idx_parrillas_proxima ON video_parrillas(proxima_generacion);
CREATE INDEX IF NOT EXISTS idx_videos_parrilla ON video_generados(parrilla_id);
CREATE INDEX IF NOT EXISTS idx_videos_status ON video_generados(status);
CREATE INDEX IF NOT EXISTS idx_videos_fecha ON video_generados(fecha_generacion DESC);

-- Comentarios para documentación
COMMENT ON TABLE video_parrillas IS 'Configuraciones de parrillas de noticias para generar videos automáticos';
COMMENT ON TABLE video_generados IS 'Videos generados a partir de parrillas de noticias';
COMMENT ON TABLE video_noticias IS 'Relación entre videos y las noticias que contienen';
