# Spec 10 — Narración de noticias con síntesis de voz (TTS)

Estado: **primera fase implementada** (generación manual + revisión editorial).
Alcance: narrar una noticia aprobada en español, guardar el WAV en disco
persistente y dejar el resultado pendiente de revisión humana.

---

## 1. Objetivo

RSS2 ingestiona, traduce y enrichece noticias. Este subsistema añade la
capacidad de **narrar** una noticia: un editor selecciona una noticia,
aprueba el guion, encola la locución y recibe un WAV que puede usar en el
informativo audiovisual.

Restricción de diseño: el audio **no se publica ni se distribuye
automáticamente**. Cada audio nace en `pending_review` y solo un
administrador lo aprueba. No hay pipeline de publicación automática en esta
fase.

## 2. Arquitectura

```
┌──────────────┐   API editorial    ┌───────────────┐   HTTP interno   ┌──────────────┐
│   Frontend   │ ─────────────────► │  Backend Go   │ ───────────────► │  Servicio    │
│  (futuro)   │                    │  ( Gin )      │                  │  TTS (Python)│
└──────────────┘                    └───────┬───────┘                  └──────┬───────┘
                                            │                                   │
                                   PostgreSQL (estado)              Chatterbox / MeloTTS / OpenVoice
                                            │                                   │
                                   Redis (cola) ────────────────────────────►│
                                     │                                       │
                                     ▼                                       ▼
                            tts_scripts / tts_jobs / tts_audios        ./data/tts_audio (WAV)
```

- **RSS2 sigue siendo el sistema principal**: ingestión, traducción, gestión
  editorial. No se reescribe nada de lo existente.
- **El servicio de TTS es independiente**: Python 3.11, CPU. No se publica
  ningún puerto: solo es alcanzable desde la red `rss2_backend`.
- **PostgreSQL** guarda estado y metadatos (`tts_voices`, `tts_scripts`,
  `tts_jobs`, `tts_audios`).
- **Redis** es el canal de despacho de tareas; Postgres es la fuente de verdad
  (cola persistente y recuperable).
- **Los archivos de audio** viven en el volumen `./data/tts_audio`. En
  Postgres solo se guarda la ruta relativa. Nunca hay blobs de audio en la
  base de datos.
- **Contrato estable**: el backend habla con el servicio por HTTP interno y
  con la base de datos. El motor de voz es una implementación de la interfaz
  `VoiceEngine`; sustituirlo (por otro TTS o por uno remoto) no obliga a
  tocar el pipeline editorial.

### 2.1 Componentes

| Componente | Ruta | Rol |
|---|---|---|
| Servicio TTS | `workers/tts_worker.py` | Punto de entrada: API + worker |
| Configuración | `workers/tts/config.py` | Todo por variables de entorno |
| Esquema | `workers/tts/schema.py` | DDL idempotente (espejo del SQL) |
| Normalizador | `workers/tts/script.py` | Cifras, fechas, abreviaturas, símbolos |
| Motores | `workers/tts/engine.py` | Interfaz + Chatterbox + MeloTTS + OpenVoice |
| Voces | `workers/tts/voices.py` | Registro y calidad de las muestras |
| Datos | `workers/tts/db.py` | Estado y metadatos |
| Cola | `workers/tts/queue.py` | Redis con respaldo en Postgres |
| API + worker | `workers/tts/service.py` | Flask + bucle de síntesis |
| API editorial | `backend/internal/handlers/tts.go` | Guiones, tareas, audios |
| Cliente Go | `backend/internal/services/tts.go` | HTTP interno hacia el TTS |
| Imagen | `Dockerfile.tts` | Python 3.11 + torch CPU |

## 3. Modelo de datos

| Tabla | Contenido |
|---|---|
| `tts_voices` | Perfiles de timbre: motor, idioma, hablante, audio de referencia, informe de calidad. `is_builtin` distingue las preinstaladas de las propias. |
| `tts_scripts` | Guion editorial: texto original (`raw_content`), texto locutado (`content`), procedencia (`noticia_id`, `translation_id`, `source_url`, `source_name`) y estado (`draft` → `approved`). |
| `tts_jobs` | Tarea de síntesis: estado, motor, voz, `idempotency_key` única, intentos, error. |
| `tts_audios` | Resultado: ruta relativa, duración, sample rate, tamaño, sha256, parámetros, motor y versión, estado editorial. |
| `tts_bulletins` | Noticiario: edición ordenada de varias narraciones unidas en un solo audio. |
| `tts_bulletin_items` | Noticia dentro de un noticiario: guion, posición, audio y desplazamiento dentro del archivo final. |

Claves foráneas: `tts_scripts.noticia_id → noticias.id` (con `ON DELETE
CASCADE`), `tts_jobs.script_id → tts_scripts.id`, `tts_audios.job_id →
tts_jobs.id`. **Cada audio conserva un vínculo verificable con su guion y con
la noticia original.**

El DDL vive en tres sitios y una prueba (`workers/tests/test_tts_schema_parity.py`)
verifica que no diverjan: `init-db/57-add-tts.sql` (canónico),
`migrations/add_tts.sql` (bases existentes) y `workers/tts/schema.py`.

## 4. Flujo editorial

1. `POST /api/tts/scripts` — se crea el guion en borrador. Si no se aporta
   texto, se toma de la traducción de la noticia en el idioma pedido. El
   servicio normaliza el texto (cifras, fechas, abreviaturas, símbolos) y
   conserva el original en `raw_content`.
2. `POST /api/tts/scripts/:id/status?status=approved` — un editor aprueba el
   guion. **Sin aprobación no hay locución.**
3. `POST /api/tts/jobs` — se encola la síntesis. La clave de idempotencia
   (`sha256(guion|voz|motor)`) evita tareas duplicadas.
4. El worker del servicio toma la tarea (Redis o Postgres), sintetiza, valida
   el WAV (existe, se decodifica, tiene duración) y registra el audio.
5. `GET /api/tts/jobs/:id` — estado de la tarea.
6. `GET /api/tts/audios/:id` — metadatos y procedencia.
7. `GET /api/tts/audios/:id/file` — descarga del WAV (con autenticación).
8. `POST /api/tts/audios/:id/status?status=approved` — un administrador
   aprueba el audio. Hasta entonces no puede usarse en el pipeline.

### 4.2 Elegir las noticias para el informativo

La búsqueda admite filtros pensados para hilar un informativo:

| Parámetro | Efecto |
|---|---|
| `desde` | Ventana temporal: fecha ISO (`2026-10-20`) o relativa (`6h`, `24h`, `7d`, `30m`). |
| `hasta` | Fecha final. |
| `orden` | `reciente` (defecto) o `relevancia` (`ts_rank` sobre el vector de búsqueda). |
| `min_chars` | Longitud mínima del texto **que se va a locutar** (contenido o resumen, traducido si existe). Filtra las noticias que darían un audio de dos segundos. |
| `dedup=cluster` | Una sola noticia por historia: se queda con la principal de cada grupo de eventos, no con las cinco fuentes que la cuentan. |
| `topic_id` | Filtra por un tema de `news_topics`. |
| `semantic=true` | Búsqueda por significado (embeddings + pgvector). Acepta los mismos filtros de ventana, longitud y deduplicación. |

Todos son opcionales: sin ellos el comportamiento es el de siempre.

### 4.3 Tendencias

`GET /api/tts/trends` devuelve lo que está manteniendo la cobertura periodística
en una ventana de tiempo:

- **Historias** (`event_clusters`): cada cluster es una historia, con su
  noticia representativa (`es_principal`), el número de medios que la cubren y
  su importancia (`breaking` / `major` / `normal` / `minor`). Es la vía para
  elegir el tema del día sin locutar cinco veces lo mismo.
- **Temas** (`news_topics` + `topics`): los temas más publicados en la ventana.

Las noticias se devuelven **traducidas al español** cuando existe traducción.

> Nota operativa: las historias dependen del worker `event-clustering`, que
> agrupa por similitud de embeddings. Si la tabla `event_clusters` está
> vacía, el endpoint devuelve solo los temas.

### 4.4 Voz y velocidad del noticiario

El noticiario guarda la **voz** (slug de `tts_voices`), el **motor** y la
**velocidad** de locución (0,5x–2,0x). Todos los items se locutan igual.

La velocidad entra en la clave de idempotencia: la misma noticia locutada más
rápido es una narración distinta.

Medición real (836 caracteres): 60,1 s a 1,0x y 48,3 s a 1,3x (la diferencia
es menor que el nominal porque las pausas entre frases no se escalan).

### 4.5 Revisión del guion (paso previo a locutar)

Antes de generar audio, el editor puede **leer y corregir el texto completo** de
cada noticia:

1. `POST /api/tts/scripts/prepare` deja listo un guion en borrador por noticia
   (si el mismo usuario ya tenía un borrador, lo reutiliza: no se duplica).
2. `PUT /api/tts/scripts/:id` guarda la corrección. El texto se **vuelve a
   normalizar** al guardar (para que las cifras y fechas se lean bien), pero la
   versión escrita por el editor queda en `raw_content`.
3. Solo se editan guiones **en borrador**. Un guion aprobado ya tiene audio
   detrás: cambiarlo obligaría a regenerarlo, así que la API lo rechaza.

### 4.6 Noticiario (varias noticias en un solo audio)

El noticiario es la pieza que convierte noticias sueltas en un informativo.
No duplica la búsqueda ni la selección: reutiliza lo que ya existe.

1. **Búsqueda**: `GET /api/search` (ya existía) con filtros `q`,
   `categoria_id`, `pais_id`, `lang` y `semantic`. Devuelve las noticias con
   su traducción al español.
2. **Selección**: el editor elige las noticias. Puede usar los resultados de
   la búsqueda o una lista guardada (`/api/lists`, ya existía).
3. **Creación**: `POST /api/tts/bulletins` con `noticia_ids` (el backend crea
   los guiones en borrador) o `script_ids` (guiones ya aprobados). El orden de
   las noticias es el orden de locución. `silence_ms` fija la pausa entre
   bloques (800 ms por defecto).
4. **Aprobación de contenido**: `POST /api/tts/bulletins/:id/approve` aprueba
   los guiones del noticiario y **encola las narraciones**. Es el momento en
   que el editor da el visto bueno al contenido.
5. **Generación**: el worker narra cada noticia por separado. Cuando la
   última pieza está lista, monta el noticiario concatenando los WAV con las
   pausas indicadas (`workers/tts/bulletin.py`).
6. **Manifiesto**: cada item guarda su `start_offset` y su duración dentro del
   audio final, además de la noticia y la fuente de origen. El audio del
   noticiario se puede mapear sobre las noticias.
7. **Aprobación final**: `POST /api/tts/bulletins/:id/status?status=approved`
   (administrador) valida el audio montado. Hasta entonces no se publica.

Estados del noticiario: `pending` (faltan piezas) → `ready` (audio montado)
→ `approved` | `rejected`. `failed` si algún item no se pudo montar.

La concatenación se hace por bloques (no se carga el noticiario entero en
memoria) y es idempotente: se puede volver a montar sin duplicar audio.

## 5. API

Todas las rutas requieren autenticación (`Bearer`). Las de gestión de voces
propias y las de aprobación de audio exigen además rol de administrador.

| Método | Ruta | Descripción |
|---|---|---|
| GET | `/api/tts/status` | Estado del motor (delega en el servicio). |
| GET | `/api/tts/voices` | Voces registradas. |
| POST | `/api/tts/voices` | Registrar muestra de voz propia (multipart, admin). |
| DELETE | `/api/tts/voices/:id` | Eliminar voz y muestra (admin). |
| GET | `/api/tts/voices/:id/quality` | Informe de calidad de la muestra (admin). |
| GET | `/api/tts/scripts` | Listar guiones (filtros: `status`, `noticia_id`). |
| POST | `/api/tts/scripts` | Crear guion en borrador. |
| POST | `/api/tts/scripts/prepare` | Prepara (o reutiliza) los guiones de varias noticias para poder revisarlos. |
| GET | `/api/tts/scripts/:id` | Detalle del guion. |
| PUT | `/api/tts/scripts/:id` | Guarda el texto revisado por el editor (se normaliza al guardar). |
| POST | `/api/tts/scripts/:id/status` | Aprobar (`approved`) o rechazar (`rejected`). |
| GET | `/api/tts/jobs` | Listar tareas. |
| POST | `/api/tts/jobs` | Encolar síntesis. |
| GET | `/api/tts/jobs/:id` | Estado de la tarea. |
| POST | `/api/tts/jobs/:id/cancel` | Cancelar (solo si está en cola). |
| GET | `/api/tts/audios` | Listar audios. |
| GET | `/api/tts/audios/:id` | Metadatos del audio. |
| GET | `/api/tts/audios/:id/file` | Archivo WAV. |
| POST | `/api/tts/audios/:id/status` | Aprobar o rechazar el audio (admin). |
| GET | `/api/tts/bulletins` | Listar noticiarios. |
| POST | `/api/tts/bulletins` | Crear un noticiario desde noticias o guiones. |
| GET | `/api/tts/bulletins/:id` | Noticiario con sus items y el manifiesto. |
| GET | `/api/tts/bulletins/:id/file` | Audio del noticiario. |
| POST | `/api/tts/bulletins/:id/approve` | Aprobar el contenido y encolar las narraciones. |
| GET | `/api/tts/bulletins/:id/status` | Aprobar o rechazar el audio final (admin). |
| GET | `/api/tts/trends` | Historias consolidadas y temas en tendencia. |

Errores: `models.ErrorResponse` (`{"error": ..., "message": ...}`), con
códigos HTTP 400 (validación), 404, 409 (estado incompatible), 413 (límite de
tamaño) y 503 (servicio TTS no disponible).

### 5.1 API interna del servicio TTS

No se expone fuera de la red de Docker. Con `TTS_API_KEY` definida exige la
cabecera `X-API-Key` (el backend Go la envía).

| Método | Ruta | Descripción |
|---|---|---|
| GET | `/health` | Estado de los motores y de la cola. Sin autenticación (healthcheck). |
| GET | `/api/voices` | Voces registradas (sin rutas internas). |
| POST | `/api/voices` | Registrar muestra (multipart). |
| DELETE | `/api/voices/:id` | Eliminar voz y muestra. |
| GET | `/api/voices/:id/quality` | Informe de calidad. |
| POST | `/api/scripts/normalize` | Normalizar un guion. |
| GET | `/api/jobs/:id` | Estado de una tarea. |
| POST | `/api/jobs/:id/cancel` | Cancelar una tarea en cola. |
| POST | `/api/synthesize` | Síntesis síncrona (pruebas y uso interno). |

## 6. Motores de voz

La interfaz `VoiceEngine` (`workers/tts/engine.py`) define el contrato:
`available()`, `health()`, `synthesize()`, `speakers()`.

| Motor | Qué hace | Requisitos |
|---|---|---|
| **`chatterbox`** (por defecto) | Chatterbox Multilingual (Resemble AI): sintetiza y clona en un solo paso, **23 idiomas**. La voz sale de un audio de referencia o de la voz por defecto del modelo (`conds.pt`). | Pesos V3 (~3,2 GB) en `models/chatterbox`. |
| `melotts` | Síntesis estándar con la voz española del modelo. El modelo se entrenó con **256 hablantes**: cada uno es una voz distinta. | Modelo ES de MeloTTS. |
| `openvoice` | Conversión de timbre: MeloTTS sintetiza y OpenVoice V2 convierte al timbre de un audio de referencia **o a un embedding precalculado** (`.pth`, 1,7 KB). | Checkpoints V2 + audio de referencia o embedding. |

Las voces con embedding precalculado (`ov-es`, `ov-en-us`, `ov-en-au`,
`ov-en-br`, `ov-en-india`, `ov-en-newest`, `ov-fr`, `ov-jp`, `ov-kr`, `ov-zh`)
son las que publica OpenVoice V2: permiten clonar el timbre sin audio de
referencia. Los hablantes del modelo se registran bajo demanda
(`POST /api/tts/speakers`) para no llenar el catálogo con 256 filas.

Cambiar de motor es registrar otra clase en `ENGINE_CLASSES` y selectable por
la voz (`tts_voices.engine`) o por la tarea. El pipeline editorial no cambia.

> **Marca de agua.** OpenVoice V2 marca el audio con `wavmark` por defecto
> (mecanismo antiuso). El servicio lo desactiva: el paquete no se distribuye y
> su modelo se descargaría en runtime, lo que contradice el diseño de no
> descargar nada durante la ejecución. Los pesos son MIT y permiten uso
> comercial. **Chatterbox es distinto**: aplica siempre la marca de agua
> PerTh, incluida en el propio paquete (no descarga nada) y sin opción de
> desactivarla; es el mecanismo de trazabilidad del modelo. Toda locución
> generada por Chatterbox sale marcada y a **24 kHz** (los otros motores
> salen a 22,05 kHz).
>
> **`speed` en Chatterbox no es nativo**: si el trabajo pide otra velocidad,
> el audio se estira con el phase vocoder de librosa (conserva el tono).
> **Una voz `ov-*` no sirve para Chatterbox**: es un embedding `.pth`, no
> audio; el motor rechaza la tarea con `voice_embedding_unsupported` en
> lugar de narrar con la voz por defecto.

**Sobre la fidelidad**: OpenVoice aproxima el timbre. No reproduce con
exactitud el acento, la prosodia ni la expresividad de la persona de
referencia. Cualquier afirmación sobre fidelidad debe apoyarse en pruebas
auditivas controladas (comparación A/B con la voz original), no en la
descripción del modelo.

## 7. Rendimiento y estabilidad

- **Una sola instancia de inferencia**: un hilo de worker y un `Lock` que
  serializa el acceso al motor. No se cargan varios modelos en memoria.
- **CPU**: `TTS_DEVICE=cpu`, `TTS_TORCH_THREADS=2` (configurable, no se
  cambia solo: cada hilo consume memoria).
- **Límites del contenedor**: 3 CPU y 8 GB de memoria (`docker-compose.yml`).
  Medido en CPU: el arranque con Chatterbox cargado pica **~5,0 GiB** (pesos
  3,2 GB + torch + page cache); el margen extra cubre el KV cache de un
  guion largo. Solo se precarga el motor por defecto (`_preload_engines`) y
  el resto carga en su primer uso.
- **Cola persistente**: Postgres. Si el worker muere, `recover_stale_jobs()`
  reencola las tareas cuyo `started_at` es antiguo (> `TTS_JOB_TIMEOUT_SECONDS`).
- **Reintentos**: `TTS_MAX_ATTEMPTS` (2 por defecto). Solo se reintentan los
  errores marcados como reintentables.
- **Idempotencia**: `tts_jobs.idempotency_key` es única.
- **Tiempos**: el worker registra el tiempo de carga del modelo, el tiempo de
  síntesis y la duración del audio.
- **Limpieza**: los temporales van a `./data/tts_tmp` y se borran al terminar
  (éxito o error). Nunca se borra un audio registrado.
- **Proceso separado**: el TTS no comparte proceso ni límites con los workers
  críticos de RSS2. Si el motor falla, la ingestión no se ve afectada.

Rendimiento medido en la máquina de desarrollo (ThinkCentre M700, i5-6500T,
CPU): ~3.700 caracteres ≈ 4 minutos de audio en ≈ 220 segundos (≈ 17x tiempo
real). Un artículo completo puede tardar varios minutos: por eso la cola es
asíncrona y el audio se sirve ya generado.

## 8. Seguridad y control editorial

- El servicio TTS **no se publica** por nginx ni tiene puertos mapeados.
- El audio se sirve por el backend Go con autenticación; la ruta se valida
  contra el directorio de audios (path traversal bloqueado, prueba en
  `backend/internal/handlers/tts_test.go`).
- Las muestras de voz personales viven en `./data/tts_voices`, que **no**
  monta ningún servidor web. No hay endpoint de descarga de muestras.
- Registrar, usar y borrar una muestra es siempre una acción explícita
  (`TTS_ALLOW_VOICE_UPLOAD` debe estar activo; el servicio no procesa ninguna
  grabación por su cuenta).
- El servicio no descarga modelos ni ejecuta código remoto al arrancar: los
  pesos se preparan con `scripts/prepare_tts_models.sh`.
- Las credenciales van por variable de entorno (`TTS_API_KEY`); no hay
  secretos en el repositorio.
- Los logs no contienen texto de las noticias ni datos personales: solo
  identificadores, códigos de error y duraciones.
- No hay publicación automática: el audio requiere aprobación de un
  administrador.

## 9. Licencias

| Componente | Licencia | Nota |
|---|---|---|
| Chatterbox (código) | MIT | `resemble-ai/chatterbox`. |
| Chatterbox (pesos multilingües V3) | MIT | HuggingFace `ResembleAI/chatterbox`. |
| MeloTTS (código) | MIT | MyShell.ai, 2024. |
| MeloTTS (pesos ES) | MIT | Repositorio `myshell-ai/MeloTTS-Spanish`. |
| OpenVoice (código) | MIT | `LICENSE` en el repositorio. |
| OpenVoice V2 (pesos) | MIT | El README declara uso comercial libre desde abril de 2024. |
| Coqui TTS (base de MeloTTS) | MPL-2.0 | Solo el código base; los pesos publicados son MIT. |

Antes de publicar en producción hay que confirmar la licencia de los pesos
concretos que se descarguen (`myshell-ai/MeloTTS-Spanish` y el paquete de
checkpoints de OpenVoice) y dejar constancia en `specs/`.

## 10. Qué queda pendiente

- Interfaz de usuario para editar el guion, aprobarlo y escuchar el audio
  (la capa de API del frontend ya existe: `frontend/src/services/api.ts`).
- Flujo completo del noticiario en el frontend: buscar → elegir → ordenar →
  aprobar → escuchar. La API está; la pantalla no.
- Locución de cabecera y de cierre del noticiario (presentación, fecha):
  hoy el noticiario son las noticias seguidas.
- Pipeline de publicación: hoy la aprobación solo cambia el estado; el
  consumo del audio por el informativo audiovisual es la fase siguiente.
- Clon de voz personal: la arquitectura ya lo contempla (voces con
  `reference_file`, motor `openvoice`, comprobación de calidad, borrado
  explícito), pero no se ha ejecutado ninguna prueba con grabaciones reales.
- Evaluación de calidad: falta una batería de pruebas auditivas A/B con
  varias voces y textos de noticias reales.
- Posible répartition de carga entre varios workers si el volumen lo exige
  (hoy `TTS_ENGINE_CLASSES` fija un solo motor por proceso).
