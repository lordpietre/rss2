package main

import (
	"testing"
	"time"
)

func TestSanitizeDate(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		in   time.Time
		want time.Time
	}{
		{
			name: "fecha normal se respeta",
			in:   time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC),
			want: time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC),
		},
		{
			name: "archivo antiguo legitimo se respeta",
			in:   time.Date(2007, 3, 14, 8, 0, 0, 0, time.UTC),
			want: time.Date(2007, 3, 14, 8, 0, 0, 0, time.UTC),
		},
		{
			name: "futuro razonable (reloj del publisher adelantado) se respeta",
			in:   now.Add(50 * time.Minute),
			want: now.Add(50 * time.Minute),
		},
		{
			name: "futuro absurdo se sustituye por now",
			in:   time.Date(3000, 1, 1, 19, 54, 42, 0, time.UTC),
			want: now,
		},
		{
			name: "futuro de un feed con el reloj desfasado se sustituye por now",
			in:   now.Add(8 * time.Hour),
			want: now,
		},
		{
			name: "epoch de unix se sustituye por now",
			in:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
			want: now,
		},
		{
			name: "year 1 (zero time de Go) se sustituye por now",
			in:   time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
			want: now,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := sanitizeDate(c.in, now)
			if !got.Equal(c.want) {
				t.Errorf("sanitizeDate(%v) = %v, se esperaba %v", c.in, got, c.want)
			}
		})
	}
}

func TestResolveURL(t *testing.T) {
	base := "https://www.ejemplo.com/"

	cases := []struct {
		name string
		base string
		in   string
		want string
	}{
		{
			name: "URL absoluta no se toca",
			base: base,
			in:   "https://otro.com/a",
			want: "https://otro.com/a",
		},
		{
			name: "raiz relativa se resuelve contra el host",
			base: base,
			in:   "/tour",
			want: "https://www.ejemplo.com/tour",
		},
		{
			name: "ruta relativa se resuelve",
			base: base,
			in:   "noticias/x",
			want: "https://www.ejemplo.com/noticias/x",
		},
		{
			name: "URL sin esquema (protocolo relativo) se resuelve",
			base: base,
			in:   "//www.ejemplo.com/a",
			want: "https://www.ejemplo.com/a",
		},
		{
			name: "base invalida deja el link tal cual",
			base: "no-es-una-url",
			in:   "/tour",
			want: "/tour",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := resolveURL(c.base, c.in)
			if got != c.want {
				t.Errorf("resolveURL(%q, %q) = %q, se esperaba %q", c.base, c.in, got, c.want)
			}
		})
	}
}

func TestResolveURLEvitaColisionesDeID(t *testing.T) {
	// Dos feeds distintos que sirven "/tour" no deben generar el mismo id:
	// antes el id era el md5 del link crudo.
	a := generateID(resolveURL("https://a.com/feed", "/tour"))
	b := generateID(resolveURL("https://b.com/feed", "/tour"))

	if a == b {
		t.Errorf("mismo id para links relativos de feeds distintos: %s", a)
	}
}

// La fecha se guarda en una columna 'timestamp without time zone': si se
// deja la hora de pared de la zona del feed, Postgres se queda con ella y
// la noticia queda desfasada (una de un feed japones, 9 horas por delante).
// El ingestor normaliza a UTC antes de insertar.
func TestFechaSeNormalizaAUTC(t *testing.T) {
	now := time.Date(2026, 10, 27, 2, 0, 0, 0, time.UTC)

	// dc:date de un feed japones: +09:00.
	jst := time.FixedZone("JST", 9*3600)
	published := time.Date(2026, 10, 27, 10, 15, 29, 0, jst)

	got := sanitizeDate(published, now).UTC()

	want := time.Date(2026, 10, 27, 1, 15, 29, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("fecha = %v, se esperaba %v", got, want)
	}

	// Y el instante no cambia: solo cambia como se representa.
	if !sanitizeDate(published, now).Equal(published) {
		t.Errorf("sanitizeDate cambio el instante: %v != %v", sanitizeDate(published, now), published)
	}
}

func TestNormalizeURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "mismo articulo con esquema distinto",
			in:   "https://www.proycontra.com.pe/noticias/x",
			want: "proycontra.com.pe/noticias/x",
		},
		{
			name: "mismo articulo con www y http",
			in:   "http://www.proycontra.com.pe/noticias/x",
			want: "proycontra.com.pe/noticias/x",
		},
		{
			name: "barra final",
			in:   "https://ejemplo.com/noticias/x/",
			want: "ejemplo.com/noticias/x",
		},
		{
			name: "parametros de tracking",
			in:   "https://ejemplo.com/noticias/x?utm_source=rss&utm_medium=feed&id=7",
			want: "ejemplo.com/noticias/x?id=7",
		},
		{
			name: "orden de parametros irrelevante",
			in:   "https://ejemplo.com/a?b=2&a=1",
			want: "ejemplo.com/a?a=1&b=2",
		},
		{
			name: "puerto por defecto",
			in:   "https://ejemplo.com:443/a",
			want: "ejemplo.com/a",
		},
		{
			name: "host en mayusculas",
			in:   "https://EJEMPLO.COM/A",
			want: "ejemplo.com/A",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := normalizeURL(c.in); got != c.want {
				t.Errorf("normalizeURL(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestGenerateIDSameArticleDifferentURL(t *testing.T) {
	// Variantes del mismo articulo deben producir el MISMO id, o se
	// guardarian varias copias (y varios trabajos de traduccion).
	a := generateID("https://www.ejemplo.com/noticia")
	b := generateID("http://ejemplo.com/noticia/")
	c := generateID("https://ejemplo.com/noticia?utm_source=rss")

	if a != b || a != c {
		t.Errorf("ids distintos para el mismo articulo: %s %s %s", a, b, c)
	}

	// Articulos distintos deben producir ids distintos.
	d := generateID("https://www.ejemplo.com/otra-noticia")
	if a == d {
		t.Errorf("mismo id para articulos distintos: %s", a)
	}
}
