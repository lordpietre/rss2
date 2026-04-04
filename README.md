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
*   **Detección de Idioma**: Identificación automática del idioma de origen para routing correcto a traducción.
*   **Clustering de Noticias**: Agrupación automática de noticias relacionadas en eventos mediante embeddings.
*   **Remote Workers GPU**: Workers de traducción GPUremotos conectados vía WebSocket para procesamiento distribuido.
*   **Backup Automatizado**: Exportación completa de la base de datos en SQL o ZIP.

---

## 🏗️ Arquitectura de Servicios (Docker)

El sistema se orquestra mediante Docker Compose y se divide en capas especializadas:

### Capa de Acceso y API
| Servicio | Tecnología | Descripción |
|---------|------------|-------------|
| **`nginx`** | Nginx Alpine | Gateway y Proxy Inverso (Puerto **8888**). |
| **`rss2_frontend`** | React + Vite | Interfaz web de usuario moderna y responsiva. |
| **`backend-go`** | Go + Gin | API REST principal y gestión de lógica de negocio. |

### Ingesta y Descubrimiento (Go)
| Servicio | Tecnología | Descripción |
|---------|------------|-------------|
| **`rss-ingestor-go`** | Go | Crawler de alto rendimiento para feeds RSS (100 workers). |
| **`scraper`** | Go | Scraper profundo con sanitización de HTML y extracción de texto. |
| **`discovery`** | Go | Agente autónomo para descubrir nuevos feeds a partir de URLs. |

### Procesamiento de Datos e IA (Go & Python)
| Servicio | Tecnología | Descripción |
|---------|------------|-------------|
| **`translator`** | NLLB-200 (CPU) | Traducción neuronal optimizada con CTranslate2. |
| **`translator-gpu`** | NLLB-200 (GPU) | Traducción acelerada por hardware (CUDA). |
| **`remote-translator`** | WebSocket | Worker GPU remoto (conexión vía ws://backend-go:8080/ws/worker). |
| **`wiki-worker`** | Go | Integración con Wikipedia y gestión de imágenes locales. |
| **`embeddings`** | S-Transformers | Generación de vectores para búsqueda semántica (paraphrase-multilingual-MiniLM-L12-v2). |
| **`ner`** | Spacy / es_core_news_lg | Reconocimiento de entidades nombradas (NER). |
| **`llm-categorizer`** | Ollama / Mistral | Clasificación avanzada mediante modelos de lenguaje. |
| **`topics`** | Go | Matcher automático de países y temas predefinidos. |
| **`related`** | Go | Motor de detección de noticias relacionadas (similitud coseno). |
| **`qdrant-worker`** | Go | Vectorización y búsqueda semántica con Qdrant + Ollama. |
| **`cluster`** | Python | Agrupación de noticias por eventos. |
| **`langdetect`** | Python | Detección de idioma para routing de traducciones. |
| **`translation-scheduler`** | Python | Creador de tareas de traducción. |

### Capa de Almacenamiento
| Servicio | Tecnología | Descripción |
|---------|------------|-------------|
| **`db`** | PostgreSQL 18 | Base de datos relacional principal. |
| **`qdrant`** | Qdrant | Base de datos vectorial para búsqueda por similitud. |
| **`redis`** | Redis 7 | Colas de mensajes y caché de alto desempeño. |

### Monitoreo
| Servicio | Tecnología | Descripción |
|---------|------------|-------------|
| **`prometheus`** | Prometheus | Métricas del sistema. |
| **`grafana`** | Grafana | Dashboard (puerto **3001**). |
| **`cadvisor`** | cAdvisor | Monitoreo Docker. |

---

## 📊 Endpoints de API

### News
- `GET /api/news` - Listar noticias (paginado, filtros: q, category_id, country_id, translated_only)
- `GET /api/news/:id` - Ver noticia con entidades (incluye Wikipedia enrichment)
- `DELETE /api/news/:id` - Eliminar noticia (admin)

### Feeds
- `GET /api/feeds` - Listar feeds
- `POST /api/feeds` - Crear feed (auth requerido)
- `PUT /api/feeds/:id` - Actualizar feed (auth requerido)
- `DELETE /api/feeds/:id` - Eliminar feed (auth requerido)
- `POST /api/feeds/:id/toggle` - Activar/desactivar feed
- `POST /api/feeds/:id/reactivate` - Reactivar feed

### Search
- `GET /api/search?q=...` - Búsqueda texto (filtros: lang, categoria_id, pais_id)
- `GET /api/search?q=...&semantic=true` - Búsqueda semántica con Qdrant

### Entities
- `GET /api/entities?tipo=persona|organizacion|lugar` - Listar entidades (con aliasing)

### Admin
- `GET /api/admin/backup` - Backup SQL completo (descargable)
- `GET /api/admin/backup/news` - Backup noticias (ZIP)
- `GET /api/admin/users` - Listar usuarios
- `POST /api/admin/users/:id/promote` - Promover a admin
- `POST /api/admin/users/:id/demote` - Quitar admin
- `POST /api/admin/reset-db` - Resetear base de datos
- `POST /api/admin/aliases` - Crear alias de entidad
- `GET /api/admin/aliases/export` - Exportar aliases (CSV)
- `POST /api/admin/aliases/import` - Importar aliases (CSV)
- `POST /api/admin/entities/retype` - Cambiar tipo de entidad
- `POST /api/admin/workers/config` - Configurar workers (type, workers)
- `POST /api/admin/workers/start` - Iniciar workers traducción
- `POST /api/admin/workers/stop` - Detener workers traducción
- `GET /api/admin/workers/status` - Estado de workers
- `GET /api/admin/workers/remote` - Listar remote workers
- `POST /api/admin/workers/remote` - Crear remote worker
- `DELETE /api/admin/workers/remote/:id` - Eliminar remote worker

### Auth
- `POST /api/auth/login` - Iniciar sesión
- `POST /api/auth/register` - Registrarse
- `GET /api/auth/me` - Usuario actual (auth requerido)
- `GET /api/auth/check-first-user` - Verificar si existe primer usuario

### Stats
- `GET /api/stats` - Estadísticas globales (total news, feeds, users, hoy/semana/mes)
- `GET /api/categories` - Listar categorías
- `GET /api/countries` - Listar países con continentes

### WebSocket
- `GET /ws/worker` - Conexión de remote workers (protocolo de autenticación con API key)

---

## 🗄️ Esquema de Base de Datos (Tablas Principales)

- **noticias**: Artículos RSS (titulo, resumen, url, feed_id, categoria_id, pais_id, lang)
- **feeds**: Fuentes RSS (nombre, url, activo, ultimo_fetch)
- **traducciones**: Traducciones (noticia_id, lang_from, lang_to, titulo_trad, resumen_trad, status, vectorized)
- **tags**: Entidades (valor, tipo: persona/organizacion/lugar/tema, wiki_summary, wiki_url, image_path)
- **tags_noticia**: Relación noticia-tag (traduccion_id, noticia_id, tag_id)
- **entity_aliases**: Alias de entidades (canonical_name, alias, tipo)
- **categorias**: Categorías de noticias
- **paises**: Países con continentes
- **related_noticias**: Noticias relacionadas (traduccion_id, related_traduccion_id, score)
- **traduccion_embeddings**: Embeddings en BD (traduccion_id, model, embedding)
- **users**: Usuarios (email, username, password_hash, is_admin)
- **config**: Configuración del sistema (translator_type, translator_workers, translator_status)
- **remote_workers**: Workers remotos (name, api_key, capabilities, status, last_seen)
- **favoritos**: Noticias favoritas de usuarios
- **search_history**: Historial de búsquedas

---

## ⚙️ Guía de Configuración

### 1. Requisitos de Hardware
*   **Modo Básico (CPU)**: 4+ Cores CPU, 8GB RAM.
*   **Modo Avanzado (IA)**: NVIDIA GPU con 8GB+ VRAM (mínimo recomendado para LLM y Traducción GPU).

### 2. Instalación Rápida con Seguridad Automática

#### Opción A: Despliegue con Validación Automática (RECOMENDADO)

```bash
# Paso 1: Clonar el proyecto
git clone <repo_url>
cd rss2

# Paso 2: Generar credenciales seguras automáticamente
./pre-deploy.sh --generate

# Paso 3: Validar y desplegar
./pre-deploy.sh

# Paso 4: Si eliges opción 2 en el prompt, despliega manualmente
docker compose up -d
```

**Ventajas:**
- ✅ Genera credenciales seguras de 32 caracteres
- ✅ Valida que no haya contraseñas por defecto
- ✅ Muestra resumen de credenciales en consola
- ✅ Automático y seguro para producción

#### Opción B: Despliegue Manual (Para desarrolladores)

```bash
git clone <repo_url>
cd rss2

# Generar credenciales manualmente
./generate_secure_credentials.sh

# Copiar credenciales a .env
cp .env.generated .env

# Desplegar
docker compose up -d
```

---

## 🔐 Despliegue con Seguridad Automática (RECOMENDADO)

### 🛡️ Sistema de Validación Pre-Despliegue

RSS2 incluye `pre-deploy.sh`, un script inteligente que se ejecuta **antes** de `docker compose` para:

1. ✅ **Validar credenciales**: Asegura que `POSTGRES_PASSWORD`, `REDIS_PASSWORD`, `DB_PASS` estén definidas
2. ✅ **Generar automáticamente**: Crea credenciales seguras de 32 caracteres si faltan
3. ✅ **Prevenir errores**: Evita que los WARN de Docker por variables vacías
4. ✅ **Mostrar resumen**: Te muestra todas las credenciales en consola

### 🚀 Despliegue Rápido y Seguro (3 Comandos)

```bash
# 1. Generar credenciales seguras automáticamente
./pre-deploy.sh --generate

# 2. Validar y desplegar (elige opción 2 para despliegue manual)
./pre-deploy.sh

# 3. Alternativa: Despliegue manual después de validar
./pre-deploy.sh  # ← Elige "2) Solo validar y salir"
docker compose up -d
```

**Flujo recomendado:**
```bash
./pre-deploy.sh --generate   # Genera .env con credenciales seguras
./pre-deploy.sh              # Valida y te pregunta si quieres desplegar
# Si eliges "2): Solo validar", luego:
docker compose up -d         # Despliega manualmente
```

### 📋 Opciones del Script `pre-deploy.sh`

```bash
# Validar credenciales existentes
./pre-deploy.sh

# Generar credenciales seguras automáticamente
./pre-deploy.sh --generate

# Saltar validación (NO RECOMENDADO para producción)
./pre-deploy.sh --skip

# Ver ayuda
./pre-deploy.sh --help
```

**Salida típica del script:**
```
╔═══════════════════════════════════════════════════════════╗
║   🔐 Pre-Deploy Security Check - RSS2 Platform           ║
╚═══════════════════════════════════════════════════════════╝

✅ Archivo .env encontrado
✅ DEFINIDO: POSTGRES_PASSWORD=8x7f2k9m4p1q3w5e
✅ DEFINIDO: REDIS_PASSWORD=a9b8c7d6e5f4g3h2
✅ DEFINIDO: DB_PASS=8x7f2k9m4p1q3w5e

📋 Resumen de Credenciales Activas
   POSTGRES_PASSWORD: ✓
   REDIS_PASSWORD: ✓
   DB_PASS: ✓

✅ VALIDACIÓN COMPLETADA - LISTO PARA DESPLEGAR
```

### 🔒 Variables Críticas que Requieren Contraseñas

| Variable | Descripción | Uso |
|----------|-------------|-----|
| `POSTGRES_PASSWORD` | Contraseña de PostgreSQL | Base de datos |
| `REDIS_PASSWORD` | Contraseña de Redis | Cache y colas |
| `DB_PASS` | Contraseña para workers | Conexiones de workers |
| `SECRET_KEY` | Key secreta de la aplicación | JWT, encriptación |
| `GRAFANA_PASSWORD` | Contraseña de Grafana | Dashboard de monitoring |

---

### 3. Escalado de Workers GPU (¡Importante!)

#### 🚀 Workers de Traducción GPU Multi-Instancia

RSS2 soporta múltiples workers de traducción ejecutándose en paralelo utilizando GPUs, lo que acelera significativamente el proceso de traducción masiva.

**Arquitectura Multi-Worker:**
- Cada worker es una instancia independiente del servicio `ctranslator_worker`
- Los workers compiten automáticamente por las traducciones pendientes usando `SELECT ... FOR UPDATE SKIP LOCKED`
- Cada worker procesa su propio lote (batch) de traducciones sin bloquearse mutuamente
- Soporta hasta 8 workers simultáneos (configurable vía panel admin)

**Requisitos GPU:**
```bash
# Verificar GPU disponibles
nvidia-smi

# Instalar drivers NVIDIA (si no los tienes)
# Ubuntu/Debian:
sudo apt-get update
sudo apt-get install -y nvidia-driver-535
```

**Despliegue con Múltiples Workers GPU:**

```bash
# Opción 1: Desplegar 1 worker GPU (mínimo recomendado)
docker compose up -d --scale translator-gpu=1

# Opción 2: Desplegar 2 workers GPU (recomendado: 16GB+ VRAM total)
docker compose up -d --scale translator-gpu=2

# Opción 3: Desplegar 4 workers GPU (requiere: 32GB+ VRAM total)
docker compose up -d --scale translator-gpu=4

# Opción 4: Combinar CPU y GPU
# 2 workers CPU + 2 workers GPU
docker compose up -d --scale translator=2 --scale translator-gpu=2
```

**Panel de Administración:**
Desde `/admin/settings`, puedes:
- Activar modo GPU (CUDA) o CPU
- Configurar número de workers (1-8)
- Iniciar/detener workers automáticamente

**Configuración de GPU por Worker:**
Cada worker GPU está configurado con:
- `CT2_DEVICE=cuda` - Usa dispositivos CUDA
- `CT2_COMPUTE_TYPE=float16` - Precisión mixta para velocidad
- `TRANSLATOR_BATCH=128` - Tamaño de lote optimizado
- `PYTORCH_CUDA_ALLOC_CONF=max_split_size_mb:512` - Gestión de memoria PyTorch
- `NCCL_DEBUG=INFO` - Debug de comunicación multi-GPU

**Monitorización:**
```bash
# Ver estado de workers
docker compose --profile gpu ps translator-gpu

# Ver uso de GPU
watch -n 1 nvidia-smi

# Ver logs en tiempo real
docker compose logs -f translator-gpu

# Métricas en Grafana
# Dashboard: Translation Workers
# Métricas: translator_*_transitions_per_minute
```

**Performance Esperada:**
| Workers GPU | VRAM Total | Tasa de Traducción |
|-------------|------------|-------------------|
| 1 worker | 8GB+ | ~500 traducciones/min |
| 2 workers | 16GB+ | ~1000 traducciones/min |
| 4 workers | 32GB+ | ~2000 traducciones/min |
| 8 workers | 64GB+ | ~4000 traducciones/min |

---

## 🛡️ Administración y Mantenimiento

### Copias de Seguridad (Backups)

Desde el panel de Administración (`/admin/settings`), puedes realizar:

**Backup Completo (SQL):**
- Descarga un archivo `.sql` con todo el contenido de la base de datos
- Incluye: noticias, feeds, traducciones, usuarios, etiquetas, favoritos, videos, eventos, historial de búsqueda
- Formato: PostgreSQL custom format con datos y estructura
- Tamaño típico: 10-100MB dependiendo del volumen de datos

**Backup de Noticias (ZIP):**
- Genera un archivo comprimido `.zip` con:
  - Tabla `noticias` (artículos completos)
  - Tabla `traducciones` (todas las traducciones)
  - Tablas `tags` y `tags_noticia` (metadatos y relaciones)
- Ideal para: Migraciones de contenido, exportación de datos, backup ligero
- Tamaño típico: 5-50MB (solo contenido de noticias)

### Variables de Entorno Clave (`.env`)
| Variable | Descripción |
|----------|-------------|
| `WIKI_SLEEP` | Tiempo de espera entre peticiones a Wikipedia (evita bloqueos). |
| `SCHEDULER_BATCH`| Cantidad de noticias a enviar a traducir por ciclo. |
| `TARGET_LANGS` | Idiomas destino (ej: `es`). |
| `OLLAMA_URL` | URL del servidor Ollama para categorización. |

---

## 🎯 Descripción de Workers

### rss-ingestor-go
Crawler RSS de alto rendimiento escrito en Go. Gestiona hasta 100 workers paralelos para procesar múltiples feeds simultáneamente. Detecta nuevos artículos, extrae metadatos básicos y los inserta en la base de datos.

### scraper (Go)
Scraper profundo que:
- Descarga el contenido completo de URLs de artículos
- Sanitiza HTML removiendo scripts y estilos
- Extrae texto limpio del body
- Limita el contenido a 20 noticias por ciclo para evitar sobrecarga

### discovery (Go)
Agente de descubrimiento de feeds que:
- Analiza URLs proporcionadas para detectar feeds RSS/Atom
- Soporta hasta 5 feeds por URL
- Ejecuta cada 900 segundos (15 minutos)

### translator (Python/CTranslate2)
Worker de traducción neuronal:
- Modelo: facebook/nllb-200-distilled-600M convertido a CTranslate2
- Dispositivo: CPU (int8)
- Batch: 32 traducciones por ciclo
- Locking: `FOR UPDATE SKIP LOCKED` para evitar procesamiento duplicado
- Soporta múltiples idiomas fuente

### translator-gpu (Python/CTranslate2)
Versión acelerada por GPU del translator:
- Dispositivo: CUDA
- Compute type: float16
- Batch: 128 traducciones por ciclo
- Ideal para procesamiento de alto volumen

### remote-translator (Python)
Worker remoto conectado vía WebSocket:
- Autenticación con API key
- Conexión a `ws://backend-go:8080/ws/worker`
- Modelo: facebook/nllb-200-1.3B (más grande)
- GPU requerida

### langdetect (Python)
Detector de idioma:
- Usa biblioteca `langdetect`
- Proceso: 1000 noticias por ciclo
- Establece lang_from para correcto routing a traducción

### ner (Python/spaCy)
Extracción de entidades nombradas:
- Modelo: es_core_news_lg
- Tipos: PERSON (persona), ORG (organizacion), LOC/GPE (lugar)
- También extrae topics (noun-chunks)
- Configurable via entity_config.json (blacklist, synonyms)

### embeddings (Python)
Generador de embeddings para búsqueda semántica:
- Modelo: paraphrase-multilingual-MiniLM-L12-v2
- Dispositivo: GPU (cuda)
- Genera vectores de 384 dimensiones
- Alimenta Qdrant

### qdrant-worker (Go)
Worker de vectorización Qdrant:
- Conecta a Qdrant (puerto 6333)
- Genera embeddings via Ollama (mxbai-embed-large)
- Sube puntos a collection "news_vectors"
- Actualiza estado en BD (vectorized=true)

### related (Go)
Detector de noticias relacionadas:
- Usa similitud coseno entre embeddings
- Almacena top-k relaciones en tabla related_noticias
- Soporta configuración de threshold mínimo

### wiki-worker (Go)
Enriquecedor de Wikipedia:
- Descarga summaries de Wikipedia API
- Descarga thumbnails de imágenes
- Almacena en `./data/wiki_images`
- Evita rate limiting con WIKI_SLEEP

### llm-categorizer (Python)
Categorizador con LLM:
- Conexión a Ollama
- Usa modelo Mistral para clasificación
- Procesa 10 noticias por batch

### cluster (Python)
Agrupador de noticias por eventos:
- Usa embeddings paraphrase-multilingual-MiniLM-L12-v2
- Threshold: 0.35 de distancia
- Agrupa noticias similares

### translation-scheduler (Python)
Creador de tareas de traducción:
- Query: noticias sin traducción al español
- Crea entradas en tabla traducciones
- Batch: 1000 por ciclo
- Ciclo: 30 segundos

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

## 🧪 Testing

### Backend (Go)
```bash
cd backend && go test ./internal/handlers -v -run TestLogin
cd backend && go test ./internal/auth -v -run TestGenerateToken
```

### Frontend
```bash
cd frontend && npm run test:ui
```

---

## 📁 Estructura de Archivos del Proyecto

```
rss2/
├── backend/               # API REST Go (Gin)
├── frontend/              # Interfaz React + TypeScript
├── workers/               # Workers Python
├── rss-ingestor-go/       # Crawler RSS Go
├── docker-compose.yml     # Orquestación
├── nginx.conf             # Config Nginx
├── monitoring/            # Prometheus + Grafana
├── init-db/               # Migraciones SQL
├── data/                  # Datos persistentes
├── models/                # Modelos ML
├── hf_cache/              # Cache HuggingFace
├── pre-deploy.sh          # Script seguridad
├── generate_secure_credentials.sh
└── AGENTS.md              # Guía desarrollo
```

---

**RSS2** - *Transformando noticias en inteligencia con IA localizada.*