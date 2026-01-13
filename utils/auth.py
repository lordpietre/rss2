"""
Authentication utilities for user management.
Provides password hashing, verification, and authentication decorators.
"""
import bcrypt
from functools import wraps
from flask import session, redirect, url_for, flash
from db import get_conn
from psycopg2 import extras


def hash_password(password: str) -> str:
    """Hash a password using bcrypt.
    
    Args:
        password: Plain text password
        
    Returns:
        Hashed password string
    """
    salt = bcrypt.gensalt(rounds=12)
    return bcrypt.hashpw(password.encode('utf-8'), salt).decode('utf-8')


def verify_password(password: str, password_hash: str) -> bool:
    """Verify a password against its hash.
    
    Args:
        password: Plain text password to verify
        password_hash: Bcrypt hash to check against
        
    Returns:
        True if password matches, False otherwise
    """
    try:
        return bcrypt.checkpw(password.encode('utf-8'), password_hash.encode('utf-8'))
    except Exception:
        return False


def get_current_user():
    """Get the currently authenticated user from session.
    
    Returns:
        User dict with id, username, email, etc. or None if not authenticated
    """
    user_id = session.get('user_id')
    if not user_id:
        return None
    
    try:
        with get_conn() as conn:
            with conn.cursor(cursor_factory=extras.DictCursor) as cur:
                cur.execute("""
                    SELECT id, username, email, created_at, last_login, is_active, avatar_url
                    FROM usuarios
                    WHERE id = %s AND is_active = TRUE
                """, (user_id,))
                user = cur.fetchone()
                return dict(user) if user else None
    except Exception:
        return None


def is_authenticated() -> bool:
    """Check if current user is authenticated.
    
    Returns:
        True if user is logged in, False otherwise
    """
    return 'user_id' in session and session.get('user_id') is not None


def login_required(f):
    """Decorator to require authentication for a route.
    
    Usage:
        @app.route('/protected')
        @login_required
        def protected_route():
            return "You can only see this if logged in"
    """
    @wraps(f)
    def decorated_function(*args, **kwargs):
        if not is_authenticated():
            flash('Por favor inicia sesión para acceder a esta página.', 'warning')
            return redirect(url_for('auth.login', next=request.url))
        return f(*args, **kwargs)
    return decorated_function


def validate_username(username: str) -> tuple[bool, str]:
    """Validate username format.
    
    Args:
        username: Username to validate
        
    Returns:
        Tuple of (is_valid, error_message)
    """
    if not username or len(username) < 3:
        return False, "El nombre de usuario debe tener al menos 3 caracteres"
    if len(username) > 50:
        return False, "El nombre de usuario no puede tener más de 50 caracteres"
    if not username.replace('_', '').replace('-', '').isalnum():
        return False, "El nombre de usuario solo puede contener letras, números, guiones y guiones bajos"
    return True, ""


def validate_password(password: str) -> tuple[bool, str]:
    """Validate password strength.
    
    Args:
        password: Password to validate
        
    Returns:
        Tuple of (is_valid, error_message)
    """
    if not password or len(password) < 6:
        return False, "La contraseña debe tener al menos 6 caracteres"
    if len(password) > 128:
        return False, "La contraseña no puede tener más de 128 caracteres"
    return True, ""


def validate_email(email: str) -> tuple[bool, str]:
    """Validate email format.
    
    Args:
        email: Email to validate
        
    Returns:
        Tuple of (is_valid, error_message)
    """
    try:
        from email_validator import validate_email as validate_email_lib, EmailNotValidError
        validate_email_lib(email)
        return True, ""
    except EmailNotValidError as e:
        return False, f"Email inválido: {str(e)}"
    except ImportError:
        # Fallback to basic regex if email-validator not available
        import re
        if re.match(r'^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$', email):
            return True, ""
        return False, "Email inválido"
