# Architecture 04 — Componentes y Contratos

## Inventario de componentes

### Backend API (Go)

| Binario | Servicio | Puerto | Dependencias |
|---------|----------|--------|--------------|
| `server` | backend | 8080 | db, redis |
| `sanitize` | backend | - | db |
| `wiki-worker` | wiki | - | db, redis |
| `topics` | topics | - | db |
| `related` | related | - | db |
| `scraper` | scraper | - | db |
| `discovery` | discovery | - | db |
| `entityscan` | entitycheck | - | db |

### Workers Python

| Servicio | Imagen | Entrypoint | Dependencias |
|----------|--------|------------|--------------|
| `langdetect` | rss2-scheduler | langdetect_worker.py | db |
| `translation-scheduler` | rss2-scheduler | translation_scheduler.py | db |
| `translator` | rss2-translator | ctranslator_worker.py | db, redis, modelo |
| `ner` | rss2-ner | ner_worker.py | db |
| `embeddings` | rss2-embeddings | embeddings_worker.py | db, redis, modelo |

## Contratos de base de datos

### Tablas principales

```
noticias
├── id VARCHAR(32) PK (md5 url)
├── titulo TEXT
├── resumen TEXT
├── url TEXT UNIQUE
├── fecha TIMESTAMP
├── lang CHAR(5) -- NULL initially, set by langdetect
├── titulo_raw TEXT -- before cleaning
├── resumen_raw TEXT -- before cleaning
├── cleaned_at TIMESTAMP
├── topics_processed BOOLEAN DEFAULT false
└── pais_id, categoria_id FK

traducciones
├── id SERIAL PK
├── noticia_id VARCHAR(32) FK → noticias
├── lang_from CHAR(5)
├── lang_to CHAR(5) DEFAULT 'es'
├── titulo_trad TEXT
├── resumen_trad TEXT
├── status VARCHAR(20) -- pending, assigned, done, error
├── worker_id VARCHAR(100)
├── assigned_at TIMESTAMP
├── created_at TIMESTAMP DEFAULT now()
└── UNIQUE(noticia_id, lang_to)

tags
├── id SERIAL PK
├── valor VARCHAR(500)
├── tipo VARCHAR(50)
├── apellido VARCHAR(200)
└── wiki_checked BOOLEAN DEFAULT false

tags_noticia
├── noticia_id VARCHAR(32) FK → noticias
├── traduccion_id INT FK → traducciones
└── tag_id INT FK → tags

traduccion_embeddings
├── traduccion_id INT PK FK → traducciones
├── embedding vector(128)
└── model VARCHAR(100)

entity_blocklist
├── valor VARCHAR(500) PK
└── created_at TIMESTAMP

news_topics
├── noticia_id VARCHAR(32) PK FK → noticias
├── topics JSONB
└── pais_id INT

fechas
├── fecha DATE PK
├── continente_id INT
└── pais_id INT

alertas
├── id SERIAL PK
├── entidad VARCHAR(500)
├── tipo VARCHAR(50)
├── pais_id INT
├── hits INT
├── baseline FLOAT
├── ratio FLOAT
├── periodo TIMESTAMP
└── leida BOOLEAN DEFAULT false

fuentes_url
├── id SERIAL PK
├── nombre VARCHAR(255)
├── url TEXT
├── categoria_id INT
├── pais_id INT
├── idioma CHAR(5)
└── active BOOLEAN DEFAULT true
```

## Contratos HTTP

### Endpoints principales

```
GET  /api/news
     ?per_page=20&page=1&lang=es&category=&pais=&search=
     ← news[] + pagination

GET  /api/news/:id
     ← noticia + traducciones[]

GET  /api/search
     ?q=query&lang=es
     ← results[]

GET  /api/entities
     ?lang=es&type=person&search=&limit=
     ← entities[] + total

GET  /api/entities/news/:id
     ← news[] with entity

GET  /api/alerts
     ?status=nueva|leida|all&limit=
     ← alerts[]

POST /api/auth/login
     {email, password} → {token, user}

POST /api/auth/register
     {email, password, nombre} → {token, user}

GET  /api/admin/ingest/stats
     ← {feeds_total, feeds_active, news_24h, ...}

PUT  /api/admin/aliases/:id
DELETE /api/admin/aliases/:id
```

### OpenAPI

Swagger disponible en `/swagger/index.html` (gin-swagger)

## Variables de entorno por componente

### Traductor (ctranslator_worker.py)

| Variable | Default | Descripción |
|----------|---------|-------------|
| `MAX_SRC_TOKENS` | 1024 | Tokens máx texto origen |
| `MAX_NEW_TOKENS` | 1024 | Tokens máx traducción |
| `TRANSLATOR_BATCH` | 128 | Tamaño de lote |
| `MAX_SEQ_PER_CALL` | 32 | Seq por llamada CT2 |
| `CT2_MODEL_PATH` | /models/nllb-ct2 | Ruta modelo |
| `REDIS_URL` | redis://... | Caché Redis |

### Ingestor (rss-ingestor-go)

| Variable | Default | Descripción |
|----------|---------|-------------|
| `RSS_MAX_WORKERS` | 20 | Workers paralelos |
| `RSS_POKE_INTERVAL_MIN` | 8 | Minutos entre polls |

### Scheduler (translation_scheduler.py)

| Variable | Default | Descripción |
|----------|---------|-------------|
| `SCHEDULER_BATCH_SIZE` | 2000 | Jobs por ciclo |
| `SCHEDULER_INTERVAL_SEC` | 30 | Segundos entre ciclos |

## Dependencias de modelos

| Modelo | Ubicación | Tamaño | Uso |
|--------|-----------|--------|-----|
| nllb-200-distilled-600M | /models/nllb-ct2 | ~1GB | Traducción |
| es_core_news_lg | spaCy download | ~500MB | NER |
| paraphrase-multilingual-MiniLM-L12-v2 | hf_cache | ~500MB | Embeddings |
