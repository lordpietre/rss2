package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rss2/backend/internal/auth"
	"github.com/rss2/backend/internal/cache"
	"github.com/rss2/backend/internal/db"
	"github.com/rss2/backend/internal/middleware"
	"github.com/rss2/backend/internal/models"
	"github.com/rss2/backend/internal/services"
)

// LogSearch records a logged-in user's search term so frequent searches gain priority.
func LogSearch(c *gin.Context) {
	user := c.MustGet("user").(*auth.Claims)
	var req struct {
		Query string `json:"q"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request"})
		return
	}
	term := strings.ToLower(strings.TrimSpace(req.Query))
	if term == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "q requerido"})
		return
	}
	if len(term) > 100 {
		term = term[:100]
	}

	_, err := db.GetPool().Exec(c.Request.Context(), `
		INSERT INTO user_search_tags (user_id, term, count, last_used)
		VALUES ($1, $2, 1, NOW())
		ON CONFLICT (user_id, term)
		DO UPDATE SET count = user_search_tags.count + 1, last_used = NOW()
	`, user.UserID, term)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to save search", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// SearchSuggestions returns the user's most used search terms (dynamic tags).
func SearchSuggestions(c *gin.Context) {
	user := c.MustGet("user").(*auth.Claims)
	q := strings.TrimSpace(c.Query("q"))

	query := `
		SELECT term FROM user_search_tags
		WHERE user_id = $1`
	args := []interface{}{user.UserID}
	if q != "" {
		query += ` AND term ILIKE $2`
		args = append(args, "%"+q+"%")
	}
	query += ` ORDER BY count DESC, last_used DESC LIMIT 10`

	rows, err := db.GetPool().Query(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to get suggestions", Message: err.Error()})
		return
	}
	defer rows.Close()

	terms := []string{}
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			continue
		}
		terms = append(terms, t)
	}

	c.JSON(http.StatusOK, gin.H{"terms": terms})
}

// SearchNews searches news with various filters
// @Summary Search news
// @Description Search news with text query, language, category, country, and semantic search options
// @Tags search
// @Produce json
// @Param q query string false "Search query"
// @Param page query int false "Page number" default(1) minimum(1)
// @Param per_page query int false "Items per page" default(30) minimum(1) maximum(100)
// @Param lang query string false "Language code" default("es")
// @Param categoria_id query string false "Category ID filter"
// @Param pais_id query string false "Country ID filter"
// @Param semantic query string false "Use semantic search" default("false")
// @Success 200 {object} map[string]interface{} "news, total, page, per_page, total_pages"
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /search [get]
func SearchNews(c *gin.Context) {
	query := c.Query("q")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "30"))
	page, perPage = validatePageParams(page, perPage, 30, 100)
	lang := c.DefaultQuery("lang", "")
	categoriaID := c.Query("categoria_id")
	paisID := c.Query("pais_id")
	useSemantic := c.Query("semantic") == "true"

	if query == "" && categoriaID == "" && paisID == "" && lang == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "At least one filter is required (q, categoria_id, pais_id, or lang)"})
		return
	}

	// Default to Spanish if no lang specified
	if lang == "" {
		lang = "es"
	}

	ctx := c.Request.Context()

	if useSemantic {
		results, err := services.SemanticSearch(ctx, query, lang, page, perPage)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Semantic search failed", Message: err.Error()})
			return
		}
		c.JSON(http.StatusOK, results)
		return
	}

	// Cache key for search results
	cacheKey := cache.SearchKey(query, lang, page, perPage)

	// Try to get from cache
	if cached, err := cache.Get(ctx, cacheKey); err == nil && cached != "" {
		c.Data(http.StatusOK, "application/json", []byte(cached))
		return
	}

	offset := (page - 1) * perPage

	// Build dynamic query
	args := []interface{}{lang}
	argNum := 2
	whereClause := "WHERE 1=1"

	if query != "" {
		// Full-text sobre search_vector_es (índice GIN, 100% poblado):
		// ~50 ms frente a ~4 s del ILIKE con seq scan. plainto_tsquery
		// aplica stemming español ("elecciones" ⊃ "elección").
		whereClause += " AND n.search_vector_es @@ plainto_tsquery('spanish', $" + strconv.Itoa(argNum) + ")"
		args = append(args, query)
		argNum++
	}

	if categoriaID != "" {
		if catID, err := strconv.ParseInt(categoriaID, 10, 64); err == nil {
			whereClause += " AND n.categoria_id = $" + strconv.Itoa(argNum)
			args = append(args, catID)
			argNum++
		}
	}

	if paisID != "" {
		if pID, err := strconv.ParseInt(paisID, 10, 64); err == nil {
			whereClause += " AND n.pais_id = $" + strconv.Itoa(argNum)
			args = append(args, pID)
			argNum++
		}
	}

	sqlQuery := `
		SELECT n.id, COALESCE(n.titulo, ''), COALESCE(n.resumen, ''), COALESCE(n.resumen, '') AS contenido, n.url, n.fecha, n.imagen_url,
		       n.categoria_id, n.pais_id, n.fuente_nombre, n.lang,
		       t.titulo_trad,
		       t.resumen_trad,
		       t.lang_to as lang_trad
		FROM noticias n
		LEFT JOIN traducciones t ON t.noticia_id = n.id AND t.lang_to = $1
		` + whereClause + `
		ORDER BY n.fecha DESC
		LIMIT $` + strconv.Itoa(argNum) + ` OFFSET $` + strconv.Itoa(argNum+1)

	args = append(args, perPage, offset)

	rows, err := db.GetPool().Query(ctx, sqlQuery, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Search failed", Message: err.Error()})
		return
	}
	defer rows.Close()

	var newsList []models.NewsWithTranslations
	for rows.Next() {
		var n models.NewsWithTranslations
		var id, titulo, resumen, contenido, url string
		var fecha *time.Time
		var imagenURL, fuenteNombre, langRaw *string
		var categoriaIDp, paisIDp *int64

		err := rows.Scan(
			&id, &titulo, &resumen, &contenido, &url, &fecha, &imagenURL,
			&categoriaIDp, &paisIDp, &fuenteNombre, &langRaw,
			&n.TitleTranslated, &n.SummaryTranslated, &n.LangTranslated,
		)
		if err != nil {
			continue
		}
		n.ID = id
		n.Titulo = titulo
		n.Resumen = resumen
		n.Contenido = contenido
		n.URL = url
		if fecha != nil {
			f := fecha.Format(time.RFC3339)
			n.Fecha = &f
		}
		if imagenURL != nil {
			n.ImagenURL = imagenURL
		}
		if fuenteNombre != nil {
			n.FuenteNombre = *fuenteNombre
		}
		n.CategoryID = categoriaIDp
		n.CountryID = paisIDp
		if langRaw != nil {
			n.Lang = strings.TrimSpace(*langRaw)
		}
		newsList = append(newsList, n)
	}

	// Get total count
	countArgs := args[:len(args)-2]

	var total int
	err = db.GetPool().QueryRow(ctx, `
		SELECT COUNT(*) FROM noticias n
		LEFT JOIN traducciones t ON t.noticia_id = n.id AND t.lang_to = $1
		`+whereClause, countArgs...).Scan(&total)
	if err != nil {
		total = len(newsList)
	}

	totalPages := (total + perPage - 1) / perPage

	response := gin.H{
		"news":        newsList,
		"total":       total,
		"page":        page,
		"per_page":    perPage,
		"total_pages": totalPages,
	}

	// Cache the response (pass the struct; cache.Set marshals once)
	cache.Set(ctx, cacheKey, response, cache.TTLShort)

c.JSON(http.StatusOK, response)
}

// GetStats returns global statistics
// @Summary Get statistics
// @Description Returns global statistics about news, feeds, users, and translations
// @Tags stats
// @Produce json
// @Success 200 {object} models.Stats
// @Failure 500 {object} models.ErrorResponse
// @Router /stats [get]
func GetStats(c *gin.Context) {
	ctx := c.Request.Context()
	cacheKey := cache.StatsKey()

	if cached, err := cache.Get(ctx, cacheKey); err == nil && cached != "" {
		c.Data(http.StatusOK, "application/json", []byte(cached))
		return
	}

	var stats models.Stats

	err := db.GetPool().QueryRow(ctx, `
		SELECT 
			(SELECT COUNT(*) FROM noticias) as total_news,
			(SELECT COUNT(*) FROM feeds WHERE activo = true) as total_feeds,
			(SELECT COUNT(*) FROM users) as total_users,
			(SELECT COUNT(*) FROM noticias WHERE fecha::date = CURRENT_DATE) as news_today,
			(SELECT COUNT(*) FROM noticias WHERE fecha >= DATE_TRUNC('week', CURRENT_DATE)) as news_this_week,
			(SELECT COUNT(*) FROM noticias WHERE fecha >= DATE_TRUNC('month', CURRENT_DATE)) as news_this_month,
			(SELECT COUNT(DISTINCT noticia_id) FROM traducciones WHERE status = 'done') as total_translated
	`).Scan(
		&stats.TotalNews, &stats.TotalFeeds, &stats.TotalUsers,
		&stats.NewsToday, &stats.NewsThisWeek, &stats.NewsThisMonth,
		&stats.TotalTranslated,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to get stats", Message: err.Error()})
		return
	}

	rows, err := db.GetPool().Query(ctx, `
		SELECT c.id, c.nombre, COUNT(n.id) as count
		FROM categorias c
		LEFT JOIN noticias n ON n.categoria_id = c.id
		GROUP BY c.id, c.nombre
		ORDER BY count DESC
		LIMIT 10
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cs models.CategoryStat
			rows.Scan(&cs.CategoriaID, &cs.CategoriaName, &cs.Count)
			stats.TopCategories = append(stats.TopCategories, cs)
		}
	}

	rows, err = db.GetPool().Query(ctx, `
		SELECT p.id, p.nombre, p.flag_emoji, COUNT(n.id) as count
		FROM paises p
		LEFT JOIN noticias n ON n.pais_id = p.id
		GROUP BY p.id, p.nombre, p.flag_emoji
		ORDER BY count DESC
		LIMIT 10
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cs models.CountryStat
			rows.Scan(&cs.PaisID, &cs.PaisName, &cs.FlagEmoji, &cs.Count)
			stats.TopCountries = append(stats.TopCountries, cs)
		}
	}

	cache.Set(ctx, cacheKey, stats, cache.TTLMedium)

	c.JSON(http.StatusOK, stats)
}

func GetCategories(c *gin.Context) {
	ctx := c.Request.Context()
	cacheKey := cache.CategoriesKey()

	if cached, err := cache.Get(ctx, cacheKey); err == nil && cached != "" {
		c.Data(http.StatusOK, "application/json", []byte(cached))
		return
	}

	rows, err := db.GetPool().Query(ctx, `
		SELECT id, nombre FROM categorias ORDER BY nombre`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to get categories", Message: err.Error()})
		return
	}
	defer rows.Close()

	var categories []models.Category
	for rows.Next() {
		var cat models.Category
		rows.Scan(&cat.ID, &cat.Nombre)
		categories = append(categories, cat)
	}

	cache.Set(ctx, cacheKey, categories, cache.TTLLong)

	c.JSON(http.StatusOK, categories)
}

func GetCountries(c *gin.Context) {
	ctx := c.Request.Context()
	cacheKey := cache.CountriesKey()

	if cached, err := cache.Get(ctx, cacheKey); err == nil && cached != "" {
		c.Data(http.StatusOK, "application/json", []byte(cached))
		return
	}

	rows, err := db.GetPool().Query(ctx, `
		SELECT p.id, p.nombre, c.nombre as continente
		FROM paises p
		LEFT JOIN continentes c ON c.id = p.continente_id
		ORDER BY p.nombre`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to get countries", Message: err.Error()})
		return
	}
	defer rows.Close()

	var countries []models.Country
	for rows.Next() {
		var country models.Country
		rows.Scan(&country.ID, &country.Nombre, &country.Continente)
		countries = append(countries, country)
	}

	cache.Set(ctx, cacheKey, countries, cache.TTLLong)


	c.JSON(http.StatusOK, countries)
}

// GetRateLimitStats expone la observabilidad del rate limit (429s desde el
// arranque, totales y por configuración). Sin caché: son contadores live.
func GetRateLimitStats(c *gin.Context) {
	total, byCfg := middleware.RateLimitStats()
	byCfgJSON := gin.H{}
	for k, v := range byCfg {
		byCfgJSON[strconv.Itoa(k)] = v
	}
	c.JSON(http.StatusOK, gin.H{
		"rate_limit_429_total":  total,
		"rate_limit_429_by_cfg": byCfgJSON,
	})
}
