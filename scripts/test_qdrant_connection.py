#!/usr/bin/env python3
"""
Script de diagnóstico para verificar la conectividad con Qdrant.
Ejecutar desde el contenedor rss2_web para diagnosticar problemas de red.
"""
import os
import sys

def test_qdrant_connection():
    """Prueba la conexión a Qdrant y muestra información de diagnóstico."""
    
    # Configuración
    qdrant_host = os.environ.get("QDRANT_HOST", "localhost")
    qdrant_port = int(os.environ.get("QDRANT_PORT", "6333"))
    
    print("=" * 60)
    print("🔍 DIAGNÓSTICO DE CONEXIÓN QDRANT")
    print("=" * 60)
    print(f"Host: {qdrant_host}")
    print(f"Port: {qdrant_port}")
    print()
    
    # 1. Test de resolución DNS
    print("1️⃣ Probando resolución DNS...")
    try:
        import socket
        ip = socket.gethostbyname(qdrant_host)
        print(f"   ✅ Host '{qdrant_host}' resuelve a: {ip}")
    except Exception as e:
        print(f"   ❌ ERROR: No se pudo resolver '{qdrant_host}': {e}")
        return False
    
    # 2. Test de conectividad TCP
    print("\n2️⃣ Probando conectividad TCP...")
    try:
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.settimeout(5)
        result = sock.connect_ex((ip, qdrant_port))
        sock.close()
        
        if result == 0:
            print(f"   ✅ Puerto {qdrant_port} está abierto")
        else:
            print(f"   ❌ ERROR: Puerto {qdrant_port} está cerrado o inaccesible")
            return False
    except Exception as e:
        print(f"   ❌ ERROR en test TCP: {e}")
        return False
    
    # 3. Test de cliente Qdrant
    print("\n3️⃣ Probando cliente Qdrant...")
    try:
        from qdrant_client import QdrantClient
        
        client = QdrantClient(host=qdrant_host, port=qdrant_port, timeout=5)
        collections = client.get_collections()
        
        print(f"   ✅ Cliente Qdrant conectado exitosamente")
        print(f"   📊 Colecciones disponibles: {[c.name for c in collections.collections]}")
        
        # Test de búsqueda
        for collection in collections.collections:
            try:
                info = client.get_collection(collection.name)
                print(f"   📁 {collection.name}: {info.points_count} vectores")
            except Exception as e:
                print(f"   ⚠️ No se pudo obtener info de {collection.name}: {e}")
        
        return True
        
    except Exception as e:
        print(f"   ❌ ERROR en cliente Qdrant: {e}")
        import traceback
        traceback.print_exc()
        return False
    
    print("\n" + "=" * 60)

if __name__ == "__main__":
    success = test_qdrant_connection()
    
    if success:
        print("\n✅ DIAGNÓSTICO EXITOSO: Qdrant está accesible")
        sys.exit(0)
    else:
        print("\n❌ DIAGNÓSTICO FALLIDO: Problemas de conectividad con Qdrant")
        print("\n💡 SOLUCIONES POSIBLES:")
        print("   1. Verificar que el contenedor 'qdrant' esté corriendo:")
        print("      docker ps | grep qdrant")
        print("   2. Verificar que ambos contenedores estén en la misma red:")
        print("      docker network inspect rss2_default")
        print("   3. Reiniciar el contenedor de Qdrant:")
        print("      docker restart rss2_qdrant")
        print("   4. Verificar variables de entorno QDRANT_HOST y QDRANT_PORT")
        sys.exit(1)
