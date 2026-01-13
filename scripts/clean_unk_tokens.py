#!/usr/bin/env python3
"""
Script para limpiar caracteres <unk> de las traducciones.
"""
import re
from db import get_conn

def clean_text(text):
    """Remove <unk> tokens and other problematic characters."""
    if not text:
        return text
    # Remove <unk> tokens
    text = text.replace('<unk>', '')
    text = text.replace('�', '')
    # Remove other problematic Unicode characters
    text = re.sub(r'[\x00-\x08\x0B-\x0C\x0E-\x1F\x7F-\x9F]', '', text)
    return text.strip()

def main():
    """Clean all translations with <unk> tokens."""
    print("🧹 Limpiando tokens <unk> de traducciones...")
    
    with get_conn() as conn:
        with conn.cursor() as cur:
            # Find translations with <unk> tokens
            cur.execute("""
                SELECT id, titulo_trad, resumen_trad
                FROM traducciones
                WHERE titulo_trad LIKE '%<unk>%' 
                   OR resumen_trad LIKE '%<unk>%' 
                   OR titulo_trad LIKE '%�%'
                   OR resumen_trad LIKE '%�%'
            """)
            
            translations = cur.fetchall()
            print(f"📊 Encontradas {len(translations)} traducciones con tokens problemáticos")
            
            if not translations:
                print("✅ No hay traducciones que limpiar")
                return
            
            updated_count = 0
            for row in translations:
                tr_id, titulo, resumen = row
                
                # Clean the fields
                new_titulo = clean_text(titulo) if titulo else titulo
                new_resumen = clean_text(resumen) if resumen else resumen
                
                # Update only if something changed
                if new_titulo != titulo or new_resumen != resumen:
                    cur.execute("""
                        UPDATE traducciones
                        SET titulo_trad = %s,
                            resumen_trad = %s
                        WHERE id = %s
                    """, (new_titulo, new_resumen, tr_id))
                    updated_count += 1
                    
                    if updated_count % 100 == 0:
                        print(f"  ⏳ Procesadas {updated_count} traducciones...")
            
            conn.commit()
            print(f"✅ Limpieza completada: {updated_count} traducciones actualizadas")

if __name__ == "__main__":
    main()
