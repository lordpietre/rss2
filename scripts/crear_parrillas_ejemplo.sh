#!/bin/bash
# Script de ejemplo para crear parrillas de videos

echo "🎬 Creando parrillas de ejemplo..."

# 1. Noticias de Bulgaria
docker-compose exec -T db psql -U rss -d rss << EOF
INSERT INTO video_parrillas (
    nombre, descripcion, tipo_filtro,
    pais_id, max_noticias, duracion_maxima,
    idioma_voz, template, include_images, include_subtitles,
    frecuencia, activo
) VALUES (
    'Noticias de Bulgaria',
    'Resumen diario de las noticias más importantes de Bulgaria',
    'pais',
    (SELECT id FROM paises WHERE nombre ILIKE '%bulgaria%' LIMIT 1),
    5, 180,
    'es', 'standard', true, true,
    'daily', true
) ON CONFLICT DO NOTHING;
EOF

# 2. Ciencia en Europa
docker-compose exec -T db psql -U rss -d rss << EOF
INSERT INTO video_parrillas (
    nombre, descripcion, tipo_filtro,
    categoria_id, continente_id, max_noticias,
    idioma_voz, template, include_subtitles,
    frecuencia, activo
) VALUES (
    'Ciencia en Europa',
    'Las últimas noticias científicas de Europa',
    'categoria',
    (SELECT id FROM categorias WHERE nombre ILIKE '%ciencia%' LIMIT 1),
    (SELECT id FROM continentes WHERE nombre = 'Europa' LIMIT 1),
    7,
    'es', 'modern', true,
    'daily', true
) ON CONFLICT DO NOTHING;
EOF

# 3. Tecnología Global
docker-compose exec -T db psql -U rss -d rss << EOF
INSERT INTO video_parrillas (
    nombre, descripcion, tipo_filtro,
    categoria_id, max_noticias, duracion_maxima,
    idioma_voz, template, include_subtitles,
    frecuencia, activo
) VALUES (
    'Tech News Daily',
    'Resumen diario de tecnología mundial',
    'categoria',
    (SELECT id FROM categorias WHERE nombre ILIKE '%tecnolog%' LIMIT 1),
    8, 300,
    'es', 'modern', true,
    'daily', true
) ON CONFLICT DO NOTHING;
EOF

echo "✅ Parrillas creadas!"
echo ""
echo "📊 Ver parrillas creadas:"
docker-compose exec -T db psql -U rss -d rss -c "
SELECT id, nombre, tipo_filtro, max_noticias, frecuencia, activo 
FROM video_parrillas 
ORDER BY id DESC;
"

echo ""
echo "🎥 Accede a la interfaz web en: http://localhost:8001/parrillas/"
echo ""
echo "💡 Para generar un video manualmente:"
echo "   docker-compose exec web python generar_videos_noticias.py <id_parrilla>"
echo ""
echo "📅 Para generar todos los videos del día:"
echo "   docker-compose exec web python generar_videos_noticias.py"
