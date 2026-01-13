import html
import psycopg2
from db import get_conn
import re

def fix_entities():
    print("🔧 Fixing HTML entities in database...")
    
    with get_conn() as conn:
        with conn.cursor() as cur:
            # 1. Update Noticias
            print("Processing 'noticias' table...")
            cur.execute("""
                SELECT id, titulo, resumen 
                FROM noticias 
                WHERE titulo LIKE '%&%;%' OR resumen LIKE '%&%;%'
            """)
            rows = cur.fetchall()
            print(f"Found {len(rows)} rows in 'noticias' to check.")
            
            count = 0
            for r in rows:
                nid, tit, res = r
                
                new_tit = html.unescape(tit) if tit else tit
                new_res = html.unescape(res) if res else res
                
                if new_tit != tit or new_res != res:
                    cur.execute("""
                        UPDATE noticias 
                        SET titulo = %s, resumen = %s 
                        WHERE id = %s
                    """, (new_tit, new_res, nid))
                    count += 1
                    if count % 100 == 0:
                        print(f"Updated {count} noticias...")
            
            print(f"Updated {count} rows in 'noticias'.")

            # 2. Update Traducciones
            print("\nProcessing 'traducciones' table...")
            cur.execute("""
                SELECT id, titulo_trad, resumen_trad 
                FROM traducciones 
                WHERE titulo_trad LIKE '%&%;%' OR resumen_trad LIKE '%&%;%'
            """)
            rows = cur.fetchall()
            print(f"Found {len(rows)} translations to check.")
            
            count_tr = 0
            for r in rows:
                tid, tit, res = r
                
                new_tit = html.unescape(tit) if tit else tit
                new_res = html.unescape(res) if res else res
                
                if new_tit != tit or new_res != res:
                    cur.execute("""
                        UPDATE traducciones 
                        SET titulo_trad = %s, resumen_trad = %s 
                        WHERE id = %s
                    """, (new_tit, new_res, tid))
                    count_tr += 1
            
            print(f"Updated {count_tr} rows in 'traducciones'.")
            
            conn.commit()
            print("✅ Database cleaning complete.")

if __name__ == "__main__":
    fix_entities()
