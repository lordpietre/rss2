// Package textclean normaliza y sanea el texto que llega de feeds RSS y del
// scraping de páginas web antes de guardarlo en la base de datos.
//
// Garantías:
//
//   - Seguridad: nunca se usa regex para "quitar HTML"; el HTML se parsea con
//     x/net/html. Los regex auxiliares usan RE2 (Go), con tiempo lineal, por lo
//     que no hay riesgo de ReDoS aunque el contenido venga de un feed hostil.
//     La entrada se acota (MaxInput) antes de parsear.
//   - Idempotencia: Clean(Clean(x)) == Clean(x). Importante para poder re-ejecutar
//     el backfill sobre las noticias ya existentes sin perder información.
//   - Reversibilidad: el llamante puede guardar el original (p.ej. en
//     noticias.resumen_raw) porque Clean no muta nada más que el texto devuelto.
//   - Minimismo: solo se elimina ruido identificable de forma segura (chrome de
//     página, menús, código, líneas vacías/URLs sueltas). Una duda conserva el
//     texto.
package textclean

import (
	"encoding/json"
	stdhtml "html"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
)

const (
	// DefaultMaxInput limita el texto crudo que se llega a parsear.
	DefaultMaxInput = 256 << 10 // 256 KB
	// DefaultMaxLen es la longitud máxima del resumen limpio.
	DefaultMaxLen = 4000
	// DefaultMaxTitleLen es la longitud máxima del título limpio.
	DefaultMaxTitleLen = 500

	// maxRepeat/maxRepeatLen: una línea corta repetida muchas veces dentro del
	// mismo documento es menú o plantilla, no contenido.
	maxRepeat    = 4
	maxRepeatLen = 160

	// mixedLen: por encima de esta longitud una línea casi nunca es "solo
	// ruido": hay CMS que pegan el artículo entero en un renglón y entonces
	// descartar la línea entera borraría texto real.
	mixedLen = 400
	// ratioProsa: fracción de espacios por encima de la cual una línea así es
	// texto corrido (el CSS/JSON/JS minificado casi no lleva espacios).
	ratioProsa = 0.05
)

// Options configura Clean. El valor cero usa los defaults.
type Options struct {
	// MaxInput acota el texto crudo antes de parsearlo.
	MaxInput int
	// MaxLen acota el texto de salida (cortando en frontera de frase).
	MaxLen int
	// StripLines activa el filtro de líneas de ruido (desactivado en títulos).
	StripLines bool
}

var (
	// Etiqueta HTML. RE2 => tiempo lineal, sin backtracking catastrófico.
	tagRe = regexp.MustCompile(`</?[a-zA-Z][^<>]{0,511}>`)
	// Nombres de elementos que ponen al parser en modo "texto crudo": si aparecen
	// sin su cierre, parsear haría que se tragara el resto del documento.
	rawTextTags = []string{"script", "style", "textarea", "title", "xmp", "iframe", "noembed", "noframes", "noscript"}

	spaceRe = regexp.MustCompile(`[ \t\f\v]+`)

	// Marcadores de código (JS/CSS/JSON) embebidos en la descripción del feed.
	codeRe = regexp.MustCompile(`(function\s*\(|jQuery\s*\(|document\.(querySelector|getElementById|location|addEventListener|cookie)|window\.(location|onload|addEventListener)|window\.[A-Za-z_$][\w$]*\s*\(|\.push\s*\(|\.classList\.|\.focus\s*\(|\$\{[^{}]{1,120}\}|\{\{[^{}]{0,160}\}\}|\{\}|/\*|<!\[CDATA\[|\btry\s*[\({]|\bcatch\s*[\({]|\b(var|let|const)\s+[A-Za-z_$][A-Za-z0-9_$]*\s*(=[^=]|;)|=>\s*\{|}\s*;|\bif\s*\(\s*[A-Za-z_$][\w$]*\s*(===|==|!==|!=|<=?|>=?)\s*)`)

	// Fragmento de etiqueta sin '>' con el que completarla (p. ej. "<img
	// srcset=…414w, …"): tagRe exige el cierre, aquí no existe.
	tagStartRe = regexp.MustCompile(`^<[a-zA-Z/]`)
	// Llamada a función con punto al principio de la línea: "googletag.display(",
	// "document.body.appendChild("… La prosa jamás empieza "palabra.palabra(".
	callRe = regexp.MustCompile(`^\s*[A-Za-z_$][\w$]*(\.[A-Za-z_$][\w$]*)+\s*\(`)
	// Asignación al principio de la línea: "lastScrollTop = …" o
	// "window._taboola = …".
	assignRe = regexp.MustCompile(`^\s*[A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*)*\s*=[^=]`)
	// Cualquier llamada con punto en cualquier sitio ("googletag.push(…)").
	callSpanRe = regexp.MustCompile(`[A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*)+\s*\(`)
	// Identificador camelCase con dos jorobas (lastScrollTop, visibleItemCount):
	// es código, no prosa. "iPhone" o "McCarthy" no encajan.
	camelRe = regexp.MustCompile(`\b[a-z]+[A-Z][a-z]+[A-Z]`)
	// Cabeceras de línea que solo puede tener CSS: "@media…" y ".foo{" / "#id{".
	atRuleRe  = regexp.MustCompile(`^@[A-Za-z-]+`)
	cssHeadRe = regexp.MustCompile(`^[.#][A-Za-z0-9_-]`)

	// Línea que es JSON: `"clave": valor,` (feeds que meten JSON-LD o
	// meta-tagging como si fuera el cuerpo del artículo).
	jsonKeyRe = regexp.MustCompile(`^\s*"[^"]{1,120}"\s*:`)
	// Interpolación de plantilla: `{{titulo}}`, `{{#if (eq x "y")}}`, `${title}`.
	tmplRe = regexp.MustCompile(`^\s*\$?[{]{1,3}[^{}]{1,160}[}]{1,3}\s*$`)
	// Código "desnudo" sin palabras: `} else {`, `});`, `}); catch`…
	braceTailRe = regexp.MustCompile(`^\s*[)}\];]+\s*(else|catch|finally)?\s*[{(]?\s*$`)
	// Declaración CSS/JS suelta en una línea (CSS multilínea llegado el caso):
	// propiedad en minúsculas: valor; — sin segundos ':' para no tocar horarios.
	declRe = regexp.MustCompile(`^[a-z][a-z0-9-]{0,30}\s*:\s*[^;:]{1,120};$`)

	// Declaración/Regla CSS o JSON minificada: {... : o ; ...} en la misma
	// línea. Hay feeds cuyo contenido es CSS suelto SIN etiqueta <style>
	// (p. ej. plantillas de WordPress), así que no pasa por el parser.
	braceCodeRe = regexp.MustCompile(`[{][^{}]{0,500}(?:;|:)[^{}]{0,500}[}]`)
	// Selector de CSS a final de línea: ".foo {" o "@media (...) {".
	selectorRe = regexp.MustCompile(`^[^{}]{0,180}\{\s*$`)

	// Líneas que son únicamente chrome de página (ancladas a la línea entera:
	// nunca tocan una frase que simplemente contenga la palabra).
	uiRe = regexp.MustCompile(`(?i)^(\W|\d){0,12}` +
		`(menu|menú|menü|navig\w*|search|buscar|share|compartir|partager|teilen|tweet|twittear|` +
		`print|imprimir|drucken|email|e-?mail|subscribe|suscrib\w*|abonn\w*|newsletter|sign in|log in|login|` +
		`register|registro|registrieren|comment\w*|comentarios|commentaires|read more|leer más|mehr lesen|` +
		`read later|guardar|related|relacionados|ver también|más noticias|more stories|more|follow us|síguenos|` +
		`copyright|todos los derechos reservados|all rights reserved|privacy|privacidad|terms|condiciones|` +
		`cookies?|advertisement|publicidad|advertising|sponsor\w*|patrocinad\w*|next|previous|siguiente|anterior|` +
		`home|inicio|contacto|contact|about|acerca|download|descargar|back to top|volver arriba|close|cerrar|` +
		`page|páginas?|share this|compartir esto|imprimir página|print page|breaking news|última hora|` +
		`trending|popular|destacados|seguir leyendo|continue reading|ensuite|suite|` +
		`pinterest|linkedin|tumblr|reddit|whatsapp|telegram|flipboard|messenger|` +
		`password|contraseña)(\W|\d){0,12}$`)

	// uiPhrases: lo mismo, pero como frase completa (una o varias palabras),
	// comparada en minúsculas y sin decoración. Cubre casos como
	// "Advertise with us" o "Read Later".
	uiPhrases = map[string]bool{
		"menu": true, "menú": true, "menüs": true, "menü": true, "meni": true,
		"navegación": true, "navigation": true, "nav": true,
		"search": true, "buscar": true, "búsqueda": true, "busqueda": true,
		"share": true, "compartir": true, "share this": true, "compartir esto": true,
		"tweet": true, "tweets": true, "twittear": true, "twitter": true,
		"print": true, "imprimir": true, "print page": true, "imprimir página": true,
		"email": true, "e-mail": true, "correo": true,
		"subscribe": true, "suscribir": true, "suscríbete": true, "suscribete": true,
		"subscribe to our newsletter": true, "newsletter": true, "boletín": true,
		"sign in": true, "sign up": true, "log in": true, "login": true,
		"register": true, "registrarse": true, "registro": true,
		"comments": true, "comment": true, "comentarios": true, "commentaires": true,
		"read more": true, "leer más": true, "lea más": true, "read later": true,
		"continuar leyendo": true, "continue reading": true, "seguir leyendo": true,
		"related": true, "related articles": true, "relacionados": true, "ver también": true,
		"más noticias": true, "more news": true, "more stories": true, "more": true,
		"follow us": true, "síguenos": true, "siguenos": true, "seguirnos": true,
		"copyright": true, "todos los derechos reservados": true, "all rights reserved": true,
		"privacy": true, "privacidad": true, "terms": true, "terms and conditions": true,
		"condiciones": true, "cookies": true, "cookie": true, "política de cookies": true,
		"advertisement": true, "advertise with us": true, "advertising": true,
		"publicidad": true, "anuncio": true, "sponsored": true, "patrocinado": true,
		"next": true, "previous": true, "siguiente": true, "anterior": true,
		"home": true, "inicio": true, "contact": true, "contacto": true,
		"about": true, "acerca de": true, "download": true, "descargar": true,
		"back to top": true, "volver arriba": true, "close": true, "cerrar": true,
		"page": true, "página": true, "páginas": true, "paginas": true,
		"breaking news": true, "última hora": true, "ultima hora": true,
		"trending": true, "trending now": true, "destacados": true, "populares": true,
		"sections": true, "secciones": true, "sección": true,
		"share on facebook": true, "share on twitter": true, "compartir en facebook": true,
		"newsletter signup": true, "ver todos": true, "view all": true,
		// Menús sociales y formularios de login (anclados a la línea entera).
		"pinterest": true, "linkedin": true, "linked in": true, "tumblr": true,
		"reddit": true, "whatsapp": true, "telegram": true, "flipboard": true,
		"messenger": true, "facebook messenger": true, "share on whatsapp": true,
		"copy url": true, "copy link": true, "url is copied": true, "link copied": true,
		"email address": true, "password": true, "iniciar sesión": true,
		"iniciar sesion": true, "sign out": true, "log out": true, "cerrar sesión": true,
	}

	urlOnlyRe    = regexp.MustCompile(`^(https?://|www\.)\S+$`)
	emailOnlyRe  = regexp.MustCompile(`^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}$`)
	entityOnlyRe = regexp.MustCompile(`^(&[A-Za-z]+;|&#\d{1,7};|\s)+$`)

	// Bloques: tras ellos se inserta un salto de línea para no pegar palabras.
	blockTags = map[string]bool{
		"p": true, "div": true, "li": true, "ul": true, "ol": true, "dl": true,
		"dt": true, "dd": true, "tr": true, "td": true, "th": true, "table": true,
		"thead": true, "tbody": true, "tfoot": true, "caption": true,
		"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
		"section": true, "article": true, "main": true, "blockquote": true,
		"pre": true, "figure": true, "figcaption": true, "address": true,
		"details": true, "summary": true, "fieldset": true, "legend": true,
		"hr": true, "br": true,
	}

	// Elementos que nunca aportan texto útil (o son peligrosos).
	skipTags = map[string]bool{
		"script": true, "style": true, "noscript": true, "template": true,
		"iframe": true, "svg": true, "math": true, "object": true, "embed": true,
		"form": true, "input": true, "button": true, "select": true, "option": true,
		"textarea": true, "nav": true, "footer": true, "aside": true,
		"link": true, "meta": true, "video": true, "audio": true, "canvas": true,
		"picture": true, "source": true, "map": true, "area": true, "dialog": true,
		"applet": true, "frame": true, "frameset": true,
	}

	skipRoles = map[string]bool{
		"navigation": true, "banner": true, "complementary": true,
		"menu": true, "dialog": true, "alert": true, "menubar": true,
	}
)

// Clean devuelve el texto listo para persistir/traducir.
func Clean(s string) string {
	return CleanWith(s, Options{MaxInput: DefaultMaxInput, MaxLen: DefaultMaxLen, StripLines: true})
}

// CleanWith aplica Clean con opciones concretas.
func CleanWith(s string, o Options) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	if o.MaxInput <= 0 {
		o.MaxInput = DefaultMaxInput
	}
	if o.MaxLen <= 0 {
		o.MaxLen = DefaultMaxLen
	}

	s = cutAtByte(s, o.MaxInput)
	s = unescapeOnce(s)

	if looksLikeHTML(s) && balancedRawTextTags(s) {
		if parsed, ok := parseToText(s); ok {
			s = parsed
		}
	}

	lines := normalize(s, o.StripLines)
	full := strings.Join(lines, "\n")
	out := truncate(full, o.MaxLen)

	// El corte puede dejar la última línea a medio hacer y entonces ya no pasa
	// el filtro (p. ej. un comentario "//…" que se quedó sin letras): se vuelve
	// a normalizar ese trozo, si no Clean(x) != Clean(Clean(x)).
	if o.StripLines && len(out) < len(full) {
		head, last := "", out
		if i := strings.LastIndexByte(out, '\n'); i >= 0 {
			head, last = out[:i+1], out[i+1:]
		}
		if last != "" {
			out = head + strings.Join(normalize(last, o.StripLines), "\n")
		}
	}
	out = strings.TrimSpace(out)

	// Si el contenido era JSON puro (bloque JSON-LD del <head> que algunos CMS
	// meten en el campo de contenido) y no queda nada, se recupera la
	// descripción: evita dejar el resumen vacío y, si acaso, el scraper
	// seguirá enriqueciéndolo con el cuerpo real de la página.
	if out == "" {
		if alt := jsonldText(s); alt != "" {
			alt = truncate(strings.Join(normalize(alt, o.StripLines), "\n"), o.MaxLen)
			if alt = strings.TrimSpace(alt); alt != "" {
				return alt
			}
		}
	}
	return out
}

// jsonldText extrae el texto legible de un feed cuyo contenido es JSON/JSON-LD
// puro. Devuelve "" si no empieza por JSON o no tiene campos de texto.
func jsonldText(raw string) string {
	t := strings.TrimSpace(raw)
	if t == "" || (t[0] != '{' && t[0] != '[') || len(t) > DefaultMaxInput {
		return ""
	}
	// Solo se decodifica el PRIMER valor JSON: a menudo el JSON-LD viene
	// pegado a CSS/JS del resto de la página ("...} } @media ..."), y eso
	// hacía que json.Valid() fallara y se tirara la descripción entera.
	dec := json.NewDecoder(strings.NewReader(t))
	var v any
	if err := dec.Decode(&v); err != nil {
		return ""
	}
	return findJSONText(v, 0)
}

// jsonTextKeys: campos de JSON-LD que contienen prosa.
var jsonTextKeys = []string{"description", "articleBody", "abstract", "summary"}

func findJSONText(v any, depth int) string {
	if depth > 4 {
		return ""
	}
	switch t := v.(type) {
	case map[string]any:
		for _, k := range jsonTextKeys {
			if s, ok := t[k].(string); ok && len(s) >= 30 {
				return s
			}
		}
		// Orden determinista: los mapas de Go iteran en orden aleatorio.
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if s := findJSONText(t[k], depth+1); s != "" {
				return s
			}
		}
	case []any:
		for _, e := range t {
			if s := findJSONText(e, depth+1); s != "" {
				return s
			}
		}
	}
	return ""
}

// CleanTitle limpia un título: una sola línea, sin etiquetas y con tope corto.
func CleanTitle(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	s = cutAtByte(s, DefaultMaxInput)
	s = unescapeOnce(s)

	if looksLikeHTML(s) && balancedRawTextTags(s) {
		if parsed, ok := parseToText(s); ok {
			s = parsed
		}
	}

	s = strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(s)
	s = spaceRe.ReplaceAllString(s, " ")
	s = stripTags(s)
	s = spaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(truncate(strings.TrimSpace(s), DefaultMaxTitleLen))
}

// ---------------------------------------------------------------------------
// HTML
// ---------------------------------------------------------------------------

func looksLikeHTML(s string) bool { return tagRe.MatchString(s) }

// stripTags elimina las etiquetas de una línea. Se itera porque al quitar una
// etiqueta puede quedar un artefacto con forma de etiqueta (HTML malformado,
// p.ej. "<A<sCript>>" deja "<A >"). Cada pasada reduce la longitud, así que el
// bucle termina; se acota además por prudencia.
func stripTags(s string) string {
	if strings.IndexByte(s, '<') < 0 {
		return s
	}
	for i := 0; i < 8 && tagRe.MatchString(s); i++ {
		s = tagRe.ReplaceAllString(s, " ")
	}
	return s
}

// unescapeOnce descodifica entidades (&amp;, &#8230;, &lt;p&gt;) solo si la
// descodificación es de un único nivel. Si al descodificar de nuevo el
// resultado cambiara (entrada doblemente escapada, p.ej. "&amp;lt;"), no se
// aplica: así Clean es idempotente aunque se ejecute dos veces seguidas.
func unescapeOnce(s string) string {
	if !strings.Contains(s, "&") {
		return s
	}
	u := stdhtml.UnescapeString(s)
	if u == s {
		return s
	}
	if stdhtml.UnescapeString(u) != u {
		return s
	}
	return u
}

// balancedRawTextTags comprueba que no aparezca un elemento de texto crudo sin
// cierre (p.ej. la frase "si x <style entonces"): parsear en ese caso haría que
// el parser se tragara el resto del documento.
func balancedRawTextTags(s string) bool {
	lower := strings.ToLower(s)
	for _, t := range rawTextTags {
		open := strings.Index(lower, "<"+t)
		if open < 0 {
			continue
		}
		// ¿es realmente una etiqueta y no "x <stylebar"?
		after := open + 1 + len(t)
		if after < len(lower) {
			c := lower[after]
			if c != '>' && c != '/' && c != ' ' && c != '\t' && c != '\n' && c != '\r' {
				continue
			}
		}
		if !strings.Contains(lower, "</"+t) {
			return false
		}
	}
	return true
}

// parseToText convierte HTML en texto plano separando bloques con saltos de
// línea y descartando chrome (nav/header/footer/script/aria-hidden/...).
func parseToText(s string) (string, bool) {
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return "", false
	}
	var sb strings.Builder
	sb.Grow(len(s))
	writeNodes(doc, &sb, false)
	return sb.String(), true
}

func writeNodes(n *html.Node, sb *strings.Builder, inArticle bool) {
	switch n.Type {
	case html.CommentNode:
		return
	case html.TextNode:
		sb.WriteString(n.Data)
		return
	case html.ElementNode:
		if skipElement(n, inArticle) {
			return
		}
		childCtx := inArticle || n.Data == "article" || n.Data == "main"
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			writeNodes(c, sb, childCtx)
		}
		if blockTags[n.Data] {
			sb.WriteByte('\n')
		}
	default: // DocumentNode, DoctypeNode
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			writeNodes(c, sb, inArticle)
		}
	}
}

func skipElement(n *html.Node, inArticle bool) bool {
	if skipTags[n.Data] {
		return true
	}
	// <header> solo se descarta fuera de <article>/<main>: dentro lleva entradilla.
	if n.Data == "header" && !inArticle {
		return true
	}
	for _, a := range n.Attr {
		switch a.Key {
		case "aria-hidden":
			if a.Val == "true" {
				return true
			}
		case "hidden":
			return true
		case "role":
			if skipRoles[strings.ToLower(a.Val)] {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Normalización y filtro de ruido
// ---------------------------------------------------------------------------

func normalize(s string, stripLines bool) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.Map(func(r rune) rune {
		switch r {
		case '\u00a0', '\u3000': // NBSP y espacio ideográfico
			return ' '
		case '\u200b', '\u200c', '\u200d', '\ufeff':
			return -1 // cero de ancho
		}
		return r
	}, s)

	raw := strings.Split(s, "\n")
	out := make([]string, 0, len(raw))
	for _, l := range raw {
		l = stripTags(l) // etiquetas residuales: fuera
		// "}texto": cierre de bloque CSS/JS pegado al artículo (caso real de
		// Grecia: "}Σχετικό άρθρο …"). Solo se quitan las llaves iniciales y se
		// conserva la frase; si lo que viene detrás es código, lo quitan las
		// reglas de la línea. TrimLeft es idempotente por construcción.
		l = strings.TrimLeft(l, "} \t")
		l = spaceRe.ReplaceAllString(l, " ")
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if stripLines && isNoiseLine(l) {
			continue
		}
		// Línea gigante conservada por ser prosa: se le recorta el código que
		// venga pegado (p. ej. "googletag.push(function(){…});") sin tocar el
		// resto del texto.
		if len(l) > mixedLen {
			l = limpiaLlamadas(l)
			if l == "" {
				continue
			}
		}
		if n := len(out); n > 0 && out[n-1] == l {
			continue // duplicado consecutivo (menús pegados)
		}
		out = append(out, l)
	}
	if stripLines && len(out) > maxRepeat {
		out = dropRepeated(out)
		// Quitar repetidas puede dejar dos líneas iguales pegadas; sin este
		// segundo dedupe Clean(x) != Clean(Clean(x)).
		out = dedupeConsecutive(out)
	}
	return out
}

// dedupeConsecutive elimina líneas idénticas seguidas (sobre la lista ya
// filtrada, cuando la eliminación de repetidas ha podido juntarlas).
func dedupeConsecutive(lines []string) []string {
	out := lines[:0]
	for _, l := range lines {
		if n := len(out); n > 0 && out[n-1] == l {
			continue
		}
		out = append(out, l)
	}
	return out
}

// isNoiseLine decide si una línea ya recortada es ruido. Reglas conservadoras:
// si una duda razonable existe, la línea se conserva.
func isNoiseLine(l string) bool {
	// 1. Línea sin ninguna letra (fechas sueltas, separadores, "•", "1 de 3")
	if len(l) <= 40 && !hasLetter(l) {
		return true
	}
	// 2. Línea que es únicamente una entidad HTML (&nbsp; suelta)
	if len(l) <= 40 && entityOnlyRe.MatchString(l) {
		return true
	}
	// 3. Línea muy larga: hay feeds que pegan el artículo entero en un
	//    renglón. Descartarla entera borraría la noticia, así que solo se tira
	//    si la línea ES basura (CSS/JSON/JS pegado), no si la contiene.
	if len(l) > mixedLen {
		return esBasuraGiant(l)
	}
	// 4. Código embebido (JS/CSS/JSON), llaves sueltas y etiquetas a medias.
	if esCodigo(l) {
		return true
	}
	// 5. Chrome de página: la línea completa es un menú o una frase de plantilla
	if isUILine(l) {
		return true
	}
	// 6. URL o correo sueltos en su propia línea
	if len(l) <= 300 && urlOnlyRe.MatchString(l) {
		return true
	}
	if len(l) <= 120 && emailOnlyRe.MatchString(l) {
		return true
	}
	return false
}

// esCodigo dice si la línea entera es código/CSS/JSON/etiqueta. Se usa tanto
// para líneas cortas como para las largas que ya se ha visto que no son prosa.
func esCodigo(l string) bool {
	return codeRe.MatchString(l) || braceCodeRe.MatchString(l) || selectorRe.MatchString(l) ||
		jsonKeyRe.MatchString(l) || tmplRe.MatchString(l) || braceTailRe.MatchString(l) ||
		declRe.MatchString(l) || camelRe.MatchString(l) ||
		callRe.MatchString(l) || assignRe.MatchString(l) ||
		tagStartRe.MatchString(l) ||
		// 3b. Llave sin su pareja: resto de bloque CSS/JS partido en varios
		//     renglones (p. ej. ".foo {order: 3;margin-left: auto;").
		strings.Contains(l, "{") != strings.Contains(l, "}")
}

// esBasuraGiant decide si una línea de más de mixedLen caracteres es ruido de
// verdad. Distingue dos casos:
//   - basura pura (JSON, CSS, JS, etiqueta): se descarta entera;
//   - prosa con basura pegada (el CMS junta todo sin saltos de línea): se
//     conserva, porque dentro hay texto del artículo.
func esBasuraGiant(l string) bool {
	t := strings.TrimLeft(l, " \t")
	if t == "" {
		return true
	}
	switch t[0] {
	case '{', '[': // JSON/JSON-LD: jsonldText recupera la descripción si hace falta
		return true
	case '<': // etiqueta HTML sin cerrar
		return true
	case '@': // @media, @import, @keyframes…
		return atRuleRe.MatchString(t) && strings.Contains(t, "{")
	case '.', '#': // selector CSS: ".foo{" o "#id {"
		return cssHeadRe.MatchString(t) && strings.Contains(t, "{")
	case '(': // IIFE: "(function() { …"
		return codeRe.MatchString(t[:min(len(t), 300)])
	}
	// Llamada con punto al inicio ("googletag.display("): código pegado.
	if callRe.MatchString(t) {
		return true
	}
	// Llamada con punto en cualquier parte de la línea (patrones JS comunes
	// embebidos en chrome de página sin separador: "…Reklamwindow._taboola…" ).
	if callSpanRe.MatchString(t) {
		return true
	}
	// Texto corrido: más de un 5% de espacios. Se queda aunque lleve código
	// pegado (p. ej. "…googletag.cmd.push(function(){…}); el artículo sigue…").
	if ratioEspacios(l) >= ratioProsa {
		return false
	}
	// Sin espacios: o es código minificado o es un idioma sin espacios (CJK),
	// que no suele encajar en las reglas de código.
	return esCodigo(l)
}

// ratioEspacios devuelve la fracción de la línea que son espacios.
func ratioEspacios(s string) float64 {
	if s == "" {
		return 0
	}
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' {
			n++
		}
	}
	return float64(n) / float64(len(s))
}

// limpiaLlamadas recorta de la línea las llamadas con punto que vengan pegadas
// a la prosa. Se repite hasta que deja de cambiar, así que aplicarla dos veces
// da exactamente el mismo resultado (idempotencia).
func limpiaLlamadas(l string) string {
	for i := 0; i < 3; i++ {
		next := limpiaUnaLlamada(l)
		if next == l {
			// Si no se encontró llamada, probar con asignaciones JS.
			next = limpiaAsignacionesJS(l)
			if next == l {
				return l
			}
		}
		l = next
	}
	return l
}

// limpiaAsignacionesJS recorta asignaciones JavaScript comunes embebidas en prosa
// sin separador (ej: "…Reklamwindow._taboola = window._taboola || [];").
// Retorna la línea sin el código JavaScript, o la línea original si no encontró patrón.
func limpiaAsignacionesJS(l string) string {
	// Patrones de asignaciones JS que aparecen pegados al texto sin espacio separador.
	// Se buscan con boundaries flexibles porque a veces están pegados a textoturco
	// (ej: "Reklamwindow._taboola" donde "Reklam" es la palabra turca de advertisement).
	patterns := []string{
		`window\._taboola\s*=\s*window\._taboola\s*\|\|\s*\[\]\s*;`,
		`window\.__taboola\s*=\s*window\.__taboola\s*\|\|\s*\[\]\s*;`,
		`googletag\.cmd\.push\s*\(`, // No tiene paréntesis de cierre predecible, se recorta hasta ;
	}

	for _, pat := range patterns {
		re := regexp.MustCompile(pat)
		loc := re.FindStringIndex(l)
		if loc != nil {
			// Encontrado: eliminar desde el inicio del patrón hasta el siguiente ';'
			// o fin de línea, lo que ocurra primero.
			end := loc[1]
			// Buscar el ';' de cierre de la sentencia
			for end < len(l) && l[end] != ';' {
				end++
			}
			if end < len(l) {
				end++ // incluir el ';'
			}
			// Recomponer: texto antes + texto después (después del ';')
			before := strings.TrimRight(l[:loc[0]], " \t")
			after := strings.TrimLeft(l[end:], " \t")
			if before != "" && after != "" {
				return before + " " + after
			} else if before != "" {
				return before
			} else if after != "" {
				return after
			}
		}
	}
	return l
}

// limpiaUnaLlamada elimina la primera llamada con punto equilibrada cuyo
// interior parece código (lleva ';' '{' o '='). Si no la hay, devuelve la línea
// tal cual: un "v2.0 (beta)" de la prosa no se toca.
func limpiaUnaLlamada(l string) string {
	loc := callSpanRe.FindStringIndex(l)
	if loc == nil {
		return l
	}
	// loc[1]-1 es la posición del paréntesis que abre la llamada.
	depth, end := 0, -1
	limit := min(len(l), loc[1]-1+500)
	for i := loc[1] - 1; i < limit; i++ {
		switch l[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		}
		if depth == 0 {
			end = i + 1
			break
		}
	}
	if end < 0 {
		return l // no se equilibra en 500 car.: no se arriesga
	}
	if !strings.ContainsAny(l[loc[0]:end], ";{=") {
		return l
	}
	if end < len(l) && l[end] == ';' {
		end++ // el ';' de la sentencia también es código
	}
	return strings.TrimRight(l[:loc[0]], " ") + " " + strings.TrimLeft(l[end:], " ")
}

// isUILine decide si una línea es exclusivamente chrome de página (menú,
// botón, aviso de cookies...). Solo se mira la línea completa: una frase que
// contenga la palabra "share" por medio no se toca.
func isUILine(l string) bool {
	if len(l) > 80 {
		return false
	}
	key := strings.Trim(strings.ToLower(l), " \t·•|*-–—/<>«»\"'()[]{},;:¡!¿?.")
	if key == "" {
		return false
	}
	if uiPhrases[key] {
		return true
	}
	return uiRe.MatchString(l)
}

// dropRepeated elimina líneas cortas repetidas maxRepeat veces o más (menús,
// pies de foto repetidos, plantillas del CMS). Solo actúa sobre líneas cortas.
func dropRepeated(lines []string) []string {
	counts := make(map[string]int, len(lines))
	for _, l := range lines {
		if len(l) <= maxRepeatLen {
			counts[l]++
		}
	}
	out := lines[:0]
	for _, l := range lines {
		if len(l) <= maxRepeatLen && counts[l] >= maxRepeat {
			continue
		}
		out = append(out, l)
	}
	return out
}

// hasLetter indica si la línea contiene alguna letra (cualquier script).
func hasLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Límites
// ---------------------------------------------------------------------------

// truncate acorta s a max bytes cortando en frontera de frase cuando es posible.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	window := s[:max]

	// 1) Fin de frase dentro de los últimos 40% del límite (puntuación ASCII,
	//    que es la única que cabe en un byte).
	lo := max * 3 / 5
	for i := len(window) - 2; i > lo; i-- {
		switch window[i] {
		case '.', '!', '?':
			if window[i+1] == ' ' || window[i+1] == '\n' {
				return strings.TrimSpace(s[:i+1])
			}
		}
	}
	// 2) Último espacio en el último 10%.
	lo = max * 9 / 10
	for i := len(window) - 1; i > lo; i-- {
		if window[i] == ' ' {
			return strings.TrimSpace(s[:i])
		}
	}
	// 3) Corte duro respetando la frontera de rune.
	return strings.TrimSpace(cutAtByte(s, max))
}

// Truncate acorta s a max bytes cortando en frontera de frase cuando es
// posible y sin partir un rune por la mitad.
func Truncate(s string, max int) string { return truncate(s, max) }

// cutAtByte trunca en n bytes sin partir un rune por la mitad.
func cutAtByte(s string, n int) string {
	if n >= len(s) {
		return s
	}
	if n <= 0 {
		return ""
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
