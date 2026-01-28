# RSS2 - Plataforma de Inteligencia de Noticias con IA 🚀

RSS2 es una plataforma avanzada de agregación, traducción, análisis y vectorización de noticias diseñada para transformar flujos masivos de información en inteligencia accionable. Utiliza una arquitectura de **microservicios híbrida (Go + Python)** con modelos de **Inteligencia Artificial** de vanguardia para ofrecer búsqueda semántica, clasificación inteligente y automatización de contenidos.

---

## ✨ Características Principales

*   🤖 **Categorización Inteligente (LLM)**: Clasificación de noticias mediante **Mistral-7B** local (ExLlamaV2/GPTQ), procesando lotes de alta velocidad.
*   🔍 **Búsqueda Semántica**: Motor vectorial **Qdrant** para encontrar noticias por contexto y significado, no solo por palabras clave.
*   🌍 **Traducción Neuronal de Alta Calidad**: Integración con **NLLB-200** para traducir noticias de múltiples idiomas al español con validación post-proceso para evitar repeticiones.
*   📊 **Inteligencia de Entidades**: Extracción automática y normalización de Personas, Organizaciones y Lugares para análisis de tendencias.
*   📺 **Automatización de Video**: Generación automática de noticias en formato video y gestión de "parrillas" de programación.
*   📄 **Exportación Inteligente**: Generación de informes en **PDF** con diseño profesional y limpieza de ruido de red.
*   🔔 **Notificaciones en Tiempo Real**: API de monitoreo para detectar eventos importantes al instante.
*   ⭐ **Gestión de Favoritos**: Sistema robusto para guardar y organizar noticias, compatible con usuarios y sesiones temporales.

---

## 🏗️ Arquitectura de Servicios (Docker)

El sistema está orquestado mediante Docker Compose, garantizando aislamiento y escalabilidad.

### 🌐 Core & Acceso (Frontend)
| Servicio | Tecnología | Descripción |
|----------|------------|-------------|
| **`nginx`** | Nginx Alpine | Gateway y Proxy Inverso (Puerto **8001**). |
| **`rss2_web`** | Flask + Gunicorn | API principal e Interfaz Web de usuario. |

### 📥 Ingesta y Descubrimiento (Backend)
| Servicio | Tecnología | Descripción |
|----------|------------|-------------|
| **`rss-ingestor-go`** | **Go** | Crawler de ultra-alto rendimiento (Cientos de feeds/min). |
| **`url-worker`** | Python | Scraper profundo con limpieza de HTML via `newspaper3k`. |
| **`url-discovery`** | Python | Agente autónomo para el descubrimiento de nuevos feeds. |

### 🧠 Procesamiento de IA (Background Workers)
| Servicio | Modelo / Función | Descripción |
|----------|-------------------|-------------|
| **`llm-categorizer`** | **Mistral-7B** | Categorización contextual avanzada (15 categorías). |
| **`translator`** (x3) | **NLLB-200** | Traducción neural masiva escalada horizontalmente. |
| **`embeddings`** | **S-Transformers** | Conversión de texto a vectores para búsqueda semántica. |
| **`ner`** | **Spacy/BERT** | Extracción de entidades (Personas, Lugares, Orgs). |
| **`cluster` & `related`**| Algoritmos Propios | Agrupación de eventos y detección de noticias relacionadas. |

### 💾 Almacenamiento y Datos
| Servicio | Rol | Descripción |
|----------|-----|-------------|
| **`db`** | **PostgreSQL 18** | Almacenamiento relacional principal y metadatos. |
| **`qdrant`** | **Vector DB** | Motor de búsqueda por similitud de alta velocidad. |
| **`redis`** | **Redis 7** | Gestión de colas de tareas (Celery-style) y caché. |

---

## 🚀 Guía de Inicio Rápido

### 1. Preparación
```bash
git clone <repo>
cd rss2
./generate_secure_credentials.sh  # Genera .env seguro y contraseñas robustas
```

### 2. Configuración de Modelos (IA)
Para activar la categorización inteligente y traducción, descarga los modelos:
```bash
./scripts/download_llm_model.sh  # Recomendado: Mistral-7B GPTQ
python3 scripts/download_models.py # Modelos NLLB y Embeddings
```

### 3. Arranque del Sistema
```bash
./start_docker.sh  # Script de inicio con verificación de dependencias
```

---

## 📖 Documentación Especializada

Consulte nuestras guías detalladas para configuraciones específicas:

*   📘 **[QUICKSTART_LLM.md](QUICKSTART_LLM.md)**: Guía rápida para el categorizador Mistral-7B.
*   🚀 **[DEPLOY.md](DEPLOY.md)**: Guía detallada de despliegue en nuevos servidores.
*   📊 **[TRANSLATION_FIX_SUMMARY.md](TRANSLATION_FIX_SUMMARY.md)**: Resumen de mejoras en calidad de traducción.
*   🛡️ **[SECURITY_GUIDE.md](SECURITY_GUIDE.md)**: Manual avanzado de seguridad y endurecimiento.
*   🏗️ **[QDRANT_SETUP.md](QDRANT_SETUP.md)**: Configuración y migración de la base de datos vectorial.
*   📑 **[FUNCIONES_DE_ARCHIVOS.md](FUNCIONES_DE_ARCHIVOS.md)**: Inventario detallado de la lógica del proyecto.

---

## 💻 Requisitos de Hardware

Para un rendimiento óptimo, se recomienda:
*   **GPU**: NVIDIA (mínimo 12GB VRAM para Mistral-7B y traducción simultánea).
*   **Drivers**: NVIDIA Container Toolkit instalado.
*   **AllTalk TTS**: Instancia activa (puerto 7851) para la generación de audio en videos.

---

## 🔧 Operaciones y Mantenimiento

### Verificación de Calidad de Traducción
El sistema incluye herramientas para asegurar la calidad de los datos:
```bash
# Monitorear calidad en tiempo real
docker exec rss2_web python3 scripts/monitor_translation_quality.py --watch

# Limpiar automáticamente traducciones defectuosas
docker exec rss2_web python3 scripts/clean_repetitive_translations.py
```

### Gestión de Contenidos
```bash
# Generar videos de noticias destacadas
docker exec rss2_web python3 scripts/generar_videos_noticias.py

# Iniciar migración a Qdrant (Vectores)
docker exec rss2_web python3 scripts/migrate_to_qdrant.py
```

### Diagnóstico de Ingesta (Feeds)
```bash
docker exec rss2_web python3 scripts/diagnose_rss.py --url <FEED_URL>
```

---

## 📊 Observabilidad
Acceso a métricas de rendimiento (Solo vía Localhost/Tunel):
*   **Grafana**: [http://localhost:3001](http://localhost:3001) (Admin/Pass en `.env`)
*   **Proxy Nginx**: [http://localhost:8001](http://localhost:8001)

---

**RSS2** - *Transformando noticias en inteligencia con IA Local.*
