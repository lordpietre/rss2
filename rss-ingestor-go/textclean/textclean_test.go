package textclean

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// ---------------------------------------------------------------------------
// Casos reales capturados de la base (2026-09-30)
// ---------------------------------------------------------------------------

// Winnipeg Free Press: descripción con la plantilla de la página (pestañas,
// publicidad, botones sociales) y un bloque <script> con jQuery.
const winnipeg = "Appeals court weighs fate of judge’s contempt probe \t\t\t\n" +
	"\t\t\tBy: Michael Kunzelman, The Associated Press\n" +
	"\t\t\tPosted: 2:40 PM CDT Tuesday, Sep. 29, 2026\n" +
	"\tAdvertisement\n\t\t\t\t\tAdvertise with us\n" +
	"\t\t\t\t\tTweet\n\t Share \n\t Print \n\t Email \n\t Read Later \n" +
	"<script>jQuery('#add-credit-card-form').append('<input type=\"hidden\" />');</script>" +
	"\nThe Ninth Circuit heard arguments Tuesday in a case involving deportation flights."

// theregister: el resumen ENTERO es código JavaScript embebido.
const registerJS = "(function() { let windowUrl = window.location.href; " +
	"windowUrl = windowUrl.substring(windowUrl.indexOf('?') + 1); " +
	"let messageElement = document.querySelector('.shareableMessage'); " +
	"if (windowUrl && windowUrl.includes('code')) { messageElement.style.display = 'block'; } })();"

// mma.gob.cl: HTML literal (etiqueta escapada en el feed) dentro del texto.
const mma = "Publicado el 29 septiembre, 202629 septiembre, 2026" +
	"Puchuncaví culmina programa internacional de compostaje con balance positivo" +
	"<img width=\"1500\" height=\"936\" src=\"https://mma.gob.cl/wp/xvalp2909.jpg\" /> " +
	"La actividad se realizó en la comuna de Puchuncaví."

// Folha: página completa con menús repetidos (miles de caracteres).
const folha = "Painel\n                  \n                \n" +
	"Editado por Fábio Zanini, espaço traz notícias e bastidores da política.\n" +
	"Menu\nPolitica\nEconomia\nEsportes\n" +
	"Menu\nPolitica\nEconomia\nEsportes\n" +
	"Menu\nPolitica\nEconomia\nEsportes\n" +
	"Menu\nPolitica\nEconomia\nEsportes\n" +
	"Lula disse nesta quarta-feira que o país precisa crescer mais de 3,5% ao ano."

// aktuality.sk: bloques sin espacio entre ellos (texto pegado).
const aktuality = "<div>Pridajte si Šport.sk ako preferovaný zdroj</div>" +
	"<div>na Google</div><div>Pridať ako preferovaný zdroj na Google</div>" +
	"<p>Slovensko zdolalo Kazachstan 2:0 v Lige národov.</p>"

// Resumen normal: no debe tocarse (salvo espacios sobrantes).
const normal = `El Gobierno aprobó este martes un paquete de medidas fisuales por 2.500 millones de euros
que incluye ayudas para pymes y una rebaja del IVA en la factura eléctrica.

La oposición pidió comparecer en el Congreso para exigir explicaciones sobre el impacto presupuestario.`

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestBloquesNoSePegan(t *testing.T) {
	got := Clean(aktuality)
	if strings.Contains(got, "GooglePrida") || strings.Contains(got, "zdrojna") {
		t.Errorf("los bloques se han quedado pegados:\n%s", got)
	}
	if !strings.Contains(got, "Slovensko zdolalo Kazachstan 2:0") {
		t.Errorf("se perdió el contenido:\n%s", got)
	}
	if !strings.Contains(got, "\n") {
		t.Errorf("esperaba separación de bloques con saltos de línea:\n%q", got)
	}
	if strings.Contains(got, "<div>") || strings.Contains(got, "</p>") {
		t.Errorf("quedaron etiquetas:\n%s", got)
	}
}

func TestScriptYJavascriptFuera(t *testing.T) {
	got := Clean(winnipeg)
	if strings.Contains(got, "jQuery") || strings.Contains(got, "<script") || strings.Contains(got, "add-credit-card") {
		t.Errorf("el código JavaScript no se ha eliminado:\n%s", got)
	}
	if !strings.Contains(got, "Ninth Circuit heard arguments") {
		t.Errorf("se perdió la entradilla de la noticia:\n%s", got)
	}
}

func TestJavaScriptComoContenidoUnico(t *testing.T) {
	if got := Clean(registerJS); got != "" {
		t.Errorf("una noticia que es solo código debe quedar vacía, got: %q", got)
	}
}

func TestHTMLLiteralSeEliminaPeroElTextoNo(t *testing.T) {
	got := Clean(mma)
	if strings.Contains(got, "<img") || strings.Contains(got, "width=") || strings.Contains(got, "src=") {
		t.Errorf("quedó HTML literal:\n%s", got)
	}
	if !strings.Contains(got, "Puchuncaví culmina programa internacional") {
		t.Errorf("se perdió texto:\n%s", got)
	}
	if !strings.Contains(got, "La actividad se realizó en la comuna") {
		t.Errorf("se perdió texto tras la etiqueta:\n%s", got)
	}
}

func TestChromeDePagina(t *testing.T) {
	got := Clean(winnipeg)
	for _, noise := range []string{"Advertisement", "Advertise with us", "Tweet", "Read Later", "\t\t"} {
		if strings.Contains(got, noise) {
			t.Errorf("sobrevivió el ruido %q:\n%s", noise, got)
		}
	}
	if !strings.Contains(got, "By: Michael Kunzelman") {
		t.Errorf("la firma del artículo debería conservarse:\n%s", got)
	}
}

func TestMenusRepetidos(t *testing.T) {
	got := Clean(folha)
	if strings.Count(got, "Menu") > 1 {
		t.Errorf("los menús repetidos no se eliminaron:\n%s", got)
	}
	if !strings.Contains(got, "Lula disse nesta quarta-feira") {
		t.Errorf("se perdió el cuerpo de la noticia:\n%s", got)
	}
	if !strings.Contains(got, "Editado por Fábio Zanini") {
		t.Errorf("se perdió la entradilla (aparece una sola vez, no es ruido):\n%s", got)
	}
}

func TestTextoNormalConservado(t *testing.T) {
	got := Clean(normal)
	want := strings.Join([]string{
		"El Gobierno aprobó este martes un paquete de medidas fisuales por 2.500 millones de euros",
		"que incluye ayudas para pymes y una rebaja del IVA en la factura eléctrica.",
		"La oposición pidió comparecer en el Congreso para exigir explicaciones sobre el impacto presupuestario.",
	}, "\n")
	if got != want {
		t.Errorf("el texto normal debe conservarse intacto:\ngot  %q\nwant %q", got, want)
	}
}

func TestTextoPlanoSinHTML(t *testing.T) {
	const plain = "El dato < 5 por ciento y el otro > 3 por ciento siguen estables, dijo el banco."
	got := Clean(plain)
	if got != plain {
		t.Errorf("una comparación con < y > no debe modificarse:\ngot  %q\nwant %q", got, plain)
	}
}

func TestETagSinCierreNoTragaTexto(t *testing.T) {
	// "si el valor es <style se rompe": no hay </style>, no se parsea.
	const s = "El informe dice que si el valor es <style entonces se rompe el diseño de la página."
	got := Clean(s)
	if !strings.Contains(got, "entonces se rompe el diseño") {
		t.Errorf("se perdió texto tras un '<' sospechoso:\n%s", got)
	}
}

func TestEntidadesSeDescodificanUnaVez(t *testing.T) {
	got := Clean("Bajo 3,5% &amp; la inflación sube a &#8230; más")
	if strings.Contains(got, "&amp;") || strings.Contains(got, "&#8230;") {
		t.Errorf("las entidades no se descodificaron: %q", got)
	}
	if !strings.Contains(got, "3,5%") || !strings.Contains(got, "inflación") {
		t.Errorf("texto perdido: %q", got)
	}
}

func TestDobleEscapeSeConserva(t *testing.T) {
	// Entrada doblemente escapada: no se toca (garantiza idempotencia).
	const s = "literal &amp;lt;script&amp;gt; en el texto"
	got := Clean(s)
	if got != s {
		t.Errorf("una doble entidad no debe descodificarse dos veces:\ngot  %q\nwant %q", got, s)
	}
}

func TestTitulo(t *testing.T) {
	cases := map[string]struct{ in, want string }{
		"entidades":     {"La inflación baja al 2,1% &amp; el paro &#8230;", "La inflación baja al 2,1% & el paro …"},
		"etiquetas":     {"<b>Breaking:</b> tiroleses ganan <a href=\"#\">la etapa</a>", "Breaking: tiroleses ganan la etapa"},
		"html escapado": {"&lt;h1&gt;Titular&lt;/h1&gt;", "Titular"},
		"espacios":      {"  Hola    mundo \n y otra  ", "Hola mundo y otra"},
	}
	for name, c := range cases {
		if got := CleanTitle(c.in); got != c.want {
			t.Errorf("%s:\ngot  %q\nwant %q", name, got, c.want)
		}
	}
}

func TestTituloTope(t *testing.T) {
	long := strings.Repeat("palabra ", 200)
	got := CleanTitle(long)
	if len(got) > DefaultMaxTitleLen {
		t.Errorf("título demasiado largo: %d > %d", len(got), DefaultMaxTitleLen)
	}
	if !utf8.ValidString(got) {
		t.Error("el título cortado no es UTF-8 válido")
	}
	if strings.HasSuffix(got, "pala") {
		t.Error("el corte debe evitar partir una palabra")
	}
}

func TestTruncadoEnFronteraDeFrase(t *testing.T) {
	s := strings.Repeat("Esto es una frase larga que suma. ", 500) // > 4000 car.
	got := Clean(s)
	if len(got) > DefaultMaxLen {
		t.Errorf("la salida supera el tope: %d > %d", len(got), DefaultMaxLen)
	}
	if !utf8.ValidString(got) {
		t.Error("el texto cortado no es UTF-8 válido")
	}
	if !strings.HasSuffix(got, ".") {
		t.Errorf("el corte debería caer en frontera de frase, got tail %q", got[len(got)-40:])
	}
}

func TestIdempotencia(t *testing.T) {
	fixtures := map[string]string{
		"winnipeg":  winnipeg,
		"register":  registerJS,
		"mma":       mma,
		"folha":     folha,
		"aktuality": aktuality,
		"normal":    normal,
		"plain":     "El dato < 5 por ciento y el otro > 3 por ciento siguen estables.",
		"double":    "literal &amp;lt;script&amp;gt; en el texto",
		"entities":  "Bajo 3,5% &amp; la inflación sube a &#8230; más",
		"long":      strings.Repeat("Esto es una frase larga que suma. ", 500),
		// El corte a 4000 cae justo tras "//" y el trozo final queda como
		// comentario sin letras (ruido): ver TestCorteNoDejaComentario.
		"corte":  strings.Repeat("x", 3996) + "\n// resto del comentario con letras y más texto para que siga.",
		"empty":  "",
		"blank":  "   \n\t  ",
		"tags":   "<p>uno</p><p>dos</p><p>tres</p>",
		"oneTag": "texto <img src=\"x.jpg\" alt=\"y\"> final",
	}
	for name, in := range fixtures {
		once := Clean(in)
		twice := Clean(once)
		if once != twice {
			t.Errorf("%s: Clean no es idempotente\n1ª: %q\n2ª: %q", name, once, twice)
		}
	}
}

func TestCorteNoDejaComentario(t *testing.T) {
	// El corte en frontera de frase puede partir una línea y dejar un
	// fragmento que ya es ruido (p. ej. un "//"). Sin volver a filtrar ese
	// trozo, Clean(x) != Clean(Clean(x)) y el resumen terminaría en "//".
	in := strings.Repeat("x", 3996) + "\n// resto del comentario con letras y más texto para que siga."

	out := Clean(in)
	if strings.HasSuffix(out, "//") {
		t.Errorf("el corte dejó un comentario sin letras al final: %q", sufijo(out, 120))
	}
	if again := Clean(out); again != out {
		t.Errorf("no idempotente\n1ª: %q\n2ª: %q", trunc(out, 120), trunc(again, 120))
	}
}

func TestLimitesDefensivos(t *testing.T) {
	// Entrada enorme: se acota, no se cuelga.
	huge := strings.Repeat(strings.Repeat("a", 80)+"\n", 40000) // ~3,4 MB
	if got := Clean(huge); len(got) > DefaultMaxInput {
		t.Errorf("la entrada no se acotó: %d", len(got))
	}

	// HTML malformado y binario-raro: no debe panic.
	for _, s := range []string{
		"<<<<<>>>>", "<p><div><span><a href=>", "\x00\x01\x02 etiqueta <b> ",
		strings.Repeat("<div>", 5000), strings.Repeat("&", 5000) + "amp;",
		"<script>", "<script>foo", strings.Repeat("<a href=\"x\">", 2000),
	} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic con %q: %v", s[:min(len(s), 40)], r)
				}
			}()
			_ = Clean(s)
		}()
	}
}

func TestOpcionesExplicitas(t *testing.T) {
	got := CleanWith("Menu\nContenido real\nMenu\n", Options{MaxInput: 1024, MaxLen: 100, StripLines: false})
	if !strings.Contains(got, "Menu") {
		t.Errorf("con StripLines=false no se deben descartar líneas: %q", got)
	}
}

func TestJSONLDComoContenido(t *testing.T) {
	// Feed cuyo "cuerpo" es el JSON-LD del <head>, pegado además al CSS de la
	// página (caso real de Yahoo Sports): se recupera la prosa.
	const ld = `{"articleSection":"Yahoo Sports","author":{"jobTitle":"","name":"Daniel Chavkin"},` +
		`"dateModified":"2026-09-30T00:01:02Z",` +
		`"description":"Here's a look at McCarthy's new number with the Giants, after he changed it this week.",` +
		`"headline":"J.J. McCarthy chooses new number"}` +
		`Advertisement#_R_3dplh9 { display: none; }` +
		`@media screen and (min-width: 768px) { #_R_3dplh9 { display: flex; } }`

	got := Clean(ld)
	if !strings.Contains(got, "McCarthy's new number") {
		t.Errorf("no se extrajo la descripción del JSON: %q", got)
	}
	if strings.Contains(got, "@type") || strings.Contains(got, `"author"`) {
		t.Errorf("quedó JSON: %q", got)
	}
}

func TestJSONSinTextoSeVacia(t *testing.T) {
	// Miga de pan (schema.org): no hay prosa, no debe sobrevivir.
	const ld = `{"@context":"https://schema.org","@type":"BreadcrumbList","itemListElement":[` +
		`{"@type":"ListItem","position":1,"item":{"@id":"https://x.es/culture","name":"Culture"}},` +
		`{"@type":"ListItem","position":2,"item":{"@id":"https://x.es/movies","name":"Movies"}}]}`

	if got := Clean(ld); got != "" {
		t.Errorf("la miga de pan debe quedar vacía, got %q", got)
	}
}

func TestCSFSueltoSinEtiquetas(t *testing.T) {
	// CSS minificado que llega como texto plano (sin <style>): todo es ruido.
	lines := []string{
		".tdi_2{min-height:0}",
		".tdi_4,.tdi_4 .tdc-columns{min-height:0}.tdi_4 .tdc-columns{display:block}",
		".tdb-breadcrumbs{margin-bottom:11px;font-family:var(--x);color:#747474;line-height:18px}",
		"@media screen and (max-width: 767px) {",
		"  .foo { display: flex; }",
		"}",
		"color: red;",
	}
	in := strings.Join(lines, "\n")
	if got := Clean(in); got != "" {
		t.Errorf("el CSS suelto debe eliminarse por completo, got %q", got)
	}
}

func TestCodigoDesnudo(t *testing.T) {
	in := strings.Join([]string{
		"Entra aquí para ver el partido en directo.",
		"} else {",
		"{{article.titulo}}",
		"window.open(`fb-messenger://share?link=https://x.es&app_id=1`);",
		"(adsbygoogle = window.adsbygoogle || []).push({});",
		"\"url\": \"https://www.folha.com.br/colunas/x.shtml\",",
		"Texto final de la entradilla.",
	}, "\n")

	got := Clean(in)
	for _, noise := range []string{"} else {", "{{article", "window.open", "adsbygoogle", `"url"`} {
		if strings.Contains(got, noise) {
			t.Errorf("sobrevivió el código %q:\n%s", noise, got)
		}
	}
	if !strings.Contains(got, "Entra aquí para ver el partido") || !strings.Contains(got, "Texto final") {
		t.Errorf("se perdió texto legítimo:\n%s", got)
	}
}

func TestLlaveInicialConservaLaProsa(t *testing.T) {
	in := strings.Join([]string{
		"}.post-meta-r2 .is-flex .gem-bubble-wrap {order: 3;margin-left: auto;",
		"}Σχετικό άρθρο για την εποχή των νέων media.",
		"Fin del texto de la entradilla.",
	}, "\n")

	got := Clean(in)
	if strings.Contains(got, "post-meta") || strings.Contains(got, "margin-left") {
		t.Errorf("sobrevivió CSS: %q", got)
	}
	if !strings.Contains(got, "Σχετικό άρθρο") || !strings.Contains(got, "Fin del texto") {
		t.Errorf("se perdió la prosa tras la llave: %q", got)
	}
}

func TestMenuSocial(t *testing.T) {
	in := "Titular del artículo.\n\nFacebook\nX\nPinterest\nTumblr\nReddit\nWhatsApp\nCopy URL\nURL is copied.\n\nEntra aquí para leerlo."
	got := Clean(in)
	for _, noise := range []string{"Pinterest", "Tumblr", "Reddit", "WhatsApp", "Copy URL", "URL is copied"} {
		if strings.Contains(got, noise) {
			t.Errorf("sobrevivió el menú social %q: %s", noise, got)
		}
	}
	if !strings.Contains(got, "Titular del artículo") || !strings.Contains(got, "Entra aquí") {
		t.Errorf("se perdió texto legítimo: %s", got)
	}
}

func TestLimpiaCodigoPegadoALaProsa(t *testing.T) {
	// Línea gigante: el CMS junta entradilla, firma y un fragmento de JS sin
	// saltos de línea. No se puede descartar entera (se borraría la noticia).
	frase := "El estudio describe cómo la publicidad programática cambia el mercado de medios. "
	prosa := strings.Repeat(frase, 6)
	in := prosa[:200] +
		"googletag.cmd.push(function() { googletag.display('div-gpt-ad-leaderboard4'); });" +
		prosa

	got := Clean(in)
	if !strings.Contains(got, "El estudio describe") {
		t.Errorf("se perdió la prosa: %q", trunc(got, 200))
	}
	if strings.Contains(got, "googletag") || strings.Contains(got, "display(") {
		t.Errorf("no se recortó el código pegado: %q", trunc(got, 320))
	}
	if again := Clean(got); again != got {
		t.Errorf("no idempotente\n1ª: %q\n2ª: %q", trunc(got, 200), trunc(again, 200))
	}
}

func TestNoTocaParensDeLaProsa(t *testing.T) {
	// Un paréntesis con versión no es código.
	in := strings.Repeat("El informe habla de la versión v2.0 (beta) del sistema operativo. ", 8)
	got := Clean(in)
	if !strings.Contains(got, "v2.0 (beta)") {
		t.Errorf("se comió un paréntesis de la prosa: %q", trunc(got, 200))
	}
}

func FuzzClean(f *testing.F) {
	f.Add(winnipeg)
	f.Add(mma)
	f.Add("<p>hola</p><script>x()</script>")
	f.Add("&amp;lt;script&amp;gt;")
	f.Add(strings.Repeat("palabra ", 600))
	f.Fuzz(func(t *testing.T, s string) {
		out := Clean(s)
		if !utf8.ValidString(out) {
			t.Fatalf("salida no UTF-8 válida para %q", s)
		}
		if len(out) > DefaultMaxInput {
			t.Fatalf("salida demasiado larga: %d", len(out))
		}
		if Clean(out) != out {
			t.Fatalf("no idempotente para %q -> %q -> %q", s, out, Clean(out))
		}
	})
}

// trunc y sufijo recortan textos largos para que los mensajes de error sean
// legibles (solo se usan en tests).
func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func sufijo(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}
