# Plan: Sistema de Workers Remotos para Traducción

## Visión General

Sistema que permite conectar workers de traducción desde equipos externos (con GPU) al servidor central mediante WebSockets, eliminando la necesidad de exponer el servidor a internet y permitiendo procesamiento distribuido.

---

## Arquitectura del Sistema

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              SERVidor VPS                                    │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐   │
│  │  API REST   │  │   WebSocket │  │  Scheduler  │  │   PostgreSQL    │   │
│  │   (Go)      │◄─┤   Server    │  │   (Go)      │  │   (Cola Jobs)   │   │
│  │             │  │   (Go)      │  │             │  │                 │   │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────────┘   │
│         │                │                │                                 │
│         └────────────────┼────────────────┘                                 │
│                          │                                                  │
│                   PUERTO 80/443 (ya expuesto)                              │
└─────────────────────────────────────────────────────────────────────────────┘
                                    ▲
                                    │ WebSocket (conexión saliente)
                                    │ API Key autenticación
                                    │
┌─────────────────────────────────────────────────────────────────────────────┐
│                        EQUIPO LOCAL CON GPU                                  │
│  ┌─────────────────────────────────────────────────────────────────┐       │
│  │                    Worker Client (Go)                           │       │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │       │
│  │  │  WS Client  │  │   Lógica    │  │  CTranslate2 (subproc)  │  │       │
│  │  │   (Go)      │  │   (Go)      │  │  + Modelo NLLB-1.3B    │  │       │
│  │  └─────────────┘  └─────────────┘  └─────────────────────────┘  │       │
│  └─────────────────────────────────────────────────────────────────┘       │
│                                                                              │
│  Conexión saliente (no requiere puertos abiertos)                          │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Flujo de Datos

1. **Scheduler (servidor)** crea jobs en `traducciones` con status `pending`
2. **Worker remoto** se conecta por WS, envía capabilities (`gpu`/`cpu`)
3. **Servidor** asigna job disponible que coincida con capabilities → status `assigned`
4. **Worker** recibe job → traduce con NLLB-1.3B → envía resultado por WS
5. **Servidor** actualiza `traducciones` con resultado → status `completed`

---

## Análisis de la Aplicación Actual

### Componentes Relevantes Identificados

| Componente | Ubicación | Función |
|------------|-----------|---------|
| Handlers admin | `backend/internal/handlers/admin.go` | API workers locales (Docker) |
| Modelos | `backend/internal/models/models.go` | Estructuras de datos |
| Tabla traducciones | `init-db/00-complete-schema.sql` | Cola de traducciones |
| Worker Python | `workers/ctranslator_worker.py` | Traducción actual |
| Scheduler Python | `workers/translation_scheduler.py` | Creador de jobs |
| Frontend Admin | `frontend/src/pages/AdminWorkers.tsx` | Panel de control |

### Estados Actuales de traducción

La tabla `traducciones` tiene:
- `pending` - disponible
- `done` - completado
- `error` - fallido

**Problema identificado:** No hay estado `assigned` → varios workers pueden tomar el mismo job.

### Query Actual del Worker

```python
# workers/ctranslator_worker.py:361-378
SELECT ... FROM traducciones t
WHERE t.lang_to = %s 
  AND (t.titulo_trad IS NULL OR t.resumen_trad IS NULL)
  AND (t.locked_at IS NULL OR t.locked_at < NOW() - INTERVAL '10 minutes')
ORDER BY n.fecha DESC
LIMIT %s
FOR UPDATE SKIP LOCKED
```

Usa `locked_at` como mecanismo de locking, pero no actualiza un `status`.

---

## Decisiones de Diseño

### 1. Conexión: WebSocket

- Bidireccional, tiempo real
- El worker solo necesita conexión saliente (como navegación web)
- No requiere exponer puertos adicionales en el VPS

### 2. Autenticación: API Key

- El servidor genera una API key única por worker
- El worker la envía en cada conexión WS o como header
- Más simple que usuario/password

### 3. Cola de Trabajos: PostgreSQL

- Persistencia estable (no Redis, para simplificar)
- El servidor escribe resultados (más control que acceso directo del worker)
- Si el worker falla, el servidor puede reasignar el job

### 4. ML en Worker: CTranslate2

- El worker llama a CTranslate2 como subproceso
- Mismo mecanismo que el worker actual (Docker) pero standalone
- No requiere bindings Go nativos

---

## Problemas Potenciales y Soluciones

| Problema | Severity | Solución |
|----------|----------|----------|
| Worker sin internet | N/A | No es problema: el worker inicia conexión saliente |
| Worker se desconecta durante procesamiento | Alta | Timeout en servidor → job vuelve a `pending` |
| Job asignado a dos workers | Alta | Nuevo estado `assigned` + columna `worker_id` |
| Worker escribe directamente a BD | Media | Enviar resultados por WS → servidor controla escritura |
| Modelo NLLB no disponible localmente | Baja | Descargar modelo convertido previamente |

---

## Plan de Implementación

### Fase 1: Base de Datos

**Archivo:** `init-db/35-remote-workers.sql`

```sql
-- Tabla de workers remotos
CREATE TABLE IF NOT EXISTS remote_workers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    api_key VARCHAR(64) UNIQUE NOT NULL,
    capabilities VARCHAR(50) DEFAULT 'cpu',  -- 'cpu' o 'gpu'
    status VARCHAR(20) DEFAULT 'offline',     -- 'online', 'offline', 'disabled'
    last_seen TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Añadir columnas a traducciones para workers remotos
ALTER TABLE traducciones ADD COLUMN IF NOT EXISTS worker_id INTEGER REFERENCES remote_workers(id);
ALTER TABLE traducciones ADD COLUMN IF NOT EXISTS assigned_at TIMESTAMP;

-- Nuevo índice para buscar jobs disponibles por capabilities
CREATE INDEX IF NOT EXISTS idx_traducciones_pending_worker 
    ON traducciones(lang_to, status, worker_id) 
    WHERE status = 'pending' AND worker_id IS NULL;

-- Índice para jobs asignados (para cleanup de timeout)
CREATE INDEX IF NOT EXISTS idx_traducciones_assigned_timeout 
    ON traducciones(assigned_at) 
    WHERE status = 'assigned' AND assigned_at IS NOT NULL;
```

### Fase 2: Modelos (Backend Go)

**Nuevo archivo:** `backend/internal/models/worker.go`

```go
package models

import "time"

// Worker remoto registrado
type RemoteWorker struct {
    ID           int       `json:"id"`
    Name         string    `json:"name"`
    APIKey       string    `json:"api_key,omitempty"`  // Solo al crear
    Capabilities string    `json:"capabilities"`      // 'cpu' o 'gpu'
    Status       string    `json:"status"`            // 'online', 'offline', 'disabled'
    LastSeen     time.Time `json:"last_seen"`
    CreatedAt    time.Time `json:"created_at"`
}

// Job de traducción para worker remoto
type TranslationJob struct {
    ID        int64  `json:"id"`
    NewsID    int64  `json:"noticia_id"`
    LangFrom  string `json:"lang_from"`
    LangTo    string `json:"lang_to"`
    Title     string `json:"title"`
    Summary   string `json:"summary"`
}

// Resultado de traducción enviado por worker
type TranslationResult struct {
    JobID       int64  `json:"job_id"`
    TitleTr    string `json:"title_trad"`
    SummaryTr   string `json:"resumen_trad"`
    Error       string `json:"error,omitempty"`
}

// Mensaje WebSocket: cliente → servidor
type WSClientMessage struct {
    Type        string `json:"type"`         // "register", "heartbeat", "result"
    Capabilities string `json:"capabilities,omitempty"`
    APIKey      string `json:"api_key,omitempty"`
    WorkerID    int    `json:"worker_id,omitempty"`
    Result      *TranslationResult `json:"result,omitempty"`
}

// Mensaje WebSocket: servidor → cliente
type WSServerMessage struct {
    Type string          `json:"type"`  // "job", "ack", "error"
    Job  *TranslationJob `json:"job,omitempty"`
}
```

### Fase 3: Handlers REST (Backend Go)

**Nuevo archivo:** `backend/internal/handlers/remote_worker.go`

#### Endpoints

| Método | Endpoint | Descripción |
|--------|----------|-------------|
| POST | `/api/admin/workers/remote` | Crear worker (recibe nombre, capabilities) |
| GET | `/api/admin/workers/remote` | Listar todos los workers |
| GET | `/api/admin/workers/remote/:id` | Ver detalle de un worker |
| DELETE | `/api/admin/workers/remote/:id` | Eliminar worker |
| PATCH | `/api/admin/workers/remote/:id/toggle` | Habilitar/deshabilitar |

#### Funciones principales

1. **CreateRemoteWorker** - Genera API key aleatoria (64 chars), guarda en BD
2. **ListRemoteWorkers** - Retorna todos con jobs counts
3. **GetRemoteWorker** - Retorna detalle + estadísticas
4. **DeleteRemoteWorker** - Soft delete (marcar como disabled)
5. **ToggleRemoteWorker** - Enable/disable sin borrar

### Fase 4: WebSocket Handler (Backend Go)

**Nuevo archivo:** `backend/internal/handlers/worker_ws.go`

```go
// Estructuras requeridas
type WSWorker struct {
    conn         *websocket.Conn
    workerID     int
    capabilities string
    lastHeartbeat time.Time
}

// Conexiónmap[int]*WSWorker  // workerID → conexión
var workers = make(map[int]*WSWorker)
var workersByConn = make(map[*websocket.Conn]*WSWorker)

// Handler principal
func HandleWorkerWS(c *gin.Context) {
    // 1. Verificar API key en query string o header
    apiKey := c.Query("api_key")
    if apiKey == "" {
        apiKey = c.GetHeader("X-API-Key")
    }
    
    // 2. Buscar worker por API key
    workerID, err := validateWorkerAPIKey(apiKey)
    if err != nil {
        c.JSON(401, gin.H{"error": "Invalid API key"})
        return
    }
    
    // 3. Upgrade a WebSocket
    upgrader := websocket.Upgrader{
        CheckOrigin: func(r *http.Request) bool { return true },
    }
    conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
    if err != nil {
        return
    }
    
    // 4. Registrar worker como online
    wsWorker := &WSWorker{
        conn:         conn,
        workerID:     workerID,
        capabilities: getWorkerCapabilities(workerID),
        lastHeartbeat: time.Now(),
    }
    workers[workerID] = wsWorker
    workersByConn[conn] = wsWorker
    
    // 5. Update status en BD
    updateWorkerStatus(workerID, "online")
    
    // 6. Loop de lectura
    go handleWSRead(wsWorker)
}

// Loop de lectura de mensajes del worker
func handleWSRead(wsWorker *WSWorker) {
    for {
        var msg WSClientMessage
        err := wsWorker.conn.ReadJSON(&msg)
        if err != nil {
            // Worker desconectado
            cleanupWorker(wsWorker)
            return
        }
        
        switch msg.Type {
        case "register":
            // Registro inicial (capabilities)
            wsWorker.capabilities = msg.Capabilities
            sendAck(wsWorker, "registered")
            
        case "heartbeat":
            wsWorker.lastHeartbeat = time.Now()
            sendAck(wsWorker, "ok")
            
        case "result":
            // Worker completó un job
            handleTranslationResult(wsWorker.workerID, msg.Result)
            sendAck(wsWorker, "received")
        }
    }
}

// Asignar job disponible al worker
func assignJobToWorker(workerID int) *TranslationJob {
    caps := getWorkerCapabilities(workerID)
    
    // Buscar job pending que coincida con capabilities del worker
    // Por ahora: cualquier job pending funciona (el worker decide qué procesar)
    job := getNextPendingJob(caps)
    if job == nil {
        return nil
    }
    
    // Asignar: UPDATE status='assigned', worker_id=workerID, assigned_at=NOW()
    assignJob(job.ID, workerID)
    
    return job
}
```

### Fase 5: Scheduler (Migración Python → Go)

**Nuevo archivo:** `backend/internal/workers/translation_scheduler.go`

```go
package workers

func StartTranslationScheduler() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            scheduleTranslations()
        }
    }
}

func scheduleTranslations() {
    // Para cada idioma destino (es)
    targetLangs := []string{"es"}
    
    for _, lang := range targetLangs {
        // INSERT jobs para noticias sin traducción
        _, err := db.Exec(`
            INSERT INTO traducciones (noticia_id, lang_from, lang_to, status, created_at)
            SELECT n.id, n.lang, $1, 'pending', NOW()
            FROM noticias n
            WHERE n.lang IS NOT NULL 
              AND TRIM(n.lang) != ''
              AND n.lang != $1
              AND NOT EXISTS (
                  SELECT 1 FROM traducciones t 
                  WHERE t.noticia_id = n.id AND t.lang_to = $1
              )
            ORDER BY n.fecha DESC
            LIMIT $2
            ON CONFLICT (noticia_id, lang_to) DO NOTHING
        `, lang, 2000)
    }
}

// Cleanup de jobs huérfanos (assigned hace demasiado tiempo)
func cleanupStaleAssignments() {
    db.Exec(`
        UPDATE traducciones 
        SET status = 'pending', worker_id = NULL, assigned_at = NULL
        WHERE status = 'assigned' 
          AND assigned_at < NOW() - INTERVAL '5 minutes'
    `)
}
```

### Fase 6: Cliente Worker Remoto (Go)

**Nuevo directorio:** `worker-client/`

```
worker-client/
├── cmd/
│   └── main.go           # Entry point
├── internal/
│   ├── config/
│   │   └── config.go     # Flags y configuración
│   ├── ws/
│   │   └── client.go     # WebSocket client
│   ├── translator/
│   │   └── ctranslate2.go  # Llamada a CTranslate2
│   └── store/
│       └── local.go      # Persistencia local (SQLite)
├── go.mod
└── README.md
```

#### cmd/main.go

```go
func main() {
    // Flags
    serverURL := flag.String("server", "ws://localhost:8080/ws/worker", "WebSocket server URL")
    apiKey := flag.String("api-key", "", "API key for authentication")
    device := flag.String("device", "cuda", "Device: cuda or cpu")
    modelPath := flag.String("model-path", "./models/nllb-ct2", "CTranslate2 model path")
    batchSize := flag.Int("batch", 8, "Translation batch size")
    flag.Parse()
    
    // Validar
    if *apiKey == "" {
        log.Fatal("API key required")
    }
    
    // Inicializar
    cfg := config.Config{
        ServerURL:   *serverURL,
        APIKey:      *apiKey,
        Device:      *device,
        ModelPath:   *modelPath,
        BatchSize:   *batchSize,
    }
    
    // Conectar y loop
    client := ws.NewClient(&cfg)
    client.ConnectAndRun()
}
```

#### internal/ws/client.go

```go
func (c *Client) ConnectAndRun() {
    for {
        err := c.connect()
        if err != nil {
            log.Printf("Connection failed: %v", err)
            time.Sleep(5 * time.Second)
            continue
        }
        
        // Loop de mensajes
        c.readLoop()
    }
}

func (c *Client) handleMessage(msg []byte) {
    var serverMsg WSServerMessage
    if err := json.Unmarshal(msg, &serverMsg); err != nil {
        return
    }
    
    switch serverMsg.Type {
    case "job":
        c.processJob(serverMsg.Job)
    case "ping":
        c.sendHeartbeat()
    }
}

func (c *Client) processJob(job *TranslationJob) {
    // 1. Traducir título
    titleTr := translate(job.Title, job.LangFrom, job.LangTo)
    
    // 2. Traducir resumen (chunks si es largo)
    summaryTr := translateLongText(job.Summary, job.LangFrom, job.LangTo)
    
    // 3. Enviar resultado
    c.SendResult(job.ID, titleTr, summaryTr, "")
}
```

#### internal/translator/ctranslate2.go

```go
func Translate(text, srcLang, tgtLang string) (string, error) {
    // Mapear códigos de idioma
    srcCode := langMap[srcLang]
    tgtCode := langMap[tgtLang]
    
    // Construir comando
    cmd := exec.Command(
        "ct2-transformers-converter",  // O el binario de ctranslate2
        "--source", text,
        "--src_lang", srcCode,
        "--tgt_lang", tgtCode,
    )
    
    output, err := cmd.CombinedOutput()
    if err != nil {
        return "", err
    }
    
    return string(output), nil
}
```

**Nota:** La forma más estable es usar el binario `ctranslate2` directamente como wrapper del modelo convertido. Ver documentación de CTranslate2 para la interfaz CLI.

### Fase 7: Frontend (Panel Admin)

**Modificar:** `frontend/src/pages/AdminWorkers.tsx`

Añadir nueva sección "Workers Remotos":

```tsx
// Estados adicionales
const [remoteWorkers, setRemoteWorkers] = useState<RemoteWorker[]>([])
const [showAddForm, setShowAddForm] = useState(false)

// Fetch
const fetchRemoteWorkers = async () => {
    const res = await api.get('/admin/workers/remote')
    setRemoteWorkers(res.data)
}

// UI: Añadir después de la sección de workers locales
<div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
    <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
        <Wifi className="h-5 w-5" />
        Workers Remotos
    </h2>
    
    {/* Botón añadir */}
    <button onClick={() => setShowAddForm(true)} className="btn-primary mb-4">
        Añadir Worker
    </button>
    
    {/* Lista de workers */}
    <div className="space-y-3">
        {remoteWorkers.map(worker => (
            <div key={worker.id} className="flex items-center justify-between p-4 bg-gray-50 rounded-lg">
                <div>
                    <span className="font-medium">{worker.name}</span>
                    <span className={`ml-2 px-2 py-0.5 text-xs rounded ${
                        worker.status === 'online' ? 'bg-green-100' : 'bg-gray-100'
                    }`}>
                        {worker.status}
                    </span>
                </div>
                <div className="flex gap-2">
                    <button className="btn-secondary">Ver detalles</button>
                    <button className="btn-danger">Eliminar</button>
                </div>
            </div>
        ))}
    </div>
</div>
```

---

## Archivos a Crear/Modificar

### Backend (Go)

| Archivo | Acción | Descripción |
|---------|--------|-------------|
| `init-db/35-remote-workers.sql` | Crear | Tabla remote_workers + columnas |
| `backend/internal/models/worker.go` | Crear | Modelos RemoteWorker, TranslationJob |
| `backend/internal/handlers/remote_worker.go` | Crear | CRUD REST API |
| `backend/internal/handlers/worker_ws.go` | Crear | WebSocket handler |
| `backend/internal/workers/translation_scheduler.go` | Crear | Scheduler migrado de Python |
| `backend/cmd/server/main.go` | Modificar | Añadir rutas + iniciar scheduler |
| `backend/go.mod` | Modificar | Añadir dependencia `github.com/gorilla/websocket` |

### Frontend (React)

| Archivo | Acción | Descripción |
|---------|--------|-------------|
| `frontend/src/pages/AdminWorkers.tsx` | Modificar | Añadir sección workers remotos |

### Cliente Worker

| Archivo | Acción | Descripción |
|---------|--------|-------------|
| `worker-client/cmd/main.go` | Crear | CLI entry point |
| `worker-client/internal/config/config.go` | Crear | Configuración |
| `worker-client/internal/ws/client.go` | Crear | WebSocket client |
| `worker-client/internal/translator/ctranslate2.go` | Crear | Wrapper CTranslate2 |
| `worker-client/go.mod` | Crear | Módulos Go |
| `worker-client/README.md` | Crear | Instrucciones de uso |

---

## Consideraciones de Seguridad

1. **API Key**: Generar con `crypto/rand` - 64 caracteres hex
2. **Rate limiting**: En endpoints de creación de workers
3. **Validación**: Verificar que el worker tiene capabilities válidas
4. **Timeout**: Jobs asignados sin respuesta en 5 min vuelven a cola
5. **Conexión**: El servidor debe verificar origen del WebSocket

---

## Testing Plan

1. **Unidad**: Tests de handlers (mock de BD)
2. **Integración WS**: Probar conexión, registro, heartbeat, resultado
3. **E2E**:
   - Iniciar cliente worker local
   - Crear job pending manualmente
   - Verificar que worker recibe job
   - Verificar resultado en BD

---

## Notas de Implementación

- Mantener compatibilidad con workers Docker existentes (CPU/GPU locales)
- Los workers remotos serán una opción adicional, no reemplazo
- El scheduler puede coexistir (Go + Python) durante transición
- Considerar métricas: jobs procesadors por worker, tiempo promedio