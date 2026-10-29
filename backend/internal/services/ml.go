package services

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rss2/backend/internal/config"
	"github.com/rss2/backend/internal/db"
	"github.com/rss2/backend/internal/models"
)

var (
	cfg *config.Config
)

func Init(c *config.Config) {
	cfg = c
}

type TranslationRequest struct {
	SourceLang string   `json:"source_lang"`
	TargetLang string   `json:"target_lang"`
	Texts      []string `json:"texts"`
}

type TranslationResponse struct {
	Translations []string `json:"translations"`
}

func Translate(ctx context.Context, sourceLang, targetLang string, texts []string) ([]string, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	reqBody := TranslationRequest{
		SourceLang: sourceLang,
		TargetLang: targetLang,
		Texts:      texts,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Post(cfg.TranslationURL+"/translate", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("translation request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("translation service returned status %d", resp.StatusCode)
	}

	var result TranslationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Translations, nil
}

func GetEmbeddings(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// Use our embeddings service (sentence-transformers)
	reqBody := map[string]interface{}{
		"texts": texts,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Timeout: 60 * time.Second}
	resp, err := httpClient.Post(cfg.EmbeddingsURL+"/api/embeddings", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embeddings request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embeddings service returned status %d", resp.StatusCode)
	}

	var result struct {
		Embeddings [][]float64 `json:"embeddings"`
		Model      string      `json:"model"`
		Dimension  int         `json:"dimension"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Embeddings, nil
}

type NERRequest struct {
	Text string `json:"text"`
}

type NERResponse struct {
	Entities []Entity `json:"entities"`
}

type Entity struct {
	Text  string `json:"text"`
	Label string `json:"label"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

func ExtractEntities(ctx context.Context, text string) ([]Entity, error) {
	reqBody := NERRequest{Text: text}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Post(cfg.SpacyURL+"/ner", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("NER request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NER service returned status %d", resp.StatusCode)
	}

	var result NERResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Entities, nil
}

// SemanticFilters acota la busqueda semantica. Todos los campos son
// opcionales: si se omiten el comportamiento es el de siempre.
type SemanticFilters struct {
	Desde    *time.Time
	MinChars int
	Dedup    bool
}

// SemanticSearch busca noticias por significado (embeddings + pgvector).
func SemanticSearch(ctx context.Context, query, lang string, page, perPage int, filters ...SemanticFilters) (*models.NewsListResponse, error) {
	// Get embedding for the query from Ollama
	embeddings, err := GetEmbeddings(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("failed to get query embedding: %w", err)
	}

	if len(embeddings) == 0 || len(embeddings[0]) == 0 {
		return &models.NewsListResponse{}, nil
	}

	queryEmbedding := embeddings[0]

	// Filtros opcionales (ventana temporal, longitud minima, deduplicacion).
	filter := SemanticFilters{}
	if len(filters) > 0 {
		filter = filters[0]
	}
	extraWhere := ""
	extraArgs := []interface{}{}
	if filter.Desde != nil {
		extraWhere += " AND n.fecha >= $" + strconv.Itoa(len(extraArgs)+2)
		extraArgs = append(extraArgs, *filter.Desde)
	}
	// Se acota por arriba: hay noticias con fecha corrupta (ano 3000).
	extraWhere += " AND (n.fecha IS NULL OR n.fecha <= NOW() + INTERVAL '1 day')"
	if filter.MinChars > 0 {
		extraWhere += " AND length(COALESCE(NULLIF(t.contenido_trad, ''), NULLIF(n.contenido, ''), NULLIF(t.resumen_trad, ''), NULLIF(n.resumen, ''), '')) >= $" + strconv.Itoa(len(extraArgs)+2)
		extraArgs = append(extraArgs, filter.MinChars)
	}
	if filter.Dedup {
		extraWhere += ` AND NOT EXISTS (
			SELECT 1 FROM noticia_clusters nc2
			JOIN noticia_clusters nc1 ON nc1.cluster_id = nc2.cluster_id
			WHERE nc2.noticia_id = n.id AND nc1.es_principal AND nc1.noticia_id <> n.id
		)`
	}

	// Build the SQL query to find similar translations using vector similarity
	offset := (page - 1) * perPage

	// First get total count
	//
	// Dos fuentes: las traducciones terminadas y el texto ORIGINAL de las
	// noticias que ya estan en el idioma destino (canales marcados como
	// traducidos), que no tienen fila en traducciones pero tambien deben
	// aparecer en la busqueda.
	var total int
	countQuery := `
		SELECT COUNT(*) FROM (
			SELECT t.noticia_id AS nid
			FROM traduccion_embeddings te
			JOIN traducciones t ON te.traduccion_id = t.id
			JOIN noticias n ON t.noticia_id = n.id
			WHERE te.embedding_vector IS NOT NULL
			  AND te.embedding_vector::text != '[]'
			  AND te.embedding_vector::text != '0'
			  AND t.lang_to = $1
			  AND t.status = 'done'` + extraWhere + `
			UNION ALL
			SELECT n.id AS nid
			FROM traduccion_embeddings te
			JOIN noticias n ON n.id = te.noticia_id
			LEFT JOIN traducciones t ON t.noticia_id = n.id AND t.lang_to = $1
			WHERE te.traduccion_id IS NULL
			  AND te.embedding_vector IS NOT NULL
			  AND te.embedding_vector::text != '[]'
			  AND te.embedding_vector::text != '0'
			  AND n.lang = $1` + extraWhere + `
		) x`
	countArgs := append([]interface{}{lang}, extraArgs...)
	err = db.GetPool().QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to count results: %w", err)
	}

	if total == 0 {
		return &models.NewsListResponse{
			News:       []models.NewsWithTranslations{},
			Total:      0,
			Page:       page,
			PerPage:    perPage,
			TotalPages: 0,
		}, nil
	}

	// Convert query embedding to PostgreSQL array format for vector
	embeddingStr := "[" + floatArrayToString(queryEmbedding) + "]"

	// Search for similar translations using cosine distance
	// Note: pgvector <=> operator works with string casting
	// El idioma destino es SIEMPRE el primer argumento ($1): los filtros van
	// despues (searchArgs = [lang] + extraArgs + [perPage, offset]). Antes se
	// calculaba el placeholder como len(extraArgs)+1, que con filtros
	// apuntava al argumento equivocado y colisionaba con el de extraWhere
	// ("operator does not exist: integer >= character"): la busqueda
	// semantica con filtros fallaba siempre.
	searchQuery := fmt.Sprintf(`
		SELECT * FROM (
			SELECT
				n.id, COALESCE(n.titulo, ''), COALESCE(n.resumen, ''), COALESCE(n.contenido, ''),
				n.url, n.fecha, n.imagen_url, n.fuente_nombre, n.lang,
				t.titulo_trad, t.resumen_trad, t.contenido_trad,
				1 - (te.embedding_vector <=> '%s'::vector) AS score
			FROM traduccion_embeddings te
			JOIN traducciones t ON te.traduccion_id = t.id
			JOIN noticias n ON t.noticia_id = n.id
			WHERE te.embedding_vector IS NOT NULL
			  AND te.embedding_vector::text != '[]'
			  AND te.embedding_vector::text != '0'
			  AND t.lang_to = $%d
			  AND t.status = 'done'%s
			UNION ALL
			SELECT
				n.id, COALESCE(n.titulo, ''), COALESCE(n.resumen, ''), COALESCE(n.contenido, ''),
				n.url, n.fecha, n.imagen_url, n.fuente_nombre, n.lang,
				t.titulo_trad, t.resumen_trad, t.contenido_trad,
				1 - (te.embedding_vector <=> '%s'::vector) AS score
			FROM traduccion_embeddings te
			JOIN noticias n ON n.id = te.noticia_id
			LEFT JOIN traducciones t ON t.noticia_id = n.id AND t.lang_to = $%d
			WHERE te.traduccion_id IS NULL
			  AND te.embedding_vector IS NOT NULL
			  AND te.embedding_vector::text != '[]'
			  AND te.embedding_vector::text != '0'
			  AND n.lang = $%d%s
		) x
		ORDER BY score DESC
		LIMIT $%d OFFSET $%d
	`, embeddingStr, 1, extraWhere, embeddingStr, 1, 1, extraWhere, len(extraArgs)+2, len(extraArgs)+3)

	searchArgs := append([]interface{}{lang}, extraArgs...)
	searchArgs = append(searchArgs, perPage, offset)

	rows, err := db.GetPool().Query(ctx, searchQuery, searchArgs...)
	if err != nil {
		return nil, fmt.Errorf("semantic search query failed: %w", err)
	}
	defer rows.Close()

	news := []models.NewsWithTranslations{}
	for rows.Next() {
		var n models.NewsWithTranslations
		var id, titulo, resumen, contenido, url string
		var fecha sql.NullTime
		var img, fuenteNombre, langRaw sql.NullString
		var score float64

		if err := rows.Scan(&id, &titulo, &resumen, &contenido, &url, &fecha, &img,
			&fuenteNombre, &langRaw, &n.TitleTranslated, &n.SummaryTranslated, &n.ContentTranslated, &score); err != nil {
			continue
		}

		n.ID = id
		n.Titulo = titulo
		n.Resumen = resumen
		n.Contenido = contenido
		n.URL = url
		if fecha.Valid {
			fechaStr := fecha.Time.Format(time.RFC3339)
			n.Fecha = &fechaStr
		}
		if img.Valid {
			n.ImagenURL = &img.String
		}
		n.FuenteNombre = fuenteNombre.String
		n.Lang = strings.TrimSpace(langRaw.String)

		news = append(news, n)
	}

	totalPages := (total + perPage - 1) / perPage

	return &models.NewsListResponse{
		News:       news,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}, nil
}

func floatArrayToString(arr []float64) string {
	result := ""
	for i, v := range arr {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("%f", v)
	}
	return result
}
