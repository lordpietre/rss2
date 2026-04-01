package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/rss2/backend/internal/db"
	"github.com/rss2/backend/internal/models"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type WSWorker struct {
	conn         *websocket.Conn
	workerID     int
	capabilities string
	workerName   string
	lastHeartbeat time.Time
	mu           sync.Mutex
}

var (
	workers      = make(map[int]*WSWorker)
	workersByAPI = make(map[string]*WSWorker)
	workersMu    sync.RWMutex
)

func HandleWorkerWS(c *gin.Context) {
	apiKey := c.Query("api_key")
	if apiKey == "" {
		apiKey = c.GetHeader("X-API-Key")
	}

	if apiKey == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "API key required"})
		return
	}

	ctx := c.Request.Context()

	var workerID int
	var capabilities string
	err := db.GetPool().QueryRow(ctx, `
		SELECT id, capabilities FROM remote_workers 
		WHERE api_key = $1 AND status != 'disabled'
	`, apiKey).Scan(&workerID, &capabilities)

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	wsWorker := &WSWorker{
		conn:         conn,
		workerID:     workerID,
		capabilities: capabilities,
		lastHeartbeat: time.Now(),
	}

	workersMu.Lock()
	workers[workerID] = wsWorker
	workersByAPI[apiKey] = wsWorker
	workersMu.Unlock()

	db.GetPool().Exec(ctx, `
		UPDATE remote_workers SET status = 'online', last_seen = NOW() WHERE id = $1
	`, workerID)

	go heartbeatLoop(wsWorker)
	go writeLoop(wsWorker)
	readLoop(wsWorker, apiKey)
}

func readLoop(wsWorker *WSWorker, apiKey string) {
	defer func() {
		cleanupWorker(wsWorker, apiKey)
	}()

	for {
		var msg models.WSClientMessage
		err := wsWorker.conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("Worker %d read error: %v", wsWorker.workerID, err)
			break
		}

		switch msg.Type {
		case "register":
			wsWorker.mu.Lock()
			if msg.WorkerName != "" {
				wsWorker.workerName = msg.WorkerName
			}
			if msg.Capabilities != "" {
				wsWorker.capabilities = msg.Capabilities
			}
			wsWorker.mu.Unlock()

			sendWS(wsWorker, models.WSServerMessage{Type: "ack", Job: nil})

		case "heartbeat":
			wsWorker.mu.Lock()
			wsWorker.lastHeartbeat = time.Now()
			wsWorker.mu.Unlock()
			sendWS(wsWorker, models.WSServerMessage{Type: "ack", Job: nil})

		case "result":
			if msg.Result != nil {
				handleTranslationResult(wsWorker.workerID, msg.Result)
			}
		}
	}
}

func writeLoop(wsWorker *WSWorker) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := wsWorker.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func heartbeatLoop(wsWorker *WSWorker) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			wsWorker.mu.Lock()
			elapsed := time.Since(wsWorker.lastHeartbeat)
			wsWorker.mu.Unlock()

			if elapsed > 60*time.Second {
				log.Printf("Worker %d heartbeat timeout", wsWorker.workerID)
				wsWorker.conn.Close()
				return
			}

			sendWS(wsWorker, models.WSServerMessage{Type: "ping", Job: nil})
		}
	}
}

func sendWS(wsWorker *WSWorker, msg models.WSServerMessage) {
	data, _ := json.Marshal(msg)
	wsWorker.conn.WriteMessage(websocket.TextMessage, data)
}

func cleanupWorker(wsWorker *WSWorker, apiKey string) {
	workersMu.Lock()
	delete(workers, wsWorker.workerID)
	delete(workersByAPI, apiKey)
	workersMu.Unlock()

	ctx := context.Background()
	db.GetPool().Exec(ctx, `
		UPDATE remote_workers SET status = 'offline', last_seen = NOW() WHERE id = $1
	`, wsWorker.workerID)

	wsWorker.conn.Close()

	db.GetPool().Exec(ctx, `
		UPDATE traducciones 
		SET status = 'pending', worker_id = NULL, assigned_at = NULL
		WHERE worker_id = $1 AND status = 'assigned'
	`, wsWorker.workerID)

	log.Printf("Worker %d disconnected", wsWorker.workerID)
}

func handleTranslationResult(workerID int, result *models.TranslationResult) {
	ctx := context.Background()

	if result.Error != "" {
		db.GetPool().Exec(ctx, `
			UPDATE traducciones 
			SET status = 'error', worker_id = NULL, assigned_at = NULL
			WHERE id = $1
		`, result.JobID)
		log.Printf("Job %d failed: %s", result.JobID, result.Error)
		return
	}

	_, err := db.GetPool().Exec(ctx, `
		UPDATE traducciones 
		SET titulo_trad = $1, resumen_trad = $2, status = 'done', 
		    worker_id = $3, assigned_at = NULL
		WHERE id = $4
	`, result.TitleTr, result.SummaryTr, workerID, result.JobID)

	if err != nil {
		log.Printf("Error updating translation result: %v", err)
	}

	db.GetPool().Exec(ctx, `
		UPDATE remote_workers SET last_seen = NOW() WHERE id = $1
	`, workerID)

	log.Printf("Job %d completed by worker %d", result.JobID, workerID)
}

func AssignJobToWorker(workerID int) *models.TranslationJob {
	ctx := context.Background()

	workersMu.RLock()
	wsWorker, exists := workers[workerID]
	workersMu.RUnlock()

	if !exists {
		return nil
	}

	wsWorker.mu.Lock()
	workerCapabilities := wsWorker.capabilities
	wsWorker.mu.Unlock()

	_ = workerCapabilities

	var job models.TranslationJob
	err := db.GetPool().QueryRow(ctx, `
		SELECT t.id, t.noticia_id, t.lang_from, t.lang_to, n.titulo, n.resumen
		FROM traducciones t
		JOIN noticias n ON n.id = t.noticia_id
		WHERE t.status = 'pending' 
		  AND (t.worker_id IS NULL OR t.status = 'assigned' AND t.assigned_at < NOW() - INTERVAL '5 minutes')
		  AND t.lang_to = 'es'
		  AND (t.titulo_trad IS NULL OR t.resumen_trad IS NULL)
		ORDER BY n.fecha DESC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`).Scan(&job.ID, &job.NewsID, &job.LangFrom, &job.LangTo, &job.Title, &job.Summary)

	if err != nil {
		return nil
	}

	db.GetPool().Exec(ctx, `
		UPDATE traducciones 
		SET status = 'assigned', worker_id = $1, assigned_at = NOW()
		WHERE id = $2
	`, workerID, job.ID)

	sendWS(wsWorker, models.WSServerMessage{
		Type: "job",
		Job:  &job,
	})

	return &job
}

func StartJobAssigner() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				workersMu.RLock()
				for workerID := range workers {
					workersMu.RUnlock()
					AssignJobToWorker(workerID)
					workersMu.RLock()
				}
				workersMu.RUnlock()

				ctx := context.Background()
				db.GetPool().Exec(ctx, `
					UPDATE traducciones 
					SET status = 'pending', worker_id = NULL, assigned_at = NULL
					WHERE status = 'assigned' 
					  AND assigned_at < NOW() - INTERVAL '10 minutes'
				`)
			}
		}
	}()
}