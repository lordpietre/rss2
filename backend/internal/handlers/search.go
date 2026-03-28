package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/rss2/backend/internal/db"
	"github.com/rss2/backend/internal/models"
	"github.com/rss2/backend/internal/services"
)

func SearchNews(c *gin.Context) {
	query := c.Query("q")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "30"))
	lang := c.DefaultQuery("lang", "")
	categoriaID := c.Query("categoria_id")
	paisID := c.Query("pais_id")
	useSemantic := c.Query("semantic") == "true"

	if query == "" && categoriaID == "" && paisID == "" && lang == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "At least one filter is required (q, categoria_id, pais_id, or lang)"})
		return
	}

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 30
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

	offset := (page - 1) * perPage

	// Build dynamic query
	args := []interface{}{}
	argNum := 1
	whereClause := "WHERE 1=1"

	if query != "" {
		whereClause += " AND (n.titulo ILIKE $" + strconv.Itoa(argNum) + " OR n.resumen ILIKE $" + strconv.Itoa(argNum) + " OR n.contenido ILIKE $" + strconv.Itoa(argNum) + ")"
		args = append(args, "%"+query+"%")
		argNum++
	}

	if lang != "" {
		whereClause += " AND t.lang_to = $" + strconv.Itoa(argNum)
		args = append(args, lang)
		argNum++
	}

	if categoriaID != "" {
		whereClause += " AND n.categoria_id = $" + strconv.Itoa(argNum)
		catID, err := strconv.ParseInt(categoriaID, 10, 64)
		if err == nil {
			args = append(args, catID)
			argNum++
		}
	}

	if paisID != "" {
		whereClause += " AND n.pais_id = $" + strconv.Itoa(argNum)
		pID, err := strconv.ParseInt(paisID, 10, 64)
		if err == nil {
			args = append(args, pID)
			argNum++
		}
	}

	args = append(args, perPage, offset)

	sqlQuery := `
		SELECT n.id, n.titulo, n.resumen, n.contenido, n.url, n.imagen, 
		       n.feed_id, n.lang, n.categoria_id, n.pais_id, n.created_at, n.updated_at,
		       COALESCE(t.titulo_trad, '') as titulo_trad,
		       COALESCE(t.resumen_trad, '') as resumen_trad,
		       t.lang_to as lang_trad,
		       f.nombre as fuente_nombre
		FROM noticias n
		LEFT JOIN traducciones t ON t.noticia_id = n.id AND t.lang_to = $` + strconv.Itoa(argNum) + `
		LEFT JOIN feeds f ON f.id = n.feed_id
		` + whereClause + `
		ORDER BY n.created_at DESC
		LIMIT $` + strconv.Itoa(argNum+1) + ` OFFSET $` + strconv.Itoa(argNum+2)

	rows, err := db.GetPool().Query(ctx, sqlQuery, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Search failed", Message: err.Error()})
		return
	}
	defer rows.Close()

	var newsList []models.NewsWithTranslations
	for rows.Next() {
		var n models.NewsWithTranslations
		var imagen *string

		err := rows.Scan(
			&n.ID, &n.Title, &n.Summary, &n.Content, &n.URL, &imagen,
			&n.FeedID, &n.Lang, &n.CategoryID, &n.CountryID, &n.CreatedAt, &n.UpdatedAt,
			&n.TitleTranslated, &n.SummaryTranslated, &n.LangTranslated,
		)
		if err != nil {
			continue
		}
		if imagen != nil {
			n.ImageURL = imagen
		}
		newsList = append(newsList, n)
	}

	// Get total count
	countArgs := args[:len(args)-2]

	// Remove LIMIT/OFFSET from args for count
	var total int
	err = db.GetPool().QueryRow(ctx, `
		SELECT COUNT(*) FROM noticias n
		LEFT JOIN traducciones t ON t.noticia_id = n.id AND t.lang_to = $`+strconv.Itoa(argNum)+`
		`+whereClause, countArgs...).Scan(&total)
	if err != nil {
		total = len(newsList)
	}

	totalPages := (total + perPage - 1) / perPage

	response := models.NewsListResponse{
		News:       newsList,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}

	c.JSON(http.StatusOK, response)
}

func GetStats(c *gin.Context) {
	var stats models.Stats

	err := db.GetPool().QueryRow(c.Request.Context(), `
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

	rows, err := db.GetPool().Query(c.Request.Context(), `
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
			rows.Scan(&cs.CategoryID, &cs.CategoryName, &cs.Count)
			stats.TopCategories = append(stats.TopCategories, cs)
		}
	}

	rows, err = db.GetPool().Query(c.Request.Context(), `
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
			rows.Scan(&cs.CountryID, &cs.CountryName, &cs.FlagEmoji, &cs.Count)
			stats.TopCountries = append(stats.TopCountries, cs)
		}
	}

	c.JSON(http.StatusOK, stats)
}

func GetCategories(c *gin.Context) {
	rows, err := db.GetPool().Query(c.Request.Context(), `
		SELECT id, nombre FROM categorias ORDER BY nombre`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to get categories", Message: err.Error()})
		return
	}
	defer rows.Close()

	type Category struct {
		ID     int64  `json:"id"`
		Nombre string `json:"nombre"`
	}

	var categories []Category
	for rows.Next() {
		var cat Category
		rows.Scan(&cat.ID, &cat.Nombre)
		categories = append(categories, cat)
	}

	c.JSON(http.StatusOK, categories)
}

func GetCountries(c *gin.Context) {
	rows, err := db.GetPool().Query(c.Request.Context(), `
		SELECT p.id, p.nombre, c.nombre as continente
		FROM paises p
		LEFT JOIN continentes c ON c.id = p.continente_id
		ORDER BY p.nombre`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to get countries", Message: err.Error()})
		return
	}
	defer rows.Close()

	type Country struct {
		ID         int64  `json:"id"`
		Nombre     string `json:"nombre"`
		Continente string `json:"continente"`
	}

	var countries []Country
	for rows.Next() {
		var country Country
		rows.Scan(&country.ID, &country.Nombre, &country.Continente)
		countries = append(countries, country)
	}

	c.JSON(http.StatusOK, countries)
}
