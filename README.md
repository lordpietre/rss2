# RSS2 - Plataforma de Inteligencia de Noticias con IA

RSS2 es una plataforma avanzada de agregación, traducción y análisis de noticias diseñada para procesar grandes volúmenes de información en tiempo real. Utiliza una arquitectura de **microservicios híbrida (Go + Python/FastAPI)** y modelos de **Inteligencia Artificial** locales para transformar flujos RSS crudos en inteligencia accionable.

---

## 🏗️ Arquitectura y Servicios

El sistema se compone de múltiples contenedores Docker orquestados, divididos en 3 redes aisladas (`frontend`, `backend`, `monitoring`) para máxima seguridad.

### 🌐 Core & Frontend
| Servicio | Tecnología | Puerto | Descripción |
|----------|------------|--------|-------------|
| **`nginx`** | Nginx Alpine | **8001** | **Único punto de entrada público**. Reverse proxy, SSL, estáticos. |
| **`rss2_web`** | Python/FastAPI | - | Backend principal. API REST, Jinja2, lógica de negocio. |
| **`rss-web-go`** | Go/Gin | - | (Opcional) Microservicio web de alto rendimiento. |

### 🤖 Inteligencia Artificial & Workers
| Servicio | Descripción | Recursos |
|----------|-------------|----------|
| **`translator`** (x3) | Traducción Neural (NLLB-200) de cualquier idioma a Español. | GPU/CPU |
| **`embeddings`** | Generación de vectores semánticos para búsqueda inteligente. | GPU/CPU |
| **`ner`** | Reconocimiento de Entidades (Personas, Org, Lugares). | CPU |
| **`cluster`** | Agrupación de noticias por eventos similares. | CPU |
| **`topics`** | Clasificación temática de noticias. | CPU |
| **`qdrant-worker`** | Sincronización de vectores con Qdrant. | CPU |

### 📥 Ingesta de Datos
| Servicio | Descripción |
|----------|-------------|
| **`rss-ingestor-go`** | Crawler de alto rendimiento en Go. Descarga cientos de RSS/min. |
| **`url-worker`** | Scraper que descarga y limpia el contenido completo (HTML) de las noticias. |
| **`url-discovery`** | Descubrimiento automático de nuevos feeds RSS. |

### 💾 Almacenamiento de Datos
| Servicio | Tecnología | Descripción |
|----------|------------|-------------|
| **`db`** | PostgreSQL 18 | Base de datos principal (Escritura). Contraseñas fuertes. |
| **`db-replica`** | PostgreSQL 18 | Réplica de lectura (Actualmente en standby). |
| **`qdrant`** | Qdrant | **Base de datos vectorial**. Almacena embeddings para búsqueda semántica. |
| **`redis`** | Redis 7 | Broker de mensajes y caché. **Autenticado**. |

### 📊 Monitorización (Stack de Observabilidad)
| Servicio | Acceso | Descripción |
|----------|--------|-------------|
| **`grafana`** | `localhost:3001` | Dashboard visual de métricas del sistema y contenedores. |
| **`prometheus`** | *Interno* | Recolección de métricas de todos los servicios. |
| **`cadvisor`** | *Interno* | Métricas de uso de recursos de Docker (CPU, RAM). |

---

## 🚀 Instalación y Despliegue

### 1. Clonar y Configurar
```bash
git clone <repo>
cd rss2
```

### 2. Generación de Credenciales Seguras (IMPORTANTE)
El sistema incluye un script para generar contraseñas fuertes automáticamente:
```bash
./generate_secure_credentials.sh
```
Esto creará un archivo `.env` con contraseñas aleatorias para DB, Redis y Grafana.

### 3. Iniciar Servicios
El despliegue se gestiona con el script maestro seguro:
```bash
./migrate_to_secure.sh
```
O manualmente:
```bash
docker-compose up -d
```

### 4. Acceso
*   **Web Pública**: [http://localhost:8001](http://localhost:8001)
*   **Grafana (Monitorización)**: [http://localhost:3001](http://localhost:3001)
    *   *Usuario*: `admin`
    *   *Password*: (Ver archivo `.env` variable `GRAFANA_PASSWORD` o el output del generador)

---

## 🔒 Seguridad

El sistema ha sido auditado y fortificado (Enero 2026):

1.  **Redes Segmentadas**:
    *   `rss2_frontend`: Solo Nginx y Web App.
    *   `rss2_backend`: Base de datos y Workers (sin acceso externo).
    *   `rss2_monitoring`: Stack de observabilidad.

2.  **Puertos Cerrados**:
    *   Qdrant (6333), Prometheus (9090), Redis (6379) NO están expuestos al host.
    *   Solo puerto **8001** (Web) y **3001** (Grafana/Local) están abiertos.

3.  **Autenticación**:
    *   Redis requiere contraseña (`requirepass`).
    *   PostgreSQL usa autenticación estricta.

4.  **Scripts de Seguridad**:
    *   `verify_security.sh`: Ejecuta un test completo de la configuración de seguridad.
    *   `SECURITY_GUIDE.md`: Guía detallada de administración segura.

---

## 🛠️ Comandos de Mantenimiento

### Verificar estado del sistema
```bash
docker-compose ps
```

### Ver logs
```bash
docker-compose logs -f rss2_web    # Web App
docker-compose logs -f translator  # Traductor
```

### Copia de Seguridad (Backup)
```bash
# Backup de Base de Datos
docker exec rss2_db pg_dump -U rss rss > backup_$(date +%Y%m%d).sql

# Backup de Vectores (Qdrant)
# (Detener servicio antes recomendado)
tar -czf qdrant_backup.tar.gz qdrant_storage/
```

### Actualización
```bash
git pull
docker-compose up -d --build
```

---

## 🧠 Características de IA

*   **Búsqueda Semántica**: Encuentra noticias por significado ("conflictos en oriente medio") incluso si no contienen las palabras exactas, gracias a los embeddings vectoriales en Qdrant.
*   **Detección de Idioma**: Automática para dirigir al traductor correcto.
*   **Entidades**: Explorador visual de *quién* y *dónde* en las noticias.

---

## 📂 Estructura de Directorios

*   `/routers`: API Endpoints (Python).
*   `/workers`: Lógica de fondo (Traducción, Ingesta, IA).
*   `/rss-ingestor-go`: Código del crawler en Go.
*   `/monitoring`: Configuración de Prometheus y Grafana.
*   `/templates`: Vistas HTML (Jinja2).
*   `/static`: Assets frontend.
*   `docker-compose.yml`: Definición de infraestructura.
