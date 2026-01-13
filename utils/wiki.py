import requests
import logging
from cache import cache_get, cache_set
from db import get_read_conn, get_write_conn

logger = logging.getLogger(__name__)

# Cache for 24 hours
CACHE_TTL = 86400

def fetch_wiki_data(name, entity_type=None):
    """
    Fetch image URL AND summary from Wikipedia API for any entity.
    Returns tuple: (image_url, summary)
    """
    # 1. Check Cache
    cache_key = f"wiki:data:{name.lower()}"
    cached_data = cache_get(cache_key)
    if cached_data is not None:
        # Cache stores dict: {"image": url, "summary": text}
        if isinstance(cached_data, dict):
            return cached_data.get("image"), cached_data.get("summary")
        # Legacy cache (string URL only)? Support migration or ignore
        if isinstance(cached_data, str) and cached_data != "NO_IMAGE":
             return cached_data, None 
        return None, None

    # 2. Check Database
    try:
        with get_read_conn() as conn:
            with conn.cursor() as cur:
                cur.execute("SELECT image_url, summary, summary_es FROM entity_images WHERE entity_name = %s", (name,))
                row = cur.fetchone()
                if row:
                    image_url, summary, summary_es = row
                    # Prefer the translated summary if it exists
                    final_summary = summary_es if summary_es else summary
                    
                    # Update cache and return
                    cache_value = {"image": image_url, "summary": final_summary}
                    cache_set(cache_key, cache_value, ttl_seconds=CACHE_TTL)
                    return image_url, final_summary
    except Exception as e:
        logger.error(f"DB read error for {name}: {e}")

    # 3. Fetch from Wikipedia
    summary_en = None
    summary_es = None
    status_es = 'none'
    
    image_url, summary = _query_wikipedia_api_full(name, lang='es')
    if summary:
        summary_es = summary
        status_es = 'done'
    else:
        # Try English if Spanish failed
        img_en, summ_en = _query_wikipedia_api_full(name, lang='en')
        if summ_en:
            summary = summ_en
            summary_en = summ_en
            status_es = 'pending'
            if not image_url:
                image_url = img_en

    # 4. Persist to Database (found or not)
    try:
        with get_write_conn() as conn:
            with conn.cursor() as cur:
                cur.execute("""
                    INSERT INTO entity_images (entity_name, image_url, summary, summary_en, summary_es, status_es, last_checked)
                    VALUES (%s, %s, %s, %s, %s, %s, NOW())
                    ON CONFLICT (entity_name) DO UPDATE 
                    SET image_url = EXCLUDED.image_url, 
                        summary = EXCLUDED.summary,
                        summary_en = EXCLUDED.summary_en,
                        summary_es = EXCLUDED.summary_es,
                        status_es = EXCLUDED.status_es,
                        last_checked = NOW()
                """, (name, image_url, summary, summary_en, summary_es, status_es))
                conn.commit()
    except Exception as e:
         logger.error(f"DB write error for {name}: {e}")

    # 5. Cache Result
    cache_value = {"image": image_url, "summary": summary}
    cache_set(cache_key, cache_value, ttl_seconds=CACHE_TTL)
    
    return image_url, summary


def _query_wikipedia_api_full(query, lang='es'):
    """
    Query Wikipedia API for thumbnail and summary.
    """
    try:
        url = f"https://{lang}.wikipedia.org/w/api.php"
        params = {
            "action": "query",
            "format": "json",
            "prop": "pageimages|extracts",
            "piprop": "thumbnail",
            "pithumbsize": 300, # Larger size requested
            "exintro": 1,
            "explaintext": 1,
            "exchars": 400, # Limit chars
            "titles": query,
            "redirects": 1,
            "origin": "*"
        }
        
        # Wikipedia requires a User-Agent
        headers = {
            "User-Agent": "NewsEntityStats/1.0 (internal_tool; contact@example.com)"
        }
        
        response = requests.get(url, params=params, headers=headers, timeout=2) # Fast timeout
        data = response.json()
        
        pages = data.get("query", {}).get("pages", {})
        
        for page_id, page_data in pages.items():
            if page_id == "-1": 
                continue # Not found
            
            image_url = None
            if "thumbnail" in page_data:
                image_url = page_data["thumbnail"]["source"]
            
            summary = page_data.get("extract")
            if summary and "may refer to:" in summary: # Disambiguation page
                 summary = None
            
            if image_url or summary:
                return image_url, summary
                
    except Exception as e:
        logger.error(f"Error fetching wiki data for {query} ({lang}): {e}")
        
    return None, None
