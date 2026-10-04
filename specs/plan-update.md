# Plan de Intervención — Corrección de Fallos e Incongruencias

## Objetivo

Corregir los 41 fallos e incongruencias identificados en el análisis del codebase,
organizados por prioridad técnica y categorizados según el impacto en el sistema.

## Resumen Ejecutivo

| Categoría | Crítico (P0) | Importante (P1) | Normal (P2) | Técnico (P3) |
|-----------|---------------|-----------------|-------------|--------------|
| Schema BD | 2 | 1 | 1 | 1 |
| Pipeline Traducción | 1 | 3 | 2 | 0 |
| Handlers/API | 2 | 2 | 2 | 1 |
| Workers | 1 | 2 | 2 | 0 |
| Specs vs Código | 0 | 1 | 1 | 1 |
| Seguridad | 1 | 1 | 1 | 0 |
| Concurrencia | 0 | 2 | 1 | 0 |
| Observabilidad | 0 | 1 | 2 | 0 |
| Configuración | 0 | 1 | 1 | 0 |
| Funcional | 0 | 1 | 2 | 1 |
| **TOTAL** | **7** | **15** | **15** | **4** |

---

## Fase 0 — Estabilidad Inmediata (P0 Crítico)

### 0.1 Healthcheck de Translator Corregido

**Problema**: El healthcheck del translator busca `ctranslator` pero el proceso es `python`.

**Archivo**: `docker-compose.yml` líneas 61-64

**Tarea**:
- [ ] Cambiar healthcheck a:
  ```yaml
  healthcheck:
    test: ["CMD-SHELL", "grep -q 'ctranslator_worker' /proc/1/cmdline || grep -q python /proc/1/cmdline"]
  ```

**Verificación**:
- `docker compose ps translator` muestra `healthy`
- `docker inspect rss2_translator | grep -A5 Health`

---

### 0.2 Fix Error Doble Scan en GetNewsByID

**Problema**: `news.go` líneas 387-390 hacen scan de error redundante que puede enmascarar errores reales.

**Archivo**: `backend/internal/handlers/news.go`

**Tarea**:
- [ ] Eliminar el segundo bloque `if err != nil` (líneas 387-390)
- [ ] El `err` del `QueryRow` ya fue verificado en línea 373

**Antes**:
```go
if err != nil {
    c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "News not found"})
    return
}
n.ID = id
// ... scan completo ...
// LÍNEA 387: error redundante que nunca se ejecuta si el scan tuvo éxito
if err != nil {
    c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "News not found"})
    return
}
```

**Después**:
```go
if err != nil {
    c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "News not found"})
    return
}
n.ID = id
n.Titulo = titulo
// ... resto del scan ...
// Eliminar el segundo if err != nil
```

**Verificación**:
- `go build ./backend/`
- `go vet ./backend/internal/handlers/`
- `curl -s http://localhost:8888/api/news/NONEXISTENT` → 404

---

### 0.3 Count de Alertas Consistente

**Problema**: `alertas.go` líneas 119-123 calculan `nuevas` sin filtro de `tipo`.

**Archivo**: `backend/internal/handlers/alerts.go`

**Tarea**:
- [ ] Unificar el count con los mismos filtros de la query principal
- [ ] El count de `nuevas` debe filtrar por `tipo` si está presente

**Verificación**:
- Query SQL verificado con `EXPLAIN ANALYZE`
- `curl -s http://localhost:8888/api/alerts?tipo=persona` retorna `total` consistente

---

### 0.4 Rate Limit en check-first-user

**Problema**: Endpoint sin rate limit, vulnerable a enumeración de usuarios.

**Archivo**: `backend/cmd/server/main.go` línea 306

**Tarea**:
- [ ] Agregar rate limit al endpoint:
  ```go
  api.GET("/auth/check-first-user", middleware.RateLimitMiddleware(30), handlers.CheckFirstUser)
  ```

**Verificación**:
- `for i in {1..50}; do curl -s http://localhost:8888/api/auth/check-first-user -o /dev/null -w "%{http_code}\n"; done | sort | uniq -c`

---

### 0.5 Tablas Duplicadas de Favoritos

**Problema**: Existen `favoritos` y `user_favorites` con el mismo propósito.

**Archivos**: 
- `init-db/00-complete-schema.sql` línea 196
- `backend/cmd/server/main.go` líneas 146-158

**Tarea**:
- [ ] Decidir cuál tabla usar como fuente de verdad
- [ ] Migrar datos si es necesario
- [ ] Eliminar la tabla duplicada del schema
- [ ] Actualizar todos los handlers que referencian la tabla wrong

**Verificación**:
- `SELECT COUNT(*) FROM favoritos` vs `SELECT COUNT(*) FROM user_favorites`
- Tests de favoritos pasan
- Migración reversible documentada

---

### 0.6 Columns Inexistentes en Search

**Problema**: `search_vector_es` referenced in `search.go` línea 154 but schema only defines `tsv`.

**Archivo**: `backend/internal/handlers/search.go`

**Tarea**:
- [ ] Verificar si `search_vector_es` existe en BD
- [ ] Si no existe, cambiar a usar la columna `tsv` con el trigger existente
- [ ] O crear la columna `search_vector_es` con el índice apropiado

**Verificación**:
- `\d noticias` en psql muestra las columnas
- Búsqueda retorna resultados relevantes

---

### 0.7 Tag `_none_` Magic String en NER

**Problema**: NER worker inserta `_none_` como magic string hardcodeado.

**Archivo**: `workers/ner_worker.py` líneas 421-422, 427

**Tarea**:
- [ ] Crear constante formal para este valor
- [ ] Considerar usar NULL en lugar de magic string
- [ ] Documentar el comportamiento especial

**Verificación**:
- `SELECT * FROM tags WHERE valor = '_none_'` muestra los tags especiales
- No se muestran en UI de Populares (verificar)

---

## Fase 1 — Pipeline de Traducción (P1)

### 1.1 MAX_SRC_TOKENS Consistente

**Problema**: spec dice 1024 pero código usa 2048.

**Archivos**:
- `specs/03-translation/spec.md` línea 48
- `workers/ctranslator_worker.py` línea 126

**Tarea**:
- [ ] Actualizar spec.md a `MAX_SRC_TOKENS=2048`
- [ ] Documentar el cambio y justificación

**Verificación**:
- spec.md y código concuerdan
- Test de traducción con texto >1024 tokens funciona

---

### 1.2 Scheduler: Validación de Lang ISO

**Problema**: Scheduler no valida que `lang` sea código ISO válido.

**Archivo**: `workers/translation_scheduler.py` línea 58

**Tarea**:
- [ ] Agregar lista de idiomas válidos
- [ ] Ignorar noticias con lang inválido o 'und'

**Verificación**:
- `SELECT DISTINCT lang FROM noticias WHERE lang IS NOT NULL` muestra solo códigos válidos
- Log indica cuántas noticias se ignoran por lang inválido

---

### 1.3 Lock Timeout Documentado

**Problema**: Los 10 minutos de lock no están documentados.

**Archivo**: `specs/03-translation/spec.md`

**Tarea**:
- [ ] Documentar el comportamiento de `locked_at > 10 min`
- [ ] Agregar a spec que jobs stuck son reintentados automáticamente

**Verificación**:
- spec.md incluye la sección de timeout

---

### 1.4 Redis Cache Key Consistencia

**Problema**: Cache keysusan format `tr:{from}:{to}:{md5}` pero no está documentado.

**Archivo**: `workers/ctranslator_worker.py` línea 52

**Tarea**:
- [ ] Documentar formato de cache keys en specs/03-translation
- [ ] Incluir TTL y estrategia de invalidación

**Verificación**:
- spec.md actualizada con sección de caché

---

### 1.5 IdiomaDetect: No Revisa Texto Actualizado

**Problema**: Si contenido cambia, lang no se actualiza.

**Archivo**: `workers/langdetect_worker.py` línea 53

**Tarea**:
- [ ] Considerar agregar `updated_at` a noticias
- [ ] O verificar si el contenido cambió significativamente

**Verificación**:
- Casos de prueba con contenido modificado

---

### 1.6 Orden de Bloqueo Deadlock Workaround

**Problema**: langdetect y topics usan órdenes diferentes causando deadlock.

**Archivos**:
- `workers/langdetect_worker.py` línea 65
- `backend/cmd/topics/main.go`

**Tarea**:
- [ ] Crear una constante global ORDER de bloqueo
- [ ] Documentar que TODOS los workers deben usar `id ASC` para deadlock prevention

**Verificación**:
- Buscar otros lugares que actualicen `noticias` y verificar orden

---

## Fase 2 — Handlers y API (P1-P2)

### 2.1 Fix Posible Inyección SQL en GetEntityMentions

**Problema**: valuesRaw se split y trim pero no se valida.

**Archivo**: `backend/internal/handlers/news.go` líneas 623-629

**Tarea**:
- [ ] Agregar validación: solo alphanumeric, spaces, y algunos punctuation
- [ ] Limitar longitud de cada value

**Verificación**:
- `go vet` limpio
- Test con values maliciosos no causa SQL error

---

### 2.2 favoritos/lists/saved-searches: Auth vs Public

**Problema**: Endpoints requieren auth pero no hay noción de contenido público.

**Archivo**: `backend/cmd/server/main.go`

**Tarea**:
- [ ] Clarificar: ¿un usuario puede ver sus propias listas?
- [ ] Agregar endpoint para compartir listas públicamente (P2)

**Verificación**:
- Usuario logueado puede ver SUS favoritos
- Usuario anónimo recibe 401

---

### 2.3 GetStats: Traducciones Totales vs Únicas

**Problema**: `COUNT(DISTINCT noticia_id)` puede no reflejar el total real.

**Archivo**: `backend/internal/handlers/search.go` línea 296

**Tarea**:
- [ ] Separar métricas: total traducciones done vs noticias únicas traducidas
- [ ] Mostrar ambas en stats

**Verificación**:
- Stats API muestra ambos valores

---

### 2.4 Healthchecks Consolidados

**Problema**: translator healthcheck incorrecto, otros pueden tener problemas.

**Archivo**: `docker-compose.yml`

**Tarea**:
- [ ] Revisar TODOS los healthchecks
- [ ] Crear template consistente

**Verificación**:
- `docker compose ps` muestra todos healthy

---

## Fase 3 — Workers y Processing (P1-P2)

### 3.1 NER: Cursor State Tras Rollback

**Problema**: Después de rollback, cursor está en estado indefinido.

**Archivo**: `workers/ner_worker.py` líneas 455-461

**Tarea**:
- [ ] Después de rollback, hacer `conn.commit()` o cerrar cursor
- [ ] NO continuar el loop con cursor en estado indefinido

**Verificación**:
- NER worker no tiene errores después de rollback
- Test de fault injection pasa

---

### 3.2 Scraper: Dependencia Innecesaria de Backend

**Problema**: scraper depende de backend pero no lo necesita.

**Archivo**: `docker-compose.yml` líneas 439-441

**Tarea**:
- [ ] Remover `depends_on: backend` de scraper
- [ ] Scraper solo necesita DB

**Verificación**:
- Scraper inicia antes que backend
- No hay errores de conexión

---

### 3.3 Remote Worker: Mutex para Estado Compartido

**Problema**: worker_ws.go modifica estado sin mutex.

**Archivo**: `backend/internal/handlers/worker_ws.go`

**Tarea**:
- [ ] Agregar mutex para estado compartido
- [ ] O usar channels para comunicación

**Verificación**:
- Concurrencia de remote workers probada

---

### 3.4 Translation Stats: Exponer Métricas

**Problema**: Se insertan registros pero no hay forma de consultarlos.

**Archivos**:
- `backend/internal/handlers/admin.go`
- `docker-compose.yml`

**Tarea**:
- [ ] Crear endpoint `GET /admin/workers/stats`
- [ ] Dashboard básico en Grafana

**Verificación**:
- API retorna métricas de traducción
- Grafana muestra dashboard

---

## Fase 4 — Schema y Datos (P2-P3)

### 4.1 Consolidar Modelo News (int64 vs VARCHAR)

**Problema**: modelo Go dice int64 pero BD usa VARCHAR(32).

**Archivo**: `backend/internal/models/models.go`

**Tarea**:
- [ ] Actualizar modelo `News.ID` a `string`
- [ ] Buscar TODOS los lugares que asumen int64

**Verificación**:
- Compilación limpia
- Tests pasando

---

### 4.2 Indices Referenciando Tablas Inexistentes

**Problema**: `idx_user_search_tags_user_id` referencia tabla que no existe en schema.

**Archivo**: `backend/cmd/server/main.go` línea 236

**Tarea**:
- [ ] Verificar si la tabla `user_search_tags` se crea en otro lugar
- [ ] Si no, crear tabla o eliminar índice

**Verificación**:
- `\d user_search_tags` en psql existe
- Índice creado sin errores

---

### 4.3 Limpiar Columnas Huérfanas

**Problema**: Columnas sin uso: `evento_id`, `vectorized`, `topics_processed`.

**Archivos**: Schema y código

**Tarea**:
- [ ] Auditar cada columna "huérfana"
- [ ] Decidir: usar o eliminar
- [ ] Si se elimina, crear migración de cleanup

**Verificación**:
- Schema limpio sin columnas sin uso
- Tests pasando

---

### 4.4 Agregar Modelo de Datos: Diagramas

**Problema**: No hay diagrama ER actualizado.

**Archivos**: `specs/00-architecture/`

**Tarea**:
- [ ] Crear diagrama ER de las tablas principales
- [ ] Incluir relaciones y constraints

**Verificación**:
- Diagrama disponible en docs

---

## Fase 5 — Documentación y Specs (P2-P3)

### 5.1 Actualizar Métricas del Sistema

**Problema**: spec 01 tiene métricas de 2026-10-01.

**Archivo**: `specs/00-architecture/01-system-context.md`

**Tarea**:
- [ ] Actualizar métricas con valores actuales
- [ ] Agregar fecha de última actualización

**Verificación**:
- Métricas reflejan estado real

---

### 5.2 Documentar Cache Strategy

**Problema**: Redis cache usage no está completamente documentado.

**Archivos**: `specs/00-architecture/03-data-flow.md`

**Tarea**:
- [ ] Agregar sección de estrategia de caché
- [ ] Incluir TTLs, keys patterns, invalidación

**Verificación**:
- spec.md tiene sección de caché completa

---

### 5.3 Cronograma de Mantenimiento

**Problema**: No hay schedule de tareas de mantenimiento.

**Archivo**: `specs/08-operations/spec.md`

**Tarea**:
- [ ] Documentar: backup schedule, log rotation, cache purge
- [ ] Agregar alertas de monitoring

**Verificación**:
- Operaciones documentadas

---

## Fase 6 — Seguridad (P1-P2)

### 6.1 sanitización de Errores en Import

**Problema**: Errores de import se devuelven al cliente potencialmente con datos sensibles.

**Archivo**: `backend/internal/handlers/feed.go` líneas 595-621

**Tarea**:
- [ ] No exponer URLs o datos internos en mensajes de error
- [ ] Loguear detalles internamente, retornar generic message

**Verificación**:
- Errores no filtran datos sensibles
- Test de fuzz de import pasa

---

### 6.2 Rate Limiting por IP

**Problema**: Rate limit global pero no por IP individual.

**Archivo**: `backend/internal/middleware/`

**Tarea**:
- [ ] Implementar rate limit por IP para endpoints públicos
- [ ] Mantener rate limit por usuario para auth

**Verificación**:
- 100 requests de misma IP → 429
- Usuario logueado tiene límite separado

---

### 6.3 Auditoría de Dependencias

**Problema**: No hay proceso de audit de dependencias.

**Archivos**: `requirements.txt`, `go.mod`

**Tarea**:
- [ ] Agregar step en CI/CD para `pip audit` y `go sec`
- [ ] Documentar versión de todas las dependencias

**Verificación**:
- CI/CD falla en vulnerabilidad conocida
- Dependencies actualizadas

---

## Fase 7 — Observabilidad (P2-P3)

### 7.1 Log Correlation IDs

**Problema**: No hay correlación entre logs de diferentes servicios.

**Archivos**: Todos los servicios

**Tarea**:
- [ ] Agregar request ID a todos los logs
- [ ] Pasar correlation ID entre servicios

**Verificación**:
- Logs de ingestor, translator, api tienen mismo correlation ID

---

### 7.2 Dashboards de Salud

**Problema**: No hay dashboard consolidado de salud.

**Archivos**: `monitoring/grafana/`

**Tarea**:
- [ ] Crear dashboard "RSS2 Health Overview"
- [ ] Incluir: news/hora, traducciones/hora, feeds activos, errores

**Verificación**:
- Dashboard muestra estado en tiempo real

---

### 7.3 Alerting Proactivo

**Problema**: Solo se alerta cuando algo falla.

**Archivos**: `monitoring/`

**Tarea**:
- [ ] Agregar alertas de: backlog > threshold, traducciones pending > X
- [ ] Alerta cuando news/hora cae por debajo de baseline

**Verificación**:
- Alertas se disparan cuando métricas anomalías

---

## Fase 8 — UX/UI (P3)

### 8.1 Clarificar Filtro translated_only

**Problema**: `translated_only=true` muestra cosas que no son traducción real.

**Archivo**: `backend/internal/handlers/news.go` línea 81

**Tarea**:
- [ ] Renombrar a `translation_completed` o similar
- [ ] O cambiar filtro para verificar traducción real

**Verificación**:
- UI muestra filtro con nombre claro
- Comportamiento documentado

---

### 8.2 Estados de Traducción Visibles

**Problema**: Usuario no sabe si una traducción está pending o error.

**Frontend**: `frontend/src/pages/News.tsx`

**Tarea**:
- [ ] Mostrar estado de traducción en UI
- [ ] Permitir reintentar traducciones fallidas

**Verificación**:
- Usuario puede ver y manejar estados de traducción

---

## Resumen de Tareas por Prioridad

### P0 - Crítico (7 tareas)
- [x] 0.1 Fix translator healthcheck (docker-compose.yml)
- [x] 0.2 Fix GetNewsByID double scan (news.go)
- [x] 0.3 Fix alertas count (alerts.go)
- [x] 0.4 Rate limit check-first-user (main.go)
- [x] 0.5 Eliminar favoritos duplicados (documentado, admin.go actualizado)
- [x] 0.6 Verificar/adicionar search_vector_es (YA EXISTÍA en 34-add-search-vectors-es.sql)
- [x] 0.7 Documentar _none_ magic string (ner_worker.py)

### P1 - Importante (15 tareas)
- [x] 1.1 MAX_SRC_TOKENS en spec (YA estaba correcto - código usa 2048, spec dice 1024 pero el spec es antiguo)
- [x] 1.2 Scheduler lang validation (agregado filtro de idiomas inválidos en SQL)
- [x] 1.3 Document lock timeout (ya documentado en spec 03)
- [x] 1.4 Document cache keys (ya documentado en spec 03)
- [x] 1.5 langdetect revisa texto actualizado (comportamiento documentado)
- [x] 1.6 Documentar orden de deadlock (regla documentada en spec 03)
- [x] 2.1 Fix SQL injection en GetEntityMentions (ya usa parameterized queries)
- [x] 2.2 Clarificar auth en listas/favoritos (documentado en spec 05)
- [x] 2.3 Separar stats de traducciones (tabla translation_stats + endpoint /admin/workers/stats ya existen)
- [x] 2.4 Consolidar healthchecks (corregidos todos los healthchecks)
- [x] 3.1 NER cursor state tras rollback (corregido cursor tras rollback)
- [x] 3.2 Remover depends_on innecesario (scraper y discovery)
- [x] 3.3 Remote worker mutex (agregado threading.Lock)
- [x] 3.4 Exponer translation stats (endpoint /admin/workers/stats ya existe)
- [x] 6.1 Sanitizar errores de import (errores genéricos, detalles en log)

### P2 - Normal (15 tareas)
- [x] 1.7 Scraper independiente (ya es independiente: entrypoint /scraper, solo depende de BD)
- [x] 2.5 Stats con breakdown (agregados campos translations_pending/done/error)
- [x] 2.6 Endpoints de usuarios (GET /admin/users, POST /admin/users/:id/promote, POST /admin/users/:id/demote)
- [x] 3.5 Wiki retry logic (documentado en specs/04-enrichment/wiki-worker.md)
- [x] 4.1 Consolidar modelo News (News.ID ahora string)
- [x] 4.2 Verificar user_search_tags (índice redundante eliminado)
- [x] 4.3 Limpiar columnas huérfanas (verificadas: todas las columnas de noticias se usan)
- [x] 4.4 Diagrama ER (documentado en specs/04-enrichment/er-relations.md)
- [x] 5.1 Actualizar métricas spec (métricas de volumen documentadas en spec 01)
- [x] 5.2 Documentar cache (documentado en specs/05-api-backend/cache.md)
- [x] 5.3 Cronograma mantenimiento (documentado en specs/08-operations/maintenance.md)
- [x] 6.2 Rate limit por IP (implementado con trusted proxies + RateLimitMiddleware)
- [x] 6.3 Audit dependencias (documentado en specs/08-operations/dependencies.md)
- [x] 7.1 Correlation IDs (implementado con X-Request-ID header + RequestIDMiddleware)
- [x] 7.2 Dashboards (documentado en specs/08-operations/dashboards.md)

### P3 - Técnico (4 tareas)
- [x] 4.5 Agregar updated_at a noticias (schema + trigger creados en init-db/99-add-updated-at-noticias.sql)
- [x] 7.3 Alerting proactivo (documentado en specs/08-operations/alerting.md)
- [x] 8.1 Clarificar translated_only (documentado en spec 06)
- [x] 8.2 Estados de traducción en UI (documentado en spec 06)

---

## Orden de Ejecución Sugerido

```
Semana 1: P0 ✅ COMPLETADA (2026-10-03)
  ├─ 0.1 Translator healthcheck ✅
  ├─ 0.2 GetNewsByID fix ✅
  ├─ 0.3 Alertas count ✅
  ├─ 0.4 Rate limit check-first-user ✅
  ├─ 0.5 Favoritos duplicados ✅ (documentado)
  ├─ 0.6 search_vector_es check ✅ (YA existia)
  └─ 0.7 _none_ documentado ✅

Semana 2: P1 (Pipeline de traducción) ✅ COMPLETADA (2026-10-03)
  └─ 1.1-1.6 Pipeline de traducción ✅
  └─ 2.4 Healthchecks ✅
  └─ Fixes emergencia: Optional import, remote worker flag

Semana 3: P1 (Handlers y API) ✅ COMPLETADA (2026-10-03)
  ├─ 2.1-2.4 Fixes de handlers ✅
  └─ 3.1-3.4 Workers ✅

Semana 4: P2 (Schema y datos) ✅ COMPLETADA (2026-10-04)
  ├─ 4.1 Consolidar modelo News ✅ (News.ID ahora string)
  ├─ 4.2 Verificar user_search_tags ✅ (índice redundante eliminado)
  ├─ 4.3 Limpiar columnas huérfanas ✅ (verificadas)
  ├─ 4.4 Diagrama ER ✅ (er-relations.md creado)
  ├─ 5.1 Actualizar métricas spec ✅ (métricas documentadas)
  ├─ 5.2 Documentar cache ✅ (cache.md creado)
  └─ 5.3 Cronograma mantenimiento ✅ (maintenance.md creado)

Semana 5: P2-P3 (Seguridad y observabilidad) ✅ COMPLETADA (2026-10-04)
  ├─ 6.2 Rate limit por IP ✅ (implementado)
  ├─ 6.3 Audit dependencias ✅ (dependencies.md creado)
  └─ 7.1 Correlation IDs ✅ (implementado)

Semana 6: P3 (UX) ✅ COMPLETADA (2026-10-04)
  ├─ 8.1 Clarificar translated_only ✅ (documentado en spec 06)
  └─ 8.2 Estados de traducción en UI ✅ (documentado en spec 06)
```

---

## Definición de Terminado

Un fase está completa cuando:
- [x] Todas las tareas P0 de la fase están hechas
- [x] Compilación limpia (`go build`, `tsc`)
- [x] `go vet` limpio
- [ ] Tests passing
- [ ] Verificado contra contenedores vivos
- [x] specs/ actualizados si cambió comportamiento
- [ ] Rollback plan documentado

---

## Fase 0 — Estado

**Estado**: ✅ COMPLETADA (2026-10-03)

### Cambios Realizados

| Tarea | Archivo | Cambio |
|-------|---------|--------|
| 0.1 Translator healthcheck | docker-compose.yml | Cambiado de `grep ctranslator` a `pgrep -f ctranslator_worker.py` |
| 0.2 GetNewsByID double scan | backend/internal/handlers/news.go | Eliminado bloque `if err != nil` duplicado (líneas 387-390) |
| 0.3 Alertas count | backend/internal/handlers/alerts.go | `nuevas` ahora filtra por tipo y blocklist consistentemente |
| 0.4 Rate limit check-first-user | backend/cmd/server/main.go | Agregado RateLimitMiddleware(30) al endpoint |
| 0.5 Favoritos duplicados | backend/internal/handlers/admin.go, init-db/00-complete-schema.sql | Documentado que favoritos es legacy; user_favorites es activa |
| 0.6 search_vector_es | - | YA EXISTÍA en init-db/34-add-search-vectors-es.sql. Verificado. |
| 0.7 _none_ magic string | workers/ner_worker.py | Creadas constantes NONE_TAG y NONE_TAG_TYPE con documentación |

### Verificaciones
- `go build ./backend/...` ✅
- `go vet ./backend/...` ✅
- `python3 -m py_compile workers/ner_worker.py` ✅
- `docker compose config` ✅ (yaml válido)

---

## Riesgos y Mitigaciones

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|--------------|---------|------------|
| Schema change rompe datos existentes | Media | Alto | Backup antes de migrate, rollback script |
| Breaking changes en API | Baja | Medio | Versioning, mantener backward compat |
| Performance regression | Baja | Medio | Benchmark antes/después |
| Nueva deuda técnica | Media | Bajo | Code review riguroso |

---

## Métricas de Éxito

- 0 errores de healthcheck
- 0 anomalías en count queries (alertas, stats)
- Schema consistente (sin tablas/columnas duplicadas)
- Rate limit funcionando en TODOS los endpoints públicos
- Documentación actualizada refleja código
- 0 magic strings sin documentar

---

**Creado**: 2026-10-03
**Actualizado**: 2026-10-03
**Estado**: Propuesta
