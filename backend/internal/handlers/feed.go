package handlers

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rss2/backend/internal/db"
	"github.com/rss2/backend/internal/models"
)

type FeedResponse struct {
	ID          int64   `json:"id"`
	Nombre      string  `json:"nombre"`
	Descripcion *string `json:"descripcion"`
	URL         string  `json:"url"`
	CategoriaID *int64  `json:"categoria_id"`
	PaisID      *int64  `json:"pais_id"`
	Idioma      *string `json:"idioma"`
	Activo      bool    `json:"activo"`
	Fallos      *int64  `json:"fallos"`
	LastError   *string `json:"last_error"`
	FuenteURLID *int64  `json:"fuente_url_id"`
}

func GetFeeds(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "50"))
	activo := c.Query("activo")
	categoriaID := c.Query("categoria_id")
	paisID := c.Query("pais_id")

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}

	offset := (page - 1) * perPage

	where := "1=1"
	args := []interface{}{}
	argNum := 1

	if activo != "" {
		where += fmt.Sprintf(" AND activo = $%d", argNum)
		args = append(args, activo == "true")
		argNum++
	}
	if categoriaID != "" {
		where += fmt.Sprintf(" AND categoria_id = $%d", argNum)
		args = append(args, categoriaID)
		argNum++
	}
	if paisID != "" {
		where += fmt.Sprintf(" AND pais_id = $%d", argNum)
		args = append(args, paisID)
		argNum++
	}

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM feeds WHERE %s", where)
	err := db.GetPool().QueryRow(c.Request.Context(), countQuery, args...).Scan(&total)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to count feeds", Message: err.Error()})
		return
	}

	sqlQuery := fmt.Sprintf(`
		SELECT f.id, f.nombre, f.descripcion, f.url,
		       f.categoria_id, f.pais_id, f.idioma, f.activo, f.fallos, f.last_error,
		       c.nombre AS categoria, p.nombre AS pais,
		       (SELECT COUNT(*) FROM noticias n WHERE n.fuente_nombre = f.nombre) as noticias_count
		FROM feeds f
		LEFT JOIN categorias c ON c.id = f.categoria_id
		LEFT JOIN paises p ON p.id = f.pais_id
		WHERE %s
		ORDER BY p.nombre NULLS LAST, f.activo DESC, f.fallos ASC, c.nombre NULLS LAST, f.nombre
		LIMIT $%d OFFSET $%d
	`, where, argNum, argNum+1)

	args = append(args, perPage, offset)

	rows, err := db.GetPool().Query(c.Request.Context(), sqlQuery, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch feeds", Message: err.Error()})
		return
	}
	defer rows.Close()

	type FeedWithStats struct {
		FeedResponse
		Categoria     *string `json:"categoria"`
		Pais          *string `json:"pais"`
		NoticiasCount int64   `json:"noticias_count"`
	}

	var feeds []FeedWithStats
	for rows.Next() {
		var f FeedWithStats
		err := rows.Scan(
			&f.ID, &f.Nombre, &f.Descripcion, &f.URL,
			&f.CategoriaID, &f.PaisID, &f.Idioma, &f.Activo, &f.Fallos, &f.LastError,
			&f.Categoria, &f.Pais, &f.NoticiasCount,
		)
		if err != nil {
			continue
		}
		feeds = append(feeds, f)
	}

	totalPages := (total + perPage - 1) / perPage

	c.JSON(http.StatusOK, gin.H{
		"feeds":       feeds,
		"total":       total,
		"page":        page,
		"per_page":    perPage,
		"total_pages": totalPages,
	})
}

func GetFeedByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid feed ID"})
		return
	}

	var f FeedResponse
	err = db.GetPool().QueryRow(c.Request.Context(), `
		SELECT id, nombre, descripcion, url, categoria_id, pais_id, idioma, activo, fallos
		FROM feeds WHERE id = $1`, id).Scan(
		&f.ID, &f.Nombre, &f.Descripcion, &f.URL,
		&f.CategoriaID, &f.PaisID, &f.Idioma, &f.Activo, &f.Fallos,
	)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Feed not found"})
		return
	}

	c.JSON(http.StatusOK, f)
}

type CreateFeedRequest struct {
	Nombre      string  `json:"nombre" binding:"required"`
	URL         string  `json:"url" binding:"required,url"`
	Descripcion *string `json:"descripcion"`
	CategoriaID *int64  `json:"categoria_id"`
	PaisID      *int64  `json:"pais_id"`
	Idioma      *string `json:"idioma"`
}

func CreateFeed(c *gin.Context) {
	var req CreateFeedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Message: err.Error()})
		return
	}

	var feedID int64
	err := db.GetPool().QueryRow(c.Request.Context(), `
		INSERT INTO feeds (nombre, descripcion, url, categoria_id, pais_id, idioma)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		req.Nombre, req.Descripcion, req.URL, req.CategoriaID, req.PaisID, req.Idioma,
	).Scan(&feedID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to create feed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": feedID, "message": "Feed created successfully"})
}

type UpdateFeedRequest struct {
	Nombre      string  `json:"nombre" binding:"required"`
	URL         string  `json:"url" binding:"required,url"`
	Descripcion *string `json:"descripcion"`
	CategoriaID *int64  `json:"categoria_id"`
	PaisID      *int64  `json:"pais_id"`
	Idioma      *string `json:"idioma"`
	Activo      *bool   `json:"activo"`
}

func UpdateFeed(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid feed ID"})
		return
	}

	var req UpdateFeedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Message: err.Error()})
		return
	}

	activeVal := true
	if req.Activo != nil {
		activeVal = *req.Activo
	}

	result, err := db.GetPool().Exec(c.Request.Context(), `
		UPDATE feeds 
		SET nombre = $1, descripcion = $2, url = $3, 
		    categoria_id = $4, pais_id = $5, idioma = $6, activo = $7
		WHERE id = $8`,
		req.Nombre, req.Descripcion, req.URL,
		req.CategoriaID, req.PaisID, req.Idioma, activeVal, id,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to update feed", Message: err.Error()})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Feed not found"})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "Feed updated successfully"})
}

func DeleteFeed(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid feed ID"})
		return
	}

	result, err := db.GetPool().Exec(c.Request.Context(), "DELETE FROM feeds WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to delete feed", Message: err.Error()})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Feed not found"})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "Feed deleted successfully"})
}

func ToggleFeedActive(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid feed ID"})
		return
	}

	result, err := db.GetPool().Exec(c.Request.Context(), `
		UPDATE feeds SET activo = NOT activo WHERE id = $1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to toggle feed", Message: err.Error()})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Feed not found"})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "Feed toggled successfully"})
}

func ReactivateFeed(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid feed ID"})
		return
	}

	result, err := db.GetPool().Exec(c.Request.Context(), `
		UPDATE feeds SET activo = TRUE, fallos = 0 WHERE id = $1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to reactivate feed", Message: err.Error()})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Feed not found"})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "Feed reactivated successfully"})
}

func ExportFeeds(c *gin.Context) {
	activo := c.Query("activo")
	categoriaID := c.Query("categoria_id")
	paisID := c.Query("pais_id")

	where := "1=1"
	args := []interface{}{}
	argNum := 1

	if activo != "" {
		where += fmt.Sprintf(" AND activo = $%d", argNum)
		args = append(args, activo == "true")
		argNum++
	}
	if categoriaID != "" {
		where += fmt.Sprintf(" AND categoria_id = $%d", argNum)
		args = append(args, categoriaID)
		argNum++
	}
	if paisID != "" {
		where += fmt.Sprintf(" AND pais_id = $%d", argNum)
		args = append(args, paisID)
		argNum++
	}

	query := fmt.Sprintf(`
		SELECT f.id, f.nombre, f.descripcion, f.url,
		       f.categoria_id, c.nombre AS categoria,
		       f.pais_id, p.nombre AS pais,
		       f.idioma, f.activo, f.fallos
		FROM feeds f
		LEFT JOIN categorias c ON c.id = f.categoria_id
		LEFT JOIN paises p ON p.id = f.pais_id
		WHERE %s
		ORDER BY f.id
	`, where)

	rows, err := db.GetPool().Query(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch feeds", Message: err.Error()})
		return
	}
	defer rows.Close()

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=feeds_export.csv")

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	writer.Write([]string{"id", "nombre", "descripcion", "url", "categoria_id", "categoria", "pais_id", "pais", "idioma", "activo", "fallos"})

	for rows.Next() {
		var id int64
		var nombre, url string
		var descripcion, idioma *string
		var categoriaID, paisID, fallos *int64
		var activo bool
		var categoria, pais *string

		err := rows.Scan(&id, &nombre, &descripcion, &url, &categoriaID, &categoria, &paisID, &pais, &idioma, &activo, &fallos)
		if err != nil {
			continue
		}

		writer.Write([]string{
			fmt.Sprintf("%d", id),
			nombre,
			stringOrEmpty(descripcion),
			url,
			int64ToString(categoriaID),
			stringOrEmpty(categoria),
			int64ToString(paisID),
			stringOrEmpty(pais),
			stringOrEmpty(idioma),
			fmt.Sprintf("%t", activo),
			int64ToString(fallos),
		})
	}
}

func ImportFeeds(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "No file provided"})
		return
	}

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to open file", Message: err.Error()})
		return
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to read file", Message: err.Error()})
		return
	}

	reader := csv.NewReader(strings.NewReader(string(content)))
	_, err = reader.Read()
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid CSV format"})
		return
	}

	imported := 0
	skipped := 0
	failed := 0
	errors := []string{}

	tx, err := db.GetPool().Begin(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to start transaction", Message: err.Error()})
		return
	}
	defer tx.Rollback(context.Background())

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			failed++
			continue
		}

		if len(record) < 4 {
			skipped++
			continue
		}

		nombre := strings.TrimSpace(record[1])
		url := strings.TrimSpace(record[3])

		if nombre == "" || url == "" {
			skipped++
			continue
		}

		var descripcion *string
		if len(record) > 2 && strings.TrimSpace(record[2]) != "" {
			descripcionStr := strings.TrimSpace(record[2])
			descripcion = &descripcionStr
		}

		var categoriaID *int64
		if len(record) > 4 && strings.TrimSpace(record[4]) != "" {
			catID, err := strconv.ParseInt(strings.TrimSpace(record[4]), 10, 64)
			if err == nil {
				categoriaID = &catID
			}
		}

		var paisID *int64
		if len(record) > 6 && strings.TrimSpace(record[6]) != "" {
			pID, err := strconv.ParseInt(strings.TrimSpace(record[6]), 10, 64)
			if err == nil {
				paisID = &pID
			}
		}

		var idioma *string
		if len(record) > 8 && strings.TrimSpace(record[8]) != "" {
			lang := strings.TrimSpace(record[8])
			if len(lang) > 2 {
				lang = lang[:2]
			}
			idioma = &lang
		}

		activo := true
		if len(record) > 9 && strings.TrimSpace(record[9]) != "" {
			activo = strings.ToLower(strings.TrimSpace(record[9])) == "true"
		}

		var fallos int64
		if len(record) > 10 && strings.TrimSpace(record[10]) != "" {
			f, err := strconv.ParseInt(strings.TrimSpace(record[10]), 10, 64)
			if err == nil {
				fallos = f
			}
		}

		var existingID int64
		err = tx.QueryRow(context.Background(), "SELECT id FROM feeds WHERE url = $1", url).Scan(&existingID)
		if err == nil {
			_, err = tx.Exec(context.Background(), `
				UPDATE feeds SET nombre=$1, descripcion=$2, categoria_id=$3, pais_id=$4, idioma=$5, activo=$6, fallos=$7
				WHERE id=$8`,
				nombre, descripcion, categoriaID, paisID, idioma, activo, fallos, existingID,
			)
			if err != nil {
				failed++
				errors = append(errors, fmt.Sprintf("Error updating %s: %v", url, err))
				continue
			}
		} else {
			_, err = tx.Exec(context.Background(), `
				INSERT INTO feeds (nombre, descripcion, url, categoria_id, pais_id, idioma, activo, fallos)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
				nombre, descripcion, url, categoriaID, paisID, idioma, activo, fallos,
			)
			if err != nil {
				failed++
				errors = append(errors, fmt.Sprintf("Error inserting %s: %v", url, err))
				continue
			}
		}

		imported++
	}

	if err := tx.Commit(context.Background()); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to commit transaction", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"imported": imported,
		"skipped":  skipped,
		"failed":   failed,
		"errors":   errors,
		"message":  fmt.Sprintf("Import completed. Imported: %d, Skipped: %d, Failed: %d", imported, skipped, failed),
	})
}

func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func int64ToString(i *int64) string {
	if i == nil {
		return ""
	}
	return fmt.Sprintf("%d", *i)
}
