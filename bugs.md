# Análisis de Errores e Inconsistencias - RSS2

Fecha de análisis: 2026-04-01

---

## Resumen Ejecutivo

Se han identificado **13 problemas** clasificados en:
- 3 Errores Críticos
- 7 Inconsistencias  
- 3 Warnings de Arquitectura

---

## 1. Errores Críticos

### 1.1 Rate Limiting No Implementado

**Ubicación:** `backend/internal/middleware/auth.go:104-108`

**Descripción:**
La función existe pero no hace nada:
```go
func RateLimitMiddleware(requestsPerMinute int) gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Next()
    }
}
```

**Problema:**
No implementa ningún tipo de limitación de rate. Vulnerable a ataques de fuerza bruta y DoS.

**Impacto:** Alto - Seguridad comprometida.

---

### 1.2 CORS Demasiado Permisivo

**Ubicación:** `backend/internal/middleware/auth.go:77-90`

**Descripción:**
```go
c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
```

**Problema:**
Permite cualquier origen, lo cual es inseguro para APIs de producción. Permite ataques CSRF y expone datos a orígenes maliciosos.

**Impacto:** Alto - Seguridad comprometida.

---

### 1.3 Secret Key Por Defecto Hardcodeada

**Ubicación:** `backend/internal/config/config.go:32`

**Descripción:**
```go
SecretKey: getEnv("SECRET_KEY", "change-this-secret-key"),
```

**Problema:**
Si no se proporciona la variable de entorno, la aplicación usa una clave por defecto conocida. Cualquier despliegue sin configurar esta variable es vulnerable.

**Impacto:** Crítico - Compromiso total de autenticación.

---

## 2. Inconsistencias

### 2.1 Mapeo de Campos Inconsistente (Snake Case vs Camel Case)

**Descripción:**
La aplicación usa convenciones de nomenclatura diferentes en distintas capas:

| Capa | Estructura | Ejemplo |
|------|------------|---------|
| Models | Snake case | `Title`, `Summary`, `ImageURL` |
| Handlers | Pascal case | `Titulo`, `Resumen`, `ImagenURL` |
| Frontend TS | camelCase | `titulo`, `resumen`, `imagen_url` |

**Archivos afectados:**
- `backend/internal/models/models.go:7-19`
- `backend/internal/handlers/news.go:14-28`
- `frontend/src/services/api.ts:20-34`

**Problema:**
Dificulta el mantenimiento y genera potenciales errores de serialización.

---

### 2.2 Idioma de Traducción Hardcodeado

**Descripción:**
El idioma destino de traducción (`es`) está hardcodeado en múltiples lugares:

**news.go:67:**
```go
where += " AND t.status = 'done' AND t.titulo_trad IS NOT NULL"
```

**news.go:96:**
```go
LEFT JOIN traducciones t ON t.noticia_id = n.id AND t.lang_to = 'es'
```

**search.go:34-37:**
```go
if lang == "" {
    lang = "es"
}
```

**Problema:**
No es configurable. Si se quiere traducir a otros idiomas, requiere cambios en múltiples archivos.

---

### 2.3 Columna `role` en Users Sin映射 en Modelo

**Descripción:**
Se añade una columna `role` a la tabla users:

**cmd/server/main.go:41-48:**
```go
_, err = db.GetPool().Exec(ctx, `
    ALTER TABLE users ADD COLUMN IF NOT EXISTS role VARCHAR(20) DEFAULT 'user'
`)
```

**Pero el modelo User no tiene este campo:**

**models/models.go:86-94:**
```go
type User struct {
    ID           int64     `json:"id"`
    Email        string    `json:"email"`
    Username     string    `json:"username"`
    PasswordHash string    `json:"-"`
    IsAdmin      bool      `json:"is_admin"`
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}
```

**Problema:**
La columna `role` nunca se usa realmente. Hay dos campos (`role` y `is_admin`) que pueden causar confusión.

---

### 2.4 Feature No Implementada en Discovery

**Ubicación:** `backend/cmd/discovery/main.go:129-133`

**Descripción:**
```go
func findFeedLinksInHTML(baseURL string) ([]string, error) {
    // Simple feed link finder - returns empty for now
    // In production, use goquery to parse HTML and find RSS/Atom links
    return []string{}, nil
}
```

**Problema:**
El worker de discovery no puede encontrar feeds RSS embebidos en páginas HTML. Solo detecta feeds directos.

**Impacto:** Medio - Reduce la efectividad del descubrimiento de feeds.

---

### 2.5 LoggerMiddleware No Funcional

**Ubicación:** `backend/internal/middleware/auth.go:93-101`

**Descripción:**
```go
func LoggerMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Next()

        status := c.Writer.Status()
        if status >= 400 {
            // Log error responses - pero no hace nada!
        }
    }
}
```

**Problema:**
El logging de errores está commented out. No se registra nada.

---

### 2.6 Columnas de BD Diferentes a Modelos

**Descripción:**
El handler `search.go` referencia columnas que no existen en los modelos:

**search.go:91-92:**
```go
SELECT n.id, n.titulo, n.resumen, n.contenido, n.url, n.imagen, 
       n.feed_id, n.lang, n.categoria_id, n.pais_id, n.created_at, n.updated_at,
```

**Pero en models.go:7-19:**
```go
type News struct {
    ID          int64      `json:"id"`
    Title       string     `json:"title"`
    Summary     string     `json:"summary"`
    Content     string     `json:"content"`
    URL         string     `json:"url"`
    ImageURL    *string    `json:"image_url"`
    PublishedAt *time.Time `json:"published_at"`
    Lang        string     `json:"lang"`
    FeedID      int64      `json:"feed_id"`
    ...
}
```

**Problema:**
- Modelo usa `Title`, BD usa `titulo`
- Modelo usa `Summary`, BD usa `resumen`
- Modelo usa `ImageURL`, BD usa `imagen`
- Modelo usa `PublishedAt`, BD usa `fecha`

Esto causa errores de serialización potenciales.

---

### 2.7 Inicialización de Base de Datos Duplicada

**Descripción:**
Existen dos formas de conectar a la base de datos:

1. `db.Connect()` en `backend/internal/db/postgres.go:13`
2. `workers.Connect()` en `backend/internal/workers/db.go`

**Problema:**
Códigos duplicados que mantienen conexiones separadas. Puede causar problemas de pool de conexiones.

---

## 3. Warnings de Arquitectura

### 3.1 Creación de Tablas en Runtime

**Ubicación:** `backend/cmd/server/main.go:20-77`

**Descripción:**
En lugar de usar migraciones formales, el servidor crea tablas al iniciar:

```go
func initDB() {
    _, err := db.GetPool().Exec(ctx, `
        CREATE TABLE IF NOT EXISTS entity_aliases (...)
    `)
    // más tablas...
}
```

**Problema:**
- No hay control de versiones
- No hay rollback
- Difícil de mantener en producción

**Recomendación:** Usar golang-migrate o similar.

---

### 3.2 No Hay Tests Unitarios

**Descripción:**
No se encontraron archivos `*_test.go` en el código backend.

**Problema:**
Sin tests, cualquier cambio puede romper funcionalidad existente sin detección.

---

### 3.3 Pool de Conexiones Sin Validación Periódica

**Ubicación:** `backend/internal/db/postgres.go`

**Descripción:**
El pool se crea pero no hay health check periódico.

**Problema:**
Las conexiones zombie pueden acumularse sin detección.

---

## 4. Recomendaciones de Corrección

### Alta Prioridad

1. **Unificar JWT Secret:** Usar un único package para manejar el secret
2. **Implementar Rate Limiting:** Usar una librería como `golang.org/x/time/rate`
3. **Configurar CORS correctamente:** Usar variable de entorno para orígenes permitidos
4. **Forzar Secret Key:** Hacer requerida la variable `SECRET_KEY`

### Media Prioridad

5. **Traducir idioma configurable:** Pasar idioma como parámetro o configuración
6. **Usar migraciones:** Implementar golang-migrate
7. **Añadir tests:** Crear tests unitarios para handlers críticos
8. **Corregir mapeo de campos:** Estandarizar snake_case en toda la aplicación

### Baja Prioridad

9. **Implementar LoggerMiddleware:** Añadir logging real
10. **Implementar findFeedLinksInHTML:** Usar goquery para parsing HTML
11. **Consolidar DB:** Unificar inicialización de base de datos

---

## 5. Estadísticas

| Tipo | Cantidad |
|------|----------|
| Errores Críticos | 3 |
| Inconsistencias | 7 |
| Warnings | 3 |
| **Total** | **13** |

---

## 6. Problemas Adicionales Detectados

### 6.1 Mapeo de Campos Inconsistente (Frontend)

**Descripción:** El frontend usa `snake_case` pero los modelos Go usan `PascalCase`.

**Archivos:**
- `frontend/src/services/api.ts` - usa `titulo`, `resumen`
- `backend/internal/models/models.go` - usa `Title`, `Summary`

---

### 6.2 Structs Duplicados para Mismo Recurso

**Descripción:** `handlers/news.go` define `NewsResponse` diferente a `models/models.go:News`.

---

### 6.3 Query Params Inconsistentes

**Descripción:** Algunos handlers usan `category_id` (inglés) y otros `categoria_id` (español).

---

### 6.4 Duplicación Config DB

**Descripción:** API usa `DATABASE_URL`, workers usan `DB_HOST`, `DB_PORT`, etc.

---

### 6.5 Sin Health Checks en Workers

**Descripción:** Workers no implementan endpoint `/health`.

---

### 6.6 Manejo de Errores Inconsistente

**Descripción:** Mezcla de `gin.H{"error": ...}` y `models.ErrorResponse`.

---

## 7. Errores y Problemas Críticos Detectados

### 7.1 ~~Errores Ignorados Silenciosamente~~ **(RESUELTO)**

**Descripción:** Múltiples lugares donde errores eran ignorados sin logging.

**Ejemplos corregidos:**
- `admin.go:488-492` - Ahora verifica errores de `db.GetPool().Exec()`
- `admin.go:522` - Verifica error al actualizar estado

**Impacto:** Fallos silenciosos que pasan desapercibidos.

---

### 7.2 ~~Rutas Hardcodeadas~~ **(RESUELTO)**

**Descripción:** Rutas de archivos hardcodeadas en el código.

**Correcciones:**
- `admin.go:469,474,509` - Ahora usa `config.Load().DockerComposeDir`
- `server/main.go:113` - Ahora usa `cfg.WikiImagesPath`
- Añadidos nuevos campos en config: `DockerComposeDir`, `WikiImagesPath`

---

## 7.3 Secret Key Por Defecto

**Descripción:** `config.go:32` usa clave por defecto known.

```go
SecretKey: getEnv("SECRET_KEY", "change-this-secret-key")
```

**Impacto:** Medio - Si no se configura, usar clave insegura.

---

### 7.4 Rate Limiting No Implementado

**Descripción:** `middleware/auth.go:104-108` tiene función vacía.

**Impacto:** Alto - Vulnerable a ataques de fuerza bruta.

---

### 7.5 CORS Demasiado Permisivo

**Descripción:** `middleware/auth.go:79` permite cualquier origen.

**Impacto:** Alto - Expuesto a ataques CSRF.

---

## 8. Recomendaciones de Corrección (Actualizado)

### Alta Prioridad (CRÍTICO)

1. ~~Unificar JWT Secret~~ ✅
2. Implementar Rate Limiting
3. ~~**Configurar CORS correctamente:** Usar variable de entorno~~ ✅
4. ~~**Corregir manejo de errores:** No ignorar silenciosamente~~ ✅
5. ~~**Eliminar rutas hardcodeadas**~~ ✅

### Media Prioridad

7. Idioma traducciones configurable
8. Usar migraciones
9. Añadir tests
10. Corregir mapeo de campos

### Baja Prioridad

11. Implementar LoggerMiddleware
12. Implementar findFeedLinksInHTML
13. Consolidar DB
14. Estandarizar respuestas API
15. Añadir health checks
