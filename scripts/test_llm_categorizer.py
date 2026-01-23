#!/usr/bin/env python3
"""
Script de prueba para el LLM Categorizer
Prueba la categorización con datos de ejemplo sin necesidad del contenedor
"""

import os
import sys

# Datos de prueba
TEST_NEWS = [
    {
        'id': 'test_1',
        'titulo': 'El gobierno anuncia nuevas medidas económicas para combatir la inflación',
        'resumen': 'El presidente del gobierno ha presentado un paquete de medidas económicas destinadas a reducir la inflación y proteger el poder adquisitivo de las familias.'
    },
    {
        'id': 'test_2',
        'titulo': 'Nueva vacuna contra el cáncer muestra resultados prometedores',
        'resumen': 'Investigadores de la Universidad de Stanford han desarrollado una vacuna experimental que ha mostrado una eficacia del 85% en ensayos clínicos con pacientes con melanoma.'
    },
    {
        'id': 'test_3',
        'titulo': 'El Real Madrid gana la Champions League por decimoquinta vez',
        'resumen': 'El equipo blanco se impuso por 2-1 en la final celebrada en Wembley, consolidándose como el club más laureado de la competición europea.'
    },
    {
        'id': 'test_4',
        'titulo': 'OpenAI lanza GPT-5 con capacidades multimodales mejoradas',
        'resumen': 'La nueva versión del modelo de lenguaje incorpora mejor comprensión de imágenes, video y audio, además de un razonamiento más avanzado.'
    },
    {
        'id': 'test_5',
        'titulo': 'Crisis diplomática entre Estados Unidos y China por aranceles',
        'resumen': 'Las tensiones comerciales se intensifican después de que Washington impusiera nuevos aranceles del 25% a productos tecnológicos chinos.'
    }
]

def test_without_llm():
    """Prueba básica sin LLM (categorización basada en keywords)"""
    print("=== Prueba de Categorización Básica (sin LLM) ===\n")
    
    # Categorías con palabras clave simples
    CATEGORIES_KEYWORDS = {
        'Política': ['gobierno', 'presidente', 'político', 'parlamento', 'elecciones'],
        'Economía': ['económic', 'inflación', 'aranceles', 'bolsa', 'financiero'],
        'Salud': ['vacuna', 'hospital', 'médico', 'tratamiento', 'enfermedad'],
        'Deportes': ['fútbol', 'champions', 'equipo', 'partido', 'gana'],
        'Tecnología': ['tecnológic', 'digital', 'software', 'ai', 'gpt', 'openai'],
        'Internacional': ['estados unidos', 'china', 'rusia', 'diplomática', 'crisis'],
    }
    
    for news in TEST_NEWS:
        text = (news['titulo'] + ' ' + news['resumen']).lower()
        
        best_category = 'Otros'
        max_score = 0
        
        for category, keywords in CATEGORIES_KEYWORDS.items():
            score = sum(1 for kw in keywords if kw in text)
            if score > max_score:
                max_score = score
                best_category = category
        
        print(f"ID: {news['id']}")
        print(f"Título: {news['titulo']}")
        print(f"Categoría: {best_category} (score: {max_score})")
        print()

def test_with_llm():
    """Prueba con el LLM real (requiere modelo descargado)"""
    print("\n=== Prueba de Categorización con LLM ===\n")
    
    # Configurar path del modelo
    MODEL_PATH = os.environ.get("LLM_MODEL_PATH", "/home/x/rss2/models/llm")
    
    if not os.path.exists(MODEL_PATH):
        print(f"❌ Error: No se encuentra el modelo en {MODEL_PATH}")
        print(f"Por favor ejecuta primero: ./scripts/download_llm_model.sh")
        return
    
    # Verificar si exllamav2 está instalado
    try:
        import exllamav2
        print(f"✓ ExLlamaV2 instalado: {exllamav2.__version__}")
    except ImportError:
        print("❌ Error: ExLlamaV2 no está instalado")
        print("Instalar con: pip install exllamav2")
        return
    
    # Importar el categorizer
    sys.path.insert(0, '/home/x/rss2')
    from workers.llm_categorizer_worker import ExLlamaV2Categorizer
    
    print(f"Cargando modelo desde: {MODEL_PATH}")
    print("(Esto puede tardar unos minutos...)\n")
    
    try:
        categorizer = ExLlamaV2Categorizer(MODEL_PATH)
        print("✓ Modelo cargado exitosamente\n")
        
        results = categorizer.categorize_news(TEST_NEWS)
        
        print("\n=== Resultados ===\n")
        for i, news in enumerate(TEST_NEWS):
            result = results[i]
            print(f"ID: {news['id']}")
            print(f"Título: {news['titulo']}")
            print(f"Categoría: {result['categoria']}")
            print(f"Confianza: {result['confianza']:.2f}")
            print()
            
    except Exception as e:
        print(f"❌ Error: {e}")
        import traceback
        traceback.print_exc()

def main():
    print("=" * 60)
    print("Script de Prueba del LLM Categorizer")
    print("=" * 60)
    print()
    
    # Prueba básica siempre funciona
    test_without_llm()
    
    # Preguntar si probar con LLM
    print("\n¿Deseas probar con el LLM real? (requiere modelo descargado)")
    print("Esto cargará el modelo en GPU y puede tardar varios minutos.")
    response = input("Continuar? [s/N]: ").strip().lower()
    
    if response in ['s', 'si', 'y', 'yes']:
        test_with_llm()
    else:
        print("\nPrueba finalizada. Para probar con el LLM:")
        print("1. Descarga el modelo: ./scripts/download_llm_model.sh")
        print("2. Ejecuta este script de nuevo y acepta probar con LLM")

if __name__ == "__main__":
    main()
