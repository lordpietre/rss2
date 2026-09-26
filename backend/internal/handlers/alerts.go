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
	ID          int64   `json:"id"`
	Valor       string  `json:"valor"`
	Tipo        string  `json:"tipo"`
	Periodo     string  `json:"periodo"`
	Hits        int     `json:"hits"`
	Baseline    float64 `json:"baseline"`
	Ratio       float64 `json:"ratio"`
	Status      string  `json:"status"`
	CreatedAt   string  `json:"created_at"`
	WikiSummary *string `json:"wiki_summary"`
	WikiURL     *string `json:"wiki_url"`
	ImagePath   *string `json:"image_path"`
}

func GetAlertas(c *gin.Context) {
	status := c.Query("status")
	limitStr := c.DefaultQuery("limit", "100")

	limit, _ := strconv.Atoi(limitStr)
	if limit < 1 || limit > 500 {
		limit = 100
	}

	query := `
		SELECT a.id, a.valor, a.tipo,
		       to_char(a.periodo, 'YYYY-MM-DD HH24:MI') AS periodo,
		       a.hits, a.baseline,
		       a.ratio, a.status, to_char(a.created_at, 'YYYY-MM-DD HH24:MI'),
		       t.wiki_summary, t.wiki_url, t.image_path
		FROM alertas a
		LEFT JOIN tags t ON t.valor = a.valor AND t.tipo = a.tipo
	`
	args := []interface{}{}
	if status != "" {
		query += fmt.Sprintf(" WHERE a.status = $%d", len(args)+1)
		args = append(args, status)
	}
	query += fmt.Sprintf(" ORDER BY a.periodo DESC, a.ratio DESC, a.id DESC LIMIT $%d", len(args)+1)
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
		if err := rows.Scan(&a.ID, &a.Valor, &a.Tipo, &a.Periodo, &a.Hits, &a.Baseline, &a.Ratio, &a.Status, &a.CreatedAt, &a.WikiSummary, &a.WikiURL, &a.ImagePath); err != nil {
			log.Printf("alerts: scan row error: %v", err)
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
// actividad en la hora de referencia supera con holgura su media de las horas
// anteriores.
//
// Decisiones de diseño (2026-09-26):
//
//  1. Granularidad HORARIA, no diaria. Con la base reconstruida solo había
//     un día completo etiquetado, de modo que la serie de 8 días previos
//     venía vacía y el baseline salía 0 en las 67 430 entidades: el filtro
//     baseline >= umbral descartaba absolutamente todo y el scan devolvía 0
//     sin explicación.
//
//  2. El baseline solo promedia buckets CON datos. Antes se cruzaba la serie
//     de calendario con COALESCE(cnt, 0), confundiendo "ese tramo no había
//     datos etiquetados" con "cero menciones": o bien rebañaba el baseline a
//     0, o bien, con historia parcial, lo dividía por el número total de
//     huecos y falseaba el ratio hacia arriba.
//
//  3. El baseline se ajusta al volumen de la hora de referencia, es decir
//     menciones por noticia de la base por las noticias de la hora ref. La
//     ingesta es irregular y el NER va por detrás, así que cada hora tiene
//     distinto número de noticias; sin ajustar, el ratio máximo medido en
//     producción era 1.8 y ninguna alerta alcanzaba el umbral. Al
//     normalizar, el ratio vuelve a ser hits/baseline puro cuando las horas
//     igualan volumen, así que la regla se degrada a la comparación
//     absoluta de siempre.
//
//  4. Se descarta la hora en curso (incompleta) y se exige un mínimo de
//     buckets de baseline para no derivar un ratio de una sola hora.
func RunAlertScan(ctx context.Context) (int, error) {
	minHits := 3
	minRatio := 5.0
	minBaseline := 0.5
	lookbackHours := 24
	minBuckets := 2
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
	if v := os.Getenv("ALERTS_LOOKBACK_HOURS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			lookbackHours = n
		}
	}
	if v := os.Getenv("ALERTS_MIN_BASELINE_BUCKETS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			minBuckets = n
		}
	}

	// Diagnóstico previo: deja rastro de por qué un scan no dispara en vez
	// de un "0 new alerts" sin contexto.
	var refHour, refInfo string
	var active, baseBuckets int
	if err := db.GetPool().QueryRow(ctx, `
		WITH hourly AS (`+hourlyCTE+`),
		ref AS (SELECT MAX(hora) AS h FROM hourly)
		SELECT COALESCE((SELECT h FROM ref)::text, '') AS ref_hora,
		       (SELECT count(DISTINCT hora) FROM hourly) AS activas,
		       (SELECT count(DISTINCT hora) FROM hourly
		         WHERE hora < (SELECT h FROM ref)
		           AND hora >= (SELECT h FROM ref) - ($1::int * interval '1 hour')) AS baseline
	`, lookbackHours).Scan(&refHour, &active, &baseBuckets); err != nil {
		return 0, fmt.Errorf("diagnóstico de alertas: %w", err)
	}
	refInfo = fmt.Sprintf("ref=%s buckets_activos=%d baseline=%d", refHour, active, baseBuckets)
	if refHour == "" {
		log.Printf("alerts: sin datos etiquetados, scan omitido")
		return 0, nil
	}

	query := `
		WITH hourly AS (` + hourlyCTE + `),
		vol AS (
			SELECT date_trunc('hour', n.fecha) AS hora,
			       COUNT(DISTINCT n.id) AS noticias
			FROM tags_noticia tn
			JOIN noticias n ON tn.noticia_id = n.id
			WHERE n.fecha >= CURRENT_DATE - 30
			  AND n.fecha < date_trunc('hour', CURRENT_TIMESTAMP)
			GROUP BY 1
		),
		ref AS (
			SELECT MAX(hora) AS ref_hora FROM hourly
		),
		current_h AS (
			SELECT h.tag_id, h.valor, h.tipo, SUM(h.cnt)::int AS hits
			FROM hourly h
			WHERE h.hora = (SELECT ref_hora FROM ref)
			GROUP BY h.tag_id, h.valor, h.tipo
		),
		base AS (
			SELECT c.tag_id,
			       AVG(h.cnt::float / v.noticias) * r.noticias AS baseline,
			       COUNT(DISTINCT h.hora) AS n_buckets
			FROM current_h c
			JOIN hourly h ON h.tag_id = c.tag_id
			JOIN vol v ON v.hora = h.hora
			CROSS JOIN (
				SELECT noticias FROM vol WHERE hora = (SELECT ref_hora FROM ref)
			) r
			WHERE h.hora <  (SELECT ref_hora FROM ref)
			  AND h.hora >= (SELECT ref_hora FROM ref) - ($3::int * interval '1 hour')
			GROUP BY c.tag_id, r.noticias
		)
		SELECT h.valor, h.tipo, h.hits, b.baseline,
		       (h.hits::float / b.baseline) AS ratio,
		       (SELECT ref_hora FROM ref) AS periodo
		FROM current_h h
		JOIN base b USING (tag_id)
		WHERE h.hits >= $1
		  AND b.n_buckets >= $5
		  AND b.baseline >= $4
		  AND (h.hits::float / b.baseline) >= $2
	`

	rows, err := db.GetPool().Query(ctx, query, minHits, minRatio, lookbackHours, minBaseline, minBuckets)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	inserted := 0
	candidates := 0
	for rows.Next() {
		var valor, tipo string
		var hits int
		var baseline, ratio float64
		var periodo time.Time
		if err := rows.Scan(&valor, &tipo, &hits, &baseline, &ratio, &periodo); err != nil {
			log.Printf("alerts: scan row error: %v", err)
			continue
		}
		candidates++
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

	log.Printf("alerts: %s candidatos=%d nuevas=%d (hits>=%d ratio>=%.1f baseline>=%.1f buckets>=%d)",
		refInfo, candidates, inserted, minHits, minRatio, minBaseline, minBuckets)

	return inserted, nil
}

// hourlyCTE agrega menciones por HORA de publicación (no por día) dentro de
// la ventana deslizante, excluyendo la hora en curso. Se comparte con la
// consulta de diagnóstico para que ambos midan exactamente lo mismo.
const hourlyCTE = `
	SELECT t.id AS tag_id, t.valor, t.tipo,
	       date_trunc('hour', n.fecha) AS hora,
	       COUNT(DISTINCT n.id) AS cnt
	FROM tags_noticia tn
	JOIN tags t ON tn.tag_id = t.id
	JOIN traducciones tr ON tn.traduccion_id = tr.id
	JOIN noticias n ON tr.noticia_id = n.id
	WHERE n.fecha >= CURRENT_DATE - 30
	  AND n.fecha < date_trunc('hour', CURRENT_TIMESTAMP)
	  AND t.tipo IN ('persona', 'lugar', 'organizacion', 'tema')
	GROUP BY t.id, t.valor, t.tipo, date_trunc('hour', n.fecha)
`

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