"""
Authentication router - User registration, login, and logout.
"""
from flask import Blueprint, request, render_template, redirect, url_for, session, flash
from psycopg2 import extras, IntegrityError
from db import get_conn
from utils.auth import (
    hash_password, verify_password, is_authenticated,
    validate_username, validate_password, validate_email
)
from datetime import datetime

auth_bp = Blueprint("auth", __name__, url_prefix="/auth")


def migrate_anonymous_favorites(session_id: str, user_id: int):
    """Migrate anonymous favorites to user account.
    
    Args:
        session_id: Anonymous session ID
        user_id: User ID to migrate favorites to
    """
    if not session_id:
        return
    
    with get_conn() as conn:
        with conn.cursor() as cur:
            # Migrate favorites, avoiding duplicates
            cur.execute("""
                UPDATE favoritos 
                SET user_id = %s, session_id = NULL
                WHERE session_id = %s 
                  AND noticia_id NOT IN (
                    SELECT noticia_id FROM favoritos WHERE user_id = %s
                  )
            """, (user_id, session_id, user_id))
            
            # Delete any remaining duplicates
            cur.execute("""
                DELETE FROM favoritos 
                WHERE session_id = %s
            """, (session_id,))
        conn.commit()


# ============================================================
#   Registration
# ============================================================

@auth_bp.route("/register", methods=["GET", "POST"])
def register():
    """User registration page and handler."""
    if is_authenticated():
        return redirect(url_for('account.index'))
    
    if request.method == "POST":
        username = request.form.get("username", "").strip()
        email = request.form.get("email", "").strip().lower()
        password = request.form.get("password", "")
        password_confirm = request.form.get("password_confirm", "")
        
        # Validation
        valid_username, username_error = validate_username(username)
        if not valid_username:
            flash(username_error, "danger")
            return render_template("register.html", username=username, email=email)
        
        valid_email, email_error = validate_email(email)
        if not valid_email:
            flash(email_error, "danger")
            return render_template("register.html", username=username, email=email)
        
        valid_password, password_error = validate_password(password)
        if not valid_password:
            flash(password_error, "danger")
            return render_template("register.html", username=username, email=email)
        
        if password != password_confirm:
            flash("Las contraseñas no coinciden", "danger")
            return render_template("register.html", username=username, email=email)
        
        # Create user
        try:
            password_hash = hash_password(password)
            
            with get_conn() as conn:
                with conn.cursor() as cur:
                    cur.execute("""
                        INSERT INTO usuarios (username, email, password_hash, last_login)
                        VALUES (%s, %s, %s, NOW())
                        RETURNING id
                    """, (username, email, password_hash))
                    user_id = cur.fetchone()[0]
                conn.commit()
            
            # Auto-login after registration
            old_session_id = session.get('user_session')
            session['user_id'] = user_id
            session['username'] = username
            
            # Migrate anonymous favorites if any
            if old_session_id:
                migrate_anonymous_favorites(old_session_id, user_id)
                session.pop('user_session', None)
            
            flash(f"¡Bienvenido {username}! Tu cuenta ha sido creada exitosamente.", "success")
            return redirect(url_for('account.index'))
            
        except IntegrityError as e:
            if 'username' in str(e):
                flash("Este nombre de usuario ya está en uso", "danger")
            elif 'email' in str(e):
                flash("Este email ya está registrado", "danger")
            else:
                flash("Error al crear la cuenta. Por favor intenta de nuevo.", "danger")
            return render_template("register.html", username=username, email=email)
        except Exception as e:
            flash("Error al crear la cuenta. Por favor intenta de nuevo.", "danger")
            return render_template("register.html", username=username, email=email)
    
    return render_template("register.html")


# ============================================================
#   Login
# ============================================================

@auth_bp.route("/login", methods=["GET", "POST"])
def login():
    """User login page and handler."""
    if is_authenticated():
        return redirect(url_for('account.index'))
    
    if request.method == "POST":
        username_or_email = request.form.get("username", "").strip()
        password = request.form.get("password", "")
        
        if not username_or_email or not password:
            flash("Por favor ingresa tu usuario/email y contraseña", "danger")
            return render_template("login.html", username=username_or_email)
        
        try:
            with get_conn() as conn:
                with conn.cursor(cursor_factory=extras.DictCursor) as cur:
                    # Try login with username or email
                    cur.execute("""
                        SELECT id, username, email, password_hash, is_active, avatar_url
                        FROM usuarios
                        WHERE (username = %s OR email = %s) AND is_active = TRUE
                    """, (username_or_email, username_or_email.lower()))
                    user = cur.fetchone()
                    
                    if not user:
                        flash("Usuario o contraseña incorrectos", "danger")
                        return render_template("login.html", username=username_or_email)
                    
                    if not verify_password(password, user['password_hash']):
                        flash("Usuario o contraseña incorrectos", "danger")
                        return render_template("login.html", username=username_or_email)
                    
                    # Update last login
                    cur.execute("""
                        UPDATE usuarios SET last_login = NOW() WHERE id = %s
                    """, (user['id'],))
                conn.commit()
            
            # Create session
            old_session_id = session.get('user_session')
            session['user_id'] = user['id']
            session['username'] = user['username']
            session['avatar_url'] = user.get('avatar_url')
            
            # Migrate anonymous favorites
            if old_session_id:
                migrate_anonymous_favorites(old_session_id, user['id'])
                session.pop('user_session', None)
            
            flash(f"¡Bienvenido de vuelta, {user['username']}!", "success")
            
            # Redirect to 'next' parameter if exists
            next_page = request.args.get('next')
            if next_page and next_page.startswith('/'):
                return redirect(next_page)
            return redirect(url_for('account.index'))
            
        except Exception as e:
            flash("Error al iniciar sesión. Por favor intenta de nuevo.", "danger")
            return render_template("login.html", username=username_or_email)
    
    return render_template("login.html")


# ============================================================
#   Logout
# ============================================================

@auth_bp.route("/logout", methods=["POST", "GET"])
def logout():
    """Log out the current user."""
    username = session.get('username', 'Usuario')
    session.clear()
    flash(f"Hasta luego, {username}. Has cerrado sesión exitosamente.", "info")
    return redirect(url_for('home.index'))
