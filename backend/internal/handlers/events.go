package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rss2/backend/internal/db"
	"github.com/rss2/backend/internal/models"
)

// ============================================================================
// Event Cluster Types
// ============================================================================

type EventCluster struct {
	ID                int64      `json:"id"`
	ClusterHash       string     `json:"cluster_hash"`
	TituloTemplate    string     `json:"titulo_template"`
	PrimerNoticiaID   string     `json:"primer_noticia_id"`
	FechaInicio       *time.Time `json:"fecha_inicio"`
	FechaFin          *time.Time `json:"fecha_fin"`
	Popularidad       int        `json:"popularidad"`
	Importancia       string     `json:"importancia"`
	CreatedAt         *time.Time `json:"created_at"`
	UpdatedAt         *time.Time `json:"updated_at"`
}

type EventClusterDetail struct {
	EventCluster
	Noticias []EventNews `json:"noticias"`
}

type EventNews struct {
	ID                string    `json:"id"`
	Titulo            string    `json:"titulo"`
	Resumen           string    `json:"resumen"`
	URL               string    `json:"url"`
	Fecha             *time.Time `json:"fecha"`
	FuenteNombre      string    `json:"fuente_nombre"`
	Similitud         float64   `json:"similitud"`
	EsPrincipal       bool      `json:"es_principal"`
}

type EventListResponse struct {
	Events     []EventCluster `json:"events"`
	Total      int            `json:"total"`
	Page       int            `json:"page"`
	PerPage    int            `json:"per_page"`
	TotalPages int            `json:"total_pages"`
}

// ============================================================================
// GET /api/events - Lista de clusters de eventos (paginados)
// ============================================================================

// GetEvents returns paginated event clusters
// @Summary List event clusters
// @Description Returns paginated event clusters with optional filters
// @Tags events
// @Produce json
// @Param page query int false "Page number" default(1) minimum(1)
// @Param per_page query int false "Items per page" default(20) minimum(1) maximum(100)
// @Param importancia query string false "Filter by importance (breaking, major, normal, minor)"
// @Param days query int false "Filter by recency (days)" default(7)
// @Success 200 {object} EventListResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /events [get]
func GetEvents(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	page, perPage = validatePageParams(page, perPage, 20, 100)
	offset := (page - 1) * perPage

	importancia := c.Query("importancia")
	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
	if days < 1 {
		days = 7
	}
	if days > 30 {
		days = 30
	}

	where := "1=1"
	args := []interface{}{}
	argNum := 1

	if importancia != "" {
		where += " AND e.importancia = $" + strconv.Itoa(argNum)
		args = append(args, importancia)
		argNum++
	}

	where += " AND e.fecha_inicio >= NOW() - INTERVAL '" + strconv.Itoa(days) + " days'"

	// Count total
	var total int
	countQuery := "SELECT COUNT(*) FROM event_clusters e WHERE " + where
	if err := db.GetPool().QueryRow(c.Request.Context(), countQuery, args...).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to count events", Message: err.Error()})
		return
	}

	// Fetch events
	args = append(args, perPage, offset)
	query := `
		SELECT e.id, e.cluster_hash, COALESCE(e.titulo_template, ''), 
		       COALESCE(e.primer_noticia_id, ''), e.fecha_inicio, e.fecha_fin,
		       e.popularidad, e.importancia, e.created_at, e.updated_at
		FROM event_clusters e
		WHERE ` + where + `
		ORDER BY e.fecha_inicio DESC
		LIMIT $` + strconv.Itoa(argNum) + " OFFSET $" + strconv.Itoa(argNum+1)

	rows, err := db.GetPool().Query(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch events", Message: err.Error()})
		return
	}
	defer rows.Close()

	var events []EventCluster
	for rows.Next() {
		var e EventCluster
		if err := rows.Scan(&e.ID, &e.ClusterHash, &e.TituloTemplate, &e.PrimerNoticiaID,
			&e.FechaInicio, &e.FechaFin, &e.Popularidad, &e.Importancia, &e.CreatedAt, &e.UpdatedAt); err != nil {
			continue
		}
		events = append(events, e)
	}

	if events == nil {
		events = []EventCluster{}
	}

	totalPages := (total + perPage - 1) / perPage

	c.JSON(http.StatusOK, EventListResponse{
		Events:     events,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	})
}

// ============================================================================
// GET /api/events/breaking - Eventos breaking del día
// ============================================================================

// GetBreakingEvents returns breaking events from today
// @Summary Get breaking events
// @Description Returns all breaking events from today
// @Tags events
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} models.ErrorResponse
// @Router /events/breaking [get]
func GetBreakingEvents(c *gin.Context) {
	query := `
		SELECT e.id, e.cluster_hash, COALESCE(e.titulo_template, ''), 
		       COALESCE(e.primer_noticia_id, ''), e.fecha_inicio, e.fecha_fin,
		       e.popularidad, e.importancia, e.created_at, e.updated_at
		FROM event_clusters e
		WHERE e.importancia = 'breaking'
		  AND e.fecha_inicio >= CURRENT_DATE
		ORDER BY e.popularidad DESC, e.fecha_inicio DESC
		LIMIT 50
	`

	rows, err := db.GetPool().Query(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch breaking events", Message: err.Error()})
		return
	}
	defer rows.Close()

	var events []EventCluster
	for rows.Next() {
		var e EventCluster
		if err := rows.Scan(&e.ID, &e.ClusterHash, &e.TituloTemplate, &e.PrimerNoticiaID,
			&e.FechaInicio, &e.FechaFin, &e.Popularidad, &e.Importancia, &e.CreatedAt, &e.UpdatedAt); err != nil {
			continue
		}
		events = append(events, e)
	}

	if events == nil {
		events = []EventCluster{}
	}

	c.JSON(http.StatusOK, gin.H{
		"events": events,
		"total":  len(events),
	})
}

// ============================================================================
// GET /api/events/:id - Detalle de un cluster con todas sus noticias
// ============================================================================

// GetEventByID returns a single event cluster with all its news
// @Summary Get event by ID
// @Description Returns a single event cluster with all associated news
// @Tags events
// @Produce json
// @Param id path int true "Event Cluster ID"
// @Success 200 {object} EventClusterDetail
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /events/{id} [get]
func GetEventByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid event ID"})
		return
	}

	// Fetch event cluster
	var e EventCluster
	err = db.GetPool().QueryRow(c.Request.Context(), `
		SELECT e.id, e.cluster_hash, COALESCE(e.titulo_template, ''), 
		       COALESCE(e.primer_noticia_id, ''), e.fecha_inicio, e.fecha_fin,
		       e.popularidad, e.importancia, e.created_at, e.updated_at
		FROM event_clusters e
		WHERE e.id = $1
	`, id).Scan(&e.ID, &e.ClusterHash, &e.TituloTemplate, &e.PrimerNoticiaID,
		&e.FechaInicio, &e.FechaFin, &e.Popularidad, &e.Importancia, &e.CreatedAt, &e.UpdatedAt)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Event not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch event", Message: err.Error()})
		return
	}

	// Fetch news in this cluster
	newsRows, err := db.GetPool().Query(c.Request.Context(), `
		SELECT n.id, COALESCE(n.titulo, ''), COALESCE(n.resumen, ''), 
		       n.url, n.fecha, COALESCE(n.fuente_nombre, ''),
		       nc.similitud, nc.es_principal
		FROM noticia_clusters nc
		JOIN noticias n ON nc.noticia_id = n.id
		WHERE nc.cluster_id = $1
		ORDER BY nc.es_principal DESC, nc.similitud DESC, n.fecha DESC
	`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch event news", Message: err.Error()})
		return
	}
	defer newsRows.Close()

	var noticias []EventNews
	for newsRows.Next() {
		var n EventNews
		if err := newsRows.Scan(&n.ID, &n.Titulo, &n.Resumen, &n.URL, &n.Fecha,
			&n.FuenteNombre, &n.Similitud, &n.EsPrincipal); err != nil {
			continue
		}
		noticias = append(noticias, n)
	}

	if noticias == nil {
		noticias = []EventNews{}
	}

	detail := EventClusterDetail{
		EventCluster: e,
		Noticias:     noticias,
	}

	c.JSON(http.StatusOK, detail)
}
