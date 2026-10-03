package handlers

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"github.com/gin-gonic/gin"
	"github.com/rss2/backend/internal/config"
	"github.com/rss2/backend/internal/db"
	"github.com/rss2/backend/internal/models"
)

func CreateAlias(c *gin.Context) {
	var req models.EntityAliasRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "message": err.Error()})
		return
	}

	ctx := c.Request.Context()
	tx, err := db.GetPool().Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction", "message": err.Error()})
		return
	}
	defer tx.Rollback(ctx)

	// Extract last name from entity value (last word after splitting by space)
	lastName := extractLastName(req.CanonicalName)

	// 1. Ensure the canonical tag exists in tags table
	var canonicalTagId int
	err = tx.QueryRow(ctx, `
		INSERT INTO tags (valor, tipo, apellido) VALUES ($1, $2, $3)
		ON CONFLICT (valor, tipo) DO UPDATE SET valor = EXCLUDED.valor, apellido = EXCLUDED.apellido
		RETURNING id`, req.CanonicalName, req.Tipo, lastName).Scan(&canonicalTagId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to ensure canonical tag", "message": err.Error()})
		return
	}

	for _, alias := range req.Aliases {
		alias = strings.TrimSpace(alias)
		if alias == "" {
			continue
		}

		// Insert the alias mapping into entity_aliases
		_, err = tx.Exec(ctx, `
			INSERT INTO entity_aliases (canonical_name, alias, tipo)
			VALUES ($1, $2, $3)
			ON CONFLICT (alias, tipo) DO UPDATE SET canonical_name = EXCLUDED.canonical_name`,
			req.CanonicalName, alias, req.Tipo)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to insert alias", "message": err.Error()})
			return
		}

		// 2. Check if the original alias string actually exists as a tag
		var aliasTagId int
		err = tx.QueryRow(ctx, "SELECT id FROM tags WHERE valor = $1 AND tipo = $2", alias, req.Tipo).Scan(&aliasTagId)
		if err == nil && aliasTagId != 0 && aliasTagId != canonicalTagId {
			// 3. Move all mentions in tags_noticia to the canonical tag id safely
			_, err = tx.Exec(ctx, `
				UPDATE tags_noticia 
				SET tag_id = $1 
				WHERE tag_id = $2 AND NOT EXISTS (
					SELECT 1 FROM tags_noticia tn2 
					WHERE tn2.tag_id = $1 AND tn2.noticia_id = tags_noticia.noticia_id AND tn2.traduccion_id = tags_noticia.traduccion_id
				)
			`, canonicalTagId, aliasTagId)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reassign news mentions safely", "message": err.Error()})
				return
			}

			// Delete any remaining orphaned mentions of the alias that couldn't be merged (duplicates)
			_, err = tx.Exec(ctx, "DELETE FROM tags_noticia WHERE tag_id = $1", aliasTagId)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete orphaned mentions", "message": err.Error()})
				return
			}

			// 4. Delete the original alias tag
			_, err = tx.Exec(ctx, "DELETE FROM tags WHERE id = $1", aliasTagId)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete old tag", "message": err.Error()})
				return
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction", "message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":        "Aliases created and metrics merged successfully",
		"canonical_name": req.CanonicalName,
		"aliases_added":  req.Aliases,
		"tipo":           req.Tipo,
	})
}

func extractLastName(valor string) string {
	parts := strings.Fields(valor)
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func ListAliases(c *gin.Context) {
	tipo := strings.TrimSpace(c.Query("tipo"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "100"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 500 {
		perPage = 100
	}

	var rows interface {
	}
	_ = rows

	query := "SELECT id, alias, canonical_name, tipo, created_at FROM entity_aliases"
	args := []interface{}{}
	if tipo != "" {
		query += " WHERE tipo = $1"
		args = append(args, tipo)
	}
	query += " ORDER BY created_at DESC LIMIT $" + strconv.Itoa(len(args)+1) + " OFFSET $" + strconv.Itoa(len(args)+2)
	args = append(args, perPage, (page-1)*perPage)

	qrows, err := db.GetPool().Query(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list aliases", "message": err.Error()})
		return
	}
	defer qrows.Close()

	aliases := []models.EntityAlias{}
	for qrows.Next() {
		var a models.EntityAlias
		if err := qrows.Scan(&a.ID, &a.Alias, &a.CanonicalName, &a.Tipo, &a.CreatedAt); err != nil {
			log.Printf("ListAliases: scan error: %v", err)
			continue
		}
		aliases = append(aliases, a)
	}
	if aliases == nil {
		aliases = []models.EntityAlias{}
	}
	c.JSON(http.StatusOK, gin.H{"aliases": aliases, "page": page, "per_page": perPage, "total": len(aliases)})
}

func UpdateAlias(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid alias ID"})
		return
	}
	var req struct {
		Alias         string `json:"alias" binding:"required"`
		CanonicalName string `json:"canonical_name" binding:"required"`
		Tipo          string `json:"tipo" binding:"required,oneof=persona organizacion lugar tema"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "message": err.Error()})
		return
	}
	res, err := db.GetPool().Exec(c.Request.Context(),
		"UPDATE entity_aliases SET alias = $1, canonical_name = $2, tipo = $3 WHERE id = $4",
		strings.TrimSpace(req.Alias), strings.TrimSpace(req.CanonicalName), req.Tipo, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update alias", "message": err.Error()})
		return
	}
	if res.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Alias not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Alias updated", "id": id})
}

func DeleteAlias(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid alias ID"})
		return
	}
	res, err := db.GetPool().Exec(c.Request.Context(), "DELETE FROM entity_aliases WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete alias", "message": err.Error()})
		return
	}
	if res.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Alias not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Alias deleted", "id": id})
}

func GetIngestStats(c *gin.Context) {
	ctx := c.Request.Context()
	var total, activos, fallidos, noticiasTotal, noticias7d, feedsConNoticias7d int
	db.GetPool().QueryRow(ctx, "SELECT COUNT(*) FROM feeds").Scan(&total)
	db.GetPool().QueryRow(ctx, "SELECT COUNT(*) FROM feeds WHERE activo").Scan(&activos)
	db.GetPool().QueryRow(ctx, "SELECT COUNT(*) FROM feeds WHERE COALESCE(fallos,0) > 0").Scan(&fallidos)
	db.GetPool().QueryRow(ctx, "SELECT COUNT(*) FROM noticias").Scan(&noticiasTotal)
	db.GetPool().QueryRow(ctx, "SELECT COUNT(*) FROM noticias WHERE fecha >= NOW() - INTERVAL '7 days'").Scan(&noticias7d)
	db.GetPool().QueryRow(ctx, `SELECT COUNT(DISTINCT f.id) FROM feeds f JOIN noticias n ON n.fuente_nombre = f.nombre WHERE n.fecha >= NOW() - INTERVAL '7 days' AND f.activo`).Scan(&feedsConNoticias7d)
	coverage := 0.0
	if activos > 0 {
		coverage = math.Round(float64(feedsConNoticias7d)/float64(activos)*10000) / 100
	}
	c.JSON(http.StatusOK, gin.H{
		"feeds_total":          total,
		"feeds_activos":        activos,
		"feeds_con_fallos":     fallidos,
		"noticias_total":       noticiasTotal,
		"noticias_ultimos_7d":  noticias7d,
		"feeds_con_noticias_7d": feedsConNoticias7d,
		"cobertura_7d_pct":     coverage,
	})
}

func ExportAliases(c *gin.Context) {	rows, err := db.GetPool().Query(c.Request.Context(),
		"SELECT alias, canonical_name, tipo FROM entity_aliases ORDER BY tipo, canonical_name")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get aliases", "message": err.Error()})
		return
	}
	defer rows.Close()

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=aliases.csv")
	c.Header("Cache-Control", "no-cache")

	writer := csv.NewWriter(c.Writer)
	writer.Write([]string{"alias", "canonical_name", "tipo"})

	for rows.Next() {
		var alias, canonical, tipo string
		rows.Scan(&alias, &canonical, &tipo)
		writer.Write([]string{alias, canonical, tipo})
	}
	writer.Flush()
}

func ImportAliases(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file"})
		return
	}
	defer src.Close()

	reader := csv.NewReader(src)
	records, err := reader.ReadAll()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse CSV", "message": err.Error()})
		return
	}

	if len(records) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "CSV file is empty or has no data rows"})
		return
	}

	ctx := context.Background()
	tx, err := db.GetPool().Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback(ctx)

	inserted := 0
	skipped := 0

	for i, record := range records[1:] {
		if len(record) < 3 {
			skipped++
			continue
		}

		alias := strings.TrimSpace(record[0])
		canonical := strings.TrimSpace(record[1])
		tipo := strings.TrimSpace(record[2])

		if alias == "" || canonical == "" {
			skipped++
			continue
		}

		_, err = tx.Exec(ctx,
			"INSERT INTO entity_aliases (alias, canonical_name, tipo) VALUES ($1, $2, $3) ON CONFLICT (alias, tipo) DO UPDATE SET canonical_name = $2",
			alias, canonical, tipo)
		if err != nil {
			fmt.Printf("Error inserting row %d: %v\n", i+1, err)
			skipped++
			continue
		}
		inserted++
	}

	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Import completed",
		"inserted": inserted,
		"skipped":  skipped,
	})
}

func GetAdminStats(c *gin.Context) {
	var totalUsers, totalAliases int

	db.GetPool().QueryRow(c.Request.Context(), "SELECT COUNT(*) FROM users").Scan(&totalUsers)
	db.GetPool().QueryRow(c.Request.Context(), "SELECT COUNT(*) FROM entity_aliases").Scan(&totalAliases)

	c.JSON(http.StatusOK, gin.H{
		"total_users":   totalUsers,
		"total_aliases": totalAliases,
	})
}

func GetUsers(c *gin.Context) {
	rows, err := db.GetPool().Query(c.Request.Context(), `
		SELECT id, email, username, is_admin, created_at, updated_at 
		FROM users ORDER BY created_at DESC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get users", "message": err.Error()})
		return
	}
	defer rows.Close()

	type UserRow struct {
		ID        int64  `json:"id"`
		Email     string `json:"email"`
		Username  string `json:"username"`
		IsAdmin   bool   `json:"is_admin"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}

	var users []UserRow
	for rows.Next() {
		var u UserRow
		if err := rows.Scan(&u.ID, &u.Email, &u.Username, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt); err != nil {
			continue
		}
		users = append(users, u)
	}

	if users == nil {
		users = []UserRow{}
	}

	c.JSON(http.StatusOK, gin.H{"users": users, "total": len(users)})
}

func PromoteUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	result, err := db.GetPool().Exec(c.Request.Context(), "UPDATE users SET is_admin = true WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to promote user", "message": err.Error()})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User promoted to admin"})
}

func DemoteUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	result, err := db.GetPool().Exec(c.Request.Context(), "UPDATE users SET is_admin = false WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to demote user", "message": err.Error()})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User demoted from admin"})
}

func ResetDatabase(c *gin.Context) {
	ctx := c.Request.Context()

	tables := []string{
		"noticias",
		"feeds",
		"traducciones",
		"tags_noticia",
		"tags",
		"entity_aliases",
		"favoritos",
		"videos",
		"video_parrillas",
		"eventos",
		"search_history",
	}

	tx, err := db.GetPool().Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback(ctx)

	for _, table := range tables {
		_, err = tx.Exec(ctx, "DELETE FROM "+table)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete from " + table, "message": err.Error()})
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "Database reset successfully. All data has been deleted.",
		"tables_cleared": tables,
	})
}

type WorkerConfig struct {
	Type    string `json:"type"`
	Workers int    `json:"workers"`
	Status  string `json:"status"`
}

func GetWorkerStatus(c *gin.Context) {
	var translatorType, translatorWorkers, translatorStatus string

	err := db.GetPool().QueryRow(c.Request.Context(), "SELECT value FROM config WHERE key = 'translator_type'").Scan(&translatorType)
	if err != nil {
		translatorType = "cpu"
	}

	err = db.GetPool().QueryRow(c.Request.Context(), "SELECT value FROM config WHERE key = 'translator_workers'").Scan(&translatorWorkers)
	if err != nil {
		translatorWorkers = "2"
	}

	err = db.GetPool().QueryRow(c.Request.Context(), "SELECT value FROM config WHERE key = 'translator_status'").Scan(&translatorStatus)
	if err != nil {
		translatorStatus = "stopped"
	}

	workers, _ := strconv.Atoi(translatorWorkers)

	// Verificar si los contenedores están corriendo
	runningCount := 0
	if translatorStatus == "running" {
		cmd := exec.Command("docker", "compose", "ps", "-q", "translator")
		output, _ := cmd.Output()
		if len(output) > 0 {
			runningCount = workers
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"type":    translatorType,
		"workers": workers,
		"status":  translatorStatus,
		"running": runningCount,
	})
}

// GetTranslationStats returns real translation throughput and live worker count.
func GetTranslationStats(c *gin.Context) {
	ctx := c.Request.Context()
	var last1min, last5min, activeWorkers int64
	var elapsed1min, elapsed5min float64

	db.GetPool().QueryRow(ctx, `
		SELECT COALESCE(SUM(items_translated),0), COALESCE(SUM(elapsed_ms),0)
		FROM translation_stats WHERE created_at >= NOW() - INTERVAL '1 minute'
	`).Scan(&last1min, &elapsed1min)

	db.GetPool().QueryRow(ctx, `
		SELECT COALESCE(SUM(items_translated),0), COALESCE(SUM(elapsed_ms),0)
		FROM translation_stats WHERE created_at >= NOW() - INTERVAL '5 minutes'
	`).Scan(&last5min, &elapsed5min)

	// Live workers = distinct hosts that recorded stats in the last 2 minutes
	db.GetPool().QueryRow(ctx, `
		SELECT COUNT(DISTINCT hostname) FROM translation_stats
		WHERE created_at >= NOW() - INTERVAL '2 minutes' AND hostname IS NOT NULL
	`).Scan(&activeWorkers)

	// Also count WS remote workers currently online
	remoteOnline := 0
	if err := db.GetPool().QueryRow(ctx, `
		SELECT COUNT(*) FROM remote_workers WHERE status = 'online'
	`).Scan(&remoteOnline); err != nil {
		remoteOnline = 0
	}

	rateSec := 0.0
	if elapsed1min > 0 {
		rateSec = float64(last1min) / (elapsed1min / 1000.0)
	}

	c.JSON(http.StatusOK, gin.H{
		"translations_last_1min":  last1min,
		"translations_last_5min":  last5min,
		"rate_per_second":         math.Round(rateSec*100) / 100,
		"rate_per_minute":         math.Round(rateSec*60*100) / 100,
		"active_workers":          activeWorkers + int64(remoteOnline),
		"remote_workers_online":   remoteOnline,
	})
}

func SetWorkerConfig(c *gin.Context) {
	var req WorkerConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "message": err.Error()})
		return
	}

	if req.Type != "cpu" && req.Type != "gpu" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Type must be 'cpu' or 'gpu'"})
		return
	}

	if req.Workers < 1 || req.Workers > 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Workers must be between 1 and 8"})
		return
	}

	ctx := c.Request.Context()

	_, err := db.GetPool().Exec(ctx, "UPDATE config SET value = $1, updated_at = NOW() WHERE key = 'translator_type'", req.Type)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update translator_type"})
		return
	}

	_, err = db.GetPool().Exec(ctx, "UPDATE config SET value = $1, updated_at = NOW() WHERE key = 'translator_workers'", strconv.Itoa(req.Workers))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update translator_workers"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Worker configuration updated",
		"type":    req.Type,
		"workers": req.Workers,
		"status":  req.Status,
	})
}

func StartWorkers(c *gin.Context) {
	var req WorkerConfig
	c.ShouldBindJSON(&req)

	ctx := c.Request.Context()

	// Obtener configuración actual
	var translatorType, translatorWorkers string
	err := db.GetPool().QueryRow(ctx, "SELECT value FROM config WHERE key = 'translator_type'").Scan(&translatorType)
	if err != nil || translatorType == "" {
		translatorType = "cpu"
	}
	err = db.GetPool().QueryRow(ctx, "SELECT value FROM config WHERE key = 'translator_workers'").Scan(&translatorWorkers)
	if err != nil || translatorWorkers == "" {
		translatorWorkers = "2"
	}

	if req.Type != "" {
		translatorType = req.Type
	}
	if req.Workers > 0 {
		translatorWorkers = strconv.Itoa(req.Workers)
	}

	workers, _ := strconv.Atoi(translatorWorkers)
	if workers < 1 {
		workers = 2
	}
	if workers > 8 {
		workers = 8
	}

	// Determinar qué servicio iniciar
	serviceName := "translator"
	if translatorType == "gpu" {
		serviceName = "translator-gpu"
	}

	// Detener cualquier translator existente
	composeDir := config.Load().DockerComposeDir
	stopCmd := exec.Command("docker", "compose", "stop", "translator", "translator-gpu")
	stopCmd.Dir = composeDir
	stopCmd.Run()

	// Iniciar con el número de workers
	startCmd := exec.Command("docker", "compose", "up", "-d", "--scale", fmt.Sprintf("%s=%d", serviceName, workers), serviceName)
	startCmd.Dir = composeDir
	output, err := startCmd.CombinedOutput()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to start workers",
			"details": string(output),
		})
		return
	}

	// Actualizar estado en BD
	if _, err := db.GetPool().Exec(ctx, "UPDATE config SET value = 'running', updated_at = NOW() WHERE key = 'translator_status'"); err != nil {
		log.Printf("Failed to update translator_status: %v", err)
	}
	if _, err := db.GetPool().Exec(ctx, "UPDATE config SET value = $1, updated_at = NOW() WHERE key = 'translator_type'", translatorType); err != nil {
		log.Printf("Failed to update translator_type: %v", err)
	}
	if _, err := db.GetPool().Exec(ctx, "UPDATE config SET value = $1, updated_at = NOW() WHERE key = 'translator_workers'", translatorWorkers); err != nil {
		log.Printf("Failed to update translator_workers: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Workers started successfully",
		"type":    translatorType,
		"workers": workers,
		"status":  "running",
	})
}

func StopWorkers(c *gin.Context) {
	// Detener traductores
	composeDir := config.Load().DockerComposeDir
	cmd := exec.Command("docker", "compose", "stop", "translator", "translator-gpu")
	cmd.Dir = composeDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to stop workers",
			"details": string(output),
		})
		return
	}

	// Actualizar estado en BD
	if _, err := db.GetPool().Exec(c.Request.Context(), "UPDATE config SET value = 'stopped', updated_at = NOW() WHERE key = 'translator_status'"); err != nil {
		log.Printf("Failed to update translator_status: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Workers stopped successfully",
		"status":  "stopped",
	})
}

// PatchEntityTipo changes the tipo of all tags matching a given valor
func PatchEntityTipo(c *gin.Context) {
	var req struct {
		Valor   string `json:"valor" binding:"required"`
		NewTipo string `json:"new_tipo" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "message": err.Error()})
		return
	}

	validTipos := map[string]bool{"persona": true, "organizacion": true, "lugar": true, "tema": true}
	if !validTipos[req.NewTipo] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tipo. Must be persona, organizacion, lugar or tema"})
		return
	}

	ctx := c.Request.Context()
	tx, err := db.GetPool().Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction", "message": err.Error()})
		return
	}
	defer tx.Rollback(ctx)

	// Since we don't know the exact old Tipo, we find all tags with this valor that ARE NOT already the new tipo
	rows, err := tx.Query(ctx, "SELECT id, tipo FROM tags WHERE valor = $1 AND tipo != $2", req.Valor, req.NewTipo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch existing tags", "message": err.Error()})
		return
	}

	type OldTag struct {
		ID   int
		Tipo string
	}
	var tagsToMove []OldTag
	for rows.Next() {
		var ot OldTag
		if err := rows.Scan(&ot.ID, &ot.Tipo); err == nil {
			tagsToMove = append(tagsToMove, ot)
		}
	}
	rows.Close()

	if len(tagsToMove) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "No entities found to update or already the requested tipo"})
		return
	}

	// Make sure the target tag (valor, new_tipo) exists
	lastName := extractLastName(req.Valor)
	var targetTagId int
	err = tx.QueryRow(ctx, `
		INSERT INTO tags (valor, tipo, apellido) VALUES ($1, $2, $3)
		ON CONFLICT (valor, tipo) DO UPDATE SET valor = EXCLUDED.valor, apellido = EXCLUDED.apellido
		RETURNING id`, req.Valor, req.NewTipo, lastName).Scan(&targetTagId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to ensure target tag", "message": err.Error()})
		return
	}

	totalMoved := 0
	for _, old := range tagsToMove {
		if old.ID == targetTagId {
			continue
		}

		// Move valid tags_noticia references to the target tag id safely
		res, err := tx.Exec(ctx, `
			UPDATE tags_noticia 
			SET tag_id = $1 
			WHERE tag_id = $2 AND NOT EXISTS (
				SELECT 1 FROM tags_noticia tn2 
				WHERE tn2.tag_id = $1 AND tn2.noticia_id = tags_noticia.noticia_id AND tn2.traduccion_id = tags_noticia.traduccion_id
			)
		`, targetTagId, old.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reassign news mentions", "message": err.Error()})
			return
		}
		totalMoved += int(res.RowsAffected())

		// Delete any remaining orphaned mentions (duplicates)
		_, err = tx.Exec(ctx, "DELETE FROM tags_noticia WHERE tag_id = $1", old.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete orphaned mentions", "message": err.Error()})
			return
		}

		// Delete the old tag since it's now merged
		_, err = tx.Exec(ctx, "DELETE FROM tags WHERE id = $1", old.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete old tag", "message": err.Error()})
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Entity tipo updated and merged successfully",
		"valor":         req.Valor,
		"new_tipo":      req.NewTipo,
		"tags_merged":   len(tagsToMove),
		"rows_affected": totalMoved,
	})
}

// pgConnInfo holds PostgreSQL connection parameters for pg_dump
type pgConnInfo struct {
	host, port, name, user, pass string
}

// resolvePgConnInfo prefers DATABASE_URL (the variable the API container gets)
// and falls back to the individual DB_* variables used by the workers.
func resolvePgConnInfo() pgConnInfo {
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		if u, err := url.Parse(dbURL); err == nil && u.Hostname() != "" {
			info := pgConnInfo{
				host: u.Hostname(),
				user: u.User.Username(),
				name: strings.TrimPrefix(u.Path, "/"),
				port: u.Port(),
			}
			if p, ok := u.User.Password(); ok {
				info.pass = p
			}
			if info.port == "" {
				info.port = "5432"
			}
			if info.user == "" {
				info.user = "postgres"
			}
			if info.name == "" {
				info.name = "postgres"
			}
			return info
		}
	}

	info := pgConnInfo{
		host: os.Getenv("DB_HOST"),
		port: os.Getenv("DB_PORT"),
		name: os.Getenv("DB_NAME"),
		user: os.Getenv("DB_USER"),
		pass: os.Getenv("DB_PASS"),
	}
	if info.host == "" {
		info.host = "db"
	}
	if info.port == "" {
		info.port = "5432"
	}
	if info.name == "" {
		info.name = "rss"
	}
	if info.user == "" {
		info.user = "rss"
	}
	return info
}

// flushWriter flushes the underlying response writer after every write so that
// large backups stream to the client instead of being buffered in RAM.
type flushWriter struct {
	w io.Writer
}

func (fw flushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	if f, ok := fw.w.(http.Flusher); ok {
		f.Flush()
	}
	return n, err
}

// BackupDatabase runs pg_dump and returns the SQL as a downloadable file.
// The output is streamed to the client so arbitrarily large dumps do not
// exhaust the container memory.
func BackupDatabase(c *gin.Context) {
	info := resolvePgConnInfo()

	cmd := exec.Command("pg_dump",
		"-h", info.host,
		"-p", info.port,
		"-U", info.user,
		"-d", info.name,
		"--no-password",
		"--format=plain",
	)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", info.pass))

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "pg_dump failed",
			"details": stderr.String(),
		})
		return
	}

	filename := fmt.Sprintf("backup_%s.sql", time.Now().Format("2006-01-02_15-04-05"))
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Cache-Control", "no-cache")
	c.Status(http.StatusOK)

	fw := flushWriter{w: c.Writer}
	copyDone := make(chan struct{})
	go func() {
		defer close(copyDone)
		_, _ = io.Copy(fw, pr)
	}()

	err := cmd.Wait()
	pw.Close()
	<-copyDone

	if err != nil {
		log.Printf("BackupDatabase: pg_dump failed: %v: %s", err, stderr.String())
		c.Writer.Flush()
		return
	}
	c.Writer.Flush()
}

// BackupNewsZipped performs a pg_dump of news tables and returns a ZIP file.
// pg_dump output is streamed into the ZIP as it is produced, keeping memory usage flat.
func BackupNewsZipped(c *gin.Context) {
	info := resolvePgConnInfo()

	// Tables to backup
	tables := []string{"noticias", "traducciones", "tags", "tags_noticia"}

	args := []string{
		"-h", info.host,
		"-p", info.port,
		"-U", info.user,
		"-d", info.name,
		"--no-password",
	}

	for _, table := range tables {
		args = append(args, "-t", table)
	}

	cmd := exec.Command("pg_dump", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", info.pass))

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "pg_dump failed",
			"details": stderr.String(),
		})
		return
	}

	filename := fmt.Sprintf("backup_noticias_%s.zip", time.Now().Format("2006-01-02_15-04-05"))
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Cache-Control", "no-cache")
	c.Status(http.StatusOK)

	fw := flushWriter{w: c.Writer}
	zw := zip.NewWriter(fw)

	sqlFileName := fmt.Sprintf("backup_noticias_%s.sql", time.Now().Format("2006-01-02"))
	f, err := zw.Create(sqlFileName)
	if err != nil {
		log.Printf("BackupNewsZipped: failed to create ZIP entry: %v", err)
		pw.Close()
		pr.Close()
		return
	}

	copyDone := make(chan struct{})
	go func() {
		defer close(copyDone)
		_, _ = io.Copy(f, pr)
	}()

	waitErr := cmd.Wait()
	pw.Close()
	<-copyDone
	closeErr := zw.Close()
	c.Writer.Flush()

	if waitErr != nil || closeErr != nil {
		log.Printf("BackupNewsZipped: pg_dump=%v zip=%v: %s", waitErr, closeErr, stderr.String())
		return
	}
}

// RestoreDatabase accepts an uploaded .sql or .zip backup and restores it via psql
func RestoreDatabase(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "No file uploaded",
			"message": "Se requiere un archivo .sql o .zip",
		})
		return
	}

	uploaded, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to open uploaded file"})
		return
	}
	defer uploaded.Close()

	data, err := io.ReadAll(uploaded)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read uploaded file"})
		return
	}

	var sqlData []byte
	lowerName := strings.ToLower(file.Filename)
	switch {
	case strings.HasSuffix(lowerName, ".sql"):
		sqlData = data
	case strings.HasSuffix(lowerName, ".zip"):
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ZIP file", "message": err.Error()})
			return
		}
		found := false
		for _, zf := range zr.File {
			if zf.FileInfo().IsDir() || !strings.HasSuffix(strings.ToLower(zf.Name), ".sql") {
				continue
			}
			rc, err := zf.Open()
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to open SQL inside ZIP", "message": err.Error()})
				return
			}
			sqlData, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read SQL inside ZIP", "message": err.Error()})
				return
			}
			found = true
			break
		}
		if !found {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No SQL file found inside ZIP"})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Unsupported file type",
			"message": "Solo se permiten archivos .sql o .zip",
		})
		return
	}

	if len(sqlData) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Empty backup file"})
		return
	}

	info := resolvePgConnInfo()

	cmd := exec.Command("psql",
		"-h", info.host,
		"-p", info.port,
		"-U", info.user,
		"-d", info.name,
		"--no-password",
		"-v", "ON_ERROR_STOP=1",
	)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", info.pass))
	cmd.Stdin = bytes.NewReader(sqlData)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Restore failed",
			"details": stderr.String(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Base de datos restaurada correctamente",
		"filename": file.Filename,
		"output":   out.String(),
	})
}

// DownloadRemoteWorker generates a downloadable package with configured remote workers
func DownloadRemoteWorker(c *gin.Context) {
	countStr := c.DefaultQuery("count", "1")
	count, err := strconv.Atoi(countStr)
	if err != nil || count < 1 || count > 10 {
		count = 1
	}

	serverURL := c.DefaultQuery("server", "")
	if serverURL == "" {
		proto := "ws"
		if c.Request.TLS != nil {
			proto = "wss"
		}
		host := c.Request.Host
		if strings.HasSuffix(host, ":8080") {
			host = strings.TrimSuffix(host, ":8080")
		}
		serverURL = fmt.Sprintf("%s://%s/ws/worker", proto, host)
	}

	ctx := c.Request.Context()

	type WorkerInfo struct {
		ID     int
		Name   string
		APIKey string
	}

	workers := []WorkerInfo{}

	for i := 1; i <= count; i++ {
		workerName := fmt.Sprintf("remote-worker-%d", i)

		var id int
		var apiKey string

		err := db.GetPool().QueryRow(ctx, `
			SELECT id, api_key FROM remote_workers WHERE name = $1
		`, workerName).Scan(&id, &apiKey)

		if err != nil {
			apiKey = generateAPIKey()
			err = db.GetPool().QueryRow(ctx, `
				INSERT INTO remote_workers (name, api_key, capabilities, status, created_at)
				VALUES ($1, $2, 'cpu', 'offline', NOW())
				RETURNING id
			`, workerName, apiKey).Scan(&id)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create worker: " + err.Error()})
				return
			}
		}

		workers = append(workers, WorkerInfo{
			ID:     id,
			Name:   workerName,
			APIKey: apiKey,
		})
	}

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	files := map[string]string{
		"worker.py":          generateWorkerPy(),
		"Dockerfile":         generateWorkerDockerfile(),
		"docker-compose.yml": generateDockerCompose(count, serverURL),
		"deploy.sh":          generateDeployScript(),
		"README.md":          generateReadme(),
	}

	if count == 1 {
		files[".env"] = generateEnvFile(workers[0].APIKey, serverURL)
	}

	for filename, content := range files {
		w, err := zipWriter.Create(filename)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create zip: " + err.Error()})
			return
		}
		w.Write([]byte(content))
	}

	zipWriter.Close()

	filename := fmt.Sprintf("rss2-remote-workers-%d.zip", count)

	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Data(http.StatusOK, "application/zip", buf.Bytes())
}

func generateAPIKey() string {
	b := make([]byte, 16)
	for i := range b {
		b[i] = byte(time.Now().UnixNano() % 256)
	}
	return fmt.Sprintf("%x", b)
}

func generateWorkerPy() string {
	return `#!/usr/bin/env python3
"""
Remote Translation Worker para RSS2.
Se conecta al backend por WebSocket y traduce noticias.
Traduce títulos, resúmenes y contenido completo.
"""

import os
import sys
import time
import json
import logging
import re
from typing import List

import websocket

import ctranslate2
from transformers import AutoTokenizer

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s: %(message)s",
    handlers=[logging.StreamHandler(sys.stdout)]
)
LOG = logging.getLogger("remote-translator")

WORKER_NAME = os.environ.get("WORKER_NAME", "remote-worker")
WORKER_API_KEY = os.environ.get("WORKER_API_KEY", "")
WORKER_SERVER = os.environ.get("WORKER_SERVER", "ws://localhost:8080/ws/worker")

DEVICE = os.environ.get("CT2_DEVICE", "cpu")
MODEL_PATH = os.environ.get("CT2_MODEL_PATH", "/app/models/nllb-ct2")
COMPUTE_TYPE = os.environ.get("CT2_COMPUTE_TYPE", "int8")
UNIVERSAL_MODEL = os.environ.get("UNIVERSAL_MODEL", "facebook/nllb-200-distilled-600M")

MAX_SRC_TOKENS = int(os.environ.get("MAX_SRC_TOKENS", "2048"))
MAX_NEW_TOKENS = int(os.environ.get("MAX_NEW_TOKENS", "2048"))
MAX_BODY_CHARS = int(os.environ.get("MAX_BODY_CHARS", "80000"))
BODY_CHARS_CHUNK = int(os.environ.get("BODY_CHARS_CHUNK", "2000"))
MAX_SEQ_PER_CALL = int(os.environ.get("MAX_SEQ_PER_CALL", "32"))

LANG_CODE_MAP = {
    "en": "eng_Latn", "es": "spa_Latn", "fr": "fra_Latn", "de": "deu_Latn",
    "it": "ita_Latn", "pt": "por_Latn", "nl": "nld_Latn", "sv": "swe_Latn",
    "da": "dan_Latn", "fi": "fin_Latn", "no": "nob_Latn", "pl": "pol_Latn",
    "cs": "ces_Latn", "sk": "slk_Latn", "hu": "hun_Latn", "ro": "ron_Latn",
    "el": "ell_Grek", "ru": "rus_Cyrl", "uk": "ukr_Cyrl", "tr": "tur_Latn",
    "ar": "arb_Arab", "fa": "pes_Arab", "he": "heb_Hebr", "zh": "zho_Hans",
    "ja": "jpn_Jpan", "ko": "kor_Hang", "vi": "vie_Latn", "lt": "lit_Latn",
    "bg": "bul_Cyrl", "sq": "als_Latn", "so": "som_Latn", "sk": "slk_Latn",
}

SENTENCE_END_CHARS = set(".!?;؟؛।။॥।")

_tokenizer = None
_translator = None
_ws = None
_reconnect_delay = 5
_running = True
_stats = {"jobs_completed": 0, "jobs_failed": 0}

def ensure_model():
    global _tokenizer, _translator
    if _translator:
        return
    model_bin = os.path.join(MODEL_PATH, "model.bin")
    if not os.path.exists(model_bin):
        LOG.info(f"Model not found, converting...")
        convert_model()
    LOG.info(f"Loading model from {MODEL_PATH}")
    _translator = ctranslate2.Translator(MODEL_PATH, device=DEVICE, compute_type=COMPUTE_TYPE)
    _tokenizer = AutoTokenizer.from_pretrained(UNIVERSAL_MODEL)
    LOG.info("Model loaded successfully")

def convert_model():
    import subprocess
    os.makedirs(MODEL_PATH, exist_ok=True)
    cmd = ["ct2-transformers-converter", "--model", UNIVERSAL_MODEL,
           "--output_dir", MODEL_PATH, "--quantization", COMPUTE_TYPE, "--force"]
    LOG.info(f"Converting {UNIVERSAL_MODEL}...")
    result = subprocess.run(cmd, capture_output=True, text=True, timeout=3600)
    if result.returncode != 0:
        LOG.error(f"Conversion failed: {result.stderr}")
        raise RuntimeError("Model conversion failed")

def translate_texts(src, tgt, texts):
    if not texts:
        return []
    ensure_model()
    clean = [(t or "").strip() for t in texts]
    if all(not t for t in clean):
        return ["" for _ in clean]
    src_code = LANG_CODE_MAP.get(src, f"{src}_Latn")
    tgt_code = LANG_CODE_MAP.get(tgt, "spa_Latn")
    try:
        _tokenizer.src_lang = src_code
    except:
        pass
    sources = []
    for t in clean:
        if t:
            ids = _tokenizer.encode(t, truncation=True, max_length=MAX_SRC_TOKENS)
            tokens = _tokenizer.convert_ids_to_tokens(ids)
            sources.append(tokens)
        else:
            sources.append([])
    target_prefix = [[tgt_code]] * len(sources)
    translated = []
    for i in range(0, len(sources), MAX_SEQ_PER_CALL):
        results = _translator.translate_batch(
            sources[i:i+MAX_SEQ_PER_CALL],
            target_prefix=target_prefix[i:i+MAX_SEQ_PER_CALL],
            beam_size=1, max_decoding_length=MAX_NEW_TOKENS,
            repetition_penalty=1.2, no_repeat_ngram_size=2)
        for result in results:
            try:
                if result.hypotheses and len(result.hypotheses) > 0:
                    hyp = result.hypotheses[0]
                    if isinstance(hyp, list) and len(hyp) > 0:
                        first_hyp = hyp[0]
                        if isinstance(first_hyp, dict) and "token_ids" in first_hyp:
                            tokens = first_hyp["token_ids"]
                            text = _tokenizer.decode(tokens)
                            translated.append(text.strip())
                        elif isinstance(first_hyp, str):
                            token_strings = hyp[1:] if len(hyp) > 1 else []
                            if token_strings:
                                text = _tokenizer.convert_tokens_to_string(token_strings)
                                translated.append(text.strip())
                            else:
                                translated.append("")
                        else:
                            translated.append("")
                else:
                    translated.append("")
            except:
                translated.append("")
    return translated

def truncate_at_sentence_boundary(text, max_len):
    if len(text) <= max_len:
        return text
    search_end = min(len(text), max_len)
    for i in range(search_end - 1, max(0, search_end - 200), -1):
        if text[i] in SENTENCE_END_CHARS:
            return text[:i + 1]
    cutoff = int(max_len * 0.9)
    last_space = text.rfind(" ", cutoff, max_len)
    if last_space > 0:
        return text[:last_space]
    return text[:max_len]

def split_into_chunks(text):
    text = (text or "").strip()
    if len(text) <= BODY_CHARS_CHUNK:
        return [text] if text else []
    parts = re.split(r"(\n\n+|(?<=[\.\!\?؛؟।။॥।])\s+)", text)
    chunks, current = [], ""
    for part in parts:
        if not part:
            continue
        if len(current) + len(part) <= BODY_CHARS_CHUNK:
            current += part
        else:
            if current.strip():
                chunks.append(current.strip())
            current = part
    if current.strip():
        chunks.append(current.strip())
    return chunks if chunks else [text]

def translate_long_text(src, tgt, body):
    body = (body or "").strip()
    if not body:
        return ""
    if len(body) > MAX_BODY_CHARS:
        body = truncate_at_sentence_boundary(body, MAX_BODY_CHARS)
    chunks = split_into_chunks(body)
    if len(chunks) == 1:
        return translate_texts(src, tgt, [body])[0]
    translated_chunks = translate_texts(src, tgt, chunks)
    return join_chunks(translated_chunks)

def join_chunks(chunks):
    if not chunks:
        return ""
    if len(chunks) == 1:
        return chunks[0]
    result = chunks[0]
    for chunk in chunks[1:]:
        if not chunk:
            continue
        starts_lower = chunk and chunk[0].islower()
        prev_ends_lower = result and result[-1].islower() if result else False
        if prev_ends_lower and starts_lower:
            result += " " + chunk
        elif result and result[-1] in SENTENCE_END_CHARS:
            result += " " + chunk
        else:
            result += " " + chunk
    return result

def process_job(job):
    job_id = job.get("id")
    lang_from = job.get("lang_from", "en")
    lang_to = job.get("lang_to", "es")
    title = job.get("title", "")
    summary = job.get("summary", "")
    content = job.get("content", "")
    LOG.info(f"Processing job {job_id}: {lang_from} -> {lang_to}")
    if lang_from == lang_to:
        return {"job_id": job_id, "title_trad": title, "summary_trad": summary, "contenido_trad": content}
    try:
        title_tr = translate_texts(lang_from, lang_to, [title])[0] if title else title
        summary_tr = translate_long_text(lang_from, lang_to, summary) if summary else summary
        content_tr = translate_long_text(lang_from, lang_to, content) if content else content
        return {"job_id": job_id, "title_trad": title_tr, "summary_trad": summary_tr, "contenido_trad": content_tr}
    except Exception as e:
        LOG.error(f"Job {job_id} failed: {e}")
        return {"job_id": job_id, "error": str(e)}

def connect_ws():
    global _ws, _reconnect_delay
    try:
        ws_url = f"{WORKER_SERVER}?api_key={WORKER_API_KEY}"
        _ws = websocket.WebSocketApp(ws_url,
            on_open=on_open, on_message=on_message,
            on_error=on_error, on_close=on_close)
        LOG.info(f"Connecting to {ws_url}")
        _ws.run_forever()
    except Exception as e:
        LOG.error(f"WebSocket error: {e}")
    _reconnect_delay = min(_reconnect_delay * 2, 60)
    LOG.info(f"Reconnecting in {_reconnect_delay}s...")
    time.sleep(_reconnect_delay)
    _reconnect_delay = 5
    connect_ws()

def on_open(ws):
    LOG.info("WebSocket connected")
    ws.send(json.dumps({"type": "register", "worker_name": WORKER_NAME, "capabilities": "cpu"}))
    LOG.info(f"Registered as {WORKER_NAME}")

def on_message(ws, message):
    global _running
    try:
        msg = json.loads(message)
        msg_type = msg.get("type")
        if msg_type == "ping":
            ws.send(json.dumps({"type": "pong"}))
        elif msg_type == "job":
            job = msg.get("job", {})
            result = process_job(job)
            ws.send(json.dumps({"type": "result", **result}))
            _stats["jobs_completed"] += 1
        elif msg_type == "stop":
            _running = False
    except Exception as e:
        LOG.error(f"Error: {e}")

def on_error(ws, error):
    LOG.error(f"WebSocket error: {error}")

def on_close(ws, close_status_code, close_msg):
    LOG.info(f"Closed: {close_status_code}")

def main():
    LOG.info(f"Starting {WORKER_NAME}")
    ensure_model()
    connect_ws()

if __name__ == "__main__":
    main()
`
}

func generateWorkerDockerfile() string {
	return `FROM python:3.11-slim-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends \
    patchelf gcc git curl wget \
    && rm -rf /var/lib/apt/lists/*

ENV PYTHONUNBUFFERED=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    TOKENIZERS_PARALLELISM=false \
    HF_HOME=/root/.cache/huggingface

WORKDIR /app

RUN pip install --no-cache-dir --upgrade pip && \
    pip install --no-cache-dir torch==2.1.0 torchvision==0.16.0 --index-url https://download.pytorch.org/whl/cpu && \
    pip install --no-cache-dir \
    ctranslate2==3.24.0 \
    sentencepiece \
    transformers==4.36.0 \
    protobuf==3.20.3 \
    "numpy<2" \
    websocket-client

COPY worker.py /app/worker.py

ENV CT2_DEVICE=${CT2_DEVICE:-cpu}
ENV CT2_COMPUTE_TYPE=${CT2_COMPUTE_TYPE:-int8}

CMD ["python", "/app/worker.py"]
`
}

func generateDockerCompose(count int, serverURL string) string {
	services := `version: '3.8'

services:
`
	for i := 1; i <= count; i++ {
		services += fmt.Sprintf(`  worker-%d:
    build: .
    container_name: rss2-worker-%d
    environment:
      WORKER_NAME: "worker-%d"
      WORKER_API_KEY: "${WORKER_API_KEY_%d}"
      WORKER_SERVER: "%s"
      CT2_DEVICE: "cpu"
      CT2_COMPUTE_TYPE: "int8"
      MAX_SRC_TOKENS: 2048
      MAX_NEW_TOKENS: 2048
      MAX_BODY_CHARS: 80000
      BODY_CHARS_CHUNK: 2000
      MAX_SEQ_PER_CALL: 32
    volumes:
      - ./models:/app/models
    restart: unless-stopped
`, i, i, i, i, serverURL)
	}

	services += `
networks:
  default:
    name: rss2_remote
`
	return services
}

func generateDeployScript() string {
	return `#!/bin/bash
set -e

echo "=== RSS2 Remote Worker Deployment ==="
echo ""

echo "Building Docker image..."
docker build --no-cache -t rss2-remote-worker:latest .

echo "Creating models directory..."
mkdir -p models

echo "Starting workers..."
docker compose up -d

echo ""
echo "=== Deployed successfully! ==="
echo "View logs: docker compose logs -f"
echo "Stop: docker compose down"
`
}

func generateEnvFile(apiKey, serverURL string) string {
	return fmt.Sprintf(`WORKER_API_KEY_1=%s
SERVER_URL=%s
`, apiKey, serverURL)
}

func generateReadme() string {
	return `# RSS2 Remote Workers

Downloaded from RSS2 Admin Panel

## Quick Start

1. Extract the zip file
2. Run: chmod +x deploy.sh && ./deploy.sh
3. Or: docker compose up -d

## Configuration

Edit .env or docker-compose.yml to adjust:
- WORKER_API_KEY: Your API key  
- SERVER_URL: WebSocket URL to backend

## Files Included

- worker.py - Python translation worker
- Dockerfile - Docker image definition
- docker-compose.yml - Multi-worker setup
- deploy.sh - Deployment script

## Requirements

- Docker
- 4GB RAM per worker
- CPU with AVX2 support
`
}
