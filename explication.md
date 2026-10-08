# RSS2 - Agregador Multilingüe de Noticias RSS

## Resumen Ejecutivo

**RSS2** es un agregador de noticias RSS que ingiere feeds de todo el mundo, detecta el idioma automáticamente, traduce al español usando NLLB (No Language Left Behind), extrae entidades nombradas (NER), calcula embeddings locales, detecta tópicos, busca noticias relacionadas, enriquece con Wikipedia y genera alertas cuando hay picos de actividad.

**Puertos expuestos**: Solo `:8888` (nginx gateway). Todo el tráfico pasa por nginx.

---

## Tecnologías

### Backend
- **Go 1.23** con Gin framework
- **PostgreSQL 16** - Base de datos principal
- **Redis 7** - Caché y rate limiting
- **Gin** - HTTP framework
- **pgx** - Driver PostgreSQL

### Workers (Python)
- **Python 3** workers para tareas específicas
- **spaCy** - NER (Named Entity Recognition) con `es_core_news_lg`
- **CTranslate2** - Traducción NLLB-200-distilled-600M (CPU)
- **sentence-transformers** - Embeddings multilingües (MiniLM-L12-v2)

### Frontend
- **React 18** con TypeScript
- **Vite** - Bundler
- **Tailwind CSS** - Estilos
- **React Router** - Navegación
- **TanStack Query** - Gestión de estado servidor
- **Axios** - Cliente HTTP

### Infrastructure
- **Docker + Docker Compose** - Orquestación
- **nginx:alpine** - API Gateway y proxy reverso
- **Prometheus + Grafana** - Monitoreo (opcional)

---

## Estructura de Directorios

```
/home/x/rss2/
├── backend/                    # API Go y workers compilados
│   ├── cmd/
│   │   ├── server/           # API principal (:8080)
│   │   ├── scraper/         # Worker de scraping
│   │   ├── discovery/       # Worker de descubrimiento de feeds
│   │   ├── wiki-worker/     # Worker de Wikipedia
│   │   ├── topics/          # Worker de tópicos
│   │   ├── related/         # Worker de noticias relacionadas
│   │   ├── sanitize/        # Limpiador de texto
│   │   └── entityscan/      # Verificador de entidades
│   ├── internal/
│   │   ├── handlers/       # Controladores HTTP (favorites, news, feeds, etc.)
│   │   ├── models/          # Modelos de datos Go
│   │   ├── db/              # Conexión PostgreSQL
│   │   ├── cache/           # Cache Redis
│   │   ├── middleware/       # Auth, rate limiting, CORS
│   │   └── logger/          # Logging estructurado (zerolog)
│   └── docs/                # Swagger docs
├── frontend/                  # SPA React
│   ├── src/
│   │   ├── pages/          # Páginas (Home, News, Search, Favorites, Feeds, etc.)
│   │   ├── components/      # Componentes UI
│   │   │   ├── layout/     # Layout principal con navbar
│   │   │   └── ui/         # Componentes reutilizables
│   │   ├── hooks/          # Custom hooks (useApiFavorites, etc.)
│   │   └── services/        # Cliente API
│   └── Dockerfile
├── rss-ingestor-go/        # Ingestor RSS en Go (separado por rendimiento)
│   ├── main.go
│   └── textclean/          # Limpiador de texto HTML
├── workers/                  # Workers Python legacy
│   ├── ner_worker.py       # NER con spaCy
│   ├── ctranslator_worker.py # Traductor CTranslate2
│   └── langdetect_worker.py # Detección de idioma
├── translator/               # Workers de traducción Python
│   └── workers/
│       ├── ctranslator_worker.py
│       └── translation_scheduler.py
├── init-db/                 # Migraciones SQL del schema
│   ├── 00-complete-schema.sql
│   ├── 01.schema.sql
│   ├── 03-categorias.sql
│   └── ...
├── migrations/              # Migraciones adicionales
├── specs/                   # Documentación de arquitectura
│   ├── 00-architecture/
│   ├── 01-system-overview/
│   └── ...
├── nginx.conf              # Configuración nginx
├── docker-compose.yml       # Orquestación principal
└── .env.example           # Variables de entorno
```

---

## Contenedores Docker

### Servicios Principales

| Contenedor | Imagen | Función | Puerto |
|------------|--------|---------|--------|
| `nginx` | nginx:alpine | API Gateway + frontend estático | `:8888` |
| `backend` | rss2-backend | API Go (Gin) | `:8080` (interno) |
| `frontend` | rss2-frontend | SPA React servida por nginx | `:80` (interno) |
| `db` | postgres:16-alpine | PostgreSQL | `:5432` (interno) |
| `redis` | redis:7-alpine | Caché y rate limiting | `:6379` (interno) |
| `ingestor` | rss2-ingestor | Ingesta RSS feeds | - |

### Workers (reutilizan imagen `rss2-backend` con distintos entrypoints)

| Contenedor | Entrypoint | Función |
|------------|------------|---------|
| `translator` | `python -m workers.ctranslator_worker` | Traduce a español |
| `translator-2` | `python -m workers.ctranslator_worker` | Traductor secundario |
| `langdetect` | `python workers/langdetect_worker` | Detecta idioma |
| `ner` | `python workers/ner_worker` | Extrae entidades (spaCy) |
| `embeddings` | `python workers/embeddings_worker` | Calcula embeddings |
| `topics` | `/topics` | Clasifica por tópicos |
| `wiki` | `/wiki-worker` | Enriquece con Wikipedia |
| `scraper` | `/scraper` | Scraping de resúmenes |
| `discovery` | `/discovery` | Descubre feeds RSS |
| `related` | `/related` | Busca noticias relacionadas |

### Monitoreo (profile `monitoring`)

| Contenedor | Función | Puerto |
|------------|---------|--------|
| `prometheus` | Métricas | `:9090` |
| `grafana` | Dashboards | `:3000` |
| `cadvisor` | Uso de contenedores | `:8080` |

---

## Modelo de Datos (PostgreSQL)

### Tablas Principales

```sql
-- Noticias
noticias (
  id VARCHAR(32) PRIMARY KEY,    -- ID del feed + hash URL
  titulo TEXT,
  resumen TEXT,
  url TEXT UNIQUE,
  fecha TIMESTAMP,
  imagen_url TEXT,
  fuente_nombre VARCHAR(255),
  categoria_id INTEGER REFERENCES categorias(id),
  pais_id INTEGER REFERENCES paises(id),
  lang CHAR(5),                -- Código ISO idioma
  tsv TSVECTOR,               -- Búsqueda full-text
  topics_processed BOOLEAN
)

-- Traducciones
traducciones (
  id SERIAL PRIMARY KEY,
  noticia_id VARCHAR(32) REFERENCES noticias(id),
  lang_from CHAR(5),
  lang_to CHAR(5) NOT NULL,    -- Siempre 'es' para traducciones
  titulo_trad TEXT,
  resumen_trad TEXT,
  status VARCHAR(16),           -- 'pending', 'done', 'error'
  locked_at TIMESTAMP
)

-- Entidades/Tags
tags (
  id SERIAL PRIMARY KEY,
  valor VARCHAR(255) NOT NULL,
  tipo VARCHAR(32),            -- 'persona', 'organizacion', 'lugar', 'tema'
  apellido VARCHAR(100),
  UNIQUE(valor, tipo)
)

tags_noticia (
  tag_id INTEGER REFERENCES tags(id),
  noticia_id VARCHAR(32) REFERENCES noticias(id),
  traduccion_id INTEGER REFERENCES traducciones(id)
)

-- Tópicos
topics (
  id SERIAL PRIMARY KEY,
  nombre VARCHAR(100) UNIQUE,
  weight REAL,
  keywords TEXT
)

news_topics (
  noticia_id VARCHAR(32) REFERENCES noticias(id),
  topic_id INTEGER REFERENCES topics(id),
  confidence REAL
)

-- Favoritos y Listas
user_favorites (
  user_id INTEGER REFERENCES users(id),
  noticia_id VARCHAR(32),
  UNIQUE(user_id, noticia_id)
)

user_lists (
  id SERIAL PRIMARY KEY,
  user_id INTEGER REFERENCES users(id),
  name VARCHAR(255),
  keywords TEXT DEFAULT ''       -- Búsqueda automática
)

user_list_items (
  list_id INTEGER REFERENCES user_lists(id),
  noticia_id VARCHAR(32),
  UNIQUE(list_id, noticia_id)
)

-- Feeds RSS
feeds (
  id SERIAL PRIMARY KEY,
  nombre VARCHAR(255),
  url TEXT UNIQUE,
  categoria_id INTEGER,
  pais_id INTEGER,
  idioma CHAR(2),
  activo BOOLEAN DEFAULT TRUE,
  fallos INTEGER DEFAULT 0,
  last_error TEXT
)

-- Búsquedas guardadas
user_saved_searches (
  user_id INTEGER REFERENCES users(id),
  type VARCHAR(20),            -- 'news' o 'entity'
  label VARCHAR(255),
  params JSONB
)

-- Alertas
alertas (
  valor VARCHAR(255),
  tipo VARCHAR(32),
  hits INTEGER,
  baseline REAL,
  ratio REAL,
  status VARCHAR(16),          -- 'nueva', 'leida', 'descartada'
  periodo TIMESTAMP
)
```

---

## API Endpoints

### Autenticación
| Método | Ruta | Descripción |
|--------|------|-------------|
| POST | `/api/auth/register` | Registro |
| POST | `/api/auth/login` | Login (retorna JWT) |
| GET | `/api/auth/me` | Usuario actual |

### Noticias
| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/news` | Lista noticias (filtros: q, category_id, country_id, translated_only) |
| GET | `/api/news/:id` | Detalle noticia |
| DELETE | `/api/news/:id` | Eliminar (admin) |
| GET | `/api/search` | Búsqueda full-text |

### Feeds
| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/feeds` | Lista feeds (filtros: activo, categoria, pais) |
| POST | `/api/feeds` | Crear feed (auth) |
| PUT | `/api/feeds/:id` | Actualizar (auth) |
| DELETE | `/api/feeds/:id` | Eliminar (auth) |
| DELETE | `/api/feeds/inactive` | Eliminar inactivos (auth) |
| POST | `/api/feeds/:id/toggle` | Activar/desactivar (auth) |

### Entidades y Alertas
| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/entities` | Lista entidades populares |
| GET | `/api/entities/news` | Noticias de una entidad |
| GET | `/api/alerts` | Lista alertas |
| POST | `/api/alerts/:id/read` | Marcar leída |

### Favoritos y Listas
| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/favorites` | Favoritos del usuario |
| POST | `/api/favorites/:noticiaId` | Añadir favorito |
| DELETE | `/api/favorites/:noticiaId` | Quitar favorito |
| GET | `/api/lists` | Listas del usuario |
| POST | `/api/lists` | Crear lista |
| PUT | `/api/lists/:id` | Renombrar/actualizar keywords |
| GET | `/api/lists/:id/items` | Items de lista |
| GET | `/api/lists/:id/tags` | Tags de la lista (nube) |
| GET | `/api/lists/:id/related` | Noticias relacionadas (auto-búsqueda) |
| GET | `/api/lists/suggested` | Noticias sugeridas para sidebar |

### Admin
| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/admin/users` | Lista usuarios |
| POST | `/api/admin/aliases` | Crear alias |
| GET | `/api/admin/ingest/stats` | Stats de ingestión |
| POST | `/api/admin/backup/news` | Backup |
| POST | `/api/admin/restore` | Restaurar |

---

## Flujo de Datos

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              INGESTIÓN                                      │
└─────────────────────────────────────────────────────────────────────────────┘

feeds RSS ──▶ ingestor (Go, cada 15 min) ──▶ noticias

┌─────────────────────────────────────────────────────────────────────────────┐
│                         ENRIQUECIMIENTO                                     │
└─────────────────────────────────────────────────────────────────────────────┘

noticias ──▶ langdetect (detecta idioma) ──▶ translation-scheduler ──▶ translator

noticias ──▶ ner (spaCy) ──▶ tags/tags_noticia

traducciones (done) ──▶ embeddings (sentence-transformers) ──▶ traduccion_embeddings

noticias ──▶ topics ──▶ news_topics

noticias ──▶ wiki-worker ──▶ tags.wiki_summary, tags.wiki_url, tags.image_path

┌─────────────────────────────────────────────────────────────────────────────┐
│                            BÚSQUEDA                                        │
└─────────────────────────────────────────────────────────────────────────────┘

noticias.tsv ──▶ to_tsquery('spanish', q) ──▶ SearchNews

traduccion_embeddings ──▶ coseno similarity ──▶ related_noticias

┌─────────────────────────────────────────────────────────────────────────────┐
│                           ALERTAS                                           │
└─────────────────────────────────────────────────────────────────────────────┘

tags_noticia ──▶ scanner (cada hora) ──▶ alertas (picos de actividad)

┌─────────────────────────────────────────────────────────────────────────────┐
│                         USUARIO                                             │
└─────────────────────────────────────────────────────────────────────────────┘

user_favorites ──▶ favoritos ──▶ multi-device sync

user_lists ──▶ keywords ──▶ auto-búsqueda related news

user_saved_searches ──▶ búsquedas guardadas
```

---

## Variables de Entorno Principales

### Base de Datos
```env
POSTGRES_DB=rss
POSTGRES_USER=rss
POSTGRES_PASSWORD=<password>
```

### Redis
```env
REDIS_PASSWORD=<password>
```

### Traductor
```env
MAX_SRC_TOKENS=1024      # Tokens máx texto origen
MAX_NEW_TOKENS=1024      # Tokens máx traducción
TRANSLATOR_BATCH=128     # Tamaño de lote
```

### Ingestor
```env
RSS_POKE_INTERVAL_MIN=15  # Intervalo de ingestión (min)
RSS_MAX_WORKERS=20       # Workers paralelos
```

### NER
```env
NER_LANG=es
NER_MODEL=es_core_news_lg
```

### Embeddings
```env
EMB_MODEL=sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2
EMB_BATCH=128
EMB_LANGS=es
```

---

## Workers - Detalle

### Translator (CTranslate2 + NLLB)
- Recibe jobs del `translation-scheduler`
- Traduce a español (`lang_to = 'es'`)
- Supporta ~100 idiomas fuente
- Batching automático por idioma

### Langdetect
- Detecta idioma de `noticias.resumen`
- Ignora si ya tiene `lang` o si está en lista negra
- Solo procesa resúmenes < 200 caracteres

### NER (spaCy)
- Extrae personas, organizaciones, lugares, temas
- Blocklist para reducir falsos positivos
- Actualiza `entity_aliases` para variantes

### Embeddings
- Modelo multilingüe MiniLM-L12-v2
- Genera vectores de 384 dimensiones
- Almacena en `traduccion_embeddings`

### Wiki Worker
- Enrich tags con Wikipedia (summary, URL, imagen)
- Respetando entity_aliases canonicales

### Topics Worker
- Clasifica noticias en tópicos configurados
- Keywords match con umbrales de weight

### Scraper
- Enriquece resúmenes cortos (<200 chars)
- Fallback si no tiene contenido

### Discovery
- Descubre feeds RSS en URLs de `fuentes_url`
- Detecta país e idioma automáticamente

### Related
- Busca noticias relacionadas por coseno
- Usa embeddings de traducciones

---

## Autenticación

- **JWT** (JSON Web Tokens)
- Token en header `Authorization: Bearer <token>`
- middleware `AuthRequired()` protege endpoints de usuario
- middleware `AdminRequired()` para operaciones admin
- Rate limiting por IP y por usuario

---

## Caché (Redis)

| Key Pattern | TTL | Contenido |
|-------------|-----|-----------|
| `news:*` | 30s | News query results |
| `feeds:*` | 5min | Feeds list |
| `stats` | 30min | Global stats |
| `categories` | 1h | Categories list |
| `countries` | 1h | Countries list |

---

## Rate Limiting

| Endpoint | Límite |
|----------|--------|
| `/api/auth/*` | 10/min |
| `/api/*` | 60/min |
| `/api/search` | 30/min |

---

## Comandos Útiles

```bash
# Iniciar todo
docker compose up -d

# Ver logs de un servicio
docker compose logs -f backend

# Reiniciar un worker
docker compose restart translator

# Ver estado
docker compose ps

# Ver métricas
curl http://localhost:8888/api/stats

# Ver jobs de traducción
curl -H "Authorization: Bearer <token>" http://localhost:8888/api/admin/workers/stats

# Backup manual
docker exec rss2_backend backup_news_zipped

# Restaurar backup
# docker exec -it rss2_backend restore_database <file.zip>
```

---

## Características Especiales

### Limpieza de Texto
- Elimina Chrome/Taboola residual
- Detecta y limpia JavaScript embebido
- Chunking inteligente para textos largos
- Truncamiento en límites de oración (no bytes)

### Full-Text Search
- Usa `to_tsvector('spanish', ...)` con stemming
- Búsqueda en título (weight A) y resumen (weight B)
- Index GIN para rendimiento

### Detección de Entidades
- spaCy con modelo `es_core_news_lg`
- Alias entity para variaciones (Trump/TRUMP/Donald Trump)
- Blocklist global para falsos positivos comunes

### Alertas de Picos
- Compara mentions por hora vs baseline histórico
- Ratio > 5x + hits >= 5 + baseline >= 1 = alerta
- Cooldown de 6 horas entre alertas del mismo tag

---

## Líneas de Código Aproximadas

| Componente | Lenguaje | Líneas |
|------------|----------|--------|
| backend | Go | ~15,000 |
| frontend | TypeScript/React | ~10,000 |
| rss-ingestor-go | Go | ~2,000 |
| workers | Python | ~5,000 |
| **Total** | | **~32,000** |

---

## Puertos y Servicios Externos

| Puerto | Servicio | Acceso |
|--------|----------|--------|
| 8888 | nginx (API + Frontend) | Público |
| 9090 | Prometheus | Interno |
| 3000 | Grafana | Interno |
| 8080 | backend API | Solo interno (via nginx) |

No hay puertos exposés directos a servicios externos (Twitter, etc.) - todas las conexiones son entrantes o hacia APIs externas vía workers.
