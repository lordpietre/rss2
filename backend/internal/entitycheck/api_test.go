package entitycheck

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"
)

// Estos tests hacen lo mismo que la página de Populares contra la API real
// (lo más popular de España y una pasada global) y comprueban con las mismas
// reglas que usa la API para filtrar que no se cuele ningún FP fuerte.
//
// Se saltan solos si la API no está disponible, así `make test` sigue
// sirviendo para desarrollo sin levantar el stack.
//
//	API_URL=http://127.0.0.1:8888/api go test ./internal/entitycheck -v

type apiEntity struct {
	Valor string `json:"valor"`
	Tipo  string `json:"tipo"`
	Count int    `json:"count"`
}

type entitiesResp struct {
	Entities  []apiEntity `json:"entities"`
	Total     int         `json:"total"`
	Page      int         `json:"page"`
	TotalPage int         `json:"total_pages"`
}

type country struct {
	ID     int    `json:"id"`
	Nombre string `json:"nombre"`
}

func apiBase() string {
	if v := os.Getenv("API_URL"); v != "" {
		return v
	}
	return "http://127.0.0.1:8888/api"
}

func apiGet(t *testing.T, path string, params url.Values) []byte {
	t.Helper()
	u := apiBase() + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		t.Skipf("API no disponible (%s): %v", apiBase(), err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		t.Fatalf("leyendo %s: %v", u, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s -> HTTP %d: %s", u, resp.StatusCode, cut(string(body), 300))
	}
	return body
}

// checkPopulares repite la consulta de la pestaña Populares y falla si algún
// FP fuerte vuelve a ser visible para el usuario.
func checkPopulares(t *testing.T, nombre string, params url.Values) {
	t.Helper()
	var r entitiesResp
	if err := json.Unmarshal(apiGet(t, "/entities", params), &r); err != nil {
		t.Fatalf("%s: decodificando respuesta: %v", nombre, err)
	}
	if r.Total == 0 {
		t.Errorf("%s: /entities devuelve 0 entidades (¿parámetros rotos?)", nombre)
		return
	}

	var fuertes []string
	debiles := 0
	for _, e := range r.Entities {
		sev, motivo := Classify(e.Tipo, e.Valor)
		switch sev {
		case SeverityStrong:
			fuertes = append(fuertes,
				fmt.Sprintf("  [%s] %-11s %4d menc. %q", motivo, e.Tipo, e.Count, e.Valor))
		case SeverityWeak:
			debiles++
		}
	}
	t.Logf("%-34s total=%-5d visibles=%-3d débiles=%d", nombre, r.Total, len(r.Entities), debiles)
	for _, f := range fuertes {
		t.Logf("  visible: %s", f)
	}
	if len(fuertes) > 0 {
		t.Errorf("%s: %d FP fuertes visibles (ejecuta `make entities-scan-apply`): \n%s",
			nombre, len(fuertes), join(fuertes, "\n"))
	}
}

// Lo más popular de España: personas y organizaciones, igual que la pestaña.
func TestPopularesEspana(t *testing.T) {
	var countries []country
	if err := json.Unmarshal(apiGet(t, "/countries", nil), &countries); err != nil {
		t.Fatalf("leyendo /countries: %v", err)
	}
	id := 0
	for _, c := range countries {
		if c.Nombre == "España" {
			id = c.ID
		}
	}
	if id == 0 {
		t.Fatal("/countries no devuelve España")
	}

	for _, tipo := range []string{"persona", "organizacion"} {
		p := url.Values{}
		p.Set("tipo", tipo)
		p.Set("country_id", strconv.Itoa(id))
		p.Set("page", "1")
		p.Set("per_page", "50")
		checkPopulares(t, "España · "+tipo, p)
	}
}

// Pasada global: todos los países, los cuatro tipos con los que trabaja
// Populares (persona, organización, lugar y tema).
func TestPopularesGlobal(t *testing.T) {
	for _, tipo := range []string{"persona", "organizacion", "lugar", "tema"} {
		p := url.Values{}
		p.Set("tipo", tipo)
		p.Set("page", "1")
		p.Set("per_page", "50")
		checkPopulares(t, "global · "+tipo, p)
	}
}

func cut(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func join(items []string, sep string) string {
	out := ""
	for i, s := range items {
		if i > 0 {
			out += sep
		}
		out += s
	}
	return out
}
