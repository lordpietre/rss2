INSERT INTO continentes (id, nombre) VALUES
(1, 'África'),
(2, 'América'),
(3, 'Asia'),
(4, 'Europa'),
(5, 'Oceanía'),
(6, 'Antártida')
ON CONFLICT (id) DO NOTHING;

