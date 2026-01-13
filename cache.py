"""
Redis cache module for high-traffic endpoints.
Provides caching decorator and invalidation utilities.
"""
import redis
import json
import logging
import hashlib
from functools import wraps
from config import REDIS_HOST, REDIS_PORT, REDIS_PASSWORD, REDIS_TTL_DEFAULT

logger = logging.getLogger(__name__)

_redis_client = None


def get_redis():
    """Get Redis client singleton."""
    global _redis_client
    if _redis_client is None:
        try:
            redis_config = {
                'host': REDIS_HOST,
                'port': REDIS_PORT,
                'decode_responses': True,
                'socket_connect_timeout': 2,
                'socket_timeout': 2
            }
            
            # Agregar autenticación si está configurada
            if REDIS_PASSWORD:
                redis_config['password'] = REDIS_PASSWORD
            
            _redis_client = redis.Redis(**redis_config)
            _redis_client.ping()
        except redis.ConnectionError as e:
            logger.warning(f"Redis connection failed: {e}. Caching disabled.")
            _redis_client = None
    return _redis_client


def cached(ttl_seconds=None, prefix="cache"):
    """
    Decorator for caching function results in Redis.
    Falls back to calling function directly if Redis is unavailable.
    
    Args:
        ttl_seconds: Time to live in seconds (default from config)
        prefix: Key prefix for cache entries
    """
    if ttl_seconds is None:
        ttl_seconds = REDIS_TTL_DEFAULT
    
    def decorator(func):
        @wraps(func)
        def wrapper(*args, **kwargs):
            r = get_redis()
            if r is None:
                # Redis unavailable, call function directly
                return func(*args, **kwargs)
            
            # Build cache key from function name and arguments
            # Use md5 for deterministic hash across processes
            key_data = f"{args}:{sorted(kwargs.items())}"
            
            # Add flask request args if available to prevent collision on filtered routes
            try:
                from flask import request
                if request:
                    key_data += f":args:{sorted(request.args.items())}"
            except Exception:
                pass

            key_hash = hashlib.md5(key_data.encode('utf-8')).hexdigest()
            cache_key = f"cache:{prefix}:{func.__name__}:{key_hash}"
            
            try:
                # Try to get from cache
                cached_value = r.get(cache_key)
                if cached_value is not None:
                    # If it's a JSON response, we might need to return it correctly
                    try:
                        data = json.loads(cached_value)
                        # Detect if we should return as JSON
                        from flask import jsonify
                        return jsonify(data)
                    except (json.JSONDecodeError, ImportError):
                        return cached_value
                
                # Cache miss - call function and cache result
                result = func(*args, **kwargs)
                
                # Handle Flask Response objects
                cache_data = result
                try:
                    from flask import Response
                    if isinstance(result, Response):
                        if result.is_json:
                            cache_data = result.get_json()
                        else:
                            cache_data = result.get_data(as_text=True)
                except (ImportError, Exception):
                    pass

                r.setex(cache_key, ttl_seconds, json.dumps(cache_data, default=str))
                return result
            except (redis.RedisError, json.JSONDecodeError) as e:
                logger.warning(f"Cache error for {func.__name__}: {e}")
                return func(*args, **kwargs)
        
        return wrapper
    return decorator


def invalidate_pattern(pattern):
    """
    Invalidate all cache keys matching pattern.
    
    Args:
        pattern: Pattern to match (e.g., "home:*" or "stats:*")
    """
    r = get_redis()
    if r is None:
        return
    
    try:
        cursor = 0
        deleted = 0
        while True:
            cursor, keys = r.scan(cursor, match=f"cache:{pattern}", count=100)
            if keys:
                r.delete(*keys)
                deleted += len(keys)
            if cursor == 0:
                break
        if deleted > 0:
            logger.info(f"Invalidated {deleted} cache keys matching '{pattern}'")
    except redis.RedisError as e:
        logger.warning(f"Cache invalidation failed: {e}")


def cache_get(key):
    """Get value from cache by key."""
    r = get_redis()
    if r is None:
        return None
    try:
        value = r.get(f"cache:{key}")
        return json.loads(value) if value else None
    except (redis.RedisError, json.JSONDecodeError):
        return None


def cache_set(key, value, ttl_seconds=None):
    """Set value in cache with optional TTL."""
    if ttl_seconds is None:
        ttl_seconds = REDIS_TTL_DEFAULT
    r = get_redis()
    if r is None:
        return False
    try:
        r.setex(f"cache:{key}", ttl_seconds, json.dumps(value, default=str))
        return True
    except redis.RedisError:
        return False


def cache_del(key):
    """Delete a key from cache."""
    r = get_redis()
    if r is None:
        return False
    try:
        r.delete(f"cache:{key}")
        return True
    except redis.RedisError:
        return False
