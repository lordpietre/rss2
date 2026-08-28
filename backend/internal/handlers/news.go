package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rss2/backend/internal/config"
	"github.com/rss2/backend/internal/db"
	"github.com/rss2/backend/internal/models"
)

// getTargetLang returns the target language for translations, defaulting to config DefaultLang
func getTargetLang(c *gin.Context) string {
	lang := c.Query("lang")
	if lang == "" {
		cfg := config.Load()
		lang = cfg.DefaultLang
	}
	return lang
}

// GetNews returns paginated news with optional filters
// @Summary List news
// @Description Returns paginated news with optional filtering by query, category, country, and translation status
// @Tags news
// @Produce json
// @Param page query int false "Page number" default(1) minimum(1)
// @Param per_page query int false "Items per page" default(30) minimum(1) maximum(100)
// @Param q query string false "Search query"
// @Param category_id query string false "Category ID filter"
// @Param country_id query string false "Country ID filter"
// @Param lang query string false "Target translation language" default(es)
// @Param translated_only query string false "Only show translated news" default("false")
// @Success 200 {object} map[string]interface{} "news, total, page, per_page, total_pages"
// @Failure 500 {object} models.ErrorResponse
// @Router /news [get]
func GetNews(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "30"))
	page, perPage = validatePageParams(page, perPage, 30, 100)
	query := c.Query("q")
	categoryID := c.Query("category_id")
	countryID := c.Query("country_id")
	translatedOnly := c.Query("translated_only") == "true"
	targetLang := getTargetLang(c)

	offset := (page - 1) * perPage

	where := "1=1"
	args := []interface{}{}
	argNum := 1

	if query != "" {
		if translatedOnly {
			// With translated_only, search the translated fields too so users can
			// find translated news by the target language wording.
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
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM noticias n LEFT JOIN traducciones t ON t.noticia_id = n.id AND t.lang_to = $%d WHERE %s", argNum, where)
	args = append(args, targetLang)
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

	sqlQuery := `
		SELECT n.id, n.titulo, COALESCE(n.resumen, ''), n.contenido, n.url, n.fecha, n.imagen_url, 
		       n.categoria_id, n.pais_id, n.fuente_nombre, n.lang,
		       t.titulo_trad,
		       t.resumen_trad,
		       t.lang_to as lang_trad
		FROM noticias n
		LEFT JOIN traducciones t ON t.noticia_id = n.id AND t.lang_to = $` + strconv.Itoa(argNum) + `
		WHERE ` + where + `
		ORDER BY n.fecha DESC LIMIT $` + strconv.Itoa(argNum+1) + ` OFFSET $` + strconv.Itoa(argNum+2)

	args = append(args, targetLang, perPage, offset)

	rows, err := db.GetPool().Query(c.Request.Context(), sqlQuery, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch news", Message: err.Error()})
		return
	}
	defer rows.Close()

	var newsList []models.NewsWithTranslations
	for rows.Next() {
		var n models.NewsWithTranslations
		var imagenURL, fuenteNombre *string
		var categoriaID, paisID *int32
		var lang string

		err := rows.Scan(
			&n.ID, &n.Titulo, &n.Resumen, &n.Contenido, &n.URL, &n.Fecha, &imagenURL,
			&categoriaID, &paisID, &fuenteNombre, &lang,
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
			n.CategoryID = &catID
		}
		if paisID != nil {
			pID := int64(*paisID)
			n.CountryID = &pID
		}
		n.Lang = lang
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
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	page, perPage = validatePageParams(page, perPage, 20, 50)
	offset := (page - 1) * perPage
	targetLang := getTargetLang(c)

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

	params := []interface{}{valor, tipo, targetLang}
	next := 4
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
		JOIN traducciones tr ON tn.traduccion_id = tr.id AND tr.lang_to = $3
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
		SELECT DISTINCT n.id, n.titulo, COALESCE(n.resumen,''), n.contenido, n.url, n.fecha, n.imagen_url,
		       n.fuente_nombre, n.lang, tr.titulo_trad, tr.resumen_trad
		%s
		ORDER BY n.fecha DESC
		LIMIT $%d OFFSET $%d`, base, next, next+1)

	rows, err := db.GetPool().Query(c.Request.Context(), query, append(params, perPage, offset)...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to get entity news", Message: err.Error()})
		return
	}
	defer rows.Close()

	var news []models.NewsWithTranslations
	for rows.Next() {
		var n models.NewsWithTranslations
		var img *string
		if err := rows.Scan(&n.ID, &n.Titulo, &n.Resumen, &n.Contenido, &n.URL, &n.Fecha, &img,
			&n.FuenteNombre, &n.Lang, &n.TitleTranslated, &n.SummaryTranslated); err != nil {
			continue
		}
		if img != nil {
			n.ImagenURL = img
		}
		news = append(news, n)
	}
	if news == nil {
		news = []models.NewsWithTranslations{}
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

// GetNewsByID returns a single news item with entities
// @Summary Get news by ID
// @Description Returns a single news item with translations and associated entities
// @Tags news
// @Produce json
// @Param id path string true "News ID"
// @Param lang query string false "Target translation language" default(es)
// @Success 200 {object} models.NewsWithTranslations
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /news/{id} [get]
func GetNewsByID(c *gin.Context) {
	id := c.Param("id")
	targetLang := getTargetLang(c)

	// Combined query to fetch news with entities in one round trip
	sqlQuery := `
		SELECT 
			n.id, n.titulo, COALESCE(n.resumen, ''), n.contenido, n.url, n.fecha, n.imagen_url, 
		       n.categoria_id, n.pais_id, n.fuente_nombre, n.lang,
		       t.titulo_trad,
		       t.resumen_trad,
		       t.lang_to as lang_trad,
		       json_agg(
		           json_build_object(
		               'valor', ent.valor,
		               'tipo', ent.tipo,
		               'count', ent.cnt,
		               'wiki_summary', ent.wiki_summary,
		               'wiki_url', ent.wiki_url,
		               'image_path', ent.image_path
		           )
		       ) FILTER (WHERE ent.valor IS NOT NULL) as entities
		FROM noticias n
		LEFT JOIN traducciones t ON t.noticia_id = n.id AND t.lang_to = $1
		LEFT JOIN (
			SELECT tn.noticia_id, t.valor, t.tipo, 1 as cnt, t.wiki_summary, t.wiki_url, t.image_path
			FROM tags_noticia tn
			JOIN tags t ON tn.tag_id = t.id
			JOIN traducciones tr ON tn.traduccion_id = tr.id
			WHERE t.tipo IN ('persona', 'organizacion')
		) ent ON ent.noticia_id = n.id
		WHERE n.id = $2
		GROUP BY n.id, n.titulo, n.resumen, n.contenido, n.url, n.fecha, n.imagen_url, 
		         n.categoria_id, n.pais_id, n.fuente_nombre, n.lang,
		         t.titulo_trad, t.resumen_trad, t.lang_to
	`

	var n models.NewsWithTranslations
	var imagenURL, fuenteNombre *string
	var categoriaID, paisID *int32
	var lang string
	var entitiesJSON *string

	err := db.GetPool().QueryRow(c.Request.Context(), sqlQuery, targetLang, id).Scan(
		&n.ID, &n.Titulo, &n.Resumen, &n.Contenido, &n.URL, &n.Fecha, &imagenURL,
		&categoriaID, &paisID, &fuenteNombre, &lang,
		&n.TitleTranslated, &n.SummaryTranslated, &n.LangTranslated,
		&entitiesJSON,
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
		n.CategoryID = &catID
	}
	if paisID != nil {
		pID := int64(*paisID)
		n.CountryID = &pID
	}
	n.Lang = lang

	// Parse entities from JSON
	if entitiesJSON != nil && *entitiesJSON != "null" {
		var entities []models.Entity
		if err := json.Unmarshal([]byte(*entitiesJSON), &entities); err == nil {
			n.Entities = entities
		}
	} else {
		n.Entities = []models.Entity{}
	}

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

// GetEntities returns paginated entities with optional filters
// @Summary List entities
// @Description Returns paginated entities (personas, organizations, locations) with optional filters
// @Tags entities
// @Produce json
// @Param tipo query string false "Entity type" default("persona") Enums(persona, organizacion, lugar, tema)
// @Param page query int false "Page number" default(1) minimum(1)
// @Param per_page query int false "Items per page" default(50) minimum(1) maximum(100)
// @Param country_id query string false "Country ID filter"
// @Param category_id query string false "Category ID filter"
// @Param q query string false "Search query"
// @Success 200 {object} models.EntityListResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /entities [get]
func GetEntities(c *gin.Context) {
	countryID := c.Query("country_id")
	categoryID := c.Query("category_id")
	entityType := c.DefaultQuery("tipo", "persona")
	
	q := c.Query("q")
	
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "50"))
	page, perPage = validatePageParams(page, perPage, 50, 100)

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
		c.JSON(http.StatusOK, models.EntityListResponse{
			Entities:   []models.Entity{},
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

	var entities []models.Entity
	for rows.Next() {
		var e models.Entity
		if err := rows.Scan(&e.Valor, &e.Tipo, &e.Count, &e.WikiSummary, &e.WikiURL, &e.ImagePath); err != nil {
			continue
		}
		entities = append(entities, e)
	}

	if entities == nil {
		entities = []models.Entity{}
	}

	totalPages := (total + perPage - 1) / perPage

	c.JSON(http.StatusOK, models.EntityListResponse{
		Entities:   entities,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	})
}

func GetEntityMentions(c *gin.Context) {
	valuesRaw := c.DefaultQuery("values", "")
	days := 30
	if v := c.Query("days"); v != "" {
		if d, err := strconv.Atoi(v); err == nil && d >= 1 && d <= 365 {
			days = d
		}
	}

	var values []string
	for _, v := range strings.Split(valuesRaw, ",") {
		v = strings.TrimSpace(v)
		if v != "" {
			values = append(values, v)
		}
	}
	if len(values) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "values is required (comma separated)"})
		return
	}
	if len(values) > 20 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Too many values (max 20)"})
		return
	}

	start := time.Now().AddDate(0, 0, -(days - 1)).Truncate(24 * time.Hour)

	series := make([]models.MentionSeries, 0, len(values))
	for _, valor := range values {
		s, err := buildMentionSeries(c.Request.Context(), valor, days)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to get mentions", Message: err.Error()})
			return
		}

		m := map[string]int{}
		for _, p := range s.Data {
			m[p.Fecha] = p.Count
		}
		data := make([]models.MentionPoint, 0, days)
		for i := 0; i < days; i++ {
			fecha := start.AddDate(0, 0, i).Format("2006-01-02")
			data = append(data, models.MentionPoint{Fecha: fecha, Count: m[fecha]})
		}
		s.Data = data
		s.Count = 0
		for _, p := range s.Data {
			s.Count += p.Count
		}
		if s.Tipo == "" {
			s.Tipo = "desconocido"
		}
		series = append(series, *s)
	}

	c.JSON(http.StatusOK, models.MentionsResponse{Days: days, Series: series})
}

func buildMentionSeries(ctx context.Context, valor string, days int) (*models.MentionSeries, error) {
	s := &models.MentionSeries{Valor: valor}

	var tagIDs []int64
	rows, err := db.GetPool().Query(ctx, `
		SELECT DISTINCT t.id, t.tipo
		FROM tags t
		LEFT JOIN entity_aliases ea ON ea.tipo = t.tipo AND LOWER(ea.alias) = LOWER(t.valor)
		WHERE LOWER(t.valor) = LOWER($1)
		   OR LOWER(COALESCE(ea.canonical_name, t.valor)) = LOWER($1)
	`, valor)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		var tipo string
		if err := rows.Scan(&id, &tipo); err != nil {
			continue
		}
		tagIDs = append(tagIDs, id)
		if s.Tipo == "" {
			s.Tipo = tipo
		}
	}
	rows.Close()

	if len(tagIDs) == 0 {
		s.Data = []models.MentionPoint{}
		return s, nil
	}

	drows, err := db.GetPool().Query(ctx, `
		SELECT n.fecha::date::text AS dia, COUNT(DISTINCT n.id) AS cnt
		FROM tags_noticia tn
		JOIN tags t ON tn.tag_id = t.id
		JOIN traducciones tr ON tn.traduccion_id = tr.id
		JOIN noticias n ON tr.noticia_id = n.id
		WHERE t.id = ANY($1) AND n.fecha >= CURRENT_DATE - ($2::int - 1)
		GROUP BY n.fecha::date
		ORDER BY n.fecha::date
	`, tagIDs, days)
	if err != nil {
		return nil, err
	}
	defer drows.Close()

	for drows.Next() {
		var fecha string
		var cnt int
		if err := drows.Scan(&fecha, &cnt); err != nil {
			continue
		}
		s.Data = append(s.Data, models.MentionPoint{Fecha: fecha, Count: cnt})
	}
	if s.Data == nil {
		s.Data = []models.MentionPoint{}
	}

	return s, nil
}
