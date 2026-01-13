import html
import psycopg2
from db import get_conn
import sys

def recursive_unescape(text):
    if not text:
        return text
    
    # Limit loops to prevent infinite loops on weird edge cases
    max_loops = 5
    current = text
    
    for _ in range(max_loops):
        new_text = html.unescape(current)
        if new_text == current:
            break
        current = new_text
        
    return current

def fix_entities_recursive():
    print("🔧 Fixing HTML entities RECURSIVELY in database...")
    
    with get_conn() as conn:
        with conn.cursor() as cur:
            # 1. Update Noticias
            print("Processing 'noticias' table...")
            # We select ALL rows that contain '&' to catch any entity
            # Optimisation: limit to rows with '&' 
            # Note: This might be slow if table is huge, but we have ~13k rows, it's fine.
            cur.execute("""
                SELECT id, titulo, resumen 
                FROM noticias 
                WHERE titulo LIKE '%&%' OR resumen LIKE '%&%'
            """)
            rows = cur.fetchall()
            print(f"Found {len(rows)} candidates in 'noticias'.")
            
            count = 0
            for r in rows:
                nid, tit, res = r
                
                new_tit = recursive_unescape(tit)
                new_res = recursive_unescape(res)
                
                if new_tit != tit or new_res != res:
                    cur.execute("""
                        UPDATE noticias 
                        SET titulo = %s, resumen = %s 
                        WHERE id = %s
                    """, (new_tit, new_res, nid))
                    count += 1
                    if count % 100 == 0:
                        print(f"Updated {count} noticias...")
            
            print(f"Total updated in 'noticias': {count}")

            # 2. Update Traducciones
            print("\nProcessing 'traducciones' table...")
            cur.execute("""
                SELECT id, titulo_trad, resumen_trad 
                FROM traducciones 
                WHERE titulo_trad LIKE '%&%' OR resumen_trad LIKE '%&%'
            """)
            rows = cur.fetchall()
            print(f"Found {len(rows)} candidates in 'traducciones'.")
            
            count_tr = 0
            for r in rows:
                tid, tit, res = r
                
                new_tit = recursive_unescape(tit)
                new_res = recursive_unescape(res)
                
                if new_tit != tit or new_res != res:
                    cur.execute("""
                        UPDATE traducciones 
                        SET titulo_trad = %s, resumen_trad = %s 
                        WHERE id = %s
                    """, (new_tit, new_res, tid))
                    count_tr += 1
                    if count_tr % 100 == 0:
                        print(f"Updated {count_tr} traducciones...")
            
            print(f"Total updated in 'traducciones': {count_tr}")
            
            conn.commit()
            print("✅ Database cleaning complete.")

if __name__ == "__main__":
    fix_entities_recursive()
