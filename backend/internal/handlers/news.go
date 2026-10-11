package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/rss2/backend/internal/cache"
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

// GetIdiomas returns all active languages available for translation
// @Summary List available languages
// @Description Returns all active languages configured for translation
// @Tags idiomas
// @Produce json
// @Success 200 {object} models.IdiomaListResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /api/idiomas [get]
func GetIdiomas(c *gin.Context) {
	rows, err := db.GetPool().Query(c.Request.Context(), `
		SELECT codigo, nombre, nombre_en, flag_emoji, activo
		FROM idiomas_disponibles
		ORDER BY nombre ASC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch languages", Message: err.Error()})
		return
	}
	defer rows.Close()

	var idiomas []models.Idioma
	for rows.Next() {
		var i models.Idioma
		var flagEmoji sql.NullString
		if err := rows.Scan(&i.Codigo, &i.Nombre, &i.NombreEn, &flagEmoji, &i.Activo); err != nil {
			log.Printf("GetIdiomas: scan error: %v", err)
			continue
		}
		if flagEmoji.Valid {
			i.FlagEmoji = flagEmoji.String
		}
		idiomas = append(idiomas, i)
	}

	if idiomas == nil {
		idiomas = []models.Idioma{}
	}

	c.JSON(http.StatusOK, models.IdiomaListResponse{
		Idiomas: idiomas,
		Total:   len(idiomas),
	})
}

// GetSharePreview returns an HTML page with Open Graph meta tags for social sharing
// This allows Facebook and other social networks to display rich previews with the news content
// @Summary Get share preview page
// @Description Returns HTML with OG meta tags for social media sharing
// @Tags news
// @Produce html
// @Param id path string true "News ID"
// @Success 200 {string} string "HTML page with OG meta tags"
// @Failure 404 {object} models.ErrorResponse
// @Router /api/share/{id} [get]
func GetSharePreview(c *gin.Context) {
	id := c.Param("id")
	targetLang := getTargetLang(c)

	// Query to get news with translation, country name, and category name
	sqlQuery := `
		SELECT 
			n.id, 
			COALESCE(n.titulo, ''), 
			COALESCE(n.resumen, ''), 
			COALESCE(n.contenido, ''), 
			n.url, 
			n.fecha, 
			n.imagen_url, 
			n.fuente_nombre,
			n.lang,
			t.titulo_trad,
			t.resumen_trad,
			t.contenido_trad,
			p.nombre as pais_nombre,
			cat.nombre as categoria_nombre
		FROM noticias n
		LEFT JOIN traducciones t ON t.noticia_id = n.id AND t.lang_to = $1
		LEFT JOIN paises p ON p.id = n.pais_id
		LEFT JOIN categorias cat ON cat.id = n.categoria_id
		WHERE n.id = $2
	`

	var n struct {
		ID              string
		Titulo          string
		Resumen         string
		Contenido       string
		URL             string
		Fecha           *time.Time
		ImagenURL       *string
		FuenteNombre    *string
		Lang            *string
		TituloTrad      *string
		ResumenTrad     *string
		ContenidoTrad   *string
		PaisNombre     *string
		CategoriaNombre *string
	}

	err := db.GetPool().QueryRow(c.Request.Context(), sqlQuery, targetLang, id).Scan(
		&n.ID, &n.Titulo, &n.Resumen, &n.Contenido, &n.URL, &n.Fecha,
		&n.ImagenURL, &n.FuenteNombre, &n.Lang,
		&n.TituloTrad, &n.ResumenTrad, &n.ContenidoTrad,
		&n.PaisNombre, &n.CategoriaNombre,
	)

	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "News not found"})
		return
	}

	// Use translated content if available
	title := n.Titulo
	summary := n.Resumen
	if n.TituloTrad != nil && *n.TituloTrad != "" {
		title = *n.TituloTrad
	}
	if n.ResumenTrad != nil && *n.ResumenTrad != "" {
		summary = *n.ResumenTrad
	}

	// Build the og:description from summary + meta info
	description := summary
	metaInfo := ""
	if n.FuenteNombre != nil && *n.FuenteNombre != "" {
		metaInfo = *n.FuenteNombre
	}
	if n.Fecha != nil {
		if metaInfo != "" {
			metaInfo += " • "
		}
		metaInfo += n.Fecha.Format("02/01/2006")
	}
	if n.PaisNombre != nil && *n.PaisNombre != "" {
		if metaInfo != "" {
			metaInfo += " • "
		}
		metaInfo += *n.PaisNombre
	}

	if metaInfo != "" {
		description = metaInfo + "\n\n" + summary
	}

	// Use the original URL as og:url so sharing points to the source
	ogURL := n.URL
	if n.ImagenURL == nil || *n.ImagenURL == "" {
		n.ImagenURL = nil
	}

	// HTML template with Open Graph meta tags
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="es">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<meta name="description" content="%s">
	
	<!-- Open Graph / Facebook -->
	<meta property="og:type" content="article">
	<meta property="og:url" content="%s">
	<meta property="og:title" content="%s">
	<meta property="og:description" content="%s">
	<meta property="og:image" content="%s">
	<meta property="og:site_name" content="RSS2 News">
	
	<!-- Twitter -->
	<meta property="twitter:card" content="summary_large_image">
	<meta property="twitter:url" content="%s">
	<meta property="twitter:title" content="%s">
	<meta property="twitter:description" content="%s">
	<meta property="twitter:image" content="%s">
	
	<title>%s</title>
	
	<style>
		body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 600px; margin: 50px auto; padding: 20px; line-height: 1.6; }
		.header { color: #333; margin-bottom: 20px; }
		.meta { color: #666; font-size: 14px; margin-bottom: 15px; }
		.summary { font-size: 18px; color: #222; margin-bottom: 20px; }
		.content { color: #444; }
		.source { margin-top: 20px; padding-top: 15px; border-top: 1px solid #eee; color: #888; font-size: 12px; }
		a { color: #0066cc; }
	</style>
</head>
<body>
	<article>
		<div class="header">
			<h1>%s</h1>
		</div>
		<div class="meta">%s</div>
		<div class="summary">%s</div>
		<div class="source">
			Fuente: <a href="%s" target="_blank" rel="noopener">%s</a>
		</div>
	</article>
</body>
</html>`,
		escapeHTML(description),
		escapeHTML(ogURL),
		escapeHTML(title),
		escapeHTML(description),
		getImageOrEmpty(n.ImagenURL),
		escapeHTML(ogURL),
		escapeHTML(title),
		escapeHTML(description),
		getImageOrEmpty(n.ImagenURL),
		escapeHTML(title),
		escapeHTML(title),
		escapeHTML(metaInfo),
		escapeHTML(summary),
		escapeHTML(n.URL),
		escapeHTML(n.URL),
	)

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, html)
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

func getImageOrEmpty(img *string) string {
	if img != nil && *img != "" {
		return *img
	}
	return ""
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
			// Use FTS on both original (n.search_vector_es) and translated (t.search_vector_es) content
			// t.search_vector_es contains the Spanish translation
			where += fmt.Sprintf(" AND (n.search_vector_es @@ plainto_tsquery('spanish', $%d) OR t.search_vector_es @@ plainto_tsquery('spanish', $%d))", argNum, argNum+1)
			args = append(args, query, query) // Need same query twice for both sides of OR
			argNum += 2
		} else {
			// Use Full-Text Search with existing index on search_vector_es
			// plainto_tsquery converts user input to tsquery format
			where += fmt.Sprintf(" AND n.search_vector_es @@ plainto_tsquery('spanish', $%d)", argNum)
			args = append(args, query)
			argNum++
		}
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

	// Count results for page 1 - now fast with FTS index
	var total int
	if page == 1 {
		if query == "" {
			// Browse mode - need to count properly with translatedOnly filter
			if translatedOnly {
				// Count only news that have translations done
				countQuery := `
					SELECT COUNT(*) FROM noticias n
					INNER JOIN traducciones t ON t.noticia_id = n.id AND t.lang_to = $1
					WHERE t.status = 'done' AND t.titulo_trad IS NOT NULL AND t.titulo_trad != n.titulo`
				err := db.GetPool().QueryRow(c.Request.Context(), countQuery, targetLang).Scan(&total)
				if err != nil {
					log.Printf("Count query error (translatedOnly): %v", err)
					total = 0
				}
			} else {
				// Use approximate count for all news
				countQuery := "SELECT reltuples::bigint FROM pg_class WHERE relname = 'noticias'"
				db.GetPool().QueryRow(c.Request.Context(), countQuery).Scan(&total)
			}
		} else {
			// Build separate query for COUNT with proper JOIN
			if translatedOnly {
				// Need JOIN with traducciones for translatedOnly
				countQuery := fmt.Sprintf(`
					SELECT COUNT(*) FROM noticias n
					INNER JOIN traducciones t ON t.noticia_id = n.id AND t.lang_to = $3
					WHERE %s`, where)
				countArgs := []interface{}{query, query, targetLang}
				err := db.GetPool().QueryRow(c.Request.Context(), countQuery, countArgs...).Scan(&total)
				if err != nil {
					log.Printf("Count query error: %v", err)
					total = 0
				}
			} else {
				countArgs := []interface{}{query}
				countQuery := fmt.Sprintf("SELECT COUNT(*) FROM noticias n WHERE %s", where)
				err := db.GetPool().QueryRow(c.Request.Context(), countQuery, countArgs...).Scan(&total)
				if err != nil {
					log.Printf("Count query error: %v", err)
					total = 0
				}
			}
		}
	}

	// Append targetLang for the JOIN condition (needed for main query)
	args = append(args, targetLang)

	var joinType string
	if translatedOnly {
		joinType = "INNER JOIN"
	} else {
		joinType = "LEFT JOIN"
	}

	sqlQuery := `
		SELECT n.id, COALESCE(n.titulo, ''), COALESCE(n.resumen, ''), COALESCE(n.contenido, '') AS contenido, n.url, n.fecha, n.imagen_url,
		       n.categoria_id, n.pais_id, n.fuente_nombre, n.lang,
		       t.titulo_trad,
		       t.resumen_trad,
		       t.contenido_trad,
		       t.lang_to as lang_trad,
		       ec.id as cluster_id
		FROM noticias n
		` + joinType + ` traducciones t ON t.noticia_id = n.id AND t.lang_to = $` + strconv.Itoa(argNum) + `
		LEFT JOIN noticia_clusters nc ON nc.noticia_id = n.id
		LEFT JOIN event_clusters ec ON nc.cluster_id = ec.id
		WHERE ` + where + `
		ORDER BY n.fecha DESC LIMIT $` + strconv.Itoa(argNum+1) + ` OFFSET $` + strconv.Itoa(argNum+2)

	args = append(args, perPage, offset)

	rows, err := db.GetPool().Query(c.Request.Context(), sqlQuery, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch news", Message: err.Error()})
		return
	}
	defer rows.Close()

	var newsList []models.NewsWithTranslations
	for rows.Next() {
		var n models.NewsWithTranslations
		var id, titulo, resumen, contenido, url string
		var fecha *time.Time
		var imagenURL, fuenteNombre, langRaw *string
		var categoriaID, paisID *int32
		var clusterID sql.NullInt64

		err := rows.Scan(
			&id, &titulo, &resumen, &contenido, &url, &fecha, &imagenURL,
			&categoriaID, &paisID, &fuenteNombre, &langRaw,
			&n.TitleTranslated, &n.SummaryTranslated, &n.ContentTranslated, &n.LangTranslated,
			&clusterID,
		)
		if err != nil {
			log.Printf("GetNews: scan error: %v", err)
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
		if categoriaID != nil {
			catID := int64(*categoriaID)
			n.CategoryID = &catID
		}
		if paisID != nil {
			pID := int64(*paisID)
			n.CountryID = &pID
		}
		if langRaw != nil {
			n.Lang = strings.TrimSpace(*langRaw)
		}
		// lang_to es CHAR(5): Postgres rellena con espacios ("es   ") y el
		// frontend lo pinta tal cual (Search.tsx). Mismo trato que n.Lang.
		if n.LangTranslated != nil {
			t := strings.TrimSpace(*n.LangTranslated)
			n.LangTranslated = &t
		}
		if clusterID.Valid {
			n.ClusterID = &clusterID.Int64
		}
		newsList = append(newsList, n)
	}

	// For search queries (query != ""), total stays 0 - frontend shows "Page X" without "of Y"
	// For browse mode (query == ""), total is already set from pg_class

	var totalPages int
	if total == 0 && len(newsList) > 0 {
		// We don't know total, but we have results - show at least 1 page
		totalPages = 1
	} else {
		totalPages = (total + perPage - 1) / perPage
	}

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
		WHERE LOWER(COALESCE(ea.canonical_name, t.valor)) = LOWER($1) AND t.tipo = $2%s
		  AND NOT EXISTS (
			SELECT 1 FROM entity_blocklist b WHERE b.tipo = t.tipo AND b.valor_lower = LOWER(t.valor)
		  )`, timeCond)

	var total int
	if err := db.GetPool().QueryRow(c.Request.Context(),
		fmt.Sprintf("SELECT COUNT(DISTINCT tn.noticia_id) %s", base), params...).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to count entity news", Message: err.Error()})
		return
	}

	query := fmt.Sprintf(`
		SELECT DISTINCT n.id, COALESCE(n.titulo, ''), COALESCE(n.resumen,''), COALESCE(n.contenido, '') AS contenido, n.url, n.fecha, n.imagen_url,
		       n.fuente_nombre, n.lang, tr.titulo_trad, tr.resumen_trad, tr.contenido_trad
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
		var id, titulo, resumen, contenido, url string
		var fecha *time.Time
		var img, fuenteNombre, langRaw *string
		if err := rows.Scan(&id, &titulo, &resumen, &contenido, &url, &fecha, &img,
			&fuenteNombre, &langRaw, &n.TitleTranslated, &n.SummaryTranslated, &n.ContentTranslated); err != nil {
			log.Printf("GetEntityNews: scan error: %v", err)
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
		if img != nil {
			n.ImagenURL = img
		}
		if fuenteNombre != nil {
			n.FuenteNombre = *fuenteNombre
		}
		if langRaw != nil {
			n.Lang = strings.TrimSpace(*langRaw)
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

	// Combined query to fetch news with entities and cluster in one round trip
	sqlQuery := `
		SELECT 
			n.id, COALESCE(n.titulo, ''), COALESCE(n.resumen, ''), COALESCE(n.contenido, '') AS contenido, n.url, n.fecha, n.imagen_url, 
		       n.categoria_id, n.pais_id, n.fuente_nombre, n.lang,
		       t.titulo_trad,
		       t.resumen_trad,
		       t.contenido_trad,
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
		       ) FILTER (WHERE ent.valor IS NOT NULL) as entities,
		       ec.id as cluster_id
		FROM noticias n
		LEFT JOIN traducciones t ON t.noticia_id = n.id AND t.lang_to = $1
		LEFT JOIN (
			SELECT tn.noticia_id, t.valor, t.tipo, 1 as cnt, t.wiki_summary, t.wiki_url, t.image_path
			FROM tags_noticia tn
			JOIN tags t ON tn.tag_id = t.id
			JOIN traducciones tr ON tn.traduccion_id = tr.id
			WHERE t.tipo IN ('persona', 'organizacion')
		) ent ON ent.noticia_id = n.id
		LEFT JOIN noticia_clusters nc ON nc.noticia_id = n.id
		LEFT JOIN event_clusters ec ON nc.cluster_id = ec.id
		WHERE n.id = $2
		GROUP BY n.id, n.titulo, n.resumen, n.contenido, n.url, n.fecha, n.imagen_url, 
		         n.categoria_id, n.pais_id, n.fuente_nombre, n.lang,
		         t.titulo_trad, t.resumen_trad, t.contenido_trad, t.lang_to,
		         ec.id
	`

	var n models.NewsWithTranslations
	var titulo, resumen, contenido, url string
	var fecha *time.Time
	var imagenURL, fuenteNombre, langRaw *string
	var categoriaID, paisID *int32
	var entitiesJSON *string
	var clusterID sql.NullInt64

	err := db.GetPool().QueryRow(c.Request.Context(), sqlQuery, targetLang, id).Scan(
		&id, &titulo, &resumen, &contenido, &url, &fecha, &imagenURL,
		&categoriaID, &paisID, &fuenteNombre, &langRaw,
		&n.TitleTranslated, &n.SummaryTranslated, &n.ContentTranslated, &n.LangTranslated,
		&entitiesJSON,
		&clusterID,
	)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "News not found"})
		return
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
	if categoriaID != nil {
		catID := int64(*categoriaID)
		n.CategoryID = &catID
	}
	if paisID != nil {
		pID := int64(*paisID)
		n.CountryID = &pID
	}
	if langRaw != nil {
		n.Lang = strings.TrimSpace(*langRaw)
	}
	// CHAR(5) rellena con espacios ("es   "); el frontend lo muestra.
	if n.LangTranslated != nil {
		t := strings.TrimSpace(*n.LangTranslated)
		n.LangTranslated = &t
	}

	// Set cluster_id if news belongs to an event
	if clusterID.Valid {
		n.ClusterID = &clusterID.Int64
	}

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

	// Invalidate related caches
	cache.InvalidateStats(c.Request.Context())
	cache.InvalidateSearch(c.Request.Context())

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
	sortBy := c.DefaultQuery("sort_by", "count")

	q := c.Query("q")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "50"))
	page, perPage = validatePageParams(page, perPage, 50, 100)

	offset := (page - 1) * perPage

	// Check if we should use precomputed entity_stats (only for unfiltered queries)
	useStats := countryID == "" && categoryID == "" && q == "" && c.Query("apellido") == ""

	if useStats {
		// Fast path: use precomputed entity_stats table (no JOIN, all data pre-computed)
		orderBy := "count DESC"
		switch sortBy {
		case "count":
			orderBy = "count DESC"
		case "name":
			orderBy = "canonical_name ASC"
		case "recent":
			orderBy = "last_date DESC NULLS LAST"
		}

		countQuery := "SELECT COUNT(*) FROM entity_stats WHERE entity_type = $1"
		var total int
		db.GetPool().QueryRow(c.Request.Context(), countQuery, entityType).Scan(&total)

		query := fmt.Sprintf(`
			SELECT canonical_name as valor, entity_type as tipo, 
			       apellido, count,
			       wiki_summary, wiki_url, image_path, last_date
			FROM entity_stats
			WHERE entity_type = $1
			ORDER BY %s
			LIMIT $2 OFFSET $3
		`, orderBy)
		args := []interface{}{entityType, perPage, offset}

		rows, err := db.GetPool().Query(c.Request.Context(), query, args...)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to get entities", Message: err.Error()})
			return
		}
		defer rows.Close()

		var entities []models.Entity
		for rows.Next() {
			var e models.Entity
			var lastDate sql.NullTime
			if err := rows.Scan(&e.Valor, &e.Tipo, &e.Apellido, &e.Count, &e.WikiSummary, &e.WikiURL, &e.ImagePath, &lastDate); err != nil {
				log.Printf("GetEntities: scan error: %v", err)
				continue
			}
			if lastDate.Valid {
				e.LastDate = &lastDate.Time
			}
			entities = append(entities, e)
		}

		if entities == nil {
			entities = []models.Entity{}
		}

		c.JSON(http.StatusOK, models.EntityListResponse{
			Entities:   entities,
			Total:      total,
			Page:       page,
			PerPage:    perPage,
			TotalPages: (total + perPage - 1) / perPage,
		})
		return
	}

	// Slow path: original query with filters
	var where string
	var args []interface{}
	if entityType != "" {
		where = "t.tipo = $1"
		args = []interface{}{entityType}
	} else {
		where = "1=1"
		args = []interface{}{}
	}

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

	if apellido := c.Query("apellido"); apellido != "" {
		where += fmt.Sprintf(" AND t.apellido ILIKE $%d", len(args)+1)
		args = append(args, "%"+apellido+"%")
	}

	// Validate sort_by
	orderBy := "cnt DESC"
	switch sortBy {
	case "count":
		orderBy = "cnt DESC"
	case "name":
		orderBy = "valor ASC"
	case "recent":
		orderBy = "MAX(n.fecha) DESC NULLS LAST"
	case "last_mentioned":
		orderBy = "t.last_mentioned_at DESC NULLS LAST"
	}

	// Skip slow COUNT DISTINCT for entities. Frontend shows "Page X" without "of Y".
	var total int

	// Fetch the paginated entities. Forma visible = variante exacta más
	// frecuente (mode): evita que un variant TODO-MAYÚSCULAS gane por orden
	// ASCII a MIN(). MAX() en wiki_* ignora NULLs (gana el tag con wiki).
	args = append(args, perPage, offset)
	query := fmt.Sprintf(`
		SELECT mode() WITHIN GROUP (ORDER BY COALESCE(ea.canonical_name, t.valor)) as valor, t.tipo, MAX(t.apellido), COUNT(*)::int as cnt,
		       MAX(t.wiki_summary), MAX(t.wiki_url), MAX(t.image_path), MAX(n.fecha) as last_date
		FROM tags_noticia tn
		JOIN tags t ON tn.tag_id = t.id
		JOIN traducciones tr ON tn.traduccion_id = tr.id
		JOIN noticias n ON tr.noticia_id = n.id
		LEFT JOIN entity_aliases ea ON LOWER(ea.alias) = LOWER(t.valor) AND ea.tipo = t.tipo
		WHERE %s AND NOT EXISTS (
			SELECT 1 FROM entity_blocklist b WHERE b.tipo = t.tipo AND b.valor_lower = LOWER(t.valor)
		)
		GROUP BY LOWER(COALESCE(ea.canonical_name, t.valor)), t.tipo
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, where, orderBy, len(args)-1, len(args))

	rows, err := db.GetPool().Query(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to get entities", Message: err.Error()})
		return
	}
	defer rows.Close()

	var entities []models.Entity
	for rows.Next() {
		var e models.Entity
		var lastDate sql.NullTime
		if err := rows.Scan(&e.Valor, &e.Tipo, &e.Apellido, &e.Count, &e.WikiSummary, &e.WikiURL, &e.ImagePath, &lastDate); err != nil {
			log.Printf("GetEntities: scan error: %v", err)
			continue
		}
		if lastDate.Valid {
			e.LastDate = &lastDate.Time
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

func GetLastNames(c *gin.Context) {
	ctx := c.Request.Context()

	// Query distinct last names from tags table, only for persona type
	rows, err := db.GetPool().Query(ctx, `
		SELECT DISTINCT apellido 
		FROM tags 
		WHERE tipo = 'persona' AND apellido IS NOT NULL AND apellido != ''
		ORDER BY apellido
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to get last names", Message: err.Error()})
		return
	}
	defer rows.Close()

	var lastNames []string
	for rows.Next() {
		var apellido string
		if err := rows.Scan(&apellido); err != nil {
			log.Printf("GetLastNames: scan error: %v", err)
			continue
		}
		lastNames = append(lastNames, apellido)
	}

	c.JSON(http.StatusOK, models.LastNameListResponse{
		LastNames: lastNames,
		Total:     len(lastNames),
	})
}

func GetEntityMentions(c *gin.Context) {
	valuesRaw := c.DefaultQuery("values", "")
	hours := 720 // 30 days default
	if v := c.Query("hours"); v != "" {
		if h, err := strconv.Atoi(v); err == nil && h >= 1 && h <= 2160 { // max 90 days
			hours = h
		}
	}

	// Determine if we use hourly or daily granularity
	useHourly := hours <= 48

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
	// Limit to 3 entities for performance - too many causes slow queries
	if len(values) > 3 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Too many values (max 3 for performance)"})
		return
	}

	start := time.Now().Add(-time.Duration(hours) * time.Hour).Truncate(time.Hour)
	ctx := c.Request.Context()

	// OPTIMIZED: Single query to get all tag IDs for all values using UNNEST
	// Instead of N queries (one per value), we do 1 query
	type tagInfo struct {
		ID   int64
		Tipo string
	}

	// Build array of lowercase values for the query
	lowerValues := make([]string, len(values))
	for i, v := range values {
		lowerValues[i] = strings.ToLower(v)
	}

	// Query all tag IDs for all values at once
	// Optimized: split into two queries - one for direct matches, one for alias matches - then union
	rows, err := db.GetPool().Query(ctx, `
		WITH matching_tags AS (
			SELECT DISTINCT t.id, t.tipo, LOWER(t.valor) as valor_lower
			FROM tags t
			WHERE LOWER(t.valor) = ANY($1)
			UNION
			SELECT DISTINCT t.id, t.tipo, LOWER(t.valor) as valor_lower
			FROM tags t
			JOIN entity_aliases ea ON ea.tipo = t.tipo AND LOWER(ea.alias) = LOWER(t.valor)
			WHERE LOWER(ea.canonical_name) = ANY($1)
		)
		SELECT DISTINCT id, tipo, valor_lower FROM matching_tags
	`, lowerValues)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to get tag IDs", Message: err.Error()})
		return
	}
	defer rows.Close()

	// Map valor -> tagInfo list
	valorToTags := make(map[string][]tagInfo)
	valorToTipo := make(map[string]string)
	for rows.Next() {
		var ti tagInfo
		var valorLower string
		if err := rows.Scan(&ti.ID, &ti.Tipo, &valorLower); err != nil {
			log.Printf("GetEntityMentions: scan error: %v", err)
			continue
		}
		valorToTags[valorLower] = append(valorToTags[valorLower], ti)
		if valorToTipo[valorLower] == "" {
			valorToTipo[valorLower] = ti.Tipo
		}
	}

	// Collect all tag IDs for the aggregation query
	allTagIDs := []int64{}
	tagIDToValor := make(map[int64]string) // reverse map
	for _, v := range values {
		lv := strings.ToLower(v)
		for _, ti := range valorToTags[lv] {
			allTagIDs = append(allTagIDs, ti.ID)
			tagIDToValor[ti.ID] = lv
		}
	}

	if len(allTagIDs) == 0 {
		// No tags found for any of the values
		series := make([]models.MentionSeries, 0, len(values))
		for _, valor := range values {
			var data []models.MentionPoint
			if useHourly {
				data = make([]models.MentionPoint, hours)
				for i := 0; i < hours; i++ {
					data[i] = models.MentionPoint{Fecha: start.Add(time.Duration(i) * time.Hour).Format("2006-01-02 15:00"), Count: 0}
				}
			} else {
				data = make([]models.MentionPoint, hours/24)
				for i := 0; i < hours/24; i++ {
					data[i] = models.MentionPoint{Fecha: start.AddDate(0, 0, i).Format("2006-01-02"), Count: 0}
				}
			}
			series = append(series, models.MentionSeries{
				Valor: valor,
				Data:  data,
				Count: 0,
				Tipo:  "desconocido",
			})
		}
		c.JSON(http.StatusOK, models.MentionsResponse{Days: hours, Series: series})
		return
	}

	// OPTIMIZED: Single query to get all mention counts for all tag IDs
	var drows pgx.Rows
	if useHourly {
		drows, err = db.GetPool().Query(ctx, `
			SELECT tn.tag_id, date_trunc('hour', n.fecha)::text AS hora, COUNT(DISTINCT tn.noticia_id) AS cnt
			FROM tags_noticia tn
			JOIN traducciones tr ON tn.traduccion_id = tr.id
			JOIN noticias n ON tr.noticia_id = n.id
			WHERE tn.tag_id = ANY($1) AND n.fecha >= $2::timestamptz
			GROUP BY tn.tag_id, date_trunc('hour', n.fecha)
			ORDER BY tn.tag_id, date_trunc('hour', n.fecha)
		`, allTagIDs, start)
	} else {
		drows, err = db.GetPool().Query(ctx, `
			SELECT tn.tag_id, n.fecha::date::text AS dia, COUNT(DISTINCT tn.noticia_id) AS cnt
			FROM tags_noticia tn
			JOIN traducciones tr ON tn.traduccion_id = tr.id
			JOIN noticias n ON tr.noticia_id = n.id
			WHERE tn.tag_id = ANY($1) AND n.fecha >= CURRENT_DATE - ($2::int - 1)
			GROUP BY tn.tag_id, n.fecha::date
			ORDER BY tn.tag_id, n.fecha::date
		`, allTagIDs, hours/24)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to get mention counts", Message: err.Error()})
		return
	}
	defer drows.Close()

	// Build tag_id -> dia -> count
	tagDayCount := make(map[int64]map[string]int)
	for drows.Next() {
		var tagID int64
		var fecha string
		var cnt int
		if err := drows.Scan(&tagID, &fecha, &cnt); err != nil {
			log.Printf("GetEntityMentions: scan error: %v", err)
			continue
		}
		if tagDayCount[tagID] == nil {
			tagDayCount[tagID] = make(map[string]int)
		}
		tagDayCount[tagID][fecha] = cnt
	}

	// Build series for each value
	series := make([]models.MentionSeries, 0, len(values))
	for _, valor := range values {
		lv := strings.ToLower(valor)
		tags := valorToTags[lv]

		// Aggregate counts across all tags for this value
		periodCount := make(map[string]int)
		totalCount := 0
		for _, ti := range tags {
			for period, cnt := range tagDayCount[ti.ID] {
				periodCount[period] += cnt
				totalCount += cnt
			}
		}

		// Build complete data points
		var data []models.MentionPoint
		if useHourly {
			data = make([]models.MentionPoint, 0, hours)
			for i := 0; i < hours; i++ {
				fecha := start.Add(time.Duration(i) * time.Hour).Format("2006-01-02 15:00")
				data = append(data, models.MentionPoint{Fecha: fecha, Count: periodCount[fecha]})
			}
		} else {
			data = make([]models.MentionPoint, 0, hours/24)
			for i := 0; i < hours/24; i++ {
				fecha := start.AddDate(0, 0, i).Format("2006-01-02")
				data = append(data, models.MentionPoint{Fecha: fecha, Count: periodCount[fecha]})
			}
		}

		tipo := valorToTipo[lv]
		if tipo == "" {
			tipo = "desconocido"
		}
		series = append(series, models.MentionSeries{
			Valor: valor,
			Data:  data,
			Count: totalCount,
			Tipo:  tipo,
		})
	}

	c.JSON(http.StatusOK, models.MentionsResponse{Days: hours, Series: series})
}

// RefreshEntityStatsStartup refreshes entity_stats on server startup (non-blocking)
func RefreshEntityStatsStartup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	start := time.Now()
	_, err := db.GetPool().Exec(ctx, "SELECT refresh_entity_stats()")
	if err != nil {
		return fmt.Errorf("refresh_entity_stats failed: %w", err)
	}

	var count int
	db.GetPool().QueryRow(ctx, "SELECT COUNT(*) FROM entity_stats").Scan(&count)
	log.Printf("EntityStats: refreshed %d entities in %v", count, time.Since(start))
	return nil
}

// RefreshEntityStats refreshes the entity_stats table (HTTP handler)
func RefreshEntityStats(c *gin.Context) {
	ctx := c.Request.Context()

	// Check if refresh is already running
	var isRefreshing bool
	err := db.GetPool().QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM pg_stat_activity 
			WHERE query LIKE '%refresh_entity_stats%' AND state = 'active'
		)
	`).Scan(&isRefreshing)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to check refresh status", Message: err.Error()})
		return
	}

	if isRefreshing {
		c.JSON(http.StatusConflict, models.ErrorResponse{Error: "Entity stats refresh already in progress"})
		return
	}

	// Get count before refresh
	var countBefore int
	db.GetPool().QueryRow(ctx, "SELECT COUNT(*) FROM entity_stats").Scan(&countBefore)

	// Execute refresh
	start := time.Now()
	_, err = db.GetPool().Exec(ctx, "SELECT refresh_entity_stats()")
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to refresh entity stats", Message: err.Error()})
		return
	}

	// Get count after refresh
	var countAfter int
	db.GetPool().QueryRow(ctx, "SELECT COUNT(*) FROM entity_stats").Scan(&countAfter)

	c.JSON(http.StatusOK, gin.H{
		"status":         "success",
		"count_before":   countBefore,
		"count_after":    countAfter,
		"duration_ms":    time.Since(start).Milliseconds(),
	})
}
