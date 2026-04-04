# AGENTS.md - RSS2 Development Guide

## 🛠️ Build & Test Commands

### Backend (Go)
```bash
cd backend && go mod tidy
cd backend && go build -o ../bin/server ./cmd/server
cd rss-ingestor-go && go build -o ../bin/rss-ingestor .

# Single test
cd backend && go test ./internal/handlers -v -run TestLogin
cd backend && go test ./internal/auth -v -run TestGenerateToken
```

### Frontend
```bash
cd frontend && npm install
cd frontend && npm run dev      # Development
cd frontend && npm run build    # Production build
cd frontend && npm test         # Run tests
cd frontend && npm run test:ui  # UI mode
```

### Makefile
```bash
make build          # Build all binaries
make clean          # Remove binaries
make docker-build   # Build Docker images
```

---

## 📁 Project Structure
```
rss2/
├── backend/                    # Go API server + Workers
│   ├── cmd/
│   │   ├── server/main.go       # API REST principal (Gin)
│   │   ├── wiki_worker/main.go  # Wikipedia integration
│   │   ├── qdrant/main.go       # Vector indexing worker
│   │   ├── related/main.go      # Related news worker
│   │   ├── topics/main.go       # Country/topic matcher
│   │   ├── scraper/main.go      # Deep scraping worker
│   │   ├── discovery/main.go    # RSS feed discovery
│   │   └── topics/main.go       # Topic matching
│   └── internal/
│       ├── handlers/            # HTTP endpoints
│       ├── models/              # Data models
│       ├── auth/                # JWT authentication
│       ├── middleware/         # CORS, Auth middleware
│       ├── services/            # ML services (Translate, Embeddings, NER, Semantic Search)
│       ├── db/                  # PostgreSQL connection
│       ├── cache/              # Redis connection
│       └── config/             # Configuration loading
├── frontend/                   # React + TypeScript + Vite
│   └── src/
│       ├── pages/              # Home, News, Search, Admin, Feeds, Stats
│       ├── components/        # Layout, UI components
│       └── services/           # API client
├── workers/                   # Python workers
│   ├── ctranslator_worker.py  # NLLB-200 translation (CTranslate2)
│   ├── ner_worker.py          # Spacy NER + Topic extraction
│   ├── embeddings_worker.py   # Sentence transformers embeddings
│   ├── cluster_worker.py      # News clustering
│   ├── langdetect_worker.py   # Language detection
│   ├── llm_categorizer_worker.py   # Ollama LLM categorization
│   ├── simple_categorizer_worker.py
│   ├── simple_translator.py
│   ├── simple_translator_worker.py
│   ├── translation_worker.py
│   ├── translation_scheduler.py
│   └── remote_translator_worker.py  # Remote GPU worker via WebSocket
├── data/                      # PostgreSQL, Redis, Qdrant data
├── models/                    # ML models (nllb-ct2)
├── hf_cache/                  # HuggingFace cache
├── init-db/                   # SQL migrations
├── monitoring/                # Prometheus + Grafana config
├── rss-ingestor-go/           # RSS crawler (Go)
└── docker-compose.yml         # Full stack orchestration
```

---

## 🚀 Servicios del Sistema

### Capa de Acceso y API (Puerto 8888)
| Servicio | Tecnología | Descripción |
|---------|------------|-------------|
| **nginx** | Nginx Alpine | Gateway y Proxy Inverso |
| **rss2_frontend** | React + Vite | Interfaz web responsiva |
| **backend-go** | Go + Gin | API REST principal |

### Ingesta y Descubrimiento (Go)
| Servicio | Tecnología | Descripción |
|---------|------------|-------------|
| **rss-ingestor-go** | Go | Crawler RSS de alto rendimiento |
| **scraper** | Go | Scraper profundo con sanitización HTML |
| **discovery** | Go | Agente de descubrimiento de feeds RSS |

### Procesamiento de Datos e IA
| Servicio | Tecnología | Descripción |
|---------|------------|-------------|
| **translator** | NLLB-200 (CPU) | Traducción neuronal CTranslate2 |
| **translator-gpu** | NLLB-200 (GPU) | Traducción acelerada CUDA |
| **remote-translator** | WebSocket | Worker GPU remoto |
| **embeddings** | S-Transformers | Generación de vectores semánticos |
| **ner** | Spacy + BERT | Reconocimiento de entidades (PER, ORG, LOC) |
| **llm-categorizer** | Ollama/Mistral | Clasificación con modelos de lenguaje |
| **wiki-worker** | Go | Integración Wikipedia + thumbnails |
| **topics** | Go | Matcher de países y temas |
| **related** | Go | Detección de noticias relacionadas |
| **qdrant-worker** | Go | Vectorización + búsqueda semántica |
| **cluster** | Python | Agrupación de noticias |
| **langdetect** | Python | Detección de idioma |
| **translation-scheduler** | Python | Creador de tareas de traducción |

### Capa de Almacenamiento
| Servicio | Tecnología | Descripción |
|---------|------------|-------------|
| **db** | PostgreSQL 18 | Base de datos relacional |
| **qdrant** | Qdrant | Base de datos vectorial |
| **redis** | Redis 7 | Cache y colas de mensajes |

### Monitoreo
| Servicio | Tecnología | Descripción |
|---------|------------|-------------|
| **prometheus** | Prometheus | Métricas del sistema |
| **grafana** | Grafana | Dashboard (puerto 3001) |
| **cadvisor** | cAdvisor | Monitoreo Docker |

---

## 📝 Code Style Guidelines

### Backend (Go)

#### Imports
```go
package handlers

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/rss2/backend/internal/auth"
    "github.com/rss2/backend/internal/models"
)
```
**Order:** Standard → Third-party → Local packages
**Use:** `goimports` to auto-format

#### Naming
- Packages: lowercase (`handlers`, `services`)
- Functions/Methods: camelCase (`GetNews`, `CreateUser`)
- Variables: camelCase (`userId`, `newsList`)
- Constants: UPPER_SNAKE_CASE (`MaxPageSize`)
- Types: PascalCase (`NewsResponse`, `User`)

#### Error Handling
```go
func CreateResource(c *gin.Context) {
    var req CreateRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, models.ErrorResponse{
            Error:   "Invalid request",
            Message: err.Error(),
        })
        return
    }
    // ...
}
```
**Always return errors** | **Use `models.ErrorResponse`**

#### Database
```go
err := db.GetPool().QueryRow(ctx, "SELECT * FROM users WHERE id = $1", id).Scan(&user)
if err != nil {
    c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "User not found"})
    return
}
```
**Use context** | **Named parameters ($1, $2)**

#### Tests
```go
func TestLoginInvalidRequest(t *testing.T) {
    router := gin.New()
    router.POST("/auth/login", Login)
    body := []byte(`{}`)
    req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(body))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    if w.Code != http.StatusBadRequest {
        t.Errorf("expected 400, got %d", w.Code)
    }
}
```
**Use `gin.TestMode`** | **Test error paths**

---

### Frontend (TypeScript + React)

#### Imports
```tsx
import React, { useState, useEffect } from 'react'
import { Routes, Route } from 'react-router-dom'
import { Layout } from './components/layout/Layout'
import { api } from './services/api'
```
**Order:** React → Router → Components → Services → Utils

#### Components
```tsx
function NewsList() {
  const [news, setNews] = useState<News[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => { fetchNews(); }, [])

  const fetchNews = async () => {
    try {
      const res = await api.get('/news')
      setNews(res.data)
    } catch (err) {
      setError(err.message)
    }
  }

  return (
    <div className="news-list">
      {loading && <Spinner />}
      {error && <ErrorBanner message={error} />}
    </div>
  )
}
```

#### Types
```tsx
interface News {
  id: number
  title: string
  summary: string
  url: string
  publishedAt: string
}
```

#### Styling
- Use Tailwind CSS utility classes
- Avoid inline styles
- Use `clsx` for conditional classes

#### TypeScript Rules
- Use `strict: true` mode
- Avoid `any` - define interfaces
- Use type guards for narrowing
- Prefer optional chaining `?.`

---

## 🔒 Security Guidelines

1. **Never commit secrets** - Use `.env` in `.gitignore`
2. **Validate all inputs** - Use `go-playground/validator`
3. **Prepared statements** - Prevent SQL injection
4. **Rate limiting** - On sensitive endpoints
5. **HTTPS only** - Enforce in production

### Variables de Entorno Críticas
```bash
POSTGRES_PASSWORD  # Contraseña PostgreSQL
REDIS_PASSWORD     # Contraseña Redis
DB_PASS            # Contraseña para workers
SECRET_KEY         # Key JWT
GRAFANA_PASSWORD   # Dashboard password
```

---

## 🧪 Testing Best Practices

### Backend
- Test error cases and edge cases
- Use `gin.TestMode` for HTTP tests

### Frontend
- Test component rendering
- Test API integration
- Test error states
- Use vitest

---

## 📦 Deployment

```bash
# Generar credenciales seguras
./pre-deploy.sh --generate

# Validar y desplegar
./pre-deploy.sh

# O manualmente
docker compose up -d

# Escalar workers de traducción
docker compose up -d --scale translator-gpu=4
```

### Escalado de Workers GPU
```bash
# 1 worker GPU (8GB+ VRAM)
docker compose up -d --scale translator-gpu=1

# 2 workers GPU (16GB+ VRAM)
docker compose up -d --scale translator-gpu=2

# 4 workers GPU (32GB+ VRAM)
docker compose up -d --scale translator-gpu=4
```

---

## 🔧 Configuración de Workers

### Environment Variables Principales

| Variable | Descripción | Default |
|----------|-------------|---------|
| `DB_HOST` | Host PostgreSQL | localhost |
| `DB_PORT` | Puerto PostgreSQL | 5432 |
| `DB_NAME` | Nombre base de datos | rss |
| `DB_USER` | Usuario PostgreSQL | rss |
| `DB_PASS` | Contraseña PostgreSQL | - |
| `TARGET_LANGS` | Idiomas destino | es |
| `TRANSLATOR_BATCH` | Tamaño de batch | 32 |
| `CT2_DEVICE` | Dispositivo (cpu/cuda) | cpu |
| `CT2_COMPUTE_TYPE` | Tipo (int8/float16) | int8 |
| `NER_BATCH` | Batch NER | 64 |
| `EMB_BATCH` | Batch embeddings | 64 |

---

## 📊 Endpoints de API Principales

### News
- `GET /api/news` - Listar noticias (paginado, filtros)
- `GET /api/news/:id` - Ver noticia con entidades
- `DELETE /api/news/:id` - Eliminar noticia (admin)

### Feeds
- `GET /api/feeds` - Listar feeds
- `POST /api/feeds` - Crear feed (auth)
- `PUT /api/feeds/:id` - Actualizar feed (auth)
- `DELETE /api/feeds/:id` - Eliminar feed (auth)

### Search
- `GET /api/search?q=...` - Búsqueda texto
- `GET /api/search?q=...&semantic=true` - Búsqueda semántica

### Entities
- `GET /api/entities?tipo=persona` - Listar entidades (PER, ORG, LOC)

### Admin
- `GET /api/admin/backup` - Backup SQL completo
- `GET /api/admin/backup/news` - Backup noticias (ZIP)
- `GET /api/admin/users` - Listar usuarios
- `POST /api/admin/workers/start` - Iniciar workers traducción
- `POST /api/admin/workers/stop` - Detener workers traducción
- `GET /api/admin/workers/status` - Estado de workers

### Auth
- `POST /api/auth/login` - Iniciar sesión
- `POST /api/auth/register` - Registrarse
- `GET /api/auth/me` - Usuario actual (auth)

### Stats
- `GET /api/stats` - Estadísticas globales

---

## 🎯 Capabilities del Sistema

1. **Enriquecimiento Wikipedia**: Detecta personas/orgs, descarga biografías e imágenes
2. **Categorización LLM**: Clasificación con Mistral-7B vía Ollama
3. **Búsqueda Semántica**: Qdrant vector search con mxbai-embed-large
4. **Traducción Neuronal**: NLLB-200 (600M-1.3B params) CPU/GPU
5. **NER**: Spacy es_core_news_lg para entidades nombradas
6. **Noticias Relacionadas**: Similitud coseno entre embeddings
7. **Detección de Idioma**: langdetect
8. **Clustering**: Agrupación automática de noticias
9. **WebSocket Workers**: Workers GPU remotos conectados por WS
10. **Backup Automático**: pg_dump con compresión ZIP

---

## 📖 Documentación Adicional

- [README.md](../README.md) - Guía de despliegue completo
- [DEPLOY.md](./DEPLOY.md) - Instrucciones de producción
- [SECURITY_GUIDE.md](./SECURITY_GUIDE.md) - Guía de seguridad
- [QUICKSTART_LLM.md](./QUICKSTART_LLM.md) - Configuración LLM
- [remote-worker.md](./remote-worker.md) - Workers remotos