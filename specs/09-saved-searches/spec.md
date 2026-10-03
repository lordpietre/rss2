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
| GET | `/api/favorites/:noticiaId` | Check si noticia es favorita |

### Listas

| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/api/lists` | Obtener todas las listas del usuario |
| POST | `/api/lists` | Crear nueva lista |
| PUT | `/api/lists/:id` | Renombrar lista |
| DELETE | `/api/lists/:id` | Eliminar lista |
| GET | `/api/lists/:id/items` | Obtener noticias de una lista |
| POST | `/api/lists/:id/items/:noticiaId` | Añadir noticia a lista |
| DELETE | `/api/lists/:id/items/:noticiaId` | Quitar noticia de lista |

### Búsquedas Guardadas

| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/api/saved-searches` | Obtener búsquedas guardadas |
| POST | `/api/saved-searches` | Crear búsqueda guardada |
| DELETE | `/api/saved-searches/:id` | Eliminar búsqueda guardada |
| PUT | `/api/saved-searches/:id` | Actualizar label |

## Arquitectura

- Backend: Go + Gin en `backend/`
- Frontend: React con hooks que llaman a la API
- Auth: JWT igual que el resto (user_id del token)

## Tareas

- [x] Crear migración SQL
- [x] Implementar handlers API en backend
- [x] Crear hooks frontend que usen API
- [x] Actualizar componentes para usar API en vez de localStorage
- [ ] Verificación

## Estado

- [x] Propuesta
- [x] Aprobada
- [x] Implementada
- [ ] Verificada
