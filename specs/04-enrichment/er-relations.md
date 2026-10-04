# Entidades y Relaciones RSS2

## Entidades Principales

### noticias
```
noticias (id VARCHAR(32) PK, titulo, resumen, url, fecha, imagen_url, 
         fuente_nombre, categoria_id FK, pais_id FK, tsv, lang, 
         search_vector_es, resumen_raw, titulo_raw, cleaned_at, contenido,
         topics_processed)
```

### feeds
```
feeds (id SERIAL PK, nombre, url, site_url, descripcion, imagen_url, idioma,
       categoria_id FK, pais_id FK, activo, fallos, last_error, last_fetched)
```

### categorias
```
categorias (id SERIAL PK, nombre, color, icon, parent_id FK)
```

### paises
```
pais (id SERIAL PK, nombre, codigo, continente, flag_emoji)
```

### users
```
users (id SERIAL PK, email, username, password_hash, is_admin, role)
```

## Relaciones

```
noticias.categoria_id → categorias(id) [ON DELETE SET NULL]
noticias.pais_id → pais(id) [ON DELETE SET NULL]

feeds.categoria_id → categorias(id) [ON DELETE SET NULL]
feeds.pais_id → pais(id) [ON DELETE SET NULL]

noticias → traducciones (1:N por noticia_id)
noticias → favoritos (N:M via favoritos/noticias_favoritos)
noticias → tags_noticia (N:M via tag_id)
noticias → related_noticias (N:M self-referential)
noticias → news_topics (N:M via news_topic_id)

users → favoritos (1:N, user_id)
users → user_lists (1:N)
users → user_favorites (1:N)
users → user_search_tags (1:N)
users → search_history (1:N)
users → user_saved_searches (1:N)

traducciones.noticia_id → noticias(id) [ON DELETE CASCADE]
traducciones.status → 'pending'|'done'|'error'

tags_noticia.tag_id → tags(id) [ON DELETE CASCADE]
tags_noticia.noticia_id → noticias(id) [ON DELETE CASCADE]
tags_noticia.traduccion_id → traducciones(id) [ON DELETE CASCADE]

entity_aliases.canonical_name → entities(nombre) [不详]
```

## Tablas Dependientes de Traducción

```
traducciones (id, noticia_id FK, lang_from, lang_to, titulo, resumen, 
              status, error, created_at, updated_at)

translation_stats (id, lang_to, items_translated, elapsed_ms, hostname, created_at)
```

## Tablas de Workers

```
remote_workers (id, hostname, status, last_seen, api_key)
```

## Notas de Diseño

1. **Soft delete**: No hay soft delete; se usa `ON DELETE CASCADE` para referencias.
2. **Idempotencia**: URLs únicas en `noticias` y `feeds` con `UNIQUE`.
3. **Full-text search**: `tsv` y `search_vector_es` son tsvector columns con GIN index.
4. **Lang detection**: `noticias.lang` se detecta con langdetect; valores inválidos se filtran.
