# Architecture 02 — Vista de Contenedores

## Topología completa

```
                         :8888 (puerto público)
                             │
                       ┌─────┴─────┐
                       │   nginx   │
                       └─────┬─────┘
                     ┌───────┴───────┐
                     │              │
               ┌─────┴─────┐  ┌─────┴─────┐
               │ frontend  │  │  backend  │
               │  (SPA)    │  │  :8080    │
               └───────────┘  └─────┬─────┘
                                   │
               ┌───────────────────┼───────────────────┐
               │                   │                   │
         ┌─────┴─────┐      ┌─────┴─────┐      ┌─────┴─────┐
         │    db     │      │   redis   │      │ translator │
         │ postgres  │      │   cache   │      │  NLLB-200 │
         │   :5432   │      │   :6379   │      │   :8000   │
         └───────────┘      └───────────┘      └─────┬─────┘
                                                   │
                         ┌─────────────────────────┼─────────────────────────┐
                         │                         │                         │
                   ┌─────┴─────┐           ┌─────┴─────┐           ┌─────┴─────┐
                   │ translator-2│           │ translator-3│           │ translator-gpu│
                   │   (CPU)    │           │   (CPU)    │           │ (NVIDIA, opt)│
                   └───────────┘           └───────────┘           └───────────┘

                         ┌─────────────────────────┼─────────────────────────┐
                         │                         │                         │
                   ┌─────┴─────┐           ┌─────┴─────┐           ┌─────┴─────┐
                   │  ingestor │           │langdetect │           │  scheduler │
                   │    (Go)   │           │  (Python) │           │  (Python) │
                   └───────────┘           └───────────┘           └───────────┘

                         ┌─────────────────────────┼─────────────────────────┐
                         │                         │                         │
                   ┌─────┴─────┐           ┌─────┴─────┐           ┌─────┴─────┐
                   │    ner     │           │embeddings │           │   wiki    │
                   │  (spaCy)  │           │  (MiniLM) │           │           │
                   └───────────┘           └───────────┘           └───────────┘

                         ┌─────────────────────────┼─────────────────────────┐
                         │                         │                         │
                   ┌─────┴─────┐           ┌─────┴─────┐           ┌─────┴─────┐
                   │  related   │           │  topics   │           │  scraper  │
                   │           │           │           │           │           │
                   └───────────┘           └───────────┘           └───────────┘
```

## Servicios por función

### Núcleo de datos
| Servicio | Imagen | Puertos internos | Propósito |
|----------|--------|------------------|-----------|
| `db` | postgres:16-alpine | 5432 | Fuente de verdad |
| `redis` | redis:7-alpine | 6379 | Caché |

### API y frontend
| Servicio | Imagen | Puertos | Propósito |
|----------|--------|---------|-----------|
| `backend` | rss2-backend | 8080 | API REST Go |
| `frontend` | rss2-frontend | 3000 | SPA React |
| `nginx` | (embedded) | 8888 | Gateway + SSL |

### Ingesta
| Servicio | Imagen | Frecuencia | Propósito |
|----------|--------|------------|-----------|
| `ingestor` | rss2-ingestor (Go) | cada 8 min | RSS → noticias |

### Procesamiento de texto
| Servicio | Imagen | Input | Output |
|----------|--------|-------|--------|
| `langdetect` | rss2-scheduler | noticias.lang=NULL | noticias.lang |
| `translation-scheduler` | rss2-scheduler | noticias | traducciones.pending |
| `translator` (+ replicas) | rss2-translator | traducciones.pending | traducciones.done |
| `ner` | rss2-ner | traducciones.done | tags |

### Enriquecimiento
| Servicio | Imagen | Input | Output |
|----------|--------|-------|--------|
| `embeddings` | rss2-embeddings | traducciones.done | traduccion_embeddings |
| `related` | rss2-backend | traduccion_embeddings | related_noticias |
| `wiki` | rss2-backend | tags | wiki_summary |
| `topics` | rss2-backend | noticias | news_topics |
| `scraper` | rss2-backend | noticias.resumen<200 | noticias.enriquecido |

### Workers opcionales
| Servicio | Profile | Hardware |
|----------|---------|----------|
| `translator-gpu` | gpu | NVIDIA GPU |

## Variables de entorno compartidas

```yaml
DB_HOST: db
DB_PORT: 5432
DB_NAME: rss
DB_USER: rss
DB_PASS: ${POSTGRES_PASSWORD}
REDIS_URL: redis://:${REDIS_PASSWORD}@redis:6379
```

## Redes Docker

- `frontend`: nginx, frontend, backend
- `backend`: todos los servicios internos

## Healthchecks

Todos los workers verifican con: `grep -q <nombre> /proc/1/cmdline`
