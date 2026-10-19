# 🌐 RSS2 — Agregador Multilingüe de Noticias RSS

> **Tu窗口 al mundo**: Noticias de miles de fuentes internacionales, traducidas automáticamente al español.

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](./LICENSE)
[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8.svg)](https://golang.org/)
[![React](https://img.shields.io/badge/React-18-61DAFB.svg)](https://reactjs.org/)

---

## 🎯 Qué es RSS2

RSS2 es un **agregador de noticias RSS inteligente** que:

- 🌐 **Recolecta** noticias de miles de fuentes RSS/Atom de todo el mundo
- 🌍 **Detecta** automáticamente el idioma de cada noticia
- 🇪🇸 **Traduce** al español usando NLLB (No Language Left Behind) de Meta
- 👤 **Extrae** personas, organizaciones y temas mencionados (NER)
- 🔗 **Relaciona** noticias similares usando embeddings semánticos
- 📚 **Enriquece** con información de Wikipedia
- 📊 **Muestra** tendencias y alertas de actividad inusual
- ❤️ **Permite** guardar favoritos y crear listas personalizadas

Todo en un **único puerto**: `:8888`

---

## 🚀 Inicio Rápido

### Requisitos

- Docker y Docker Compose
- 8GB+ RAM recomendados
- ~20GB espacio en disco

### Instalación

```bash
# 1. Clonar el repositorio
git clone <repo-url> && cd rss2

# 2. Copiar y configurar variables de entorno
cp .env.example .env
# Editar .env y rellenar:
# - POSTGRES_PASSWORD (contraseña para PostgreSQL)
# - REDIS_PASSWORD (contraseña para Redis)
# - SECRET_KEY (clave secreta para JWT)

# 3. Iniciar la aplicación
docker compose up -d --build backend   # Primero: construir backend
docker compose up -d --no-build       # Luego: iniciar el resto

# 4. Esperar a que esté listo (~5 minutos) y acceder a:
```

🌐 **Frontend**: http://localhost:8888  
📡 **API REST**: http://localhost:8888/api  
❤️ **Favoritos**: Requiere registro/login

---

## 🏗️ Arquitectura

```
                         INTERNET
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                        RSS2 SYSTEM                               │
│                                                                  │
│  ┌──────────────┐      ┌──────────────────────────────────┐    │
│  │   INGESTOR   │────▶│           POSTGRES DB             │    │
│  │     (Go)      │      │    + pgvector (búsqueda vectorial) │    │
│  │  Cada 8 min   │      │                                   │    │
│  └──────────────┘      └──────────────────────────────────┘    │
│                                  │                               │
│                                  ▼                               │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                    WORKERS (en paralelo)                 │   │
│  │                                                           │   │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────────┐     │   │
│  │  │ langdetect │  │translator │  │  embeddings     │     │   │
│  │  │ Detecta    │  │  NLLB-200 │  │  sentence-     │     │   │
│  │  │  idioma    │  │  CPU/GPU  │  │  transformers │     │   │
│  │  └────────────┘  └────────────┘  └────────────────┘     │   │
│  │                                                           │   │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────────┐     │   │
│  │  │    NER     │  │    wiki    │  │   related      │     │   │
│  │  │   spaCy    │  │  Wikipedia │  │   coseno SQL   │     │   │
│  │  │   NER      │  │  resumen   │  │                │     │   │
│  │  └────────────┘  └────────────┘  └────────────────┘     │   │
│  │                                                           │   │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────────┐     │   │
│  │  │  topics    │  │  scraper   │  │  discovery     │     │   │
│  │  │ categoría  │  │enriquece   │  │ nuevos feeds   │     │   │
│  │  │   país     │  │ resumenes  │  │               │     │   │
│  │  └────────────┘  └────────────┘  └────────────────┘     │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                  │                               │
│                                  ▼                               │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │              API Go (Puerto interno :8080)                │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                  │                               │
│                                  ▼                               │
│  ┌──────────────────┐      ┌──────────────────────────────┐    │
│  │   FRONTEND       │◀────│   NGINX GATEWAY (:8888)      │    │
│  │   (React SPA)    │      │   Un único puerto público    │    │
│  └──────────────────┘      └──────────────────────────────┘    │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Componentes Principales

#### 🔙 Backend (Go)

- **API REST** (`backend/cmd/server/`) — Puerto interno :8080
- **Workers Go** compilados como binarios separados:
  - `/scraper` — Enrichment de resúmenes cortos
  - `/discovery` — Descubrimiento de nuevos feeds RSS
  - `/topics` — Categorización y detección de país
  - `/wiki` — Resúmenes e imágenes de Wikipedia
  - `/related` — Cálculo de noticias relacionadas

#### 🎨 Frontend (React)

- **SPA** con Vite + TypeScript + Tailwind CSS
- **Páginas**:
  - Portada con carrusel y tendencias
  - Búsqueda con filtros avanzados
  - Entidades populares (red de personas/orgs)
  - Radar de trends
  - Cobertura global (mapa mundial)
  - Evolución temporal de entidades
  - Alertas de actividad inusual
  - Perfil de usuario con favoritos y listas

#### 🐍 Workers Python

| Worker | Tecnología | Función |
|--------|-----------|---------|
| `langdetect` | langdetect | Detecta idioma de noticias |
| `translation-scheduler` | psycopg | Encola traducciones pendientes |
| `translator` | CTranslate2 + NLLB-200 | Traduce al español |
| `ner` | spaCy (`es_core_news_lg`) | Extrae entidades nombradas |
| `embeddings` | sentence-transformers | Genera embeddings multilingües |

#### 🗄️ Datos

- **PostgreSQL 16** + **pgvector** — Base de datos principal
- **Redis** — Caché y rate limiting
- **nginx** — Gateway API (un solo puerto :8888)

---

## 📊 Pipeline de Datos

```
feeds RSS
    │
    ▼
┌─────────────────┐
│    INGESTOR     │  Cada 8 min: descarga feeds, inserta noticias
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   LANGDETECT    │  Detecta idioma (¿en? ¿ja? ¿ru? ¿pt?...)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ TRANSLATION     │  Crea jobs pendientes en tabla traducciones
│  SCHEDULER     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  TRANSLATOR     │  NLLB-200: traduce contenido → español
│   (CPU/GPU)     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│      NER        │  spaCy: extrae personas, orgs, temas
│    (spaCy)      │
└────────┬────────┘
         │
         ├──────────────────┐
         ▼                  ▼
┌─────────────────┐  ┌─────────────────┐
│   EMBEDDINGS    │  │      WIKI       │
│ sentence-trans.  │  │   Wikipedia     │
└────────┬────────┘  └────────┬────────┘
         │                   │
         ▼                   │
┌─────────────────┐         │
│    RELATED      │         │
│  coseno SQL     │         │
└────────┬────────┘         │
         │                   │
         ▼                   ▼
┌─────────────────────────────────────────┐
│              news_topics                 │
│         categorías + país                 │
└─────────────────────────────────────────┘
```

---

## 🔌 Endpoints Principales

### Noticias

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/news` | Lista de noticias con paginación |
| GET | `/api/news/:id` | Detalle de una noticia |
| GET | `/api/feeds` | Lista de feeds |

### Entidades

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/entities` | Entidades populares |
| GET | `/api/entities/news?valor=&tipo=` | Noticias de una entidad |
| GET | `/api/entities/graph?tipo=` | Grafo de co-ocurrencias |

### Búsqueda

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/search?q=` | Búsqueda con filtros |
| GET | `/api/tags/popular` | Tags más mencionados |

### Estadísticas

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/stats` | Estadísticas globales |
| GET | `/api/countries` | Países disponibles |
| GET | `/api/categories` | Categorías disponibles |

### Usuario

| Método | Ruta | Descripción |
|--------|------|-------------|
| POST | `/api/auth/register` | Registro |
| POST | `/api/auth/login` | Login |
| GET | `/api/favorites` | Favoritos |
| GET | `/api/lists` | Listas del usuario |

### Alertas

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/alerts` | Lista de alertas |
| POST | `/api/alerts/:id/read` | Marcar leída |
| POST | `/api/alerts/read-all` | Marcar todas leídas |

---

## 🛠️ Comandos de Desarrollo

### Go (Backend)

```bash
make all          # Deps + build completo
make deps         # go mod download + tidy
make build        # Compilar binarios → bin/
make clean        # Limpiar bin/
make test         # Tests en backend/ y rss-ingestor-go/
make parity       # Verificar que textclean es idéntico en ambos módulos
```

### Workers Go (local)

```bash
make run-scraper     DB_HOST=localhost DB_PORT=5432 DB_NAME=rss DB_USER=rss DB_PASS=rss
make run-discovery   DB_HOST=localhost DB_PORT=5432 DB_NAME=rss DB_USER=rss DB_PASS=rss
make run-topics      DB_HOST=localhost DB_PORT=5432 DB_NAME=rss DB_USER=rss DB_PASS=rss
make run-related     DB_HOST=localhost DB_PORT=5432 DB_NAME=rss DB_USER=rss DB_PASS=rss RELATED_SLEEP=10
```

### Frontend

```bash
cd frontend
npm install        # Instalar dependencias
npm test           # Tests con vitest
npm run build      # Build producción
npm run dev        # Desarrollo
```

### Docker

```bash
# Primera vez: construir imagen compartida del backend
docker compose build backend

# Luego iniciar todo
docker compose up -d

# Ver logs
docker compose logs -f backend

# Reiniciar un servicio
docker compose restart backend
```

---

## 📁 Estructura del Proyecto

```
rss2/
├── backend/                  # API Go + workers compilados
│   ├── cmd/
│   │   ├── server/          # API REST (main.go)
│   │   ├── scraper/         # Worker: enrichment
│   │   ├── discovery/        # Worker: nuevos feeds
│   │   ├── topics/          # Worker: categorización
│   │   ├── wiki/            # Worker: Wikipedia
│   │   └── related/         # Worker: noticias relacionadas
│   ├── internal/
│   │   ├── handlers/        # Handlers HTTP
│   │   ├── models/          # Modelos de datos
│   │   ├── cache/           # Redis cache
│   │   └── db/              # Conexión PostgreSQL
│   └── Dockerfile
│
├── frontend/                 # SPA React
│   ├── src/
│   │   ├── pages/           # Páginas (Home, Search, etc.)
│   │   ├── components/      # Componentes reutilizables
│   │   ├── services/         # Llamadas API
│   │   └── hooks/           # React hooks
│   └── Dockerfile
│
├── workers/                  # Workers Python
│   ├── ner/                 # Named Entity Recognition
│   ├── langdetect/           # Detección de idioma
│   ├── translation-scheduler/# Scheduler de traducciones
│   └── embeddings/           # Embeddings semánticos
│
├── translator/               # Worker de traducción NLLB
│
├── init-db/                  # Scripts SQL de inicialización
│   ├── 001_init.sql         # Schema principal
│   ├── 002_*.sql            # Migraciones
│   └── ...
│
├── specs/                    # Especificaciones
│   ├── 00-architecture/      # Arquitectura del sistema
│   ├── 01-system-overview/  # Visión general
│   ├── 05-api-backend/     # Contratos API
│   ├── constitution.md       # Reglas obligatorias
│   └── plan.md              # Roadmap
│
├── nginx.conf                # Configuración nginx
├── docker-compose.yml        # Orquestación Docker
├── Makefile                  # Comandos de desarrollo
├── .env.example              # Variables de entorno ejemplo
└── README.md                # Este archivo
```

---

## 🔧 Configuración

### Variables de Entorno

| Variable | Descripción | Requerido |
|---------|-------------|-----------|
| `POSTGRES_PASSWORD` | Contraseña PostgreSQL | ✅ |
| `REDIS_PASSWORD` | Contraseña Redis | ✅ |
| `SECRET_KEY` | Clave para JWT | ✅ |
| `DB_HOST` | Host PostgreSQL (default: db) | No |
| `DB_PORT` | Puerto PostgreSQL (default: 5432) | No |
| `REDIS_URL` | URL Redis (default: redis://:pwd@redis:6379) | No |

### Perfiles Docker Compose

```bash
# Perfil por defecto (CPU)
docker compose up -d

# Con GPU (traductor NVIDIA)
docker compose --profile gpu up -d

# Con monitoreo (Prometheus + Grafana)
docker compose --profile monitoring up -d
```

---

## 🐛 Solución de Problemas

### El backend no inicia ("unhealthy")

El servidor tarda ~5 minutos en iniciar porque:
1. Crea índices en tablas grandes (1.4M+ entidades)
2. Refresca estadísticas de entidades

Espera al menos 5 minutos después de `docker compose up`.

### "No space left on device"

Aumenta `shm_size` en `docker-compose.yml`:

```yaml
db:
  shm_size: '512m'  # Antes: 256m
```

### Stats no aparecen

Limpia el caché de nginx:

```bash
docker compose restart nginx
```

### Queries lentas

Verifica queries lentas en PostgreSQL:

```bash
docker exec rss2_db psql -U rss -d rss -c \
  "SELECT pid, now() - query_start AS duration, query FROM pg_stat_activity WHERE state = 'active' AND now() - query_start > '5 seconds';"
```

---

## 📜 Licencia

MIT License — ver [LICENSE](./LICENSE) para detalles.

---

## 🤝 Contribuir

1. Lee [specs/constitution.md](./specs/constitution.md) — reglas obligatorias
2. Crea spec antes de código — ver [specs/00-architecture/05-spec-driven-development.md](./specs/00-architecture/05-spec-driven-development.md)
3. Haz tests antes de commit
4. Asegura `make parity` pasa (textclean debe ser idéntico en backend/ y rss-ingestor-go/)

---

## 📚 Documentación Adicional

- [DEPLOY.md](./DEPLOY.md) — Guía de despliegue detallada
- [diagrama.md](./diagrama.md) — Diagramas de arquitectura
- [explication.md](./explication.md) — Explicación técnica profunda
- [specs/](./specs/) — Especificaciones completas del sistema
- [docs/](./docs/) — Documentación adicional

---

<p align="center">
  <strong>RSS2</strong> — Tu ventana al mundo · Hecho con ❤️ y Go
</p>
