#!/usr/bin/env python3
"""
Generador de videos de noticias a partir de parrillas.
Este script procesa parrillas pendientes y genera videos con TTS.
"""
import os
import sys
import json
import logging
from datetime import datetime
from pathlib import Path
import requests
from db import get_conn
from psycopg2 import extras

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Configuración
OUTPUT_DIR = Path("/app/data/videos")
AUDIO_DIR = Path("/app/data/audio")
SUBTITLES_DIR = Path("/app/data/subtitles")
OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
AUDIO_DIR.mkdir(parents=True, exist_ok=True)
SUBTITLES_DIR.mkdir(parents=True, exist_ok=True)

# URL del servicio AllTalk TTS (ajustar según configuración)
ALLTALK_URL = os.getenv("ALLTALK_URL", "http://alltalk:7851")


def obtener_noticias_parrilla(parrilla, conn):
    """
    Obtiene las noticias que se incluirán en el video según los filtros de la parrilla.
    """
    with conn.cursor(cursor_factory=extras.DictCursor) as cur:
        where_clauses = []
        params = []
        
        if parrilla['pais_id']:
            where_clauses.append("n.pais_id = %s")
            params.append(parrilla['pais_id'])
            
        if parrilla['categoria_id']:
            where_clauses.append("n.categoria_id = %s")
            params.append(parrilla['categoria_id'])
            
        if parrilla['entidad_nombre']:
            where_clauses.append("""
                EXISTS (
                    SELECT 1 FROM tags_noticia tn
                    JOIN tags t ON t.id = tn.tag_id
                    WHERE tn.traduccion_id = tr.id
                    AND t.tipo = %s
                    AND t.valor ILIKE %s
                )
            """)
            params.append(parrilla['entidad_tipo'])
            params.append(f"%{parrilla['entidad_nombre']}%")
        
        # Solo noticias recientes (últimas 24 horas)
        where_clauses.append("n.fecha >= NOW() - INTERVAL '1 day'")
        
        where_sql = " AND ".join(where_clauses) if where_clauses else "1=1"
        
        cur.execute(f"""
            SELECT 
                n.id,
                n.titulo,
                n.imagen_url,
                n.url,
                n.fecha,
                n.fuente_nombre,
                tr.id as traduccion_id,
                tr.titulo_trad,
                tr.resumen_trad,
                p.nombre as pais,
                c.nombre as categoria
            FROM noticias n
            LEFT JOIN traducciones tr ON tr.noticia_id = n.id 
                AND tr.lang_to = %s 
                AND tr.status = 'done'
            LEFT JOIN paises p ON p.id = n.pais_id
            LEFT JOIN categorias c ON c.id = n.categoria_id
            WHERE {where_sql}
            AND tr.id IS NOT NULL
            ORDER BY n.fecha DESC
            LIMIT %s
        """, [parrilla['idioma_voz']] + params + [parrilla['max_noticias']])
        
        return cur.fetchall()


def generar_audio_tts(texto, output_path, idioma='es'):
    """
    Genera audio usando el servicio AllTalk TTS.
    """
    try:
        # Preparar request para AllTalk
        payload = {
            "text_input": texto,
            "text_filtering": "standard",
            "character_voice_gen": "irene2.wav",
            "narrator_enabled": False,
            "narrator_voice_gen": "male_01.wav",
            "text_not_inside": "character",
            "language": idioma,
            "output_file_name": output_path.stem,
            "output_file_timestamp": False,
            "autoplay": False,
            "autoplay_volume": 0.8
        }
        
        response = requests.post(
            f"{ALLTALK_URL}/api/tts-generate",
            json=payload,
            timeout=60
        )
        response.raise_for_status()
        
        # El audio se guarda automáticamente por AllTalk
        # Verificar que existe
        if output_path.exists():
            logger.info(f"Audio generado: {output_path}")
            return True
        else:
            logger.error(f"Audio no encontrado después de generación: {output_path}")
            return False
            
    except Exception as e:
        logger.error(f"Error generating TTS audio: {e}")
        return False


def generar_subtitulos(noticias, output_path):
    """
    Genera archivo SRT de subtítulos.
    """
    try:
        with open(output_path, 'w', encoding='utf-8') as f:
            timestamp = 0
            
            for i, noticia in enumerate(noticias, 1):
                titulo = noticia['titulo_trad'] or noticia['titulo']
                resumen = noticia['resumen_trad'] or ''
                
                # Estimar duración basada en longitud de texto (aprox 150 palabras/min)
                palabras = len((titulo + " " + resumen).split())
                duracion = max(5, palabras / 2.5)  # segundos
                
                # Formatear timestamp SRT
                start_time = timestamp
                end_time = timestamp + duracion
                
                f.write(f"{i}\n")
                f.write(f"{format_srt_time(start_time)} --> {format_srt_time(end_time)}\n")
                f.write(f"{titulo}\n\n")
                
                timestamp = end_time
                
        logger.info(f"Subtítulos generados: {output_path}")
        return True
        
    except Exception as e:
        logger.error(f"Error generating subtitles: {e}")
        return False


def format_srt_time(seconds):
    """Formatea segundos a formato SRT (HH:MM:SS,mmm)."""
    hours = int(seconds // 3600)
    minutes = int((seconds % 3600) // 60)
    secs = int(seconds % 60)
    millis = int((seconds % 1) * 1000)
    return f"{hours:02d}:{minutes:02d}:{secs:02d},{millis:03d}"


def procesar_parrilla(parrilla_id):
    """
    Procesa una parrilla y genera el video.
    """
    logger.info(f"Procesando parrilla {parrilla_id}")
    
    with get_conn() as conn:
        with conn.cursor(cursor_factory=extras.DictCursor) as cur:
            # Obtener configuración de parrilla
            cur.execute("SELECT * FROM video_parrillas WHERE id = %s", (parrilla_id,))
            parrilla = cur.fetchone()
            
            if not parrilla or not parrilla['activo']:
                logger.warning(f"Parrilla {parrilla_id} no encontrada o inactiva")
                return False
            
            # Obtener noticias
            noticias = obtener_noticias_parrilla(parrilla, conn)
            
            if not noticias:
                logger.warning(f"No hay noticias disponibles para parrilla {parrilla_id}")
                return False
            
            logger.info(f"Encontradas {len(noticias)} noticias para el video")
            
            # Crear registro de video
            cur.execute("""
                INSERT INTO video_generados (
                    parrilla_id, titulo, descripcion, status, num_noticias
                ) VALUES (
                    %s, %s, %s, 'processing', %s
                ) RETURNING id
            """, (
                parrilla_id,
                f"{parrilla['nombre']} - {datetime.now().strftime('%Y-%m-%d')}",
                f"Noticias de {parrilla['nombre']}",
                len(noticias)
            ))
            video_id = cur.fetchone()[0]
            conn.commit()
            
            # Preparar directorios
            video_dir = OUTPUT_DIR / str(video_id)
            video_dir.mkdir(exist_ok=True, parents=True)
            
            # --- SETUP LOGGING FOR THIS VIDEO ---
            log_file = video_dir / "generation.log"
            file_handler = logging.FileHandler(log_file, mode='w')
            file_handler.setFormatter(logging.Formatter('%(asctime)s - %(levelname)s - %(message)s'))
            logger.addHandler(file_handler)
            
            try:
                logger.info(f"Iniciando generación de video {video_id}")
                logger.info(f"Directorio: {video_dir}")
                
                # Generar script de narración
                logger.info("Generando guion narrativo...")
                
                script_parts = []
                script_parts.append(f"Hola, bienvenidos a {parrilla['nombre']}.")
                script_parts.append(f"Estas son las noticias más importantes de hoy, {datetime.now().strftime('%d de %B de %Y')}.")
                
                for i, noticia in enumerate(noticias, 1):
                    titulo = noticia['titulo_trad'] or noticia['titulo']
                    resumen = noticia['resumen_trad'] or ''
                    
                    script_parts.append(f"Noticia número {i}.")
                    script_parts.append(titulo)
                    if resumen:
                        script_parts.append(resumen[:500])  # Limitar longitud
                    script_parts.append("")  # Pausa
                
                script_parts.append("Esto ha sido todo por hoy. Gracias por su atención.")
                
                full_script = "\n".join(script_parts)
                
                # Guardar script
                script_path = video_dir / "script.txt"
                with open(script_path, 'w', encoding='utf-8') as f:
                    f.write(full_script)
                
                # Generar audio
                logger.info(f"Generando audio TTS con AllTalk en: {ALLTALK_URL}")
                audio_path = video_dir / "audio.wav"
                if not generar_audio_tts(full_script, audio_path, parrilla['idioma_voz']):
                    raise Exception(f"Fallo al generar audio TTS en {ALLTALK_URL}")
                
                # Generar subtítulos
                if parrilla['include_subtitles']:
                    logger.info("Generando subtítulos SRT...")
                    subtitles_path = video_dir / "subtitles.srt"
                    generar_subtitulos(noticias, subtitles_path)
                else:
                    subtitles_path = None
                
                # Registrar noticias en el video
                for i, noticia in enumerate(noticias, 1):
                    cur.execute("""
                        INSERT INTO video_noticias (
                            video_id, noticia_id, traduccion_id, orden
                        ) VALUES (%s, %s, %s, %s)
                    """, (video_id, noticia['id'], noticia['traduccion_id'], i))
                
                # Actualizar registro de video
                cur.execute("""
                    UPDATE video_generados
                    SET status = 'completed',
                        audio_path = %s,
                        subtitles_path = %s,
                        noticias_ids = %s
                    WHERE id = %s
                """, (
                    str(audio_path),
                    str(subtitles_path) if subtitles_path else None,
                    [n['id'] for n in noticias],
                    video_id
                ))
                
                # Actualizar parrilla
                cur.execute("""
                    UPDATE video_parrillas
                    SET ultima_generacion = NOW()
                    WHERE id = %s
                """, (parrilla_id,))
                
                conn.commit()
                
                logger.info(f"Video {video_id} generado exitosamente")
                
                # Cleanup handler
                logger.removeHandler(file_handler)
                file_handler.close()
                return True
                
            except Exception as e:
                logger.error(f"Error processing video: {e}", exc_info=True)
                
                # Marcar como error
                cur.execute("""
                    UPDATE video_generados
                    SET status = 'error',
                        error_message = %s
                    WHERE id = %s
                """, (str(e), video_id))
                conn.commit()
                
                # Cleanup handler
                logger.removeHandler(file_handler)
                file_handler.close()
                
                return False


def main():
    """
    Función principal: procesa parrillas activas que necesitan generación.
    """
    logger.info("Iniciando generador de videos de noticias")
    
    with get_conn() as conn:
        with conn.cursor(cursor_factory=extras.DictCursor) as cur:
            # Buscar parrillas activas que necesitan generación
            # Por ahora, procesar todas las activas manualmente
            # TODO: Implementar lógica de programación automática
            
            if len(sys.argv) > 1:
                # Modo manual: procesar parrilla específica
                parrilla_id = int(sys.argv[1])
                procesar_parrilla(parrilla_id)
            else:
                # Modo batch: procesar todas las parrillas activas
                cur.execute("""
                    SELECT id FROM video_parrillas
                    WHERE activo = true
                    AND frecuencia = 'daily'
                    AND (ultima_generacion IS NULL 
                         OR ultima_generacion < NOW() - INTERVAL '1 day')
                    ORDER BY id
                """)
                
                parrillas = cur.fetchall()
                logger.info(f"Encontradas {len(parrillas)} parrillas para procesar")
                
                for p in parrillas:
                    try:
                        procesar_parrilla(p['id'])
                    except Exception as e:
                        logger.error(f"Error procesando parrilla {p['id']}: {e}")
                        continue


if __name__ == "__main__":
    main()
