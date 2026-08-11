package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rss2/backend/internal/db"
	"github.com/rss2/backend/internal/models"
)

type NewsResponse struct {
	ID                string     `json:"id"`
	Titulo            string     `json:"titulo"`
	Resumen           string     `json:"resumen"`
	URL               string     `json:"url"`
	Fecha             *time.Time `json:"fecha"`
	ImagenURL         *string    `json:"imagen_url"`
	CategoriaID       *int64     `json:"categoria_id"`
	PaisID            *int64     `json:"pais_id"`
	FuenteNombre      string     `json:"fuente_nombre"`
	TitleTranslated   *string    `json:"title_translated"`
	SummaryTranslated *string    `json:"summary_translated"`
	LangTranslated    *string    `json:"lang_translated"`
	Entities          []Entity   `json:"entities,omitempty"`
}

func GetNews(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "30"))
	query := c.Query("q")
	categoryID := c.Query("category_id")
	countryID := c.Query("country_id")
	translatedOnly := c.Query("translated_only") == "true"

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 30
	}

	offset := (page - 1) * perPage

	where := "1=1"
	args := []interface{}{}
	argNum := 1

	if query != "" {
		if translatedOnly {
			// With translated_only, search the translated fields too so users can
			// find translated news by the Spanish (translated) wording.
			where += fmt.Sprintf(" AND (n.titulo ILIKE $%d OR n.resumen ILIKE $%d OR t.titulo_trad ILIKE $%d OR t.resumen_trad ILIKE $%d)", argNum, argNum, argNum, argNum)
		} else {
			where += fmt.Sprintf(" AND (n.titulo ILIKE $%d OR n.resumen ILIKE $%d)", argNum, argNum)
		}
		args = append(args, "%"+query+"%")
		argNum++
	}
	if categoryID != "" {
		where += fmt.Sprintf(" AND n.categoria_id = $%d", argNum)
		args = append(args, categoryID)
		argNum++
	}
	if countryID != "" {
		where += fmt.Sprintf(" AND n.pais_id = $%d", argNum)
		args = append(args, countryID)
		argNum++
	}
	if translatedOnly {
		where += " AND t.status = 'done' AND t.titulo_trad IS NOT NULL AND t.titulo_trad != n.titulo"
	}

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM noticias n LEFT JOIN traducciones t ON t.noticia_id = n.id AND t.lang_to = 'es' WHERE %s", where)
	err := db.GetPool().QueryRow(c.Request.Context(), countQuery, args...).Scan(&total)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to count news", Message: err.Error()})
		return
	}

	if total == 0 {
		c.JSON(http.StatusOK, models.NewsListResponse{
			News:       []models.NewsWithTranslations{},
			Total:      0,
			Page:       page,
			PerPage:    perPage,
			TotalPages: 0,
		})
		return
	}

	sqlQuery := fmt.Sprintf(`
		SELECT n.id, n.titulo, COALESCE(n.resumen, ''), n.url, n.fecha, n.imagen_url, 
		       n.categoria_id, n.pais_id, n.fuente_nombre,
		       t.titulo_trad,
		       t.resumen_trad,
		       t.lang_to as lang_trad
		FROM noticias n
		LEFT JOIN traducciones t ON t.noticia_id = n.id AND t.lang_to = 'es'
		WHERE %s
		ORDER BY n.fecha DESC LIMIT $%d OFFSET $%d
	`, where, argNum, argNum+1)

	args = append(args, perPage, offset)

	rows, err := db.GetPool().Query(c.Request.Context(), sqlQuery, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch news", Message: err.Error()})
		return
	}
	defer rows.Close()

	var newsList []NewsResponse
	for rows.Next() {
		var n NewsResponse
		var imagenURL, fuenteNombre *string
		var categoriaID, paisID *int32

		err := rows.Scan(
			&n.ID, &n.Titulo, &n.Resumen, &n.URL, &n.Fecha, &imagenURL,
			&categoriaID, &paisID, &fuenteNombre,
			&n.TitleTranslated, &n.SummaryTranslated, &n.LangTranslated,
		)
		if err != nil {
			continue
		}
		if imagenURL != nil {
			n.ImagenURL = imagenURL
		}
		if fuenteNombre != nil {
			n.FuenteNombre = *fuenteNombre
		}
		if categoriaID != nil {
			catID := int64(*categoriaID)
			n.CategoriaID = &catID
		}
		if paisID != nil {
			pID := int64(*paisID)
			n.PaisID = &pID
		}
		newsList = append(newsList, n)
	}

	totalPages := (total + perPage - 1) / perPage

	c.JSON(http.StatusOK, gin.H{
		"news":        newsList,
		"total":       total,
		"page":        page,
		"per_page":    perPage,
		"total_pages": totalPages,
	})
}

// GetEntityNews returns the (translated) news that mention a given entity/alias value.
func GetEntityNews(c *gin.Context) {
	valor := c.Query("valor")
	tipo := c.DefaultQuery("tipo", "persona")
	pageStr := c.DefaultQuery("page", "1")
	perPageStr := c.DefaultQuery("per_page", "20")

	page, _ := strconv.Atoi(pageStr)
	perPage, _ := strconv.Atoi(perPageStr)
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 50 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	daysStr := c.Query("days")
	days := 0
	if daysStr != "" {
		days, _ = strconv.Atoi(daysStr)
		if days < 1 {
			days = 0
		}
	}
	today := c.Query("today") == "true"
	offsetStr := c.Query("day_offset")
	dayOffset := 0
	useDayOffset := false
	if offsetStr != "" {
		dayOffset, _ = strconv.Atoi(offsetStr)
		if dayOffset < 0 {
			dayOffset = 0
		}
		useDayOffset = true
	}

	params := []interface{}{valor, tipo}
	next := 3
	timeCond := ""
	switch {
	case useDayOffset:
		// Día exacto: desde el inicio del día (hoy - offset) hasta el inicio del día siguiente
		timeCond = fmt.Sprintf(
			" AND n.fecha >= (CURRENT_DATE - %d)::timestamp AND n.fecha < (CURRENT_DATE - %d + 1)::timestamp",
			dayOffset, dayOffset)
	case today:
		timeCond = " AND n.fecha >= date_trunc('day', NOW())"
	case days > 0:
		timeCond = fmt.Sprintf(" AND n.fecha >= NOW() - make_interval(days => $%d)", next)
		params = append(params, days)
		next++
	}

	base := fmt.Sprintf(`
		FROM tags_noticia tn
		JOIN tags t ON tn.tag_id = t.id
		JOIN traducciones tr ON tn.traduccion_id = tr.id
		JOIN noticias n ON tr.noticia_id = n.id
		LEFT JOIN entity_aliases ea ON LOWER(ea.alias) = LOWER(t.valor) AND ea.tipo = t.tipo
		WHERE LOWER(COALESCE(ea.canonical_name, t.valor)) = LOWER($1) AND t.tipo = $2%s`, timeCond)

	var total int
	if err := db.GetPool().QueryRow(c.Request.Context(),
		fmt.Sprintf("SELECT COUNT(DISTINCT tn.noticia_id) %s", base), params...).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to count entity news", Message: err.Error()})
		return
	}

	query := fmt.Sprintf(`
		SELECT DISTINCT n.id, n.titulo, COALESCE(n.resumen,''), n.url, n.fecha, n.imagen_url,
		       n.fuente_nombre, tr.titulo_trad, tr.resumen_trad
		%s
		ORDER BY n.fecha DESC
		LIMIT $%d OFFSET $%d`, base, next, next+1)

	rows, err := db.GetPool().Query(c.Request.Context(), query, append(params, perPage, offset)...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to get entity news", Message: err.Error()})
		return
	}
	defer rows.Close()

	var news []NewsResponse
	for rows.Next() {
		var n NewsResponse
		var img *string
		if err := rows.Scan(&n.ID, &n.Titulo, &n.Resumen, &n.URL, &n.Fecha, &img,
			&n.FuenteNombre, &n.TitleTranslated, &n.SummaryTranslated); err != nil {
			continue
		}
		if img != nil {
			n.ImagenURL = img
		}
		news = append(news, n)
	}
	if news == nil {
		news = []NewsResponse{}
	}

	totalPages := (total + perPage - 1) / perPage

	c.JSON(http.StatusOK, gin.H{
		"news":        news,
		"total":       total,
		"page":        page,
		"per_page":    perPage,
		"total_pages": totalPages,
	})
}

func GetNewsByID(c *gin.Context) {
	id := c.Param("id")

	sqlQuery := `
		SELECT n.id, n.titulo, COALESCE(n.resumen, ''), n.url, n.fecha, n.imagen_url, 
		       n.categoria_id, n.pais_id, n.fuente_nombre,
		       t.titulo_trad,
		       t.resumen_trad,
		       t.lang_to as lang_trad
		FROM noticias n
		LEFT JOIN traducciones t ON t.noticia_id = n.id AND t.lang_to = 'es'
		WHERE n.id = $1`

	var n NewsResponse
	var imagenURL, fuenteNombre *string
	var categoriaID, paisID *int32

	err := db.GetPool().QueryRow(c.Request.Context(), sqlQuery, id).Scan(
		&n.ID, &n.Titulo, &n.Resumen, &n.URL, &n.Fecha, &imagenURL,
		&categoriaID, &paisID, &fuenteNombre,
		&n.TitleTranslated, &n.SummaryTranslated, &n.LangTranslated,
	)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "News not found"})
		return
	}

	if imagenURL != nil {
		n.ImagenURL = imagenURL
	}
	if fuenteNombre != nil {
		n.FuenteNombre = *fuenteNombre
	}
	if categoriaID != nil {
		catID := int64(*categoriaID)
		n.CategoriaID = &catID
	}
	if paisID != nil {
		pID := int64(*paisID)
		n.PaisID = &pID
	}

	// Fetch entities for this news
	entitiesQuery := `
		SELECT t.valor, t.tipo, 1 as cnt, t.wiki_summary, t.wiki_url, t.image_path
		FROM tags_noticia tn
		JOIN tags t ON tn.tag_id = t.id
		JOIN traducciones tr ON tn.traduccion_id = tr.id
		WHERE tr.noticia_id = $1 AND t.tipo IN ('persona', 'organizacion')
	`
	rows, err := db.GetPool().Query(c.Request.Context(), entitiesQuery, id)
	var entities []Entity
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var e Entity
			if err := rows.Scan(&e.Valor, &e.Tipo, &e.Count, &e.WikiSummary, &e.WikiURL, &e.ImagePath); err == nil {
				entities = append(entities, e)
			}
		}
	}
	if entities == nil {
		entities = []Entity{}
	}
	n.Entities = entities

	c.JSON(http.StatusOK, n)
}

func DeleteNews(c *gin.Context) {
	id := c.Param("id")

	result, err := db.GetPool().Exec(c.Request.Context(), "DELETE FROM noticias WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to delete news", Message: err.Error()})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "News not found"})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "News deleted successfully"})
}

type Entity struct {
	Valor       string  `json:"valor"`
	Tipo        string  `json:"tipo"`
	Count       int     `json:"count"`
	WikiSummary *string `json:"wiki_summary"`
	WikiURL     *string `json:"wiki_url"`
	ImagePath   *string `json:"image_path"`
}

type EntityListResponse struct {
	Entities   []Entity `json:"entities"`
	Total      int      `json:"total"`
	Page       int      `json:"page"`
	PerPage    int      `json:"per_page"`
	TotalPages int      `json:"total_pages"`
}

func GetEntities(c *gin.Context) {
	countryID := c.Query("country_id")
	categoryID := c.Query("category_id")
	entityType := c.DefaultQuery("tipo", "persona")
	
	q := c.Query("q")
	
	pageStr := c.DefaultQuery("page", "1")
	perPageStr := c.DefaultQuery("per_page", "50")

	page, _ := strconv.Atoi(pageStr)
	perPage, _ := strconv.Atoi(perPageStr)

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}

	offset := (page - 1) * perPage

	where := "t.tipo = $1"
	args := []interface{}{entityType}

	if countryID != "" {
		where += fmt.Sprintf(" AND n.pais_id = $%d", len(args)+1)
		args = append(args, countryID)
	}

	if categoryID != "" {
		where += fmt.Sprintf(" AND n.categoria_id = $%d", len(args)+1)
		args = append(args, categoryID)
	}
	
	if q != "" {
		where += fmt.Sprintf(" AND COALESCE(ea.canonical_name, t.valor) ILIKE $%d", len(args)+1)
		args = append(args, "%"+q+"%")
	}

	// 1. Get the total count of distinct canonical entities matching the filter
	countQuery := fmt.Sprintf(`
		SELECT COUNT(DISTINCT COALESCE(ea.canonical_name, t.valor))
		FROM tags_noticia tn
		JOIN tags t ON tn.tag_id = t.id
		JOIN traducciones tr ON tn.traduccion_id = tr.id
		JOIN noticias n ON tr.noticia_id = n.id
		LEFT JOIN entity_aliases ea ON LOWER(ea.alias) = LOWER(t.valor) AND ea.tipo = t.tipo
		WHERE %s
	`, where)

	var total int
	err := db.GetPool().QueryRow(c.Request.Context(), countQuery, args...).Scan(&total)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to get entities count", Message: err.Error()})
		return
	}

	if total == 0 {
		c.JSON(http.StatusOK, EntityListResponse{
			Entities:   []Entity{},
			Total:      0,
			Page:       page,
			PerPage:    perPage,
			TotalPages: 0,
		})
		return
	}

	// 2. Fetch the paginated entities
	args = append(args, perPage, offset)
	query := fmt.Sprintf(`
		SELECT COALESCE(ea.canonical_name, t.valor) as valor, t.tipo, COUNT(*)::int as cnt,
		       MAX(t.wiki_summary), MAX(t.wiki_url), MAX(t.image_path)
		FROM tags_noticia tn
		JOIN tags t ON tn.tag_id = t.id
		JOIN traducciones tr ON tn.traduccion_id = tr.id
		JOIN noticias n ON tr.noticia_id = n.id
		LEFT JOIN entity_aliases ea ON LOWER(ea.alias) = LOWER(t.valor) AND ea.tipo = t.tipo
		WHERE %s
		GROUP BY COALESCE(ea.canonical_name, t.valor), t.tipo
		ORDER BY cnt DESC
		LIMIT $%d OFFSET $%d
	`, where, len(args)-1, len(args))

	rows, err := db.GetPool().Query(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to get entities", Message: err.Error()})
		return
	}
	defer rows.Close()

	var entities []Entity
	for rows.Next() {
		var e Entity
		if err := rows.Scan(&e.Valor, &e.Tipo, &e.Count, &e.WikiSummary, &e.WikiURL, &e.ImagePath); err != nil {
			continue
		}
		entities = append(entities, e)
	}

	if entities == nil {
		entities = []Entity{}
	}

	totalPages := (total + perPage - 1) / perPage

	c.JSON(http.StatusOK, EntityListResponse{
		Entities:   entities,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	})
}
