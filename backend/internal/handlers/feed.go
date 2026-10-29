package handlers

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rss2/backend/internal/cache"
	"github.com/rss2/backend/internal/db"
	"github.com/rss2/backend/internal/models"
)

// GetFeeds returns paginated feeds with optional filters
// @Summary List feeds
// @Description Returns paginated feeds with optional filtering by active status, category, and country
// @Tags feeds
// @Produce json
// @Param page query int false "Page number" default(1) minimum(1)
// @Param per_page query int false "Items per page" default(50) minimum(1) maximum(100)
// @Param activo query string false "Active status filter"
// @Param categoria_id query string false "Category ID filter"
// @Param pais_id query string false "Country ID filter"
// @Success 200 {object} map[string]interface{} "feeds, total, page, per_page, total_pages"
// @Failure 500 {object} models.ErrorResponse
// @Router /feeds [get]
func GetFeeds(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "50"))
	page, perPage = validatePageParams(page, perPage, 50, 100)
	activo := c.Query("activo")
	categoriaID := c.Query("categoria_id")
	paisID := c.Query("pais_id")
	// Busqueda simple por nombre, descripcion o URL. Es la via practica para
	// localizar un grupo de canales (p.ej. las Wikipedia de cambios
	// recientes) y borrarlos de una vez.
	query := strings.TrimSpace(c.Query("q"))
	// Filtro por la lista de canales ya traducidos.
	traducido := c.Query("traducido")

	offset := (page - 1) * perPage

	// Cache key based on filters
	filterStr := fmt.Sprintf("activo_%s_cat_%s_pais_%s_q_%s_trad_%s", activo, categoriaID, paisID, query, traducido)
	cacheKey := cache.FeedListKey(filterStr, page, perPage)

	ctx := c.Request.Context()

	// Try to get from cache
	if cached, err := cache.Get(ctx, cacheKey); err == nil && cached != "" {
		c.Data(http.StatusOK, "application/json", []byte(cached))
		return
	}

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
	if query != "" {
		pattern := "%" + query + "%"
		// 'nombre'/'url' existen tambien en categorias y paises: hay que
		// prefijar con el alias o la consulta es ambigua.
		where += fmt.Sprintf(` AND (f.nombre ILIKE $%d OR f.url ILIKE $%d OR COALESCE(f.descripcion, '') ILIKE $%d)`, argNum, argNum, argNum)
		args = append(args, pattern)
		argNum++
	}
	if traducido != "" {
		where += fmt.Sprintf(" AND f.traducido = $%d", argNum)
		args = append(args, traducido == "true")
		argNum++
	}

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM feeds f WHERE %s", where)
	err := db.GetPool().QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to count feeds", Message: err.Error()})
		return
	}

	sqlQuery := fmt.Sprintf(`
		SELECT f.id, f.nombre, f.descripcion, f.url,
		       f.categoria_id, f.pais_id, f.idioma, f.activo, f.fallos, f.last_error, f.last_fetch,
		       f.traducido,
		       c.nombre AS categoria, p.nombre AS pais,
		       COALESCE(fs.noticias_count, 0) as noticias_count
		FROM feeds f
		LEFT JOIN categorias c ON c.id = f.categoria_id
		LEFT JOIN paises p ON p.id = f.pais_id
		LEFT JOIN feed_stats fs ON fs.fuente_nombre = f.nombre
		WHERE %s
		ORDER BY p.nombre NULLS LAST, f.activo DESC, f.fallos ASC, c.nombre NULLS LAST, f.nombre
		LIMIT $%d OFFSET $%d
	`, where, argNum, argNum+1)

	args = append(args, perPage, offset)

	rows, err := db.GetPool().Query(ctx, sqlQuery, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch feeds", Message: err.Error()})
		return
	}
	defer rows.Close()

	type FeedWithStats struct {
		models.Feed
		Categoria     *string `json:"categoria"`
		Pais          *string `json:"pais"`
		NoticiasCount int64   `json:"noticias_count"`
	}

	var feeds []FeedWithStats
	for rows.Next() {
		var f FeedWithStats
		err := rows.Scan(
			&f.ID, &f.Nombre, &f.Descripcion, &f.URL,
			&f.CategoriaID, &f.PaisID, &f.Idioma, &f.Activo, &f.Fallos, &f.LastError, &f.LastFetched,
			&f.Traducido,
			&f.Categoria, &f.Pais, &f.NoticiasCount,
		)
		if err != nil {
			continue
		}
		feeds = append(feeds, f)
	}

	totalPages := (total + perPage - 1) / perPage

	response := gin.H{
		"feeds":       feeds,
		"total":       total,
		"page":        page,
		"per_page":    perPage,
		"total_pages": totalPages,
	}

	// Cache the response (pass the struct; cache.Set marshals once)
	cache.Set(ctx, cacheKey, response, cache.TTLMedium)

	c.JSON(http.StatusOK, response)
}

func GetFeedByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid feed ID"})
		return
	}

	var f models.Feed
	err = db.GetPool().QueryRow(c.Request.Context(), `
		SELECT id, nombre, descripcion, url, categoria_id, pais_id, idioma, activo, fallos, last_fetch, traducido
		FROM feeds WHERE id = $1`, id).Scan(
		&f.ID, &f.Nombre, &f.Descripcion, &f.URL,
		&f.CategoriaID, &f.PaisID, &f.Idioma, &f.Activo, &f.Fallos, &f.LastFetched,
		&f.Traducido,
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

	// Invalidate feed list cache
	cache.InvalidateFeedList(c.Request.Context())

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

	// Invalidate feed cache
	cache.InvalidateFeed(c.Request.Context(), id)
	cache.InvalidateFeedList(c.Request.Context())

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "Feed updated successfully"})
}

func DeleteFeed(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid feed ID"})
		return
	}

	ctx := c.Request.Context()
	tx, err := db.GetPool().Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to delete feed", Message: err.Error()})
		return
	}
	defer tx.Rollback(ctx)

	// Las noticias cuelgan del feed por la FK (ON DELETE CASCADE), y en cascada
	// se van tambien traducciones, tags, embeddings y clusters. Sin esto el
	// feed se borraba y sus noticias quedaban huerfanas en la base.
	var noticias int64
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM noticias WHERE feed_id = $1`, id).Scan(&noticias); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to count feed news", Message: err.Error()})
		return
	}

	result, err := tx.Exec(ctx, "DELETE FROM feeds WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to delete feed", Message: err.Error()})
		return
	}
	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Feed not found"})
		return
	}

	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to delete feed", Message: err.Error()})
		return
	}

	// Invalidate feed cache
	cache.InvalidateFeed(ctx, id)
	cache.InvalidateFeedList(ctx)

	c.JSON(http.StatusOK, gin.H{
		"message":         "Feed deleted successfully",
		"noticias_deleted": noticias,
	})
}

// BulkDeleteFeeds borra varios canales de golpe, opcionalmente todos los que
// casan con una busqueda (p.ej. las Wikipedia de cambios recientes).
//
// body: { "ids": [1,2,3] } y/o { "q": "wikipedia", "confirmar": true }
//
// Exige "confirmar": true. Sin el, un DELETE accidental (o un curl mal
// escrito) borraria miles de canales y sus noticias: el confirm() del
// navegador no protege a quien use la API directamente.
func BulkDeleteFeeds(c *gin.Context) {
	var req struct {
		IDs       []int64 `json:"ids"`
		Query     string  `json:"q"`
		Confirmar bool    `json:"confirmar"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Message: err.Error()})
		return
	}

	ctx := c.Request.Context()
	tx, err := db.GetPool().Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to delete feeds", Message: err.Error()})
		return
	}
	defer tx.Rollback(ctx)

	// Seleccion de los feeds a borrar.
	var feeds []int64
	if len(req.IDs) > 0 {
		feeds = req.IDs
	}
	if q := strings.TrimSpace(req.Query); q != "" {
		rows, err := tx.Query(ctx, `
			SELECT id FROM feeds
			WHERE nombre ILIKE $1 OR url ILIKE $1 OR COALESCE(descripcion, '') ILIKE $1`,
			"%"+q+"%")
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to find feeds", Message: err.Error()})
			return
		}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err == nil {
				feeds = append(feeds, id)
			}
		}
		rows.Close()
	}
	if len(feeds) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "No feeds to delete (ids or q)"})
		return
	}

	// Cuantas noticias se van a llevar por delante (para informarlo y para
	// que el llamante pueda decidir).
	var noticias int64
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM noticias WHERE feed_id = ANY($1)`, feeds).Scan(&noticias); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to count news", Message: err.Error()})
		return
	}

	if !req.Confirmar {
		// Se responde con el impacto previsto y NO se borra nada.
		c.JSON(http.StatusPreconditionRequired, gin.H{
			"error":            "confirma la operacion",
			"message":          "Falta 'confirmar': true. Esta operacion borra las fuentes y sus noticias de forma irreversible.",
			"feeds_affected":   len(feeds),
			"noticias_affected": noticias,
		})
		return
	}

	result, err := tx.Exec(ctx, "DELETE FROM feeds WHERE id = ANY($1)", feeds)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to delete feeds", Message: err.Error()})
		return
	}

	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to delete feeds", Message: err.Error()})
		return
	}

	cache.InvalidateFeedList(ctx)

	c.JSON(http.StatusOK, gin.H{
		"message":          "Feeds deleted",
		"feeds_deleted":    result.RowsAffected(),
		"noticias_deleted": noticias,
	})
}

func DeleteInactiveFeeds(c *gin.Context) {
	ctx := c.Request.Context()

	// Optional country filter
	paisID := c.Query("pais_id")

	query := "DELETE FROM feeds WHERE activo = false"
	args := []interface{}{}

	if paisID != "" {
		query += " AND pais_id = $1"
		args = append(args, paisID)
	}

	result, err := db.GetPool().Exec(ctx, query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to delete inactive feeds", Message: err.Error()})
		return
	}

	// Invalidate feed list cache
	cache.InvalidateFeedList(ctx)

	c.JSON(http.StatusOK, gin.H{
		"message":        "Inactive feeds deleted",
		"deleted_count": result.RowsAffected(),
	})
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

	// Invalidate feed cache
	cache.InvalidateFeed(c.Request.Context(), id)
	cache.InvalidateFeedList(c.Request.Context())

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "Feed toggled successfully"})
}

// SetFeedTraducido marca (o desmarca) un canal como "ya traducido": sus
// noticias se insertan con lang = idioma destino y el pipeline no las
// procesa (ni langdetect ni traduccion).
func SetFeedTraducido(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid feed ID"})
		return
	}

	var req struct {
		Traducido *bool `json:"traducido"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Traducido == nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Message: "traducido (bool) es obligatorio"})
		return
	}

	result, err := db.GetPool().Exec(c.Request.Context(), `
		UPDATE feeds SET traducido = $1 WHERE id = $2`, *req.Traducido, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to update feed", Message: err.Error()})
		return
	}
	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Feed not found"})
		return
	}

	cache.InvalidateFeed(c.Request.Context(), id)
	cache.InvalidateFeedList(c.Request.Context())

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "Feed actualizado"})
}

// BulkSetFeedsTraducido marca (o desmarca) varios canales de una vez: es la
// forma practica de montar la lista de canales que ya estan en espanol.
func BulkSetFeedsTraducido(c *gin.Context) {
	var req struct {
		IDs        []int64 `json:"ids"`
		Traducido  bool    `json:"traducido"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Message: err.Error()})
		return
	}
	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "ids no puede estar vacio"})
		return
	}

	result, err := db.GetPool().Exec(c.Request.Context(), `
		UPDATE feeds SET traducido = $1 WHERE id = ANY($2)`, req.Traducido, req.IDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to update feeds", Message: err.Error()})
		return
	}

	cache.InvalidateFeedList(c.Request.Context())

	c.JSON(http.StatusOK, gin.H{
		"message": "Feeds actualizados",
		"updated": result.RowsAffected(),
	})
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

	// Invalidate feed cache
	cache.InvalidateFeed(c.Request.Context(), id)
	cache.InvalidateFeedList(c.Request.Context())

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
	records, err := reader.ReadAll()
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid CSV format"})
		return
	}
	if len(records) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Empty CSV file"})
		return
	}

	colIndex := func(header []string, names ...string) int {
		for i, h := range header {
			h = strings.ToLower(strings.TrimSpace(h))
			for _, n := range names {
				if h == n {
					return i
				}
			}
		}
		return -1
	}

	header := records[0]
	isHeader := colIndex(header, "url", "link", "feed", "nombre", "name", "titulo") != -1

	var dataRows [][]string
	iNombre, iDesc, iURL, iCat, iPais, iLang, iActivo, iFallos := -1, -1, -1, -1, -1, -1, -1, -1
	if isHeader {
		iNombre = colIndex(header, "nombre", "name", "titulo", "title")
		iDesc = colIndex(header, "descripcion", "description")
		iURL = colIndex(header, "url", "link", "feed")
		iCat = colIndex(header, "categoria_id", "category_id", "categoria", "category")
		iPais = colIndex(header, "pais_id", "country_id", "pais", "country")
		iLang = colIndex(header, "idioma", "language", "lang")
		iActivo = colIndex(header, "activo", "active", "enabled")
		iFallos = colIndex(header, "fallos", "failures", "fails")
		dataRows = records[1:]
	} else {
		dataRows = records
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
	committed := false
	defer func() {
		if !committed {
			tx.Rollback(context.Background())
		}
	}()

	field := func(record []string, idx int) string {
		if idx < 0 || idx >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[idx])
	}

	for _, record := range dataRows {
		if len(record) == 0 {
			skipped++
			continue
		}

		var nombre, url string
		if isHeader {
			nombre = field(record, iNombre)
			url = field(record, iURL)
		} else if len(record) == 1 {
			url = field(record, 0)
		} else if len(record) == 2 {
			a := field(record, 0)
			b := field(record, 1)
			if looksLikeURL(a) {
				url, nombre = a, b
			} else {
				nombre, url = a, b
			}
		} else {
			nombre = field(record, 1)
			url = field(record, 3)
		}

		if url == "" {
			skipped++
			continue
		}
		if nombre == "" {
			nombre = nombreFromURL(url)
		}

		var descripcion *string
		var categoriaID *int64
		var paisID *int64
		var idioma *string
		activo := true
		var fallos int64

		if isHeader {
			if d := field(record, iDesc); d != "" {
				descripcion = &d
			}
			if v := field(record, iCat); v != "" {
				if id, err := strconv.ParseInt(v, 10, 64); err == nil {
					categoriaID = &id
				}
			}
			if v := field(record, iPais); v != "" {
				if id, err := strconv.ParseInt(v, 10, 64); err == nil {
					paisID = &id
				}
			}
			if l := field(record, iLang); l != "" {
				if len(l) > 2 {
					l = l[:2]
				}
				idioma = &l
			}
			if a := field(record, iActivo); a != "" {
				activo = strings.ToLower(a) == "true"
			}
			if f := field(record, iFallos); f != "" {
				if v, err := strconv.ParseInt(f, 10, 64); err == nil {
					fallos = v
				}
			}
		} else {
			if d := field(record, 2); d != "" {
				descripcion = &d
			}
			if v := field(record, 4); v != "" {
				if id, err := strconv.ParseInt(v, 10, 64); err == nil {
					categoriaID = &id
				}
			}
			if v := field(record, 6); v != "" {
				if id, err := strconv.ParseInt(v, 10, 64); err == nil {
					paisID = &id
				}
			}
			if l := field(record, 8); l != "" {
				if len(l) > 2 {
					l = l[:2]
				}
				idioma = &l
			}
			if a := field(record, 9); a != "" {
				activo = strings.ToLower(a) == "true"
			}
			if f := field(record, 10); f != "" {
				if v, err := strconv.ParseInt(f, 10, 64); err == nil {
					fallos = v
				}
			}
		}

		if _, err := tx.Exec(context.Background(), "SAVEPOINT sp"); err != nil {
			failed++
			// No exponemos detalles del error de BD al cliente
			log.Printf("Import feed savepoint error for %s: %v", url, err)
			errors = append(errors, fmt.Sprintf("Error processing feed: savepoint failed"))
			continue
		}

		var existingID int64
		err = tx.QueryRow(context.Background(), "SELECT id FROM feeds WHERE url = $1", url).Scan(&existingID)
		if err == nil {
			_, err = tx.Exec(context.Background(), `
				UPDATE feeds SET nombre=$1, descripcion=$2, categoria_id=$3, pais_id=$4, idioma=$5, activo=$6, fallos=$7
				WHERE id=$8`,
				nombre, descripcion, categoriaID, paisID, idioma, activo, fallos, existingID,
			)
		} else {
			_, err = tx.Exec(context.Background(), `
				INSERT INTO feeds (nombre, descripcion, url, categoria_id, pais_id, idioma, activo, fallos)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
				nombre, descripcion, url, categoriaID, paisID, idioma, activo, fallos,
			)
		}

		if err != nil {
			if _, rbErr := tx.Exec(context.Background(), "ROLLBACK TO SAVEPOINT sp"); rbErr != nil {
				log.Printf("Import feed rollback error for %s: %v", url, rbErr)
				errors = append(errors, "Error processing feed: rollback failed")
			} else {
				log.Printf("Import feed upsert error for %s: %v", url, err)
				errors = append(errors, "Error processing feed: upsert failed")
			}
			failed++
			continue
		}

		if _, err := tx.Exec(context.Background(), "RELEASE SAVEPOINT sp"); err != nil {
			failed++
			log.Printf("Import feed release savepoint error for %s: %v", url, err)
			errors = append(errors, "Error processing feed: release savepoint failed")
			continue
		}

		imported++
	}

	if err := tx.Commit(context.Background()); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to commit transaction", Message: err.Error()})
		return
	}
	committed = true

	// Invalidate feed list cache after import
	cache.InvalidateFeedList(context.Background())

	c.JSON(http.StatusOK, gin.H{
		"imported": imported,
		"skipped":  skipped,
		"failed":   failed,
		"errors":   errors,  // errors contiene solo códigos, no detalles de BD
		"message":  fmt.Sprintf("Import completed. Imported: %d, Skipped: %d, Failed: %d", imported, skipped, failed),
	})
}

func looksLikeURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://www.") || strings.HasPrefix(s, "https://www.") || strings.HasPrefix(s, "www.") || strings.HasPrefix(s, "feed://")
}

func nombreFromURL(u string) string {
	u = strings.TrimSpace(u)
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		u = "https://" + u
	}
	parsed, err := url.Parse(u)
	if err != nil || parsed.Host == "" {
		return u
	}
	return strings.TrimPrefix(parsed.Host, "www.")
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
