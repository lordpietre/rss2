# Spec 09 — Favoritos y Listas Persistentes (Postgres)

## Objetivo

Persistir favoritos y listas de noticias en la base de datos Postgres para que estén disponibles en cualquier navegador/dispositivo del usuario.

## Decisiones de diseño

### Modelo de datos (Postgres)

```sql
-- Tabla de favoritos: noticias marcadas como favoritas por usuario
CREATE TABLE user_favorites (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    noticia_id VARCHAR(32) NOT NULL,  -- ID de la noticia (no FK, puede no existir)
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, noticia_id)
);

-- Tabla de listas: listas nombradas por usuario
CREATE TABLE user_lists (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    keywords TEXT DEFAULT '',           -- Palabras clave separadas por comas para búsqueda automática
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Tabla de noticias en listas
CREATE TABLE user_list_items (
    id SERIAL PRIMARY KEY,
    list_id INTEGER NOT NULL REFERENCES user_lists(id) ON DELETE CASCADE,
    noticia_id VARCHAR(32) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(list_id, noticia_id)
);

-- Tabla de búsquedas guardadas
CREATE TABLE user_saved_searches (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(20) NOT NULL,  -- 'news' o 'entity'
    label VARCHAR(255) NOT NULL,
    params JSONB NOT NULL,  -- parámetros como JSON
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

## Contrato API

### Favoritos

| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/api/favorites` | Obtener favoritos del usuario |
| POST | `/api/favorites/:noticiaId` | Añadir noticia a favoritos |
| DELETE | `/api/favorites/:noticiaId` | Quitar noticia de favoritos |
| GET | `/api/favorites/:noticiaId/check` | Verificar si es favorita |

### Listas

| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/api/lists` | Obtener todas las listas del usuario (incluye keywords) |
| POST | `/api/lists` | Crear nueva lista (con keywords opcionales) |
| PUT | `/api/lists/:id` | Renombrar lista y/o actualizar keywords |
| DELETE | `/api/lists/:id` | Eliminar lista |
| GET | `/api/lists/:id/items` | Obtener noticias de una lista |
| POST | `/api/lists/:id/items/:noticiaId` | Añadir noticia a lista |
| DELETE | `/api/lists/:id/items/:noticiaId` | Quitar noticia de lista |
| GET | `/api/lists/:id/tags` | Obtener nube de tags de la lista |
| GET | `/api/lists/:id/related` | Buscar noticias relacionadas (auto-actualiza keywords) |

### Sugerencias para Sidebar

| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/api/lists/suggested` | Noticias sugeridas basadas en keywords de todas las listas del usuario |

### Búsquedas Guardadas

| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/api/saved-searches` | Obtener búsquedas guardadas |
| POST | `/api/saved-searches` | Crear búsqueda guardada |
| DELETE | `/api/saved-searches/:id` | Eliminar búsqueda guardada |
| PUT | `/api/saved-searches/:id` | Actualizar label |

## Funcionalidades Avanzadas

### Búsqueda Automática de Noticias Relacionadas

Cuando se accede a `GET /api/lists/:id/related`:

1. Obtiene los 10 tags con más menciones de las noticias en la lista
2. Los convierte automáticamente en keywords (separadas por comas)
3. Actualiza el campo `keywords` de la lista si cambió
4. Busca noticias relacionadas usando full-text search (tsquery con fallback ILIKE)
5. Excluye noticias que ya están en la lista

### Nube de Tags

`GET /api/lists/:id/tags` retorna los tags/entities de todas las noticias en la lista, ordenados por frecuencia:

```json
{
  "tags": [
    { "valor": "Biden", "tipo": "persona", "count": 15 },
    { "valor": "Estados Unidos", "tipo": "lugar", "count": 12 }
  ]
}
```

### Sidebar de Sugerencias

`GET /api/lists/suggested` busca noticias de los últimos 7 días que:
- Coincidan con las keywords de alguna lista del usuario
- No estén ya en ninguna de sus listas

## Arquitectura

- Backend: Go + Gin en `backend/`
- Frontend: React con hooks que llaman a la API
- Auth: JWT igual que el resto (user_id del token)

## Tareas

- [x] Crear migración SQL
- [x] Implementar handlers API en backend
- [x] Crear hooks frontend que usen API
- [x] Actualizar componentes para usar API en vez de localStorage
- [x] Endpoint de tags para nube interactiva
- [x] Búsqueda automática de noticias relacionadas
- [x] Actualización automática de keywords desde top 10 tags
- [x] Endpoint de sugerencias para sidebar
- [x] Verificación

## Estado

- [x] Propuesta
- [x] Aprobada
- [x] Implementada
- [x] Verificada
