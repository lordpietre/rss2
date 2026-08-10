BEGIN;

INSERT INTO topics (nombre, descripcion, weight, keywords) VALUES
('Política', 'Noticias políticas y de gobierno', 3, 'política,político,gobierno,presidente,ministro,parlamento,elecciones,partido,senado,diputado'),
('Economía', 'Economía y finanzas', 3, 'economía,económico,finanzas,mercado,inflación,banco,bancos,empresa,empresas,negocio,bolsa,inversión,presupuesto,impuestos'),
('Seguridad', 'Seguridad y defensa', 3, 'seguridad,ataque,ataques,explosión,terrorismo,terrorista,ejército,militar,patrulla,armas,policía'),
('Afganistán', 'Asuntos de Afganistán', 4, 'afganistán,afgano,afgana,kabul,candahar,kandahar,herat,mazar,talibán,talibanes,emirato'),
('Conflicto', 'Conflictos y guerra', 3, 'conflicto,guerra,enfrentamiento,enfrentamientos,crisis,víctimas,frontera,invasIón,ocupación'),
('Internacional', 'Relaciones internacionales', 2, 'internacional,diplomacia,diplomático,acuerdo,acuerdos,naciones unidas,embajador,tratado,extranjero'),
('Deportes', 'Deportes y competiciones', 2, 'deportes,deporte,fútbol,futbol,partido,equipo,liga,campeonato,tenis,baloncesto,cricket,selección'),
('Salud', 'Salud y medicina', 2, 'salud,hospital,médico,médicos,enfermedad,vacuna,tratamiento,pandemia,clínica,medicina'),
('Tecnología', 'Tecnología y digital', 2, 'tecnología,internet,digital,telefonía,telecomunicaciones,software,aplicación,aplicaciones,internet,tecnológico'),
('Educación', 'Educación y enseñanza', 2, 'educación,escuela,escuelas,universidad,universidades,estudiantes,profesor,enseñanza,academia,alumnos'),
('Cultura', 'Cultura y arte', 1, 'cultura,arte,literatura,cine,música,música,patrimonio,historia,museo,libro,libros'),
('Medio Ambiente', 'Clima y medio ambiente', 2, 'medio ambiente,clima,cambio climático,contaminación,sequía,inundación,agua,ecología,desastre natural')
ON CONFLICT (nombre) DO NOTHING;

COMMIT;
