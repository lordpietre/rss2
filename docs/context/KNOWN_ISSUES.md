# RSS2 — Problemas Conocidos

> Registro de problemas. **Separación estricta** entre:
> - ✅ **CONFIRMADO** — reproducido o medido en esta auditoría (2026-10-26).
> - 🟡 **SOSPECHA** — inferido del código, sin medición directa.
> - 💡 **SOLUCIÓN PROPUESTA** — idea, **no aplicada**. Requiere validación antes de implementarse.
>
> Fuentes: `[DB]` = base de datos en producción; `[LOG]` = logs de contenedor; `[CÓDIGO]` = lectura de ficheros.

---

## 1. Críticos

### ISS-001 — La traducción no alcanza el ritmo de ingesta · ✅ CONFIRMADO

**Síntoma.** 427.200 traducciones en estado `pending` frente a 26.524 `done` (5,8 %).
Sólo el **3,3 %** del corpus está en español.

**Medición** `[DB]`:

| Dato | Valor |
|---|---|
| `traducciones` total | 453.724 |
| `pending` | 427.200 (94,2 %) |
| `done` | 26.524 (5,8 %) |
| Noticias totales | 814.290 |
| Noticias traducidas | 26.524 (**3,3 %**) |

| Productor | Ritmo medido | Fuente |
|---|---|---|
| Ingesta (2026-10-25) | 635.681/día = **441/min** | `[DB] noticias.created_at` |
| Ingesta (2026-10-26, parcial) | 174.727/día = **121/min** | `[DB]` |
| Traducción | 128 lotes de 455–1.214 s → **15–17/min por traductor** | `[DB] translation_stats` |
| Traducción (2 traductores) | **~31/min** | derivado |
| API `/api/stats` | `total_translated: 26.885` | `[DB] GET /api/stats` |

**Cálculo:** la cola crece entre **4× y 14×** más rápido de lo que se drena.
Para igualar el ritmo de la ingesta con 2 traductores harían falta **~26 traductores**;
para igualar el ritmo bajo (121/min), ~7.

**Por qué es el cuello de botella real** (contradicción con la hipótesis inicial del encargo):
la ingesta no es el problema — 33.528 feeds con 20 workers HTTP es holgado. Lo que no escala es
la traducción: CTranslate2 int8 en CPU con 4 cores saturados (load ≈15) y un límite de cgroup de 3 GB
que obliga a `MAX_SEQ_PER_CALL=8`.

**Impacto.** El noticiario (que sólo ofrece noticias traducidas, `translated_only=true`) tiene un
conjunto cada vez más reducido del que elegir; la búsqueda semántica cubre menos; las alertas
dependen de `tags_noticia`, que a su vez depende de traducciones `done`.

**Causas raíz identificadas** `[CÓDIGO]`:
1. `MAX_SEQ_PER_CALL=8` → el pico de memoria del KV cache (~24 KB/token/secuencia) limita el lote.
2. `CT2_INTRA_THREADS=2` y `CT2_INTER_THREADS=1` → con 4 cores y 2 contenedores, la CPU es el
   recurso escaso.
3. `ORDER BY n.fecha DESC` prioriza lo reciente → la cola **nunca** se vacía, sólo se desplaza.
4. Sin worker remoto operativo (`remote-worker/` desplegado en 0 hosts).

**Palancas medidas (no estimadas)** — ver `REMEDIATION_PLAN.md` W7.1:
- ❌ **El orden del reclamo NO es una palanca de caudal.** Se evaluó cambiar a
  `ORDER BY t.created_at ASC` (14× más rápido en la selección del lote) y se descartó: el
  propietario quiere primero lo más reciente, y el solape entre el top-128 por `n.fecha DESC` y el
  top-128 por `t.created_at DESC` es de **1 de 128** (`DECISIONS.md` A-26, A-31). El coste actual
  (93 ms) es **0,02 %** del tiempo del lote.
- ⚠️ **El orden sí tiene un coste real, distinto**: el `JOIN` sin `OF t` bloquea filas de
  `noticias` además de `traducciones` → contención con `langdetect`/`topics` (ISS-022).
  Se corrige **sin tocar el orden**.
- **Reparto justo (opcional)**: 1 de cada N lotes a la cola antigua para que "se traduce todo"
  sea compatible con "primero lo más reciente" (W7.5). Con un reparto del **5 %** de la capacidad
  actual (1,6/min), las 155.071 pendientes de más de 7 días se drenarían en **~70 días**; la
  cobertura de actualidad bajaría de 9,4 % a 8,9 %.
- **Presupuesto de tokens**: el traductor reserva `MAX_SRC_TOKENS + MAX_NEW_TOKENS = 4096`
  tokens por secuencia, pero **ninguna llamada real al modelo pasa de ~525 tokens** (los
  fragmentos se trocean a 2.000 caracteres; p50 del texto = 305 caracteres, p95 = 2.970).
  Bajar a 1.024+1.024 reduciría la reserva a la mitad → `MAX_SEQ_PER_CALL` 8→16 con el mismo
  límite de 3 GB. **Teóricamente ~2×; falta medirlo** (W7.1b).
- **Duplicación**: descartada. El factor de duplicación por título es **1,05** (477.659 noticias
  de 24 h → 455.881 títulos únicos). El volumen es real.

**💡 Soluciones propuestas** (ninguna aplicada):
- Desplegar los `remote-worker` en hosts adicionales (el camino ya está implementado y probado:
  el protocolo WebSocket funciona; sólo falta operación).
- Usar el perfil `gpu` (`translator-gpu`, `float16`/cuda) si aparece hardware NVIDIA.
- Recortar el trabajo: traducir **título + resumen** y dejar `contenido` para una pasada posterior
  bajo demanda (el `min_chars` del noticiario puede servirse con el resumen).
- **Priorizar por valor, no por fecha**: los feeds marcados `traducido` ya están resueltos; el resto
  se puede priorizar por categoría/país.
- Reducir el corpus: 28.250 feeds generan 635 K noticias/día — se podría desactivar en bloque los
  feeds con menor valor (el bulk-delete con confirmación ya existe).

---

### ISS-002 — El TTS tarda ~19× el tiempo real · ✅ CONFIRMADO

**Síntoma.** Una sola noticia de 17,6 s de audio tardó **327,1 s** en sintetizarse.

**Evidencia** `[LOG] docker compose logs tts`:
```
Procesando tarea TTS 6 (script=6, motor=chatterbox)
... Sampling: 443/1000 ...
Tarea TTS 6 completada en 327.1s (audio 6, 17.6s de audio)
Noticiario 1 montado: 6 noticias, 138.0s de audio
```

**Factor de tiempo real ≈ 18,6×.** El boletín de 138 s de audio costó **~33 minutos** de reloj.

**Contexto del motor** `[DB] GET /health del servicio tts`:
```json
{"engines":{"chatterbox":{"available":true,"loaded":true,"load_seconds":91.28,
 "sample_rate":24000,"t3_model":"v3","languages":23}},
 "melotts":{"available":false,"loaded":false},
 "openvoice":{"available":false,"loaded":false},
 "queue":{"pending":0,"redis":false},
 "stats":{"jobs_done":6,"jobs_failed":0,"last_job_seconds":327.13}}
```

**Causas** `[CÓDIGO]`:
- Chatterbox corre en **CPU** (`TTS_DEVICE=cpu`), con `OMP_NUM_THREADS=2` y `MKL_NUM_THREADS=2`
  (`Dockerfile.tts:13-19`). El contenedor tiene 3 CPU de límite.
- `sampling` de 1.000 pasos con ~2,1 it/s → ~8 minutos para un fragmento corto.
- El contenedor TTS tiene `start_period: 600s` en su healthcheck porque la carga del modelo tarda 91 s.

**Impacto.** El ciclo editorial completo (preparar → revisar → aprobar → audio) es de **media hora
como mínimo**. Es la causa directa de la insatisfacción con el subsistema: el propietario percibe
que el noticiario "no funciona bien", cuando en realidad funciona pero es lentísimo.

**Decisión tomada (2026-10-26):** el motor de producción **es Chatterbox** (`DECISIONS.md` A-27).
⇒ El problema **no** es la elección de motor sino el **rendimiento en CPU**.

**💡 Soluciones propuestas**:
- **Truncar el guion** (W8.1a): el coste es proporcional al texto; 3 frases por item reduce el
  tiempo del boletín de ~33 min a ~10 min. Es la mejor relación valor/esfuerzo de todo el plan.
- **GPU**: Chatterbox en CUDA es ~10–50× más rápido. El `Dockerfile.tts` ya instala torch CPU-only;
  haría falta una variante con CUDA y un perfil de compose.
- **Precalentado y reutilización**: ya se precarga el motor por defecto; los otros dos se cargan en
  el primer uso (cada uno ~2 GB de RAM).
- **Concurrencia**: el servicio TTS procesa **un trabajo a la vez** (bucle con lock, porque Chatterbox
  es stateful: el clonado muta `model.conds`). Con una GPU se podrían paralelizar con cuidado.
- **Recortar antes de sintetizar**: `TTS_MAX_SCRIPT_CHARS=8000`; los guiones de las noticias reales
  ronda los 1.500–2.500 caracteres. Limitar la duración por item (p. ej. 3 frases) reduciría el coste
  proporcionalmente.
- **Evaluar motores más ligeros** para un noticiario largo (Kokoro / Piper no están instalados;
  ver ISS-014).

---

## 2. Altos

### ISS-003 — Tormenta de escaneos secuenciales sobre `feeds` · ✅ CONFIRMADO (histórico)

**Síntoma.** `[DB] pg_stat_user_tables`:
```
feeds       seq_scan = 4.615.698   seq_tup_read = 81.173.258.205   (33.528 filas)
paises      seq_scan = 1.040.318   seq_tup_read =    121.965.973   (195 filas)
categorias  seq_scan =   923.441   seq_tup_read =      8.836.692   (15 filas)
```

**81.173 millones de tuplas leídas** en la tabla `feeds` desde el último arranque de Postgres
(≈2,4 M de lecturas completas de la tabla).

**Estado actual:** el contador **no crece** (delta 0 en una ventana de 20 s) y el log de nginx no
registra peticiones a `/api/feeds` en los últimos 30 min → **la tormenta no está activa**.
La causa exacta **no se ha identificado**.

**Candidatos** (🟡 sospechas, no confirmadas):
- `GetFeeds` (`handlers/feed.go:102-115`) hace `COUNT(*)` + `LEFT JOIN categorias/paises/feed_stats`
  + `ORDER BY p.nombre, f.activo, f.fallos, c.nombre, f.nombre` → obliga a leer y ordenar toda la
  tabla. La caché dura 1 h pero la clave incluye la página → 212 claves distintas.
- `getActiveFeeds()` del ingestor (`rss-ingestor-go/main.go:197`) → 1 seq scan por ciclo (60 min).
- El reset del corpus (2026-10-25) con sus operaciones masivas.

**💡** Crear un índice parcial `feeds (activo) WHERE activo` o `(activo, fallos)`; cachear el
`COUNT(*)`; sustituir el `ORDER BY` multi-tabla por un orden por `feeds` con un `LEFT JOIN`
pospuesto, o precomputar el nombre de categoría/país en la propia fila.

---

### ISS-004 — El ingestor no limita el tamaño de la respuesta HTTP · 🟡 SOSPECHA (riesgo real)

**Síntoma.** `rss-ingestor-go/main.go:407` → `fp.Parse(resp.Body)` **sin** `io.LimitReader`.
`grep LimitReader|MaxBytesReader` en `rss-ingestor-go/` → **0 resultados** (sólo existen en `backend/`).

**Contexto.** El contenedor `ingestor` tiene límite de **512 MB** y `shared_buffers` aparte.
Un feed hostil o gigante (un podcast RSS con enclosures de vídeo, un feed de agregador con el
historial completo) podría agotar la memoria del contenedor.

**Contraste:** el `scraper` Go **sí** limita el cuerpo a 512 KiB (`cmd/scraper/main.go:30`).

**💡** Envolver `resp.Body` con `io.LimitReader(resp.Body, N<<20)` antes de `fp.Parse`, y rechazar
con un error explícito los feeds que lo excedan (para que `fallos` aumente y el feed se desactive).

---

### ISS-005 — El rol admin se decide sólo por el claim del JWT · ✅ CONFIRMADO

**Síntoma.** `middleware.AdminRequired()` (`backend/internal/middleware/auth.go:48-66`) lee
`claims.IsAdmin` y **no re-consulta la base de datos**. `GenerateToken` graba el rol en el token con
expiración **hard-coded de 24 h** (`backend/internal/auth/jwt.go:29`).

**Impacto.** Un administrador degradado (`POST /api/admin/users/:id/demote`) conserva privilegios de
administrador — incluyendo `POST /api/admin/reset-db`, `POST /api/admin/restore` y el borrado
masivo de feeds — **hasta 24 h**.

**Problema secundario.** `config.JWTExpiration` existe (`24h`) pero **nunca se usa**: la expiración
está duplicada y hard-coded en `jwt.go`.

**💡** (a) Usar `config.JWTExpiration`; (b) re-validar el rol contra `users` cuando el token tenga
más de X minutos, o mantener un contador de versión de usuario; (c) reducir la expiración a 1–2 h
con refresh token.

---

### ISS-006 — La parrilla del noticiario no se persiste y el flujo puede perder noticias · ✅ CONFIRMADO

**Síntoma A — pérdida de la selección.** `frontend/src/pages/Noticiario.tsx:127`:
`const [selected, setSelected] = useState<News[]>([])`.
`grep localStorage` en `Noticiario.tsx` → **ningún `setItem`**. Recargar la página **pierde la
selección y su orden**. No hay endpoint de borrador.

**Síntoma B — pérdida de noticias en silencio.** `processBulletin` (`Noticiario.tsx:441-485`):
```ts
let prepared = scripts
if (prepared.length === 0) { /* preparar ahora */ }
```
`scripts` sólo se rellena en `openReview` (línea 351). Si el usuario abre la revisión, la cierra y
luego **añade o quita** noticias, se envían los guiones **antiguos** → las noticias nuevas se
pierden sin aviso. Y **reordenar** la parrilla tras abrir la revisión tampoco cambia el resultado,
porque el orden sale de `scripts`, no de `selected`.

**Síntoma C — el reordenamiento no se propaga.** No hay `PUT /api/tts/bulletins/:id` ni
`PATCH .../items/:id/position`; las posiciones son inmutables desde el cliente.

**💡** Persistir la parrilla (borrador en `tts_scripts` + un estado `lineup` o `tts_bulletins` en
estado `draft`); derivar siempre `script_ids` de la selección vigente; añadir reordenamiento del
boletín ya creado.

---

### ISS-007 — El `idempotency_key` enviado por el cliente se ignora · ✅ CONFIRMADO

**Síntoma.** `handlers/tts.go:1005-1012` calcula `key` y hace `_ = key` (línea 1019, descartado);
`enqueueTTSJob` (línea 1054) lo **recalcula** internamente con `sha256("script:%d|voice:%s|engine:%s|speed:%.2f")`.

**Impacto.** Un cliente que envíe su propia clave de idempotencia para reintentar con seguridad
**no lo consigue**: la clave real siempre incluye el `speed`. `specs/10-tts/spec.md:99` describe la
clave sin el componente `speed` → **discrepancia entre spec y código**.

**💡** Aceptar la clave del cliente si viene, o documentar el comportamiento real en la spec.

---

## 3. Medios

### ISS-008 — La cola Redis del TTS no está conectada · ✅ CONFIRMADO

**Síntoma.** `GET /health` del servicio TTS → `"queue": {"pending": 0, "redis": false}`.

**Consecuencia.** El `LPUSH tts:queue` del backend va a una lista que nadie lee; el worker recupera
cada trabajo por `claim_next_job` (`FOR UPDATE SKIP LOCKED` sobre Postgres). **Funciona**, pero:
- se pierde la señalización inmediata (el worker sondea);
- se acumulan claves `tts:queue` huérfanas en Redis;
- con `allkeys-lru` y `maxmemory 512mb`, Redis **puede evictar** claves de la cola.

**💡** Diagnosticar por qué falla la conexión (la variable `REDIS_URL` del contenedor `tts` existe y
es válida) o aceptar Postgres como único mecanismo y **eliminar el push a Redis**.

---

### ISS-009 — `embeddings.ensure_schema()` contradice la migración 60 · ✅ CONFIRMADO

**Síntoma.** `workers/embeddings_worker.py:77` declara `traduccion_id INT NOT NULL`.
`init-db/60-add-embeddings-noticia-id.sql:11-12` lo hace **nullable** y añade `noticia_id`.

**Impacto actual:** nulo — la tabla ya existe, el `CREATE TABLE IF NOT EXISTS` no hace nada.
**Impacto latente:** en una base nueva donde el worker gane la carrera, **la ruta de los originales
en español fallaría** (la columna NOT NULL impediría `traduccion_id = NULL`).

**💡** Corregir el DDL del worker para que coincida con la migración 60, o eliminar el
`CREATE TABLE` del worker y dejar el esquema como única fuente (init-db + migraciones).

---

### ISS-010 — Configuración muerta en `.env` y en el código · ✅ CONFIRMADO

| Variable / constante | Realidad |
|---|---|
| `RSS_MAX_WORKERS=3` (`.env`) | `docker-compose.yml:232` fija **20** → el valor del `.env` no se usa |
| `RSS_POKE_INTERVAL_MIN=15` (`.env`) | compose fija **60** |
| `EMB_DEVICE=cuda` (`.env`) | el worker lee `DEVICE` (que compose fija a `cpu`) |
| `NER_LANG` | se lee en `ner_worker.py:46` pero **nunca se usa** |
| `JWTExpiration` | se carga pero `GenerateToken` **no lo usa** |
| `NEWS_PER_PAGE` | **nunca se lee** (los handlers usan 30 hard-coded) |
| `OLLAMA_URL` | **nunca se usa** |
| `DOCKER_COMPOSE_DIR` | **nunca se usa** |
| `TRANSLATION_URL` | sólo lo usa `services.Translate`, que **nadie llama** |
| `SPACY_URL` | sólo lo usa `services.ExtractEntities`, que **nadie llama** |
| `services.Translate` / `services.ExtractEntities` | **código muerto** |
| `cache.NewsKey`, `cache.EntityKey`, `cache.StatsTopKey` | **nunca se escriben** (sólo se invalidan) |
| `handlers.GetAdminStats` | handler **no registrado** |
| `handlers.GetModelDownloadURL` | handler **no registrado** |
| `services/apiClient.ts` + `contexts/AppContext.tsx` | cliente paralelo **inactivo** (`AppProvider` nunca se monta) |
| `Home.tsx`, `LoginPage.tsx`, `OnboardingPage.tsx`, `SettingsPage.tsx`, `Layout.tsx` | **no enrutados** |
| `cmd/discovery.findFeedLinksInHTML` | **stub** que devuelve lista vacía |
| `Dockerfile` (raíz) | **no lo usa ningún servicio** de compose |
| `rss-ingestor-go/rss-ingestor` | **binario de 12,7 MB versionado en git** (`.gitignore` sólo cubre `bin/`) |

**Impacto.** Un cambio en `.env` puede no tener efecto (y quien lo cambie no lo sabrá).

**💡** Que compose use `${VAR:-default}` en lugar de valores fijos, y eliminar el código muerto.

---

### ISS-011 — `search.go` no selecciona `feed_id` → siempre 0 en los resultados · ✅ CONFIRMADO

**Síntoma.** `grep feed_id backend/internal/handlers/search.go` → **0 resultados**.
El DTO `models.News.FeedID int64` existe (`models.go:16`) → se serializa como `0`.

**Consecuencia observable** en la API:
```json
{"id":"f0c69b15…","lang":"es","feed_id":0,"categoria_id":7,"pais_id":59,…}
```
aunque en la BD la fila tenga `feed_id > 0` (813.959 de 814.290 filas lo tienen).

**Impacto.** El frontend recibe `feed_id: 0` en todas las búsquedas → no puede enlazar la noticia con
su fuente. `GET /api/news` sí lo selecciona correctamente.

**💡** Añadir `n.feed_id` al SELECT de `search.go` (y a la lista de campos del escaneo).

---

### ISS-012 — `topics` está vacía · ✅ CONFIRMADO

`[DB] SELECT count(*) FROM topics` → **0**. El worker `topics` lee la tabla para hacer coincidencias
por palabras clave, y escribe `news_topics` (183.023 filas) — pero **no hay filas de referencia**,
y `news_topics.topic_id` apunta a IDs inexistentes.

**Impacto.** La clasificación temática no se está aplicando en la práctica; la tabla `news_topics`
es un registro huérfano. La detección de tendencias (`GET /api/tts/trends`) usa `news_topics` y
`topics` → su calidad depende de esto.

**Nota.** `init-db/37-topics-seed.sql` existe pero apparently no se aplicó (la semilla está en
`init-db/`, que sólo corre en la primera inicialización de `data/pgdata`).

**💡** Comprobar si `37-topics-seed.sql` se aplicó; si no, aplicarla y **verificar el resultado**
(la tabla `topics` es de seeds de país/categoría).

---

### ISS-013 — `related_noticias` está vacía pese a que el worker corre · ✅ CONFIRMADO

`[DB] SELECT count(*) FROM related_noticias` → **0**. El worker `related` está `healthy` y lleva
~3 h corriendo con `RELATED_SLEEP=10`.

**Hipótesis** 🟡: el criterio de selección (traducciones `es` con `status='done'` **y** con
embedding) no encuentra filas, porque sólo 26.524 traducciones están `done` y de ésas
`traduccion_embeddings` tiene 26.304 → debería encontrar ~26 K. Alternativa: el filtro de tiempo, o
el `min_score`. Requiere traza de los logs del worker.

**Impacto.** La funcionalidad "noticias relacionadas" no devuelve nada.

**💡** Revisar los logs de `related` y el criterio de selección.

---

### ISS-014 — El motor de TTS no cubre las necesidades de calidad · ✅ CONFIRMADO (satisfacción del propietario)

**Estado técnico** `[DB] /health`:
- Chatterbox: disponible y cargado (23 idiomas, 24 kHz, clonado en un paso). **Es el único en uso.**
- MeloTTS: instalado, **nunca cargado** (el propietario lo considera insuficiente).
- OpenVoice: instalado, **nunca cargado**.
- **Kokoro, XTTS, Piper: no instalados** (`grep -iE "kokoro|xtts|pyttsx|piper"` en todo el repo → 0).

**Problemas estructurales con las alternativas**:
- MeloTTS no clona voz (sólo 256 hablantes preentrenados).
- OpenVoice **no** puede usar una voz sin embedding precomputado, y su `wavmark` está *stubbeado*
  (`workers/tts/engine.py:323-352`) → **la salida no lleva marca de agua antiabuso**.
- Chatterbox **sí** clona con un audio de referencia, en un paso → es el camino natural.

**Hueco de seguridad/producto:** **no existe ningún mecanismo de consentimiento ni registro de
autorización de voz.** `POST /api/tts/voices` (admin) sube cualquier `.wav`/`.mp3` y lo usa para
clonar. El `TTS_ALLOW_VOICE_UPLOAD=1` está activo en producción. Subir la voz de otra persona sin su
consentimiento es un riesgo legal y reputacional.

**Decisión tomada (2026-10-26):** *"el motor de TTS es Chatterbox"* y *"voces estándar y clonada
como funciona ahora"* (`DECISIONS.md` A-27, A-28).

**Consecuencias de esa decisión:**
- **No** hay que añadir clonación ni motores nuevos: ya funciona (voz estándar = `conds` guardados
  del modelo; clonada = audio de referencia).
- MeloTTS y OpenVoice **salen de producción** → `TTS_ENGINES=chatterbox`.
- ⚠️ **17 voces en la base** tienen `engine` = `openvoice` (10 `ov-*`, 3 `demo-speaker*`,
  1 `referencia-ejemplo`) o `melotts` (3 `melotts-speaker-*`). Como `resolveTTSEngine` prioriza **la voz** sobre el default global, esas
  voces fallarían con `unknown_engine` → hay que migrarlas o retirarlas (`REMEDIATION_PLAN.md` W8.3).
- El riesgo de OpenVoice (salida sin marca de agua por el `wavmark` stubbeado) desaparece al sacarlo
  de producción.
- **B-06 (registro de consentimiento) deja de ser bloqueante** y pasa a recomendado.

**Puntos de extensión verificados** (ver `ARCHITECTURE.md` §9.6): `workers/tts/engine.py`
(`VoiceEngine` ABC, `ENGINE_CLASSES`, `build_engines`, `resolve_engine`),
`workers/tts/config.py` (`TTS_ENGINES`, `TTS_ENGINE`), `workers/tts/voices.py`
(`analyze_sample`, `store_sample`), `workers/tts/service.py` (precarga, `_build_engine`),
`init-db/57-add-tts.sql` (ya persiste `engine` y `engine_version` → **no hay que tocar el esquema**),
`backend/internal/handlers/tts.go` (`resolveTTSEngine`).

---

### ISS-015 — Prometheus y Grafana no observan la aplicación · ✅ CONFIRMADO

- `monitoring/prometheus.yml` sólo define el target `prometheus` (localhost:9090) y `cadvisor`.
  El target de la aplicación (`rss2_web`) está **comentado** y apunta a un puerto inexistente.
- **No existe ningún endpoint `/metrics`** en el backend Go ni en los workers Python.
- `monitoring/grafana/dashboards/json/` está **vacío** → cero dashboards.
- El perfil `monitoring` no está activo (los contenedores `prometheus`/`grafana`/`cadvisor` no corren).

**Lo que sí existe como instrumentación:** `translation_stats` (tabla), `/api/stats/ratelimit`
(contadores), `/health` en cada worker, y los logs de `zerolog` / `logging`.

**💡** Instrumentar el backend con `promhttp` (Go) y añadir un job de scrape; crear 2–3 dashboards
mínimos (ingesta, traducción, TTS). Alternativa barata: un endpoint `/metrics` en formato texto
escrito a mano por los workers Python.

---

### ISS-027 — El corpus crece sin techo y las consultas se degradan con él · ✅ CONFIRMADO

**Síntoma.** No hay ninguna política de retención. El coste por noticia es de **9,75 KB**
(heap 1,8 + índices 0,85 + vector y HNSW 7,1 — medido sobre el corpus actual).

**Proyección** (a 477.659 noticias/día, el ritmo real medido):

| Horizonte | Noticias | Tamaño total | Índice HNSW | `COUNT(*) FROM noticias` |
|---|---|---|---|---|
| hoy (2 días) | 814 K | 3,6 GB | 117 MB | **635 ms** (medido) |
| 10 días | 4,8 M | **44 GB** | 10 GB | ~4 s |
| 30 días | 14,3 M | **133 GB** | 30 GB | ~11 s |
| 90 días | 43,0 M | **400 GB** | **90 GB** | ~33 s |
| 1 año | 174,3 M | **1,6 TB** | 366 GB | ~136 s |

**Impacto.** El `COUNT(*)` es la consulta que más sufre, y el backend la ejecuta **en cada página**
de `/api/news` y `/api/search` cuando hay filtros (`handlers/news.go:356`, `handlers/search.go:394`).
El índice HNSW (90 GB a 90 días) tiene que caber en RAM para que la búsqueda semántica sea rápida.

**Nota.** El **disco no es el límite ahora** (el propietario movdrá el sistema a hosts con más
recursos); el problema es la **degradación de las consultas** y el consumo de memoria del índice
vectorial.

**💡 Solución propuesta.** Tier frío en otra base de datos, con ventana de ~10 días en el nivel
caliente y `postgres_fdw` para mantener una sola consulta. Diseño completo en
`ARCHITECTURE.md` §13 y `DECISIONS.md` B-16. ⚠️ La ventana debe ser **compatible con el backlog de
traducción** (A-26): sólo se archiva lo ya traducido.

> ⏸️ **APLAZADO el 2026-10-26** por decisión del propietario. Se reactiva si la latencia con
> filtros se degrada de forma medible, si el corpus supera ~5 M filas, o si la RAM deja de poder
> alojar cómodamente el índice HNSW.

---

### ISS-016 — `make test` estaba en ROJO con el stack levantado · ✅ CONFIRMADO → **CERRADO 2026-10-26**

**Síntoma.** `go test -count=1 ./...` en `backend/` → **FAIL** en `internal/entitycheck`:

```
--- FAIL: TestAlertasCalidad (0.01s)
    alertas_api_test.go:58: http://127.0.0.1:8888/api/alerts?limit=500 -> HTTP 401:
        {"error":"Authorization header required"}
--- FAIL: TestAlertasFiltros
    alertas_api_test.go:133: http://127.0.0.1:8888/api/alerts?... -> HTTP 401
--- FAIL: TestPopularesEspana (1.31s)
    api_test.go:128: España · persona: /entities devuelve 0 entidades (¿parámetros rotos?)
--- FAIL: TestPopularesGlobal (0.11s)
    api_test.go:140: global · persona: 1 FP fuertes visibles (ejecuta `make entities-scan-apply`):
        [símbolos] persona  26 menc.  "Razón(s"
```

Todos los demás paquetes Go pasan (`auth`, `cache`, `config`, `handlers`, `middleware`, `textclean`),
y `rss-ingestor-go` pasa completo.

**Tres causas distintas, sólo una es un bug real de datos:**

| # | Causa | Gravedad |
|---|---|---|
| 1 | **`/api/alerts` ahora exige JWT** y los tests no envían `Authorization` | Test desincronizado con la API |
| 2 | **`/entities?country_id=…` devuelve `total = 0` a propósito** (`handlers/news.go:943-944`: *"Skip slow COUNT DISTINCT for entities. Frontend shows 'Page X' without 'of Y'"*), y `checkPopulares` trata `total == 0` como "parámetros rotos" | Test desincronizado con la API |
| 3 | **Un falso positivo fuerte es visible**: `[símbolos] persona "Razón(s"` (tag 189328, 26 menciones) | 🔴 **Bug real de datos** (ver ISS-017) |

**Trampa estructural:** los tests sólo hacen `t.Skipf` si la API **no responde** (`api_test.go:57-59`).
Con el stack **apagado** pasan (skip); con el stack **encendido** fallan. Es decir, la red de
seguridad sólo se activa cuando el sistema está en marcha — justo cuando más falta hace.

**💡 (propuesto).** (a) Enviar un token en los tests de alertas (o `t.Skip` si devuelve 401).
(b) Cambiar la aserción de `total == 0` por "la lista de entidades no está vacía" (es lo que el test
quiere comprobar). (c) Re-ejecutar `make entities-scan-apply` (ver ISS-017).

**✅ RESUELTO el 2026-10-26** (`backend/internal/entitycheck/api_test.go`):
- (a) `apiGet` ahora admite `API_TOKEN` por entorno y envía `Authorization: Bearer <token>`. Si la
  API responde **401** y no hay token, hace `t.Skip` con un mensaje claro → **nunca se incrusta una
  credencial en el repositorio** y `make test` no se rompe en un entorno sin credenciales.
- (b) `checkPopulares` comprueba `len(r.Entities) == 0` en vez de `r.Total == 0`, con un comentario
  que explica que la API devuelve `total = 0` **a propósito** en la ruta con `country_id`
  (`handlers/news.go`, *"Skip slow COUNT DISTINCT for entities"*).
- (c) ver ISS-017.
**Verificado**: `go test -count=1 ./...` en **ambos** módulos → **todo en verde**.

---

### ISS-017 — El blocklist de entidades estaba obsoleto tras el reset del corpus · ✅ CONFIRMADO → **CERRADO 2026-10-26**

**Síntoma.** `entity_blocklist` tiene 7.430 entradas, sembradas **antes** del truncado del corpus
(2026-10-25). El falso positivo `"Razón(s"` (tipo `persona`, 26 menciones, tag 189328) **no está**
en el blocklist y por tanto **aparece visible** en la página de Populares y en el grafo de
entidades.

**Causa.** El seed del blocklist lo hace el binario `entityscan` (`make entities-scan-apply`),
que clasifica cada valor de `tags` con `entitycheck.Classify`. Como el corpus se regeneró
completo, el blocklist quedó obsoleto.

**Impacto.** Falsos positivos visibles para el usuario (`"Razón(s"` como "persona"), y
`TestPopularesGlobal` en rojo.

**💡** Ejecutar `make entities-scan-apply` (dry-run primero con `make entities-scan`).
**Aviso:** es una operación de escritura sobre `entity_blocklist`; revisar el dry-run antes.

**✅ RESUELTO el 2026-10-26**:
1. **Copia de seguridad** previa de la tabla:
   `data/backup/entity_blocklist_pre_scan_20261026_215620.dump` (212 KB) — reversible.
2. **Dry-run** (`make entities-scan`, 12 s, **no escribe nada**: el `INSERT` está tras el
   `if !*apply` y la transacción hace rollback):
   168.975 valores leídos · **3.052 FP fuertes** (1,8 %) · 5.390 débiles (sólo informe).
   Por motivo: "termina en función (cortado)" 952 · "frase muy larga" 687 · "símbolos" 585 ·
   "hora/fecha pegada" 445 · "url/email" 170 · "marcador" 153 · "cta/plantilla web" 50 ·
   "solo puntuación" 9 · "palabra genérica" 1.
3. **Aplicación** (`make entities-scan-apply`): `entity_blocklist` 7.430 → **10.478** entradas
   (`ON CONFLICT (tipo, valor_lower) DO UPDATE`, idempotente).
4. **Refresco de la tabla derivada** `entity_stats` con `SELECT refresh_entity_stats();` (708 ms).
   ⚠️ **Este paso es imprescindible**: la ruta "rápida" de `/api/entities` lee `entity_stats`
   (tabla precomputada), no `tags`. Sin refrescarla, el FP seguía visible.
5. **Verificado**: `go test -count=1 ./...` → los 4 tests que fallaban ahora **pasan**.
   Los 3 FP concretos (`Razón(s`, `Merengues`, `Mbappé 1-0 Yamal`) están bloqueados.

---

## 4. Bajos

### ISS-018 — `npm test` no se puede ejecutar en esta máquina · ✅ CONFIRMADO → **CERRADO 2026-10-26**

Node.js no está instalado (`which node nodejs npm` → vacío; `node_modules/.bin/vitest` →
`env: 'node': No existe el archivo o directorio`). Los 18 tests del frontend **no se han validado**
en esta auditoría. `make test` (Go) sí es ejecutable.

**Impacto.** Un cambio en el frontend no se puede verificar localmente sin instalar Node.

**💡** Instalar Node (nvm o apt) o documentar que los tests del frontend sólo corren en CI/Docker.

**✅ RESUELTO el 2026-10-26 sin instalar nada en el host.** La imagen `node:20-alpine` ya estaba
local. Se añadió al `Makefile`:

```bash
make test-frontend   # vitest dentro de node:20-alpine
make frontend-deps   # npm install dentro del contenedor
```
Detalles que hacen que funcione:
- `--user $(id -u):$(id -g)` → el contenedor corre con el usuario del host.
- `--tmpfs /app/node_modules/.vite:rw,uid=…,gid=…` → el caché de vitest va a memoria y **no** se
  intenta escribir en `node_modules/.vite`, que es de root (EACCES) y rompe el final de la salida.
- `-e HOME=/tmp` → npm no busca config en el home del host.
**Verificado**: `make test-frontend` → **2 ficheros, 18 tests, todos en verde**, salida limpia.

---

### ISS-019 — `ALERTS_SCAN_INTERVAL_MIN` por defecto no coincide con la documentación · ✅ CONFIRMADO

`handlers/alerts.go:462` → **60 min**. `docker-compose.yml:755` → **15**. `AGENTS.md` dice "default 15".
El valor efectivo en producción es 15 (compose gana), pero el default del código es incorrecto y
confunde a quien lea el código.

**💡** Unificar en 15.

---

### ISS-020 — El `translator` (servicio 1) no declara `depends_on: db` · ✅ CONFIRMADO

`docker-compose.yml:79-127`: `translator` **no** tiene `depends_on`, mientras que `translator-2` sí
(`db: service_healthy`) y `translator-gpu` tampoco. Asimetría: el primer traductor puede arrancar
antes que la base y fallar (el `restart: unless-stopped` lo recupera, pero con ruido en el log).

**💡** Igualar los tres.

---

### ISS-021 — Semilla de `langdetect` no determinista · 🟡 SOSPECHA

`workers/langdetect_worker.py:15` **no** fija `DetectorFactory.seed = 0`, a diferencia de
`translator/workers/ctranslator_worker.py:29`. ⇒ La detección de idioma puede dar resultados
distintos para el mismo texto entre reinicios del worker.

**💡** Fijar la semilla.

---

### ISS-022 — El `translator` bloquea filas de `noticias` y compacta temprano · 🟡 SOSPECHA

1. `ctranslator_worker.py:680-693`: el `SELECT ... FOR UPDATE SKIP LOCKED` hace `JOIN noticias` **sin
   `OF t`** → PostgreSQL bloquea filas de **ambas** tablas → contención con los UPDATE masivos de
   `langdetect` y `topics`.
2. `ctranslator_worker.py:456-463`: en la ruta "mismo idioma" se hace `commit()` **antes** de marcar
   `locked_at` → se liberan los locks de `FOR UPDATE` mientras las filas aún no están marcadas
   → **ventana de doble reclamo** (estrecha, pero real).

**💡** Añadir `OF t` al `FOR UPDATE` y mover el `commit` después del `UPDATE locked_at`.

---

### ISS-023 — El cursor de embeddings se reinicia en reposo · 🟡 SOSPECHA

`workers/embeddings_worker.py:466-472`: cuando el lote de originales viene vacío, el cursor se
resetea a `None` → en reposo se re-recorre `noticias` desde el principio cada 5 s. El cursor sólo
ayuda mientras hay backlog.

**💡** No resetear el cursor a `None`; avanzar con `created_at` máximo visto y usar un
`ORDER BY created_at` consistente.

---

### ISS-024 — `embedding_vector` se escribe con un UPDATE por fila · ✅ CONFIRMADO (rendimiento)

`workers/embeddings_worker.py:245-259`: tras el `execute_values`, hace **un UPDATE por fila** para
escribir el literal `'[v1,v2,…]'::vector`. Con `EMB_BATCH=128` son 128 idas y vueltas por ciclo.

**💡** `execute_values` con la expresión vectorial, o `COPY` a una tabla temporal + `INSERT … SELECT`.

---

---

### ISS-025 — Redis puede evictar la cola TTS · 🟡 SOSPECHA

`--maxmemory 512mb --maxmemory-policy allkeys-lru` con 413.732 claves (el 99 % es caché de
traducción con TTL 30 días). `allkeys-lru` **puede** evictar claves sin TTL, entre ellas `tts:queue`.
Hoy la cola está desconectada (ISS-008), así que el riesgo es latente.

**💡** `volatile-lru` (sólo evicta claves con TTL) o poner TTL a la cola.

---

### ISS-026 — `cmd/scraper` genera ids sin `normalizeURL` · 🟡 SOSPECHA

`cmd/scraper/main.go:354`: `md5(url)` **sin** normalizar, frente al `md5(normalizeURL(url))` del
ingestor. Las noticias creadas por el scraper pueden duplicar las del ingestor o quedar huérfanas.

**💡** Reutilizar `normalizeURL` (el `textclean` ya está duplicado; el mismo patrón serviría).

---

## 5. Discrepancias entre documentación y código

| # | Documento | Afirma | El código hace | Dónde |
|---|---|---|---|---|
| D1 | `specs/AUDIT_STATUS.md` | 1.284.617 noticias, 194.324 traducciones | 814.290 noticias, 26.524 traducciones | corpus truncado el 2026-10-25 |
| D2 | `README.md` (diagrama) | El ingestor_polling "Cada 8 min" | 60 min en compose | `docker-compose.yml:232` |
| D3 | `AGENTS.md` | "Escala: ~1.500 noticias/min" | 635.681/día (441/min) en el pico | `[DB]` |
| D4 | `AGENTS.md` | "`ALERTS_SCAN_INTERVAL_MIN` (default 15)" | 60 en el código, 15 en compose | `alerts.go:462` |
| D5 | `specs/10-tts/spec.md:99` | `idempotency_key` sin `speed` | incluye `speed` | `tts.go:2151-2155` |
| D6 | `scripts/setup_tts_test_db.sh` | `make test-tts-db` / `make test-tts` | los targets son `tts-test-db` / `tts-test` | `Makefile:149,154` |
| D7 | `AGENTS.md` | "el ingestor añade ~1.500 noticias/min (33k feeds polled)" | 28.250 feeds activos, 441/min | `[DB]` |
| D8 | `docs/AUDIT_WORKERS.md` | — | state's workers' real configuration | pendiente de revisión |

---

## 6. Resumen priorizado

| # | Severidad | Esfuerzo | Bloquea |
|---|---|---|---|
| ISS-001 | 🔴 | Alto (despliegue) | El valor del producto (traducción) |
| ISS-002 | 🔴 | Medio (GPU) | La experiencia del noticiario |
| ISS-016 | 🟠 | Bajo | **CERRADO** 2026-10-26 |
| ISS-017 | 🟠 | Bajo | **CERRADO** 2026-10-26 |
| ISS-006 | 🟠 | Bajo-medio | La experiencia del noticiario |
| ISS-005 | 🟠 | Bajo | Seguridad |
| ISS-004 | 🟠 | Bajo | Estabilidad del ingestor |
| ISS-003 | 🟠 | Bajo | I/O de Postgres |
| ISS-014 | 🟠 | Medio | Calidad de voz + cumplimiento legal |
| ISS-015 | 🟠 | Bajo-medio | Observabilidad |
| ISS-011 | 🟡 | Bajo | Corrección de datos |
| ISS-012 | 🟡 | Bajo | Tendencias |
| ISS-013 | 🟡 | Bajo | Noticias relacionadas |
| ISS-008 | 🟡 | Bajo | Latencia del TTS |
| ISS-010 | 🟡 | Bajo | Confusión de configuración |
| ISS-009 | 🟡 | Bajo | Bases de datos nuevas |
| ISS-007 | 🟢 | Bajo | — |
| ISS-018 … ISS-026 | 🟢 | Bajo | — |

> El plan de corrección secuenciado está en `REMEDIATION_PLAN.md`.
