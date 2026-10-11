-- Tabla de idiomas disponibles para traducción
CREATE TABLE IF NOT EXISTS idiomas_disponibles (
    codigo CHAR(5) PRIMARY KEY,
    nombre VARCHAR(50) NOT NULL,
    nombre_en VARCHAR(50) NOT NULL,
    flag_emoji VARCHAR(10),
    activo BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Seed inicial con idiomas principales
INSERT INTO idiomas_disponibles (codigo, nombre, nombre_en, flag_emoji, activo) VALUES
    ('es', 'Español', 'Spanish', '🇪🇸', TRUE),
    ('en', 'English', 'English', '🇬🇧', TRUE),
    ('fr', 'Français', 'French', '🇫🇷', TRUE),
    ('de', 'Deutsch', 'German', '🇩🇪', TRUE),
    ('pt', 'Português', 'Portuguese', '🇵🇹', TRUE),
    ('it', 'Italiano', 'Italian', '🇮🇹', TRUE),
    ('ru', 'Русский', 'Russian', '🇷🇺', TRUE),
    ('zh', '中文', 'Chinese', '🇨🇳', TRUE),
    ('ja', '日本語', 'Japanese', '🇯🇵', TRUE),
    ('ko', '한국어', 'Korean', '🇰🇷', TRUE),
    ('ar', 'العربية', 'Arabic', '🇸🇦', TRUE),
    ('hi', 'हिन्दी', 'Hindi', '🇮🇳', TRUE)
ON CONFLICT (codigo) DO NOTHING;

-- Tabla pivote para rastrear qué idiomas se han solicitado para cada noticia
-- Esto permite saber qué traducciones se han pedido (no necesariamente completado)
CREATE TABLE IF NOT EXISTS traducciones_target (
    noticia_id VARCHAR(32) REFERENCES noticias(id) ON DELETE CASCADE,
    lang_to CHAR(5) REFERENCES idiomas_disponibles(codigo),
    created_at TIMESTAMP DEFAULT NOW(),
    PRIMARY KEY (noticia_id, lang_to)
);

-- Índice para consultas por idioma
CREATE INDEX IF NOT EXISTS idx_traducciones_target_lang_to ON traducciones_target (lang_to);
CREATE INDEX IF NOT EXISTS idx_traducciones_target_noticia_id ON traducciones_target (noticia_id);

-- Añadir columna lang_to a traducciones si no existe (para compatibilidad)
-- La columna lang_to ya existe en 05-traducciones.sql como CHAR(5) NOT NULL
-- Esta migración asegura que REFERENCIAS EXTERNAS funcionen correctamente

-- Añadir clave foránea a traducciones.lang_to referencing idiomas_disponibles
-- Primero verificamos si la constraint existe, si no la añadimos
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints 
        WHERE constraint_name = 'traducciones_lang_to_fkey' 
        AND table_name = 'traducciones'
    ) THEN
        -- Añadir constraint solo si es segura (datos existentes son compatibles)
        ALTER TABLE traducciones 
        ADD CONSTRAINT traducciones_lang_to_fkey 
        FOREIGN KEY (lang_to) REFERENCES idiomas_disponibles(codigo);
    END IF;
END $$;
