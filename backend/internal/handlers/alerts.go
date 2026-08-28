package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rss2/backend/internal/db"
	"github.com/rss2/backend/internal/models"
)

type Alerta struct {
	ID        int64   `json:"id"`
	Valor     string  `json:"valor"`
	Tipo      string  `json:"tipo"`
	Periodo   string  `json:"periodo"`
	Hits      int     `json:"hits"`
	Baseline  float64 `json:"baseline"`
	Ratio     float64 `json:"ratio"`
	Status    string  `json:"status"`
	CreatedAt string  `json:"created_at"`
}

func GetAlertas(c *gin.Context) {
	status := c.Query("status")
	limitStr := c.DefaultQuery("limit", "100")

	limit, _ := strconv.Atoi(limitStr)
	if limit < 1 || limit > 500 {
		limit = 100
	}

	query := `
		SELECT id, valor, tipo, periodo::text, hits, baseline, ratio, status, to_char(created_at, 'YYYY-MM-DD HH24:MI') AS created_at
		FROM alertas
	`
	args := []interface{}{}
	if status != "" {
		query += fmt.Sprintf(" WHERE status = $%d", len(args)+1)
		args = append(args, status)
	}
	query += fmt.Sprintf(" ORDER BY periodo DESC, ratio DESC, id DESC LIMIT $%d", len(args)+1)
	args = append(args, limit)

	rows, err := db.GetPool().Query(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to get alerts", Message: err.Error()})
		return
	}
	defer rows.Close()

	alertas := []Alerta{}
	for rows.Next() {
		var a Alerta
		if err := rows.Scan(&a.ID, &a.Valor, &a.Tipo, &a.Periodo, &a.Hits, &a.Baseline, &a.Ratio, &a.Status, &a.CreatedAt); err != nil {
			continue
		}
		alertas = append(alertas, a)
	}

	var nuevas int
	db.GetPool().QueryRow(c.Request.Context(), "SELECT COUNT(*)::int FROM alertas WHERE status = 'nueva'").Scan(&nuevas)

	c.JSON(http.StatusOK, gin.H{
		"alertas": alertas,
		"total":   len(alertas),
		"nuevas":  nuevas,
	})
}

func MarkAlertaRead(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid alert id"})
		return
	}

	_, err = db.GetPool().Exec(c.Request.Context(), "UPDATE alertas SET status = 'leida' WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to update alert", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func MarkAllAlertasRead(c *gin.Context) {
	_, err := db.GetPool().Exec(c.Request.Context(), "UPDATE alertas SET status = 'leida' WHERE status = 'nueva'")
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to update alerts", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func ScanAlertasAdmin(c *gin.Context) {
	inserted, err := RunAlertScan(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Scan failed", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "new_alerts": inserted})
}

// RunAlertScan busca conceptos (persona, lugar, organizacion, tema) cuya
// actividad supera significativamente su media de los días anteriores.
//
// En lugar de comparar siempre contra CURRENT_DATE (que se queda vacío si la
// cadena de traducción/NER va por detrás de la ingesta), se toma como día de
// referencia el día más reciente que tenga datos de tags etiquetados y se
// compara contra la media de los ALERTS_LOOKBACK_DAYS días previos, contando
// los días sin actividad como 0.
func RunAlertScan(ctx context.Context) (int, error) {
	minHits := 5
	minRatio := 5.0
	minBaseline := 2.0
	lookbackDays := 8
	if v := os.Getenv("ALERTS_MIN_HITS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			minHits = n
		}
	}
	if v := os.Getenv("ALERTS_MIN_RATIO"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			minRatio = f
		}
	}
	if v := os.Getenv("ALERTS_MIN_BASELINE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			minBaseline = f
		}
	}
	if v := os.Getenv("ALERTS_LOOKBACK_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			lookbackDays = n
		}
	}

	query := `
		WITH daily AS (
			SELECT t.id AS tag_id, t.valor, t.tipo, n.fecha::date AS dia,
			       COUNT(DISTINCT n.id) AS cnt
			FROM tags_noticia tn
			JOIN tags t ON tn.tag_id = t.id
			JOIN traducciones tr ON tn.traduccion_id = tr.id
			JOIN noticias n ON tr.noticia_id = n.id
			WHERE n.fecha >= CURRENT_DATE - 30
			  AND n.fecha <= CURRENT_DATE
			  AND t.tipo IN ('persona', 'lugar', 'organizacion', 'tema')
			GROUP BY t.id, t.valor, t.tipo, n.fecha::date
		),
		ref AS (
			SELECT MAX(dia) AS ref_date FROM daily
		),
		prev_series AS (
			SELECT generate_series((SELECT ref_date - $3::int FROM ref),
			                       (SELECT ref_date - 1 FROM ref), '1 day')::date AS dia
		),
		today AS (
			SELECT tag_id, valor, tipo, SUM(cnt)::int AS hits
			FROM daily
			WHERE dia = (SELECT ref_date FROM ref)
			GROUP BY tag_id, valor, tipo
		),
		base AS (
			SELECT t.tag_id, t.valor, t.tipo,
			       AVG(COALESCE(d.cnt, 0))::float AS baseline
			FROM today t
			CROSS JOIN prev_series s
			LEFT JOIN daily d ON d.tag_id = t.tag_id AND d.dia = s.dia
			GROUP BY t.tag_id, t.valor, t.tipo
		)
		SELECT t.valor, t.tipo, t.hits, b.baseline,
		       (t.hits::float / b.baseline) AS ratio,
		       (SELECT ref_date FROM ref)::date AS periodo
		FROM today t
		JOIN base b USING (tag_id)
		WHERE t.hits >= $1
		  AND b.baseline >= $4
		  AND (t.hits::float / b.baseline) >= $2
	`

	rows, err := db.GetPool().Query(ctx, query, minHits, minRatio, lookbackDays, minBaseline)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	inserted := 0
	for rows.Next() {
		var valor, tipo string
		var hits int
		var baseline, ratio float64
		var periodo time.Time
		if err := rows.Scan(&valor, &tipo, &hits, &baseline, &ratio, &periodo); err != nil {
			continue
		}
		res, err := db.GetPool().Exec(ctx, `
			INSERT INTO alertas (valor, tipo, periodo, hits, baseline, ratio)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (valor, tipo, periodo) DO NOTHING
		`, valor, tipo, periodo, hits, baseline, ratio)
		if err != nil {
			log.Printf("alerts: insert failed for %s (%s): %v", valor, tipo, err)
			continue
		}
		if n := res.RowsAffected(); n > 0 {
			inserted++
		}
	}

	return inserted, nil
}

// StartAlertScanner lanza una goroutine que revisa picos de actividad
// periódicamente (cada ALERTS_SCAN_INTERVAL_MIN, por defecto 120 min).
func StartAlertScanner() {
	interval := 120
	if v := os.Getenv("ALERTS_SCAN_INTERVAL_MIN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			interval = n
		}
	}

		go func() {
			time.Sleep(45 * time.Second)
			for {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
				inserted, err := RunAlertScan(ctx)
				cancel()
			if err != nil {
				log.Printf("alerts: scan error: %v", err)
			} else {
				log.Printf("alerts: scan finished, %d new alerts", inserted)
			}
			time.Sleep(time.Duration(interval) * time.Minute)
		}
	}()
}