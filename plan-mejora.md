# Plan de Mejora - RSS2 Application

## Análisis Complejo

Se ha realizado un análisis exhaustivo identificando **35 problemas** en total, clasificados a partir de bugs.md y revisión de código:

- **6 Errores Críticos** (seguridad/estabilidad)
- **10 Inconsistencias** (funcionalidad)
- **5 Warnings de Arquitectura** (mantenibilidad)
- **14 Problemas Adicionales** (UI/funcionalidad/performance)

---

## ⚠️ Problemas Adicionales Detectados (Nuevos)

### 🔴 Críticos (2 nuevos)

#### A. Sin Validación de Entrada en Endpoints Públicos
- **Archivos:** `handlers/feed.go`, `handlers/news.go`, `handlers/search.go`
- **Descripción:** Endpoints aceptan parámetros sin validación estricta (ej. `page`, `per_page` sin límites duros, SQL injection risk en `fmt.Sprintf`)
- **Impacto:** Posible DoS via parámetros grandes, inyección SQL si cambian queries
- **Acción:** Usar validator library (`go-playground/validator/v10`), validar y sanitizar todos los inputs

#### B. Transacciones BD sin Manejo de Errores Adecuado
- **Archivos:** `handlers/feed.go:559-592` (ImportFeeds)
- **Descripción:** `SAVEPOINT`/`ROLLBACK TO SAVEPOINT`/`RELEASE SAVEPOINT` sin verificar errores en cada paso
- **Impacto:** Transacciones parcialmente aplicadas si falla a mitad
- **Acción:** Verificar error en cada `Exec`, usar `tx.Rollback()` en defer

### 🟡 Alta Priority (3 nuevos)

#### C. N+1 Query Problem en Entity Loading
- **Archivos:** `handlers/news.go:314-335` (GetNewsByID)
- **Descripción:** Query separada para entities por cada noticia
- **Impacto:** Latencia alta al cargar detalle de noticia
- **Acción:** JOIN en query principal o batch loading

#### D. Pool de Conexiones Sin Configuración Óptima
- **Archivos:** `backend/internal/db/postgres.go`
- **Descripción:** Pool creado con defaults, sin `MaxConns`, `MinConns`, `MaxConnLifetime`
- **Impacto:** Agotamiento de conexiones bajo carga, conexiones zombie
- **Acción:** Configurar pool size según cores/load, agregar health checks periódicos

#### E. Falta de Índices BD en Tablas Críticas
- **Archivos:** `init-db/` + `cmd/server/main.go` (initDB)
- **Descripción:** Tablas `noticias`, `traducciones`, `tags_noticia` sin índices en FKs y columnas de filtro
- **Impacto:** Queries lentas en producción con datos reales
- **Acción:** Añadir índices en `noticias.fecha`, `noticias.fuente_nombre`, `traducciones.noticia_id+lang_to`, `tags_noticia.noticia_id+tag_id`

#### F. Workers Python Sin Graceful Shutdown
- **Ubicación:** `workers/*.py`
- **Descripción:** Workers no manejan SIGTERM/SIGINT, terminan abruptamente
- **Impacto:** Tareas a medio procesar, estados inconsistentes
- **Acción:** Implementar signal handling, graceful shutdown con timeout

### 🟢 Media Priority (2 nuevos)

#### G. Cache Inconsistente / Sin Cache Strategy
- **Ubicación:** `backend/internal/cache/redis.go` + handlers
- **Descripción:** Redis disponible pero uso esporádico, sin TTL estándar, sin invalidación
- **Impacto:** Queries repetidas costosas, datos stale
- **Acción:** Definir cache keys + TTL estándar, invalidar en writes, usar en feeds/categorías/paises

#### H. Logging Estructurado Ausente
- **Ubicación:** Todo el backend
- **Descripción:** Solo `fmt.Println`/`log.Printf`, sin niveles, sin JSON, sin correlation IDs
- **Impacto:** Debugging difícil en producción
- **Acción:** Migrar a `zerolog` o `zap` con JSON output, request IDs

### 🔵 Baja Priority (2 nuevos)

#### I. Documentación API (OpenAPI/Swagger) Ausente
- **Ubicación:** Backend handlers
- **Impacto:** Frontend y consumers no tienen spec autogenerada
- **Acción:** Añadir anotaciones `swaggo/swag` o `go-swagger`, generar `/swagger` endpoint

#### J. Secrets en Variables Entorno Sin Validación
- **Ubicación:** `config/config.go` + docker-compose
- **Descripción:** Variables como `DB_PASS`, `REDIS_PASSWORD`, `SECRET_KEY` sin validar presencia/formato
- **Impacto:** Fallos en runtime confusos si faltan
- **Acción:** Validar todas las vars requeridas al startup, fallar rápido con mensaje claro

---

## 🔴 Crítico Priority (Seguridad y Estabilidad)

### 1. Rate Limiting No Implementado
- **Archivo:** `backend/internal/middleware/auth.go:104-108`
- **Descripción:** Función vacía que no limita requests
- **Impacto:** Vulnerable a ataques DoS y fuerza bruta
- **Acción:** Implementar usando `golang.org/x/time/rate`

### 2. CORS Demasiado Permisivo
- **Archivo:** `backend/internal/middleware/auth.go:77-90`
- **Descripción:** `Access-Control-Allow-Origin: *`
- **Impacto:** CSRF, exposición de datos a orígenes maliciosos
- **Acción:** Configurar orígenes permitidos desde variable de entorno

### 3. Secret Key Por Defecto Hardcodeada
- **Archivo:** `backend/internal/config/config.go:32`
- **Descripción:** `SecretKey: getEnv("SECRET_KEY", "change-this-secret-key")`
- **Impacto:** Compromiso total de autenticación si no se configura
- **Acción:** Hacer `SECRET_KEY` obligatoria, panic si no está set

### 4. Columna `role` en Users Sin Mapeo en Modelo
- **Archivos:** `models/models.go:86-94`, `cmd/server/main.go:41-48`
- **Descripción:** Columna `role` en BD pero no en struct User
- **Impacto:** Confusión entre `role` y `is_admin`
- **Acción:** Añadir campo `Role` a modelo User o remover columna BD

### 5. LoggerMiddleware No Funcional
- **Archivo:** `middleware/auth.go:93-101`
- **Descripción:** `c.Next()` sin logging de errores
- **Impacto:** Errores van sin registrar
- **Acción:** Implementar logging real de respuestas >= 400

### 6. Traducción Idioma Hardcodeado 'es'
- **Archivo:** `handlers/news.go:75,96`
- **Descripción:** `t.lang_to = 'es'` en múltiples queries
- **Impacto:** No configurable a otros idiomas
- **Acción:** Pasar idioma como parámetro o configuración

---

## 🟡 Alta Priority (Funcionalidad y Estabilidad)

### 7. Discovery HTML Feed Finder Vacio
- **Archivo:** `backend/cmd/discovery/main.go:129-133`
- **Descripción:** `findFeedLinksInHTML` retorna `[]string{}, nil`
- **Impacto:** No detecta feeds en HTML, solo URLsdirectas
- **Acción:** Implementar con `goquery` para parsear HTML

### 8. Mapeo de Campos DB vs Modelo Inconsistente
- **Archivo:** `search.go` vs `models/models.go:7-19`
- **Descripción:** Columnas `titulo`/`resumen`/`imagen`/`fecha` vs `Title`/`Summary`/`ImageURL`/`PublishedAt`
- **Impacto:** Errores de serialización potenciales
- **Acción:** Estandizar nombres en toda la aplicación

### 9. No Hay Tests Unitarios
- **Ubicación:** Todo el backend
- **Impacto:** Cambios pueden romper funcionalidad sin detección
- **Acción:** Crear tests para handlers críticos (auth, news, feeds)

### 10. Inicialización BD Duplicada
- **Archivos:** `db.Connect()` vs `workers.Connect()`
- **Impacto:** Piscs de conexión separadas, posible agotamiento
- **Acción:** Consolidar en una sola conexión/pool

### 11. Parámetros Query Inconsistentes
- **Descripción:** Algunos usan `category_id` vs `categoria_id`
- **Impacto:** Confusión en filtros
- **Acción:** Unificar nomenclatura (preferir `category_id` en inglés)

### 12. API Field Naming Inconsistente
- **Archivo:** `frontend/src/services/api.ts`
- **Descripción:** Interface `News` usa camelCase, pero respuestas van snake_case
- **Impacto:** Datos no se muestran correctamente
- **Acción:** Arreglar mapping en api.ts o estandarizar respuestas Go

---

## 🟢 Media Priority (Calidad y Mantenimiento)

### 13. No Hay Health Checks en Workers
- **Ubicación:** Todos los workers Python/Go
- **Impacto:** Dificultad para monitorear salud de workers
- **Acción:** Implementar endpoint `/health` en cada worker

### 14. Structs Duplicadas Mismo Recurso
- **Archivo:** `handlers/news.go` vs `models/models.go:News`
- **Descripción:** `NewsResponse` diferente a `models.News`
- **Impacto:** Mantenimiento duplicado
- **Acción:** Unificar o mantener mapping explícito

### 15. Query Params Inconsistentes (category_id vs categoria_id)
- **Descripción:** Mix de inglés y español en handlers
- **Impacto:** Confusión para usuarios de la API
- **Acción:** Definir estándar y aplicarlo en todos lados

### 16. Duplicación Config DB
- **Descripción:** API usa `DATABASE_URL`, workers usan `DB_HOST`/`DB_PORT` etc.
- **Impacto:** Configuración fragmentada
- **Acción:** Unificar variables de entorno

---

## 🔵 Baja Priority (UX y Refactoring)

### 17. Frontend API Field Mapping Fixes
- **Archivo:** `frontend/src/services/api.ts`
- **Impacto:** Datos se muestran incorrectamente o undefined
- **Acción:** Revisar y corregir tipos de interfaz

### 18. Mejorar Estados de Error en UI
- **Ubicación:** Todas las páginas frontend
- **Impacto:** UX pobre cuando fallan requests
- **Acción:** Mensajes de error claros y acciones de recuperación

### 19. Skeleton Loaders en lugar de Spinners
- **Ubicación:** Componente NewsList, News detail, etc.
- **Impacto:** Perceived performance mejorada
- **Acción:** Implementar componentes skeleton con Tailwind

### 20. Diseño Responsivo Mejorado
- **Ubicación:** Grid de noticias, selectores, formularios
- **Impacto:** Mejor experiencia móvil/tablet
- **Acción:** Refinar clases Tailwind, breakpoints

### 21. Tipografía y Esquema de Color Consistente
- **Ubicación:** Todo el frontend
- **Impacto:** Aspecto amateur, inconsistencia visual
- **Acción:** Definir palette principal, usar consistentemente

### 22. Micro-interacciones y Transiciones Más Suaves
- **Ubicación:** Hover effects, focus states, button animations
- **Impacto:** Experiencia más pulida y profesional
- **Acción:** Agregar transitions, hover: clases, focus-visible

### 23. Mejoras de Accesibilidad
- **Ubicación:** Componentes form, links, imágenes
- **Impacto:** Cumplimiento WCAG, mejor usabilidad
- **Acción:** ARIA labels, contrast ratios, focus outlines, tabindex

### 24. Estados Vacios Mejorados
- **Ubicación:** Páginas sin resultados, sin noticias, estados iniciales
- **Impacto:** UX cuando no hay datos que mostrar
- **Acción:** Mensajes descriptivos, ilustraciones, acciones sugeridas

### 25. Skeletons de Carga por Componente
- **Ubicación:** News cards, feeds lists, entity lists
- **Impacto:** Indicar progreso mejor que spinners puros
- **Acción:** Implementar skeleton grids con dimensiones reales

---

## Resumen Ejecutivo

| Prioridad | Cantidad | Enfoque |
|-----------|----------|---------|
| 🔴 Crítico | 8 | Seguridad, estabilidad, validación |
| 🟡 Alta | 10 | Funcionalidad, bugs, performance |
| 🟢 Media | 8 | Mantenimiento, calidad, observabilidad |
| 🔵 Baja | 8 | UX, refinamientos, documentación |

**Total: 35 problemas**

**Primeros pasos recomendados (Sprint 1 - Seguridad + Estabilidad):**
1. Rate limiting + CORS + Secret key (seguridad inmediata)
2. Validación de entrada en endpoints públicos
3. Transacciones BD con manejo de errores correcto
4. Índices BD en tablas críticas

**Sprint 2 - Funcionalidad + Performance:**
5. Arreglar naming conventions (consistencia en todo)
6. Pool de conexiones optimizado + health checks
7. N+1 queries en entity loading
8. Cache strategy + invalidación

**Sprint 3 - Calidad + Observabilidad:**
9. Logging estructurado (zerolog/zap)
10. Tests unitarios para handlers críticos
11. Graceful shutdown en workers
12. Health checks en todos los workers

**Sprint 4 - UX + Documentación:**
13. OpenAPI/Swagger docs
14. Skeleton loaders + estados vacíos
15. Accesibilidad + micro-interacciones
16. Traducción idioma configurable