# Descripción de Archivos y Funciones del Proyecto RSS2

Este documento detalla la estructura del proyecto y la función de sus archivos principales.

## 🐳 Infraestructura y Despliegue

| Archivo / Directorio | Descripción |
|----------------------|-------------|
| `docker-compose.yml` | **Orquestador principal**. Define todos los servicios (db, web, workers, redis, qdrant, etc.), redes, volúmenes de persistencia y configuración de recursos. |
| `Dockerfile` | Definición de la imagen base para la aplicación web y la mayoría de los workers en Python. |
| `Dockerfile.llm_worker` | Imagen específica para el worker de LLM, incluye dependencias de CUDA y PyTorch para ExLlamaV2. |
| `Dockerfile.url_worker` | Imagen optimizada para el worker de descubrimiento y procesamiento de URLs. |
| `nginx.conf` | Configuración del servidor web Nginx que actúa como proxy inverso y servidos de archivos estáticos. |
| `.env` | Variables de entorno con credenciales y configuración sensible (NO compartir). |
| `gunicorn_config.py` | Configuración del servidor de aplicaciones WSGI Gunicorn para producción. |

## 🧠 Núcleo de la Aplicación (Python)

| Archivo | Descripción |
|---------|-------------|
| `app.py` | **Punto de entrada**. Inicializa la aplicación Flask, registra blueprints (rutas) y configura extensiones. |
| `config.py` | Carga y valida la configuración desde variables de entorno. Define constantes globales. |
| `db.py` | Gestión de la conexión a la base de datos PostgreSQL (pool de conexiones). |
| `cache.py` | Capa de abstracción para Redis. Maneja caché de respuestas y estados transitorios. |
| `scheduler.py` | Planificador de tareas periódicas (cron jobs internos) para mantenimiento y disparadores. |
| `requirements.txt` | Lista de dependencias de Python necesarias para el proyecto. |

## 👷 Workers (Procesamiento en Segundo Plano)

Ubicados en `workers/`:

| Archivo | Función |
|---------|---------|
| `llm_categorizer_worker.py` | Categoriza noticias usando un LLM local (Mistral/ExLlamaV2). Asigna etiquetas temáticas. |
| `url_worker_daemon.py` | Procesa y valida URLs extraídas, gestionando la cola de descargas. |
| `url_discovery_worker.py` | Busca nuevos feeds RSS a partir de las URLs base. |
| `translation_worker.py` | Traduce contenido usando modelos NLLB (No Language Left Behind). |
| `embeddings_worker.py` | Genera vectores semánticos para búsqueda y clustering. |
| `cluster_worker.py` | Agrupa noticias similares en "historias" o eventos. |
| `ner_worker.py` | Extracción de Entidades Nombradas (personas, organizaciones, lugares). |
| `topics_worker.py` | Identifica y extrae tópicos principales de los textos. |
| `qdrant_worker.py` | Sincroniza los vectores generados con la base de datos vectorial Qdrant. |

## 🌐 API y Rutas

Ubicados en `routers/`:

| Archivo | Descripción |
|---------|-------------|
| `api.py` | Endpoints generales de la API REST. |
| `feeds.py` | Gestión de fuentes RSS (CRUD). |
| `news.py` | Endpoints para listar y filtrar noticias. |
| `dashboard.py` | Rutas para el panel de administración y estadísticas. |
| `auth.py` | Manejo de autenticación y autorización. |
| `search.py` | Endpoints para búsqueda semántica y tradicional. |

## 🛠️ Herramientas y Scripts

| Archivo | Descripción |
|---------|-------------|
| `scripts/download_llm_model.sh` | Script para descargar modelos LLM cuantizados desde HuggingFace de forma segura. |
| `verify_security.sh` | Auditoría de seguridad automatizada (verifica permisos, configuración TLS, etc.). |
| `generate_secure_credentials.sh` | Genera contraseñas seguras y configura el entorno inicial. |
| `rss-ingestor-go/` | Ingestor de RSS de alto rendimiento escrito en Go. |

## 📂 Directorios de Datos

| Directorio | Contenido |
|------------|-----------|
| `models/` | Almacenamiento persistente de modelos de IA (LLMs, Embeddings, Traducción). |
| `pgdata/` | Persistencia de la base de datos PostgreSQL. |
| `qdrant_storage/` | Persistencia de la base de datos vectorial Qdrant. |
| `hf_cache/` | Caché de HuggingFace para modelos y tokenizers. |
| `templates/` | Plantillas HTML (Jinja2) para la interfaz web. |
| `static/` | Archivos CSS, JS e imágenes públicas. |
