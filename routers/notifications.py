"""
Notifications router - Check for new important news.
"""
from flask import Blueprint, jsonify, request
from db import get_conn
from datetime import datetime

notifications_bp = Blueprint("notifications", __name__, url_prefix="/api/notifications")

@notifications_bp.route("/check")
def check_notifications():
    """Check for new news since a given timestamp."""
    last_check = request.args.get("last_check")
    
    if not last_check:
        return jsonify({"has_news": False, "timestamp": datetime.utcnow().isoformat()})
    
    try:
        # Check for news created after last_check
        # We define "important" as having translation or high score (if score existed)
        # For now, just any new news to demonstrate functionality
        with get_conn() as conn:
            with conn.cursor() as cur:
                cur.execute("""
                    SELECT COUNT(*), MAX(fecha)
                    FROM noticias 
                    WHERE fecha > %s
                """, (last_check,))
                row = cur.fetchone()
                count = row[0]
                latest = row[1]
                
        if count > 0:
            return jsonify({
                "has_news": True,
                "count": count,
                "timestamp": latest.isoformat() if latest else datetime.utcnow().isoformat(),
                "message": f"¡{count} noticias nuevas encontradas!"
            })
            
    except Exception as e:
        print(f"Error checking notifications: {e}")
        
    return jsonify({"has_news": False, "timestamp": datetime.utcnow().isoformat()})
