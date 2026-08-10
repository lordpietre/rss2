package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	_ "github.com/lib/pq"
	"github.com/mmcdole/gofeed"
)

// Config holds the configuration loaded from environment variables
type Config struct {
	DBHost       string
	DBPort       string
	DBUser       string
	DBPass       string
	DBName       string
	MaxWorkers   int
	MaxFailures  int
	PokeInterval time.Duration
	FeedTimeout  int
}

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
}

var (
	db     *sql.DB
	config Config
)

func loadConfig() {
	config = Config{
		DBHost:       getEnv("DB_HOST", "localhost"),
		DBPort:       getEnv("DB_PORT", "5432"),
		DBUser:       getEnv("DB_USER", "rss"),
		DBPass:       getEnv("DB_PASS", "x"),
		DBName:       getEnv("DB_NAME", "rss"),
		MaxWorkers:   getEnvInt("RSS_MAX_WORKERS", 20), // Default to higher concurrency in Go
		MaxFailures:  getEnvInt("RSS_MAX_FAILURES", 10),
		PokeInterval: time.Duration(getEnvInt("RSS_POKE_INTERVAL_MIN", 8)) * time.Minute,
		FeedTimeout:  getEnvInt("RSS_FEED_TIMEOUT", 60),
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

	if err = db.Ping(); err != nil {
		log.Fatalf("Error connecting to DB: %v", err)
	}
	log.Println("Database connection established")
}

func getActiveFeeds() ([]Feed, error) {
	query := `
		SELECT id, nombre, url, categoria_id, pais_id, last_etag, last_modified, COALESCE(fallos, 0)
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
		if err := rows.Scan(&f.ID, &f.Nombre, &f.URL, &f.CategoriaID, &f.PaisID, &f.LastEtag, &f.LastModified, &f.Fallos); err != nil {
			log.Printf("Error scanning feed: %v", err)
			continue
		}
		feeds = append(feeds, f)
	}
	return feeds, nil
}

func generateID(link string) string {
	hash := md5.Sum([]byte(link))
	return hex.EncodeToString(hash[:])
}

func cleanHTML(input string) string {
	if input == "" {
		return ""
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(input))
	if err == nil {
		return strings.TrimSpace(doc.Text())
	}
	return strings.TrimSpace(input)
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
	// Configure custom HTTP client with timeout and User-Agent
	client := &http.Client{
		Timeout: time.Duration(config.FeedTimeout) * time.Second,
	}
	
	// Create request to set User-Agent
	req, err := http.NewRequest("GET", feed.URL, nil)
	if err != nil {
		log.Printf("[Feed %d] Error creating request: %v", feed.ID, err)
		updateFeedStatus(feed.ID, "", "", false, err.Error())
		results <- 0
		return
	}
	req.Header.Set("User-Agent", "RSS2-Ingestor-Go/1.0")

	// NOTE: We INTENTIONALLY SKIP ETag/Last-Modified headers based on user issues
	// If needed in future, uncomment:
	// if feed.LastEtag.Valid { req.Header.Set("If-None-Match", feed.LastEtag.String) }
	// if feed.LastModified.Valid { req.Header.Set("If-Modified-Since", feed.LastModified.String) }

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[Feed %d] Error fetching: %v", feed.ID, err)
		updateFeedStatus(feed.ID, "", "", false, err.Error())
		results <- 0
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 304 {
		log.Printf("[Feed %d] Not Modified (304)", feed.ID)
		// Update timestamp only? Or keep as is.
		updateFeedStatus(feed.ID, feed.LastEtag.String, feed.LastModified.String, true, "")
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
	for _, item := range parsedFeed.Items {
		if item.Link == "" {
			continue
		}
		
		pubDate := time.Now()
		if item.PublishedParsed != nil {
			pubDate = *item.PublishedParsed
		} else if item.UpdatedParsed != nil {
			pubDate = *item.UpdatedParsed
		}

		// HTML cleanup simply takes Description or Content
		resumen := item.Description
		if resumen == "" {
			resumen = item.Content
		}
		
		noticia := Noticia{
			ID:           generateID(item.Link),
			Titulo:       item.Title,
			Resumen:      cleanHTML(resumen),
			URL:          item.Link,
			Fecha:        pubDate,
			ImagenURL:    extractImage(item),
			FuenteNombre: feed.Nombre,
			CategoriaID:  feed.CategoriaID,
			PaisID:       feed.PaisID,
		}
		noticias = append(noticias, noticia)
	}

	inserted := insertNoticias(noticias)
	
	// Get new headers
	newEtag := resp.Header.Get("ETag")
	newModified := resp.Header.Get("Last-Modified")

	updateFeedStatus(feed.ID, newEtag, newModified, true, "")
	
	if inserted > 0 {
		log.Printf("[Feed %d] Inserted %d new items", feed.ID, inserted)
	}
	results <- inserted
}

func insertNoticias(noticias []Noticia) int {
	// Bulk insert with INSERT ... ON CONFLICT DO NOTHING.
	// NOTE: do NOT use pq.CopyIn here: preparing a CopyIn leaves the backend in
	// COPY mode waiting for data, which can hold an ACCESS EXCLUSIVE lock on the
	// table and block the whole database if the stream is never completed.
	return insertNoticiasWithConflict(noticias)
}

func insertNoticiasWithConflict(noticias []Noticia) int {
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
			offset := j * 9
			placeholders = append(placeholders, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)", 
				offset+1, offset+2, offset+3, offset+4, offset+5, offset+6, offset+7, offset+8, offset+9))
			
			vals = append(vals, n.ID, n.Titulo, n.Resumen, n.URL, n.Fecha, n.ImagenURL, n.FuenteNombre, n.CategoriaID, n.PaisID)
		}
		
		query := fmt.Sprintf(`
			INSERT INTO noticias (id, titulo, resumen, url, fecha, imagen_url, fuente_nombre, categoria_id, pais_id)
			VALUES %s
			ON CONFLICT (url) DO NOTHING
		`, strings.Join(placeholders, ","))

		res, err := db.Exec(query, vals...)
		if err != nil {
			log.Printf("Batch insert error: %v", err)
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
	initDB()
	
	// Run immediately on start
	ingestCycle()

	// Scheduler loop
	ticker := time.NewTicker(config.PokeInterval)
	defer ticker.Stop()

	for range ticker.C {
		ingestCycle()
	}
}
