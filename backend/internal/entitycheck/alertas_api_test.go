package entitycheck

import (
	"encoding/json"
	"math"
	"net/url"
	"testing"
	"time"
)

// TestAlertasCalidad es la red de seguridad del algoritmo de alertas
// (misma filosofía que TestPopulares*): consulta `GET /alerts` real y
// comprueba que lo que ve un usuario cumple los umbrales racionalizados
// del 2026-09-30 (spec 04):
//
//   - hits>=5, baseline>=1, ratio>=5 (y 6/2/6 para `tema`);
//   - significación: cola de Poisson P(X>=hits|baseline) < 0,01;
//   - ninguna alerta visible sobre un FP (fuerte ni débil) de entitycheck
//     ni sobre un valor de la blocklist (el scan no los inserta y la API
//     debe filtrar los heredados);
//   - volumen razonable: ≤40 alertas en las últimas 24 h (backtest: 21,5/día);
//   - las descartadas por el usuario no aparecen en la vista por defecto.
//
// Se salta solo si la API no responde, igual que el resto del paquete:
//
//	API_URL=http://127.0.0.1:8888/api go test ./internal/entitycheck -run Alertas -v
type apiAlerta struct {
	ID        int64   `json:"id"`
	Valor     string  `json:"valor"`
	Tipo      string  `json:"tipo"`
	Periodo   string  `json:"periodo"`
	Hits      int     `json:"hits"`
	Baseline  float64 `json:"baseline"`
	Ratio     float64 `json:"ratio"`
	Score     float64 `json:"score"`
	Status    string  `json:"status"`
	CreatedAt string  `json:"created_at"`
}

type alertasResp struct {
	Alertas []apiAlerta `json:"alertas"`
	Total   int         `json:"total"`
	Nuevas  int         `json:"nuevas"`
}

func getAlertas(t *testing.T, params url.Values) alertasResp {
	t.Helper()
	var r alertasResp
	if err := json.Unmarshal(apiGet(t, "/alerts", params), &r); err != nil {
		t.Fatalf("parse /alerts: %v", err)
	}
	return r
}

func TestAlertasCalidad(t *testing.T) {
	p := url.Values{}
	p.Set("limit", "500")
	resp := getAlertas(t, p)
	if len(resp.Alertas) == 0 {
		t.Skip("sin alertas todavía: nada que verificar")
	}
	if resp.Total < len(resp.Alertas) {
		t.Errorf("total=%d < filas devueltas=%d (el total no está filtrado igual)", resp.Total, len(resp.Alertas))
	}

	// Volumen de las últimas 24 h, medido contra la alerta MÁS RECIENTE
	// (así no dependemos del reloj de la BD ni de la zona horaria).
	nuevas24 := 0
	maxCreada := ""
	for _, a := range resp.Alertas {
		if a.CreatedAt > maxCreada {
			maxCreada = a.CreatedAt
		}
	}
	if t0, err := time.Parse("2006-01-02 15:04", maxCreada); err == nil {
		for _, a := range resp.Alertas {
			if tc, err := time.Parse("2006-01-02 15:04", a.CreatedAt); err == nil && !tc.Before(t0.Add(-24*time.Hour)) {
				nuevas24++
			}
		}
	}

	incumplidas := 0
	for _, a := range resp.Alertas {
		minHits, minBl, minRatio := 5, 1.0, 5.0
		if a.Tipo == "tema" {
			minHits, minBl, minRatio = 6, 2.0, 6.0
		}
		// Se corta a 20 errores para no inundar la salida con 500 líneas.
		if incumplidas >= 20 {
			break
		}
		switch {
		case a.Hits < minHits:
			t.Errorf("alerta %q (%s) hits=%d < %d", a.Valor, a.Tipo, a.Hits, minHits)
			incumplidas++
		case a.Baseline < minBl:
			t.Errorf("alerta %q (%s) baseline=%.2f < %.1f", a.Valor, a.Tipo, a.Baseline, minBl)
			incumplidas++
		case a.Ratio < minRatio:
			t.Errorf("alerta %q (%s) ratio=%.1f < %.1f", a.Valor, a.Tipo, a.Ratio, minRatio)
			incumplidas++
		}
		if p := PoissonCola(a.Hits, a.Baseline); p >= 0.01 {
			t.Errorf("alerta %q (%s) no significativa: hits=%d baseline=%.2f p=%.4f >= 0,01",
				a.Valor, a.Tipo, a.Hits, a.Baseline, p)
			incumplidas++
		}
		if sev, motivo := Classify(a.Tipo, a.Valor); sev != SeverityNone {
			t.Errorf("alerta visible sobre FP %v %q (%s): %s", sev, a.Valor, a.Tipo, motivo)
			incumplidas++
		}
		if a.Status == "descartada" {
			t.Errorf("alerta descartada %q visible en la vista por defecto", a.Valor)
			incumplidas++
		}
		if a.Score <= 0 || math.IsNaN(a.Score) || math.IsInf(a.Score, 0) {
			t.Errorf("alerta %q con score inválido: %v", a.Valor, a.Score)
			incumplidas++
		}
	}

	if nuevas24 > 40 {
		t.Errorf("%d alertas en las últimas 24 h (>40): el umbral racionalizado da ~21/día; ¿umbral viejo desplegado?", nuevas24)
	}
	t.Logf("alertas=%d total=%d nuevas24=%d", len(resp.Alertas), resp.Total, nuevas24)
}

func TestAlertasFiltros(t *testing.T) {
	p := url.Values{}
	p.Set("tipo", "tema")
	p.Set("limit", "20")
	for _, a := range getAlertas(t, p).Alertas {
		if a.Tipo != "tema" {
			t.Fatalf("filtro tipo=tema devolvió %q", a.Tipo)
		}
	}

	p = url.Values{}
	p.Set("status", "descartada")
	p.Set("limit", "20")
	for _, a := range getAlertas(t, p).Alertas {
		if a.Status != "descartada" {
			t.Fatalf("filtro status=descartada devolvió %q", a.Status)
		}
	}
}
