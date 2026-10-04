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
	"github.com/rss2/backend/internal/entitycheck"
	"github.com/rss2/backend/internal/models"
)

type Alerta struct {
	ID       int64   `json:"id"`
	Valor    string  `json:"valor"`
	Tipo     string  `json:"tipo"`
	Periodo  string  `json:"periodo"`
	Hits     int     `json:"hits"`
	Baseline float64 `json:"baseline"`
	Ratio    float64 `json:"ratio"`
	// Score es la significación del pico ((hits-baseline)/√baseline,
	// normal z de Poisson). A diferencia de `ratio` no se dispara con
	// baselines minúsculos, así que sirve para ordenar dentro de una hora.
	Score       float64 `json:"score"`
	Status      string  `json:"status"`
	CreatedAt   string  `json:"created_at"`
	WikiSummary *string `json:"wiki_summary"`
	WikiURL     *string `json:"wiki_url"`
	ImagePath   *string `json:"image_path"`
}

// envInt/envFloat leen un umbral de `ALERTS_*`; si falta o no es válido
// devuelve el por defecto, así la configuración siempre es inspeccionable.
func envInt(name string, def int) int {
	if v := os.Getenv(name); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func envFloat(name string, def float64) float64 {
	if v := os.Getenv(name); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			return f
		}
	}
	return def
}

func GetAlertas(c *gin.Context) {
	status := c.Query("status")
	tipo := c.Query("tipo")
	limitStr := c.DefaultQuery("limit", "100")

	limit, _ := strconv.Atoi(limitStr)
	if limit < 1 || limit > 500 {
		limit = 100
	}

	// El filtro anti-FP es el mismo que usa Populares: una alerta sobre un
	// valor de `entity_blocklist` es un falso positivo conocido y no se
	// enseña (ni se cuenta en `total`).
	where := `WHERE NOT EXISTS (SELECT 1 FROM entity_blocklist b
	                  WHERE b.tipo = a.tipo AND b.valor_lower = LOWER(a.valor))`
	args := []interface{}{}
	if status != "" {
		args = append(args, status)
		where += fmt.Sprintf(" AND a.status = $%d", len(args))
	} else {
		// Por defecto las descartadas (falsas alertas del usuario) se ocultan.
		where += ` AND a.status <> 'descartada'`
	}
	if tipo != "" {
		args = append(args, tipo)
		where += fmt.Sprintf(" AND a.tipo = $%d", len(args))
	}

	query := `
		SELECT a.id, a.valor, a.tipo,
		       to_char(a.periodo, 'YYYY-MM-DD HH24:MI') AS periodo,
		       a.hits, a.baseline,
		       a.ratio, a.status, to_char(a.created_at, 'YYYY-MM-DD HH24:MI'),
		       ((a.hits - a.baseline) / sqrt(GREATEST(a.baseline, 0.01))) AS score,
		       t.wiki_summary, t.wiki_url, t.image_path
		FROM alertas a
		LEFT JOIN tags t ON t.valor = a.valor AND t.tipo = a.tipo
		` + where + `
		ORDER BY a.periodo DESC, score DESC, a.id DESC
		LIMIT $` + strconv.Itoa(len(args)+1)
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
		if err := rows.Scan(&a.ID, &a.Valor, &a.Tipo, &a.Periodo, &a.Hits, &a.Baseline, &a.Ratio, &a.Status, &a.CreatedAt, &a.Score, &a.WikiSummary, &a.WikiURL, &a.ImagePath); err != nil {
			log.Printf("alerts: scan row error: %v", err)
			continue
		}
		alertas = append(alertas, a)
	}

	// `total` es el total REAL con los mismos filtros (antes era len(rows),
	// con lo que la paginación mentía).
	// `nuevas` ahora usa los MISMOS filtros de tipo y blocklist para consistencia.
	var total, nuevas int
	countArgs := append([]interface{}{}, args[:len(args)-1]...)
	db.GetPool().QueryRow(c.Request.Context(),
		"SELECT COUNT(*)::int FROM alertas a "+where, countArgs...).Scan(&total)

	// Count de nuevas con los mismos filtros (tipo, blocklist) pero solo status='nueva'
	// Se reconstruye el WHERE base con parámetros para evitar SQL injection
	nuevasWhere := `WHERE NOT EXISTS (SELECT 1 FROM entity_blocklist b
	                  WHERE b.tipo = a.tipo AND b.valor_lower = LOWER(a.valor)) AND a.status = 'nueva'`
	nuevasArgs := []interface{}{}
	if tipo != "" {
		nuevasWhere += " AND a.tipo = $1"
		nuevasArgs = append(nuevasArgs, tipo)
	}
	db.GetPool().QueryRow(c.Request.Context(),
		"SELECT COUNT(*)::int FROM alertas a "+nuevasWhere, nuevasArgs...).Scan(&nuevas)

	c.JSON(http.StatusOK, gin.H{
		"alertas": alertas,
		"total":   total,
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

// MarkAlertaDismiss marca una alerta como falsa (`status='descartada'`):
// no se vuelve a mostrar en la vista por defecto y, mientras dure el
// cooldown del scanner, tampoco se reinserta.
func MarkAlertaDismiss(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid alert id"})
		return
	}

	res, err := db.GetPool().Exec(c.Request.Context(),
		"UPDATE alertas SET status = 'descartada' WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to dismiss alert", Message: err.Error()})
		return
	}
	if n := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Alert not found"})
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
//
//  5. Umbrales reescalados el 2026-09-30 (backtest sobre 118 h de serie
//     real): `hits>=5`, `baseline>=1`, `ratio>=5`, `buckets>=2`, y para
//     `tema` `6/2/6/3`. Con baseline>=1 el peor caso de Poisson es
//     P(X>=5|λ=1)=0,0037<0,01: toda alerta es significativa por
//     construcción. Los umbrales viejos (3/0,5/5) daban 169 alertas/día,
//     el 74 % con baseline<1 y el 19 % sin significación, y nadie las
//     leía.
//
//  6. Anti-FP y cooldown: se descarta lo que esté en `entity_blocklist` o
//     clasifique distinto de "ninguna" en `entitycheck` (los FP débiles
//     también: una alerta se lee, y en Populares siguen para revisión), y
//     lo que ya se haya alertado en las últimas ALERTS_COOLDOWN_HOURS para
//     no repetir la misma entidad mientras dura el pico.
func RunAlertScan(ctx context.Context) (int, error) {
	// Umbrales por defecto: `entitycheck.AlertaBase`/`AlertaTema` (backtest
	// del 2026-09-30 sobre 118 h de serie real, spec 04). Las env `ALERTS_*`
	// siguen permitiendo moverlos sin recompilar.
	umbBase := entitycheck.Umbrales{
		MinHits:     envInt("ALERTS_MIN_HITS", entitycheck.AlertaBase.MinHits),
		MinRatio:    envFloat("ALERTS_MIN_RATIO", entitycheck.AlertaBase.MinRatio),
		MinBaseline: envFloat("ALERTS_MIN_BASELINE", entitycheck.AlertaBase.MinBaseline),
		MinBuckets:  envInt("ALERTS_MIN_BASELINE_BUCKETS", entitycheck.AlertaBase.MinBuckets),
	}
	// Los `tema` son palabras genéricas y generaban el 65 % del ruido:
	// exigen doble exigencia (≈1,6 alertas/día en el backtest).
	umbTema := entitycheck.Umbrales{
		MinHits:     envInt("ALERTS_TEMA_MIN_HITS", entitycheck.AlertaTema.MinHits),
		MinRatio:    envFloat("ALERTS_TEMA_MIN_RATIO", entitycheck.AlertaTema.MinRatio),
		MinBaseline: envFloat("ALERTS_TEMA_MIN_BASELINE", entitycheck.AlertaTema.MinBaseline),
		MinBuckets:  envInt("ALERTS_TEMA_MIN_BASELINE_BUCKETS", entitycheck.AlertaTema.MinBuckets),
	}
	lookbackHours := envInt("ALERTS_LOOKBACK_HOURS", 24)
	cooldownHours := envInt("ALERTS_COOLDOWN_HOURS", 6)

	// Diagnóstico previo: deja rastro de por qué un scan no dispara en vez
	// de un "0 new alerts" sin contexto. El retraso se calcula dentro de la
	// BD (mismo reloj que `n.fecha`) para no mezclar zonas horarias.
	var refHour, refInfo string
	var active, baseBuckets int
	var retraso float64
	if err := db.GetPool().QueryRow(ctx, `
		WITH hourly AS (`+hourlyCTE+`),
		ref AS (SELECT MAX(hora) AS h FROM hourly)
		SELECT COALESCE((SELECT h FROM ref)::text, '') AS ref_hora,
		       (SELECT count(DISTINCT hora) FROM hourly) AS activas,
		       (SELECT count(DISTINCT hora) FROM hourly
		         WHERE hora < (SELECT h FROM ref)
		           AND hora >= (SELECT h FROM ref) - ($1::int * interval '1 hour')) AS baseline,
		       EXTRACT(EPOCH FROM (date_trunc('hour', CURRENT_TIMESTAMP)
		                            - COALESCE((SELECT h FROM ref),
		                                       date_trunc('hour', CURRENT_TIMESTAMP)))) / 3600 AS retraso_h
	`, lookbackHours).Scan(&refHour, &active, &baseBuckets, &retraso); err != nil {
		return 0, fmt.Errorf("diagnóstico de alertas: %w", err)
	}
	refInfo = fmt.Sprintf("ref=%s buckets_activos=%d baseline=%d", refHour, active, baseBuckets)
	if refHour == "" {
		log.Printf("alerts: sin datos etiquetados, scan omitido")
		return 0, nil
	}
	if retraso > 3 {
		log.Printf("alerts: WARNING la hora ref lleva %.0f h de retraso (¿NER parado?): las alertas son sobre datos viejos", retraso)
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
		SELECT h.valor, h.tipo, h.hits, b.baseline, b.n_buckets,
		       (h.hits::float / b.baseline) AS ratio,
		       (SELECT ref_hora FROM ref) AS periodo,
		       EXISTS (SELECT 1 FROM entity_blocklist bl
		               WHERE bl.tipo = h.tipo AND bl.valor_lower = LOWER(h.valor)) AS en_blocklist,
		       EXISTS (SELECT 1 FROM alertas a
		               WHERE a.valor = h.valor AND a.tipo = h.tipo
		                 AND a.periodo >= (SELECT ref_hora FROM ref)
		                      - ($6::int * interval '1 hour')) AS reciente
		FROM current_h h
		JOIN base b USING (tag_id)
		WHERE h.hits >= $1
		  AND b.n_buckets >= $5
		  AND b.baseline >= $4
		  AND (h.hits::float / b.baseline) >= $2
	`

	rows, err := db.GetPool().Query(ctx, query, umbBase.MinHits, umbBase.MinRatio, lookbackHours,
		umbBase.MinBaseline, umbBase.MinBuckets, cooldownHours)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	inserted := 0
	candidates := 0
	omitBlocklist, omitCooldown, omitFP, omitWeak, omitTema, omitUmbral := 0, 0, 0, 0, 0, 0
	for rows.Next() {
		var valor, tipo string
		var hits, nBuckets int
		var baseline, ratio float64
		var periodo time.Time
		var enBlocklist, reciente bool
		if err := rows.Scan(&valor, &tipo, &hits, &baseline, &nBuckets, &ratio, &periodo, &enBlocklist, &reciente); err != nil {
			log.Printf("alerts: scan row error: %v", err)
			continue
		}
		candidates++

		// Capa anti-FP: la misma que Populares (blocklist + reglas únicas),
		// pero aquí también se descartan los FP DÉBILES: una alerta es un
		// aviso que alguien lee, así que prima la precisión (los débiles
		// siguen visibles en Populares, que es donde están para revisión).
		if enBlocklist {
			omitBlocklist++
			continue
		}
		if sev, _ := entitycheck.Classify(tipo, valor); sev != entitycheck.SeverityNone {
			if sev == entitycheck.SeverityStrong {
				omitFP++
			} else {
				omitWeak++
			}
			continue
		}
		// Cooldown: una entidad ya alertada en las últimas horas no vuelve
		// a entrar aunque el pico dure (el ON CONFLICT cubre la hora exacta).
		if reciente {
			omitCooldown++
			continue
		}
		// Regla extra para `tema` (palabras genéricas): la SQL solo aplica
		// los umbrales base, así que el de tema se comprueba aquí.
		umb := umbBase
		if tipo == "tema" {
			umb = umbTema
		}
		if ok, motivo := umb.Acepta(hits, baseline, ratio, nBuckets); !ok {
			if tipo == "tema" {
				omitTema++
			} else {
				log.Printf("alerts: descartada %q (%s) por %s", valor, tipo, motivo)
				omitUmbral++
			}
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

	log.Printf("alerts: %s candidatos=%d omitidas_bd=%d cooldown=%d fp=%d debiles=%d tema=%d umbral=%d nuevas=%d "+
		"| base hits>=%d ratio>=%.1f baseline>=%.1f buckets>=%d · tema hits>=%d ratio>=%.1f baseline>=%.1f buckets>=%d · cooldown=%dh",
		refInfo, candidates, omitBlocklist, omitCooldown, omitFP, omitWeak, omitTema, omitUmbral, inserted,
		umbBase.MinHits, umbBase.MinRatio, umbBase.MinBaseline, umbBase.MinBuckets,
		umbTema.MinHits, umbTema.MinRatio, umbTema.MinBaseline, umbTema.MinBuckets, cooldownHours)

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
// periódicamente (cada ALERTS_SCAN_INTERVAL_MIN, por defecto 60 min: con
// 120 min solo se cubrían ~12 de las 24 horas ref al día).
func StartAlertScanner() {
	interval := envInt("ALERTS_SCAN_INTERVAL_MIN", 60)

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
