-- Tabla de usuarios
-- Almacena información de autenticación y perfil de usuarios

CREATE TABLE IF NOT EXISTS usuarios (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    last_login TIMESTAMP,
    is_active BOOLEAN DEFAULT TRUE,
    CONSTRAINT username_min_length CHECK (LENGTH(username) >= 3),
    CONSTRAINT email_format CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$')
);

-- Índices para búsquedas rápidas
CREATE INDEX IF NOT EXISTS idx_usuarios_username ON usuarios(username);
CREATE INDEX IF NOT EXISTS idx_usuarios_email ON usuarios(email);
CREATE INDEX IF NOT EXISTS idx_usuarios_active ON usuarios(is_active) WHERE is_active = TRUE;

-- Trigger para actualizar updated_at
CREATE OR REPLACE FUNCTION update_usuario_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_usuario_timestamp
    BEFORE UPDATE ON usuarios
    FOR EACH ROW
    EXECUTE FUNCTION update_usuario_timestamp();

-- Comentarios
COMMENT ON TABLE usuarios IS 'Usuarios registrados del sistema';
COMMENT ON COLUMN usuarios.username IS 'Nombre de usuario único';
COMMENT ON COLUMN usuarios.email IS 'Correo electrónico único';
COMMENT ON COLUMN usuarios.password_hash IS 'Hash bcrypt de la contraseña';
COMMENT ON COLUMN usuarios.is_active IS 'Indica si la cuenta está activa';
