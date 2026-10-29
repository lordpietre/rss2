# Narración de noticias con TTS — guía de operación

Guía práctica del subsistema de síntesis de voz de RSS2. La arquitectura y las
decisiones están en [`specs/10-tts/spec.md`](../specs/10-tts/spec.md).

---

## 1. Qué hace

Un editor elige una noticia, aprueba el guion, encola la locución y recibe un
WAV. El audio queda **pendiente de revisión** hasta que un administrador lo
aprueba: no se publica ni se distribuye automáticamente.

---

## 2. Requisitos

- Docker y Docker Compose (el servicio es un contenedor).
- ~6 GB libres para la imagen (torch CPU + Chatterbox) y ~3,5 GB para los
  pesos (Chatterbox ~3,2 GB + OpenVoice ~130 MB).
- 3 CPU y 8 GB de RAM asignados al contenedor (configurable en
  `docker-compose.yml`). Medido en CPU: el arranque con Chatterbox cargado
  pica **~5,0 GiB**; el margen extra es para el KV cache de un guion largo.
- Sin GPU: la configuración inicial es CPU (`TTS_DEVICE=cpu`).

---

## 3. Instalación

### 3.1 Esquema de la base de datos

Bases nuevas: `init-db/57-add-tts.sql` se aplica solo al inicializar el
volumen de Postgres.

Bases ya existentes:

```bash
docker compose exec -T db psql -U rss -d rss < migrations/add_tts.sql
```

El servicio también asegura el esquema al arrancar, pero conviene aplicar la
migración para que el backend tenga las tablas desde el principio.

### 3.2 Imagen

```bash
docker compose build tts
```

### 3.3 Pesos del modelo (no se descargan al arrancar)

```bash
make tts-models           # o: ./scripts/prepare_tts_models.sh
```

Deja en `./models/chatterbox` los pesos del motor por defecto:

| Contenido | Tamaño | De dónde sale |
|-----------|--------|--------------|
| `t3_mtl23ls_v3.safetensors` — transformador multilingüe (V3) | 2,14 GB | HuggingFace `ResembleAI/chatterbox` (MIT) |
| `s3gen.pt` — decodificador de voz | 1,06 GB | HuggingFace `ResembleAI/chatterbox` |
| `ve.pt` — encoder de voz (clonación) | 5,7 MB | HuggingFace `ResembleAI/chatterbox` |
| `conds.pt` — voz por defecto del modelo | 0,1 MB | HuggingFace `ResembleAI/chatterbox` |
| `grapheme_mtl_merged_expanded_v1.json` — léxico de tokenización | 0,1 MB | HuggingFace `ResembleAI/chatterbox` |

Y en `./models/openvoice`:

| Contenido | Tamaño | De dónde sale |
|-----------|--------|--------------|
| `checkpoints_v2/` — conversor de timbre de OpenVoice V2 | 126 MB | HuggingFace `myshell-ai/OpenVoiceV2` (MIT) |
| `base_speakers/` — embeddings de timbre precalculados (10 voces) | 48 KB | HuggingFace `myshell-ai/OpenVoiceV2` |
| `voices/` — audios de referencia de las voces demo + manifiesto | 2,4 MB | Repositorio de OpenVoice |

> El bucket S3 de MyShell (`myshell-public-repo-host`) que usaba la
> documentación original **ya no existe**: el script toma los checkpoints de
> HuggingFace.

Y precarga en `./hf_cache` el modelo español de MeloTTS.

Comprobación:

```bash
./scripts/prepare_tts_models.sh --verify
```

### 3.4 Arranque

```bash
docker compose up -d tts
docker compose logs -f tts
```

El servicio **no publica puertos**: solo es alcanzable desde la red interna
`rss2_backend`. El backend Go es quien expone la API.

---

## 4. Configuración

### 4.1 Servicio TTS (contenedor `tts`)

| Variable | Default | Descripción |
|---|---|---|
| `DB_HOST` / `DB_PORT` / `DB_NAME` / `DB_USER` / `DB_PASS` | `db`, `5432`, `rss`, `rss`, — | Conexión a Postgres. |
| `REDIS_URL` | `redis://redis:6379` | Cola de despacho (opcional: hay respaldo en Postgres). |
| `TTS_API_KEY` | vacío | Clave que exige la API interna (`X-API-Key`). **Defínela en producción.** |
| `TTS_PORT` | `7000` | Puerto de la API interna. |
| `TTS_ENGINES` | `chatterbox,melotts,openvoice` | Motores activos: `chatterbox`, `melotts`, `openvoice`. |
| `TTS_ENGINE` | `chatterbox` | Motor por defecto. |
| `TTS_MELO_LANGUAGE` | `ES` | Idioma del modelo base (MeloTTS). |
| `TTS_MELO_SPEAKER_ID` | `0` | Hablante del modelo (español = 0). |
| `TTS_CHATTERBOX_MODEL_DIR` | `/app/models/chatterbox` | Pesos planos que lee `from_local`. |
| `TTS_CHATTERBOX_T3_MODEL` | `v3` | Peso del transformador: `v3`, `v2` o un `.safetensors`. |
| `TTS_CHATTERBOX_LANGUAGE` | `es` | Idioma por defecto (código de 2 letras). |
| `TTS_CHATTERBOX_EXAGGERATION` | `0.5` | Expresividad (más alto = más dramático). |
| `TTS_CHATTERBOX_CFG_WEIGHT` | `0.5` | Fuerza del clasificador (más bajo = más relajado). |
| `TTS_CHATTERBOX_TEMPERATURE` | `0.8` | Muestreo. |
| `TTS_CHATTERBOX_REPETITION_PENALTY` | `1.2` | Penalización de repeticiones. |
| `TTS_DEVICE` | `cpu` | `cpu` o `cuda`. |
| `TTS_TORCH_THREADS` | `2` | Hilos de torch. No cambiar sin medir. |
| `TTS_MAX_SCRIPT_CHARS` | `8000` | Límite de longitud de un guion. |
| `TTS_MAX_ATTEMPTS` | `2` | Reintentos por tarea. |
| `TTS_SLEEP_IDLE` | `5` | Espera entre polls de la cola (s). |
| `TTS_JOB_TIMEOUT_SECONDS` | `3600` | Antigüedad a partir de la cual una tarea en marcha se considera zombie. |
| `TTS_ALLOW_VOICE_UPLOAD` | `0` | `1` habilita el registro de muestras propias. |
| `TTS_AUDIO_DIR` | `/app/data/tts_audio` | Volumen de los WAV generados. |
| `TTS_VOICES_DIR` | `/app/data/tts_voices` | Volumen de las muestras de voz. **No lo publica ningún servidor web.** |
| `TTS_TMP_DIR` | `/app/data/tts_tmp` | Temporales (se limpian solos). |
| `TTS_OPENVOICE_MODEL_DIR` | `/app/models/openvoice` | Checkpoints de OpenVoice. |
| `TTS_BUILTIN_VOICES` | `es-base` | Voces del motor sin audio de referencia. |
| `TTS_BUILTIN_VOICES_MANIFEST` | `/app/models/openvoice/voices/manifest.json` | Manifiesto de voces preinstaladas. |
| `TTS_NORMALIZE_SCRIPT` | `1` | Normalizar cifras/fechas antes de locutar. |

### 4.2 Backend Go (contenedor `backend`)

| Variable | Default | Descripción |
|---|---|---|
| `TTS_URL` | `http://tts:7000` | Dirección del servicio de voz. |
| `TTS_API_KEY` | vacío | Debe coincidir con la del servicio. |
| `TTS_ENGINE` | `chatterbox` | Motor por defecto de los trabajos que no piden otro. |
| `TTS_AUDIO_PATH` | `/app/data/tts_audio` | Volumen (solo lectura) desde el que se sirve el audio. |
| `TTS_MAX_SCRIPT_CHARS` | `8000` | Límite de longitud de un guion. |
| `TTS_MAX_ATTEMPTS` | `2` | Reintentos por tarea. |

---

## 4 bis. Motores de voz

RSS2 no habla con un motor concreto: habla con la interfaz `VoiceEngine`
(`workers/tts/engine.py`). Cada voz de la base de datos declara qué motor
usa; el trabajo hereda el motor por defecto salvo que se pida otro.

| Motor | Qué hace | Salida | Cuándo usarlo |
|---|---|---|---|
| **`chatterbox`** (por defecto) | Sintetiza y clona en un solo paso, 23 idiomas. | WAV 24 kHz con marca de agua PerTh | Locución por defecto; clones por audio de referencia. |
| `melotts` | Voz base en español (256 hablantes). | WAV 22,05 kHz | Voces ya registradas como `melotts-speaker-N` o `es-base`. |
| `openvoice` | Convierte el timbre de un audio con un embedding precalculado. | WAV 22,05 kHz | Voces `ov-*` (embeddings `.pth`) y conversiones existentes. |

Detalles de Chatterbox que conviene conocer:

- **Salida a 24 kHz**, distinta de los 22,05 kHz de los otros motores: los
  archivos se reproducen bien en cualquier reproductor, pero la duración del
  buffer cambia.
- **Marca de agua PerTh**: toda la salida lleva la marca imperceptible de
  Resemble AI. No se puede desactivar y es el mecanismo de trazabilidad del
  modelo.
- **`speed` no es nativo**: si el trabajo pide otra velocidad, el audio se
  estira con el phase vocoder de librosa (conserva el tono).
- **Las voces `ov-*` no sirven para Chatterbox**: son embeddings `.pth`, no
  audio. Si una locución las usa con Chatterbox, la tarea falla con
  `voice_embedding_unsupported` en lugar de narrar con la voz equivocada.
- Solo el motor por defecto se precarga al arrancar; el resto carga en su
  primer uso (memoria: ~2 GB por motor cargado).

---

## 5. Uso

Todos los ejemplos necesitan un token JWT (`Authorization: Bearer <token>`).
Las rutas de voces propias y de aprobación de audio exigen administrador.

### 5.1 Crear y aprobar un guion

```bash
# Crear el guion (borrador) desde una noticia, en español
curl -s -X POST http://localhost:8888/api/tts/scripts \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"noticia_id": "abc123", "language": "es"}'

# Aprobar el guion (id 1)
curl -s -X POST "http://localhost:8888/api/tts/scripts/1/status?status=approved" \
  -H "Authorization: Bearer $TOKEN"
```

El servicio normaliza el texto: `12/03/2024` → "doce de marzo de dos mil
veinte y cuatro", `12,4%` → "doce coma cuatro por ciento", `1.234` → "mil
doscientos treinta y cuatro". El original queda en `raw_content`.

### 5.2 Encolar la locución

```bash
curl -s -X POST http://localhost:8888/api/tts/jobs \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"script_id": 1, "voice": "es-base", "speed": 1.1}'
```

La clave de idempotencia se calcula sola (`sha256(guion|voz|motor|velocidad)`):
pedir dos veces la misma narración devuelve la tarea existente en lugar de
generar otro audio.

### 5 bis.0 Voces disponibles

| Origen | Cuántas | Motor | Requiere descarga |
|--------|---------|-------|-------------------|
| **Voz por defecto de Chatterbox** (`chatterbox-default`) | 1 | `chatterbox` | `make tts-models` (3,2 GB) |
| **Clones propios con Chatterbox** | los que registres | `chatterbox` | Tu audio de referencia |
| **Voz base en español** (`es-base`) | 1 | `melotts` | `make tts-models` |
| **Hablantes del modelo** (`melotts-speaker-N`) | 256 | `melotts` | No — ya están en el modelo |
| **Voces con embedding precalculado** (`ov-es`, `ov-en-us`, …) | 10 | `openvoice` | `make tts-models` (48 KB) |
| **Clones de las voces demo** (`demo-speaker0/1/2`, `referencia-ejemplo`) | 4 | `openvoice` | `make tts-models` (2,4 MB) |

El modelo español de MeloTTS se entrenó con **256 hablantes**: cada uno es una
voz distinta (timbre, prosodia y ritmo). Se registran bajo demanda para no
llenar el catálogo:

```bash
# Ver cuántos hablantes hay
curl -s http://localhost:8888/api/tts/speakers -H "Authorization: Bearer $TOKEN"

# Registrar el hablante 42 como voz
curl -s -X POST http://localhost:8888/api/tts/speakers \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"speaker_id": 42, "engine": "melotts"}'
```

En la interfaz, el selector de voz tiene un campo **Hablante (0-255)** con un
botón *Añadir voz*: lo registras y queda en la lista para usarlo.

> Las voces del modelo no han pasado un control de calidad editorial: son
> hablantes de un corpus amplio y suenan distintos entre sí, pero algunas
> pueden tener ruido de fondo. **Pruébalas antes de usarlas en producción.**

### 5.3 Seguir el estado

```bash
curl -s http://localhost:8888/api/tts/jobs/1 -H "Authorization: Bearer $TOKEN"
```

Estados: `queued` → `running` → `succeeded` | `failed` | `cancelled`.

### 5.4 Revisar el audio

```bash
# Metadatos (duración, tamaño, procedencia)
curl -s http://localhost:8888/api/tts/audios/1 -H "Authorization: Bearer $TOKEN"

# Descargar el WAV
curl -s -o narracion.wav http://localhost:8888/api/tts/audios/1/file \
  -H "Authorization: Bearer $TOKEN"

# Aprobar (admin) para que pueda usarse en el informativo
curl -s -X POST "http://localhost:8888/api/tts/audios/1/status?status=approved" \
  -H "Authorization: Bearer $TOKEN"
```

---

## 5 bis. Noticiario por voz (varias noticias en un solo audio)

El noticiario es la pieza que convierte noticias sueltas en un informativo.
**No duplica la búsqueda ni la selección**: usa las que ya existen.

### 5 bis.1 Buscar y elegir

```bash
# Buscar (endpoint ya existente en RSS2)
curl -s "http://localhost:8888/api/search?q=econom%C3%ADa&per_page=10" \
  -H "Authorization: Bearer $TOKEN"

# Guardar la selección en una lista (también existente)
curl -s -X POST http://localhost:8888/api/lists \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name": "Noticiario del lunes"}'

curl -s -X POST http://localhost:8888/api/lists/1/items/<noticia_id> \
  -H "Authorization: Bearer $TOKEN"
```

### 5 bis.2 Elegir las noticias: por palabra clave o por tendencia

La página tiene dos modos.

**Por palabra clave**: los mismos filtros de `/api/search` más los pensados
para el informativo:

```bash
# Noticias de las ultimas 6 horas, las mas relevantes, sin repetir tema,
# con texto suficiente para locutar
curl -s "http://localhost:8888/api/search?q=economia&desde=6h&orden=relevancia\
&dedup=cluster&min_chars=150&per_page=20" -H "Authorization: Bearer $TOKEN"
```

| Parámetro | Qué hace |
|---|---|
| `desde` | Ventana temporal: `6h`, `24h`, `7d`, `30m` o una fecha ISO. |
| `hasta` | Fecha final. |
| `orden` | `reciente` o `relevancia`. |
| `min_chars` | Longitud mínima del texto que se locutará. |
| `dedup=cluster` | Una noticia por historia (no repite el mismo tema). |
| `topic_id` | Filtra por un tema. |
| `semantic=true` | Búsqueda por significado. |

**Por tendencia**: las historias que más cobertura están teniendo.

```bash
curl -s "http://localhost:8888/api/tts/trends?dias=2&importancia=breaking&min_chars=150" \
  -H "Authorization: Bearer $TOKEN"
```

Devuelve las historias agrupadas (con su noticia representativa ya traducida,
el número de medios y la importancia: `breaking`, `major`, `normal`, `minor`)
y los temas en tendencia.

> Requiere el worker `event-clustering` activo para tener historias:
> `docker compose up -d event-clustering`.

### 5 bis.3 Crear el noticiario

Desde la interfaz el flujo es **Revisión → Procesar**:

- **Revisión**: prepara los guiones y abre el texto completo para editarlo.
- **Procesar**: guarda lo editado, crea el noticiario y encola la locución.

Por API, el orden de llamadas es el mismo:

```bash
# 1. Guiones (revisables)
curl -s -X POST http://localhost:8888/api/tts/scripts/prepare \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"noticia_ids": ["abc123", "def456"], "language": "es"}'
```

El orden de `noticia_ids` es el orden de locución. `silence_ms` es la pausa
entre noticias (800 ms por defecto). `voice` elige la voz y `speed` la
velocidad de locución (0,7x–1,5x en la interfaz).

Los guiones se crean en borrador. El noticiario queda en `pending`.

### 5 bis.4 Revisar el texto antes de locutarlo

El botón **Revisión** deja los guiones listos (crea un borrador por noticia,
sin duplicar) y muestra **el texto completo, editable**, con el número de
caracteres y la duración estimada de cada bloque.

```bash
# 1. Preparar los guiones de las noticias elegidas
curl -s -X POST http://localhost:8888/api/tts/scripts/prepare \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"noticia_ids": ["abc123", "def456"], "language": "es"}'

# 2. Guardar una correccion del editor
curl -s -X PUT http://localhost:8888/api/tts/scripts/10 \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"content": "El libro recorre la historia de Okinawa. La editorial publicó 3.250 ejemplares en 2024, un 12% mas que el año anterior."}'
```

El texto se vuelve a **normalizar al guardar** (3.250 → "tres mil doscientos
cincuenta", 2024 → "dos mil veinte y cuatro", 12% → "doce por ciento"), pero
la versión escrita por el editor queda guardada en `raw_content`.

Solo se pueden editar guiones **en borrador**: si ya está aprobado, el audio
habría que regenerarlo (la API responde 409).

### 5 bis.5 Procesar: generar el audio completo

El botón **Procesar** hace las tres cosas: guarda lo que estiveras modificado,
crea el noticiario con los guiones y encola la locución de todos.

```bash
# Crear el noticiario con los guiones revisados
curl -s -X POST http://localhost:8888/api/tts/bulletins \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"title": "Noticiario del lunes", "script_ids": [10, 11],
       "silence_ms": 600, "voice": "es-base", "speed": 1.1}'

# Aprobacion del contenido: encola las narraciones
curl -s -X POST http://localhost:8888/api/tts/bulletins/1/approve \
  -H "Authorization: Bearer $TOKEN"
```

Cuando la última pieza está lista, el worker monta el noticiario y la página
muestra el reproductor y el botón de descarga.

### 5 bis.6 Seguir el progreso

```bash
curl -s http://localhost:8888/api/tts/bulletins/1 -H "Authorization: Bearer $TOKEN"
```

Cada item muestra su estado y su noticia de origen. Cuando todas las
narraciones están listas, el worker monta el noticiario y el estado pasa a
`ready`.

```json
{
  "id": 1,
  "title": "Noticiario del lunes",
  "status": "ready",
  "duration_seconds": 394.2,
  "item_count": 3,
  "silence_ms": 700,
  "items": [
    {"position": 1, "script_id": 12, "start_offset": 0.0,   "duration_seconds": 30.4,
     "noticia_id": "abc123", "source_url": "https://...", "source_name": "EFE"},
    {"position": 2, "script_id": 13, "start_offset": 31.1,  "duration_seconds": 120.8,
     "noticia_id": "def456", "source_url": "https://...", "source_name": "El País"}
  ]
}
```

`start_offset` indica dónde empieza cada noticia dentro del audio final: el
noticiario se puede mapear sobre las noticias de origen.

### 5 bis.7 Descargar y aprobar el noticiario

```bash
curl -s -o noticiario.wav http://localhost:8888/api/tts/bulletins/1/file \
  -H "Authorization: Bearer $TOKEN"

# Aprobación final (administrador)
curl -s -X POST "http://localhost:8888/api/tts/bulletins/1/status?status=approved" \
  -H "Authorization: Bearer $TOKEN"
```

### 5 bis.8 Tiempos

Cada noticia se narra por separado (en CPU, ~1,5x el tiempo real medido en
esta máquina; el doble en hardware más modesto). Un noticiario de 5 noticias
de 2 minutos son ~10 minutos de generación. La cola es asíncrona: se puede
seguir el progreso y el noticiario se monta solo cuando está la última pieza.

---

## 6. Incorporar una grabación personal de voz

La clonación personal es una fase posterior, pero la arquitectura ya la
soporta. **No se ejecuta ninguna acción sobre tus grabaciones sin que la
pidas explícitamente.**

### 6.1 Preparar la muestra

- Audio **mono**, 22,05 kHz o más, entre **3 y 60 segundos**.
- Voz limpia, sin música de fondo ni reverberación.
- Formatos habituales: WAV o MP3.

El servicio comprueba la calidad al registrarla (duración, sample rate,
canales, saturación, nivel) y devuelve el informe. Si la muestra no supera
las comprobaciones, la voz queda con `quality_status = 'poor'` y el informe
explica por qué.

### 6.2 Habilitar el registro de muestras

```bash
# En .env
TTS_ALLOW_VOICE_UPLOAD=1
docker compose up -d tts
```

### 6.3 Registrar la muestra (administrador)

```bash
curl -s -X POST http://localhost:8888/api/tts/voices \
  -H "Authorization: Bearer $TOKEN" \
  -F "sample=@mi-voz.wav" \
  -F "slug=mi-voz" \
  -F "display_name=Mi voz"
```

La muestra se guarda en `./data/tts_voices/mi-voz.wav`, un volumen que **no
monta ningún servidor web**. No existe ningún endpoint de descarga de
muestras.

### 6.4 Comprobar la calidad

```bash
curl -s http://localhost:8888/api/tts/voices/2/quality \
  -H "Authorization: Bearer $TOKEN"
```

### 6.5 Usar la voz

```bash
curl -s -X POST http://localhost:8888/api/tts/jobs \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"script_id": 1, "voice": "mi-voz"}'
```

La voz registrada se crea con motor `openvoice` (conversión de timbre y
checkpoints V2: `make tts-models`). Si quieres narrar con la misma muestra
usando Chatterbox, pide el motor en la petición:

```bash
curl -s -X POST http://localhost:8888/api/tts/jobs \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"script_id": 1, "voice": "mi-voz", "engine": "chatterbox"}'
```

### 6.6 Eliminar la muestra

```bash
curl -s -X DELETE http://localhost:8888/api/tts/voices/2 \
  -H "Authorization: Bearer $TOKEN"
```

Se borran la fila y el archivo de audio.

### 6.7 Sobre la fidelidad

OpenVoice **aproxima** el timbre de la referencia: no reproduce con exactitud
el acento, la prosodia ni la expresividad de la persona. Cualquier valoración
de calidad debe apoyarse en comparaciones auditivas controladas (la misma
locución con la voz original y con el clon, a ciegas), no en la descripción
del modelo.

---

## 7. Cambiar de motor de voz

1. Escribir una clase que implemente `VoiceEngine`
   (`workers/tts/engine.py`): `available()`, `health()`, `synthesize()`,
   `speakers()`.
2. Registrarla en `ENGINE_CLASSES`.
3. Activarla con `TTS_ENGINES` y asociarla a una voz (`tts_voices.engine`).

El pipeline editorial (guiones, tareas, audios, API) no cambia.

---

## 8. Pruebas

Las pruebas unitarias no necesitan el modelo ni los pesos: el motor se
sustituye por un doble de prueba.

```bash
# Dependencias (entorno de desarrollo)
pip install -r workers/requirements-tts.txt

# Pruebas unitarias
make tts-test

# Pruebas de integracion (requieren Postgres de pruebas)
make tts-test-db
export TEST_DATABASE_URL="postgres://rss:<password>@<host>:5432/rss_tts_test"
make tts-test
```

Si `python3` no tiene las dependencias, indica otro interprete:

```bash
make tts-test PYTHON=/ruta/a/python
```

Las pruebas de integracion estan en `workers/tests/test_tts_service.py` y
`workers/tests/test_tts_api.py`; cubren el fallo del motor, el reinicio con
tareas pendientes, los reintentos y la prevención de duplicados, la
validación del WAV y la protección de los endpoints. El noticiario tiene sus
propias pruebas en `workers/tests/test_tts_bulletin.py` (orden, silencios y
manifiesto de offsets).

---

## 9. Diagnóstico

| Síntoma | Causa probable | Solución |
|---|---|---|
| `/api/tts/status` devuelve 503 | El servicio TTS no está levantado o no responde. | `docker compose logs tts` |
| La tarea falla con `engine_not_prepared` | Faltan los pesos de Chatterbox o los checkpoints de OpenVoice. | `make tts-models` |
| La tarea falla con `engine_missing` | La imagen no tiene Chatterbox/MeloTTS/OpenVoice. | `docker compose build tts` |
| La tarea falla con `voice_embedding_unsupported` | La voz es un embedding `.pth` de OpenVoice y se pidió el motor Chatterbox. | Elegir otra voz o `"engine": "openvoice"`. |
| La tarea falla con `language_unsupported` | La voz declara un idioma que Chatterbox no soporta (23 códigos). | Revisar `language` de la voz o usar MeloTTS/OpenVoice. |
| La tarea falla con `script_not_approved` | El guion no está aprobado. | `POST /api/tts/scripts/:id/status?status=approved` |
| La tarea falla con `engine_config_invalid` | `TTS_CHATTERBOX_T3_MODEL` no es `v2`, `v3` ni un `.safetensors` válido. | Corregir la variable y reiniciar. |
| El servicio avisa "Redis no disponible" | Redis no responde; **no es un error**: la cola sigue con Postgres. | Revisar `REDIS_URL` si quieres notificaciones inmediatas. |
| El audio tarda minutos | En CPU el modelo va mucho más lento que tiempo real. | Normal y esperado; usar la cola, peticiones síncronas no. |
| El contenedor se reinicia solo al arrancar | Se supera el límite de RAM (Chatterbox ~2 GB cargado). | Subir `memory` del servicio `tts` en `docker-compose.yml`. |
| `no such table tts_jobs` | Falta la migración. | `docker compose exec -T db psql -U rss -d rss < migrations/add_tts.sql` |
