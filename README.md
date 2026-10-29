# 🌐 RSS2 — Agregador Multilingüe de Noticias RSS

> **Tu窗口 al mundo**: Noticias de miles de fuentes internacionales, traducidas automáticamente al español.

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](./LICENSE)
[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8.svg)](https://golang.org/)
[![React](https://img.shields.io/badge/React-18-61DAFB.svg)](https://reactjs.org/)

---

## 🎯 Qué es RSS2

RSS2 es un **agregador de noticias RSS inteligente** que:

- 🌐 **Recolecta** noticias de miles de fuentes RSS/Atom de todo el mundo
- 🌍 **Detecta** automáticamente el idioma de cada noticia
- 🇪🇸 **Traduce** al español usando NLLB (No Language Left Behind) de Meta
- 👤 **Extrae** personas, organizaciones y temas mencionados (NER)
- 🔗 **Relaciona** noticias similares usando embeddings semánticos
- 📚 **Enriquece** con información de Wikipedia
- 📊 **Muestra** tendencias y alertas de actividad inusual
- ❤️ **Permite** guardar favoritos y crear listas personalizadas
- 🎙️ **Narra** un informativo en voz: busca, elige, revisa el guion y genera el audio

Todo en un **único puerto**: `:8888`

---

## 🚀 Inicio Rápido

### Requisitos

- Docker y Docker Compose
- 8GB+ RAM recomendados
- ~20GB espacio en disco

### Instalación

```bash
# 1. Clonar el repositorio
git clone <repo-url> && cd rss2

# 2. Copiar y configurar variables de entorno
cp .env.example .env
# Editar .env y rellenar:
# - POSTGRES_PASSWORD (contraseña para PostgreSQL)
# - REDIS_PASSWORD (contraseña para Redis)
# - SECRET_KEY (clave secreta para JWT)

# 3. Iniciar la aplicación
docker compose up -d --build          # Build + levanta todos los servicios

# 4. Esperar a que esté listo (~5 minutos) y acceder a:
```

🌐 **Frontend**: http://localhost:8888  
📡 **API REST**: http://localhost:8888/api  
❤️ **Favoritos**: Requiere registro/login

---

## 🏗️ Arquitectura

```
                         INTERNET
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                        RSS2 SYSTEM                               │
│                                                                  │
│  ┌──────────────┐      ┌──────────────────────────────────┐    │
│  │   INGESTOR   │────▶│           POSTGRES DB             │    │
│  │     (Go)      │      │    + pgvector (búsqueda vectorial) │    │
│  │  Cada 8 min   │      │                                   │    │
│  └──────────────┘      └──────────────────────────────────┘    │
│                                  │                               │
│                                  ▼                               │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                    WORKERS (en paralelo)                 │   │
│  │                                                           │   │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────────┐     │   │
│  │  │ langdetect │  │translator │  │  embeddings     │     │   │
│  │  │ Detecta    │  │  NLLB-200 │  │  sentence-     │     │   │
│  │  │  idioma    │  │  CPU/GPU  │  │  transformers │     │   │
│  │  └────────────┘  └────────────┘  └────────────────┘     │   │
│  │                                                           │   │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────────┐     │   │
│  │  │    NER     │  │    wiki    │  │   related      │     │   │
│  │  │   spaCy    │  │  Wikipedia │  │   coseno SQL   │     │   │
│  │  │   NER      │  │  resumen   │  │                │     │   │
│  │  └────────────┘  └────────────┘  └────────────────┘     │   │
│  │                                                           │   │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────────┐     │   │
│  │  │  topics    │  │  scraper   │  │  discovery     │     │   │
│  │  │ categoría  │  │enriquece   │  │ nuevos feeds   │     │   │
│  │  │   país     │  │ resumenes  │  │               │     │   │
│  │  └────────────┘  └────────────┘  └────────────────┘     │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                  │                               │
│                                  ▼                               │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │              API Go (Puerto interno :8080)                │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                  │                               │
│                                  ▼                               │
│  ┌──────────────────┐      ┌──────────────────────────────┐    │
│  │   FRONTEND       │◀────│   NGINX GATEWAY (:8888)      │    │
│  │   (React SPA)    │      │   Un único puerto público    │    │
│  └──────────────────┘      └──────────────────────────────┘    │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Componentes Principales

#### 🔙 Backend (Go)

- **API REST** (`backend/cmd/server/`) — Puerto interno :8080
- **Workers Go** compilados como binarios separados:
  - `/scraper` — Enrichment de resúmenes cortos
  - `/discovery` — Descubrimiento de nuevos feeds RSS
  - `/topics` — Categorización y detección de país
  - `/wiki` — Resúmenes e imágenes de Wikipedia
  - `/related` — Cálculo de noticias relacionadas

#### 🎨 Frontend (React)

- **SPA** con Vite + TypeScript + Tailwind CSS
- **Páginas**:
  - Portada con carrusel y tendencias
  - Búsqueda con filtros avanzados
  - Entidades populares (red de personas/orgs)
  - Radar de trends
  - Cobertura global (mapa mundial)
  - Evolución temporal de entidades
  - Alertas de actividad inusual
  - Perfil de usuario con favoritos y listas

#### 🐍 Workers Python

| Worker | Tecnología | Función |
|--------|-----------|---------|
| `langdetect` | langdetect | Detecta idioma de noticias |
| `translation-scheduler` | psycopg | Encola traducciones pendientes |
| `translator` | CTranslate2 + NLLB-200 | Traduce al español |
| `ner` | spaCy (`es_core_news_lg`) | Extrae entidades nombradas |
| `embeddings` | sentence-transformers | Genera embeddings multilingües |
| `event-clustering` | pgvector | Agrupa noticias por historia (tendencias) |
| `tts` | Chatterbox V3 + MeloTTS + OpenVoice V2 | Narra noticias aprobadas (ver [spec 10](specs/10-tts/spec.md)) |

#### 🗄️ Datos

- **PostgreSQL 16** + **pgvector** — Base de datos principal
- **Redis** — Caché y rate limiting
- **nginx** — Gateway API (un solo puerto :8888)

---

## 📊 Pipeline de Datos

```
feeds RSS
    │
    ▼
┌─────────────────┐
│    INGESTOR     │  Cada 8 min: descarga feeds, inserta noticias
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   LANGDETECT    │  Detecta idioma (¿en? ¿ja? ¿ru? ¿pt?...)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ TRANSLATION     │  Crea jobs pendientes en tabla traducciones
│  SCHEDULER     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  TRANSLATOR     │  NLLB-200: traduce contenido → español
│   (CPU/GPU)     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│      NER        │  spaCy: extrae personas, orgs, temas
│    (spaCy)      │
└────────┬────────┘
         │
         ├──────────────────┐
         ▼                  ▼
┌─────────────────┐  ┌─────────────────┐
│   EMBEDDINGS    │  │      WIKI       │
│ sentence-trans.  │  │   Wikipedia     │
└────────┬────────┘  └────────┬────────┘
         │                   │
         ▼                   │
┌─────────────────┐         │
│    RELATED      │         │
│  coseno SQL     │         │
└────────┬────────┘         │
         │                   │
         ▼                   ▼
┌─────────────────────────────────────────┐
│              news_topics                 │
│         categorías + país                 │
└─────────────────────────────────────────┘
```

---

## 🔌 Endpoints Principales

### Noticias

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/news` | Lista de noticias con paginación |
| GET | `/api/news/:id` | Detalle de una noticia |
| GET | `/api/feeds` | Lista de feeds |

### Entidades

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/entities` | Entidades populares |
| GET | `/api/entities/news?valor=&tipo=` | Noticias de una entidad |
| GET | `/api/entities/graph?tipo=` | Grafo de co-ocurrencias |

### Búsqueda

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/search?q=` | Búsqueda con filtros |
| GET | `/api/tags/popular` | Tags más mencionados |

### Estadísticas

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/stats` | Estadísticas globales |
| GET | `/api/countries` | Países disponibles |
| GET | `/api/categories` | Categorías disponibles |

### Usuario

| Método | Ruta | Descripción |
|--------|------|-------------|
| POST | `/api/auth/register` | Registro |
| POST | `/api/auth/login` | Login |
| GET | `/api/favorites` | Favoritos |
| GET | `/api/lists` | Listas del usuario |

### Alertas

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/alerts` | Lista de alertas |
| POST | `/api/alerts/:id/read` | Marcar leída |
| POST | `/api/alerts/read-all` | Marcar todas leídas |

### Narración / Noticiario (TTS)

Todas requieren autenticación. Las de voces propias y la aprobación final del
audio exigen rol de administrador.

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/tts/status` | Estado del motor de voz |
| GET | `/api/tts/voices` | Voces disponibles (base, clones y hablantes registrados) |
| GET | `/api/tts/speakers` | Hablantes del modelo base (256 en el español) |
| POST | `/api/tts/speakers` | Registrar un hablante del modelo como voz |
| POST | `/api/tts/voices` | Registrar una muestra de voz propia (multipart, admin) |
| DELETE | `/api/tts/voices/:id` | Eliminar una voz y su muestra (admin) |
| GET | `/api/tts/voices/:id/quality` | Informe de calidad de la muestra (admin) |
| GET | `/api/tts/trends` | Historias consolidadas y temas en tendencia |
| GET | `/api/tts/scripts` | Listar guiones (filtros: `status`, `noticia_id`) |
| POST | `/api/tts/scripts` | Crear un guion en borrador |
| POST | `/api/tts/scripts/prepare` | Preparar (o reutilizar) los guiones de varias noticias para revisarlos |
| GET | `/api/tts/scripts/:id` | Detalle del guion |
| PUT | `/api/tts/scripts/:id` | Guardar el texto revisado por el editor |
| POST | `/api/tts/scripts/:id/status` | Aprobar (`approved`) o rechazar (`rejected`) un guion |
| GET | `/api/tts/jobs` | Tareas de locución |
| POST | `/api/tts/jobs` | Encolar una locución (idempotente) |
| GET | `/api/tts/jobs/:id` | Estado de la tarea |
| POST | `/api/tts/jobs/:id/cancel` | Cancelar una tarea en cola |
| GET | `/api/tts/audios` | Audios generados |
| GET | `/api/tts/audios/:id` | Metadatos del audio |
| GET | `/api/tts/audios/:id/file` | Descargar el WAV |
| POST | `/api/tts/audios/:id/status` | Aprobar o rechazar el audio (admin) |
| GET | `/api/tts/bulletins` | Noticiarios |
| POST | `/api/tts/bulletins` | Crear un noticiario a partir de noticias o guiones |
| GET | `/api/tts/bulletins/:id` | Noticiario con sus items y el manifiesto de offsets |
| GET | `/api/tts/bulletins/:id/file` | Audio del noticiario |
| POST | `/api/tts/bulletins/:id/approve` | Aprobar el contenido y encolar las narraciones |
| POST | `/api/tts/bulletins/:id/status` | Aprobar o rechazar el audio final (admin) |

---

## 🎙️ Noticiario por voz (TTS)

Subsistema opcional que convierte noticias en un **informativo hablado**:
buscas las noticias, eliges las que van, corriges el texto y generas el audio.

Todo pasa por la web: **http://localhost:8888/noticiario** (icono de micrófono
en el menú lateral).

### Qué hace

| Paso | Qué ocurre |
|------|-----------|
| 1. **Buscar** | Por palabra clave o por tendencia, con filtros pensados para el informativo. |
| 2. **Elegir** | Las noticias del informativo, ordenadas (el orden es el de locución). |
| 3. **Revisión** | El texto completo de cada noticia, editable, con caracteres y duración estimada. |
| 4. **Procesar** | Se guarda lo corregido, se crea el noticiario y se encola la locución. |
| 5. **Descargar** | El WAV montado, pendiente de aprobación editorial. |

### Dos puertas de control editorial

1. **Aprobar y generar las narraciones** — el editor aprueba el *contenido*.
2. **Aprobar** (administrador) — se aprueba el *audio* montado.

Nada se publica ni se distribuye automáticamente: el audio nace en
`pending_review` y el servicio no tiene ningún endpoint de publicación.

### Búsquedas para hilar el informativo

| Filtro | Efecto |
|--------|--------|
| Ventana temporal | `6h`, `24h`, `7d`, `30m` o una fecha ISO (`2026-10-20`). Por defecto **7 días**. |
| Orden | `reciente` (por defecto) o `relevancia` (`ts_rank` sobre el vector de búsqueda). |
| Texto mínimo | Filtra las noticias demasiado cortas para locutar. |
| Una por tema | `dedup=cluster`: se queda con la noticia principal de cada historia, no con las cinco fuentes que la cuentan. |
| Tema | Filtra por los temas detectados (`news_topics`). |
| Semántica | Búsqueda por significado (embeddings + pgvector). |

En **Por tendencia** se listan las historias con más cobertura periodística
(cada cluster es una historia, con su noticia representativa ya traducida, el
número de medios y su importancia: `breaking`, `major`, `normal`, `minor`) y
los temas en tendencia.

> Las historias dependen del worker `event-clustering`, que agrupa por
> similitud de embeddings. Si la tabla `event_clusters` está vacía, el
> endpoint devuelve solo los temas.

### Voces y velocidad

- **Voz**: la voz por defecto de **Chatterbox** (motor por defecto, 23
  idiomas), la base en español (`es-base`), cualquiera de los **256
  hablantes** del modelo, las voces con embedding precalculado (`ov-es`,
  `ov-en-us`, …) o un clone tuyo registrado con una muestra de audio.
- **Velocidad**: 0,7x–1,5x desde la interfaz (0,5x–2,0x por API). El mismo
  texto a 1,0x tarda 60 s y a 1,3x 48 s.
- La velocidad forma parte de la clave de idempotencia: la misma noticia
  locutada más rápido es una narración distinta.

```bash
# Ver los hablantes del modelo y registrar uno como voz
curl -s http://localhost:8888/api/tts/speakers -H "Authorization: Bearer $TOKEN"
curl -s -X POST http://localhost:8888/api/tts/speakers \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"speaker_id": 42, "engine": "melotts"}'
```

> Las voces del modelo no han pasado un control de calidad editorial: son
> hablantes de un corpus amplio y algunas pueden tener ruido de fondo.
> Pruébalas antes de usarlas en producción.

### Clonar tu propia voz

1. Graba entre 3 y 60 s de voz limpia, en mono y a 22,05 kHz o más.
2. Activa `TTS_ALLOW_VOICE_UPLOAD=1` en `.env` y reinicia el servicio.
3. Sube la muestra (administrador):

```bash
curl -X POST http://localhost:8888/api/tts/voices \
  -H "Authorization: Bearer $TOKEN" \
  -F "sample=@mi-voz.wav" -F "slug=mi-voz" -F "display_name=Mi voz"
```

4. Comprueba la calidad y úsala en el noticiario:

```bash
curl http://localhost:8888/api/tts/voices/2/quality -H "Authorization: Bearer $TOKEN"
```

Las muestras viven en `./data/tts_voices`, un volumen que **no publica
ningún servidor web**, y borrarlas es siempre una operación explícita.

> OpenVoice **aproxima** el timbre: no reproduce con exactitud el acento, la
> prosodia ni la expresividad de la persona. Cualquier valoración de calidad
> debe apoyarse en comparaciones auditivas controladas, no en la descripción
> del modelo.

### Requisitos y tiempos

- CPU (por defecto): 3 CPU y 4 GB para el contenedor `tts`.
- Velocidad de locución: ~1,5x el tiempo real en esta máquina (2,7 millones
  de noticias, ~1,5x). Un noticiario de 5 noticias de 2 minutos son ~10
  minutos de generación. La cola es asíncrona: el audio se monta solo cuando
  llega la última pieza.
- El audio se guarda en `./data/tts_audio`; en la base solo queda la ruta y
  los metadatos (nunca blobs).

---

## 🛠️ Comandos de Desarrollo

### Go (Backend)

```bash
make all          # Deps + build completo
make deps         # go mod download + tidy
make build        # Compilar binarios → bin/
make clean        # Limpiar bin/
make test         # Tests en backend/ y rss-ingestor-go/
make parity       # Verificar que textclean es idéntico en ambos módulos
```

### Workers Go (local)

```bash
make run-scraper     DB_HOST=localhost DB_PORT=5432 DB_NAME=rss DB_USER=rss DB_PASS=rss
make run-discovery   DB_HOST=localhost DB_PORT=5432 DB_NAME=rss DB_USER=rss DB_PASS=rss
make run-topics      DB_HOST=localhost DB_PORT=5432 DB_NAME=rss DB_USER=rss DB_PASS=rss
make run-related     DB_HOST=localhost DB_PORT=5432 DB_NAME=rss DB_USER=rss DB_PASS=rss RELATED_SLEEP=10
```

### Frontend

```bash
cd frontend
npm install        # Instalar dependencias
npm test           # Tests con vitest
npm run build      # Build producción
npm run dev        # Desarrollo
```

### Docker

```bash
# Primera vez: construir imagen compartida del backend
docker compose build backend

# Luego iniciar todo
docker compose up -d

# Ver logs
docker compose logs -f backend

# Reiniciar un servicio
docker compose restart backend
```

### Narración (TTS)

```bash
make tts-models     # Descarga los pesos (Chatterbox V3 + OpenVoice V2 + MeloTTS ES)
make tts-test-db    # Crea la base de pruebas (rss_tts_test)
make tts-test       # Tests del subsistema (no necesitan modelo ni pesos)
make tts-test PYTHON=/ruta/a/python   # Si tu python3 no tiene las dependencias
```

Los tests de integración necesitan `TEST_DATABASE_URL`:

```bash
export TEST_DATABASE_URL="postgres://rss:<password>@<host>:5432/rss_tts_test"
make tts-test
```

---

## 📁 Estructura del Proyecto

```
rss2/
├── backend/                  # API Go + workers compilados
│   ├── cmd/
│   │   ├── server/          # API REST (main.go)
│   │   ├── scraper/         # Worker: enrichment
│   │   ├── discovery/        # Worker: nuevos feeds
│   │   ├── topics/          # Worker: categorización
│   │   ├── wiki/            # Worker: Wikipedia
│   │   ├── related/         # Worker: noticias relacionadas
│   │   └── event_clustering/# Worker: agrupación por historia (tendencias)
│   ├── internal/
│   │   ├── handlers/        # Handlers HTTP
│   │   ├── models/          # Modelos de datos
│   │   ├── cache/           # Redis cache
│   │   └── db/              # Conexión PostgreSQL
│   └── Dockerfile
│
├── frontend/                 # SPA React
│   ├── src/
│   │   ├── pages/           # Páginas (Home, Search, Noticiario, etc.)
│   │   ├── components/      # Componentes reutilizables
│   │   ├── services/         # Llamadas API
│   │   └── hooks/           # React hooks
│   └── Dockerfile
│
├── workers/                  # Workers Python
│   ├── ner/                 # Named Entity Recognition
│   ├── langdetect/           # Detección de idioma
│   ├── translation-scheduler/# Scheduler de traducciones
│   ├── embeddings/           # Embeddings semánticos
│   ├── tts/                  # Servicio de voz (motor, guiones, noticiario)
│   ├── tts_worker.py         # Punto de entrada del servicio de voz
│   └── tests/                # Tests del subsistema TTS
│
├── translator/               # Worker de traducción NLLB
│
├── scripts/                 # Utilidades de operación
│   ├── prepare_tts_models.sh # Descarga los pesos del motor de voz
│   └── setup_tts_test_db.sh  # Base de datos de pruebas del TTS
│
├── init-db/                  # Scripts SQL de inicialización
│   ├── 001_init.sql         # Schema principal
│   ├── 002_*.sql            # Migraciones
│   └── ...
│
├── migrations/               # Migraciones para bases ya existentes
│
├── specs/                    # Especificaciones
│   ├── 00-architecture/      # Arquitectura del sistema
│   ├── 01-system-overview/  # Visión general
│   ├── 05-api-backend/     # Contratos API
│   ├── 10-tts/              # Narración por voz (TTS)
│   ├── constitution.md       # Reglas obligatorias
│   └── plan.md              # Roadmap
│
├── nginx.conf                # Configuración nginx
├── docker-compose.yml        # Orquestación Docker
├── Dockerfile.tts            # Imagen del servicio de voz (Python + torch CPU)
├── Makefile                  # Comandos de desarrollo
├── .env.example              # Variables de entorno ejemplo
└── README.md                # Este archivo
```

---

## 🔧 Configuración

### Variables de Entorno

| Variable | Descripción | Requerido |
|---------|-------------|-----------|
| `POSTGRES_PASSWORD` | Contraseña PostgreSQL | ✅ |
| `REDIS_PASSWORD` | Contraseña Redis | ✅ |
| `SECRET_KEY` | Clave para JWT | ✅ |
| `DB_HOST` | Host PostgreSQL (default: db) | No |
| `DB_PORT` | Puerto PostgreSQL (default: 5432) | No |
| `REDIS_URL` | URL Redis (default: redis://:pwd@redis:6379) | No |
| `TTS_API_KEY` | Clave compartida backend ↔ servicio de voz | No |
| `TTS_ENGINES` | Motores de voz activos (default: `chatterbox,melotts,openvoice`) | No |
| `TTS_ENGINE` | Motor por defecto (default: `chatterbox`) | No |

### Narración de noticias (TTS)

El subsistema de síntesis de voz es **opcional** y no se activa por defecto.
Permite buscar noticias, elegir las de un informativo y generar un noticiario
audio con la voz en español.

```bash
# 1. Preparar los pesos (fuera del arranque del servicio)
make tts-models

# 2. Arrancar el servicio (interno: no publica puertos)
docker compose up -d tts

# 3. Esquema en bases ya existentes
docker compose exec -T db psql -U rss -d rss < migrations/add_tts.sql
```

En una base nueva, `init-db/57-add-tts.sql` se aplica solo.

#### Variables del servicio `tts`

| Variable | Default | Descripción |
|----------|---------|-------------|
| `TTS_API_KEY` | vacío | Clave compartida con el backend (`X-API-Key`). **Defínela en producción.** |
| `TTS_PORT` | `7000` | Puerto de la API interna (no publicado). |
| `TTS_ENGINES` | `chatterbox,melotts,openvoice` | Motores activos: `chatterbox`, `melotts`, `openvoice`. |
| `TTS_ENGINE` | `chatterbox` | Motor por defecto. |
| `TTS_MELO_LANGUAGE` | `ES` | Idioma del modelo base (MeloTTS). |
| `TTS_MELO_SPEAKER_ID` | `0` | Hablante del modelo (español = 0). |
| `TTS_CHATTERBOX_T3_MODEL` | `v3` | Peso del transformador multilingüe (`v3`, `v2`). |
| `TTS_CHATTERBOX_LANGUAGE` | `es` | Idioma por defecto de Chatterbox. |
| `TTS_CHATTERBOX_EXAGGERATION` | `0.5` | Expresividad de Chatterbox. |
| `TTS_CHATTERBOX_CFG_WEIGHT` | `0.5` | Fuerza del clasificador de Chatterbox. |
| `TTS_DEVICE` | `cpu` | `cpu` o `cuda`. |
| `TTS_TORCH_THREADS` | `2` | Hilos de torch. No cambiar sin medir. |
| `TTS_MAX_SCRIPT_CHARS` | `8000` | Límite de longitud de un guion. |
| `TTS_MAX_ATTEMPTS` | `2` | Reintentos por tarea. |
| `TTS_SLEEP_IDLE` | `5` | Espera entre polls de la cola (s). |
| `TTS_JOB_TIMEOUT_SECONDS` | `3600` | Antigüedad a partir de la cual una tarea en marcha se reencola. |
| `TTS_ALLOW_VOICE_UPLOAD` | `0` | `1` habilita el registro de muestras propias. |
| `TTS_BUILTIN_VOICES` | `es-base` | Voces del motor sin audio de referencia. |

#### Variables del backend `backend`

| Variable | Default | Descripción |
|----------|---------|-------------|
| `TTS_URL` | `http://tts:7000` | Dirección del servicio de voz. |
| `TTS_ENGINE` | `chatterbox` | Motor por defecto de los trabajos que no piden otro. |
| `TTS_API_KEY` | vacío | Debe coincidir con la del servicio. |
| `TTS_AUDIO_PATH` | `/app/data/tts_audio` | Volumen (solo lectura) con los audios generados. |
| `TTS_MAX_SCRIPT_CHARS` | `8000` | Límite de longitud de un guion. |

Detalle completo: [`docs/TTS.md`](docs/TTS.md) · [`specs/10-tts/`](specs/10-tts/spec.md)

### Perfiles Docker Compose

```bash
# Perfil por defecto (CPU)
docker compose up -d

# Con GPU (traductor NVIDIA)
docker compose --profile gpu up -d

# Con monitoreo (Prometheus + Grafana)
docker compose --profile monitoring up -d
```

---

## 🐛 Solución de Problemas

### El backend no inicia ("unhealthy")

El servidor tarda ~5 minutos en iniciar porque:
1. Crea índices en tablas grandes (1.4M+ entidades)
2. Refresca estadísticas de entidades

Espera al menos 5 minutos después de `docker compose up`.

### "No space left on device"

Aumenta `shm_size` en `docker-compose.yml`:

```yaml
db:
  shm_size: '512m'  # Antes: 256m
```

### Stats no aparecen

Limpia el caché de nginx:

```bash
docker compose restart nginx
```

### La búsqueda devuelve 0 resultados

El corpus tiene muy pocas noticias **recientes** (unas 451 en 24 h frente a
237 k en 7 días) y algunas fechas están corruptas. Si buscas con la ventana
de 24 h y el filtro de texto mínimo, es normal que no salga nada: amplía la
ventana a 7 días o quita el filtro.

Además, la caché de Redis guarda los resultados de búsqueda: si cambias los
filtros, la clave cambia también (los filtros forman parte de la clave).

### 502 al abrir la web

nginx resuelve el backend al arrancar. Si recreas el contenedor `backend`,
reinicia también nginx:

```bash
docker compose restart nginx
```

### El servicio de voz no arranca

```bash
docker compose logs tts --tail 50
```

Causas habituales:

- **Faltan los pesos**: ejecuta `make tts-models` (el servicio no descarga
  nada al arrancar, por diseño).
- **Falta el esquema**: `docker compose exec -T db psql -U rss -d rss < migrations/add_tts.sql`.
- **La tarea falla con `engine_not_prepared`**: faltan los pesos de Chatterbox
  o los checkpoints de OpenVoice.
- **La tarea falla con `script_not_approved`**: el guion no está aprobado.

### El noticiario tarda mucho

En CPU la locución va a ~1,5x el tiempo real: un noticiario de 5 noticias de
2 minutos son ~10 minutos de generación. Es normal y es asíncrono: la página
sondea el estado y monta el audio cuando llega la última pieza.

### Queries lentas

Verifica queries lentas en PostgreSQL:

```bash
docker exec rss2_db psql -U rss -d rss -c \
  "SELECT pid, now() - query_start AS duration, query FROM pg_stat_activity WHERE state = 'active' AND now() - query_start > '5 seconds';"
```

---

## 📜 Licencia

MIT License — ver [LICENSE](./LICENSE) para detalles.

---

## 🤝 Contribuir

1. Lee [specs/constitution.md](./specs/constitution.md) — reglas obligatorias
2. Crea spec antes de código — ver [specs/00-architecture/05-spec-driven-development.md](./specs/00-architecture/05-spec-driven-development.md)
3. Haz tests antes de commit
4. Asegura `make parity` pasa (textclean debe ser idéntico en backend/ y rss-ingestor-go/)

---

## 📚 Documentación Adicional

- [DEPLOY.md](./DEPLOY.md) — Guía de despliegue detallada
- [diagrama.md](./diagrama.md) — Diagramas de arquitectura
- [explication.md](./explication.md) — Explicación técnica profunda
- [docs/TTS.md](./docs/TTS.md) — Guía de operación de la narración por voz
- [specs/10-tts/](./specs/10-tts/spec.md) — Especificación del subsistema TTS
- [specs/](./specs/) — Especificaciones completas del sistema
- [docs/](./docs/) — Documentación adicional

---

<p align="center">
  <strong>RSS2</strong> — Tu ventana al mundo · Hecho con ❤️ y Go
</p>
