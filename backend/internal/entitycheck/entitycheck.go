// Package entitycheck clasifica los valores de `tags` (entidades y temas)
// detectando falsos positivos del NER: basura de página (horas, marcadores,
// fechas, urls), fragmentos de titular cortados, palabras genéricas y
// texto pegado.
//
// Es la única fuente de verdad de las reglas y la usan tres consumidores:
//
//   - `backend/cmd/entityscan`: siembra `entity_blocklist` con los FP fuertes
//     de TODOS los países y tipos (varias decenas de miles de valores);
//   - `handlers.GetEntities`/`GetEntityNews`: filtran la blocklist, así que
//     Populares ya no los muestra a ningún usuario;
//   - `TestPopulares*` (este paquete): consulta la API real y falla si la
//     basura vuelve a asomar — es la red de seguridad de regresión.
//
// Dos niveles:
//
//	Strong — basura mecánica de alta precisión: se bloquea automáticamente.
//	Weak   — sospechoso (nombres pegados, frases de 7-8 palabras…):
//	         solo se informa; la revisión es humana.
package entitycheck

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Severity es el veredicto sobre un valor de tags.
type Severity int

const (
	// SeverityNone: el valor pasa los filtros.
	SeverityNone Severity = iota
	// SeverityWeak: sospechoso; se informa pero no se bloquea.
	SeverityWeak
	// SeverityStrong: fals positivo claro; se bloquea en la API.
	SeverityStrong
)

func (s Severity) String() string {
	switch s {
	case SeverityStrong:
		return "fuerte"
	case SeverityWeak:
		return "débil"
	default:
		return "ninguna"
	}
}

var (
	// Basura que solo puede venir de una página: P97.67, "00:39", "26.09.2026".
	// El límite izquierdo evita confundirlo con el millar español "1.100" y el
	// derecho (`[^0-9]|$`) permite el caso pegado "metros00:58El".
	reTime = regexp.MustCompile(`(^|[^0-9.])[0-9]{1,2}[.:][0-9]{2}([^0-9]|$)`)
	// Marcador: "0-1", "27-24" (no "2026-27": eso es un año y es legítimo).
	reScore = regexp.MustCompile(`\b[0-9]{1,2}\s*[-–]\s*[0-9]{1,2}\b`)
	// Fecha pegada al valor: "26-09-2026", "Guiu26-09-2026", "28/09/26".
	reDate = regexp.MustCompile(`(^|[^0-9])[0-9]{1,2}[-/][0-9]{1,2}[-/][0-9]{2,4}([^0-9]|$)`)
	// Url/email. Los TLD están en lista: "EE.UU." y "TeleX.hu" no son urls.
	reURL = regexp.MustCompile(`(?i)(https?://|www\.|@|\.(com|net|org|info|xyz|news|online|site|store|blog)\b)`)
	// Símbolos que no aparecen en nombres propios. '&' se excluye a propósito:
	// "Rohde & Schwarz" o "Iberseries & Platino" son legítimos.
	reSym = regexp.MustCompile(`[(){}\[\]<>|\\=/~@#$%?]`)
	// Siglas cortas con dígito ("CR7") no deben caer en otros filtros.
	reNick = regexp.MustCompile(`^[A-Za-z]{1,4}[0-9]{1,2}$`)
	// CTA y plantilla de la fuente: el chrome de la página (botones, avisos
	// legales, newsletters), no el texto de la noticia. Se compara sobre
	// `fold` (minúsculas y sin acentos), por eso las claves van sin tildes.
	reCTA = regexp.MustCompile(`\b(haga clic|haz clic|haz click|click aqui|leer mas|ver mas|mas informacion|` +
		`suscribete|publicidad|anunciar|politica de privacidad|terminos de uso|terminos y condiciones|` +
		`aviso legal|newsletters?|advertisement|read more|click here|sign up)\b`)
)

// TAIL: una entidad que termina en palabra funcional es un recorte de frase
// ("Consejo de Administración de la", "Banco central de"). Se exigen ≥2 letras
// para no tumbar "Serie A".
var TAIL = map[string]bool{
	"de": true, "del": true, "la": true, "el": true, "en": true,
	"con": true, "para": true, "por": true, "los": true, "las": true,
	"al": true, "the": true, "der": true, "die": true, "das": true,
	"a": true, "y": true, "i": true,
}

// LEAD: artículo inicial (regla débil: también lo llevan nombres legítimos
// como "La Asamblea General de los Estados Unidos").
var LEAD = map[string]bool{
	"el": true, "la": true, "los": true, "las": true, "un": true,
	"una": true, "the": true, "der": true, "die": true, "das": true,
}

var STOP = map[string]bool{
	"de": true, "la": true, "el": true, "los": true, "las": true,
	"del": true, "y": true, "en": true, "un": true, "una": true,
	"que": true, "por": true, "con": true, "para": true, "al": true,
	"a": true, "i": true, "the": true,
}

// GENERIC: palabras sueltas que no son entidad. Solo tienen sentido en
// persona/organización/lugar: para `tema` ("fútbol", "equipo") son legítimos.
var GENERIC = map[string]map[string]bool{
	"persona": {
		"fuente": true, "fuentes": true, "leer": true, "nota": true,
		"notas": true, "prensa": true, "redes": true, "video": true,
		"imagen": true, "hilo": true, "directo": true, "click": true,
		"aqui": true,
	},
	"organizacion": {
		"real": true, "liga": true, "mas": true, "más": true,
		"blancos": true, "merengues": true, "azulgrana": true,
		"seleccion": true, "equipo": true, "club": true, "fuente": true,
		"leer": true, "nota": true, "prensa": true, "actualidad": true,
		"deportes": true, "deporte": true, "futbol": true,
	},
	"lugar": {
		"leer": true, "fuente": true, "nota": true, "prensa": true,
		"hilo": true, "directo": true, "imagen": true, "video": true,
	},
	"tema":    {},
	"sistema": {},
}

const (
	maxStrongLen   = 100 // "Universidad Moderna de Tecnología…" vive hasta aquí
	minWeakLen     = 70
	minWeakWords   = 7
	maxWeakWords   = 8
	minStrongWords = 10
	minGlueWords   = 8
)

// fold normaliza para comparar: minúsculas y sin acentos (mismos resultados
// que NFKD+quitar combinantes que usó el análisis previo).
func fold(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(s) {
		if v, ok := foldMap[r]; ok {
			b.WriteRune(v)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

var foldMap = map[rune]rune{
	'á': 'a', 'à': 'a', 'ä': 'a', 'â': 'a', 'ã': 'a', 'å': 'a',
	'é': 'e', 'è': 'e', 'ë': 'e', 'ê': 'e',
	'í': 'i', 'ì': 'i', 'ï': 'i', 'î': 'i',
	'ó': 'o', 'ò': 'o', 'ö': 'o', 'ô': 'o', 'õ': 'o', 'ø': 'o',
	'ú': 'u', 'ù': 'u', 'ü': 'u', 'û': 'u',
	'ñ': 'n', 'ç': 'c', 'ý': 'y', 'ÿ': 'y',
	'š': 's', 'ž': 'z', 'đ': 'd', 'č': 'c', 'ć': 'c',
	'ř': 'r', 'ě': 'e', 'ť': 't', 'ď': 'd', 'ň': 'n', 'ů': 'u',
	'ą': 'a', 'ę': 'e', 'ł': 'l', 'ź': 'z', 'ż': 'z',
	'ğ': 'g', 'ş': 's', 'ı': 'i',
}

// Classify devuelve la severidad y el motivo ("" si el valor pasa).
func Classify(tipo, valor string) (Severity, string) {
	if strings.TrimSpace(valor) == "" {
		return SeverityStrong, "vacío"
	}
	words := strings.Fields(valor)
	foldedWords := make([]string, len(words))
	for i, w := range words {
		foldedWords[i] = fold(w)
	}
	folded := fold(valor)

	// ---- Fuerte -----------------------------------------------------------
	if valor != strings.TrimSpace(valor) || strings.Contains(valor, "  ") {
		return SeverityStrong, "espacios"
	}
	if !hasLetterOrDigit(valor) {
		return SeverityStrong, "solo puntuación"
	}
	if reTime.MatchString(valor) {
		return SeverityStrong, "hora/fecha pegada"
	}
	if reScore.MatchString(valor) {
		return SeverityStrong, "marcador"
	}
	if reDate.MatchString(valor) {
		return SeverityStrong, "fecha pegada"
	}
	if reURL.MatchString(valor) {
		return SeverityStrong, "url/email"
	}
	if reSym.MatchString(valor) && !reNick.MatchString(strings.TrimSpace(valor)) {
		return SeverityStrong, "símbolos"
	}
	if reCTA.MatchString(folded) {
		return SeverityStrong, "cta/plantilla web"
	}
	if g, ok := GENERIC[tipo]; ok && len(words) == 1 && g[folded] {
		return SeverityStrong, "palabra genérica"
	}
	if utf8.RuneCountInString(valor) > maxStrongLen {
		return SeverityStrong, "muy largo"
	}
	// Titular disfrazado de entidad: ≥10 palabras. Para `organizacion` es
	// débil porque hay nombres legítimos de esa longitud ("Comisión de
	// Consolidación de Paz de las Naciones Unidas"); en persona/lugar/tema
	// nunca lo son.
	if len(words) >= minStrongWords {
		if tipo == "organizacion" {
			return SeverityWeak, "frase muy larga (org)"
		}
		return SeverityStrong, "frase muy larga"
	}
	// ≥2 palabras: evita tumbar homógrafos de una sola palabra que SÍ son
	// entidades ("Pará", estado brasileño, dobla a la preposición "para").
	if len(foldedWords) >= 2 {
		if last := foldedWords[len(foldedWords)-1]; len(last) >= 2 && TAIL[last] {
			return SeverityStrong, "termina en función (cortado)"
		}
	}
	if LEAD[foldedWords[0]] {
		if n := countIn(foldedWords, STOP); n >= 3 {
			return SeverityWeak, "empieza por artículo"
		}
	}

	// ---- Débil ------------------------------------------------------------
	for _, w := range words {
		if utf8.RuneCountInString(w) >= minGlueWords && upperRunes(w) >= 2 && !isAllUpper(w) {
			return SeverityWeak, "palabras pegadas"
		}
	}
	if n := utf8.RuneCountInString(valor); n > minWeakLen && n <= maxStrongLen {
		return SeverityWeak, "largo (70-100)"
	}
	if len(words) >= minWeakWords && len(words) <= maxWeakWords {
		return SeverityWeak, "frase (7-8 palabras)"
	}
	return SeverityNone, ""
}

// IsFalsePositive es el atajo: true solo para los FP que se bloquean.
func IsFalsePositive(tipo, valor string) bool {
	sev, _ := Classify(tipo, valor)
	return sev == SeverityStrong
}

func hasLetterOrDigit(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func upperRunes(s string) int {
	n := 0
	for _, r := range s {
		if unicode.IsUpper(r) {
			n++
		}
	}
	return n
}

func isAllUpper(s string) bool {
	hasLetter := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			hasLetter = true
			if !unicode.IsUpper(r) {
				return false
			}
		}
	}
	return hasLetter
}

func countIn(words []string, set map[string]bool) int {
	n := 0
	for _, w := range words {
		if set[w] {
			n++
		}
	}
	return n
}
