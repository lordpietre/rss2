INSERT INTO categorias (nombre) VALUES
('Ciencia'),
('Cultura'),
('Deportes'),
('Economía'),
('Educación'),
('Entretenimiento'),
('Internacional'),
('Medio Ambiente'),
('Moda'),
('Opinión'),
('Política'),
('Salud'),
('Sociedad'),
('Tecnología'),
('Viajes')
ON CONFLICT DO NOTHING;

