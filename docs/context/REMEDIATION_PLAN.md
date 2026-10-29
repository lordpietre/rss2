# RSS2 — Plan de Corrección

> Plan **secuenciado** para corregir los problemas de `KNOWN_ISSUES.md`.
> Redactado el **2026-10-26** a partir de la auditoría verificada.
> **Estado: planificado. Nada de este plan ha sido aplicado todavía.**
>
> Cada corrección es una *propuesta*. Los IDs remiten a `KNOWN_ISSUES.md`; las referencias
> `B-xx` remiten a las recomendaciones de `DECISIONS.md` (Parte B).

---

## 0. Reglas de la casa (no negociables)

| Regla | Motivo |
|---|---|
| **Ningún cambio sin test que lo cubra** (o sin un test nuevo) | `make test` es la única red de seguridad; no hay CI |
| **Ningún cambio en `textclean` sin `make parity`** | `backend/internal/textclean` y `rss-ingestor-go/textclean` deben ser idénticos byte a byte |
| **Ninguna migración sobre la base de producción sin copia verificada** | `pg_dump -Fc -Z 6` + restauración probada (`pg_restore --schema-only` + una tabla) |
| **Ninguna modification de `init-db/` sin actualizar `migrations/`** y al revés | `init-db/` sólo corre en la primera inicialización; las bases existentes necesitan `migrations/` |
| **Ningún cambio de esquema TTS sin los tres sitios** | `init-db/57-add-tts.sql`, `migrations/add_tts.sql` y `workers/tts/schema.py` → `make tts-test` |
| **Ningún borrado masivo sin su 428** | `POST /api/feeds/bulk-delete` exige `confirmar: true`; nunca automatizarlo |
| **Un cambio = un commit** | facilitates bisect, rollback y revisión |
| **Nada de secretos en el repositorio ni en la documentación** | sólo registrar ubicación y tipo |

### Guardarraíles del despliegue

```bash
# 1. Copia verificada ANTES de cualquier migración
docker compose exec -T db pg_dump -U rss -d rss -Fc -Z 6 -f /tmp/backup.dump
docker compose exec -T db pg_restore --schema-only -U rss -d rss /tmp/backup.dump   # verifica
docker compose exec -T db pg_restore -U rss -d rss -l /tmp/restore.list /tmp/backup.dump  # lista

# 2. Tras tocar la imagen del backend, recrear las réplicas NO gestionadas por compose
docker ps --filter name=rss2-backend-run-   # 5 contenedores: NO los toca docker compose

# 3. nginx usa resolver dinámico: tras recrear `backend` hay que reiniciar nginx
docker compose restart nginx
```

---

## 1.principios de secuenciación

```
W0  Red de seguridad (tests verdes)          ← BLOQUEA todo lo demás
 │
 ├─► W1  Instrumentación mínima (métricas)   ← bloquea la medición de W3/W4/W7/W8
 │
 ├─► W2  Bugs de datos y triviales           ← paralelizable con W1
 │
 ├─► W3  Rendimiento e I/O                   ← necesita W1 para medir
 │
 ├─► W4  Configuración y limpieza           ← paralelizable
 │
 ├─► W5  Seguridad                           ← independiente
 │
 ├─► W6  Noticiario: persistencia + notas    ← requiere migración (W6.0)
 │      └─► B-05 notas  └─► B-06 consentimiento
 │
 ├─► W7  Escala de traducción (ISS-001)      ← DECIDIDO: "se traduce todo" (A-26)
 │        ├─ W7.1a OF t (30 min)   ─┐
 │        ├─ W7.1b presupuesto ────┤ palancas baratas, mismo host
 │        ├─ W7.2 remote workers ──┤ necesitan P-05 (dónde desplegarlos)
 │        └─ W7.3 GPU ────────────┘ necesitan hardware
 │
 └─► W8  TTS: rendimiento con Chatterbox      ← DECIDIDO: motor Chatterbox (A-27)
          ├─ W8.1 usabilidad (4-8 h)   ← no necesita nada más
          ├─ W8.2 GPU (1-2 días)        ← necesita hardware
          └─ W8.3 retirar MeloTTS/OpenVoice ← 17 voces a migrar
```

**Decisiones del propietario tomadas el 2026-10-26** (`DECISIONS.md` A-26/A-27/A-28):
*"se traduce todo"* · *"el motor de TTS es Chatterbox"* · *"voces estándar y clonada como funciona ahora"*.
**Ya no bloquean nada.** Quedan abiertas P-02 (retraso aceptable), P-05, P-06, P-07, P-08 y las
nuevas P-09 (horizonte del backlog) y P-10 (recortar la oferta).

**Reglas de orden:**
1. **W0 antes que nada.** Con los tests rojos no se puede distinguir "mi cambio rompió algo" de
   "ya estaba roto".
2. **W1 (métricas) antes que W3/W7/W8**: sin medir, la optimización es una opinión.
3. **W2/W4/W5 en paralelo**: no se tocan entre sí (el único solape es `docker-compose.yml`, que
   exige serializar los commits).
4. **W6/W7/W8 requieren decisiones del propietario** (ver `DECISIONS.md` Parte C).
5. **W6.0 (migración) es un commit propio y reversible**, antes de la funcionalidad.

---

## 2. Oleadas

### 🟢 W0 — Restablecer la red de seguridad · **PRERREQUISITO** · ✅ **CERRADO 2026-10-26**

**Objetivo.** Que `make test` y `npm test` sean verdes con el stack levantado.
**Estado: COMPLETADO.** Detalle de lo hecho al final de esta sección.

**Objetivo.** Que `make test` y `npm test` sean verdes con el stack levantado.

| # | Acción | Fichero | Riesgo |
|---|---|---|---|
| ISS-016a | Los tests de alertas deben **enviar un JWT** o hacer `t.Skip` si la API responde 401 | `backend/internal/entitycheck/alertas_api_test.go` | Bajo |
| ISS-016b | `checkPopulares` no debe tratar `total == 0` como error (la API lo devuelve **a propósito** en la ruta con `country_id`): comprobar `len(r.Entities) > 0` | `backend/internal/entitycheck/api_test.go:79-82` | Bajo |
| ISS-017 | **Re-ejecutar el seed del blocklist**: `make entities-scan` (dry-run) → revisar → `make entities-scan-apply` | `entityscan` (binario) | 🟠 **Escribe en `entity_blocklist`** — revisar el dry-run |
| ISS-018 | Instalar Node.js (nvm o apt) **o** documentar que los tests del frontend sólo corren en Docker/CI | entorno | Bajo |

**Nota sobre ISS-017.** Es la única acción de todo el plan que **escribe datos de producción**.
Justificación: el blocklist se sembró antes del truncado del corpus (2026-10-25) y hoy hay un falso
positivo fuerte visible para el usuario (`"Razón(s"` como *persona*, 26 menciones).
**Alternativa si el propietario no lo aprueba:** añadir el valor a `entity_blocklist` a mano
(`INSERT ... ON CONFLICT`) y dejar el seed completo para más adelante.

**Verificación.**
```bash
cd backend && go test -count=1 ./...        # debe pasar TODO
cd rss-ingestor-go && go test -count=1 ./...
make parity                                # exit 0
cd frontend && npm test -- --run           # 18 casos en verde
```
**Rollback.** `entity_blocklist` es una tabla: `DELETE` de las filas insertadas por el scan
(un backup previo de la tabla es suficiente).

**Esfuerzo:** 1–3 h.

#### ✅ W0 — Registro de ejecución (2026-10-26)

| Paso | Qué se hizo | Resultado |
|---|---|---|
| ISS-016a | `apiGet` admite `API_TOKEN` por entorno; ante un **401** sin token hace `t.Skip` (nunca se incrusta una credencial) | `TestAlertasCalidad` y `TestAlertasFiltros` **se saltan** en vez de fallar |
| ISS-016b | `checkPopulares` comprueba `len(r.Entities) == 0` en vez de `r.Total == 0` (la API devuelve `total = 0` **a propósito** en la ruta con `country_id`) | Los tests de Populares **se ejecutan** y destapan el problema real |
| ISS-017 | Copia de la tabla → dry-run (12 s, 3.052 FP fuertes) → `make entities-scan-apply` → `SELECT refresh_entity_stats();` | `entity_blocklist` 7.430 → **10.478**; los 3 FP (`Razón(s`, `Merengues`, `Mbappé 1-0 Yamal`) bloqueados |
| ISS-018 | `make test-frontend` (vitest en `node:20-alpine`, sin instalar Node en el host) | **18/18 en verde** |

**Verificación final:**

```
backend:        ok auth · ok cache · ok config · ok entitycheck · ok handlers · ok middleware · ok textclean
rss-ingestor-go: ok rss-ingestor-go · ok rss-ingestor-go/textclean
make parity:    textclean: ambas copias idénticas
make test-frontend: Test Files 2 passed (2) · Tests 18 passed (18)
```

⚠️ **Dos aprendizajes que hay que no perder:**
1. El paso `SELECT refresh_entity_stats();` **no es opcional**: la ruta rápida de `/api/entities`
   lee la tabla precomputada `entity_stats`, no `tags`. Sin refrescarla, el FP bloqueado seguía
   visible y el test seguía rojo.
2. La red de seguridad **sólo se activate con el stack levantado**: los tests sólo hacen `skip`
   si la API no responde. Con el stack apagado, `make test` pasaba en verde y ocultaba 4 fallos.

---

### 🔵 W1 — Instrumentación mínima (métricas)

**Objetivo.** Poder medir el antes/después de W3, W7 y W8.

| # | Acción | Fichero |
|---|---|---|
| ISS-015a | Exponer `/metrics` en el backend Go con `promhttp` (el módulo ya está en `go.mod` vía transitive; hay que añadirlo como dependencia directa) | `backend/cmd/server/main.go` |
| ISS-015b | Instrumentar los workers Python: un `GET /metrics` en texto plano (formato Prometheus) con contadores básicos (lotes, elementos, errores, duración) | `workers/*.py`, `translator/workers/ctranslator_worker.py` |
| ISS-015c | Corregir `monitoring/prometheus.yml`: añadir el target del backend y de los workers; quitar el target comentado `rss2_web` | `monitoring/prometheus.yml` |
| ISS-015d | Crear 3 dashboards mínimos: **ingesta**, **traducción** (lotes/min, backlog, items/segundo) y **TTS** (trabajos, duración, factor de tiempo real) | `monitoring/grafana/dashboards/json/` |
| ISS-015e | Un panel en `/admin/workers` con el backlog de traducción y el ritmo de drenaje | `frontend/src/pages/AdminWorkers.tsx`, `backend/internal/handlers/admin.go` |

**Métricas que hay que capturar sí o sí** (son las que faltan hoy):
- `noticias` insertadas/min (por `feed_id` y total).
- Traducciones: `pending` actual, `done/min`, `items/segundo` por lote.
- TTS: trabajos pendientes, duración real del audio vs. duración del texto (**factor de tiempo real**).
- Lag del scanner de alertas (ya existe un warning en el código: `alerts.go:292-294`).

**Verificación.** `curl localhost:9090/metrics` devuelve series; el panel de Grafana muestra el
backlog de traducción descendiendo cuando el traductor avanza.

**Esfuerzo:** 4–8 h. **Depende de:** nada (puede empezar ya). **Habilita:** W3, W7, W8.

---

### 🟢 W2 — Bugs de datos y correcciones triviales

**Objetivo.** Corregir datos incorrectos que ya se están sirviendo al usuario. Sin esquema, sin
cambios de comportamiento global, desplegable de una vez.

| # | Acción | Fichero | Nota |
|---|---|---|---|
| ISS-011 | Añadir `n.feed_id` al `SELECT` de `/api/search` **y** al `rows.Scan` | `backend/internal/handlers/search.go` | Hoy el frontend recibe `feed_id: 0` en **todas** las búsquedas |
| ISS-009 | Corregir `ensure_schema()` del worker de embeddings para que coincida con la migración 60 (`traduccion_id` nullable + `noticia_id`) | `workers/embeddings_worker.py:68-95` | Hoy inofensivo; rompe las bases nuevas |
| ISS-021 | Fijar `DetectorFactory.seed = 0` | `workers/langdetect_worker.py:15` | Determinismo |
| ISS-007 | Aceptar el `idempotency_key` del cliente, o documentar que se recalcula | `backend/internal/handlers/tts.go:1005-1019, 2151-2155` + `specs/10-tts/spec.md:99` | Decidir: **recomendado** → aceptar el del cliente si viene |
| ISS-019 | Default de `ALERTS_SCAN_INTERVAL_MIN`: 60 → 15 | `backend/internal/handlers/alerts.go:462` | Alinea código, compose y `AGENTS.md` |
| ISS-005a | Usar `config.JWTExpiration` en `GenerateToken` (hoy 24 h hard-coded en dos sitios) | `backend/internal/auth/jwt.go:29`, `backend/internal/config/config.go` | elimination de duplicado |
| ISS-013 | Diagnosticar por qué `related_noticias` está vacía (trazas en los logs del worker; revisar criterio de selección y `min_score`) | `backend/cmd/related/main.go` | Verificar el criterio de tiempo/modelo |
| ISS-012 | Comprobar si `init-db/37-topics-seed.sql` se aplicó; si no, aplicarlo y **verificar** el resultado | `init-db/37-topics-seed.sql` | ⚠️ la tabla `topics` está vacía (0 filas) |

**Verificación.**
```bash
cd backend && go test -count=1 ./...
# ISS-011
curl -s "http://localhost:8888/api/search?q=ucrania&lang=es&per_page=1" | grep -o '"feed_id":[0-9]*'
# debe ser > 0 (antes: 0)
```
**Esfuerzo:** 3–6 h. **Bloquea:** nada. **Depende de:** W0.

---

### 🟡 W3 — Rendimiento e I/O

**Objetivo.** Reducir la carga de Postgres y los tiempos de espera. Todo medible con W1.

| # | Acción | Fichero | Impacto esperado |
|---|---|---|---|
| ISS-003a | Índice parcial `feeds` para la ingesta: `CREATE INDEX CONCURRENTLY idx_feeds_activo ON feeds (id) WHERE activo = TRUE` — o `(activo, fallos)` | migración nueva | Elimina el seq scan de 33 K filas por ciclo de ingesta |
| ISS-003b | `GetFeeds`: dejar de ordenar por `p.nombre`/`c.nombre` (columnas de tablas unidas). Ordenar por `feeds` o enrich en Go | `backend/internal/handlers/feed.go:102-115` | La consulta más cara de la API |
| ISS-024 | Escribir `embedding_vector` por lotes (`execute_values` con la expresión vectorial, o `COPY` a tabla temporal + `INSERT…SELECT`) | `workers/embeddings_worker.py:206-312` | −128 idas y vueltas por lote |
| ISS-023 | No resetear el cursor a `None` cuando el lote viene vacío | `workers/embeddings_worker.py:466-472` | Evita re-escanear `noticias` cada 5 s en reposo |
| ISS-004 | Envolver `resp.Body` con `io.LimitReader` (p. ej. 16 MB) antes de `fp.Parse`; devolver error explícito si se excede | `rss-ingestor-go/main.go:407` | Cierra la vía de OOM (límite del contenedor: 512 MB) |
| ISS-026 | Reutilizar `normalizeURL` en el scraper para el id | `backend/cmd/scraper/main.go:354` | Evita duplicados ingestor↔scraper |
| ISS-022 | `FOR UPDATE … OF t` en el traductor (deja de bloquear `noticias`) y mover el `commit` **después** del `UPDATE locked_at` | `translator/workers/ctranslator_worker.py:456-463, 680-693` | Menos contención con `langdetect`/`topics`; cierra la ventana de doble reclamo |

**Verificación.**
```sql
-- antes/después del índice de feeds
SELECT seq_scan, seq_tup_read FROM pg_stat_user_tables WHERE relname='feeds';
-- y confirmar el plan
EXPLAIN (ANALYZE) SELECT id FROM feeds WHERE activo = TRUE AND (fallos IS NULL OR fallos < 10);
```
**⚠️ Nota de concurrencia.** Los índices nuevos sobre tablas de 814 K filas se crean **fuera de
horario de ingesta** o con `CREATE INDEX CONCURRENTLY` (no bloquea escrituras, pero es más lento).
`CREATE INDEX` normal toma un lock `SHARE` que **bloquea los INSERT del ingestor** durante minutos.

**Esfuerzo:** 6–12 h. **Depende de:** W1 (para medir). **Riesgo:** medio (índices).

---

### 🟡 W4 — Configuración y limpieza

**Objetivo.** Que la configuración diga la verdad y que no quede código muerto que confunda.

| # | Acción | Fichero |
|---|---|---|
| ISS-010a | Sustituir los valores fijos de compose por `${VAR:-default}`: `RSS_MAX_WORKERS`, `RSS_POKE_INTERVAL_MIN`, `EMB_BATCH`, `EMB_LIMIT`, `ALERTS_SCAN_INTERVAL_MIN`, `TTS_MAX_ATTEMPTS` (este último **no** llega hoy al backend) | `docker-compose.yml` |
| ISS-010b | **Eliminar código muerto**: `handlers.GetAdminStats`, `handlers.GetModelDownloadURL`, `services.Translate`, `services.ExtractEntities`, `config.{JWTExpiration(ya…), NEWSPerPage, OllamaURL, DockerComposeDir}`, `cache.{NewsKey, EntityKey, StatsTopKey}` + sus invalidaciones, `services/apiClient.ts` + `contexts/AppContext.tsx`, páginas `Home/LoginPage/OnboardingPage/SettingsPage/Layout`, el `Dockerfile` de la raíz, el binario `rss-ingestor-go/rss-ingestor` versionado en git | varios |
| ISS-010c | `NER_LANG`: o se usa para filtrar los `lang_to` que se procesan, o se elimina | `workers/ner_worker.py:46` |
| ISS-020 | `depends_on: db: service_healthy` también en `translator` y `translator-gpu` | `docker-compose.yml` |
| ISS-008 | Diagnosticar por qué la cola Redis del TTS no conecta (`REDIS_URL` del contenedor es válida). Si es intencionado, **eliminar el `LPUSH`** y dejar Postgres como único mecanismo | `workers/tts/queue.py`, `backend/internal/cache/redis.go:109-119` |
| ISS-025 | Redis `--maxmemory-policy volatile-lru` (sólo evicta claves con TTL) o TTL a `tts:queue` | `docker-compose.yml` (servicio `redis`) |

**Verificación.**
```bash
# ISS-010a: cambiar una variable en .env y comprobar que surte efecto
docker compose config | grep RSS_MAX_WORKERS
# ISS-010b: buscar residuos
grep -rn "GetAdminStats\|services.Translate\|apiClient" --include=*.go --include=*.ts --include=*.tsx .
```
**⚠️ Nota.** `ISS-010b` toca frontend y backend a la vez ⇒ **dividir en varios commits** y
verificar `npm test` tras cada uno (no se puede ejecutar hoy → ver W0/ISS-018).

**Esfuerzo:** 4–8 h. **Depende de:** nada. **Paralelizable** con W2.

---

### 🔵 W5 — Seguridad

**Objetivo.** Que el sistema no dependa de un claim de 24 h para decidir quién es administrador.

| # | Acción | Fichero | Nota |
|---|---|---|---|
| ISS-005b | **Revalidar el rol contra la BD** en `AdminRequired()` cuando el token sea antiguo (p. ej. > 15 min), o con un contador `token_version` en `users` | `backend/internal/middleware/auth.go:48-66` | La versión con contador es más limpia: `UPDATE users SET token_version = token_version+1` al degradar |
| ISS-005c | Reducir la expiración por defecto del JWT (p. ej. 12 h) y añadir un `POST /api/auth/refresh` | `backend/internal/auth/jwt.go` | Requiere tocar el frontend (interceptor) |
| ISS-005d | Revisar que `/api/admin/reset-db` y `/api/admin/restore` exigen confirmación adicional (hoy no la tienen) | `backend/internal/handlers/admin.go:431, 1016` | `reset-db` borra 15 tablas |
| ISS-005e | Auditar las variables de entorno que fijan valores sensibles y documentar dónde viven | `.env` | **Nunca imprimir valores** |

**Verificación.**
```bash
# Login como admin → token → degradar a usuario normal → el token viejo debe seguir siendo admin (fallo)
# y tras el fix, /api/admin/users debe devolver 403
```
**Esfuerzo:** 4–8 h. **Depende de:** nada.

---

### 🟣 W6 — Noticiario: persistencia, notas y consentimiento

**Objetivo.** Que la parrilla sobreviva a un refresco, que se puedan escribir notas que **no** se
locutan, y que haya registro de autorización de las voces clonadas.

**W6.0 — Migración (commit propio y reversible).**

| # | Acción |
|---|---|
| B-05a | `ALTER TABLE tts_scripts ADD COLUMN notes TEXT` (nullable) |
| B-06a | `ALTER TABLE tts_voices ADD COLUMN consent_status TEXT, ADD COLUMN consent_note TEXT, ADD COLUMN consent_at TIMESTAMPTZ` |
| B-06b | `ALTER TABLE tts_bulletins ADD COLUMN lineup_state JSONB` (o tabla `tts_lineups`) para persistir la parrilla |
| — | **Actualizar los tres sitios del DDL**: `init-db/` (migración 66), `migrations/` y `workers/tts/schema.py` |
| — | `make tts-test` (paridad del DDL) **debe pasar** |

**W6.1 — Backend.**
- `PUT /api/tts/scripts/:id` acepta `{content, notes}` → guardar `notes` aparte de `content`
  (las notas **no** entran en `raw_content` ni en la normalización).
- `GET /api/tts/scripts/:id` devuelve `notes`.
- Endpoint para guardar/cargar la parrilla (`GET`/`PUT /api/tts/lineup`), con el orden de `script_ids`.
- `POST /api/tts/voices` exige `consent_status` (o marca `own_voice` exento).
- `PATCH /api/tts/bulletins/:id/items` para **reordenar** un boletín ya creado.

**W6.2 — Frontend (`Noticiario.tsx`).**
- Guardar la parrilla en el backend (no sólo en `useState`) → sobrevive al refresco.
- **Derivar siempre `script_ids` de la selección vigente** (corrige el bug de `processBulletin`).
- Un `<textarea>` de notas por guion, con marca visual de "no se locuta".
- Guardar el borrador de la parrilla al reordenar.
- Botones de reordenamiento del boletín ya creado.

**W6.3 — Migrar el Noticiario a componentes.**
Hoy todo el TTS vive inline en `Noticiario.tsx` (1.517 L). Extraer
`Lineup`, `ScriptEditor`, `VoicePicker`, `BulletinStatus`, `AudioPlayer` a `components/tts/`.
**Es un refactor, no una funcionalidad**: hacerlo **después** de que W6.1/W6.2 funcionen, y con los
tests del frontend en verde (W0/ISS-018).

**Verificación.**
```bash
make tts-test                     # paridad del DDL
cd frontend && npm test -- --run  # los 14 tests de Noticiario siguen en verde
```
**Esfuerzo:** 10–20 h. **Depende de:** W0 (tests verdes), W6.0 (migración). **Riesgo:** medio.

---

### 🔴 W7 — Escala de traducción: traducir **todo**

> **Decisión del propietario (2026-10-26): "se traduce todo".** Esto fija el objetivo: la cobertura
> debe tender al 100 % del corpus. Ver `DECISIONS.md` A-26.

#### W7.0 — La aritmética del problema (medida, no estimada)

| Magnitud | Valor | Fuente |
|---|---|---|
| Noticias ingeridas | **477.659/día** (últimas 24 h) | `[DB]` |
| Feeds que producen | 17.090 (de 28.250 activos) | `[DB]` |
| Noticias por feed y día | **28,0** (el propietario estimaba 7–10) | `[DB]` |
| Factor de duplicación por título | **1,05** → **no hay duplicación que eliminar** | `[DB]` |
| Ingesta sostenida | **331/min** | derivado |
| Drenaje actual (2 traductores) | **~31/min** | `[DB] translation_stats` |
| Backlog | 427.200 `pending` | `[DB]` |
| **Brecha** | **~11×** | derivado |

**Objetivo de capacidad:**
- Para **no crecer** el backlog: ≥ 331/min.
- Para **cerrarlo en 30 días**: ≥ 380/min.
- Para **cerrarlo en 7 días**: ≥ 740/min.

#### W7.1 — Palancas baratas (en este host, sin hardware nuevo)

| # | Acción | Ganancia esperada | Riesgo |
|---|---|---|---|
| **W7.1a** | **Mantener el orden actual** `ORDER BY n.fecha DESC` (más reciente por fecha de publicación primero) y **añadir, si el propietario lo quiere, un reparto justo**: 1 de cada N lotes se toma de la cola antigua (`n.fecha ASC`) en vez de la reciente | El orden **no** es una palanca de caudal (93 ms sobre un lote de ~500.000 ms = 0,02 %). Lo que sí cambia es **qué se traduce**: con reparto justo, la cola antigua avanza sin perder la actualidad | 🟠 El reparto justo **consume** capacidad que hoy va a la actualidad → el porcentaje lo decide el propietario (P-02/P-09) |
| **W7.1a-bis** | Corregir el bloqueo de filas de `noticias` con `FOR UPDATE … OF t` **sin tocar el orden** (ISS-022) | Menos contención con `langdetect`/`topics`. El `ORDER BY` se mantiene idéntico | Bajo |
| **W7.1b** | **Ajustar el presupuesto de tokens**: `MAX_SRC_TOKENS` 2048→1024, `MAX_NEW_TOKENS` 2048→1024, y subir `MAX_SEQ_PER_CALL` 8→16 con el **mismo** límite de 3 GB | Teóricamente **~2×** por core. Ninguna llamada real al modelo pasa de ~525 tokens (los fragmentos ya se trocean a 2.000 caracteres; p95 del texto = 2.970 caracteres, p50 = 305) | 🟠 **Medir antes de dejar el valor fijo** |
| **W7.1c** | Corregir el estado `online` obsoleto de los workers remotos (el panel cuenta `status='online'` sin mirar `last_seen`) | Elimina el falso positivo; prepara W7.2 | Bajo |
| **W7.1d** | Habilitar `pg_stat_statements` y `log_min_duration_statement` | Permite medir el resto (y resolver ISS-003) | Bajo |

**⚠️ Sobre W7.1a — por qué NO se cambia el orden.** Se evaluó cambiar a
`ORDER BY t.created_at ASC` (FIFO) por ser ~14× más rápido en la selección del lote, y se medió que
los dos órdenes **no son equivalentes**: el solape entre el top-128 por `n.fecha DESC` y el top-128
por `t.created_at DESC` es de **1 de 128** (`fecha` = publicación, `created_at` = entrada en el
sistema). El propietario pidió explícitamente *"primero las más recientes"*, así que
**se mantiene `n.fecha DESC`**. Detalle en `DECISIONS.md` A-31.

**Sobre W7.1b — el experimento que hay que hacer (no es una suposición):**
```bash
# Ventana de mantenimiento, con UN traductor y el otro parado:
#   1) MAX_SRC_TOKENS=1024 MAX_NEW_TOKENS=1024 MAX_SEQ_PER_CALL=8  → medir items/min y RSS
#   2) MAX_SRC_TOKENS=1024 MAX_NEW_TOKENS=1024 MAX_SEQ_PER_CALL=16 → medir items/min y RSS
#   3) MAX_SRC_TOKENS=2048 MAX_NEW_TOKENS=2048 MAX_SEQ_PER_CALL=8  → referencia (config actual)
# RSS del contenedor: docker stats rss2_translator --no-stream
# Regla: el pico de RSS debe quedar < 2,5 GB (margen bajo el límite de 3 GB)
```
⚠️ **El cgroup cuenta las páginas swapeadas**: subir el lote sin medir puede provocar un OOM
que tumbe el traductor. Medir primero, fijar después.

#### W7.2 — Escala horizontal: `remote-worker`

Es la vía que el diseño ya soporta y la que **encaja con "traducir todo"**: cada worker remoto
suma su throughput sin tocar el host principal.

|workers remotos (tras W7.1) | Throughput total | ¿Cierra el backlog en 30 días? |
|---|---|---|
| 0 (sólo los 2 locales, ×2 por A-29) | ~62/min | ❌ (crece ~269 K/día) |
| 10 | ~212/min | ❌ (crece) |
| **20** | **~362/min** | ✅ (~380/min: se mantiene y baja despacio) |
| 23 | ~407/min | ✅ (baja ~1,1 K/min) |

**Precio**: ~20 hosts extra (o menos, si cada host tiene más cores — el worker usa
`CT2_INTRA_THREADS=2`, así que un host con 4 cores rinde ~2× uno con 2).

**Sin esto, "traducir todo" no es alcanzable en CPU**: la aritmética no deja otra salida.

#### W7.3 — Escala vertical: GPU

El perfil `gpu` ya existe (`translator-gpu`, `translator/Dockerfile.gpu`, `float16`/cuda).
Con `MAX_SEQ_PER_CALL` hasta 128 en GPU, un translator-GPU podría cubrir **gran parte del hueco
por sí solo**.

**A medir antes de comprometer hardware:** el throughput real de NLLB-600M en la GPU disponible
(items/min con batch 32 y float16). Es una estimación razonable que **150–400 items/min por GPU**,
pero **no está medida** en este proyecto.

#### W7.4 — Palanca de coste: reducir la oferta (opcional, decisión del propietario)

28,0 noticias/feed/día es 2,8–4× lo que el propietario estimaba. Si el contenido real de muchas
fuentes no aporta, reducir el número de feeds **reduce la demanda de traducción en la misma
proporción** — es la palanca más barata y la que menos código toca.

Herramienta existente: el borrado masivo con su precondición 428 (`POST /api/feeds/bulk-delete`).
**Nunca automatizarlo** y revisar siempre el dry-run de impacto.

#### W7.5 — Lo que NO se debe hacer

| Prohibido | Motivo |
|---|---|
| Subir `MAX_SEQ_PER_CALL` sin medir el RSS | 32 → ~3,1 GiB ⇒ **OOM** con el límite de 3 GB. Las mediciones están en `docker-compose.yml:97-99` |
| Ampliar el swap del host esperando que lo arregle | El cgroup cuenta las páginas swapeadas: el pico del **contenedor** es el límite |
| Añadir más contenedores `translator` en esta máquina | 4 cores, load ≈15. La CPU ya es el recurso escaso |
| Cambiar `feeds.traducido` | Es una lista curada a mano |

#### W7.5 — El reparto justo (opcional, decisión del propietario)

Con el orden "más reciente primero" (A-26) y capacidad insuficiente, la cola antigua no avanza.
Medido hoy:

| Antigüedad de publicación | Pendientes |
|---|---|
| Últimas 24 h | 126.148 |
| 1–3 días | 80.115 |
| 3–7 días | 69.253 |
| **Más de 7 días** | **155.071** |

**Opciones:**
- (a) **Newest-first puro** (actual): lo más reciente siempre traducido; la cola antigua espera
  indefinidamente. *"Se traduce todo"* no se cumple para el histórico.
- (b) **Reparto justo**: 1 de cada N lotes va a la cola antigua. Con un reparto del **5 %** de la
  capacidad actual (1,6/min), las 155.071 antiguas se drenarían en **~70 días**, y el 95 %
  restante seguiría yendo a la actualidad. El porcentaje es un parámetro a ajustar:

  | Reparto a la cola antigua | Drenaje de las 155.071 | Capacidad para actualidad | Cobertura de actualidad |
  |---|---|---|---|
  | 0 % (actual) | **nunca** | 31/min | 9,4 % |
  | 4 % | ~87 días | 29,8/min | 9,0 % |
  | 5 % | ~70 días | 29,4/min | 8,9 % |
  | 11 % | ~32 días | 27,6/min | 8,3 % |
- (c) **Subir la capacidad hasta 331/min** (W7.2/W7.3) y entonces el reparto justo es innecesario:
  con capacidad suficiente, "más reciente primero" y "traducir todo" son compatibles por sí solos.

**Impacto de (b) sobre la actualidad:** es un intercambio explícito entre actualidad e histórico.
Con un reparto del 5 %, la cobertura de actualidad pasa de 9,4 % a 8,9 % — **medio punto a cambio de
70 días de drenaje del histórico**.

**Recomendación:** (b) con un reparto del **5 %** **mientras** se despliega W7.2/W7.3, y (c)
cuando la capacidad cubra la ingesta → desactivar el reparto. Es reversible con un parámetro.

#### W7.6 — Verificación

```sql
-- El backlog debe bajar de forma sostenida (no basta con que no crezca)
SELECT status, count(*) FROM traducciones GROUP BY 1;
SELECT date_trunc('hour', created_at) h, sum(items_translated) FROM translation_stats
 WHERE created_at > NOW() - interval '24 hours' GROUP BY 1 ORDER BY 1;
```
Con las métricas de W1: series de `pending` (debe bajar) y de items/min (debe superar 331/min).

#### Esfuerzo

| Paso | Esfuerzo |
|---|---|
| W7.1a-bis (`OF t`, sin cambiar el orden) | 30 min + test |
| W7.5 (reparto justo, opcional) | 2 h + parametrización |
| W7.1b (presupuesto de tokens) | 1 h de código + 2–4 h de medición |
| W7.1c/d | 1 h |
| W7.2 (20 workers remotos) | 1 día de despliegue + 1 día de operación por tanda |
| W7.3 (GPU) | 1–2 días (incluye la medición) |

---

### 🟣 W8 — TTS: rendimiento con Chatterbox

> **Decisiones del propietario (2026-10-26): motor = Chatterbox; voces estándar y clonadas con el
> modelo actual.** Ver `DECISIONS.md` A-27 y A-28.
> **Consecuencia:** el trabajo se centra en **rendimiento**, no en añadir clonación ni motores.

#### W8.0 — Qué cambia y qué no

| Antes (suposición) | Ahora (decidido) |
|---|---|
| "Hay que evaluar Kokoro/Piper/XTTS" | **No**: Chatterbox es el motor. La evaluación pasa a ser *comparar* (opcional), no *migrar* |
| "Hay que implementar clonación" | **No**: ya funciona (voz estándar = `conds` guardados; clonada = audio de referencia) |
| "Hay que registrar consentimiento antes de activar la clonación" | **No bloqueante** (B-06 pasa a recomendado) |
| "Hay que decidir el motor" | **Decidido**: Chatterbox |
| "El TTS tarda 18,6× el tiempo real" | **Sigue siendo el problema real** → GPU |

#### W8.1 — Hacerlo usable (sin GPU, ~4–8 h)

| # | Acción | Efecto |
|---|---|---|
| W8.1a | **Truncar el guion a un máximo de frases** en `prepareTtsScripts` (p. ej. 3) y a un máximo de caracteres | Reduce el coste **proporcionalmente**. Un boletín de 6 noticias pasa de ~33 min a ~10 min |
| W8.1b | Recalibrar `CHARS_PER_SECOND` **por motor y por voz** (hoy 16,6 es una media global) | Estimación honesta |
| W8.1c | Mostrar la estimación sobre el **guion**, no sobre el cuerpo de la noticia (la UI hoy usa `selected`) | La estimación coincide con lo que se va a locutar |
| W8.1d | Mostrar el **factor de tiempo real** por voz en la UI | El coste es visible antes de generar |
| W8.1e | Recortar `tts_scripts.content` a `TTS_MAX_SCRIPT_CHARS` (8.000) en la normalización | Evita locutar textos que nadie quiere oír |

#### W8.2 — Rendimiento (GPU)

| # | Acción | Nota |
|---|---|---|
| W8.2a | `Dockerfile.tts` con torch **CUDA** + perfil de compose | El contenedor ya tiene 8 GB de límite |
| W8.2b | **Evaluar la paralelización**: Chatterbox es stateful (el clonado muta `model.conds`) y el servicio usa un lock ⇒ paralelizar exige un modelo por proceso o un modelo por voz | ⚠️ Es el punto más delicado de todo el plan |
| W8.2c | Precargar el motor de la voz por defecto **de cada voz activa**, no sólo el default | Hoy la primera locución tras un reinicio espera 91 s de carga |

#### W8.3 — Salir de producción de MeloTTS y OpenVoice

Con A-27, los otros dos motores salen de producción. **No hace falta desinstalarlos** (carga
diferida, 0 RAM en reposo); basta con `TTS_ENGINES=chatterbox`.

⚠️ **Acción obligatoria asociada:** `tts_voices.engine` se persiste **por voz**, y
`resolveTTSEngine` prioriza la voz sobre el default global. Hay **14 voces con
`engine='openvoice'`** (10 `ov-*`, 3 `demo-speaker*`, 1 `referencia-ejemplo`) y **3 con
`engine='melotts'`** (`melotts-speaker-42/-200/-0`) en la base de producción.
Si se desactiva OpenVoice sin más, esas voces fallarán con `unknown_engine`.

Opciones (elegir una):
- (a) Migrar las voces `ov-*` / `melotts-*` a `chatterbox`, conservando su audio de referencia
  (las `ov-*` con embedding `.pth` **no** son compatibles con Chatterbox → hay que extraer el audio
  de referencia o descartarlas).
- (b) Marcar esas voces como no usables y ocultarlas en la UI.
- (c) Dejarlas: no se usan mientras nadie las seleccione (pero el error en `resolve_engine`
  sería `unknown_engine`).

⚠️ **Nota sobre OpenVoice:** su `wavmark` está *stubbeado* (`workers/tts/engine.py:323-352`) ⇒ la
salida **no lleva marca de agua antiabuso**. Al sacarlo de producción desaparece ese riesgo.

#### W8.4 — Verificación

```bash
# factor de tiempo real = duration_seconds_real / duration_texto
# objetivo con GPU: < 0,5×; sin GPU, tras W8.1a: < 5×
# con las métricas de W1: serie de "TTS job duration" por voz
```

#### Esfuerzo

| Paso | Esfuerzo |
|---|---|
| W8.1 (usabilidad) | 4–8 h — **la mejor relación valor/esfuerzo de todo el plan** |
| W8.2 (GPU) | 1–2 días + hardware |
| W8.3 (retirada de motores) | 2–4 h + migración de 17 voces |

---

## 3. Quick wins · **ordenados por relación valor/esfuerzo**

> Si sólo se puede hacer una cosa esta semana, que sea esta lista.

| # | Acción | Esfuerzo | Valor |
|---|---|---|---|
| 1 | ISS-011 — `feed_id` en `/api/search` | 10 min | Datos correctos en el 100 % de las búsquedas |
| 2 | ISS-016a/b — tests de alertas y de Populares | 30 min | **Vuelve a existir la red de seguridad** |
| 3 | ISS-017 — re-sembrar el blocklist | 30 min | Quita un falso positivo visible |
| 4 | ISS-019 — default de `ALERTS_SCAN_INTERVAL_MIN` | 5 min | Código y docs coinciden |
| 5 | ISS-005a — `JWTExpiration` desde config | 15 min | Fin de un default duplicado |
| 6 | ISS-021 — semilla de `langdetect` | 5 min | Detección reproducible |
| 7 | ISS-020 — `depends_on: db` en `translator` | 10 min | Menos ruido en los reinicios |
| 8 | ISS-022 — `OF t` en el `FOR UPDATE` | 30 min | Menos contención en la BD |
| 9 | ISS-009 — `ensure_schema` de embeddings | 30 min | Las bases nuevas no rompen |
| 10 | ISS-012 — sembrar `topics` | 30 min | Las tendencias empiezan a funcionar |
| 11 | ISS-003a — índice parcial de `feeds` | 1 h | −33 K filas leídas por ciclo |
| 12 | ISS-024 — `embedding_vector` por lotes | 2 h | −128 idas y vueltas por lote |
| 13 | ISS-004 — `LimitReader` en el ingestor | 30 min | Cierra una vía de OOM |
| 14 | **W7.1a-bis — `FOR UPDATE … OF t`** en el traductor (sin tocar el orden) | 30 min | Menos contención con `langdetect`/`topics` y se cierra la ventana de doble reclamo |
| 15 | W8.1a — truncar el guion a 3 frases | 2 h | El noticiario deja de tardar 30 min |
| 16 | ISS-010a — compose con `${VAR:-default}` | 1 h | `.env` deja de mentir |
| 17 | W7.1c — corregir el estado `online` obsoleto de los workers remotos | 30 min | Prepara el escalado horizontal |

**Suma:** ~1 día de trabajo → cubre 15 incidencias, incluida la que bloquea la verificación.

---

## 4. Lo que **no** se debe tocar en esta fase

| Elemento | Motivo |
|---|---|
| `backend/internal/textclean/` y `rss-ingestor-go/textclean/` | Deben ser idénticos. Si hay que tocarlos: cambiar los dos y `make parity` en el mismo commit |
| `init-db/00-complete-schema.sql` | Se aplica en la primera inicialización; cambiarlo no afecta a la base viva (y genera deriva) |
| `data/pgdata` | **Nunca** se borra en producción |
| La clave del JWT (`SECRET_KEY`) | Cambiarla invalida todas las sesiones |
| El modelo `models/nllb-ct2` | Convertirlo a `float16` invalidaría el perfil CPU |
| `feeds.traducido` | Es una lista curada a mano; no regenerar |
| `entity_blocklist` | Sólo mediante `entityscan` (o con aprobación explícita) |
| `MAX_SEQ_PER_CALL` por encima de 8 | OOM con el límite de 3 GB |

---

## 5. Riesgos transversales

| Riesgo | Mitigación |
|---|---|
| Un cambio de esquema se aplica sólo a medias | La regla de los tres sitios (init-db + migrations + schema.py) y `make tts-test` |
| Regresión silenciosa en el ingestor | `textclean` tiene 28 tests + fuzz; ejecutar `make parity` y `go test` en cada commit |
| El consumidor del Noticiario (`translated_only`) cambia de comportamiento al tocar `search.go` | Verificar `/api/search` con `translated_only=true` **y** `LEFT JOIN` (ISS-003/ISS-011 tocan ese mismo fichero) |
| Un `CREATE INDEX` bloquea la ingesta | `CREATE INDEX CONCURRENTLY` o fuera de la ventana de ingesta |
| Subir la carga de la CPU del host | El host está sobrecargado (load ≈15 con 4 cores). Antes de añadir contenedores, medir |
| Un despliegue recrea el backend y nginx conserva la IP vieja | `docker compose restart nginx` tras cualquier recreación de `backend` |
| Se pierde el trabajo de los `rss2-backend-run-*` | Recrearlos a mano tras cambiar la imagen del backend |

---

## 6. Definición de "hecho" por oleada

- [ ] `make test` en verde (Go, ambos módulos) **con el stack levantado**
- [ ] `make parity` → exit 0
- [ ] `npm test -- --run` en verde (18 casos)
- [ ] `make tts-test` en verde si se tocó el DDL de TTS
- [ ] `make check-schema` sin derivas si se tocaron migraciones
- [ ] Métricas de W1 disponibles para medir el antes/después
- [ ] `KNOWN_ISSUES.md` actualizado (el issue, cerrado o con el estado real)
- [ ] `SESSION_HANDOFF.md` actualizado
- [ ] Ningún secreto en el repo ni en la documentación

---

## 7. Checklist de despliegue por oleada

```bash
# 1. Copia verificada
docker compose exec -T db pg_dump -U rss -d rss -Fc -Z 6 -f /tmp/pre-wave.dump
docker compose exec -T db pg_restore --schema-only -U rss -d rss /tmp/pre-wave.dump

# 2. Construir (de uno en uno: misma etiqueta de imagen para varios servicios)
docker compose build backend
docker compose build ingestion-scheduler && docker compose build embeddings

# 3. Recrear sólo lo necesario
docker compose up -d --no-deps --force-recreate backend
docker compose restart nginx                      # resolver dinámico

# 4. Recrear las réplicas NO gestionadas por compose
docker ps --filter name=rss2-backend-run-        # recrear a mano si cambió la imagen

# 5. Verificar
curl -s localhost:8888/health
cd backend && go test -count=1 ./...
```

---

## 8. Referencias

- `KNOWN_ISSUES.md` — el catálogo completo (26 incidencias) con evidencia.
- `DECISIONS.md` — decisiones verificadas (Parte A) y recomendaciones (Parte B).
- `DATABASE.md` — índices, consultas hot y volúmenes (para dimensionar W3).
- `ARCHITECTURE.md` §9.6 — puntos de extensión del TTS (para W8).
- `AGENTS.md` — quirks operativos que **no** se deben romper.
- `SESSION_HANDOFF.md` — estado de la última sesión.
