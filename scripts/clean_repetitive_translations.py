#!/usr/bin/env python3
"""
Script to detect and clean repetitive/low-quality translations.
Run this periodically or as a maintenance task.
"""
import os
import re
import sys
import psycopg2
from psycopg2.extras import execute_values
from dotenv import load_dotenv

load_dotenv()

DB_CONFIG = {
    "host": os.environ.get("DB_HOST", "localhost"),
    "port": int(os.environ.get("DB_PORT", 5432)),
    "dbname": os.environ.get("DB_NAME", "rss"),
    "user": os.environ.get("DB_USER", "rss"),
    "password": os.environ.get("DB_PASS", ""),
}

def is_repetitive(text: str, threshold: float = 0.25) -> bool:
    """Check if text has repetitive patterns or low word diversity."""
    if not text or len(text) < 50:
        return False
    
    # Check for obvious repetitive patterns
    repetitive_patterns = [
        r'(\b\w+\b)( \1){3,}',  # Same word repeated 4+ times
        r'(\b\w+ \w+\b)( \1){2,}',  # Same 2-word phrase repeated 3+ times
        r'de la la ',
        r'la línea de la línea',
        r'de Internet de Internet',
        r'de la de la',
        r'en el en el',
    ]
    
    for pattern in repetitive_patterns:
        if re.search(pattern, text, re.IGNORECASE):
            return True
    
    # Check word diversity
    words = text.lower().split()
    if len(words) < 10:
        return False
    
    unique_ratio = len(set(words)) / len(words)
    return unique_ratio < threshold

def main():
    print("🔍 Scanning for repetitive translations...")
    
    conn = psycopg2.connect(**DB_CONFIG)
    
    with conn.cursor() as cur:
        # Fetch all done translations
        cur.execute("""
            SELECT id, titulo_trad, resumen_trad 
            FROM traducciones 
            WHERE status='done'
        """)
        
        rows = cur.fetchall()
        total = len(rows)
        print(f"📊 Checking {total} translations...")
        
        bad_ids = []
        for tr_id, titulo, resumen in rows:
            if is_repetitive(titulo) or is_repetitive(resumen):
                bad_ids.append(tr_id)
        
        print(f"❌ Found {len(bad_ids)} repetitive translations ({len(bad_ids)/total*100:.2f}%)")
        
        if bad_ids:
            # Show samples
            cur.execute("""
                SELECT id, LEFT(resumen_trad, 150) as sample 
                FROM traducciones 
                WHERE id = ANY(%s) 
                LIMIT 5
            """, (bad_ids,))
            
            print("\n📝 Sample bad translations:")
            for row in cur.fetchall():
                print(f"  ID {row[0]}: {row[1]}...")
            
            # Reset to pending
            print(f"\n🔄 Resetting {len(bad_ids)} translations to pending...")
            cur.execute("""
                UPDATE traducciones 
                SET status='pending', 
                    titulo_trad=NULL, 
                    resumen_trad=NULL, 
                    error='Repetitive output - auto-cleaned'
                WHERE id = ANY(%s)
            """, (bad_ids,))
            
            conn.commit()
            print(f"✅ Successfully reset {len(bad_ids)} translations")
        else:
            print("✅ No repetitive translations found!")
    
    conn.close()
    print("\n✨ Cleanup complete!")

if __name__ == "__main__":
    main()
