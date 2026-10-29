# RSS2 — Contexto del Proyecto

> **Documento de entrada.** Léelo primero al empezar una sesión.
> Última verificación: **2026-10-26 ~18:40 UTC**, contra el stack en producción (`docker compose ps`).
> Todo lo que sigue está marcado con su origen: `[CÓDIGO]`, `[DB]`, `[DOC]`, `[PROPIETARIO]`, `[HIPÓTESIS]`.

---

## 1. Propósito

RSS2 es un **agregador multilingüe de noticias RSS** que ingiere feeds de todo el mundo, detecta
el idioma, traduce al español con NLLB, extrae entidades, genera embeddings semánticos, agrupa
noticias por evento, detecta picos de atención ("alertas"), y produce **un informativo narrado en
audio** con control editorial humano previo a la locución. `[CÓDIGO]`

El servicio se despliega en un **entorno doméstico** (una sola máquina, `docker compose`) pero el
diseño apunta a poder dar servicio a otros usuarios. `[DOC: README.md, AGENTS.md]`

---

## 2. Estado actual (verificado 2026-10-26)

| Elemento | Valor | Fuente |
|---|---|---|
| Stack | 20 servicios en perfil por defecto + 4 en perfiles (`gpu`, `monitoring`) = 24 | `[DB] docker compose config` |
| Contenedores extra | 5 réplicas `rss2-backend-run-*` **no gestionadas por compose** | `[DB] docker ps` |
| PostgreSQL | 16.15 (imagen `pgvector/pgvector:pg16`), base de datos **3.592 MB** | `[DB]` |
| Redis | 7-alpine, **413.732 claves / 137 MB** (máx 512 MB, `allkeys-lru`, `appendonly yes`) | `[DB]` |
| Noticias | **814.290** | `[DB]` |
| Feeds | 33.528 totales / **28.250 activos** | `[DB]` |
| Traducciones | 453.724 filas → **26.524 `done`** / **427.200 `pending`** | `[DB]` |
| Embeddings | 58.160 | `[DB]` |
| Entidades (`tags`) / relaciones (`tags_noticia`) | 165.665 / 330.282 | `[DB]` |
| Alertas | 85, todas `nueva`, `periodo` máx. `2026-10-26 16:00` | `[DB]` |
| TTS | 23 voces, 6 guiones `approved`, 6 trabajos `succeeded`, 1 boletín `ready` (138,04 s) | `[DB]` |
| Motor TTS activo | **Chatterbox** (MeloTTS y OpenVoice instalados pero no cargados) | `[DB] GET /health` del servicio TTS` |

### 2.1 Discrepancias con las estimaciones del propietario

Las cifras del encargo son **estimaciones del propietario**, no mediciones. Contrastadas con la
base de datos en producción:

| Métrica | Estimación del propietario | **Medido (2026-10-26)** | Nota |
|---|---|---|---|
| Fuentes RSS activas | ~28.000 | **28.250** | ✅ coincide |
| Artículos nuevos/día | ~200.000 | **635.681** (2026-10-25) / 174.727 (día parcial) | ⚠️ hasta **3×** la estimación |
| Noticias originales totales | ~3.000.000 | **814.290** | ⚠️ el corpus se **truncó el 2026-10-25** (ver `AGENTS.md`) |
| Noticias traducidas | ~800.000 | **26.524** traducidas de 453.724 en cola | ❌ **no coincide**: solo el 5,8 % está traducido |

> **Conclusión operativa:** el cuello de botella real **no es la ingesta, es la traducción**.
> El backlog crece mucho más rápido de lo que se drena (ver `KNOWN_ISSUES.md` → ISS-001).

---

## 3. Arquitectura general

```
Internet → [ingestor Go] → PostgreSQL ──┬→ [langdetect] → [scheduler] → [translator NLLB]
                                         │                                        │
                                         │                                        ▼
                                         │                                  [traducciones]
                                         │                                        │
                                         │        ┌───────────────────────────────┼──────────────┐
                                         │        ▼                ▼              ▼              ▼
                                         │   [ner spaCy]     [embeddings]   [tts worker]   [wiki worker]
                                         │        │                │              │
                                         │        ▼                ▼              ▼
                                         │   tags/tags_noticia  vectores    audio WAV
                                         │        │                │
                                         │        ▼                ▼
                                         │   [alertas]  [related/event-clustering]
                                         │        │                │
                                         └────────┴────────────────┘
                                                  ▼
                                        [backend Go :8080] ← [frontend React]
                                                  ▼
                                          [nginx :8888]  ← único puerto público
```

Detalle completo: `ARCHITECTURE.md`. Diagrama Mermaid y dependencias: ahí.

**Principios estructurales verificados:**

- **PostgreSQL es la única fuente de verdad.** Redis es caché y *canal de despacho*; si Redis cae,
  el sistema sigue funcionando (fail-open) y los workers recuperan el trabajo de Postgres
  (`FOR UPDATE SKIP LOCKED`). `[CÓDIGO] backend/internal/cache/redis.go:109-119`,
  `workers/tts/queue.py:27-48`, `translator/workers/ctranslator_worker.py:692`
- **El backend Go no hace inferencia de IA.** Pide vectores al servicio de embeddings, ejecuta la
  búsqueda pgvector en Postgres, y orquesta el TTS por HTTP interno. `[CÓDIGO] backend/internal/services/ml.go`
- **El service TTS es interno**: no publica puertos, sólo es alcanzable en la red `backend`, y el
  audio sólo se sirve a usuarios autenticados. `[DOC: AGENTS.md] [CÓDIGO] docker-compose.yml:518-588`

---

## 4. Componentes principales

Detalle exhaustivo en `COMPONENTS.md`.

| Componente | Tecnología | Rol |
|---|---|---|
| `ingestor` | Go (módulo aparte `rss-ingestor-go`) | Polla 28.250 feeds, normaliza URL, deduplica por id, inserta `noticias` |
| `translation-scheduler` | Python | Crea trabajos de traducción con dos rutas (fast por `created_at`, catch-up por `updated_at`) |
| `langdetect` | Python (`langdetect`) | Detecta el idioma de `noticias.lang` |
| `translator`, `translator-2` | Python (CTranslate2 + NLLB-200-600M, int8) | Traduce a español, **~15 noticias/min cada uno** |
| `ner` | Python (spaCy `es_core_news_lg`, `en_core_web_lg`) | Extrae entidades y temas → `tags`/`tags_noticia` |
| `embeddings` | Python (sentence-transformers MiniLM multilingüe, 384 dims) | Genera vectores para original **y** traducción |
| `related`, `event-clustering` | Go | Vecinos semánticos y agrupación por evento |
| `topics` | Go | Clasifica por palabras clave → `news_topics`, infiere país |
| `wiki` | Go | Enriquece entidades con Wikipedia (resumen + miniatura) |
| `scraper` | Go | Enriquece `noticias.contenido` desde `fuentes_url` |
| `discovery` | Go | Descubre feeds RSS candidatos en páginas web |
| `cooccurrences` | Go | Recalcula pares de co-ocurrencia de entidades |
| `backend` | Go (Gin) | API REST + scanner de alertas + asignador de trabajos remotos |
| `tts` | Python (Flask) | Sintetiza voz (Chatterbox/MeloTTS/OpenVoice), ensambla boletines |
| `frontend` | React + TS + Vite | SPA de 15 rutas activas |
| `nginx` | nginx:alpine | Gateway único `:8888` |
| `db`, `redis` | PostgreSQL 16 + pgvector, Redis 7 | Persistencia y caché |

---

## 5. Tecnologías verificadas

| Tecnología | Función | Dónde | Riesgo conocido |
|---|---|---|---|
| Go 1.23 (backend) | API + 9 workers | `backend/` | Sin tests de integración en handlers |
| Go 1.25 (ingestor) | Ingesta RSS | `rss-ingestor-go/` | **Sin límite de tamaño de respuesta HTTP** |
| PostgreSQL 16 + pgvector | Persistencia + vectores | `db` | `noticias` ya ocupa 2,4 GB; sin particionado |
| Redis 7 | Caché API + caché de traducción + cola TTS | `redis` | 413 K claves, `allkeys-lru` puede evictar la cola TTS |
| spaCy + `es_core_news_lg` | NER | `ner` | `preshed` debe stay en 3.0.13 |
| CTranslate2 + NLLB-200-600M (int8) | Traducción | `translator` | Limitado a `MAX_SEQ_PER_CALL=8` por el cgroup de 3 GB |
| sentence-transformers MiniLM-L12-v2 (384 d) | Embeddings | `embeddings` | `DEVICE=cpu` en producción |
| Chatterbox Multilingual | TTS por defecto | `tts` | **19× tiempo real** (327 s para 17,6 s de audio) |
| MeloTTS | TTS alternativo (256 hablantes ES) | `tts` | Sin clonación de voz; **el propietario lo considera insuficiente** |
| OpenVoice V2 | TTS con conversión de timbre | `tts` | `wavmark` está *stubbeado* → salida sin marca de agua |
| React 18 / Vite 5 / TanStack Query v5 | Frontend | `frontend/` | Solo **2 de 21** páginas tienen tests |

---

## 6. Flujo de datos (resumen)

Detalle etapa por etapa en `DATA_FLOW.md`.

1. **Descubrimiento**: `discovery` sondea `fuentes_url`, propone feeds → `feeds`.
2. **Ingesta**: `ingestor` → `noticias` (id = md5 de URL normalizada; `ON CONFLICT DO NOTHING`).
3. **Limpieza**: `textclean` (idéntico en backend e ingestor, `make parity` lo vigila) normaliza
   título/resumen; el original queda en `*_raw`.
4. **Idioma**: `langdetect` → `noticias.lang`.
5. **Traducción**: `translation-scheduler` crea `traducciones` (`pending`); `translator` las toma con
   `FOR UPDATE SKIP LOCKED` y las deja `done` con `confidence`.
6. **Entidades**: `ner` extrae de la traducción → `tags` + `tags_noticia`.
7. **Embeddings**: `embeddings` vectoriza traducciones **y** originales ya en español
   (`traduccion_embeddings.traduccion_id IS NULL`).
8. **Relaciones**: `related` (vecinos) y `event-clustering` (eventos) sobre pgvector.
9. **Alertas**: `backend` escanea por hora y detecta picos con *baseline ponderado*.
10. **Boletín**: `frontend` → `POST /api/tts/scripts/prepare` → edición humana →
    `POST /api/tts/bulletins` → `approve` → cola → `tts` sintetiza → concatena → WAV.

---

## 7. Estado de las funcionalidades

### Terminadas y operativas `[CÓDIGO]` + `[DB]`
- Ingesta RSS multi-feed con deduplicación por URL normalizada.
- Traducción automática ES con caché Redis y cola Postgres.
- Búsqueda textual (FTS `spanish`) y **semántica** (pgvector, 384 d).
- NER + grafo de entidades + co-ocurrencias.
- Alertas por pico horario con baseline ponderado por volumen.
- Clustering de eventos y noticias relacionadas.
- Noticiario de audio **completo**: búsqueda → selección → edición → aprobación → WAV reproducible
  (verificado: boletín 1, 6 items, 138,04 s).
- Gestión de feeds (CRUD, importación/exportación CSV, borrado masivo con confirmación `428`).
- Favoritos, listas, búsquedas guardadas, alertas, suscritciones de app, usuarios y roles.

### En desarrollo / parcial
- **TTS**: sólo hay un motor realmente en uso (Chatterbox); MeloTTS y OpenVoice están instalados
  pero no se han cargado nunca. La calidad de voz sigue sin satisfacer al propietario.
- **Trabajadores remotos** (`remote-worker/`): el protocolo WebSocket funciona, pero hay **0
  traducciones** con `worker_id` y el único worker registrado (`portatil`) figura `online` con
  `last_seen` del 2026-10-09 → **falso positivo** en el panel de administración.
- **Monitoring**: Prometheus sólo se auto-scrapea y a `cadvisor`; **no hay endpoint `/metrics`** en
  la aplicación y no existe ningún dashboard de Grafana.
- **Descubrimiento de feeds**: `findFeedLinksInHTML` es un *stub* que devuelve lista vacía.

### Sólo ideas / no implementadas
- **Tier frío**: otra base de datos con el histórico y una ventana de ~10 días en el Postgres
  caliente, federada con `postgres_fdw`. Diseño en `ARCHITECTURE.md` §13. **Pendiente de P-11/P-12.**
- Persistir la parrilla del noticiario (hoy vive sólo en `useState` → **se pierde al recargar**).
- Notas/comentarios personales por guion (no existe columna ni endpoint).
- Registro de consentimiento de las voces clonadas (recomendado, **no bloqueante**).
- Particionado de `noticias` por fecha (**no viable**: el id es un md5 no monotónico).

### Decisiones del propietario tomadas el 2026-10-26
| Decisión | Consecuencia arquitectónica |
|---|---|
| **"Se traduce todo"** + **"primero las más recientes"** | La cobertura debe tender al 100 %, con **el orden actual**: `ORDER BY n.fecha DESC` (más reciente por fecha de publicación). La propuesta de FIFO quedó **descartada por medición** (los dos órdenes coinciden en 1 de 128). Brecha a cerrar: **~11×** (31/min actuales frente a 331/min de ingesta); con el orden actual, la cola antigua (155.071 de más de 7 días) no avanza sin *reparto justo* o más capacidad → `DECISIONS.md` A-26, A-31 |
| **"El motor de TTS es Chatterbox"** | MeloTTS y OpenVoice salen de producción (`TTS_ENGINES=chatterbox`). ⚠️ 17 voces tienen `engine` = `openvoice`/`melotts` → hay que migrarlas o retirarlas. → A-27 |
| **"Voces estándar y clonada como funciona ahora"** | No hay que añadir clonación: ya funciona (voz estándar = `conds` del modelo; clonada = audio de referencia). El trabajo TTS se centra en **rendimiento**. → A-28 |

---

## 8. Problemas conocidos (resumen — detalle en `KNOWN_ISSUES.md`)

| ID | Severidad | Resumen |
|---|---|---|
| ISS-001 | 🔴 Crítico | La traducción drena ~31/min frente a **331/min** de ingesta (477.659 noticias/día, 28 por feed). Backlog: 427.200 pendientes. Brecha: **11×**. |
| ISS-002 | 🔴 Crítico | El TTS tarda **19× el tiempo real** por noticia (Chatterbox en CPU). Un boletín de 2 min ≈ 30 min de espera. |
| ISS-016 | ✅ Cerrado | `make test` **estaba** en rojo con el stack levantado. Arreglado el 2026-10-26 (2 tests desincronizados). |
| ISS-017 | ✅ Cerrado | El `entity_blocklist` estaba obsoleto tras el reset. Re-sembrado el 2026-10-26 (10.478 entradas) + refresco de `entity_stats`. |
| ISS-003 | 🟠 Alto | `feeds` acumuló **81.173 millones** de tuplas leídas por escaneo secuencial (4,6 M scans). |
| ISS-004 | 🟠 Alto | Sin límite de tamaño de respuesta en el ingestor → riesgo de OOM (contenedor: 512 MB). |
| ISS-005 | 🟠 Alto | El nivel de admin se decide sólo por el claim del JWT (24 h) → un admin degradado conserva privilegios. |
| ISS-006 | 🟡 Medio | La parrilla del noticiario no se persiste; además `processBulletin` reutiliza guiones viejos si se edita tras abrir la revisión. |
| ISS-007 | 🟡 Medio | El `idempotency_key` que envía el usuario en `POST /api/tts/jobs` se ignora (se recalcula). |
| ISS-008 | 🟡 Medio | La cola Redis del TTS no está conectada (`"redis": false` en `/health`) → todo el trabajo se recupera de Postgres. |
| ISS-009 | 🟡 Medio | `workers/embeddings_worker.py:77` declara `traduccion_id INT NOT NULL`, contradiciendo la migración 60. |
| ISS-010 | 🟢 Bajo | Configuración muerta en `.env` (`RSS_MAX_WORKERS`, `RSS_POKE_INTERVAL_MIN`, `EMB_DEVICE`, `NER_LANG`). |
| ISS-027 | 🟠 Alto | El corpus crece **9,75 KB/noticia** (400 GB a 90 días). El `COUNT(*)` de cada página pasa de 635 ms a ~33 s. Propuesta de tier frío en `ARCHITECTURE.md` §13. |
| ISS-014 | 🟠 Alto | El TTS sólo tiene un motor en producción; MeloTTS y OpenVoice están instalados pero **no se usan**. La insatisfacción con la calidad se ataca ahora por **rendimiento** (18,6× tiempo real), no por falta de motores: **Chatterbox ya es el motor decidido** (A-27). |

---

## 9. Restricciones del entorno

- **Host**: 4 cores, CPU sobrecargada (load ≈15). 8 GB de swap en dos ficheros; **ampliar swap no
  ayuda** al límite de cgroup (la memoria del cgroup cuenta también las páginas swapeadas). `[DOC: AGENTS.md]`
- **Límites de cgroup**: `translator` 3 GB, `tts` 8 GB, `backend` 512 MB, `ingestor` 512 MB.
- **`MAX_SEQ_PER_CALL=8`** es obligatorio: 8 seq ≈ 1,8 GiB de pico; 32 → ~3,1 GiB (OOM). `[DOC: AGENTS.md]`
- **Postgres**: `shm_size: 256m` es obligatorio (si no, falla la agregación hash paralela con
  SQLSTATE 53100). `[DOC: AGENTS.md]`
- **nginx debe resolver el upstream dinámicamente** (`resolver 127.0.0.11` + variable en
  `proxy_pass`), si no retiene la IP vieja tras un recreado de contenedor. `[DOC: AGENTS.md]`
- **Rotación de logs obligatoria**: el ancla `x-logging` (20 MB × 3) está en los 24 servicios;
  recrear el contenedor es lo que la aplica.
- **Sin CI**: `make test` y `npm test` son la única red de seguridad.

---

## 10. Decisiones importantes

Ver `DECISIONS.md`. Las verificadas (con su porqué) incluyen: URL normalizada como id de noticia;
Redis como caché y no como fuente de verdad; `LEFT JOIN` obligatorio en el filtro de "traducido";
baseline de alertas ponderado por volumen; PostgreSQL como única fuente de verdad; `textclean`
duplicado con verificación de paridad; TTS como servicio interno separado del backend Go.

---

## 11. Mapa documental

| Documento | Contenido |
|---|---|
| `PROJECT_CONTEXT.md` | **Este documento.** Primera lectura. |
| `ARCHITECTURE.md` | Componentes, dependencias, comunicación, diagramas Mermaid. |
| `DATA_FLOW.md` | Recorrido de los datos etapa por etapa, qué pasa si cada etapa falla. |
| `DATABASE.md` | Esquema, relaciones, índices, consultas hot, volumen, cuellos de botella, Redis. |
| `COMPONENTS.md` | Inventario de módulos, servicios, workers, tareas y responsabilidades. |
| `KNOWN_ISSUES.md` | 26 problemas con evidencia, hipótesis y soluciones posibles. |
| `REMEDIATION_PLAN.md` | **Plan de corrección** en 9 oleadas, con verificación, rollback y quick wins. |
| `DECISIONS.md` | Decisiones arquitectónicas **verificadas** (A) y **recomendaciones nuevas** (B), separadas. |
| `ROADMAP.md` | Qué existe, qué falta, dependencias entre tareas. |
| `SESSION_HANDOFF.md` | Estado de la última sesión y siguiente paso. |

Documentos preexistentes **fuera** de esta carpeta (no duplicados aquí):
`README.md` (instalación), `AGENTS.md` (constitución y quirks), `explicacion.md` (divulgación),
`diagrama.md`, `docs/TTS.md` (manual de operación TTS), `specs/` (especificaciones por área),
`docs/REVISION_CODIGO.md`, `docs/AUDIT_*.md`.

---

## 12. Protocolo para futuras sesiones

1. Leer este documento.
2. Leer el documento específico del área a tocar (`DATA_FLOW.md`, `DATABASE.md`, `KNOWN_ISSUES.md`…).
3. **Verificar el estado real del código** antes de fiarse de lo que dice cualquier documento.
4. Comprobar dependencias y efectos secundarios (ver `ARCHITECTURE.md` → grafo de dependencias).
5. Ejecutar `make test` y `cd frontend && npm test` (no hay CI).
6. Al terminar: actualizar el documento afectado, `KNOWN_ISSUES.md` y `SESSION_HANDOFF.md`.
7. No reescribir la documentación entera: **actualizar sólo lo necesario** y conservar el historial.
8. La documentación **no** sustituye al código como fuente de verdad.
