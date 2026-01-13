# 🎬 Sistema de Parrillas de Videos Automatizados

## 📋 Descripción

Este sistema permite generar videos automáticos de noticias filtradas según diferentes criterios:
- **Por País**: Noticias de Bulgaria, España, Estados Unidos, etc.
- **Por Categoría**: Ciencia, Tecnología, Deport, Política, etc.
- **Por Entidad**: Personas u organizaciones específicas (ej: "Donald Trump", "OpenAI")
- **Por Continente**: Europa, Asia, América, etc.

## 🎯 Características

### ✅ Sistema Implementado

1. **Base de Datos**
   - Tabla `video_parrillas`: Configuraciones de parrillas
   - Tabla `video_generados`: Registro de videos creados
   - Tabla `video_noticias`: Relación entre videos y noticias

2. **API REST**
   - `GET /parrillas/` - Listado de parrillas
   - `GET /parrillas/<id>` - Detalle de parrilla
   - `POST /parrillas/nueva` - Crear parrilla
   - `GET /parrillas/api/<id>/preview` - Preview de noticias
   - `POST /parrillas/api/<id>/generar` - Generar video
   - `POST /parrillas/api/<id>/toggle` - Activar/desactivar
   - `DELETE /parrillas/api/<id>` - Eliminar parrilla

3. **Generador de Videos**
   - Script: `generar_videos_noticias.py`
   - Integración con AllTalk TTS
   - Generación automática de subtítulos SRT
   - Soporte para múltiples idiomas

## 🚀 Uso Rápido

### 1. Crear una Parrilla

```bash
# Acceder a la interfaz web
http://localhost:8001/parrillas/

# O usar SQL directo
docker-compose exec -T db psql -U rss -d rss -c "
INSERT INTO video_parrillas (nombre, descripcion, tipo_filtro, pais_id, max_noticias, frecuencia, activo)
VALUES ('Noticias de Bulgaria', 'Resumen diario de noticias de Bulgaria', 'pais', 
        (SELECT id FROM paises WHERE nombre = 'Bulgaria'), 5, 'daily', true);
"
```

### 2. Generar Video Manualmente

```bash
# Generar video para parrilla con ID 1
docker-compose exec web python generar_videos_noticias.py 1
```

### 3. Generación Automática (Diaria)

```bash
# Procesar todas las parrillas activas con frecuencia 'daily'
docker-compose exec web python generar_videos_noticias.py
```

## 📝 Ejemplos de Parrillas

### Ejemplo 1: Noticias de Ciencia en Europa

```sql
INSERT INTO video_parrillas (
    nombre, descripcion, tipo_filtro,
    categoria_id, continente_id,
    max_noticias, duracion_maxima, idioma_voz,
    frecuencia, activo
) VALUES (
    'Ciencia en Europa',
    'Las últimas noticias científicas de Europa',
    'categoria',
    (SELECT id FROM categorias WHERE nombre ILIKE '%ciencia%'),
    (SELECT id FROM continentes WHERE nombre = 'Europa'),
    7, 300, 'es',
    'daily', true
);
```

### Ejemplo 2: Noticias sobre una Persona

```sql
INSERT INTO video_parrillas (
    nombre, descripcion, tipo_filtro,
    entidad_nombre, entidad_tipo,
    max_noticias, idioma_voz,
    frecuencia, activo
) VALUES (
    'Donald Trump en las Noticias',
    'Todas las menciones de Donald Trump',
    'entidad',
    'Donald Trump', 'persona',
    10, 'es',
    'daily', true
);
```

### Ejemplo 3: Noticias de Tecnología

```sql
INSERT INTO video_parrillas (
    nombre, descripcion, tipo_filtro,
    categoria_id,
    max_noticias, idioma_voz,
    include_subtitles, template,
    frecuencia, activo
) VALUES (
    'Tech News Daily',
    'Resumen diario de tecnología',
    'categoria',
    (SELECT id FROM categorias WHERE nombre ILIKE '%tecnolog%'),
    8, 'es',
    true, 'modern',
    'daily', true
);
```

## 🔧 Configuración Avanzada

### Opciones de Parrilla

| Campo | Tipo | Descripción |
|-------|------|-------------|
| `nombre` | string | Nombre único de la parrilla |
| `descripcion` | text | Descripción detallada |
| `tipo_filtro` | enum | 'pais', 'categoria', 'entidad', 'continente', 'custom' |
| `pais_id` | int | ID del país (si tipo_filtro='pais') |
| `categoria_id` | int | ID de categoría |
| `continente_id` | int | ID de continente |
| `entidad_nombre` | string | Nombre de persona/organización |
| `entidad_tipo` | string | 'persona' o 'organizacion' |
| `max_noticias` | int | Máximo de noticias por video (default: 5) |
| `duracion_maxima` | int | Duración máxima en segundos (default: 180) |
| `idioma_voz` | string | Idioma del TTS ('es', 'en', etc.) |
| `template` | string | 'standard', 'modern', 'minimal' |
| `include_images` | bool | Incluir imágenes en el video |
| `include_subtitles` | bool | Generar subtítulos SRT |
| `frecuencia` | string | 'daily', 'weekly', 'manual' |

### Configuración de AllTalk

El sistema utiliza AllTalk para generar la narración con voz. Configurar en docker-compose.yml:

```yaml
environment:
  ALLTALK_URL: http://alltalk:7851
```

## 📊 Estructura de Archivos Generados

```
data/
  videos/
    <video_id>/
      script.txt          # Libreto completo del video
      audio.wav           # Audio generado con TTS
      subtitles.srt       # Subtítulos (si enabled)
      metadata.json       # Metadata del video
```

## 🔄 Workflow de Generación

1. **Consulta de Noticias**: Filtra noticias según criterios de la parrilla
2. **Construcción de Script**: Genera libreto narrativo
3. **Síntesis de Voz**: Envía texto a AllTalk TTS
4. **Generación de Subtítulos**: Crea archivo SRT con timestamps
5. **Registro en BD**: Guarda paths y metadata en `video_generados`
6. **Relación de Noticias**: Vincula noticias incluidas en `video_noticias`

## 🎨 Próximas Mejoras

- [ ] Integración con generador de videos (combinar audio + imágenes)
- [ ] Templates visuales personalizados
- [ ] Transiciones entre noticias
- [ ] Música de fondo
- [ ] Logo/branding personalizado
- [ ] Exportación directa a YouTube/TikTok
- [ ] Programación automática con cron
- [ ] Dashboard de analíticas de videos
- [ ] Sistema de thumbnails automáticos

## 🐛 Troubleshooting

### Error: "No hay noticias disponibles"
- Verificar que existan noticias traducidas (`traducciones.status = 'done'`)
- Ajustar filtros de la parrilla
- Verificar rango de fechas (por defecto últimas 24h)

### Error en AllTalk TTS
- Verificar que el servicio AllTalk esté corriendo
- Revisar URL en variable de entorno `ALLTALK_URL`
- Comprobar logs: `docker-compose logs alltalk`

### Video no se genera
- Revisar estado en tabla `video_generados`
- Ver columna `error_message` si `status = 'error'`
- Verificar permisos en directorio `/app/data/videos`

## 📞 Soporte

Para problemas o sugerencias, revisar los logs:

```bash
# Logs del generador
docker-compose exec web python generar_videos_noticias.py <id> 2>&1 | tee video_generation.log

# Ver videos en cola
docker-compose exec -T db psql -U rss -d rss -c "
SELECT id, parrilla_id, titulo, status, fecha_generacion 
FROM video_generados 
ORDER BY fecha_generacion DESC LIMIT 10;
"
```

## 📄 Licencia

Este módulo es parte del sistema RSS2 News Aggregator.
