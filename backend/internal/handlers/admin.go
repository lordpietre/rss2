package handlers

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
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

	// 1. Ensure the canonical tag exists in tags table
	var canonicalTagId int
	err = tx.QueryRow(ctx, `
		INSERT INTO tags (valor, tipo) VALUES ($1, $2)
		ON CONFLICT (valor, tipo) DO UPDATE SET valor = EXCLUDED.valor
		RETURNING id`, req.CanonicalName, req.Tipo).Scan(&canonicalTagId)
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

func ExportAliases(c *gin.Context) {
	rows, err := db.GetPool().Query(c.Request.Context(),
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
	var targetTagId int
	err = tx.QueryRow(ctx, `
		INSERT INTO tags (valor, tipo) VALUES ($1, $2)
		ON CONFLICT (valor, tipo) DO UPDATE SET valor = EXCLUDED.valor
		RETURNING id`, req.Valor, req.NewTipo).Scan(&targetTagId)
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

// BackupDatabase runs pg_dump and returns the SQL as a downloadable file
func BackupDatabase(c *gin.Context) {
	// Ejecutar pg_dump desde dentro del contenedor db para evitar problemas de red
	// Usamos docker exec para ejecutar el comando dentro del servicio db
	cmd := exec.Command("docker", "exec", "rss2_db", "pg_dump",
		"-U", os.Getenv("POSTGRES_USER"),
		"-d", os.Getenv("POSTGRES_DB"),
		"--no-password",
		"--format=plain",
		"--pghost=5432",
	)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
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
	c.Data(http.StatusOK, "application/octet-stream", out.Bytes())
}

// BackupNewsZipped performs a pg_dump of news tables and returns a ZIP file
func BackupNewsZipped(c *gin.Context) {
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "db"
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "rss"
	}
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "rss"
	}
	dbPass := os.Getenv("DB_PASS")

	// Tables to backup
	tables := []string{"noticias", "traducciones", "tags", "tags_noticia"}

	args := []string{
		"-h", dbHost,
		"-p", dbPort,
		"-U", dbUser,
		"-d", dbName,
		"--no-password",
	}

	for _, table := range tables {
		args = append(args, "-t", table)
	}

	cmd := exec.Command("pg_dump", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", dbPass))

	var sqlOut bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &sqlOut
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "pg_dump failed",
			"details": stderr.String(),
		})
		return
	}

	// Create ZIP
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	sqlFileName := fmt.Sprintf("backup_noticias_%s.sql", time.Now().Format("2006-01-02"))
	f, err := zw.Create(sqlFileName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create ZIP entry", "message": err.Error()})
		return
	}

	_, err = f.Write(sqlOut.Bytes())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write to ZIP", "message": err.Error()})
		return
	}

	if err := zw.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to close ZIP writer", "message": err.Error()})
		return
	}

	filename := fmt.Sprintf("backup_noticias_%s.zip", time.Now().Format("2006-01-02_15-04-05"))
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "application/zip", buf.Bytes())
}
