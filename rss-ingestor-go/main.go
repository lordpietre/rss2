package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/lib/pq"
	"github.com/mmcdole/gofeed"
	"rss-ingestor-go/textclean"
)

// Config holds the configuration loaded from environment variables
type Config struct {
	DBHost      string
	DBPort      string
	DBUser      string
	DBPass      string
	DBName      string
	MaxWorkers  int
	MaxFailures int
	// TargetLang: idioma destino de la traduccion. Las noticias de los
	// canales marcados como "traducidos" se insertan ya en este idioma.
	TargetLang   string
	PokeInterval time.Duration
	FeedTimeout  int
	HostDelay    time.Duration
}

var (
	isRunning   bool
	runMu       sync.Mutex
)

// Feed represents a row in the feeds table
type Feed struct {
	ID           int
	Nombre       string
	URL          string
	CategoriaID  sql.NullInt64
	PaisID       sql.NullInt64
	LastEtag     sql.NullString
	LastModified sql.NullString
	Fallos       int
	// Traducido: el canal ya publica en el idioma destino. Sus noticias se
	// insertan con lang = destino y el pipeline las salta (no pasan por
	// langdetect ni generan job de traduccion).
	Traducido bool
}

// Noticia represents a news item to be inserted
type Noticia struct {
	ID           string
	Titulo       string
	Resumen      string
	URL          string
	Fecha        time.Time
	ImagenURL    string
	FuenteNombre string
	CategoriaID  sql.NullInt64
	PaisID       sql.NullInt64
	// Lang: idioma de la noticia. Se rellena con el idioma destino cuando el
	// canal esta marcado como traducido; si no, queda vacio y lo detecta
	// langdetect.
	Lang string
	// FeedID: id del feed de origen. Permite borrar un canal con precision
	// (la FK de noticias.feed_id va en cascada).
	FeedID int
}

var (
	db         *sql.DB
	config     Config
	httpClient *http.Client
)

// Per-host politeness limiter so parallel workers don't hammer one domain.
var (
	rateMu      sync.Mutex
	lastRequest = make(map[string]time.Time)
)

func politeDelay(host string) {
	if config.HostDelay <= 0 || host == "" {
		return
	}
	rateMu.Lock()
	now := time.Now()
	next := lastRequest[host].Add(config.HostDelay)
	lastRequest[host] = next
	rateMu.Unlock()
	if wait := next.Sub(now); wait > 0 {
		time.Sleep(wait)
	}
}

func loadConfig() {
	config = Config{
		DBHost:      getEnv("DB_HOST", "localhost"),
		DBPort:      getEnv("DB_PORT", "5432"),
		DBUser:      getEnv("DB_USER", "rss"),
		DBPass:      getEnv("DB_PASS", "x"),
		DBName:      getEnv("DB_NAME", "rss"),
		MaxWorkers:  getEnvInt("RSS_MAX_WORKERS", 20), // Default to higher concurrency in Go
		MaxFailures: getEnvInt("RSS_MAX_FAILURES", 10),
		// Idioma destino de la traduccion (el primero de TARGET_LANGS).
		TargetLang:   firstLang(getEnv("TARGET_LANGS", "es")),
		PokeInterval: time.Duration(getEnvInt("RSS_POKE_INTERVAL_MIN", 15)) * time.Minute,
		FeedTimeout:  getEnvInt("RSS_FEED_TIMEOUT", 60),
		HostDelay:    time.Duration(getEnvInt("RSS_HOST_DELAY_MS", 500)) * time.Millisecond,
	}
}

// firstLang devuelve el primer idioma de una lista "es,en,fr".
func firstLang(langs string) string {
	for _, l := range strings.Split(langs, ",") {
		l = strings.TrimSpace(l)
		if l != "" {
			return l
		}
	}
	return "es"
}

func initHTTPClient() {
	httpClient = &http.Client{
		Timeout: time.Duration(config.FeedTimeout) * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        config.MaxWorkers * 2,
			MaxIdleConnsPerHost: 2,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	strValue := getEnv(key, "")
	if strValue == "" {
		return fallback
	}
	val, err := strconv.Atoi(strValue)
	if err != nil {
		return fallback
	}
	return val
}

func initDB() {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.DBHost, config.DBPort, config.DBUser, config.DBPass, config.DBName)
	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Error opening DB: %v", err)
	}

	db.SetMaxOpenConns(config.MaxWorkers + 5)
	db.SetMaxIdleConns(config.MaxWorkers)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Al arrancar el host todos los contenedores levantan a la vez: el
	// Postgres puede tardar unos segundos en aceptar conexiones. Sin esta
	// espera el proceso muere y Docker lo reinicia con backoff (varios
	// reinicios por arranque).
	for attempt := 1; attempt <= 30; attempt++ {
		if err = db.Ping(); err == nil {
			break
		}
		if attempt == 30 {
			log.Fatalf("Error connecting to DB: %v", err)
		}
		log.Printf("Waiting for database (%d/30): %v", attempt, err)
		time.Sleep(2 * time.Second)
	}
	log.Println("Database connection established")
}

func getActiveFeeds() ([]Feed, error) {
	query := `
		SELECT id, nombre, url, categoria_id, pais_id, last_etag, last_modified, COALESCE(fallos, 0), traducido
		FROM feeds
		WHERE activo = TRUE AND (fallos IS NULL OR fallos < $1)
		ORDER BY id
	`
	rows, err := db.Query(query, config.MaxFailures)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var feeds []Feed
	for rows.Next() {
		var f Feed
		if err := rows.Scan(&f.ID, &f.Nombre, &f.URL, &f.CategoriaID, &f.PaisID, &f.LastEtag, &f.LastModified, &f.Fallos, &f.Traducido); err != nil {
			log.Printf("Error scanning feed: %v", err)
			continue
		}
		feeds = append(feeds, f)
	}
	return feeds, nil
}

// trackingParams son parametros de campana que no identifican el articulo:
// el mismo articulo aparece con distintos valores y se guardaba N veces
// (mismo titulo, mismo feed, distintas URLs -> distintos ids).
var trackingParams = map[string]bool{
	"fbclid": true, "gclid": true, "dclid": true, "msclkid": true,
	"yclid": true, "twclid": true, "igshid": true, "mc_cid": true,
	"mc_eid": true, "_ga": true, "ref": true, "referrer": true,
	"ref_src": true, "ref_url": true, "source": true, "campaign_id": true,
	"spm": true, "s_kwcid": true, "cmpid": true, "ito": true,
}

// normalizeURL deja una URL en una forma canonica para que el mismo
// articulo tenga siempre el mismo id: sin esquema, sin 'www.', sin barra
// final y sin parametros de tracking.
//
// Sin esto, un feed que sirve el mismo articulo como
// 'https://www.x.com/a', 'https://x.com/a' y 'http://www.x.com/a' generaba
// tres noticias distintas (y tres trabajos de traduccion).
func normalizeURL(link string) string {
	raw := strings.TrimSpace(link)
	if raw == "" {
		return ""
	}

	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		// URL no parseable: se usa tal cual.
		return raw
	}

	host := strings.ToLower(u.Host)
	host = strings.TrimPrefix(host, "www.")
	// Puerto por defecto implicito.
	if strings.HasSuffix(host, ":80") {
		host = strings.TrimSuffix(host, ":80")
	} else if strings.HasSuffix(host, ":443") {
		host = strings.TrimSuffix(host, ":443")
	}

	path := strings.TrimRight(u.EscapedPath(), "/")

	q := u.Query()
	for k := range q {
		if trackingParams[strings.ToLower(k)] || strings.HasPrefix(strings.ToLower(k), "utm_") {
			q.Del(k)
		}
	}
	query := q.Encode() // Encode() ordena los parametros.
	if query == "" {
		return host + path
	}
	return host + path + "?" + query
}

func generateID(link string) string {
	hash := md5.Sum([]byte(normalizeURL(link)))
	return hex.EncodeToString(hash[:])
}

// Tolerancia hacia adelante: un feed puede publicar con la fecha un poco
// por delante (reloj del servidor desincronizado, o articulos programados).
// Pasada esta ventana se considera que la fecha del feed es erronea.
//
// Dos horas es un margen holgado para el caso legitimo (las noticias
// hemoglobinadas arriba llegaban 14-58 minutos por delante) y descarta a
// los feeds que publican la hora local etiquetada como UTC: CNN Turk
// (+2h, dice 'GMT') o el filipino Inquirer (+8h, dice '+0000'). Con un
// margen de 48h esos se colaban en la cabecera de la portada durante
// horas.
const maxFutureSkew = 2 * time.Hour

// Anio minimo creible. Por debajo (epoch de Unix, year 1 del zero time de
// Go, o fechas inventadas por el feed) la fecha no significa nada.
const minSaneYear = 1971

// sanitizeDate descarta las fechas que el feed declara pero que no son
// creibles, y las sustituye por el momento de la ingesta.
//
// Sin esto, un feed que sirve una fecha de 3000-01-01 en su pagina indice
// colocaba esa noticia al principio de /api/news (que ordena por fecha
// DESC) y, peor, el scraper se quedaba enriching esa misma noticia para
// siempre: seleccionaba `contenido IS NULL ORDER BY fecha DESC LIMIT 20` y
// esas 20 noticias jamas se enriquecen, con lo que el enriquecimiento se
// quedaba congelado.
//
// Los feeds con contenido de archivo (fechas de 2003, 2010...) se respetan:
// son noticias reales, solo que antiguas.
//
// OJO con la zona horaria: el resultado se normaliza a UTC (ver el .UTC()
// del llamante). La columna 'fecha' es 'timestamp without time zone', asi
// que Postgres ignora el offset que le llegue y se queda con la hora de
// pared de la zona original. Sin normalizar, una noticia de un feed
// japones (dc:date con +09:00) se guardaba 9 horas por delante y salia
// por delante de todo en la portada.
func sanitizeDate(d time.Time, now time.Time) time.Time {
	if d.Year() < minSaneYear {
		return now
	}
	if d.After(now.Add(maxFutureSkew)) {
		return now
	}
	return d
}

// resolveURL convierte un link del feed en una URL absoluta, resolviendolo
// contra la base (el sitio del feed o la propia URL del feed).
//
// Hay feeds que sirven links relativos ("/tour", "noticias/x",
// "//sitio.com/a"). Guardarlos tal cual rompia dos cosas: el scraper no
// podia descargarlos ("unsupported protocol scheme") y el id de la noticia
// (md5 de la URL normalizada) colisionaba entre feeds distintos, porque
// "/tour" es el mismo texto en todos ellos.
func resolveURL(base, link string) string {
	raw := strings.TrimSpace(link)
	if raw == "" {
		return ""
	}

	b, err := url.Parse(base)
	if err != nil || b.Host == "" {
		return raw
	}
	l, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return b.ResolveReference(l).String()
}

// cleanHTML deja el texto del feed listo para guardar: quita HTML, separa los
// bloques (evita palabras pegadas), descodifica entidades y descarta el chrome
// de la página (menús, pies, código JS). Es idempotente y está acotado.
func cleanHTML(input string) string {
	return textclean.Clean(input)
}

func extractImage(item *gofeed.Item) string {
	if item.Image != nil && item.Image.URL != "" {
		return item.Image.URL
	}
	if len(item.Enclosures) > 0 {
		for _, enc := range item.Enclosures {
			if strings.HasPrefix(enc.Type, "image/") {
				return enc.URL
			}
		}
	}
	// Try extensions
	if ex, ok := item.Extensions["media"]; ok {
		if content, ok := ex["content"]; ok {
			for _, c := range content {
				if url, ok := c.Attrs["url"]; ok {
					return url
				}
			}
		}
		if thumb, ok := ex["thumbnail"]; ok {
			for _, c := range thumb {
				if url, ok := c.Attrs["url"]; ok {
					return url
				}
			}
		}
	}

	// Try extracting from HTML description or content
	for _, html := range []string{item.Description, item.Content} {
		if html == "" {
			continue
		}
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
		if err == nil {
			if src, exists := doc.Find("img").First().Attr("src"); exists && src != "" {
				return src
			}
		}
	}

	return ""
}

func processFeed(fp *gofeed.Parser, feed Feed, results chan<- int) {
	// Shared HTTP client with connection pooling across workers
	client := httpClient

	// Create request to set User-Agent
	req, err := http.NewRequest("GET", feed.URL, nil)
	if err != nil {
		log.Printf("[Feed %d] Error creating request: %v", feed.ID, err)
		updateFeedStatus(feed.ID, "", "", false, err.Error())
		results <- 0
		return
	}
	req.Header.Set("User-Agent", "RSS2-Ingestor-Go/1.0")

	// Reuse ETag/Last-Modified so servers can reply 304 Not Modified,
	// avoiding re-downloading unchanged feeds in every cycle.
	if feed.LastEtag.Valid && feed.LastEtag.String != "" {
		req.Header.Set("If-None-Match", feed.LastEtag.String)
	}
	if feed.LastModified.Valid && feed.LastModified.String != "" {
		req.Header.Set("If-Modified-Since", feed.LastModified.String)
	}

	// Per-host politeness: pace parallel fetches to the same domain
	host := feed.URL
	if u, perr := url.Parse(feed.URL); perr == nil && u.Host != "" {
		host = u.Host
	}
	politeDelay(host)

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[Feed %d] Error fetching: %v", feed.ID, err)
		updateFeedStatus(feed.ID, "", "", false, err.Error())
		results <- 0
		return
	}
	defer resp.Body.Close()

	// Honor Retry-After on rate limits without punishing the feed with failures
	if resp.StatusCode == 429 || resp.StatusCode == 503 {
		ra, raErr := strconv.Atoi(resp.Header.Get("Retry-After"))
		if raErr == nil && ra > 0 && ra <= 3600 {
			log.Printf("[Feed %d] Rate limited (%d), sleeping %ds (Retry-After)", feed.ID, resp.StatusCode, ra)
			time.Sleep(time.Duration(ra) * time.Second)
		}
		results <- 0
		return
	}

	// On 304, keep the stored validators; update only if the server sends new ones.
	if resp.StatusCode == 304 {
		log.Printf("[Feed %d] Not Modified (304)", feed.ID)
		newEtag := feed.LastEtag.String
		newModified := feed.LastModified.String
		if h := resp.Header.Get("ETag"); h != "" {
			newEtag = h
		}
		if h := resp.Header.Get("Last-Modified"); h != "" {
			newModified = h
		}
		updateFeedStatus(feed.ID, newEtag, newModified, true, "")
		results <- 0
		return
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errMsg := fmt.Sprintf("HTTP %d", resp.StatusCode)
		log.Printf("[Feed %d] Error: %s", feed.ID, errMsg)
		updateFeedStatus(feed.ID, "", "", false, errMsg)
		results <- 0
		return
	}

	parsedFeed, err := fp.Parse(resp.Body)
	if err != nil {
		log.Printf("[Feed %d] Parser Error: %v", feed.ID, err)
		updateFeedStatus(feed.ID, "", "", false, err.Error())
		results <- 0
		return
	}

	// Prepare news items
	var noticias []Noticia

	// Base para resolver los links relativos del feed: el <link> del propio
	// feed (la web del medio) y, si no lo hay, la URL del feed.
	base := feed.URL
	if parsedFeed.Link != "" {
		if u, err := url.Parse(parsedFeed.Link); err == nil && u.Host != "" {
			base = parsedFeed.Link
		}
	}

	now := time.Now()

	for _, item := range parsedFeed.Items {
		if item.Link == "" {
			continue
		}

		// Link absoluto: los relativos se resuelven contra la base del feed.
		link := resolveURL(base, item.Link)
		if link == "" {
			continue
		}

		pubDate := now
		if item.PublishedParsed != nil {
			pubDate = *item.PublishedParsed
		} else if item.UpdatedParsed != nil {
			pubDate = *item.UpdatedParsed
		}
		pubDate = sanitizeDate(pubDate, now).UTC()

		// HTML cleanup simply takes Description or Content
		resumen := item.Description
		if resumen == "" {
			resumen = item.Content
		}

		noticia := Noticia{
			ID:           generateID(link),
			Titulo:       textclean.CleanTitle(item.Title),
			Resumen:      cleanHTML(resumen),
			URL:          link,
			Fecha:        pubDate,
			ImagenURL:    extractImage(item),
			FuenteNombre: feed.Nombre,
			CategoriaID:  feed.CategoriaID,
			PaisID:       feed.PaisID,
			FeedID:       feed.ID,
		}
		// Canal marcado como ya traducido: la noticia nace en el idioma
		// destino y el pipeline la salta (no la detecta ni la traduce).
		if feed.Traducido {
			noticia.Lang = config.TargetLang
		}
		noticias = append(noticias, noticia)
	}

	inserted := insertNoticias(feed.ID, noticias)

	// Get new headers
	newEtag := resp.Header.Get("ETag")
	newModified := resp.Header.Get("Last-Modified")

	updateFeedStatus(feed.ID, newEtag, newModified, true, "")

	if inserted > 0 {
		log.Printf("[Feed %d] Inserted %d new items", feed.ID, inserted)
	}
	results <- inserted
}

func insertNoticias(feedID int, noticias []Noticia) int {
	// Bulk insert with INSERT ... ON CONFLICT DO NOTHING.
	// NOTE: do NOT use pq.CopyIn here: preparing a CopyIn leaves the backend in
	// COPY mode waiting for data, which can hold an ACCESS EXCLUSIVE lock on the
	// table and block the whole database if the stream is never completed.
	return insertNoticiasWithConflict(feedID, noticias)
}

// isForeignKeyViolation detecta un error de clave foranea de Postgres (23503).
func isForeignKeyViolation(err error) bool {
	var pgErr *pq.Error
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23503"
	}
	return false
}

func insertNoticiasWithConflict(feedID int, noticias []Noticia) int {
	// Efficient bulk insert for Postgres using unnest
	// Or standard multi-value insert.

	count := 0
	// Chunking to avoid parameter limit (65535)
	chunkSize := 500
	for i := 0; i < len(noticias); i += chunkSize {
		end := i + chunkSize
		if end > len(noticias) {
			end = len(noticias)
		}
		chunk := noticias[i:end]

		placeholders := []string{}
		vals := []interface{}{}

		for j, n := range chunk {
			offset := j * 11
			placeholders = append(placeholders, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
				offset+1, offset+2, offset+3, offset+4, offset+5, offset+6, offset+7, offset+8, offset+9, offset+10, offset+11))

			// Lang vacio = idioma todavia desconocido (lo detectara
			// langdetect). Se escribe NULL, no '', para que el estado sea
			// inequivoco: noticias.lang es TEXT y 'sin idioma' es NULL.
			var lang interface{}
			if n.Lang != "" {
				lang = n.Lang
			}

			vals = append(vals, n.ID, n.Titulo, n.Resumen, n.URL, n.Fecha, n.ImagenURL, n.FuenteNombre, n.CategoriaID, n.PaisID, lang, n.FeedID)
		}

		query := fmt.Sprintf(`
			INSERT INTO noticias (id, titulo, resumen, url, fecha, imagen_url, fuente_nombre, categoria_id, pais_id, lang, feed_id)
			VALUES %s
			ON CONFLICT DO NOTHING
		`, strings.Join(placeholders, ","))

		res, err := db.Exec(query, vals...)
		if err != nil {
			// Lo mas probable es que el feed se borrara (la FK
			// noticias.feed_id va en cascada) mientras se descargaba: sus
			// noticias ya no existen y reinsertarlas las dejaria huerfanas.
			// Se comprueba y se avisa en vez de perder el lote en silencio.
			if isForeignKeyViolation(err) {
				log.Printf("Feed %d ya no existe (borrado durante la descarga): se descartan sus %d items", feedID, len(chunk))
			} else {
				log.Printf("Batch insert error (%d items): %v", len(chunk), err)
			}
			continue
		}

		rowsAff, _ := res.RowsAffected()
		count += int(rowsAff)
	}
	return count
}

func updateFeedStatus(id int, etag, modified string, success bool, lastError string) {
	var query string
	var args []interface{}

	if success {
		query = `UPDATE feeds SET fallos = 0, last_etag = $1, last_modified = $2, last_error = NULL WHERE id = $3`
		args = []interface{}{etag, modified, id}
	} else {
		// Increment failure count
		query = `
			UPDATE feeds 
			SET fallos = COALESCE(fallos, 0) + 1, 
			    last_error = $1,
				activo = CASE WHEN COALESCE(fallos, 0) + 1 >= $2 THEN FALSE ELSE activo END
			WHERE id = $3`
		args = []interface{}{lastError, config.MaxFailures, id}
	}

	_, err := db.Exec(query, args...)
	if err != nil {
		log.Printf("Error updating feed %d status: %v", id, err)
	}
}

func ingestCycle() {
	log.Println("Starting Ingestion Cycle...")
	start := time.Now()

	feeds, err := getActiveFeeds()
	if err != nil {
		log.Printf("Error getting feeds: %v", err)
		return
	}

	if len(feeds) == 0 {
		log.Println("No active feeds found.")
		return
	}

	log.Printf("Processing %d feeds with %d workers...", len(feeds), config.MaxWorkers)

	jobs := make(chan Feed, len(feeds))
	results := make(chan int, len(feeds))

	// Start workers
	var wg sync.WaitGroup
	for w := 0; w < config.MaxWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fp := gofeed.NewParser()
			for feed := range jobs {
				processFeed(fp, feed, results)
			}
		}()
	}

	// Send jobs
	for _, f := range feeds {
		jobs <- f
	}
	close(jobs)

	// Wait for workers in background to close results when done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Count results
	totalNew := 0
	for inserted := range results {
		totalNew += inserted
	}

	duration := time.Since(start)
	log.Printf("Ingestion Cycle Complete. Processed %d feeds in %v. New items: %d", len(feeds), duration, totalNew)
}

func main() {
	loadConfig()
	initHTTPClient()
	initDB()

	// Run immediately on start
	runMu.Lock()
	isRunning = true
	runMu.Unlock()
	ingestCycle()
	runMu.Lock()
	isRunning = false
	runMu.Unlock()

	// Scheduler loop
	ticker := time.NewTicker(config.PokeInterval)
	defer ticker.Stop()

	for range ticker.C {
		runMu.Lock()
		if isRunning {
			log.Printf("[Scheduler] Previous cycle still running, skipping this tick")
			runMu.Unlock()
			continue
		}
		isRunning = true
		runMu.Unlock()

		go func() {
			ingestCycle()
			runMu.Lock()
			isRunning = false
			runMu.Unlock()
		}()
	}
}
