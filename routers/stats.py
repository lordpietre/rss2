from flask import Blueprint, render_template, jsonify
from db import get_read_conn
from datetime import datetime, timedelta
import os
import subprocess
import time
from cache import cached

stats_bp = Blueprint("stats", __name__, url_prefix="/stats")


# ==================================================================================
# ENTITY NORMALIZATION SYSTEM
# ==================================================================================
# Dictionary to map entity name variations to canonical names
import json

CONFIG_FILE = "entity_config.json"
_config_cache = {"data": None, "mtime": 0}

def load_entity_config():
    """Load entity config from JSON file with simple modification time caching."""
    global _config_cache
    try:
        # Check if file exists
        if not os.path.exists(CONFIG_FILE):
            return {"blacklist": [], "synonyms": {}}

        # Check modification time
        mtime = os.path.getmtime(CONFIG_FILE)
        if _config_cache["data"] is not None and mtime <= _config_cache["mtime"]:
            return _config_cache["data"]

        # Load fresh config
        with open(CONFIG_FILE, 'r', encoding='utf-8') as f:
            data = json.load(f)
            
            # Normalize structure
            if "blacklist" not in data: data["blacklist"] = []
            if "synonyms" not in data: data["synonyms"] = {}
            
            # Pre-process synonyms for reverse lookup (variation -> canonical)
            lookup = {}
            for canonical, variations in data["synonyms"].items():
                lookup[canonical.lower()] = canonical # Map canonical to itself
                for var in variations:
                    lookup[var.lower()] = canonical
            
            data["_lookup"] = lookup
            data["_blacklist_set"] = {x.lower() for x in data["blacklist"]}
            
            _config_cache = {"data": data, "mtime": mtime}
            return data
            
    except Exception as e:
        print(f"Error loading entity config: {e}")
        # Return fallback or previous cache if available
        return _config_cache["data"] if _config_cache["data"] else {"blacklist": [], "synonyms": {}}


def normalize_entity_name(name: str, config=None) -> str:
    """Normalize entity name to its canonical form."""
    if config is None:
        config = load_entity_config()
    
    lookup = config.get("_lookup", {})
    return lookup.get(name.lower(), name)


def aggregate_normalized_entities(rows, entity_type='persona'):
    """Aggregate entity counts by normalized names and filter blacklisted items.
    
    Args:
        rows: List of (name, count) tuples from database
        entity_type: Type of entity for normalization (kept for compatibility but config is global now)
        
    Returns:
        List of (normalized_name, total_count) tuples sorted by count
    """
    aggregated = {}
    config = load_entity_config()
    blacklist = config.get("_blacklist_set", set())
    
    for name, count in rows:
        # 1. Check blacklist (exact or lower match)
        if name.lower() in blacklist:
            continue
            
        # 2. Normalize
        normalized = normalize_entity_name(name, config)
        
        # 3. Check blacklist again (in case canonical name is blacklisted)
        if normalized.lower() in blacklist:
            continue
            
        aggregated[normalized] = aggregated.get(normalized, 0) + count
    
    # Sort by count descending
    sorted_items = sorted(aggregated.items(), key=lambda x: x[1], reverse=True)
    return sorted_items

# ==================================================================================


@stats_bp.route("/")
def index():
    """Stats dashboard page."""
    
    # Calculate translation stats for the banner
    with get_read_conn() as conn:
        with conn.cursor() as cur:
            # Translations per minute (last 5 minutes)
            cur.execute("""
                SELECT COUNT(*) FROM traducciones 
                WHERE status = 'done' 
                AND created_at > NOW() - INTERVAL '5 minutes'
            """)
            recent_5min = cur.fetchone()[0]
            translations_per_min = round(recent_5min / 5, 1) if recent_5min else 0
            
            # Status counts
            cur.execute("SELECT COUNT(*) FROM traducciones WHERE status = 'done'")
            traducciones_count = cur.fetchone()[0]
            
            cur.execute("SELECT COUNT(*) FROM traducciones WHERE status = 'pending'")
            pending_count = cur.fetchone()[0]
            
            cur.execute("SELECT COUNT(*) FROM traducciones WHERE status = 'processing'")
            processing_count = cur.fetchone()[0]
            
            cur.execute("SELECT COUNT(*) FROM traducciones WHERE status = 'error'")
            error_count = cur.fetchone()[0]
            
            # Total noticias (exact count - cached for 5 min in view)
            cur.execute("SELECT COUNT(*) FROM noticias")
            noticias_count = cur.fetchone()[0] or 0
            
            # News ingested today
            cur.execute("""
                SELECT COUNT(*) FROM noticias 
                WHERE DATE(fecha) = CURRENT_DATE
            """)
            noticias_hoy = cur.fetchone()[0] or 0
            
            # News ingested in the last hour
            cur.execute("""
                SELECT COUNT(*) FROM noticias 
                WHERE fecha >= NOW() - INTERVAL '1 hour'
            """)
            noticias_ultima_hora = cur.fetchone()[0] or 0
            
    return render_template("stats.html",
                         translations_per_min=translations_per_min,
                         noticias_count=noticias_count,
                         traducciones_count=traducciones_count,
                         pending_count=pending_count,
                         processing_count=processing_count,
                         error_count=error_count,
                         noticias_hoy=noticias_hoy,
                         noticias_ultima_hora=noticias_ultima_hora)


@stats_bp.route("/api/activity")
@cached(ttl_seconds=300, prefix="stats")
def activity_data():
    """Get activity data (news count) for the specified range."""
    from flask import request
    range_param = request.args.get("range", "30d")
    
    # Default: 30d -> group by day
    days = 30
    minutes = 0
    interval_sql = "day" # For date_trunc or casting
    timedelta_step = timedelta(days=1)
    date_format = "%Y-%m-%d"
    
    if range_param == "1h":
        minutes = 60
        interval_sql = "minute"
        timedelta_step = timedelta(minutes=1)
        date_format = "%H:%M"
    elif range_param == "8h":
        minutes = 480
        interval_sql = "minute"
        timedelta_step = timedelta(minutes=1)
        date_format = "%H:%M"
    elif range_param == "1d": # Alias for 24h
        minutes = 1440
        interval_sql = "hour"
        timedelta_step = timedelta(hours=1)
        date_format = "%H:%M"
    elif range_param == "24h":
        minutes = 1440
        interval_sql = "hour"
        timedelta_step = timedelta(hours=1)
        date_format = "%H:%M"
    elif range_param == "7d":
        minutes = 10080
        interval_sql = "hour"
        timedelta_step = timedelta(hours=1)
        # Include Month-Day for 7d context
        date_format = "%d %H:%M"
    elif range_param == "30d":
        # Specific existing logic uses date casting, we can adapt
        minutes = 0 
        days = 30
        interval_sql = "day"
        timedelta_step = timedelta(days=1)
        date_format = "%Y-%m-%d"

    # Calculate start time
    if minutes > 0:
        start_time = datetime.utcnow() - timedelta(minutes=minutes)
        # Using timestamp column directly
        date_column = "fecha" 
    else:
        start_time = datetime.utcnow() - timedelta(days=days)
        # For 30d we might just use date part start
        start_time = start_time.replace(hour=0, minute=0, second=0, microsecond=0)
        date_column = "fecha"

    with get_read_conn() as conn:
        with conn.cursor() as cur:
            # Construct query based on interval
            if interval_sql == "day":
                 # Original logic style for 30d, but generalized
                cur.execute("""
                    SELECT 
                        fecha::date as time_slot,
                        COUNT(*) as count
                    FROM noticias
                    WHERE fecha >= %s
                    GROUP BY time_slot
                    ORDER BY time_slot
                """, (start_time,))
            else:
                # Granular logic
                cur.execute(f"""
                    SELECT 
                        date_trunc('{interval_sql}', fecha) as time_slot,
                        COUNT(*) as count
                    FROM noticias
                    WHERE fecha >= %s
                    GROUP BY time_slot
                    ORDER BY time_slot
                """, (start_time,))
            
            rows = cur.fetchall()
            
    # Fill gaps
    data_map = {row[0]: row[1] for row in rows}
    labels = []
    data = []
    
    # Iterate with step
    if minutes > 0:
        # Granular start alignment
        current = start_time.replace(second=0, microsecond=0)
        if interval_sql == "hour":
            current = current.replace(minute=0)
            
        end = datetime.utcnow().replace(second=0, microsecond=0)
        if interval_sql == "hour":
            end = end.replace(minute=0) + timedelta(hours=1)
    else:
        # Daily start alignment
        current = start_time.date() if isinstance(start_time, datetime) else start_time
        end = datetime.utcnow().date()
    
    while current <= end:
        # Format label
        labels.append(current.strftime(date_format))
        
        # Lookup key can be date or datetime depending on query
        # DB returns date for ::date and datetime for date_trunc
        # Let's handle both lookup types safely
        lookup_key = current 
        # API might have mismatch if current is date object and DB returned datetime or vice versa
        # rows[0] is date object for 'day', datetime for 'minute'/'hour'
        
        val = data_map.get(lookup_key, 0)
        # Fallback if types don't match exactly (datetime vs date) - unlikely if logic is consistent but good to check
        if val == 0 and isinstance(lookup_key, datetime) and interval_sql == 'day':
             val = data_map.get(lookup_key.date(), 0)

        data.append(val)
        
        current += timedelta_step
        
    return jsonify({
        "labels": labels,
        "data": data
    })


@stats_bp.route("/api/categories")
@cached(ttl_seconds=300, prefix="stats")
def categories_data():
    """Get news count per category (Top 8 + Others)."""
    with get_read_conn() as conn:
        with conn.cursor() as cur:
            cur.execute("""
                SELECT 
                    c.nombre,
                    COUNT(n.id) as count
                FROM noticias n
                JOIN categorias c ON c.id = n.categoria_id
                GROUP BY c.nombre
                ORDER BY count DESC
            """)
            
            rows = cur.fetchall()
            
    # Process Top 8 + Others
    labels = []
    data = []
    others_count = 0
    top_limit = 8
    
    for i, row in enumerate(rows):
        if i < top_limit:
            labels.append(row[0])
            data.append(row[1])
        else:
            others_count += row[1]
            
    if others_count > 0:
        labels.append("Otros")
        data.append(others_count)
            
    return jsonify({
        "labels": labels,
        "data": data
    })


@stats_bp.route("/api/countries")
@cached(ttl_seconds=300, prefix="stats")
def countries_data():
    """Get news count per country (Top 10 + Others)."""
    with get_read_conn() as conn:
        with conn.cursor() as cur:
            cur.execute("""
                SELECT 
                    p.nombre,
                    COUNT(n.id) as count
                FROM noticias n
                JOIN paises p ON p.id = n.pais_id
                GROUP BY p.nombre
                ORDER BY count DESC
            """)
            
            rows = cur.fetchall()

    # Process Top 10 + Others
    labels = []
    data = []
    others_count = 0
    top_limit = 10
    
    for i, row in enumerate(rows):
        if i < top_limit:
            labels.append(row[0])
            data.append(row[1])
        else:
            others_count += row[1]
            
    return jsonify({
        "labels": labels,
        "data": data
    })


@stats_bp.route("/api/countries/list")
def countries_list():
    """Get alphabetical list of all countries with flags."""
    from utils import country_flag
    with get_read_conn() as conn:
        with conn.cursor() as cur:
            cur.execute("SELECT nombre FROM paises ORDER BY nombre ASC")
            rows = cur.fetchall()
            
    return jsonify([
        {"name": row[0], "flag": country_flag(row[0])} 
        for row in rows
    ])


@stats_bp.route("/api/translations/activity")
def translations_activity_data():
    """Get translation count per day for the last 30 days."""
    days = 30
    start_date = (datetime.utcnow() - timedelta(days=days)).date()
    
    with get_read_conn() as conn:
        with conn.cursor() as cur:
            cur.execute("""
                SELECT 
                    created_at::date as day,
                    COUNT(*) as count
                FROM traducciones
                WHERE created_at >= %s
                GROUP BY day
                ORDER BY day
            """, (start_date,))
            
            rows = cur.fetchall()
            
    # Fill gaps
    data_map = {row[0]: row[1] for row in rows}
    labels = []
    data = []
    
    current = start_date
    end = datetime.utcnow().date()
    
    while current <= end:
        labels.append(current.strftime("%Y-%m-%d"))
        data.append(data_map.get(current, 0))
        current += timedelta(days=1)
        
    return jsonify({
        "labels": labels,
        "data": data
    })


@stats_bp.route("/api/translations/languages")
@cached(ttl_seconds=60, prefix="stats")
def translations_languages_data():
    """Get translation count per source language."""
    # Friendly names for common languages
    LANG_NAMES = {
        'en': 'Inglés',
        'es': 'Español',
        'fr': 'Francés',
        'de': 'Alemán',
        'it': 'Italiano',
        'pt': 'Portugués',
        'ru': 'Ruso',
        'zh': 'Chino',
        'ja': 'Japonés',
        'ar': 'Árabe'
    }
    
    with get_read_conn() as conn:
        with conn.cursor() as cur:
            cur.execute("""
                SELECT 
                    lang_from,
                    COUNT(*) as count
                FROM translation_stats
                WHERE lang_from IS NOT NULL
                GROUP BY lang_from
                ORDER BY count DESC
            """)
            
            rows = cur.fetchall()
            
    labels = []
    data = []
    for code, count in rows:
        code = code.strip().lower()
        labels.append(LANG_NAMES.get(code, code.upper()))
        data.append(count)
            
    return jsonify({
        "labels": labels,
        "data": data
    })

def get_system_uptime():
    try:
        with open('/proc/uptime', 'r') as f:
            uptime_seconds = float(f.readline().split()[0])
            days = int(uptime_seconds // (24 * 3600))
            hours = int((uptime_seconds % (24 * 3600)) // 3600)
            minutes = int((uptime_seconds % 3600) // 60)
            if days > 0:
                return f"{days}d {hours}h {minutes}m"
            return f"{hours}h {minutes}m"
    except:
        return "N/A"

def get_gpu_info():
    try:
        cmd = "nvidia-smi --query-gpu=name,temperature.gpu,utilization.gpu,memory.used,memory.total --format=csv,noheader,nounits"
        with open(os.devnull, 'w') as devnull:
            res = subprocess.check_output(cmd, shell=True, stderr=devnull).decode().strip()
        parts = [p.strip() for p in res.split(',')]
        if len(parts) >= 5:
            return {
                "name": parts[0],
                "temp": f"{parts[1]}°C",
                "util": f"{parts[2]}%",
                "mem": f"{parts[3]} MB / {parts[4]} MB"
            }
    except:
        pass
    return None

def get_cpu_info():
    try:
        load = os.getloadavg()
        cores = os.cpu_count()
        return {
            "load": f"{load[0]:.2f}, {load[1]:.2f}, {load[2]:.2f}",
            "cores": cores
        }
    except:
        return None

@stats_bp.route("/api/system/info")
def system_info_api():
    """Endpoint for real-time system monitoring."""
    return jsonify({
        "uptime": get_system_uptime(),
        "gpu": get_gpu_info(),
        "cpu": get_cpu_info(),
        "timestamp": datetime.now().strftime("%H:%M:%S")
    })


@stats_bp.route("/api/translations/rate")
@cached(ttl_seconds=60, prefix="stats")
def translations_rate_data():
    """Get translation count for the specified range (1h, 8h, 24h, 7d)."""
    # Parameters
    from flask import request
    
    range_param = request.args.get("range", "1h")
    
    # Default: 1h -> group by minute
    minutes = 60
    interval_sql = "minute"
    timedelta_step = timedelta(minutes=1)
    date_format = "%H:%M"
    
    if range_param == "8h":
        minutes = 8 * 60
        interval_sql = "minute" # Still group by minute for detailed graph? Or 5 mins?
        # Let's simple group by minute but it might be dense. 480 points. Fine.
        timedelta_step = timedelta(minutes=1) 
        date_format = "%H:%M"
        
    elif range_param == "24h":
        minutes = 24 * 60
        # Group by 15 minutes? Postgres: date_trunc('hour', ...) or extract?
        # Let's use custom grouping? Or simple 'hour' is too granular? 1440 mins.
        # Let's group by hour for 24h to be safe/clean
        interval_sql = "hour"
        timedelta_step = timedelta(hours=1)
        date_format = "%H:%M"
        
    elif range_param == "7d":
        minutes = 7 * 24 * 60
        interval_sql = "hour" # 7 * 24 = 168 points
        timedelta_step = timedelta(hours=1)
        date_format = "%Y-%m-%d %H:%M"
        
    start_time = datetime.utcnow() - timedelta(minutes=minutes)
    
    with get_read_conn() as conn:
        with conn.cursor() as cur:
            # Query translation_stats instead of traducciones
            cur.execute(f"""
                SELECT 
                    date_trunc('{interval_sql}', created_at) as time_slot,
                    COUNT(*) as count
                FROM translation_stats
                WHERE created_at >= %s
                GROUP BY time_slot
                ORDER BY time_slot
            """, (start_time,))
            
            rows = cur.fetchall()
            
    # Fill gaps
    data_map = {row[0]: row[1] for row in rows}
    labels = []
    data = []
    
    # Iterate by step
    # Align start_time to step if possible (lazy alignment)
    current = start_time.replace(second=0, microsecond=0)
    if interval_sql == "hour":
        current = current.replace(minute=0)
        
    end = datetime.utcnow().replace(second=0, microsecond=0)
    if interval_sql == "hour":
        end = end.replace(minute=0) + timedelta(hours=1) # Ensure we cover current partial hour
    
    while current <= end:
        labels.append(current.strftime(date_format))
        data.append(data_map.get(current, 0))
        current += timedelta_step
        
    return jsonify({
        "labels": labels,
        "data": data
    })


@stats_bp.route("/entities")
def entities_dashboard():
    """Dashboard for Named Entities statistics."""
    return render_template("stats_entities.html")


@stats_bp.route("/api/entities/people")
def entities_people():
    """Top 25 mentioned people, optionally filtered by country and/or date."""
    from flask import request
    from datetime import datetime
    from cache import cache_get, cache_set
    
    # 1. Check config mtime for cache invalidation
    try:
        config_mtime = os.path.getmtime(CONFIG_FILE)
    except OSError:
        config_mtime = 0
        
    country_filter = request.args.get("country")
    date_filter = request.args.get("date")
    
    # 2. Build cache key with mtime
    cache_key = f"entities:people:{country_filter}:{date_filter}:{config_mtime}"
    
    # 3. Try cache
    cached_data = cache_get(cache_key)
    if cached_data:
        return jsonify(cached_data)
    
    # Determine time range
    if date_filter:
        # Single day query
        try:
            target_date = datetime.strptime(date_filter, "%Y-%m-%d").date()
            time_condition = "DATE(tr.created_at) = %s"
            time_params = [target_date]
        except ValueError:
            # Invalid date format, fallback to 30 days
            time_condition = "tr.created_at >= NOW() - INTERVAL '30 days'"
            time_params = []
    else:
        # Default: last 30 days
        time_condition = "tr.created_at >= NOW() - INTERVAL '30 days'"
        time_params = []
    
    if country_filter and country_filter != 'global':
        # Filtered by country
        query = f"""
            SELECT t.valor, COUNT(*) as menciones
            FROM tags t
            JOIN tags_noticia tn ON tn.tag_id = t.id
            JOIN traducciones tr ON tn.traduccion_id = tr.id
            JOIN noticias n ON tr.noticia_id = n.id
            WHERE t.tipo = 'persona'
              AND {time_condition}
              AND n.pais_id = (SELECT id FROM paises WHERE nombre ILIKE %s LIMIT 1)
            GROUP BY t.valor
            ORDER BY menciones DESC
        """
        params = tuple(time_params + [country_filter])
    else:
        # Global view
        query = f"""
            SELECT t.valor, COUNT(*) as menciones
            FROM tags t
            JOIN tags_noticia tn ON tn.tag_id = t.id
            JOIN traducciones tr ON tn.traduccion_id = tr.id
            WHERE t.tipo = 'persona'
              AND {time_condition}
            GROUP BY t.valor
            ORDER BY menciones DESC
        """
        params = tuple(time_params)
    
    with get_read_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(query, params)
            rows = cur.fetchall()
    
    # Normalize and aggregate
    normalized_rows = aggregate_normalized_entities(rows, entity_type='persona')
    
    # Take top 50
    top_50 = normalized_rows[:50]
    
    # Enrich with Wikipedia Images (Parallel Execution)
    from concurrent.futures import ThreadPoolExecutor
    from utils.wiki import fetch_wiki_data

    images = []
    summaries = []
    
    def get_image_safe(name):
        try:
            return fetch_wiki_data(name)
        except Exception:
            return None, None

    if top_50:
        names = [row[0] for row in top_50]
        with ThreadPoolExecutor(max_workers=10) as executor:
            try:
                results = list(executor.map(get_image_safe, names))
                
                # Unpack results
                for img, smry in results:
                    images.append(img)
                    summaries.append(smry)
            except Exception as e:
                import logging
                logging.error(f"Error fetching wiki data: {e}")
                # Fallback to empty if threading fails
                images = [None] * len(names)
                summaries = [None] * len(names)
    else:
        images = []
        summaries = []

    result = {
        "labels": [row[0] for row in top_50],
        "data": [row[1] for row in top_50],
        "images": images,
        "summaries": summaries
    }
    
    # 4. Set cache
    cache_set(cache_key, result, ttl_seconds=600)
    
    return jsonify(result)


@stats_bp.route("/api/entities/orgs")
def entities_orgs():
    """Top mentioned organizations, optionally filtered by country."""
    from flask import request
    from cache import cache_get, cache_set
    
    country_filter = request.args.get("country")
    
    try:
        config_mtime = os.path.getmtime(CONFIG_FILE)
    except OSError:
        config_mtime = 0
        
    cache_key = f"entities:orgs:{country_filter}:{config_mtime}"
    
    cached_data = cache_get(cache_key)
    if cached_data:
        return jsonify(cached_data)

    if country_filter and country_filter != 'global':
        query = """
            SELECT t.valor, COUNT(*) as menciones
            FROM tags t
            JOIN tags_noticia tn ON tn.tag_id = t.id
            JOIN traducciones tr ON tn.traduccion_id = tr.id
            JOIN noticias n ON tr.noticia_id = n.id
            WHERE t.tipo = 'organizacion'
              AND tr.created_at >= NOW() - INTERVAL '30 days'
              AND n.pais_id = (SELECT id FROM paises WHERE nombre ILIKE %s LIMIT 1)
            GROUP BY t.valor
            ORDER BY menciones DESC
            LIMIT 50
        """
        params = (country_filter,)
    else:
        query = """
            SELECT t.valor, COUNT(*) as menciones
            FROM tags t
            JOIN tags_noticia tn ON tn.tag_id = t.id
            JOIN traducciones tr ON tn.traduccion_id = tr.id
            WHERE t.tipo = 'organizacion'
              AND tr.created_at >= NOW() - INTERVAL '30 days'
            GROUP BY t.valor
            ORDER BY menciones DESC
            LIMIT 50
        """
        params = ()

    with get_read_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(query, params)
            rows = cur.fetchall()
    
    normalized_rows = aggregate_normalized_entities(rows, entity_type='organizacion')
    
    # Enrich with Wikipedia Images
    from concurrent.futures import ThreadPoolExecutor
    from utils.wiki import fetch_wiki_data

    images = []
    summaries = []
    
    def get_info_safe(name):
        try:
            return fetch_wiki_data(name)
        except Exception:
            return None, None

    if normalized_rows:
        names = [row[0] for row in normalized_rows]
        with ThreadPoolExecutor(max_workers=10) as executor:
            results = list(executor.map(get_info_safe, names))
            for img, smry in results:
                images.append(img)
                summaries.append(smry)

    result = {
        "labels": [row[0] for row in normalized_rows],
        "data": [row[1] for row in normalized_rows],
        "images": images,
        "summaries": summaries
    }
    
    cache_set(cache_key, result, ttl_seconds=600)
    return jsonify(result)


@stats_bp.route("/api/entities/places")
def entities_places():
    """Top mentioned places, optionally filtered by country."""
    from flask import request
    from cache import cache_get, cache_set
    
    country_filter = request.args.get("country")
    
    try:
        config_mtime = os.path.getmtime(CONFIG_FILE)
    except OSError:
        config_mtime = 0
        
    cache_key = f"entities:places:{country_filter}:{config_mtime}"
    
    cached_data = cache_get(cache_key)
    if cached_data:
        return jsonify(cached_data)

    if country_filter and country_filter != 'global':
        query = """
            SELECT t.valor, COUNT(*) as menciones
            FROM tags t
            JOIN tags_noticia tn ON tn.tag_id = t.id
            JOIN traducciones tr ON tn.traduccion_id = tr.id
            JOIN noticias n ON tr.noticia_id = n.id
            WHERE t.tipo = 'lugar'
              AND tr.created_at >= NOW() - INTERVAL '30 days'
              AND n.pais_id = (SELECT id FROM paises WHERE nombre ILIKE %s LIMIT 1)
              AND n.pais_id != (SELECT id FROM paises WHERE nombre = 'España')
            GROUP BY t.valor
            ORDER BY menciones DESC
            LIMIT 50
        """
        params = (country_filter,)
    else:
        query = """
            SELECT t.valor, COUNT(*) as menciones
            FROM tags t
            JOIN tags_noticia tn ON tn.tag_id = t.id
            JOIN traducciones tr ON tn.traduccion_id = tr.id
            JOIN noticias n ON tr.noticia_id = n.id
            WHERE t.tipo = 'lugar'
              AND tr.created_at >= NOW() - INTERVAL '30 days'
              AND n.pais_id != (SELECT id FROM paises WHERE nombre = 'España')
            GROUP BY t.valor
            ORDER BY menciones DESC
            LIMIT 50
        """
        params = ()

    with get_read_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(query, params)
            rows = cur.fetchall()
            
    # Normalize
    normalized_rows = aggregate_normalized_entities(rows, entity_type='lugar')
            
    # Enrich with Wikipedia Images
    from concurrent.futures import ThreadPoolExecutor
    from utils.wiki import fetch_wiki_data

    images = []
    summaries = []
    
    def get_info_safe(name):
        try:
            return fetch_wiki_data(name)
        except Exception:
            return None, None

    if normalized_rows:
        names = [row[0] for row in normalized_rows]
        with ThreadPoolExecutor(max_workers=10) as executor:
            results = list(executor.map(get_info_safe, names))
            for img, smry in results:
                images.append(img)
                summaries.append(smry)

    result = {
        "labels": [row[0] for row in normalized_rows],
        "data": [row[1] for row in normalized_rows],
        "images": images,
        "summaries": summaries
    }
    
    cache_set(cache_key, result, ttl_seconds=600)
    return jsonify(result)
