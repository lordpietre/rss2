# RSS2 — Traspaso de Sesión

> Documento breve para continuar el trabajo en otra sesión.
> Última sesión: **2026-10-26** — auditoría técnica completa + plan de corrección.
> **Estado: planificado. No se ha aplicado ningún cambio funcional.**

---

## 1. Qué se estaba haciendo

Auditoría técnica de RSS2 de extremo a extremo (Fases A–H del encargo):
inventario del repositorio → reconstrucción de la arquitectura → flujo de datos → análisis de
PostgreSQL/Redis y de los procesos de fondo → investigación de problemas → creación de la
documentación de contexto → plan de corrección.

**Ninguna funcionalidad se modificó.** El único cambio en el repositorio es la carpeta
**`docs/context/`** (nueva, 10 ficheros markdown).

---

## 2. Qué se investigó

| Área | Alcance de la inspección |
|---|---|
| Backend Go | 59 ficheros, ~20.500 L: las 13 rutas de `cmd/server/main.go` con sus middleware, los 15 ficheros de `handlers/`, `middleware`, `models`, `services`, `cache`, `db`, `workers`, `auth`, `entitycheck`, `textclean` |
| Workers Go | Los 10 binarios de `backend/cmd/`: qué hacen, su bucle, sus tablas |
| Ingestor | `rss-ingestor-go/main.go` completo (659 L) + `textclean/` |
| Workers Python | `translation_scheduler`, `langdetect_worker`, `ner_worker`, `embeddings_worker`, `ctranslator_worker`, `remote-worker/worker.py`, `workers/tts/` (9 módulos, 2.600 L) |
| Frontend | 21 páginas, 2 clientes API, 5 hooks, 20 componentes, tests |
| Infraestructura | `docker-compose.yml` (24 servicios), los 6 Dockerfiles, `nginx.conf`, `frontend/nginx.conf`, `Makefile`, `monitoring/` |
| Base de datos | Las 50 tablas, 212 índices, 61 FK, los 66 ficheros de `init-db/`, `migrations/` |
| TTS | El motor de extremo a extremo: Go → HTTP → Python → WAV |
| **Base de datos en producción** | ~25 consultas de sólo lectura (volúmenes, índices, planes, `pg_stat_user_tables`, `pg_stat_activity`) |
| **Logs en producción** | `docker compose logs` del ingestor, el traductor y el TTS; el log de nginx |

---

## 3. Qué se ha modificado

**Sólo se ha añadido documentación** (carpeta nueva `docs/context/`):

| Fichero | Contenido |
|---|---|
| `PROJECT_CONTEXT.md` | Mapa de entrada: propósito, estado, arquitectura, componentes, tecnologías, flujo, funcionalidades, problemas, restricciones, decisiones |
| `ARCHITECTURE.md` | Componentes, dependencias, diagramas Mermaid, TTS en detalle, observabilidad |
| `DATA_FLOW.md` | Recorrido de datos etapa por etapa + puntos de fallo + diagramas Mermaid |
| `DATABASE.md` | Esquema, índices, consultas hot, volumen, cuellos de botella, Redis |
| `COMPONENTS.md` | Inventario de módulos, servicios, workers, tareas, API |
| `KNOWN_ISSUES.md` | 26 incidencias: confirmadas / sospechas / propuestas, con evidencia |
| `DECISIONS.md` | 25 decisiones verificadas + 15 recomendaciones + 8 preguntas abiertas |
| `REMEDIATION_PLAN.md` | Plan de corrección en 9 oleadas + quick wins + guardarraíles |
| `ROADMAP.md` | Qué existe, qué falta, dependencias entre tareas |
| `SESSION_HANDOFF.md` | Este documento |

**No se ha tocado:** ningún `.go`, `.py`, `.ts`, `.tsx`, `.sql`, `.yml`, `.conf`, `Dockerfile`,
`Makefile` ni `AGENTS.md`. No se ha ejecutado ninguna migración ni ninguna escritura en la base de
datos.

---

## 4. Qué pruebas se han ejecutado

### Estado tras W0 (2026-10-26) — **todo en verde**

| Prueba | Resultado |
|---|---|
| `cd backend && go test -count=1 ./...` | ✅ **ok** en los 7 paquetes con tests: `auth`, `cache`, `config`, `entitycheck`, `handlers`, `middleware`, `textclean` |
| `cd rss-ingestor-go && go test -count=1 ./...` | ✅ **ok** (2 paquetes) |
| `make parity` | ✅ `textclean: ambas copias idénticas` |
| `make test-frontend` (nuevo) | ✅ **Test Files 2 passed (2) · Tests 18 passed (18)** |
| `make tts-test` (pytest) | ⚠️ **No ejecutado** (requiere `TEST_DATABASE_URL` y `pytest`) |
| `make check-schema` | ⚠️ **No ejecutado** |
| Consultas SQL de sólo lectura | ✅ ~25 consultas contra la BD de producción |
| `curl` a `http://localhost:8888/health`, `/api/stats`, `/api/search`, `/api/entities`, `/api/countries` | ✅ todos responden correctamente |
| `GET /health` del servicio TTS | ✅ responde; confirma el motor activo y los parámetros reales |

### Lo que había antes de W0 (para contexto)

```
--- FAIL: TestAlertasCalidad    → HTTP 401 en /api/alerts (ahora exige JWT)
--- FAIL: TestAlertasFiltros    → HTTP 401 en /api/alerts
--- FAIL: TestPopularesEspana   → "/entities devuelve 0 entidades"
--- FAIL: TestPopularesGlobal   → FP fuerte visible: [símbolos] persona "Razón(s"
```

**Causas:** (1) y (3) eran tests desincronizados con la API; (4) era un **bug real de datos**
(el blocklist quedó obsoleto tras el truncado del corpus). Todo cerrado el 2026-10-26.

---

## 5. Qué resultados se han obtenido

### 5.1 Hallazgos principales

| # | Hallazgo | Evidencia |
|---|---|---|
| 1 | **La traducción es el cuello de botella, no la ingesta.** 427.200 `pending` vs 26.524 `done` (5,8 %); se drena ~31/min frente a 121–441/min de ingesta → el backlog crece 4–14× | `[DB] translation_stats`, `noticias.created_at` |
| 2 | **El TTS tarda 18,6× el tiempo real.** 327,1 s de cómputo para 17,6 s de audio; un boletín de 138 s costó ~33 min de reloj | `[LOG] docker compose logs tts` |
| 3 | `feeds` acumuló **81.173 millones** de tuplas leídas por seq scan (4,6 M scans). No está activo ahora (delta 0 en 20 s) | `[DB] pg_stat_user_tables` |
| 4 | **`make test` está en rojo** con el stack levantado (y verde si el stack está apagado: los tests sólo hacen `skip` si la API no responde) | `go test -count=1 ./...` |
| 5 | Las estimaciones del propietario **no coinciden** con la realidad: 28.250 feeds activos ✅, pero 814.290 noticias (no 3 M) y 26.524 traducidas (no 800 K) | `[DB]` |
| 6 | El **blocklist de entidades está obsoleto** tras el reset: `"Razón(s"` es visible como *persona* | `[DB]` + test |
| 7 | `topics` (0 filas), `related_noticias` (0 filas) → tendencias y relacionadas no funcionan | `[DB]` |
| 8 | `/api/search` devuelve siempre `feed_id: 0` (el campo no se selecciona) | `[CÓDIGO]` + `curl` |
| 9 | La **parrilla del noticiario no se persiste** y `processBulletin` puede enviar guiones antiguos perdiendo noticias en silencio | `[CÓDIGO] Noticiario.tsx:441-485` |
| 10 | El rol **admin** depende sólo del claim del JWT (24 h) → un admin degradado conserva privilegios | `[CÓDIGO] middleware/auth.go:48-66` |
| 11 | El servicio TTS reporta `"redis": false` → la cola Redis no está conectada; todo se recupera de Postgres | `[DB] /health` |
| 12 | Prometheus no scrapea nada de la aplicación (no hay `/metrics`) y **no hay ningún dashboard** de Grafana | `[CÓDIGO]` |
| 13 | El ingestor **no limita el tamaño de la respuesta HTTP** (límite del contenedor: 512 MB) | `[CÓDIGO] rss-ingestor-go/main.go:407` |
| 14 | `workers/embeddings_worker.py:77` contradice la migración 60 (`traduccion_id INT NOT NULL` vs nullable) | `[CÓDIGO]` |

### 5.2 Estado del sistema al terminar la sesión

- **20 contenedores** compose en marcha (perfil por defecto) + 5 réplicas `rss2-backend-run-*` no
  gestionadas por compose. Los 4 del perfil `monitoring`/`gpu` están apagados.
- Base de datos: 3.592 MB, PostgreSQL 16.15, 814.290 noticias, 453.724 traducciones.
- Redis: 413.732 claves, 137 MB de 512 MB.
- Alertas: 85, todas `nueva`, `periodo` máximo `2026-10-26 16:00`.
- TTS: 23 voces, 6 guiones aprobados, 6 trabajos `succeeded`, 1 boletín `ready` (138,04 s).

---

## 5.3 Decisiones del propietario tomadas en esta sesión

| Decisión | Documentado en | Consecuencia inmediata |
|---|---|---|
| **"Se traduce todo"** + **"primero las más recientes"** | `DECISIONS.md` **A-26** y **A-31** | **Se mantiene `ORDER BY n.fecha DESC`** (más reciente por fecha de publicación). La propuesta de FIFO queda **descartada** por medición (solape 1/128). Brecha a cerrar: **~11×** (31/min vs 331/min). Plan: `REMEDIATION_PLAN.md` W7 |
| **"El motor de TTS es Chatterbox"** | `DECISIONS.md` **A-27** | MeloTTS y OpenVoice salen de producción (`TTS_ENGINES=chatterbox`). ⚠️ **17 voces** (14 `openvoice` + 3 `melotts`) hay que migrarlas o retirarlas (W8.3) |
| **"Voces estándar y clonada como funciona ahora"** | `DECISIONS.md` **A-28** | No hay que implementar clonación: ya funciona. B-06 (consentimiento) pasa a recomendado, no bloqueante |

**Mediciones realizadas para dimensionar estas decisiones** (todas contra la BD de producción):

| Medición | Resultado |
|---|---|
| Ingesta real | 477.659 noticias/24 h · 17.090 feeds producen · **28,0 noticias/feed/día** |
| Factor de duplicación por título | **1,05** → no hay duplicación que eliminar; el volumen es real |
| Longitud del texto | título 62 car. · total 620 de media · **p50 305** · **p95 2.970** |
| Llamada real al modelo | ≤ ~525 tokens (los fragmentos se trocean a 2.000 caracteres) |
| Presupuesto reservado | `MAX_SRC_TOKENS + MAX_NEW_TOKENS = 4096` tokens/secuencia → **~4× más de lo necesario** |
| Plan del reclamo actual (`ORDER BY n.fecha DESC`) | **93,2 ms** · 15.335 buffers hit + 2.603 read · 3.551 filas de `noticias` sondeadas |
| Plan FIFO (`ORDER BY t.created_at ASC`) | **6,6 ms** · 830 hit + 182 read → 14× más rápido, pero **semánticamente distinto** |
| Plan LIFO por llegada (`ORDER BY t.created_at DESC`) | **4,9 ms** · 945 buffers |
| **Solape entre el top-128 por `fecha DESC` y por `created_at DESC`** | **1 de 128** → los dos órdenes **no** son equivalentes |
| Backlog por antigüedad | 24 h: 126.148 · 1–3 d: 80.115 · 3–7 d: 69.253 · **>7 d: 155.071** |
| Voces por motor en la base | **chatterbox 6** (`es-base`, `mi-voz`, `voz-front`, `voz-mp3`, `voz-ogg`, `audio_2026-10-24_21-34-23`) · **openvoice 14** (10 `ov-*` + 3 `demo-speaker*` + `referencia-ejemplo`) · **melotts 3** (`melotts-speaker-42/-200/-0`) → **17 filas a migrar** |

### 5.4 Petición del propietario: archivo en otra base de datos

> *"necesito que el archivo de la base de datos sea otra base de datos que no le afecte las
> consultas, para aliviar la carga, no quiero csv comprimidos"*

**Documentado, no implementado.** Diseño en `ARCHITECTURE.md` §13, recomendación en
`DECISIONS.md` B-16, incidencia en `KNOWN_ISSUES.md` ISS-027.

**Mediciones que lo sostienen:**

| Métrica | Valor |
|---|---|
| Coste por noticia | **9,75 KB** (heap 1,8 + índices 0,85 + vector/HNSW 7,1) |
| Proyección a 90 días | 43 M noticias → **400 GB**, HNSW **90 GB**, `COUNT(*)` **~33 s** |
| `COUNT(*) FROM noticias` hoy | **635 ms** (parallel index-only scan, 83.623 buffers) |
| `postgres_fdw` | **disponible y cargable** en la imagen (`pg_available_extensions`), no instalado |
| Réplica física | `wal_level=replica` + `replica_slot` (`active = f`) + `init-replica/init-replica.sh` (⚠️ con ruta de PG 18, el proyecto usa PG 16). **No hay réplica en marcha** |

**Decisiones que faltan:** P-11 (ventana), P-12 (motor), P-13 (histórico antiguo), P-14 (quién consulta el frío).

⚠️ **Regla crítica**: el archivador sólo mueve noticias **ya traducidas**. Si la ventana de 10 días
cortara la cola de traducción, "se traduce todo" (A-26) sería falso.

---

## 6. Qué queda pendiente

Todo el trabajo de corrección. Nada ha sido aplicado. Ver `REMEDIATION_PLAN.md`.

| Prioridad | Pendiente |
|---|---|
| ✅ | **W0** — **CERRADO 2026-10-26**: `make test` y `make test-frontend` en verde |
| 🔴 | **W1** — instrumentar con métricas Prometheus (mide todo lo demás) |
| 🟠 | **W2** — bugs de datos (`feed_id`, `ensure_schema`, semilla, `idempotency_key`, default de alertas) |
| 🟠 | **W3** — rendimiento e I/O (índices, `embedding_vector` por lotes, `LimitReader`, `OF t`) |
| 🟠 | **W4** — configuración real (`${VAR:-default}`) + limpieza de código muerto |
| 🟠 | **W5** — seguridad (revalidación del rol admin, expiración del JWT, confirmación en `reset-db`) |
| 🟣 | **W6** — noticiario: persistencia de la parrilla, notas personales, consentimiento de voces, refactor a componentes (requiere migración nº 66) |
| 🔴 | **W7** — escala de traducción: **W7.1a-bis (`OF t`, 30 min)** + **W7.1b (presupuesto de tokens, medir)** + **W7.5 (reparto justo, opcional)** → W7.2 (remote workers) → W7.3 (GPU) |
| 🟣 | **W8** — TTS con Chatterbox: **W8.1 (usabilidad, 4–8 h)** → W8.2 (GPU) → W8.3 (retirar MeloTTS/OpenVoice y migrar 17 voces) |

**Preguntas aún abiertas** (`DECISIONS.md` Parte C) — ninguna bloquea el inicio de W7/W8:
P-02 (retraso aceptable) · P-05 (dónde desplegar los workers remotos) · P-06 (retención) ·
P-07 (aislar el TTS) · P-08 (modo sin traducción) · **P-09** (horizonte para cerrar el backlog) ·
**P-10** (recortar los 28 articles/feed/día).

---

## 7. Siguiente paso recomendado

> **Actualizado 2026-10-26 tras cerrar W0.**

**Lo siguiente recommended: W1 (métricas).** Sin métricas, W3, W7 y W8 no se pueden evaluar, y ya
hemos fixesado varias veces "medir antes de tocar". Los dos números que hay que tener primero son el
**backlog de traducción** y el **factor de tiempo real del TTS**.

Si prefieres seguir con correcciones de datos sin instrumentar primero, el siguiente bloque de
quick wins es **W2** (5 cambios pequeños, ~3–6 h):

| # | Cambio | Esfuerzo |
|---|---|---|
| ISS-011 | `n.feed_id` en el SELECT de `/api/search` (hoy devuelve siempre 0) | 10 min |
| ISS-009 | `ensure_schema()` del worker de embeddings vs. migración 60 | 30 min |
| ISS-021 | `DetectorFactory.seed = 0` en langdetect | 5 min |
| ISS-019 | Default de `ALERTS_SCAN_INTERVAL_MIN` 60 → 15 | 5 min |
| ISS-007 | `idempotency_key` del cliente en `POST /api/tts/jobs` | 30 min |

### Paso 1 — W0: dejar los tests en verde ✅ **HECHO el 2026-10-26**

Es el prerrequisito de todo lo demás: con los tests rojos no se puede distinguir una regresión de un
fallo preexistente. Ya está hecho:

```bash
# a) ISS-016b — el test asumía total != 0, pero la API lo devuelve 0 a propósito
#    (handlers/news.go:943). Ahora comprueba len(r.Entities) > 0.
#    Fichero: backend/internal/entitycheck/api_test.go

# b) ISS-016a — /api/alerts exige JWT: apiGet admite API_TOKEN por entorno y hace
#    skip con 401 si no lo hay (sin credenciales incrustadas en el repo).

# c) ISS-017 — re-sembrar el blocklist: HECHO (copia → dry-run → apply → refresh_entity_stats)

# d) ISS-018 — npm test sin Node en el host: HECHO con `make test-frontend`
```

### Paso 2 — W2, quick wins de datos

```bash
# ISS-011 — añadir n.feed_id al SELECT y al Scan de /api/search
#   Fichero: backend/internal/handlers/search.go
# Verificar: curl -s "http://localhost:8888/api/search?q=ucrania&lang=es&per_page=1" | grep feed_id
```

### Paso 3 — W7.1a-bis: `FOR UPDATE … OF t` en el traductor (30 min)

**El orden NO se cambia**: el propietario quiere primero lo más reciente por fecha de publicación, y
eso es exactamente lo que ya hace `ORDER BY n.fecha DESC` (`DECISIONS.md` A-26, A-31).

Lo que sí se corrige es el **bloqueo innecesario de filas de `noticias`**:

```python
# translator/workers/ctranslator_worker.py:692
#   ... FOR UPDATE SKIP LOCKED   →   ... FOR UPDATE OF t SKIP LOCKED
```
- Con `OF t` sólo se bloquean filas de `traducciones` → menos contención con `langdetect` y `topics`.
- El `ORDER BY` se mantiene **idéntico**.
- Cierra también la ventana de doble reclamo (mover el `commit` después del `UPDATE locked_at`).

### Paso 3-bis — Decisión pendiente para el propietario: ¿reparto justo?

Con el orden "más reciente primero" y una cobertura del **9,4 %**, la cola antigua (155.071
pendientes de más de 7 días) **no avanza nunca**. Hay que elegir:

- (a) **Newest-first puro** (lo actual): la actualidad siempre gana; el histórico espera.
- (b) **Reparto justo**: 1 de cada N lotes a la cola antigua. Con un reparto del **5 %**, el
  histórico (>7 días) se drena en **~70 días**, a costa de 0,5 puntos de cobertura de actualidad
  (9,4 % → 8,9 %). Con el 11 %, ~32 días a costa de 1,1 puntos.
- (c) **Subir la capacidad** hasta 331/min (W7.2/W7.3) y el reparto deja de hacer falta.

→ Es la pregunta **P-02 / P-09**.

### Paso 4 — W1: métricas

Sin métricas, W3, W7 y W8 no se pueden evaluar. Empezar por el contador del **backlog de
traducción** y el **factor de tiempo real del TTS**: son los dos números que miden el problema real.

---

## 8. Información que sigue sin verificarse

| # | Qué | Por qué | Cómo verificarlo |
|---|---|---|---|
| 1 | Los **18 tests del frontend** | Node.js no está instalado | `npm install && npm test -- --run` |
| 2 | `make tts-test` (pytest) | No se ejecutó | `make tts-test-db && make tts-test` |
| 3 | `make check-schema` (deriva de esquema) | No se ejecutó | `make check-schema` |
| 4 | La **causa** de la tormenta de seq scans en `feeds` | El contador ya no crece | `pg_stat_statements` (no está habilitado) + `log_min_duration_statement` |
| 5 | Si el trigger de `feed_stats` dispara en `DELETE` (lo usa `reset-db`) | No verificado | Leer el trigger y probarlo en una base de pruebas |
| 6 | Si el id `md5(url)` del scraper colisiona con el del ingestor | Sospecha | `SELECT count(*), count(DISTINCT url) FROM noticias WHERE contenido IS NOT NULL` |
| 7 | Por qué `related_noticias` está vacía | El worker corre y no falla | Revisar los logs de `related` y su criterio de selección |
| 8 | Por qué la cola Redis del TTS no conecta | `REDIS_URL` parece válido | Trazar el `connect()` de `workers/tts/queue.py` |
| 9 | La calidad real de las voces | Sin métrica | Conjunto de frases fijas en español, sin subjetividad |
| 13 | El **presupuesto de tokens** óptimo (`MAX_SRC_TOKENS`/`MAX_NEW_TOKENS`/`MAX_SEQ_PER_CALL`) | Calculado, **no medido** | Experimento controlado en ventana de mantenimiento (procedimiento en `REMEDIATION_PLAN.md` W7.1b) |
| 14 | El throughput real de NLLB en GPU (150–400 items/min es una estimación) | No hay GPU en esta máquina | Medir con `translator-gpu` |
| 15 | El reparto justo (W7.5) con el % que el propietario elija | Afecta a la actualidad | Decidir el % (P-02/P-09) |
| 16 | El coste real del reparto justo sobre la cobertura de actualidad | Estimado: −0,5 puntos con un reparto del 5 % | Medir tras W7.1b |
| 10 | El comportamiento del `translator` con `MAX_SEQ_PER_CALL` > 8 | Sólo hay mediciones en el código | Medición controlada de memoria |
| 11 | El estado de `docker-compose.local.yml` | Referencia la red `monitoring`, que **no existe** en el compose base | `docker compose -f docker-compose.yml -f docker-compose.local.yml config` |
| 12 | El pipeline completo de `remote-worker` con un host real | 0 traducciones con `worker_id` | Desplegar un host de prueba |

---

## 9. Cómo continuar

1. Leer `PROJECT_CONTEXT.md` y luego el documento del área a tocar.
2. **Verificar el estado real del código** antes de fiarse de cualquier afirmación (incluida esta).
3. Empezar por **W0** (tests verdes) antes de cualquier otra cosa.
4. Consultar `REMEDIATION_PLAN.md` para el cambio concreto, su verificación y su rollback.
5. Un cambio = un commit. `make test` + `make parity` en verde antes de commitear.
6. Al terminar: actualizar `KNOWN_ISSUES.md` (estado real), `DECISIONS.md` (si hay decisión nueva) y
   este `SESSION_HANDOFF.md`.
7. No reescribir la documentación entera: actualizar sólo lo necesario.
