"""
Account management router - User profile and account settings.
"""
from flask import Blueprint, render_template, request, redirect, url_for, flash, jsonify
from psycopg2 import extras
from db import get_conn
from utils.auth import get_current_user, login_required, hash_password, verify_password, validate_password
from datetime import datetime

account_bp = Blueprint("account", __name__, url_prefix="/account")


@account_bp.route("/")
@login_required
def index():
    """User account dashboard."""
    user = get_current_user()
    if not user:
        return redirect(url_for('auth.login'))
    
    with get_conn() as conn:
        with conn.cursor(cursor_factory=extras.DictCursor) as cur:
            # Get favorites count
            cur.execute("""
                SELECT COUNT(*) as count 
                FROM favoritos 
                WHERE user_id = %s
            """, (user['id'],))
            favorites_count = cur.fetchone()['count']
            
            # Get search history count
            cur.execute("""
                SELECT COUNT(*) as count 
                FROM search_history 
                WHERE user_id = %s
            """, (user['id'],))
            searches_count = cur.fetchone()['count']
            
            # Get recent searches (last 10)
            cur.execute("""
                SELECT query, results_count, searched_at
                FROM search_history
                WHERE user_id = %s
                ORDER BY searched_at DESC
                LIMIT 10
            """, (user['id'],))
            recent_searches = cur.fetchall()
            
            # Get recent favorites (last 5)
            cur.execute("""
                SELECT n.id, n.titulo, n.imagen_url, f.created_at,
                       t.titulo_trad, t.id AS traduccion_id
                FROM favoritos f
                JOIN noticias n ON n.id = f.noticia_id
                LEFT JOIN traducciones t ON t.noticia_id = n.id AND t.lang_to = 'es' AND t.status = 'done'
                WHERE f.user_id = %s
                ORDER BY f.created_at DESC
                LIMIT 5
            """, (user['id'],))
            recent_favorites = cur.fetchall()
    
    return render_template("account.html",
                          user=user,
                          favorites_count=favorites_count,
                          searches_count=searches_count,
                          recent_searches=recent_searches,
                          recent_favorites=recent_favorites)


@account_bp.route("/search-history")
@login_required
def search_history():
    """Full search history page."""
    user = get_current_user()
    if not user:
        return redirect(url_for('auth.login'))
    
    page = max(1, int(request.args.get('page', 1)))
    per_page = 50
    offset = (page - 1) * per_page
    
    with get_conn() as conn:
        with conn.cursor(cursor_factory=extras.DictCursor) as cur:
            # Get total count
            cur.execute("""
                SELECT COUNT(*) as count 
                FROM search_history 
                WHERE user_id = %s
            """, (user['id'],))
            total = cur.fetchone()['count']
            
            # Get paginated results
            cur.execute("""
                SELECT query, results_count, searched_at
                FROM search_history
                WHERE user_id = %s
                ORDER BY searched_at DESC
                LIMIT %s OFFSET %s
            """, (user['id'], per_page, offset))
            searches = cur.fetchall()
    
    total_pages = (total + per_page - 1) // per_page
    
    return render_template("search_history.html",
                          user=user,
                          searches=searches,
                          page=page,
                          total_pages=total_pages,
                          total=total)


@account_bp.route("/change-password", methods=["POST"])
@login_required
def change_password():
    """Change user password."""
    user = get_current_user()
    if not user:
        return redirect(url_for('auth.login'))
    
    current_password = request.form.get("current_password", "")
    new_password = request.form.get("new_password", "")
    new_password_confirm = request.form.get("new_password_confirm", "")
    
    # Validation
    if not current_password or not new_password:
        flash("Por favor completa todos los campos", "danger")
        return redirect(url_for('account.index'))
    
    valid_password, password_error = validate_password(new_password)
    if not valid_password:
        flash(password_error, "danger")
        return redirect(url_for('account.index'))
    
    if new_password != new_password_confirm:
        flash("Las contraseñas nuevas no coinciden", "danger")
        return redirect(url_for('account.index'))
    
    try:
        with get_conn() as conn:
            with conn.cursor(cursor_factory=extras.DictCursor) as cur:
                # Verify current password
                cur.execute("""
                    SELECT password_hash
                    FROM usuarios
                    WHERE id = %s
                """, (user['id'],))
                result = cur.fetchone()
                
                if not result or not verify_password(current_password, result['password_hash']):
                    flash("La contraseña actual es incorrecta", "danger")
                    return redirect(url_for('account.index'))
                
                # Update password
                new_hash = hash_password(new_password)
                cur.execute("""
                    UPDATE usuarios 
                    SET password_hash = %s, updated_at = NOW()
                    WHERE id = %s
                """, (new_hash, user['id']))
            conn.commit()
        
        flash("Contraseña actualizada exitosamente", "success")
    except Exception as e:
        flash("Error al actualizar la contraseña", "danger")
    
    return redirect(url_for('account.index'))


@account_bp.route("/upload-avatar", methods=["POST"])
@login_required
def upload_avatar():
    """Upload user avatar."""
    import os
    import secrets
    from werkzeug.utils import secure_filename
    from flask import current_app
    
    user = get_current_user()
    if not user:
        return redirect(url_for('auth.login'))
    
    if 'avatar' not in request.files:
        flash("No se seleccionó ningún archivo", "danger")
        return redirect(url_for('account.index'))
    
    file = request.files['avatar']
    if file.filename == '':
        flash("No se seleccionó ningún archivo", "danger")
        return redirect(url_for('account.index'))
    
    if file:
        # Check extension
        allowed_extensions = {'.png', '.jpg', '.jpeg', '.gif', '.webp'}
        _, ext = os.path.splitext(file.filename)
        if ext.lower() not in allowed_extensions:
            flash("Formato de imagen no permitido. Usa JPG, PNG, GIF o WEBP.", "danger")
            return redirect(url_for('account.index'))
        
        # Save file
        try:
            # Create filename using user ID and random partial to avoid caching issues
            random_hex = secrets.token_hex(4)
            filename = f"user_{user['id']}_{random_hex}{ext.lower()}"
            
            # Ensure upload folder exists
            upload_folder = os.path.join(current_app.root_path, 'static/uploads/avatars')
            os.makedirs(upload_folder, exist_ok=True)
            
            # Delete old avatar if exists
            if user.get('avatar_url'):
                old_path = os.path.join(current_app.root_path, user['avatar_url'].lstrip('/'))
                if os.path.exists(old_path) and 'user_' in old_path: # Safety check
                    try:
                        os.remove(old_path)
                    except:
                        pass
            
            file_path = os.path.join(upload_folder, filename)
            file.save(file_path)
            
            # Update DB
            relative_path = f"/static/uploads/avatars/{filename}"
            with get_conn() as conn:
                with conn.cursor() as cur:
                    cur.execute("""
                        UPDATE usuarios 
                        SET avatar_url = %s, updated_at = NOW()
                        WHERE id = %s
                    """, (relative_path, user['id']))
                conn.commit()
            
            # Update session
            from flask import session
            session['avatar_url'] = relative_path
            
            flash("Foto de perfil actualizada", "success")
            
        except Exception as e:
            print(f"Error uploading avatar: {e}")
            flash("Error al subir la imagen", "danger")
            
    return redirect(url_for('account.index'))


@account_bp.route("/stats")
@login_required
def stats():
    """Get user statistics as JSON."""
    user = get_current_user()
    if not user:
        return jsonify({"error": "Not authenticated"}), 401
    
    with get_conn() as conn:
        with conn.cursor(cursor_factory=extras.DictCursor) as cur:
            cur.execute("""
                SELECT 
                    (SELECT COUNT(*) FROM favoritos WHERE user_id = %s) as favorites_count,
                    (SELECT COUNT(*) FROM search_history WHERE user_id = %s) as searches_count,
                    (SELECT MAX(searched_at) FROM search_history WHERE user_id = %s) as last_search
            """, (user['id'], user['id'], user['id']))
            stats = cur.fetchone()
    
    return jsonify({
        "favorites_count": stats['favorites_count'],
        "searches_count": stats['searches_count'],
        "last_search": stats['last_search'].isoformat() if stats['last_search'] else None
    })
