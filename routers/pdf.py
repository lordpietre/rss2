"""
PDF Export router.
"""
from flask import Blueprint, make_response, render_template, url_for
from db import get_conn
from psycopg2 import extras
from weasyprint import HTML
import logging
import re
from io import BytesIO

logger = logging.getLogger(__name__)

pdf_bp = Blueprint("pdf", __name__, url_prefix="/pdf")

def clean_text(text):
    """Clean text from problematic characters for PDF generation."""
    if not text:
        return ""
    # Remove <unk> tokens
    text = text.replace('<unk>', '')
    text = text.replace('�', '')
    # Remove other problematic Unicode characters
    text = re.sub(r'[\x00-\x08\x0B-\x0C\x0E-\x1F\x7F-\x9F]', '', text)
    return text.strip()

@pdf_bp.route("/noticia/<noticia_id>")
def export_noticia(noticia_id):
    """Exportar noticia a PDF."""
    try:
        with get_conn() as conn:
            with conn.cursor(cursor_factory=extras.DictCursor) as cur:
                cur.execute("""
                    SELECT 
                        n.*,
                        t.titulo_trad, t.resumen_trad, t.lang_to,
                        c.nombre as categoria_nombre,
                        p.nombre as pais_nombre
                    FROM noticias n
                    LEFT JOIN traducciones t ON t.noticia_id = n.id AND t.status = 'done' AND t.lang_to = 'es'
                    LEFT JOIN categorias c ON c.id = n.categoria_id
                    LEFT JOIN paises p ON p.id = n.pais_id
                    WHERE n.id = %s
                """, (noticia_id,))
                noticia = cur.fetchone()

        if not noticia:
            return "Noticia no encontrada", 404

        # Prepare data for template
        d = dict(noticia)
        
        # Use translated content if available and clean it
        titulo = clean_text(d.get('titulo_trad') or d.get('titulo', ''))
        resumen = clean_text(d.get('resumen_trad') or d.get('resumen', ''))
        
        # Don't include external images to avoid SSL/network errors
        # imagen_url = d.get('imagen_url') if d.get('imagen_url', '').startswith('http') else None
        
        html_content = render_template(
            "pdf_template.html",
            titulo=titulo,
            resumen=resumen,
            fecha=d.get('fecha', ''),
            fuente=d.get('fuente_nombre', ''),  # Esta columna existe directamente en noticias
            categoria=d.get('categoria_nombre', ''),
            url=d.get('url', ''),
            imagen_url=None  # Disable images for now to avoid errors
        )

        # Convert to PDF using WeasyPrint
        logger.info(f"Generating PDF for noticia {noticia_id}")
        
        # Create PDF in memory
        pdf_file = BytesIO()
        HTML(string=html_content).write_pdf(pdf_file)
        pdf_bytes = pdf_file.getvalue()
        
        response = make_response(pdf_bytes)
        response.headers['Content-Type'] = 'application/pdf'
        response.headers['Content-Disposition'] = f'attachment; filename=noticia_{noticia_id}.pdf'
        logger.info(f"PDF generated successfully for noticia {noticia_id}")
        return response
        
    except Exception as e:
        logger.error(f"Error generando PDF para noticia {noticia_id}: {str(e)}", exc_info=True)
        return f"Error generando PDF: {str(e)}", 500

