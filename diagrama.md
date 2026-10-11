# RSS2 - Diagrama de Workers y Contenedores

## Arquitectura General

```
                    ┌──────────────────────────────────────────────────────────────────┐
│   INTERNET       │                    RSS2 SYSTEM                                   │
│   Fuentes RSS     │                                                              │
└────────┬─────────┘                                                              │
         │                                                                        │
         ▼                                                                        ▼
┌─────────────────┐                                                    ┌─────────────────┐
│  RSS INGESTOR   │ ──── INSERT noticias ────▶  ┌─────────────────┐   │   FRONTEND      │
│  (Go)           │                             │   POSTGRES DB    │   │   (React)       │
│  Pasa cada 60m  │                             │   + pgvector     │   │   Puerto 8888   │
└─────────────────┘                             │   339k noticias  │   └─────────────────┘
                                               │   1.6k feeds     │          ▲
                                               │   372k tags       │          │
                                               └─────────────────┘          │
                                                        ▲                    │
                                                        │                    │
         ┌─────────────────────────────────────────────┼────────────────────┘
         │                                             │
         ▼                                             │
┌─────────────────────────────────────────────────────────────────────────────────┐
│                           WORKERS (Procesamiento en paralelo)                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## 1. INGESTOR (Go) - `rss2_ingestor`
**¿Qué hace?**
- Descarga feeds RSS/Atom de internet
- Extrae título, resumen, contenido, imagen, fecha de cada noticia
- Guarda las noticias nuevas en la base de datos

**¿Cómo lo hace?**
```
1. Cada 60 minutos:
   └─▶ Consulta la tabla "feeds" para ver qué feeds activos existen
       (Hay 1,620 feeds configurados)

2. Para cada feed (con 20 trabajadores en paralelo):
   ├─▶ Hace petición HTTP al servidor del feed
   ├─▶ Usa caché ETag/Last-Modified para evitar descargar contenido ya conocido
   ├─▶ Si el contenido es nuevo, extrae:
   │       - Título
   │       - Resumen (primeros 500 caracteres)
   │       - URL de la imagen
   │       - Fecha de publicación
   │       - Fuente (nombre del feed)
   │
   └─▶ INSERT en tabla "noticias" (ON CONFLICT DO NOTHING para evitar duplicados)
```

**Logs típicos:**
```
[Feed 123] Inserted 3 new items
[Feed 456] Not Modified (304)
[Feed 789] Error: HTTP 404
```

---

## 2. TRANSLATION-SCHEDULER (Python) - `rss2_translation_scheduler`
**¿Qué hace?**
- Detecta noticias nuevas que necesitan traducción
- Crea trabajos de traducción en la cola

**¿Cómo lo hace?**
```
Cada 30 segundos:
├─▶ SELECT noticias sin idioma detectado (lang IS NULL)
│       └─▶ UPDATE noticias SET lang = detect(titulo+resumen)
│
└─▶ SELECT noticias que:
        - Tienen idioma detectado (no es 'es')
        - NO tienen traducción pendiente o completada
    └─▶ INSERT INTO traducciones (status='pending')
```

---

## 3. TRANSLATOR (Python/CTranslate2) - `rss2_translator` y `rss2_translator_2`
**¿Qué hace?**
- Traduce noticias del idioma original → Español
- Usa el modelo NLLB-200 (Facebook) para traducciones de alta calidad

**¿Cómo lo hace?**
```
Cada 30 segundos (si hay trabajos pendientes):
├─▶ SELECT traducciones WHERE status='pending' FOR UPDATE SKIP LOCKED LIMIT 32
│       (2 translators usan SKIP LOCKED para no chocarse)
│
├─▶ Para cada traducción pendiente:
│   ├─▶ Detecta idioma (si no está)
│   ├─▶ Traduce título + resumen + contenido (completo, hasta 80k chars)
│   ├─▶ Guarda en Redis cache (para no traducir dos veces lo mismo)
│   └─▶ UPDATE traducciones SET status='done', titulo_trad, resumen_trad, contenido_trad
│
└─▶ Espera 30 segundos antes del siguiente batch
```

**Modelo:** NLLB-200 distilled (600M parámetros) - traduccciones de calidad profesional

**Logs típicos:**
```
Found 128 pending translations for es
Translating en -> es (61 items)
Finished group en -> es
```

---

## 4. NER (Named Entity Recognition) - `rss2_ner`
**¿Qué hace?**
- Detecta personas, organizaciones y lugares en las noticias traducidas
- Clasifica tags y los asocia a cada noticia

**¿Cómo lo hace?**
```
Cada 30 segundos:
├─▶ SELECT traducciones.done que aún no tienen tags
│       └─▶ Procesa en batches de 64 traducciones
│
├─▶ Para cada traducción, usa spaCy (modelo es_core_news_lg):
│   ├─▶ Extrae entidades nombradas:
│   │       - PERSON (personas)
│   │       - ORG (organizaciones)
│   │       - GPE/LOC (lugares)
│   │
│   └─▶ Limpia los nombres (elimina噪音, normaliza)
│
├─▶ INSERT INTO tags (valor, tipo) ON CONFLICT DO UPDATE
│       (Ejemplo: "Pedro Sánchez" → tipo:persona)
│
└─▶ INSERT INTO tags_noticia (traduccion_id, noticia_id, tag_id)
        (Asocia el tag con la noticia)
```

**Modelo:** spaCy `es_core_news_lg` - 1.3GB, entrenado específicamente para español

---

## 5. EMBEDDINGS (Python/sentence-transformers) - `rss2_embeddings`
**¿Qué hace?**
- Genera vectores numéricos (384 dimensiones) para cada noticia traducida
- Permite búsqueda semántica (buscar por significado, no solo palabras)

**¿Cómo lo hace?**
```
Cada 30 segundos:
├─▶ SELECT traducciones.done sin embedding
│       └─▶ Procesa en batches de 128
│
├─▶ Para cada traducción:
│   ├─▶ Combina título + resumen + contenido (texto completo)
│   ├─▶ Pasa el texto por sentence-transformers (MiniLM-L12)
│   └─▶ Obtiene vector de 384 dimensiones
│
└─▶ INSERT INTO traduccion_embeddings (traduccion_id, embedding)
        (Usa pgvector para almacenar los vectores)
```

**Modelo:** paraphrase-multilingual-MiniLM-L12-v2 (384 dims, multilingüe)

---

## 6. TOPICS (Go) - `rss2_topics`
**¿Qué hace?**
- Clasifica cada noticia en topics (temas): política, economía, deportes, etc.
- Detecta el país de la noticia

**¿Cómo lo hace?**
```
Cada 10 segundos:
├─▶ SELECT noticias WHERE topics_processed = FALSE LIMIT 500
│
├─▶ Para cada noticia:
│   ├─▶ Busca país: compara palabras del título con lista de países
│   │       (Ejemplo: "Madrid" → España, "París" → Francia)
│   │
│   └─▶ Busca topics: keywords matching
│           - Si contiene "elecciones", "votación" → tema:politica
│           - Si contiene "bolsa", "banco" → tema:economia
│           - Si contiene "futbol", "baloncesto" → tema:deportes
│
└─▶ UPDATE noticias SET pais_id, topics_processed = TRUE
```

**Keywords de topics:** 37 temas predefinidos con ~5 keywords cada uno

---

## 7. RELATED (Go) - `rss2_related`
**¿Qué hace?**
- Encuentra noticias relacionadas entre sí usando similitud coseno
- Usa los vectores de embeddings para calcular cercanía

**¿Cómo lo hace?**
```
Cada 30 segundos:
├─▶ SELECT traducciones.done sin related
│       └─▶ Procesa en batches de 200
│
├─▶ Para cada traducción:
│   ├─▶ Obtiene el embedding de la noticia
│   │
│   ├─▶ Busca las 10 noticias más similares en pgvector:
│   │       SELECT * FROM traduccion_embeddings
│   │       ORDER BY embedding <=> query_vector
│   │       LIMIT 10
│   │
│   └─▶ INSERT INTO related_noticias (noticia_id, related_id, score)
│
└─▶ UPDATE traducciones SET related_processed = TRUE
```

**Método:** Búsqueda HNSW en pgvector (similitud coseno)

---

## 8. WIKI (Go) - `rss2_wiki`
**¿Qué hace?**
- Enriquece los tags de personas y organizaciones con información de Wikipedia
- Añade resumen y foto del personaje/organización

**¿Cómo lo hace?**
```
Cada 1 segundo (más agresivo después del cambio):
├─▶ SELECT tags WHERE tipo IN ('persona','organizacion')
│       AND wiki_checked = FALSE
│       ORDER BY menciones DESC
│       LIMIT 40
│
├─▶ Para cada tag:
│   ├─▶ Consulta Wikipedia API (es.wikipedia.org + en.wikipedia.org):
│   │       1. Intento directo: /page/summary/{tag}
│   │       2. Si falla, búsqueda: /w/api.php?action=opensearch
│   │       3. Busca variantes sin acentos
│   │
│   ├─▶ Si encuentra artículo:
│   │       ├─▶ Descarga thumbnail (foto)
│   │       └─▶ Guarda resumen + URL + path de imagen
│   │
│   └─▶ UPDATE tags SET wiki_summary, wiki_url, image_path, wiki_checked=TRUE
│
└─▶ Duerme 1 segundo entre requests (respeto a Wikipedia)
```

**Tags pendientes:** ~53,000 (procesa ~3,600/hora → ~15 horas total)

---

## 9. LANGDETECT (Python) - `rss2_langdetect`
**¿Qué hace?**
- Detecta el idioma de las noticias nuevas

**¿Cómo lo hace?**
```
Cada 30 segundos:
├─▶ SELECT noticias WHERE lang IS NULL LIMIT 1000
│
└─▶ Para cada noticia:
        ├─▶ langdetect.detect(titulo + resumen)
        └─▶ UPDATE noticias SET lang = 'en'/'fr'/'de'/'pt'/etc
```

---

## 10. SCRAPER (Go) - `rss2_scraper`
**¿Qué hace?**
- Enriquece noticias con contenido HTML scrapeado
- Completa resúmenes cortos (<200 chars)

**¿Cómo lo hace?**
```
Cada 60 segundos:
├─▶ SELECT noticias WHERE resumen IS NULL OR LENGTH(resumen) < 200
│       LIMIT 100
│
└─▶ Para cada noticia:
        ├─▶ HTTP GET a la URL de la noticia
        ├─▶ Extrae contenido HTML
        ├─▶ Limpia HTML → texto plano
        └─▶ UPDATE noticias SET resumen = texto_limpio (max 500 chars)
```

---

## 11. DISCOVERY (Go) - `rss2_discovery`
**¿Qué hace?**
- Descubre nuevos feeds RSS desde fuentes de noticias
- Añade feeds automáticamente a la base de datos

**¿Cómo lo hace?**
```
Cada 15 minutos:
├─▶ SELECT fuentes_url donde activo = TRUE
│       (Fuentes externas que listan feeds)
│
└─▶ Para cada fuente:
        ├─▶ HTTP GET a la URL de la fuente
        ├─▶ Extrae enlaces RSS/Atom
        └─▶ INSERT INTO feeds (url, nombre, categoria_id, pais_id)
                ON CONFLICT(url) DO NOTHING
```

---

## BASE DE DATOS - `rss2_db` (PostgreSQL + pgvector)

### Tablas principales:

| Tabla | Contenido | Tamaño |
|-------|-----------|--------|
| `noticias` | Noticias RSS | 339,153 filas |
| `feeds` | Fuentes RSS configuradas | 1,620 filas |
| `traducciones` | Traducciones pendientes/completadas | ~300,000 filas |
| `tags` | Entidades (personas, orgs, lugares) | 372,000 filas |
| `tags_noticia` | Relación tags ↔ noticias | ~1M filas |
| `traduccion_embeddings` | Vectores de 384 dims | ~300,000 filas |
| `related_noticias` | Noticias relacionadas | ~500,000 filas |
| `user_lists` | Listas de favoritos | - |
| `alertas` | Alertas de volumen | - |

### Índices importantes:
- `idx_noticias_fecha` - Búsquedas por fecha
- `idx_traducciones_status` - Filtrar por estado
- `idx_tags_noticia_noticia_id` - JOIN tags ↔ noticias
- `idx_tags_valor_tipo` - Búsqueda de entidades
- `idx_traduccion_embeddings` (HNSW) - Búsqueda vectorial

---

## CACHE - `rss2_redis` (512MB max)

### Qué guarda:
- Resultados de Wikipedia (48h TTL)
- Traducciones cacheadas (para no traducir dos veces)
- Sesiones de usuarios
- Rate limiting

---

## FLUJO COMPLETO DE UNA NOTICIA

```
1. [INGESTOR] RSS feed → nueva noticia INSERTada
                        │
2. [LANGDETECT] Detecta idioma (ej: inglés)
                        │
3. [SCHEDULER] Crea job de traducción
                        │
4. [TRANSLATOR] Traduce en→es
                        │
5. [NER] Extrae personas y organizaciones
                        │
6. [EMBEDDINGS] Genera vector semántico
                        │
7. [TOPICS] Clasifica en temas
                        │
8. [RELATED] Encuentra noticias similares
                        │
9. [WIKI] Enriquece con info de Wikipedia
                        │
10. [SCRAPER] Completa resumen si es corto
                        │
11. [FRONTEND] Usuario ve la noticia traducida
```

---

## TIEMPOS DE PROCESAMIENTO

| Paso | Worker | Frecuencia | Tiempo/cycle |
|------|--------|------------|--------------|
| Ingestión | ingestor | 60 min | ~8 min |
| Detección idioma | langdetect | 30 seg | <1 seg |
| Traducción | translator | 30 seg | ~4 min/batch |
| NER | ner | 30 seg | ~2 min/batch |
| Embeddings | embeddings | 30 seg | ~3 min/batch |
| Topics | topics | 10 seg | <1 seg |
| Related | related | 30 seg | ~5 min/batch |
| Wiki | wiki | 1 seg/tag | ~53k tags × 15h |
| Scraper | scraper | 60 seg | variable |
| Discovery | discovery | 15 min | variable |

---

## PUERTOS Y EXPOSICIÓN

| Servicio | Puerto interno | Expuesto | Para qué |
|----------|---------------|----------|----------|
| nginx | 80 | 8888 | Gateway principal |
| backend | 8080 | 8080 | API REST + WebSocket workers |
| frontend | 80 | - | Solo vía nginx |
| redis | 6379 | - | Solo internos |
| postgres | 5432 | - | Solo internos |
| embeddings | 6000 | 6000 | API búsqueda semántica |
