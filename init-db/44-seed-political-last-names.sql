-- Seed common political last names (figuras públicas conocidas)
-- These will be inserted as tags with tipo='persona' and their apellido set
-- Note: These are just initial entries; more will be added dynamically as entities are processed

-- Insert Trump (already may exist, using ON CONFLICT pattern)
INSERT INTO tags (valor, tipo, apellido) VALUES ('Trump', 'persona', 'Trump') ON CONFLICT (valor, tipo) DO UPDATE SET apellido = EXCLUDED.apellido;

-- Insert Meloni
INSERT INTO tags (valor, tipo, apellido) VALUES ('Meloni', 'persona', 'Meloni') ON CONFLICT (valor, tipo) DO UPDATE SET apellido = EXCLUDED.apellido;

-- Insert Biden
INSERT INTO tags (valor, tipo, apellido) VALUES ('Biden', 'persona', 'Biden') ON CONFLICT (valor, tipo) DO UPDATE SET apellido = EXCLUDED.apellido;

-- Insert Macron
INSERT INTO tags (valor, tipo, apellido) VALUES ('Macron', 'persona', 'Macron') ON CONFLICT (valor, tipo) DO UPDATE SET apellido = EXCLUDED.apellido;

-- Insert Scholz
INSERT INTO tags (valor, tipo, apellido) VALUES ('Scholz', 'persona', 'Scholz') ON CONFLICT (valor, tipo) DO UPDATE SET apellido = EXCLUDED.apellido;

-- Create indexes for the new entries
CREATE INDEX IF NOT EXISTS idx_tags_valor_tipo_apellido ON tags(valor, tipo, apellido);