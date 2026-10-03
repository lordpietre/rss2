package entitycheck

import "testing"

// Casos reales extraídos del scan de los 303.749 valores de `tags`
// (todos los países y tipos). "quiere" = debe bloquearse; "no" = es legítimo
// y bloquearlo sería un falso positivo nuestro.
func TestClassify(t *testing.T) {
	cases := []struct {
		nombre string
		tipo   string
		valor  string
		quiere Severity
		motivo string // opcional
	}{
		// ---- Basura mecánica: tiene que bloquearse ------------------------
		{"persona genérica", "persona", "Fuente", SeverityStrong, "palabra genérica"},
		{"org genérica", "organizacion", "Más", SeverityStrong, "palabra genérica"},
		{"lugar genérico", "lugar", "Leer", SeverityStrong, "palabra genérica"},
		{"hora pegada", "persona", "8)00:39Absolute Traumtor Italia", SeverityStrong, "hora/fecha pegada"},
		{"hora pegada lugar", "lugar", "metros00:58El", SeverityStrong, "hora/fecha pegada"},
		{"fecha sin límite izq.", "lugar", "BILD27.09.2026", SeverityStrong, "hora/fecha pegada"},
		{"hora suelta", "lugar", "9:30 am", SeverityStrong, "hora/fecha pegada"},
		{"números con P", "lugar", "P97.67", SeverityStrong, "hora/fecha pegada"},
		{"marcador", "lugar", "Georgia 0-1 Irlanda", SeverityStrong, "marcador"},
		{"marcador org", "organizacion", "Italia 0-2 Bélgica", SeverityStrong, "marcador"},
		{"fecha pegada", "persona", "Núria Orriols Guiu26-09-2026", SeverityStrong, "fecha pegada"},
		{"url", "tema", "www.ertnews.gr", SeverityStrong, "url/email"},
		{"email", "persona", "mattbirney@bullbears.com.au", SeverityStrong, "url/email"},
		{"tld", "organizacion", "Motorsport.com", SeverityStrong, "url/email"},
		{"paréntesis", "organizacion", "SustainSoft): Sustainsoft", SeverityStrong, "símbolos"},
		{"cifras con %", "organizacion", "Cámara El 100% del Senado", SeverityStrong, "símbolos"},
		{"recortado", "organizacion", "Consejo de Administración de la", SeverityStrong, "termina en función (cortado)"},
		{"recortado tema", "tema", "El cuerpo de", SeverityStrong, "termina en función (cortado)"},
		{"recortado lugar", "lugar", "Banco central de", SeverityStrong, "termina en función (cortado)"},
		{"titular largo", "tema", "Aquí está lo que está llegando a los titulares hoy", SeverityStrong, "frase muy larga"},
		{"solo puntuación", "tema", "· ¿", SeverityStrong, "solo puntuación"},
		{"espacios dobles", "persona", "  Pedro Sánchez ", SeverityStrong, "espacios"},
		{"vacío", "persona", "", SeverityStrong, "vacío"},

		// ---- CTA/plantilla de la fuente (chrome de página) -----------------
		{"cta clic", "organizacion", "Haga clic", SeverityStrong, "cta/plantilla web"},
		{"cta tema", "tema", "Leer más", SeverityStrong, "cta/plantilla web"},
		{"cta legal", "tema", "Política de privacidad", SeverityStrong, "cta/plantilla web"},
		{"cta términos", "tema", "Términos de uso", SeverityStrong, "cta/plantilla web"},
		{"cta publicidad", "tema", "Publicidad", SeverityStrong, "cta/plantilla web"},
		{"cta inglesa", "tema", "Read more", SeverityStrong, "cta/plantilla web"},

		// ---- Legítimos: bloquearlos sería un FP nuestro --------------------
		{"nombre normal", "persona", "Pedro Sánchez", SeverityNone, ""},
		{"homógrafo de una palabra", "lugar", "Pará", SeverityNone, ""},
		{"nombre con &", "organizacion", "Rohde & Schwarz", SeverityNone, ""},
		{"nombre largo", "organizacion", "Asamblea General de las Naciones Unidas", SeverityNone, ""},
		{"nombre 8 palabras", "organizacion", "Consejo de Seguridad de las Naciones Unidas", SeverityWeak, ""},
		{"país", "lugar", "España", SeverityNone, ""},
		{"serie con A", "organizacion", "La Liga Serie A", SeverityNone, ""},
		{"millar español", "tema", "1.100 candidatos", SeverityNone, ""},
		{"año-rango", "tema", "temporada 2026-27", SeverityNone, ""},
		{"año suelto", "tema", "Juegos Asiáticos 2026", SeverityNone, ""},
		{"abreviatura", "lugar", "EE.UU", SeverityNone, ""},
		{"dominio real", "organizacion", "Amazon.co.jp", SeverityNone, ""},
		{"apodo con dígito", "persona", "CR7", SeverityNone, ""},
		{"tema deportivo", "tema", "fútbol", SeverityNone, ""},
		{"nombre propio interno", "persona", "LeBron James", SeverityNone, ""},
		{"suscripción real", "tema", "Suscripción digital", SeverityNone, ""},
		{"más que no es cta", "tema", "Más allá", SeverityNone, ""},
		{"anuncian", "tema", "Anuncian nuevo plan", SeverityNone, ""},

		// ---- Débiles: se informan, no se bloquean --------------------------
		{"pegado", "persona", "TheHindu Businessline", SeverityWeak, "palabras pegadas"},
		{"pegado2", "tema", "WhatsApp", SeverityWeak, "palabras pegadas"},
		{"frase 7 palabras", "lugar", "Festival de la Ruta Cultural de Estambul", SeverityWeak, "frase (7-8 palabras)"},
		{"artículo inicial", "organizacion", "La Asamblea General de los Estados Unidos", SeverityWeak, "empieza por artículo"},
	}

	for _, c := range cases {
		t.Run(c.nombre, func(t *testing.T) {
			sev, motivo := Classify(c.tipo, c.valor)
			if sev != c.quiere {
				t.Errorf("Classify(%q, %q) = %v/%q, quiere %v", c.tipo, c.valor, sev, motivo, c.quiere)
			}
			if c.motivo != "" && motivo != c.motivo {
				t.Errorf("motivo = %q, quiere %q", motivo, c.motivo)
			}
			if want := c.quiere == SeverityStrong; IsFalsePositive(c.tipo, c.valor) != want {
				t.Errorf("IsFalsePositive = %v, quiere %v", !want, want)
			}
		})
	}
}

// El motivo tiene que ser determinista (lo usa entityscan para etiquetar la
// blocklist y el informe).
func TestClassifyDeterminista(t *testing.T) {
	for i := 0; i < 50; i++ {
		a, ma := Classify("persona", "8)00:39Absolute Traumtor Italia")
		b, mb := Classify("persona", "8)00:39Absolute Traumtor Italia")
		if a != b || ma != mb {
			t.Fatalf("no determinista: %v/%q vs %v/%q", a, ma, b, mb)
		}
	}
}

// Tolerancia UTF-8: no debe panickear con cualquier basura (el fuzz de
// textclean cubre el otro lado; aquí solo el umbral).
func TestClassifyNoPanic(t *testing.T) {
	valores := []string{
		"\x00\x01\x02", "日本語のテキスト", "🙂🙂🙂", strings_Repeat("a", 5000),
		"   ", "-", "0-0", "12:34:56", "((((((((((", "ЁЁЁ",
	}
	for _, v := range valores {
		for _, tipo := range []string{"persona", "organizacion", "lugar", "tema", "sistema", ""} {
			Classify(tipo, v)
		}
	}
}

func strings_Repeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
