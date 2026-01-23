# RSS2 - Plataforma de Inteligencia de Noticias con IA

RSS2 es una plataforma avanzada de agregación, traducción, análisis y vectorización de noticias diseñada para procesar grandes volúmenes de información en tiempo real. Combina una arquitectura de **microservicios híbrida (Go + Python)** con modelos de **Inteligencia Artificial** locales para transformar flujos RSS crudos en inteligencia accionable, permitiendo búsqueda semántica y análisis de tendencias.

---

## 🏗️ Arquitectura de Servicios (Docker)

El sistema está orquestado mediante Docker Compose y se divide en 3 redes aisladas (`frontend`, `backend`, `monitoring`) para garantizar la seguridad y el rendimiento.

### 🌐 Core & Acceso (Red Frontend)
| Servicio | Tecnología | Puerto Ext. | Descripción |
|----------|------------|-------------|-------------|
| **`nginx`** | Nginx Alpine | **8001** | **Gateway Público**. Proxy inverso que sirve la aplicación y archivos estáticos. |
| **`rss2_web`** | Python (Flask+Gunicorn) | - | Servidor de aplicación principal. Gestiona la API, interfaz web y lógica de negocio. |

### 📥 Ingesta y Descubrimiento (Red Backend)
| Servicio | Tecnología | Descripción |
|----------|------------|-------------|
| **`rss-ingestor-go`** | **Go** | Crawler de ultra-alto rendimiento. Monitoriza y descarga cientos de feeds RSS por minuto. |
| **`url-worker`** | Python | Scraper profundo. Descarga el contenido completo (HTML limpio via `newspaper3k`) de cada noticia. |
| **`url-discovery-worker`**| Python | Agente autónomo que descubre y sugiere nuevos feeds RSS basándose en el tráfico actual. |

### � Procesamiento de IA (Red Backend)
Estos workers procesan asíncronamente la información utilizando modelos locales (GPU/CPU).

| Servicio | Función | Modelo / Tecnología |
|----------|---------|---------------------|
| **`translator`** (x3) | **Traducción Neural** | `NLLB-200`. Traduce noticias de cualquier idioma al Español. Escalado horizontalmente (3 réplicas). |
| **`embeddings`** | **Vectorización** | `Sentence-Transformers`. Convierte texto en vectores matemáticos para búsqueda semántica. |
| **`ner`** | **Entidades** | Modelos SpaCy/Bert. Extrae Personas, Organizaciones y Lugares. |
| **`topics`** | **Clasificación** | Clasifica noticias en temas (Política, Economía, Tecnología, etc.). |
| **`llm-categorizer`** | **Categorización Inteligente** | `ExLlamaV2 + Mistral-7B`. Categoriza noticias usando LLM local. Procesa 10 noticias por lote. |
| **`cluster`** | **Agrupación** | Agrupa noticias sobre el mismo evento de diferentes fuentes. |
| **`related`** | **Relaciones** | Calcula y enlaza noticias relacionadas temporal y contextualmente. |

### 💾 Almacenamiento y Búsqueda (Red Backend)
| Servicio | Rol | Descripción |
|----------|-----|-------------|
| **`db`** | Base de Datos Relacional | **PostgreSQL 18**. Almacenamiento principal de noticias, usuarios y configuración. |
| **`qdrant`** | Base de Datos Vectorial | **Qdrant**. Motor de búsqueda semántica de alta velocidad. |
| **`qdrant-worker`**| Sincronización | Worker dedicado a mantener sincronizados PostgreSQL y Qdrant. |
| **`redis`** | Caché y Colas | **Redis 7**. Gestiona las colas de tareas para los workers y caché de sesión. |

### ⚙️ Orquestación y Mantenimiento
| Servicio | Descripción |
|----------|-------------|
| **`rss-tasks`** | Scheduler (Cron) que ejecuta tareas periódicas de limpieza, mantenimiento y optimización de índices. |

### 📊 Observabilidad (Red Monitoring)
Acceso exclusivo vía localhost o túnel SSH.

| Servicio | Puerto Local | Descripción |
|----------|--------------|-------------|
| **`grafana`** | **3001** | Dashboard visual para monitorizar CPU/RAM, colas de Redis y estado de ingesta. |
| **`prometheus`**| - | Recolección de métricas de todos los contenedores. |
| **`cadvisor`** | - | Monitor de recursos del kernel de Linux para Docker. |

---

## 🚀 Guía de Inicio Rápido

### Requisitos Previos
*   Docker y Docker Compose V2.
*   Drivers de NVIDIA (Opcional, pero recomendado para inferencia rápida de IA).

### 1. Instalación
```bash
git clone <repo>
cd rss2
```

### 2. Configuración de Seguridad
Genera contraseñas robustas automáticamente para todos los servicios:
```bash
./generate_secure_credentials.sh
```
*Esto creará un archivo `.env` configurado y seguro.*

### 3. Iniciar la Plataforma
Utiliza el script de arranque que verifica dependencias y levanta el stack:
```bash
./start_docker.sh
```
*Alternativamente: `docker compose up -d`*

### 4. Acceder a la Aplicación
*   **Web Principal**: [http://localhost:8001](http://localhost:8001)
*   **Monitorización**: [http://localhost:3001](http://localhost:3001) (Usuario: `admin`, Password: ver archivo `.env`)

---

## 🔒 Seguridad y Credenciales (¡IMPORTANTE!)

El sistema viene protegido por defecto. **No existen contraseñas "hardcodeadas"**; todas se generan dinámicamente o se leen del entorno.

### 🔑 Generación de Claves
Al ejecutar `./generate_secure_credentials.sh`, el sistema crea un archivo `.env` que contiene:
1.  **`GRAFANA_PASSWORD`**: Contraseña para el usuario `admin` en Grafana.
2.  **`POSTGRES_PASSWORD`**: Contraseña maestra para la base de datos `rss`.
3.  **`REDIS_PASSWORD`**: Clave de autenticación para Redis.
4.  **`SECRET_KEY`**: Llave criptográfica para sesiones y tokens de seguridad.

**⚠️ Atención:** Si no ejecutas el script, el sistema intentará usar valores por defecto inseguros (ej. `change_this_password`) definidos en `.env.example`. **No uses esto en producción.**

### 🛡️ Niveles de Acceso
1.  **Red Pública (Internet) -> Puerto 8001**:
    *   Solo acceso a **Nginx** (Frontend).
    *   Protegido por las reglas de firewall de tu servidor.
2.  **Red Local (Localhost) -> Puerto 3001**:
    *   Acceso a **Grafana**.
    *   **Login**: Usuario `admin` / Password: Ver `GRAFANA_PASSWORD` en tu archivo `.env`.
3.  **Red Interna (Docker Backend)**:
    *   Base de datos, Redis y Qdrant **NO** están expuestos fuera de Docker.
    *   **Acceso a DB**: Solo posible vía `docker exec` (ver abajo).

### 📋 Auditoría
El repositorio incluye herramientas para verificar la seguridad:
*   `./verify_security.sh`: Ejecuta un escaneo de puertos y configuraciones.
*   `SECURITY_GUIDE.md`: Manual avanzado de administración segura.

---

## �️ Operaciones Comunes

### Ver logs en tiempo real
```bash
# Ver todo el sistema
docker compose logs -f

# Ver un servicio específico (ej. traductor o web)
docker compose logs -f translator
docker compose logs -f rss2_web
```

### Generación de Videos (Nuevo)
El sistema incluye un script para convertir noticias en videos narrados automáticamente:
```bash
# Ejecutar generador manual
python3 scripts/generar_videos_noticias.py
```

### Copias de Seguridad (Backup)
```bash
# Backup de PostgreSQL
docker exec rss2_db pg_dump -U rss rss > backup_full_$(date +%Y%m%d).sql

# Backup de Qdrant (Vectores)
tar -czf vector_backup.tar.gz qdrant_storage/
```

### Reinicio Completo (con reconstrucción)
Si modificas código o configuración:
```bash
docker compose down
docker compose up -d --build
```
