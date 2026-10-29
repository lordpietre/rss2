# RSS2 — Hoja de Ruta

> Vista de **qué existe**, **qué falta** y **en qué orden**.
> El detalle técnico de cada corrección está en `REMEDIATION_PLAN.md`; el catálogo de problemas,
> en `KNOWN_ISSUES.md`.
> Estado: **2026-10-26**.

---

## 1. Funcionalidades existentes (verificadas)

### 1.1 Núcleo

| Funcionalidad | Estado | Dónde | Notas |
|---|---|---|---|
| Ingesta RSS/Atom multi-feed | ✅ Operativa | `rss-ingestor-go/` | 28.250 feeds activos, ~441/min en pico |
| Deduplicación por URL normalizada | ✅ Operativa | `rss-ingestor-go/main.go:240-279` | id = md5(URL normalizada) |
| Saneado de texto (`textclean`) | ✅ Operativo | `backend/internal/textclean/` | 28 tests + fuzz; paridad verificada |
| Detección de idioma | ✅ Operativa | `workers/langdetect_worker.py` | 294.796 noticias aún sin idioma (cola) |
| Traducción ES (NLLB / CTranslate2 int8) | ⚠️ **Parcial** | `translator/workers/ctranslator_worker.py` | **3,3 % del corpus traducido** (ver ISS-001) |
| Búsqueda textual (FTS español) | ✅ Operativa | `handlers/search.go` | GIN sobre `search_vector_es` |
| Búsqueda semántica (pgvector 384 d) | ✅ Operativa | `services/ml.go:161-341` | Cubre traducciones **y** originales en español |
| NER (spaCy es/en) | ✅ Operativa | `workers/ner_worker.py` | 165.665 entidades, 330.282 relaciones |
| Grafo de entidades y co-ocurrencias | ✅ Operativo | `handlers/news.go:1327`, `cmd/cooccurrences/` | 36.675 pares |
| Alertas por pico horario | ✅ Operativa | `handlers/alerts.go` | 85 alertas; baseline ponderado |
| Clusterización de eventos | ✅ Operativa | `cmd/event_clustering/` | 21.886 clusters |
| Noticias relacionadas | ⚠️ **Vacía** | `cmd/related/` | `related_noticias` = 0 filas (ISS-013) |
| Clasificación temática | ⚠️ **Inactiva** | `cmd/topics/` | `topics` = 0 filas (ISS-012) |
| Enriquecimiento con Wikipedia | ✅ Operativo | `cmd/wiki_worker/` | Resumen + miniatura + caché Redis 48 h |
| Enriquecimiento de contenido (scraper) | ⚠️ **Débil** | `cmd/scraper/` | Sólo 668 noticias con `contenido` |
| Descubrimiento de feeds | ⚠️ **Parcial** | `cmd/discovery/` | `findFeedLinksInHTML` es un stub |
| Tendencias / tendencias para el noticiario | ✅ Operativa | `handlers/tts_trends.go` | Depende de `topics` (ISS-012) |

### 1.2 API y producto

| Funcionalidad | Estado |
|---|---|
| ~115 rutas REST con JWT, roles de admin y rate limit por IP | ✅ |
| Búsqueda con filtros (`desde`, `min_chars`, `dedup=cluster`, `translated_only`, `topic_id`) | ✅ |
| Favoritos, listas, búsquedas guardadas | ✅ |
| Panel de administración (usuarios, alias, backup/restore, reset, workers) | ✅ |
| Suscripciones de app (Android) y límites de plan | ✅ |
| Importación/exportación CSV de feeds | ✅ |
| Borrado masivo con precondición 428 | ✅ |
| WebSocket para workers remotos | ✅ Código · ❌ 0 hosts operativos |
| **Frontend: 15 rutas activas** | ✅ |
| **Tests del frontend: 2 ficheros / 18 casos** | ⚠️ No se pueden ejecutar (falta Node.js) |

### 1.3 Noticiario de audio

| Funcionalidad | Estado |
|---|---|
| Búsqueda con filtros de narrabilidad | ✅ |
| Selección y reordenación de la parrilla | ⚠️ En memoria (se pierde al refrescar) |
| Creación de guiones y edición del texto | ✅ (sin notas personales) |
| Normalización del texto a locutar | ✅ (delegada al servicio TTS) |
| Aprobación y encolado | ✅ |
| 3 motores TTS instalados (Chatterbox, MeloTTS, OpenVoice) | ✅ — sólo Chatterbox en uso |
| Clonación de voz (referencia de audio) | ✅ Chatterbox · ⚠️ sin registro de consentimiento |
| Registro de voces y catálogo de hablantes | ✅ (23 voces) |
| Control de calidad de la muestra de voz | ✅ |
| Ensamblado del boletín con silencios | ✅ (verificado: 6 items, 138,04 s) |
| Reproducción y descarga del WAV | ✅ |
| Watchdog de trabajos atascados | ✅ |
| Cola Redis del TTS | ⚠️ Desconectada (ISS-008) |
| **Rendimiento** | 🔴 **18,6× tiempo real** (ISS-002) |
| Persistencia de la parrilla | ❌ |
| Notas personales por guion | ❌ |
| Reordenamiento de un boletín ya creado | ❌ |
| Aprobación parcial por item | ❌ |

---

## 2. Trabajo pendiente

> Orden canónico: `REMEDIATION_PLAN.md`. Resumen por oleada:

| Oleada | Contenido | Esfuerzo | Depende de |
|---|---|---|---|
| **W0** | Tests verdes (arreglar 2 tests + re-sembrar el blocklist + Node.js) | 1–3 h | — |
| **W1** | Métricas Prometheus + panel de backlog | 4–8 h | — |
| **W2** | Bugs de datos (`feed_id`, `ensure_schema`, semilla, idempotency) | 3–6 h | W0 |
| **W3** | Rendimiento e I/O (índices, `embedding_vector`, `LimitReader`, `OF t`) | 6–12 h | W1 |
| **W4** | Configuración real + limpieza de código muerto | 4–8 h | — |
| **W5** | Seguridad (rol admin, expiración del JWT, confirmación en `reset-db`) | 4–8 h | — |
| **W6** | Noticiario: persistencia + notas + consentimiento + refactor a componentes | 10–20 h | W0, migración |
| **W7** | Escala de traducción: `OF t` + presupuesto de tokens → remote workers → GPU | 30 min → 3 días | **Decidido** (A-26/A-31). Hardware para W7.3 |
| **W8** | TTS con Chatterbox: usabilidad (truncar guion) → GPU → retirar MeloTTS/OpenVoice | 4 h → 2 días | **Decidido** (A-27). Hardware para W8.2 |

---

## 3. Mejoras propuestas

### 3.1 Producto

| # | Mejora | Motivo | Bloqueada por |
|---|---|---|---|
| 1 | **Reducir el corpus activo** (28.250 feeds → 17.090 que producen; 28 noticias/feed/día) | La ingesta (331/min) supera a la traducción (31/min) 11×. **Es la palanca más barata**: reduce la demanda en la misma proporción | P-10 (decisión del propietario) |
| 2 | Traducir **título + resumen** y dejar el contenido bajo demanda | Hace visible el texto mucho antes. ⚠️ El contenido está vacío en el 99,9 % de las filas, así que el beneficio real está en el resumen | Medición de calidad |
| 3 | **Priorizar por valor** en vez de sólo por fecha | El backlog nunca se drena con `ORDER BY fecha DESC` | — |
| 4 | Persistir la parrilla del noticiario | Se pierde al refrescar | W6 |
| 5 | Notas personales por guion (no se locutan) | Requisito explícito del propietario | W6 (migración) |
| 6 | Registro de consentimiento de voces clonadas | Requisito legal | W6 + P-04 |
| 7 | Aprobación parcial y reordenamiento de boletines | Flujo editorial más fino | W6 |
| 8 | Filtro por tema en el Noticiario (los chips son inertes) | La detección de tendencias no es accionable | ISS-012 (`topics` vacía) |
| 9 | Política de **retención** de `noticias` | +1 GB/día sin límite | P-06 |
| 10 | Retry/re-encolado automático de las traducciones en `error` | `status='error'` es terminal | — |

### 3.2 Ingeniería

| # | Mejora | Motivo |
|---|---|---|
| 11 | **CI** que ejecute `make test`, `make parity`, `make tts-test` y `npm test` | No hay CI; `make test` estaba en rojo |
| 12 | Test de la regla de parámetros posicionales (`$1` = idioma destino) | Ya rompió la búsqueda semántica una vez |
| 13 | Test de integración del `BulkDeleteFeeds` (428) | Es la única protección real |
| 14 | Test del protocolo WebSocket de workers remotos | 0 hosts operativos: el código no se ha ejercitado en producción |
| 15 | Tests para los workers Python (hoy **0**) | `translation_scheduler`, `langdetect`, `ner`, `embeddings`, `ctranslator` sin cobertura |
| 16 | Test de paridad del DDL general (`init-db/` vs. `migrations/`) | `make check-schema` existe pero sólo compara con la BD viva |
| 17 | `pg_stat_statements` habilitado | Sin él no se puede atribuir la tormenta de `feeds` (ISS-003) |
| 18 | Reducir los índices solapados de `traducciones` (5) y `tags_noticia` (3) | Escrituras más rápidas |
| 19 | `OFFSET` → paginación por cursor en las listas grandes | `OFFSET` profundo es insostenible con 814 K filas |
| 20 | Migrar `noticias` a particionamiento | ⚠️ **No viable**: el id es un md5 no monotónico (ver `DATABASE.md` §6.3) |

---

## 4. Dependencias entre tareas

```
W0 (tests verdes)
 │
 ├──► W2 (bugs de datos) ─────────────► W3 (rendimiento, con W1 midiendo)
 │       │                                      │
 │       └──► W6.0 (migración TTS) ──► W6.1/6.2 (API + UI) ──► W6.3 (refactor a componentes)
 │                                                        │
 ├──► W1 (métricas) ──────────────────────► W3, W7, W8    │
 │                                                        ▼
 └──► W4 (configuración) ──► W10 (eliminar código muerto)  W6 (notas → B-05, B-06 recomendado)

DECISIONES TOMADAS (2026-10-26) — ya no bloquean:
  A-26 "se traduce todo" + "primero lo más reciente" ──► W7 (objetivo: ≥380/min)
  A-27 "motor TTS = Chatterbox"   ──► W8 (objetivo: <0,5× tiempo real en GPU)
  A-28 "voces estándar y clonada"  ──► W8 (B-06 pasa a recomendado)

Decisiones aún abiertas (DECISIONS.md Parte C):
  P-02 (retraso aceptable) ──► dimensiona W7.2
  P-05 (dónde desplegar)   ──► condiciona W7.2 (workers remotos)
  P-06 (retención)          ──► condiciona W7 (el histórico sin traducir crece)
  P-07 (aislar el TTS)      ──► condiciona W8.2
  P-09 (horizonte del backlog) ──► dimensiona W7.2
  P-10 (recortar la oferta)  ──► palanca más barata de W7
```

**Reglas duras:**
1. **Nada de W2–W8 antes de W0.** Con los tests rojos no se puede distinguir una regresión de un
   fallo preexistente.
2. **Nada de W3/W7/W8 sin W1.** Sin métricas, "optimizar" es una opinión.
3. **W6.0 (migración) es un commit propio**, anterior a la funcionalidad, y reversible.
4. **W7 y W8 no empiezan sin la decisión del propietario** que las bloquea.
5. `textclean` no se toca sin `make parity` en el mismo commit.

---

## 5. Qué **no** está planificado (y por qué)

| Idea | Motivo |
|---|---|
| Sustituir PostgreSQL | No hay problema demostrado que lo justifique. El cuello de botella es **CPU de traducción**, no la base de datos |
| Sustituir Redis | Funciona como fail-open y es coherente con el diseño. El problema real es la CPU, no la caché |
| Particionar `noticias` | El id es un md5 no monotónico ⇒ incompatible con el particionado por fecha |
| Introducir una cola de mensajes (Kafka, SQS…) | Postgres ya hace de cola con `FOR UPDATE SKIP LOCKED`, y funciona. Añadir un broker sería un cambio arquitectónico sin problema que lo justifique |
| Migrar el frontend a otro framework | No hay problema medido. Lo que hay es deuda menor (2 clientes API, 5 páginas huérfanas) → se resuelve en W4/W6 |
| Reescribir el servicio TTS | La arquitectura (contrato Go↔Python, esquema, cola, watchdog) funciona y está verificada de extremo a extremo |

---

## 6. Referencias

- `REMEDIATION_PLAN.md` — el plan de corrección detallado (oleadas, cambios, verificación, rollback).
- `KNOWN_ISSUES.md` — las 26 incidencias con evidencia.
- `DECISIONS.md` — decisiones verificadas (A) y recomendaciones (B), y las preguntas abiertas (C).
- `PROJECT_CONTEXT.md` — el mapa general.
