package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rss2/backend/internal/db"
	"github.com/rss2/backend/internal/models"
)

func CreateRemoteWorker(c *gin.Context) {
	var req struct {
		Name         string `json:"name" binding:"required"`
		Capabilities string `json:"capabilities"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "message": err.Error()})
		return
	}

	if req.Capabilities == "" {
		req.Capabilities = "cpu"
	}

	apiKey := generateAPIKey()

	ctx := c.Request.Context()
	var workerID int
	err := db.GetPool().QueryRow(ctx, `
		INSERT INTO remote_workers (name, api_key, capabilities, status, created_at)
		VALUES ($1, $2, $3, 'offline', NOW())
		RETURNING id
	`, req.Name, apiKey, req.Capabilities).Scan(&workerID)

	if err != nil {
		log.Printf("Error creating remote worker: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create worker"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":           workerID,
		"name":         req.Name,
		"api_key":      apiKey,
		"capabilities": req.Capabilities,
		"status":       "offline",
	})
}

func ListRemoteWorkers(c *gin.Context) {
	ctx := c.Request.Context()

	rows, err := db.GetPool().Query(ctx, `
		SELECT id, name, api_key, capabilities, status, last_seen, created_at
		FROM remote_workers
		ORDER BY created_at DESC
	`)
	if err != nil {
		log.Printf("Error listing remote workers: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list workers"})
		return
	}
	defer rows.Close()

	var workers []models.RemoteWorker
	for rows.Next() {
		var w models.RemoteWorker
		if err := rows.Scan(&w.ID, &w.Name, &w.APIKey, &w.Capabilities, &w.Status, &w.LastSeen, &w.CreatedAt); err != nil {
			log.Printf("Error scanning worker: %v", err)
			continue
		}
		workers = append(workers, w)
	}

	if workers == nil {
		workers = []models.RemoteWorker{}
	}

	c.JSON(http.StatusOK, workers)
}

func GetRemoteWorker(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	var w models.RemoteWorker
	err := db.GetPool().QueryRow(ctx, `
		SELECT id, name, api_key, capabilities, status, last_seen, created_at
		FROM remote_workers
		WHERE id = $1
	`, id).Scan(&w.ID, &w.Name, &w.APIKey, &w.Capabilities, &w.Status, &w.LastSeen, &w.CreatedAt)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Worker not found"})
		return
	}

	var stats struct {
		JobsCompleted int `json:"jobs_completed"`
		JobsPending   int `json:"jobs_pending"`
	}

	db.GetPool().QueryRow(ctx, `
		SELECT COUNT(*) FROM traducciones WHERE worker_id = $1 AND status = 'done'
	`, id).Scan(&stats.JobsCompleted)

	db.GetPool().QueryRow(ctx, `
		SELECT COUNT(*) FROM traducciones WHERE worker_id = $1 AND status = 'pending'
	`, id).Scan(&stats.JobsPending)

	c.JSON(http.StatusOK, gin.H{
		"worker": w,
		"stats":  stats,
	})
}

func DeleteRemoteWorker(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	result, err := db.GetPool().Exec(ctx, `
		DELETE FROM remote_workers WHERE id = $1
	`, id)

	if err != nil {
		log.Printf("Error deleting remote worker: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete worker"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Worker not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Worker deleted"})
}

func ToggleRemoteWorker(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	var currentStatus string
	err := db.GetPool().QueryRow(ctx, `SELECT status FROM remote_workers WHERE id = $1`, id).Scan(&currentStatus)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Worker not found"})
		return
	}

	newStatus := "disabled"
	if currentStatus == "disabled" {
		newStatus = "offline"
	}

	_, err = db.GetPool().Exec(ctx, `UPDATE remote_workers SET status = $1 WHERE id = $2`, newStatus, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update worker status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": newStatus})
}

func RegenerateAPIKey(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	newAPIKey := generateAPIKey()

	_, err := db.GetPool().Exec(ctx, `UPDATE remote_workers SET api_key = $1 WHERE id = $2`, newAPIKey, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to regenerate API key"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"api_key": newAPIKey})
}

func generateAPIKey() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}