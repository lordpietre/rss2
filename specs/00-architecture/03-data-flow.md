# Architecture 03 — Flujo de Datos

## Pipeline principal

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                              PIPELINE DE INGESTA                                     │
└─────────────────────────────────────────────────────────────────────────────────────┘

  RSS Feeds (6822)                          Cada 8 minutos
        │                                         │
        v                                         ▼
┌───────────────────┐                    ┌──────────────────┐
│  rss-ingestor-go  │ ──────────────────▶│     noticias     │
│   (20 workers)    │   INSERT            │ id, titulo,      │
│                   │   (evita duplicados  │ resumen, url,    │
└───────────────────┘    por md5(url))    │ fecha, lang=NULL │
                                          └────────┬─────────┘
                                                   │
                                                   ▼
                                          ┌──────────────────┐
                                          │    langdetect     │
                                          │  (langdetect)     │
                                          └────────┬─────────┘
                                                   │
                              SET lang             ▼
                                          ┌──────────────────┐
                                          │ translation_     │
                                          │ scheduler        │
                                          │ (cada 30s)      │
                                          └────────┬─────────┘
                                                   │
                              INSERT pending        ▼
                                          ┌──────────────────┐
                                          │  traducciones    │
                                          │ status='pending' │
                                          └────────┬─────────┘
                                                   │
        ┌──────────────────────────────────────────┼──────────────────────────┐
        │                                          │                          │
        ▼                                          ▼                          ▼
┌───────────────┐                         ┌───────────────┐           ┌───────────────┐
│  translator   │                         │  translator-2 │           │ translator-N  │
│ (SKIP LOCKED) │                         │ (SKIP LOCKED) │           │ (SKIP LOCKED) │
└───────┬───────┘                         └───────┬───────┘           └───────┬───────┘
        │                                        │                          │
        │  CTranslate2 NLLB-200                  │                          │
        │  MAX_SRC_TOKENS=1024                   │                          │
        │  MAX_NEW_TOKENS=1024                   │                          │
        ▼                                        ▼                          ▼
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                              TRADUCCIÓN COMPLETADA                                  │
└─────────────────────────────────────────────────────────────────────────────────────┘
        │                                        │                          │
        ▼                                        ▼                          ▼
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                              CONSUMIDORES DE TRADUCCIONES                           │
└─────────────────────────────────────────────────────────────────────────────────────┘

        │
        ├──────────────────┬──────────────────┬──────────────────┐
        ▼                  ▼                  ▼                  ▼
┌───────────────┐  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐
│      ner      │  │   embeddings  │  │     wiki      │  │    topics     │
│   spaCy       │  │   MiniLM-L12  │  │               │  │               │
│  es_core_news │  │               │  │               │  │               │
└───────┬───────┘  └───────┬───────┘  └───────┬───────┘  └───────┬───────┘
        │                    │                    │                    │
        ▼                    ▼                    ▼                    ▼
┌───────────────┐  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐
│     tags      │  │ traduccion_   │  │   noticias    │  │  news_topics  │
│ tags_noticia  │  │ embeddings    │  │ wiki_summary  │  │               │
└───────────────┘  └───────┬───────┘  └───────────────┘  └───────────────┘
                            │
                            ▼
                    ┌───────────────┐
                    │    related    │
                    │ (coseno SQL)  │
                    └───────────────┘
```

## Text Cleaning Pipeline

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                              LIMPIEZA DE TEXTO                                       │
└─────────────────────────────────────────────────────────────────────────────────────┘

  texto bruto                          texto limpio
        │                                   │
        ▼                                   │
┌───────────────────┐                      │
│    textclean     │                      │
│  (x/net/html +   │ ────────────────────▶│
│   RE2 regex)     │                      │
└───────────────────┘                      │
                                          ▼
                              ┌───────────────────────┐
                              │ 1. Strip HTML tags   │
                              │ 2. Chrome residual   │
                              │    - window._taboola │
                              │    - googletag.cmd   │
                              │    - UI phrases      │
                              │ 3. Fix encoding      │
                              │ 4. Trim whitespace   │
                              └───────────────────────┘
```

## Chunking para traducción

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                              CHUNKING INTELIGENTE                                     │
└─────────────────────────────────────────────────────────────────────────────────────┘

  texto largo (>1024 tokens)              chunks
        │                                   │
        ▼                                   │
┌───────────────────┐                      │
│ split_body_into   │                      │
│ _chunks()        │ ────────────────────▶│
│                  │                      │
└───────────────────┘                      │
                                          ▼
                              ┌───────────────────────┐
                              │ Separadores:         │
                              │ - . ! ? (sentences)  │
                              │ - ; : (clauses)      │
                              │ - 。！？（asian）     │
                              │ - ।။॥ (indic)      │
                              │ Límite: MAX_SRC/2   │
                              └───────────────────────┘
                                          │
                                          ▼
                              ┌───────────────────────┐
                              │ join_chunks_with_    │
                              │ punctuation()        │
                              └───────────────────────┘
                                          │
                                          ▼
                              texto unido preservando
                              puntuación final
```

## Estados de traducción

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                              ESTADOS DE TRADUCCIÓN                                    │
└─────────────────────────────────────────────────────────────────────────────────────┘

  pending ─────────────────────────────────────────────────▶ done
      │                                                           │
      │ (SKIP LOCKED)                                             │
      ▼                                                           │
  assigned ────▶ error ────▶ (retry via scheduler)                │
      │                                                           │
      │ (timeout 10min)                                          │
      └───────────────────────────────────────────────────────────┘
```

## Caché Redis

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                              CACHÉ REDIS                                              │
└─────────────────────────────────────────────────────────────────────────────────────┘

  Keys por prefijo:
  
  translation:   título · cuerpo · lang_from · lang_to → traducción
  entity:       entidad · tipo → wiki_summary
  embeddings:   traduccion_id → vector (128dim)
  
  TTL: 24 horas (reconstruible)
  
  Redis Commands:
  - SET translation:{hash} {json}
  - GET translation:{hash}
  - FLUSHDB (solo tras fix de formato)
```

## Volúmenes persistentes

| Volumen | Contenido | Persistencia |
|---------|-----------|--------------|
| `./data/pgdata` | Postgres data | **Crítico** |
| `./data/redis-data` | Redis AOF | Caché, reconstruible |
| `./models` | NLLB + spaCy + MiniLM | Read-only |
| `./hf_cache` | HuggingFace cache | Build artifact |
