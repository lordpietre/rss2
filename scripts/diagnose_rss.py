import os
import psycopg2
from datetime import datetime

# Database configuration
DB_WRITE_HOST = os.environ.get("DB_WRITE_HOST", "db")
DB_NAME = os.environ.get("DB_NAME", "rss")
DB_USER = os.environ.get("DB_USER", "rss")
DB_PASS = os.environ.get("DB_PASS", "x")
DB_PORT = os.environ.get("DB_PORT", "5432")

def check_db():
    try:
        conn = psycopg2.connect(
            host=DB_WRITE_HOST,
            database=DB_NAME,
            user=DB_USER,
            password=DB_PASS,
            port=DB_PORT,
            connect_timeout=5
        )
        print("✅ Database connection successful.")
        
        with conn.cursor() as cur:
            # 1. Total news and latest date
            cur.execute("SELECT COUNT(*), MAX(fecha) FROM noticias;")
            count, latest = cur.fetchone()
            print(f"📊 Total news: {count}")
            print(f"🕒 Latest news date: {latest}")
            
            # 2. Feed status
            cur.execute("SELECT COUNT(*) FROM feeds WHERE activo = TRUE;")
            active_feeds = cur.fetchone()[0]
            cur.execute("SELECT COUNT(*) FROM feeds WHERE activo = FALSE;")
            inactive_feeds = cur.fetchone()[0]
            print(f"📡 Active feeds: {active_feeds}")
            print(f"🚫 Inactive feeds: {inactive_feeds}")
            
            # 3. Feeds with most failures
            cur.execute("SELECT id, nombre, url, fallos, last_error FROM feeds WHERE fallos > 0 ORDER BY fallos DESC LIMIT 5;")
            failures = cur.fetchall()
            if failures:
                print("\n⚠️ Feeds with most failures:")
                for f in failures:
                    print(f"  - ID {f[0]}: {f[1]} ({f[3]} fallos) - Error: {f[4]}")
            else:
                print("\n✅ No feeds with reported failures.")

            # 4. Check for unprocessed translations (if applicable)
            # Checking schema again: table 'noticias' doesn't seem to have a 'translated' flag?
            # Conversation eeb18716 mentioned 'TRAD/MIN, PENDING, PROCESSING, COMPLETED, ERRORS' metrics.
            # Let's check 'traducciones' table if it exists.
            cur.execute("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'traducciones');")
            if cur.fetchone()[0]:
                cur.execute("SELECT COUNT(*) FROM noticias WHERE id NOT IN (SELECT noticia_id FROM traducciones);")
                pending_trans = cur.fetchone()[0]
                print(f"🌎 News pending translation: {pending_trans}")

        conn.close()
    except Exception as e:
        print(f"❌ Database error: {e}")

if __name__ == "__main__":
    check_db()
