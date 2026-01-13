import logging
import hashlib
from datetime import datetime
from newspaper import Article, ArticleException, Config
import requests
from db import get_write_conn, get_read_conn

# Configuration
logger = logging.getLogger("url_worker")
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(name)s - %(levelname)s - %(message)s')

def get_active_urls():
    """Get all active URL sources."""
    with get_read_conn() as conn:
        with conn.cursor() as cur:
            cur.execute("""
                SELECT id, nombre, url, categoria_id, pais_id, idioma
                FROM fuentes_url 
                WHERE active = true
            """)
            return cur.fetchall()

def update_source_status(source_id, status, message, http_code=0):
    """Update the status of a URL source."""
    with get_write_conn() as conn:
        with conn.cursor() as cur:
            cur.execute("""
                UPDATE fuentes_url 
                SET last_check = NOW(),
                    last_status = %s,
                    status_message = %s,
                    last_http_code = %s
                WHERE id = %s
            """, (status, message, http_code, source_id))
        conn.commit()

def save_article(source, article):
    """Save the extracted article to the database."""
    source_id, source_name, source_url, cat_id, pais_id, lang = source
    
    # Use the article url if possible, otherwise source_url
    final_url = article.url or source_url
    noticia_id = hashlib.md5(final_url.encode("utf-8")).hexdigest()
    
    with get_write_conn() as conn:
        with conn.cursor() as cur:
            # Check if exists
            cur.execute("SELECT id FROM noticias WHERE id = %s", (noticia_id,))
            if cur.fetchone():
                return False # Already exists
            
            # Prepare data
            title = article.title or "Sin título"
            summary = article.summary or article.text[:500] 
            image_url = article.top_image
            pub_date = article.publish_date or datetime.utcnow()
            
            cur.execute("""
                INSERT INTO noticias (
                    id, titulo, resumen, url, fecha, imagen_url, 
                    fuente_nombre, categoria_id, pais_id
                ) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s)
                ON CONFLICT (id) DO NOTHING
            """, (
                noticia_id, title, summary, final_url, pub_date, image_url,
                source_name, cat_id, pais_id
            ))
        conn.commit()
    return True

def process_url(source):
    """Process a single URL source."""
    source_id, name, url, _, _, _ = source
    
    logger.info(f"Processing URL: {url} ({name})")
    
    try:
        # Browser-like headers
        user_agent = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36'
        config = Config()
        config.browser_user_agent = user_agent
        config.request_timeout = 30

        article = Article(url, config=config, language='es')
        article.download()
        
        if not article.html:
             update_source_status(source_id, "ERROR", "No content downloaded (Empty HTML)", 0)
             return

        article.parse()
        try:
            article.nlp()
        except:
            pass 
            
        if not article.title:
            update_source_status(source_id, "ERROR_PARSE", "Could not extract title (Page might be not an article)", 200)
            return
            
        saved = save_article(source, article)
        
        status_msg = "News created successfully" if saved else "News already exists"
        update_source_status(source_id, "OK", status_msg, 200)
        logger.info(f"Success {url}: {status_msg}")

    except ArticleException as ae:
        logger.error(f"Newspaper Error {url}: {ae}")
        update_source_status(source_id, "ERROR_DOWNLOAD", str(ae)[:200], 0)
    except requests.exceptions.RequestException as re:
        logger.error(f"Network Error {url}: {re}")
        update_source_status(source_id, "ERROR_NETWORK", str(re)[:200], 0)
    except Exception as e:
        logger.error(f"Unexpected Error {url}: {e}")
        update_source_status(source_id, "ERROR_UNKNOWN", str(e)[:200], 500)

def main():
    logger.info("Starting URL Worker")
    urls = get_active_urls()
    logger.info(f"Found {len(urls)} active URLs")
    for source in urls:
        process_url(source)

if __name__ == "__main__":
    main()
