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
// actividad de hoy supera significativamente su media de los últimos días.
func RunAlertScan(ctx context.Context) (int, error) {
	minHits := 3
	minRatio := 5.0
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

	query := `
		WITH daily AS (
			SELECT t.id AS tag_id, t.valor, t.tipo, n.fecha::date AS dia,
			       COUNT(DISTINCT n.id) AS cnt
			FROM tags_noticia tn
			JOIN tags t ON tn.tag_id = t.id
			JOIN traducciones tr ON tn.traduccion_id = tr.id
			JOIN noticias n ON tr.noticia_id = n.id
			WHERE n.fecha >= CURRENT_DATE - 8
			  AND t.tipo IN ('persona', 'lugar', 'organizacion', 'tema')
			GROUP BY t.id, t.valor, t.tipo, n.fecha::date
		),
		baseline AS (
			SELECT tag_id, valor, tipo,
			       AVG(cnt)::float AS baseline,
			       MAX(cnt)::int AS max_prev
			FROM daily
			WHERE dia < CURRENT_DATE
			GROUP BY tag_id, valor, tipo
		),
		today AS (
			SELECT tag_id, valor, tipo, SUM(cnt)::int AS hits
			FROM daily
			WHERE dia = CURRENT_DATE
			GROUP BY tag_id, valor, tipo
		)
		SELECT t.valor, t.tipo, t.hits, b.baseline,
		       (t.hits::float / NULLIF(b.baseline, 0)) AS ratio
		FROM today t
		JOIN baseline b USING (tag_id)
		WHERE t.hits >= $1
		  AND b.baseline >= 0.5
		  AND (t.hits::float / NULLIF(b.baseline, 0)) >= $2
	`

	rows, err := db.GetPool().Query(ctx, query, minHits, minRatio)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	inserted := 0
	for rows.Next() {
		var valor, tipo string
		var hits int
		var baseline, ratio float64
		if err := rows.Scan(&valor, &tipo, &hits, &baseline, &ratio); err != nil {
			continue
		}
		res, err := db.GetPool().Exec(ctx, `
			INSERT INTO alertas (valor, tipo, periodo, hits, baseline, ratio)
			VALUES ($1, $2, CURRENT_DATE, $3, $4, $5)
			ON CONFLICT (valor, tipo, periodo) DO NOTHING
		`, valor, tipo, hits, baseline, ratio)
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
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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