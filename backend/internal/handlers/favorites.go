package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rss2/backend/internal/db"
)

// --- Favorites ---

func GetFavorites(c *gin.Context) {
	userID := c.GetInt("user_id")
	ctx := c.Request.Context()

	rows, err := db.GetPool().Query(ctx, `
		SELECT n.id, n.titulo, n.resumen, n.url, n.fecha, n.imagen_url,
		       n.categoria_id, n.pais_id, n.fuente_nombre,
		       t.titulo_trad as title_translated, t.resumen_trad as summary_translated, 
		       t.contenido_trad as content_translated, t.lang_to as lang_translated
		FROM user_favorites uf
		JOIN noticias n ON uf.noticia_id = n.id
		LEFT JOIN traducciones t ON n.id = t.noticia_id AND t.lang_to = 'es'
		WHERE uf.user_id = $1
		ORDER BY uf.created_at DESC
	`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	favorites := []map[string]any{}
	for rows.Next() {
		var f struct {
			ID            string
			Titulo        sql.NullString
			Resumen       sql.NullString
			URL           string
			Fecha         sql.NullTime
			ImagenURL     sql.NullString
			CategoriaID   sql.NullInt64
			PaisID        sql.NullInt64
			FuenteNombre  sql.NullString
			TitleTranslated sql.NullString
			SummaryTranslated sql.NullString
			ContentTranslated sql.NullString
			LangTranslated sql.NullString
		}
		if err := rows.Scan(&f.ID, &f.Titulo, &f.Resumen, &f.URL, &f.Fecha, &f.ImagenURL,
			&f.CategoriaID, &f.PaisID, &f.FuenteNombre, &f.TitleTranslated,
			&f.SummaryTranslated, &f.ContentTranslated, &f.LangTranslated); err != nil {
			continue
		}
		item := map[string]any{
			"id":            f.ID,
			"titulo":        f.Titulo.String,
			"resumen":       f.Resumen.String,
			"url":           f.URL,
			"imagen_url":    f.ImagenURL.String,
			"categoria_id":  f.CategoriaID.Int64,
			"pais_id":       f.PaisID.Int64,
			"fuente_nombre": f.FuenteNombre.String,
			"title_translated": f.TitleTranslated.String,
			"summary_translated": f.SummaryTranslated.String,
			"content_translated": f.ContentTranslated.String,
			"lang_translated": strings.TrimSpace(f.LangTranslated.String),
		}
		if f.Fecha.Valid {
			item["fecha"] = f.Fecha.Time
		}
		favorites = append(favorites, item)
	}

	c.JSON(http.StatusOK, gin.H{"favorites": favorites})
}

func AddFavorite(c *gin.Context) {
	userID := c.GetInt("user_id")
	noticiaID := c.Param("noticiaId")
	ctx := c.Request.Context()

	_, err := db.GetPool().Exec(ctx, `
		INSERT INTO user_favorites (user_id, noticia_id) VALUES ($1, $2)
		ON CONFLICT (user_id, noticia_id) DO NOTHING
	`, userID, noticiaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "added"})
}

func RemoveFavorite(c *gin.Context) {
	userID := c.GetInt("user_id")
	noticiaID := c.Param("noticiaId")
	ctx := c.Request.Context()

	_, err := db.GetPool().Exec(ctx, `
		DELETE FROM user_favorites WHERE user_id = $1 AND noticia_id = $2
	`, userID, noticiaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "removed"})
}

func IsFavorite(c *gin.Context) {
	userID := c.GetInt("user_id")
	noticiaID := c.Param("noticiaId")
	ctx := c.Request.Context()

	var exists bool
	err := db.GetPool().QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM user_favorites WHERE user_id = $1 AND noticia_id = $2)
	`, userID, noticiaID).Scan(&exists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"is_favorite": exists})
}

// --- Lists ---

func GetLists(c *gin.Context) {
	userID := c.GetInt("user_id")
	ctx := c.Request.Context()

	rows, err := db.GetPool().Query(ctx, `
		SELECT ul.id, ul.name, ul.keywords, ul.created_at, ul.updated_at,
		       COUNT(uli.id) as item_count
		FROM user_lists ul
		LEFT JOIN user_list_items uli ON ul.id = uli.list_id
		WHERE ul.user_id = $1
		GROUP BY ul.id
		ORDER BY ul.updated_at DESC
	`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	lists := []map[string]any{}
	for rows.Next() {
		var id int
		var name, keywords string
		var createdAt, updatedAt sql.NullTime
		var itemCount int
		if err := rows.Scan(&id, &name, &keywords, &createdAt, &updatedAt, &itemCount); err != nil {
			continue
		}
		lists = append(lists, map[string]any{
			"id":         id,
			"name":       name,
			"keywords":   keywords,
			"created_at": createdAt.Time,
			"updated_at": updatedAt.Time,
			"item_count": itemCount,
		})
	}

	c.JSON(http.StatusOK, gin.H{"lists": lists})
}

func CreateList(c *gin.Context) {
	userID := c.GetInt("user_id")
	println("CreateList: userID =", userID)
	var body struct {
		Name     string `json:"name" binding:"required"`
		Keywords string `json:"keywords"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
	}
	ctx := c.Request.Context()

	var id int
	err := db.GetPool().QueryRow(ctx, `
		INSERT INTO user_lists (user_id, name, keywords) VALUES ($1, $2, $3) RETURNING id
	`, userID, body.Name, body.Keywords).Scan(&id)
	if err != nil {
		println("CreateList: INSERT error =", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": id, "name": body.Name, "keywords": body.Keywords})
}

func UpdateList(c *gin.Context) {
	userID := c.GetInt("user_id")
	listID := c.Param("id")
	var body struct {
		Name     string `json:"name"`
		Keywords string `json:"keywords"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	ctx := c.Request.Context()

	// Build dynamic update query
	setClauses := []string{"updated_at = NOW()"}
	args := []interface{}{}
	argIdx := 1

	if body.Name != "" {
		setClauses = append(setClauses, "name = $"+fmt.Sprintf("%d", argIdx))
		args = append(args, body.Name)
		argIdx++
	}
	if body.Keywords != "" {
		setClauses = append(setClauses, "keywords = $"+fmt.Sprintf("%d", argIdx))
		args = append(args, body.Keywords)
		argIdx++
	}

	if len(setClauses) == 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nothing to update"})
		return
	}

	args = append(args, listID, userID)
	query := fmt.Sprintf(`UPDATE user_lists SET %s WHERE id = $%d AND user_id = $%d`,
		strings.Join(setClauses, ", "), argIdx, argIdx+1)

	result, err := db.GetPool().Exec(ctx, query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "list not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func DeleteList(c *gin.Context) {
	userID := c.GetInt("user_id")
	listID := c.Param("id")
	ctx := c.Request.Context()

	result, err := db.GetPool().Exec(ctx, `
		DELETE FROM user_lists WHERE id = $1 AND user_id = $2
	`, listID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "list not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func GetListItems(c *gin.Context) {
	userID := c.GetInt("user_id")
	listID := c.Param("id")
	ctx := c.Request.Context()

	// Verify ownership
	var ownerID int
	err := db.GetPool().QueryRow(ctx, `SELECT user_id FROM user_lists WHERE id = $1`, listID).Scan(&ownerID)
	if err != nil || ownerID != userID {
		c.JSON(http.StatusNotFound, gin.H{"error": "list not found"})
		return
	}

	rows, err := db.GetPool().Query(ctx, `
		SELECT n.id, n.titulo, n.resumen, n.url, n.fecha, n.imagen_url,
		       n.categoria_id, n.pais_id, n.fuente_nombre,
		       t.titulo_trad as title_translated, t.resumen_trad as summary_translated, 
		       t.contenido_trad as content_translated, t.lang_to as lang_translated
		FROM user_list_items uli
		JOIN noticias n ON uli.noticia_id = n.id
		LEFT JOIN traducciones t ON n.id = t.noticia_id AND t.lang_to = 'es'
		WHERE uli.list_id = $1
		ORDER BY uli.created_at DESC
	`, listID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	items := []map[string]any{}
	for rows.Next() {
		var f struct {
			ID               string
			Titulo           sql.NullString
			Resumen          sql.NullString
			URL              string
			Fecha            sql.NullTime
			ImagenURL        sql.NullString
			CategoriaID      sql.NullInt64
			PaisID           sql.NullInt64
			FuenteNombre     sql.NullString
			TitleTranslated  sql.NullString
			SummaryTranslated sql.NullString
			ContentTranslated sql.NullString
			LangTranslated   sql.NullString
		}
		if err := rows.Scan(&f.ID, &f.Titulo, &f.Resumen, &f.URL, &f.Fecha, &f.ImagenURL,
			&f.CategoriaID, &f.PaisID, &f.FuenteNombre, &f.TitleTranslated,
			&f.SummaryTranslated, &f.ContentTranslated, &f.LangTranslated); err != nil {
			continue
		}
		item := map[string]any{
			"id":               f.ID,
			"titulo":           f.Titulo.String,
			"resumen":          f.Resumen.String,
			"url":              f.URL,
			"imagen_url":       f.ImagenURL.String,
			"categoria_id":     f.CategoriaID.Int64,
			"pais_id":          f.PaisID.Int64,
			"fuente_nombre":    f.FuenteNombre.String,
			"title_translated": f.TitleTranslated.String,
			"summary_translated": f.SummaryTranslated.String,
			"content_translated": f.ContentTranslated.String,
			"lang_translated":  strings.TrimSpace(f.LangTranslated.String),
		}
		if f.Fecha.Valid {
			item["fecha"] = f.Fecha.Time
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

func AddToList(c *gin.Context) {
	userID := c.GetInt("user_id")
	listID := c.Param("id")
	noticiaID := c.Param("noticiaId")
	ctx := c.Request.Context()

	// Verify ownership
	var ownerID int
	err := db.GetPool().QueryRow(ctx, `SELECT user_id FROM user_lists WHERE id = $1`, listID).Scan(&ownerID)
	if err != nil || ownerID != userID {
		c.JSON(http.StatusNotFound, gin.H{"error": "list not found"})
		return
	}

	_, err = db.GetPool().Exec(ctx, `
		INSERT INTO user_list_items (list_id, noticia_id) VALUES ($1, $2)
		ON CONFLICT (list_id, noticia_id) DO NOTHING
	`, listID, noticiaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update list's updated_at
	db.GetPool().Exec(ctx, `UPDATE user_lists SET updated_at = NOW() WHERE id = $1`, listID)

	c.JSON(http.StatusOK, gin.H{"message": "added"})
}

func RemoveFromList(c *gin.Context) {
	userID := c.GetInt("user_id")
	listID := c.Param("id")
	noticiaID := c.Param("noticiaId")
	ctx := c.Request.Context()

	// Verify ownership
	var ownerID int
	err := db.GetPool().QueryRow(ctx, `SELECT user_id FROM user_lists WHERE id = $1`, listID).Scan(&ownerID)
	if err != nil || ownerID != userID {
		c.JSON(http.StatusNotFound, gin.H{"error": "list not found"})
		return
	}

	_, err = db.GetPool().Exec(ctx, `
		DELETE FROM user_list_items WHERE list_id = $1 AND noticia_id = $2
	`, listID, noticiaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "removed"})
}

// --- Saved Searches ---

func GetSavedSearches(c *gin.Context) {
	userID := c.GetInt("user_id")
	ctx := c.Request.Context()

	rows, err := db.GetPool().Query(ctx, `
		SELECT id, type, label, params, created_at
		FROM user_saved_searches
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	searches := []map[string]any{}
	for rows.Next() {
		var id int
		var stype, label string
		var paramsJSON []byte
		var createdAt sql.NullTime
		if err := rows.Scan(&id, &stype, &label, &paramsJSON, &createdAt); err != nil {
			continue
		}
		var params map[string]any
		json.Unmarshal(paramsJSON, &params)
		searches = append(searches, map[string]any{
			"id":         id,
			"type":       stype,
			"label":      label,
			"params":      params,
			"created_at": createdAt.Time,
		})
	}

	c.JSON(http.StatusOK, gin.H{"searches": searches})
}

type savedSearchParams struct {
	Type   string `json:"type" binding:"required"`
	Label  string `json:"label" binding:"required"`
	Params any    `json:"params" binding:"required"`
}

func CreateSavedSearch(c *gin.Context) {
	userID := c.GetInt("user_id")
	var body struct {
		Type   string `json:"type" binding:"required"`
		Label  string `json:"label" binding:"required"`
		Params map[string]any `json:"params" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx := c.Request.Context()

	paramsJSON, _ := json.Marshal(body.Params)

	var id int
	err := db.GetPool().QueryRow(ctx, `
		INSERT INTO user_saved_searches (user_id, type, label, params) VALUES ($1, $2, $3, $4) RETURNING id
	`, userID, body.Type, body.Label, paramsJSON).Scan(&id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": id, "type": body.Type, "label": body.Label, "params": body.Params})
}

func UpdateSavedSearch(c *gin.Context) {
	userID := c.GetInt("user_id")
	searchID := c.Param("id")
	var body struct {
		Label string `json:"label" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "label required"})
		return
	}
	ctx := c.Request.Context()

	result, err := db.GetPool().Exec(ctx, `
		UPDATE user_saved_searches SET label = $1 WHERE id = $2 AND user_id = $3
	`, body.Label, searchID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "search not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func DeleteSavedSearch(c *gin.Context) {
	userID := c.GetInt("user_id")
	searchID := c.Param("id")
	ctx := c.Request.Context()

	result, err := db.GetPool().Exec(ctx, `
		DELETE FROM user_saved_searches WHERE id = $1 AND user_id = $2
	`, searchID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "search not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// --- List Tags (Tag Cloud) ---

func GetListTags(c *gin.Context) {
	userID := c.GetInt("user_id")
	listID := c.Param("id")
	ctx := c.Request.Context()

	// Verify ownership
	var ownerID int
	err := db.GetPool().QueryRow(ctx, `SELECT user_id FROM user_lists WHERE id = $1`, listID).Scan(&ownerID)
	if err != nil || ownerID != userID {
		c.JSON(http.StatusNotFound, gin.H{"error": "list not found"})
		return
	}

	// Get tags from all news in the list, with count
	rows, err := db.GetPool().Query(ctx, `
		SELECT t.valor, t.tipo, COUNT(*) as cnt
		FROM user_list_items uli
		JOIN tags_noticia tn ON uli.noticia_id = tn.noticia_id
		JOIN tags t ON tn.tag_id = t.id
		WHERE uli.list_id = $1
		GROUP BY t.valor, t.tipo
		ORDER BY cnt DESC, t.valor ASC
		LIMIT 50
	`, listID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	tags := []map[string]any{}
	for rows.Next() {
		var valor, tipo string
		var cnt int
		if err := rows.Scan(&valor, &tipo, &cnt); err != nil {
			continue
		}
		tags = append(tags, map[string]any{
			"valor": valor,
			"tipo":  tipo,
			"count": cnt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"tags": tags})
}

// --- Related News Search (based on list keywords) ---

func GetListRelatedNews(c *gin.Context) {
	userID := c.GetInt("user_id")
	listID := c.Param("id")
	ctx := c.Request.Context()

	// Verify ownership and get keywords
	var ownerID int
	var keywords string
	err := db.GetPool().QueryRow(ctx, `SELECT user_id, keywords FROM user_lists WHERE id = $1`, listID).Scan(&ownerID, &keywords)
	if err != nil || ownerID != userID {
		c.JSON(http.StatusNotFound, gin.H{"error": "list not found"})
		return
	}

	if keywords == "" {
		c.JSON(http.StatusOK, gin.H{"news": []map[string]any{}, "message": "no keywords set"})
		return
	}

	// Search news by keywords using full-text search
	searchTerms := strings.Split(keywords, ",")
	for i := range searchTerms {
		searchTerms[i] = strings.TrimSpace(searchTerms[i])
	}

	// Build tsquery from keywords
	tsquery := strings.Join(searchTerms, " | ")

	rows, err := db.GetPool().Query(ctx, `
		SELECT DISTINCT n.id, n.titulo, n.resumen, n.url, n.fecha, n.imagen_url,
		       n.categoria_id, n.pais_id, n.fuente_nombre,
		       t.titulo_trad as title_translated, t.resumen_trad as summary_translated,
		       t.contenido_trad as content_translated, t.lang_to as lang_translated
		FROM noticias n
		LEFT JOIN traducciones t ON n.id = t.noticia_id AND t.lang_to = 'es'
		WHERE n.tsv @@ to_tsquery('spanish', $1)
		  AND n.id NOT IN (SELECT noticia_id FROM user_list_items WHERE list_id = $2)
		ORDER BY n.fecha DESC NULLS LAST
		LIMIT 20
	`, tsquery, listID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	news := []map[string]any{}
	for rows.Next() {
		var f struct {
			ID               string
			Titulo           sql.NullString
			Resumen          sql.NullString
			URL              string
			Fecha            sql.NullTime
			ImagenURL        sql.NullString
			CategoriaID      sql.NullInt64
			PaisID           sql.NullInt64
			FuenteNombre     sql.NullString
			TitleTranslated  sql.NullString
			SummaryTranslated sql.NullString
			ContentTranslated sql.NullString
			LangTranslated   sql.NullString
		}
		if err := rows.Scan(&f.ID, &f.Titulo, &f.Resumen, &f.URL, &f.Fecha, &f.ImagenURL,
			&f.CategoriaID, &f.PaisID, &f.FuenteNombre, &f.TitleTranslated,
			&f.SummaryTranslated, &f.ContentTranslated, &f.LangTranslated); err != nil {
			continue
		}
		item := map[string]any{
			"id":               f.ID,
			"titulo":           f.Titulo.String,
			"resumen":          f.Resumen.String,
			"url":              f.URL,
			"imagen_url":       f.ImagenURL.String,
			"categoria_id":     f.CategoriaID.Int64,
			"pais_id":          f.PaisID.Int64,
			"fuente_nombre":    f.FuenteNombre.String,
			"title_translated": f.TitleTranslated.String,
			"summary_translated": f.SummaryTranslated.String,
			"content_translated": f.ContentTranslated.String,
			"lang_translated":  strings.TrimSpace(f.LangTranslated.String),
		}
		if f.Fecha.Valid {
			item["fecha"] = f.Fecha.Time
		}
		news = append(news, item)
	}

	c.JSON(http.StatusOK, gin.H{"news": news, "keywords": keywords})
}

// --- Suggested News for Sidebar (based on user's lists keywords) ---

func GetSuggestedNews(c *gin.Context) {
	userID := c.GetInt("user_id")
	ctx := c.Request.Context()

	// Get all keywords from user's lists
	rows, err := db.GetPool().Query(ctx, `
		SELECT DISTINCT keywords
		FROM user_lists
		WHERE user_id = $1 AND keywords != ''
	`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	allTerms := []string{}
	for rows.Next() {
		var keywords string
		if err := rows.Scan(&keywords); err != nil {
			continue
		}
		terms := strings.Split(keywords, ",")
		for _, t := range terms {
			t = strings.TrimSpace(t)
			if t != "" {
				allTerms = append(allTerms, t)
			}
		}
	}

	if len(allTerms) == 0 {
		c.JSON(http.StatusOK, gin.H{"news": []map[string]any{}, "message": "no keywords in lists"})
		return
	}

	// Deduplicate terms
	seen := make(map[string]bool)
	uniqueTerms := []string{}
	for _, t := range allTerms {
		if !seen[t] {
			seen[t] = true
			uniqueTerms = append(uniqueTerms, t)
		}
	}

	// Build tsquery from all unique keywords
	tsquery := strings.Join(uniqueTerms, " | ")

	// Get recent news matching any of the keywords, not already in any of user's lists
	listRows, err := db.GetPool().Query(ctx, `
		SELECT DISTINCT n.id, n.titulo, n.resumen, n.url, n.fecha, n.imagen_url,
		       n.categoria_id, n.pais_id, n.fuente_nombre,
		       t.titulo_trad as title_translated, t.resumen_trad as summary_translated,
		       t.contenido_trad as content_translated, t.lang_to as lang_translated
		FROM noticias n
		LEFT JOIN traducciones t ON n.id = t.noticia_id AND t.lang_to = 'es'
		WHERE n.tsv @@ to_tsquery('spanish', $1)
		  AND n.fecha >= NOW() - INTERVAL '7 days'
		  AND n.id NOT IN (
		      SELECT uli.noticia_id
		      FROM user_list_items uli
		      JOIN user_lists ul ON uli.list_id = ul.id
		      WHERE ul.user_id = $2
		  )
		ORDER BY n.fecha DESC NULLS LAST
		LIMIT 15
	`, tsquery, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer listRows.Close()

	news := []map[string]any{}
	for listRows.Next() {
		var f struct {
			ID               string
			Titulo           sql.NullString
			Resumen          sql.NullString
			URL              string
			Fecha            sql.NullTime
			ImagenURL        sql.NullString
			CategoriaID      sql.NullInt64
			PaisID           sql.NullInt64
			FuenteNombre     sql.NullString
			TitleTranslated  sql.NullString
			SummaryTranslated sql.NullString
			ContentTranslated sql.NullString
			LangTranslated   sql.NullString
		}
		if err := listRows.Scan(&f.ID, &f.Titulo, &f.Resumen, &f.URL, &f.Fecha, &f.ImagenURL,
			&f.CategoriaID, &f.PaisID, &f.FuenteNombre, &f.TitleTranslated,
			&f.SummaryTranslated, &f.ContentTranslated, &f.LangTranslated); err != nil {
			continue
		}
		item := map[string]any{
			"id":               f.ID,
			"titulo":           f.Titulo.String,
			"resumen":          f.Resumen.String,
			"url":              f.URL,
			"imagen_url":       f.ImagenURL.String,
			"categoria_id":     f.CategoriaID.Int64,
			"pais_id":          f.PaisID.Int64,
			"fuente_nombre":    f.FuenteNombre.String,
			"title_translated": f.TitleTranslated.String,
			"summary_translated": f.SummaryTranslated.String,
			"content_translated": f.ContentTranslated.String,
			"lang_translated":  strings.TrimSpace(f.LangTranslated.String),
		}
		if f.Fecha.Valid {
			item["fecha"] = f.Fecha.Time
		}
		news = append(news, item)
	}

	c.JSON(http.StatusOK, gin.H{"news": news})
}
