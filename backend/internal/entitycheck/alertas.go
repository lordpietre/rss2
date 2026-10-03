package entitycheck

import (
	"fmt"
	"math"
)

// Umbrales de las alertas de actividad. Son la única fuente de verdad de los
// valores: los usan `handlers.RunAlertScan` (como valores por defecto de las
// env `ALERTS_*`) y `cmd/entityscan -alertas` (para podar alertas heredadas).
//
// Origen: backtest del 2026-09-30 sobre 118 h de serie real (spec 04). Con
// MinBaseline>=1 + MinRatio>=5 + MinHits>=5 el peor caso de Poisson es
// P(X>=5|λ=1)=0,0037 < 0,01, o sea que toda alerta que pasa es
// significativa por construcción. Los umbrales viejos (3/0,5/5) daban
// 169 alertas/día con el 74 % por debajo de baseline 1 y el 19 % sin
// significación.
type Umbrales struct {
	MinHits     int     // menciones mínimas en la hora de referencia
	MinBaseline float64 // actividad esperada mínima (menciones/hora)
	MinRatio    float64 // factor de incremento mínimo (hits/baseline)
	MinBuckets  int     // buckets de baseline mínimos (>=2 para no derivar el ratio de una sola hora)
}

var (
	// AlertaBase: persona, lugar y organización.
	AlertaBase = Umbrales{MinHits: 5, MinBaseline: 1, MinRatio: 5, MinBuckets: 2}
	// AlertaTema: los temas son palabras genéricas y generaban el 65 % del
	// ruido, así que exigen doble exigencia.
	AlertaTema = Umbrales{MinHits: 6, MinBaseline: 2, MinRatio: 6, MinBuckets: 3}
)

// UmbralesDe devuelve los umbrales vigentes para un tipo de entidad.
func UmbralesDe(tipo string) Umbrales {
	if tipo == "tema" {
		return AlertaTema
	}
	return AlertaBase
}

// Acepta valida una alerta contra los umbrales y devuelve el motivo del
// rechazo ("" si pasa). nBuckets<=0 omite el chequeo de buckets, porque la
// tabla `alertas` no lo guarda.
func (u Umbrales) Acepta(hits int, baseline, ratio float64, nBuckets int) (bool, string) {
	if hits < u.MinHits {
		return false, fmt.Sprintf("hits<%d", u.MinHits)
	}
	if baseline < u.MinBaseline {
		return false, fmt.Sprintf("baseline<%.1f", u.MinBaseline)
	}
	if ratio < u.MinRatio {
		return false, fmt.Sprintf("ratio<%.1f", u.MinRatio)
	}
	if nBuckets > 0 && nBuckets < u.MinBuckets {
		return false, fmt.Sprintf("buckets<%d", u.MinBuckets)
	}
	return true, ""
}

// AlertaValida es el atajo con los umbrales por defecto del tipo.
func AlertaValida(tipo string, hits int, baseline, ratio float64, nBuckets int) (bool, string) {
	return UmbralesDe(tipo).Acepta(hits, baseline, ratio, nBuckets)
}

// PoissonCola devuelve P(X >= k) para X ~ Poisson(lambda): la significación
// de un pico. Con los umbrales vigentes toda alerta da p < 0,01, y el test
// de vigilancia lo comprueba sobre la API real.
func PoissonCola(k int, lambda float64) float64 {
	if lambda <= 0 || k <= 0 {
		return 1
	}
	term := math.Exp(-lambda)
	sum := term
	for i := 1; i < k; i++ {
		term *= lambda / float64(i)
		sum += term
	}
	p := 1 - sum
	if p < 0 {
		return 0
	}
	return p
}
