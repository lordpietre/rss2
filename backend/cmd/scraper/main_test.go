package main

import "testing"

func TestIsFetchableURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{name: "https absoluta", in: "https://ejemplo.com/a", want: true},
		{name: "http absoluta", in: "http://ejemplo.com/a", want: true},
		{name: "raiz relativa", in: "/tour", want: false},
		{name: "protocolo relativo", in: "//ejemplo.com/a", want: false},
		{name: "ruta relativa", in: "noticias/x", want: false},
		{name: "vacia", in: "", want: false},
		{name: "sin esquema", in: "ftp://ejemplo.com/a", want: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isFetchableURL(c.in); got != c.want {
				t.Errorf("isFetchableURL(%q) = %v, se esperaba %v", c.in, got, c.want)
			}
		})
	}
}
