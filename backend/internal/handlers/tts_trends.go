package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rss2/backend/internal/db"
	"github.com/rss2/backend/internal/models"
)

// TTSNewsCandidate es una noticia susceptible de locutarse, con la
// informacion que necesita el editor para decidir.
type TTSNewsCandidate struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	Summary       string  `json:"summary"`
	ContentLength int     `json:"content_length"`
	Source        string  `json:"source"`
	URL           string  `json:"url"`
	Date          *string `json:"date"`
	ImageURL      *string `json:"image_url"`
	CategoryID    *int64  `json:"categoria_id"`
	CountryID     *int64  `json:"pais_id"`
	// ClusterID agrupa las noticias del mismo tema (para no repetirlas).
	ClusterID *int64 `json:"cluster_id"`
	// ClusterPopularity es el numero de medios que cubren la historia.
	ClusterPopularity *int `json:"cluster_popularity"`
	// ClusterImportance: breaking | major | normal | minor.
	ClusterImportance *string `json:"cluster_importancia"`
}

// TTSTrendCluster es una historia consolidada (agrupacion de noticias).
type TTSTrendCluster struct {
	ID             int64             `json:"id"`
	Title          string            `json:"title"`
	Importance     string            `json:"importance"`
	Popularity     int               `json:"popularity"`
	StartDate      *string           `json:"start_date"`
	EndDate        *string           `json:"end_date"`
	NewsCount      int               `json:"news_count"`
	Representative *TTSNewsCandidate `json:"representative"`
}

// TTSTrendTopic es un tema en tendencia.
type TTSTrendTopic struct {
	ID    int64   `json:"id"`
	Name  string  `json:"name"`
	News  int     `json:"news"`
	Score float64 `json:"score"`
}

// TTSTrendsResponse es la respuesta del endpoint de tendencias.
type TTSTrendsResponse struct {
	Clusters []TTSTrendCluster      `json:"clusters"`
	Topics   []TTSTrendTopic        `json:"topics"`
	Params   map[string]interface{} `json:"params"`
}

// GetTTSTrends devuelve las historias y temas con más cobertura reciente.
//
// Pensado para montar un informativo: cada cluster es UNA historia (con su
// noticia representativa), de modo que no se locuta dos veces lo mismo.
// @Summary Tendencias para el noticiario
// @Description Historias consolidadas (clusters de eventos) y temas en tendencia, con la noticia representativa de cada historia
// @Tags tts
// @Produce json
// @Param dias query int false "Ventana en dias" default(2) minimum(1) maximum(30)
// @Param importancia query string false "breaking, major, normal o minor (vacio = todas)"
// @Param limite query int false "Numero de historias" default(30) minimum(1) maximum(100)
// @Param min_chars query int false "Longitud minima del texto de la noticia"
// @Success 200 {object} TTSTrendsResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /tts/trends [get]
func GetTTSTrends(c *gin.Context) {
	ctx := c.Request.Context()

	dias, _ := strconv.Atoi(c.DefaultQuery("dias", "2"))
	if dias < 1 {
		dias = 2
	}
	if dias > 30 {
		dias = 30
	}
	limite, _ := strconv.Atoi(c.DefaultQuery("limite", "30"))
	if limite < 1 {
		limite = 30
	}
	if limite > 100 {
		limite = 100
	}
	importancia := strings.TrimSpace(c.Query("importancia"))
	minChars, _ := strconv.Atoi(c.Query("min_chars"))

	args := []interface{}{dias}
	where := "e.fecha_inicio >= NOW() - make_interval(days => $1::int)"
	if importancia != "" {
		args = append(args, importancia)
		where += fmt.Sprintf(" AND e.importancia = $%d", len(args))
	}
	args = append(args, limite)
	limitPlaceholder := len(args)

	query := fmt.Sprintf(`
		SELECT e.id, COALESCE(e.titulo_template, ''), e.importancia,
		       COALESCE(e.popularidad, 0), e.fecha_inicio, e.fecha_fin,
		       COALESCE(nc.noticia_id, e.primer_noticia_id),
		       COALESCE(nc.titulo, n.titulo, ''),
		       COALESCE(nc.resumen, n.resumen, ''),
		       COALESCE(nc.fuente_nombre, n.fuente_nombre, ''),
		       COALESCE(nc.url, n.url, ''),
		       nc.fecha,
		       nc.imagen_url,
		       length(COALESCE(NULLIF(nc.contenido, ''), NULLIF(nc.resumen, ''), ''))
		FROM event_clusters e
		LEFT JOIN LATERAL (
			SELECT n2.id AS noticia_id,
			       COALESCE(NULLIF(tr.titulo_trad, ''), n2.titulo, '') AS titulo,
			       COALESCE(NULLIF(tr.resumen_trad, ''), n2.resumen, '') AS resumen,
			       COALESCE(NULLIF(tr.contenido_trad, ''), n2.contenido, '') AS contenido,
			       n2.url, n2.fecha, n2.imagen_url, n2.fuente_nombre
			FROM noticia_clusters nc2
			JOIN noticias n2 ON nc2.noticia_id = n2.id
			LEFT JOIN traducciones tr
			       ON tr.noticia_id = n2.id AND tr.lang_to = 'es' AND tr.status = 'done'
			WHERE nc2.cluster_id = e.id
			ORDER BY nc2.es_principal DESC, nc2.similitud DESC, n2.fecha DESC
			LIMIT 1
		) nc ON TRUE
		LEFT JOIN noticias n ON n.id = COALESCE(nc.noticia_id, e.primer_noticia_id)
		WHERE %s
		ORDER BY
			CASE e.importancia WHEN 'breaking' THEN 0 WHEN 'major' THEN 1 WHEN 'normal' THEN 2 ELSE 3 END,
			e.popularidad DESC, e.fecha_inicio DESC
		LIMIT $%d
	`, where, limitPlaceholder)

	rows, err := db.GetPool().Query(ctx, query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to fetch trends", Message: err.Error(),
		})
		return
	}
	defer rows.Close()

	clusters := []TTSTrendCluster{}
	for rows.Next() {
		var (
			cluster    TTSTrendCluster
			newsID     *string
			title      *string
			summary    *string
			source     *string
			url        *string
			date       *time.Time
			imageURL   *string
			contentLen *int
			startDate  *time.Time
			endDate    *time.Time
		)
		if err := rows.Scan(&cluster.ID, &cluster.Title, &cluster.Importance,
			&cluster.Popularity, &startDate, &endDate,
			&newsID, &title, &summary, &source, &url, &date, &imageURL, &contentLen); err != nil {
			continue
		}

		cluster.NewsCount = cluster.Popularity
		if startDate != nil {
			formatted := startDate.Format(time.RFC3339)
			cluster.StartDate = &formatted
		}
		if endDate != nil {
			formatted := endDate.Format(time.RFC3339)
			cluster.EndDate = &formatted
		}

		// Solo historias con una noticia representativa utilizable.
		if newsID != nil && *newsID != "" {
			length := 0
			if contentLen != nil {
				length = *contentLen
			}
			if minChars > 0 && length < minChars {
				continue
			}
			candidate := &TTSNewsCandidate{
				ID:                *newsID,
				Title:             stringValue(title),
				Summary:           stringValue(summary),
				ContentLength:     length,
				Source:            stringValue(source),
				URL:               stringValue(url),
				ImageURL:          imageURL,
				ClusterID:         &cluster.ID,
				ClusterPopularity: &cluster.Popularity,
				ClusterImportance: &cluster.Importance,
			}
			if date != nil {
				formatted := date.Format(time.RFC3339)
				candidate.Date = &formatted
			}
			cluster.Representative = candidate
		}

		clusters = append(clusters, cluster)
	}

	// Temas en tendencia: los mas publicados en la ventana.
	topicsQuery := `
		SELECT t.id, t.nombre, COUNT(nt.noticia_id) AS noticias,
		       COALESCE(AVG(nt.confidence), 0) AS score
		FROM news_topics nt
		JOIN topics t ON t.id = nt.topic_id
		JOIN noticias n ON n.id = nt.noticia_id
		WHERE n.fecha >= NOW() - make_interval(days => $1::int)
		GROUP BY t.id, t.nombre
		HAVING COUNT(nt.noticia_id) > 1
		ORDER BY noticias DESC, score DESC
		LIMIT 20
	`
	topics := []TTSTrendTopic{}
	topicRows, err := db.GetPool().Query(ctx, topicsQuery, dias)
	if err != nil {
		log.Printf("GetTTSTrends: no se pudieron obtener los temas: %v", err)
	} else {
		defer topicRows.Close()
		for topicRows.Next() {
			var topic TTSTrendTopic
			if err := topicRows.Scan(&topic.ID, &topic.Name, &topic.News, &topic.Score); err != nil {
				continue
			}
			topics = append(topics, topic)
		}
	}

	c.JSON(http.StatusOK, TTSTrendsResponse{
		Clusters: clusters,
		Topics:   topics,
		Params: map[string]interface{}{
			"dias":        dias,
			"importancia": importancia,
			"limite":      limite,
			"min_chars":   minChars,
		},
	})
}

// stringValue devuelve el contenido de un puntero o cadena vacia.
func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
