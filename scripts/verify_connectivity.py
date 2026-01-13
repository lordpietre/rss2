import sys
import os

# Add app to path
sys.path.append('/home/x/rss2')

try:
    from db import get_conn, get_read_conn, get_write_conn
    from cache import get_redis
    import psycopg2
    print("Imports successfull.")
except ImportError as e:
    print(f"Import failed: {e}")
    sys.exit(1)

def test_db():
    print("\n--- Testing Database Connections ---")
    
    print("Testing Primary (Write) Connection...")
    try:
        with get_write_conn() as conn:
            with conn.cursor() as cur:
                cur.execute("SELECT 1")
                print("  [OK] Primary reachable.")
    except Exception as e:
        print(f"  [FAIL] Primary unreachable: {e}")

    print("Testing Replica (Read) Connection...")
    try:
        with get_read_conn() as conn:
            with conn.cursor() as cur:
                cur.execute("SELECT 1")
                # Check if it's actually the replica (read-only mode is usually set in replica, 
                # but here we just check connectivity)
                print("  [OK] Replica reachable.")
    except Exception as e:
        print(f"  [FAIL] Replica unreachable: {e}")

def test_redis():
    print("\n--- Testing Redis Connection ---")
    try:
        r = get_redis()
        if r:
            r.ping()
            print("  [OK] Redis reachable.")
        else:
            print("  [FAIL] Redis client returned None (likely connection failed).")
    except Exception as e:
        print(f"  [FAIL] Redis error: {e}")

if __name__ == "__main__":
    test_db()
    test_redis()
    print("\nVerification complete.")
