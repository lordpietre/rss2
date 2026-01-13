import os
import psycopg2
from contextlib import contextmanager

# Database configuration
DB_HOST = os.environ.get("DB_HOST", "db")
DB_NAME = os.environ.get("DB_NAME", "rss")
DB_USER = os.environ.get("DB_USER", "rss")
DB_PASS = os.environ.get("DB_PASS", "x")
DB_PORT = os.environ.get("DB_PORT", "5432")
DB_READ_HOST = os.environ.get("DB_READ_HOST", "db-replica")
DB_WRITE_HOST = os.environ.get("DB_WRITE_HOST", "db")

@contextmanager
def get_conn():
    """Get a database connection (Default: Primary/Write)."""
    conn = None
    try:
        conn = psycopg2.connect(
            host=DB_HOST,
            database=DB_NAME,
            user=DB_USER,
            password=DB_PASS,
            port=DB_PORT
        )
        yield conn
    finally:
        if conn:
            conn.close()

@contextmanager
def get_read_conn():
    """Get a read-only database connection (Replica)."""
    conn = None
    try:
        try:
            # Attempt to connect to Replica first
            conn = psycopg2.connect(
                host=DB_READ_HOST,
                database=DB_NAME,
                user=DB_USER,
                password=DB_PASS,
                port=DB_PORT,
                connect_timeout=5
            )
        except (psycopg2.OperationalError, psycopg2.InterfaceError) as e:
            # Fallback to Primary if Replica is down on initial connection
            print(f"Warning: Replica unreachable ({e}), falling back to Primary for read.")
            conn = psycopg2.connect(
                host=DB_WRITE_HOST,
                database=DB_NAME,
                user=DB_USER,
                password=DB_PASS,
                port=DB_PORT
            )
        yield conn
    finally:
        if conn:
            conn.close()

@contextmanager
def get_write_conn():
    """Get a write database connection (Primary)."""
    conn = None
    try:
        conn = psycopg2.connect(
            host=DB_WRITE_HOST,
            database=DB_NAME,
            user=DB_USER,
            password=DB_PASS,
            port=DB_PORT
        )
        yield conn
    finally:
        if conn:
            conn.close()
