# RSS2 - AI-Powered News Intelligence Platform

RSS2 es una plataforma avanzada de agregación, traducción, análisis y vectorización de noticias, diseñada para transformar flujos masivos de información en inteligencia accionable. Utiliza una arquitectura híbrida de microservicios (Go + Python) integrada con modelos de inteligencia artificial de última generación para ofrecer búsqueda semántica, clasificación inteligente y automatización de contenidos.

---

## 🚀 Capacidades Principales

*   **Enriquecimiento con Wikipedia**: Sistema automatizado que detecta personas y organizaciones, descarga sus biografías e imágenes oficiales de Wikipedia para mostrarlas en tooltips interactivos con avatares circulares.
*   **Categorización Inteligente (LLM)**: Clasificación de noticias mediante una instancia local de Mistral-7B / Llama-3 (vía Ollama), procesando contenido en tiempo real.
*   **Búsqueda Semántica**: Motor vectorial Qdrant para descubrir noticias por contexto y significado, yendo más allá de las palabras clave tradicionales.
*   **Traducción Neuronal de Alta Calidad**: Integración de NLLB-200 (vía CTranslate2) para traducir noticias de múltiples idiomas al español con precisión profesional.
*   **Inteligencia de Entidades (NER)**: Extracción y normalización automática de Personas, Organizaciones y Lugares para análisis de tendencias y mapeo de relaciones.
*   **Búsqueda de Noticias Relacionadas**: Algoritmos de similitud que agrupan noticias sobre el mismo tema automáticamente.

---

## 🏗️ Arquitectura de Servicios (Docker)

El sistema se orquestra mediante Docker Compose y se divide en capas especializadas:

### Capa de Acceso y API
| Servicio | Tecnología | Descripción |
|---------|------------|-------------|
| **`nginx`** | Nginx Alpine | Gateway y Proxy Inverso (Puerto **8001**). |
| **`rss2_frontend`** | React + Vite | Interfaz web de usuario moderna y responsiva. |
| **`backend-go`** | Go + Gin | API REST principal y gestión de lógica de negocio. |

### Ingesta y Descubrimiento (Go)
| Servicio | Tecnología | Descripción |
|---------|------------|-------------|
| **`rss-ingestor-go`** | Go | Crawler de alto rendimiento para feeds RSS. |
| **`scraper`** | Go | Scraper profundo con sanitización de HTML y extracción de texto. |
| **`discovery`** | Go | Agente autónomo para descubrir nuevos feeds a partir de URLs. |

### Procesamiento de Datos e IA (Go & Python)
| Servicio | Tecnología | Descripción |
|---------|------------|-------------|
| **`translator`** | NLLB-200 (CPU) | Traducción neuronal optimizada con CTranslate2. |
| **`translator-gpu`**| NLLB-200 (GPU) | Traducción acelerada por hardware (CUDA). |
| **`wiki-worker`** | Go | **[NUEVO]** Integración con Wikipedia y gestión de imágenes locales. |
| **`embeddings`** | S-Transformers | Generación de vectores para búsqueda semántica. |
| **`ner`** | Spacy / BERT | Reconocimiento de entidades nombradas (NER). |
| **`llm-categorizer`**| Ollama / Mistral | Clasificación avanzada mediante modelos de lenguaje. |
| **`topics`** | Go | Matcher automático de países y temas predefinidos. |
| **`related`** | Go | Motor de detección de noticias relacionadas. |

### Capa de Almacenamiento
| Servicio | Tecnología | Descripción |
|---------|------------|-------------|
| **`db`** | PostgreSQL 18 | Base de datos relacional principal. |
| **`qdrant`** | Qdrant | Base de datos vectorial para búsqueda por similitud. |
| **`redis`** | Redis 7 | Colas de mensajes y caché de alto desempeño. |

---

## ⚙️ Guía de Configuración

### 1. Requisitos de Hardware
*   **Modo Básico (CPU)**: 4+ Cores CPU, 8GB RAM.
*   **Modo Avanzado (IA)**: NVIDIA GPU con 8GB+ VRAM (mínimo recomendado para LLM y Traducción GPU).

### 2. Instalación Rápida
```bash
git clone <repo_url>
cd rss2
cp .env.example .env
# Edita .env con tus credenciales
docker compose up -d
```

### 3. Escalado de Workers (¡Importante!)
Para aumentar la velocidad de procesamiento (especialmente la traducción), puedes escalar los workers:

```bash
# Ejecutar 4 traductores en paralelo
docker compose up -d --scale translator=4

# Si usas GPU y tienes capacidad
docker compose up -d --scale translator-gpu=2
```

---

## 🛡️ Administración y Mantenimiento

### Copias de Seguridad (Backups)
Desde el panel de Administración (`/admin/settings`), puedes realizar:
*   **Backup Completo**: Volcado SQL de toda la base de datos.
*   **Backup de Noticias (ZIP)**: **[NUEVO]** Genera un archivo comprimido que incluye las tablas de noticias, traducciones y todas sus etiquetas. Ideal para migraciones de contenido.

### Variables de Entorno Clave (`.env`)
| Variable | Descripción |
|----------|-------------|
| `WIKI_SLEEP` | Tiempo de espera entre peticiones a Wikipedia (evita bloqueos). |
| `SCHEDULER_BATCH`| Cantidad de noticias a enviar a traducir por ciclo. |
| `TARGET_LANGS` | Idiomas destino (ej: `es`). |
| `OLLAMA_HOST` | Dirección del servidor Ollama para categorización. |

---

## 📖 Documentación de la API (Campos Wikipedia)

Las respuestas de noticias ahora incluyen el objeto `entities` enriquecido:

```json
{
  "id": 67449,
  "titulo": "...",
  "entities": [
    {
      "valor": "Apple",
      "tipo": "organizacion",
      "wiki_summary": "Apple Inc. es una empresa estadounidense...",
      "wiki_url": "https://es.wikipedia.org/wiki/Apple",
      "image_path": "/api/wiki-images/wiki_5723.png"
    }
  ]
}
```

---

**RSS2** - *Transformando noticias en inteligencia con IA localizada.*
