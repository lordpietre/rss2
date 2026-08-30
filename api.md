# RSS2 API Documentation

## Base URL
- **Production**: `https://api.rss2.local/api`
- **Development**: `http://localhost:8080/api`
- **Alternative**: `http://localhost:8888/api` (via nginx proxy)

## Base Path
All endpoints: `/api/*`

---

## Authentication

### JWT Bearer Token

- **Header**: `Authorization: Bearer <token>`
- **How to obtain**: `POST /api/auth/login` with `{email, password}`
- **Token**: HS256 signed, valid 24 hours
- **Header format**: `Authorization: Bearer <token>`

### API Key (Workers)

- **Header**: `X-API-Key: <key>` (for WebSocket `/ws/worker`)
- **Query**: `api_key=<key>` (REST endpoints for workers)

---

## Authentication Endpoints (Public - No auth required)

### POST /api/auth/login

Authenticate user and receive JWT token.

**Request:**
```json
{
  "email": "user@example.com",
  "password": "password123"
}
```

**Response 200:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": 1,
    "email": "user@example.com",
    "username": "user1",
    "is_admin": false,
    "role": "user",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  },
  "is_first_user": false
}
```

**Response 401:**
```json
{
  "error": "Invalid credentials"
}
```

---

### POST /api/auth/register

Register new account. First user becomes admin automatically.

**Request:**
```json
{
  "email": "newuser@example.com",
  "username": "newuser",
  "password": "securepass123"
}
```

**Response 200:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": 2,
    "email": "newuser@example.com",
    "username": "newuser",
    "is_admin": true,
    "role": "admin",
    "created_at": "2024-01-02T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  },
  "is_first_user": true
}
```

**Response 400:**
```json
{
  "error": "Username must be at least 3 characters"
}
```

---

### GET /api/auth/check-first-user

Check if current user is the first registered user.

**Response 200:**
```json
{
  "is_first_user": true,
  "total_users": 1
}
```

---

## News Endpoints

### GET /api/news

List news with pagination and multilingual support.

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `page` | int | 1 | Page number |
| `per_page` | int | 20 | Items per page (max 100) |
| `q` | string | - | Search query (title/resumen) |
| `category_id` | int64 | - | Filter by category ID |
| `country_id` | int64 | - | Filter by country ID |
| `lang` | string | "es" | Language filter |
| `translated_only` | bool | false | Only show translated news |

**Response 200:**
```json
{
  "news": [
    {
      "id": 42,
      "titulo": "Inteligencia Artificial revoluciona la tecnología",
      "resumen": "Resumen de la noticia sobre IA",
      "contenido": "Contenido completo...",
      "url": "https://example.com/ia-article",
      "imagen_url": "https://example.com/image.jpg",
      "fecha": "2024-03-15T10:30:00Z",
      "lang": "en",
      "fuente_nombre": "TechCrunch",
      "title_translated": "Inteligencia Artificial revoluciona la tecnología",
      "summary_translated": "IA revoluciona la tecnología",
      "content_translated": "La IA revoluciona la tecnología...",
      "lang_translated": "es",
      "entities": [
        {
          "valor": "Apple",
          "tipo": "organizacion",
          "apellido": "",
          "count": 3
        }
      ]
    }
  ],
  "total": 150,
  "page": 1,
  "per_page": 20,
  "total_pages": 8
}
```

### GET /api/news/:id

Get single news item with full entities and translations.

**Path Parameters:**
- `id` - News ID

**Query Parameters:**
- `lang` - Language for translation display (default: "es")

**Response 200:**
```json
{
  "id": 42,
  "titulo": "Inteligencia Artificial revoluciona la tecnología",
  "resumen": "Resumen de la noticia sobre IA",
  "contenido": "Contenido completo sobre inteligencia artificial...",
  "url": "https://example.com/ia-article",
  "imagen_url": "https://example.com/image.jpg",
  "fecha": "2024-03-15T10:30:00Z",
  "lang": "en",
  "fuente_nombre": "TechCrunch",
  "title_translated": "Inteligencia Artificial revoluciona la tecnología",
  "summary_translated": "Resumen traducido de la noticia...",
  "content_translated": "La inteligencia artificial está transformando todos los sectores...",
  "lang_translated": "es",
  "entities": [
    {
      "valor": "Apple",
      "tipo": "organizacion",
      "apellido": "",
      "count": 3,
      "wiki_summary": "Apple Inc. is an American technology company...",
      "wiki_url": "https://en.wikipedia.org/wiki/Apple_Inc.",
      "image_path": "/static/images/apple.jpg"
    },
    {
      "valor": "Inteligencia Artificial",
      "tipo": "tema",
      "apellido": "",
      "count": 1
    }
  ]
}
```

**Response 404:**
```json
{
  "error": "News not found"
}
```

### DELETE /api/news/:id

Delete a news item.

**Auth Required**: Yes (JWT Bearer token)

**Path Parameters:**
- `id` - News ID to delete

**Response 200:**
```json
{
  "message": "News deleted successfully"
}
```

**Response 401:** Unauthorized
**Response 403:** Forbidden - Admin required
**Response 404:** News not found

## Feeds Endpoints

### GET /api/feeds

List feeds with pagination and filters.

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `page` | int | 1 | Page number |
| `per_page` | int | 20 | Items per page (max 100) |
| `activo` | bool | - | Filter by active status |
| `categoria_id` | int64 | - | Filter by category ID |
| `pais_id` | int64 | - | Filter by country ID |

**Response 200:**
```json
{
  "feeds": [
    {
      "id": 1,
      "nombre": "TechCrunch",
      "url": "https://techcrunch.com/rss",
      "descripcion": "Technology news",
      "idioma": "en",
      "activo": true,
      "categoria_id": 1,
      "pais_id": 1,
      "ultima_publicacion": "2024-03-15T10:30:00Z",
      "total_seguidores": 1500
    }
  ],
  "total": 25,
  "page": 1,
  "per_page": 20,
  "total_pages": 2
}
```

### POST /api/feeds

Create new feed. Requires authentication.

**Request (application/json):**
```json
{
  "nombre": "Bloomberg Technology",
  "url": "https://bloomberg.com/technology/rss",
  "descripcion": "Technology news from Bloomberg",
  "categoria_id": 1,
  "pais_id": 2,
  "idioma": "en"
}
```

**Response 201:**
```json
{
  "id": 5,
  "message": "Feed created successfully"
}
```

**Response 400:**
```json
{
  "error": "Invalid feed URL"
}
```

**Response 401:**
```json
{
  "error": "Unauthorized"
}
```

### GET /api/feeds/:id

Get single feed by ID.

**Path Parameters:**
- `id` - Feed ID

**Response 200:**
```json
{
  "id": 1,
  "nombre": "TechCrunch",
  "url": "https://techcrunch.com/rss",
  "descripcion": "Technology news",
  "idioma": "en",
  "activo": true,
  "categoria_id": 1,
  "pais_id": 1,
  "ultima_publicacion": "2024-03-15T10:30:00Z",
  "total_seguidores": 1500
}
```

**Response 404:**
```json
{
  "error": "Feed not found"
}
```

### POST /api/feeds/import

Import feeds from CSV. Requires authentication. Multipart form-data.

**Request:**
- **Content-Type**: `multipart/form-data`
- **Field**: `file` - CSV file with columns: `nombre,url,descripcion?,categoria_id?,pais_id?,idioma?`

**Response 200:**
```json
{
  "imported": 10,
  "skipped": 2,
  "failed": 0,
  "errors": [],
  "message": "Import completed: 10 feeds imported, 2 skipped"
}
```

### POST /api/feeds/:id/toggle

Toggle feed activo status.

**Path Parameters:**
- `id` - Feed ID

**Auth Required:** Yes

**Response 200:**
```json
{
  "message": "Feed toggled successfully"
}
```

### POST /api/feeds/:id/reactivate

Reactivate feed (set activo=true, fallos=0).

**Path Parameters:**
- `id` - Feed ID

**Auth Required:** Yes

**Response 200:**
```json
{
  "message": "Feed reactivated successfully"
}
```

## Search Endpoints

### GET /api/search

Search news with text query and filters.

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `q` | string | required | Text search query |
| `page` | int | 1 | Page number |
| `per_page` | int | 20 | Items per page (max 100) |
| `lang` | string | "es" | Language filter |
| `categoria_id` | int64 | - | Filter by category |
| `pais_id` | int64 | - | Filter by country |
| `semantic` | bool | false | Use semantic search |

**Response 200:**
```json
{
  "news": [
    {
      "id": 42,
      "titulo": "Inteligencia Artificial revoluciona la tecnología",
      "resumen": "Resumen de la noticia sobre IA",
      "url": "https://example.com/ia-article",
      "fuente_nombre": "TechCrunch",
      "score": 0.95
    }
  ],
  "total": 42,
  "page": 1,
  "per_page": 20,
  "total_pages": 3
}
```

**Response 400:**
```json
{
  "error": "Search query is required"
}
```

---

### GET /api/search/suggestions

Get user's most used search terms.

**Auth Required:** Yes (JWT Bearer token)

**Query Parameters:**
- `q` - Optional prefix filter

**Response 200:**
```json
{
  "terms": ["inteligencia artificial", "tecnología", "programación", "startup"]
}
```

**Response 401:**
```json
{
  "error": "Unauthorized"
}
```

---

### POST /api/searchlog

Log search term for priority ranking.

**Auth Required:** Yes (JWT Bearer token)

**Request (application/json):**
```json
{
  "q": "inteligencia artificial"
}
```

**Response 200:**
```json
{
  "ok": true
}
```

## Entity Endpoints

### GET /api/entities

List entities with optional filters.

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `tipo` | string | "persona" | Entity type: persona, organizacion, lugar, tema |
| `page` | int | 1 | Page number |
| `per_page` | int | 20 | Items per page |
| `country_id` | int64 | - | Filter by country ID |
| `category_id` | int64 | - | Filter by category ID |
| `q` | string | - | Search by entity value |
| `apellido` | string | - | Filter by last name (personas) |

**Response 200:**
```json
{
  "entities": [
    {
      "id": 1,
      "valor": "Apple",
      "tipo": "organizacion",
      "apellido": "",
      "count": 45,
      "wiki_summary": "Apple Inc. is an American technology company...",
      "wiki_url": "https://en.wikipedia.org/wiki/Apple_Inc.",
      "image_path": "/static/images/apple.jpg"
    }
  ],
  "total": 125,
  "page": 1,
  "per_page": 20,
  "total_pages": 7
}
```

**Response 400:**
```json
{
  "error": "Invalid tipo parameter"
}
```

---

### GET /api/entities/news

Get news mentioning a specific entity.

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `valor` | string | required | Entity value to search |
| `tipo` | string | "persona" | Entity type |
| `page` | int | 1 | Page number |
| `per_page` | int | 20 | Items per page |
| `days` | int | 30 | Time window in days |
| `today` | bool | false | Filter to today only |
| `day_offset` | int | 0 | Day offset from today |

**Response 200:**
```json
{
  "news": [
    {
      "id": 42,
      "titulo": "Apple anuncia nuevo producto",
      "resumen": "Nuevo producto de Apple presentado...",
      "url": "https://example.com/apple-product",
      "fuente_nombre": "MacRumors"
    }
  ],
  "total": 1,
  "page": 1,
  "per_page": 20,
  "total_pages": 1
}
```

**Response 400:**
```json
{
  "error": "Entity value is required"
}
```

---

### GET /api/entities/mentions

Get mention series for entities over time.

**Query Parameters:**
- `values` - Comma-separated entity values (required)
- `days` - Time window in days (default: 30)

**Response 200:**
```json
{
  "days": 30,
  "series": [
    {
      "valor": "Apple",
      "tipo": "organizacion",
      "count": 45,
      "data": [
        {"fecha": "2024-03-15", "count": 5},
        {"fecha": "2024-03-14", "count": 3}
      ]
    }
  ]
}
```

---

### GET /api/last-names

Get distinct last names for personas.

**Response 200:**
```json
{
  "last_names": ["García", "Rodríguez", "Fernández", "López", "Martínez"],
  "total": 5000
}
```

## Alerts Endpoints

### GET /api/alertas

List alerts with optional status filter.

**Query Parameters:**
- `status` - Filter by status: nueva, leida, archivada
- `limit` - Maximum number of alerts (default: 100)

**Response 200:**
```json
{
  "alertas": [
    {
      "id": 1,
      "valor": "Inteligencia Artificial",
      "tipo": "tema",
      "periodo": "24h",
      "hits": 25,
      "baseline": 5,
      "ratio": 5.0,
      "status": "nueva",
      "created_at": "2024-03-15T10:30:00Z"
    }
  ],
  "total": 50,
  "nuevas": 15
}
```

### POST /api/alertas/:id/read

Mark single alert as read.

**Auth Required:** Yes

**Path Parameters:**
- `id` - Alert ID

**Response 200:**
```json
{
  "ok": true
}
```

### POST /api/alertas/read-all

Mark all Nueva alerts as read.

**Auth Required:** Yes

**Response 200:**
```json
{
  "ok": true
}
```

## Stats and Category/Country Endpoints

### GET /api/stats

Global statistics.

**Response 200:**
```json
{
  "total_news": 15230,
  "total_feeds": 245,
  "total_users": 890,
  "news_today": 45,
  "news_this_week": 320,
  "news_this_month": 1250,
  "total_translated": 8920,
  "top_categories": [
    {"categoria_id": 1, "categoria_nombre": "Tecnologia", "count": 4500},
    {"categoria_id": 2, "categoria_nombre": "Ciencia", "count": 2100}
  ],
  "top_countries": [
    {"pais_id": 1, "pais_nombre": "Estados Unidos", "flag_emoji": "🇺🇸", "count": 5200},
    {"pais_id": 2, "pais_nombre": "España", "flag_emoji": "🇪🇸", "count": 1800}
  ]
}
```

### GET /api/categories

List all categories.

**Response 200:**
```json
[
  {"id": 1, "nombre": "Tecnologia"},
  {"id": 2, "nombre": "Ciencia"},
  {"id": 3, "nombre": "Deporte"},
  {"id": 4, "nombre": "Politica"}
]
```

### GET /api/countries

List countries with continents.

**Response 200:**
```json
[
  {"id": 1, "nombre": "Estados Unidos", "continente": "Norteamerica", "flag_emoji": "🇺🇸"},
  {"id": 2, "nombre": "España", "continente": "Europa", "flag_emoji": "🇪🇸"},
  {"id": 3, "nombre": "Mexico", "continente": "Norteamerica", "flag_emoji": "🇲🇽"}
]
```

## Auth Endpoints (User Profile)

### GET /api/auth/me

Get current authenticated user profile.

**Auth Required:** Yes (JWT Bearer token)

**Response 200:**
```json
{
  "id": 1,
  "email": "user@example.com",
  "username": "user1",
  "is_admin": false,
  "role": "user",
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

**Response 401:**
```json
{
  "error": "Unauthorized - No token provided"
}
```
                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           