"""
URL Feed Discovery Worker
This worker automatically discovers RSS feeds from URLs stored in fuentes_url table
and creates entries in the feeds table (or feeds_pending for review).
Runs every 15 minutes.
"""

import os
import sys
import time
import logging
from datetime import datetime
from typing import List, Dict

# Add parent directory to path to import modules
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from db import get_conn
from utils.feed_discovery import discover_feeds, get_feed_metadata
from utils.feed_analysis import (
    analyze_feed,
    get_country_id_by_name,
    get_category_id_by_name
)

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# Configuration
CHECK_INTERVAL = int(os.getenv('URL_DISCOVERY_INTERVAL_MIN', '15')) * 60  # Default: 15 minutes
BATCH_SIZE = int(os.getenv('URL_DISCOVERY_BATCH_SIZE', '10'))  # Process URLs in batches
MAX_FEEDS_PER_URL = int(os.getenv('MAX_FEEDS_PER_URL', '5'))  # Max feeds to create per URL


def get_pending_urls(limit: int = BATCH_SIZE) -> List[Dict]:
    """
    Get URLs that need to be processed.
    Priority: never checked > failed checks > oldest successful checks
    """
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute("""
                SELECT id, nombre, url, categoria_id, pais_id, idioma, last_status
                FROM fuentes_url
                WHERE active = TRUE
                ORDER BY 
                    CASE 
                        WHEN last_check IS NULL THEN 1  -- Never checked (highest priority)
                        WHEN last_status = 'error' THEN 2  -- Failed checks
                        WHEN last_status = 'no_feeds' THEN 3  -- No feeds found
                        ELSE 4  -- Successful checks (lowest priority)
                    END,
                    last_check ASC NULLS FIRST
                LIMIT %s
            """, (limit,))
            
            columns = [desc[0] for desc in cur.description]
            return [dict(zip(columns, row)) for row in cur.fetchall()]


def update_url_status(url_id: int, status: str, message: str = None, http_code: int = None):
    """Update the status of a URL source"""
    with get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute("""
                UPDATE fuentes_url
                SET last_check = NOW(),
                    last_status = %s,
                    status_message = %s,
                    last_http_code = %s
                WHERE id = %s
            """, (status, message, http_code, url_id))
        conn.commit()


def create_pending_feed(
    fuente_url_id: int,
    feed_url: str,
    metadata: Dict,
    analysis: Dict,
    categoria_id: int = None,
    pais_id: int = None,
    idioma: str = None
) -> bool:
    """
    Create a pending feed entry for manual review
    """
    try:
        with get_conn() as conn:
            # Get detected country ID
            detected_country_id = None
            if analysis.get('detected_country'):
                detected_country_id = get_country_id_by_name(conn, analysis['detected_country'])
            
            # Get suggested category ID
            suggested_categoria_id = None
            if analysis.get('suggested_category'):
                suggested_categoria_id = get_category_id_by_name(conn, analysis['suggested_category'])
            
            with conn.cursor() as cur:
                cur.execute("""
                   INSERT INTO feeds_pending (
                        fuente_url_id, feed_url, feed_title, feed_description,
                        feed_language, feed_type, entry_count,
                        detected_country_id, suggested_categoria_id,
                        categoria_id, pais_id, idioma, notes
                    )
                    VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                    ON CONFLICT (feed_url) DO UPDATE
                        SET feed_title = EXCLUDED.feed_title,
                            feed_description = EXCLUDED.feed_description,
                            discovered_at = NOW()
                    RETURNING id
                """, (
                    fuente_url_id,
                    feed_url,
                    metadata.get('title', 'Feed sin título'),
                    metadata.get('description', '')[:500],
                    analysis.get('language'),
                    'rss',  # Default type
                    metadata.get('entry_count', 0),
                    detected_country_id,
                    suggested_categoria_id,
                    categoria_id,
                    pais_id,
                    idioma,
                    analysis.get('analysis_notes', '')
                ))
                
                result = cur.fetchone()
                conn.commit()
                
                if result:
                    logger.info(f"Created pending feed for review: {metadata.get('title')} ({feed_url})")
                    return True
                else:
                    logger.debug(f"Pending feed updated: {feed_url}")
                    return False
                    
    except Exception as e:
        logger.error(f"Error creating pending feed {feed_url}: {e}")
        return False


def create_feed_from_metadata(
    feed_url: str,
    fuente_url_id: int = None,
    categoria_id: int = None,
    pais_id: int = None,
    idioma: str = None,
    auto_approve: bool = False,
    context_title: str = None
) -> Dict:
    """
    Create a feed entry from discovered feed URL with intelligent analysis.
    
    Returns:
        {
            'created': True/False,
            'pending': True/False,
            'status': 'created'/'pending'/'existing'/'error',
            'message': 'Description'
        }
    """
    result = {
        'created': False,
        'pending': False,
        'status': 'error',
        'message': ''
    }
    
    try:
        # Get feed metadata
        metadata = get_feed_metadata(feed_url, timeout=10)
        
        if not metadata:
            result['message'] = 'No se pudo obtener metadata del feed'
            logger.warning(f"{result['message']}: {feed_url}")
            return result
        
        # Add URL to metadata for analysis
        metadata['url'] = feed_url
        
        # Use context title if provided, otherwise use metadata title
        # This helps when feed XML title is generic (e.g. "RSS Feed") but site link had meaningful text
        feed_title = context_title if context_title else metadata.get('title', 'Feed sin título')
        # Update metadata for consistency in pending feeds AND analysis
        metadata['title'] = feed_title
        
        # Perform intelligent analysis
        analysis = analyze_feed(metadata)
        
        # Determine if we need manual review
        needs_review = False
        
        # If parent URL has no category or country, we need review
        if not categoria_id or not pais_id:
            needs_review = True
            logger.info(f"Feed needs review (missing categoria_id={categoria_id} or pais_id={pais_id})")
        
        # If auto_approve is disabled, we need review
        if not auto_approve:
            needs_review = True
        
        # Enhance metadata with analysis
        if not idioma and analysis.get('language'):
            idioma = analysis['language']
        
        # If needs review, create pending feed
        if needs_review:
            created_pending = create_pending_feed(
                fuente_url_id=fuente_url_id,
                feed_url=feed_url,
                metadata=metadata,
                analysis=analysis,
                categoria_id=categoria_id,
                pais_id=pais_id,
                idioma=idioma
            )
            
            result['pending'] = created_pending
            result['status'] = 'pending'
            result['message'] = f"Feed creado y pendiente de revisión (país: {analysis.get('detected_country', 'N/A')}, categoría sugerida: {analysis.get('suggested_category', 'N/A')})"
            return result
        
        # Otherwise, create feed directly
        nombre = feed_title
        descripcion = metadata.get('description', '')
        
        with get_conn() as conn:
            with conn.cursor() as cur:
                cur.execute("""
                    INSERT INTO feeds (nombre, descripcion, url, categoria_id, pais_id, idioma, fuente_url_id, activo)
                    VALUES (%s, %s, %s, %s, %s, %s, %s, TRUE)
                    ON CONFLICT (url) DO NOTHING
                    RETURNING id
                """, (
                    nombre,
                    descripcion[:500] if descripcion else None,
                    feed_url,
                    categoria_id,
                    pais_id,
                    idioma,
                    fuente_url_id
                ))
                
                feed_result = cur.fetchone()
                conn.commit()
                
                if feed_result:
                    logger.info(f"Created new feed: {nombre} ({feed_url})")
                    result['created'] = True
                    result['status'] = 'created'
                    result['message'] = f"Feed creado exitosamente"
                else:
                    logger.debug(f"Feed already exists: {feed_url}")
                    result['status'] = 'existing'
                    result['message'] = 'El feed ya existe'
                    
    except Exception as e:
        logger.error(f"Error creating feed from {feed_url}: {e}")
        result['message'] = str(e)
        result['status'] = 'error'
    
    return result


def process_url_source(url_data: Dict) -> Dict:
    """
    Process a single URL source to discover and create feeds.
    Returns statistics about the operation.
    """
    url_id = url_data['id']
    source_url = url_data['url']
    nombre = url_data['nombre']
    categoria_id = url_data['categoria_id']
    pais_id = url_data['pais_id']
    idioma = url_data['idioma']
    
    logger.info(f"Processing URL source: {nombre} ({source_url})")
    logger.info(f"  Parent metadata: categoria_id={categoria_id}, pais_id={pais_id}, idioma={idioma}")
    
    stats = {
        'url_id': url_id,
        'url': source_url,
        'discovered': 0,
        'created': 0,
        'pending': 0,
        'existing': 0,
        'errors': 0,
        'status': 'unknown'
    }
    
    try:
        # Discover feeds from URL
        discovered = discover_feeds(source_url, timeout=15)
        stats['discovered'] = len(discovered)
        
        if not discovered:
            logger.warning(f"No feeds discovered from: {source_url}")
            update_url_status(url_id, 'no_feeds', 'No se encontraron feeds RSS', 200)
            stats['status'] = 'no_feeds'
            return stats
        
        # Filter only valid feeds
        valid_feeds = [f for f in discovered if f.get('valid', False)]
        
        if not valid_feeds:
            logger.warning(f"No valid feeds found for: {source_url}")
            update_url_status(url_id, 'no_valid_feeds', f'Se encontraron {len(discovered)} feeds pero ninguno válido')
            stats['status'] = 'no_valid_feeds'
            return stats
        
        # Limit number of feeds per URL
        feeds_to_create = valid_feeds[:MAX_FEEDS_PER_URL]
        
        logger.info(f"Found {len(valid_feeds)} valid feeds, processing up to {len(feeds_to_create)}")
        
        # Determine if auto-approve (parent has category AND country)
        auto_approve = bool(categoria_id and pais_id)
        
        if not auto_approve:
            logger.info("→ Feeds will require manual review (parent lacks category or country)")
        else:
            logger.info("→ Feeds will be auto-approved (parent has complete metadata)")
        
        # Create feeds
        for feed_info in feeds_to_create:
            feed_url = feed_info['url']
            
            try:
                result = create_feed_from_metadata(
                    feed_url=feed_url,
                    fuente_url_id=url_id,
                    categoria_id=categoria_id,
                    pais_id=pais_id,
                    idioma=idioma,
                    auto_approve=auto_approve,
                    context_title=feed_info.get('context_label')
                )
                
                if result['status'] == 'created':
                    stats['created'] += 1
                elif result['status'] == 'pending':
                    stats['pending'] += 1
                elif result['status'] == 'existing':
                    stats['existing'] += 1
                else:
                    stats['errors'] += 1
                    
            except Exception as e:
                logger.error(f"Error creating feed {feed_url}: {e}")
                stats['errors'] += 1
        
        # Update URL status
        if stats['created'] > 0 or stats['pending'] > 0:
            parts = []
            if stats['created'] > 0:
                parts.append(f"{stats['created']} creados")
            if stats['pending'] > 0:
                parts.append(f"{stats['pending']} pendientes de revisión")
            if stats['existing'] > 0:
                parts.append(f"{stats['existing']} ya existían")
            
            message = ", ".join(parts)
            update_url_status(url_id, 'success', message, 200)
            stats['status'] = 'success'
        elif stats['existing'] > 0:
            message = f"Todos los {stats['existing']} feeds ya existían"
            update_url_status(url_id, 'existing', message, 200)
            stats['status'] = 'existing'
        else:
            message = f"No se pudieron procesar feeds ({stats['errors']} errores)"
            update_url_status(url_id, 'error', message)
            stats['status'] = 'error'
        
        logger.info(f"Processed {source_url}: {stats['created']} created, {stats['pending']} pending, {stats['existing']} existing, {stats['errors']} errors")
        
    except Exception as e:
        logger.error(f"Error processing URL {source_url}: {e}")
        update_url_status(url_id, 'error', str(e)[:200])
        stats['status'] = 'error'
        stats['errors'] += 1
    
    return stats


def process_batch():
    """Process a batch of URL sources"""
    logger.info("=" * 80)
    logger.info(f"Starting URL discovery batch - {datetime.now().isoformat()}")
    
    # Get pending URLs
    urls = get_pending_urls(limit=BATCH_SIZE)
    
    if not urls:
        logger.info("No pending URLs to process")
        return
    
    logger.info(f"Processing {len(urls)} URL sources")
    
    # Process statistics
    total_stats = {
        'processed': 0,
        'discovered': 0,
        'created': 0,
        'pending': 0,
        'existing': 0,
        'errors': 0
    }
    
    # Process each URL
    for url_data in urls:
        stats = process_url_source(url_data)
        
        total_stats['processed'] += 1
        total_stats['discovered'] += stats['discovered']
        total_stats['created'] += stats['created']
        total_stats['pending'] += stats['pending']
        total_stats['existing'] += stats['existing']
        total_stats['errors'] += stats['errors']
        
        # Small delay between URLs to avoid hammering servers
        time.sleep(2)
    
    # Log summary
    logger.info("-" * 80)
    logger.info(f"Batch complete:")
    logger.info(f"  - Processed: {total_stats['processed']} URLs")
    logger.info(f"  - Discovered: {total_stats['discovered']} feeds")
    logger.info(f"  - Created: {total_stats['created']} new feeds")
    logger.info(f"  - Pending review: {total_stats['pending']} feeds")
    logger.info(f"  - Already existing: {total_stats['existing']} feeds")
    logger.info(f"  - Errors: {total_stats['errors']}")
    logger.info("=" * 80)


def main():
    """Main worker loop"""
    logger.info("URL Feed Discovery Worker started")
    logger.info(f"Check interval: {CHECK_INTERVAL} seconds ({CHECK_INTERVAL // 60} minutes)")
    logger.info(f"Batch size: {BATCH_SIZE}")
    logger.info(f"Max feeds per URL: {MAX_FEEDS_PER_URL}")
    
    # Run immediately on start
    try:
        process_batch()
    except Exception as e:
        logger.error(f"Error in initial batch: {e}", exc_info=True)
    
    # Main loop
    while True:
        try:
            logger.info(f"Waiting {CHECK_INTERVAL} seconds until next batch...")
            time.sleep(CHECK_INTERVAL)
            process_batch()
            
        except KeyboardInterrupt:
            logger.info("Worker stopped by user")
            break
        except Exception as e:
            logger.error(f"Error in main loop: {e}", exc_info=True)
            # Wait a bit before retrying to avoid rapid failure loops
            time.sleep(60)


if __name__ == "__main__":
    main()
