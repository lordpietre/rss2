package cache

import "testing"

// TestSearchKeyIncludesFilters comprueba que los filtros de busqueda forman
// parte de la clave de cache.
//
// Sin esto, una busqueda con ventana temporal o deduplicacion devuelve el
// resultado cacheado de la misma consulta sin filtros.
func TestSearchKeyIncludesFilters(t *testing.T) {
	base := SearchKey("noticias", "es", 1, 30)

	if base != SearchKey("noticias", "es", 1, 30) {
		t.Fatal("la clave debe ser estable")
	}
	if base == SearchKey("noticias", "es", 1, 30, "24h") {
		t.Fatal("el filtro de fecha debe cambiar la clave")
	}
	if base == SearchKey("noticias", "es", 1, 30, "24h", "cluster") {
		t.Fatal("varios filtros deben cambiar la clave")
	}
	if SearchKey("noticias", "es", 1, 30, "24h") == SearchKey("noticias", "es", 1, 30, "7d") {
		t.Fatal("filtros distintos deben dar claves distintas")
	}
	if SearchKey("noticias", "es", 1, 30, "24h", "cluster") == SearchKey("noticias", "es", 1, 30, "cluster", "24h") {
		t.Fatal("el orden de los filtros no debe colisionar")
	}
}
