package main

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rss2/backend/internal/logger"
	"github.com/rss2/backend/internal/workers"
)

var (
	pool          *pgxpool.Pool
	sleepInterval = 60
	batchSize     = 10
	enrichLimit   = 20
	scraperWorkers = 5
	maxBodyBytes  = int64(512 << 10)
)

type URLSource struct {
	ID          int64
	Nombre      string
	URL         string
	CategoriaID *int64
	PaisID      *int64
	Idioma      *string
	Active      bool
}

type Noticia struct {
	ID           string
	Titulo       string
	Resumen      string
	URL          string
	FuenteNombre string
}

type Article struct {
	Title    string
	Summary  string
	Content  string
	URL      string
	ImageURL string
	PubDate  *time.Time
}

func init() {
	logLevel := os.Getenv("LOG_LEVEL")
	logger.Init("scraper-worker", logLevel)
}

func loadConfig() {
	if v := os.Getenv("SCRAPER_INTERVAL"); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i > 0 {
			sleepInterval = i
		}
	}
	if v := os.Getenv("SCRAPER_BATCH"); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i > 0 {
			batchSize = i
		}
	}
	if v := os.Getenv("SCRAPER_ENRICH_LIMIT"); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i > 0 {
			enrichLimit = i
		}
	}
	if v := os.Getenv("SCRAPER_WORKERS"); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i > 0 {
			scraperWorkers = i
		}
	}
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getActiveURLs(ctx context.Context) ([]URLSource, error) {
	rows, err := pool.Query(ctx, `
	SELECT id, nombre, url, categoria_id, pais_id, idioma, active
	FROM fuentes_url 
	WHERE active = true
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []URLSource
	for rows.Next() {
		var s URLSource
		err := rows.Scan(&s.ID, &s.Nombre, &s.URL, &s.CategoriaID, &s.PaisID, &s.Idioma, &s.Active)
		if err != nil {
			continue
		}
		sources = append(sources, s)
	}
	return sources, nil
}

func updateSourceStatus(ctx context.Context, sourceID int64, status, message string, httpCode int) error {
	_, err := pool.Exec(ctx, `
	UPDATE fuentes_url 
	SET last_check = NOW(),
	    last_status = $1,
	    status_message = $2,
	    last_http_code = $3
	WHERE id = $4
	`, status, message, httpCode, sourceID)
	return err
}

func getNoticiasToEnrich(ctx context.Context, limit int) ([]Noticia, error) {
	rows, err := pool.Query(ctx, `
	SELECT id, titulo, COALESCE(resumen, ''), url, fuente_nombre
	FROM noticias
	WHERE (resumen IS NULL OR LENGTH(resumen) < 200)
	ORDER BY fecha DESC
	LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var noticias []Noticia
	for rows.Next() {
		var n Noticia
		if err := rows.Scan(&n.ID, &n.Titulo, &n.Resumen, &n.URL, &n.FuenteNombre); err != nil {
			continue
		}
		noticias = append(noticias, n)
	}
	return noticias, nil
}

func updateNoticiaResumen(ctx context.Context, noticiaID, newResumen string) error {
	_, err := pool.Exec(ctx, `
	UPDATE noticias
	SET resumen = $1
	WHERE id = $2
	`, newResumen, noticiaID)
	return err
}

func extractArticle(source URLSource) (*Article, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", source.URL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, err
	}

	article := &Article{
		URL: source.URL,
	}

	// Extract title
	article.Title = doc.Find("meta[property='og:title']").First().AttrOr("content", "")
	if article.Title == "" {
		article.Title = doc.Find("meta[name='title']").First().AttrOr("content", "")
	}
	if article.Title == "" {
		article.Title = doc.Find("h1").First().Text()
	}
	if article.Title == "" {
		article.Title = doc.Find("title").First().Text()
	}

	// Extract description/summary
	article.Summary = doc.Find("meta[property='og:description']").First().AttrOr("content", "")
	if article.Summary == "" {
		article.Summary = doc.Find("meta[name='description']").First().AttrOr("content", "")
	}

	// Extract image
	article.ImageURL = doc.Find("meta[property='og:image']").First().AttrOr("content", "")

	// Extract main content - try common selectors
	contentSelectors := []string{
		"article",
		"[role='main']",
		"main",
		".article-content",
		".post-content",
		".entry-content",
		".content",
		"#content",
	}

	for _, sel := range contentSelectors {
		content := doc.Find(sel).First()
		if content.Length() > 0 {
			article.Content = content.Text()
			break
		}
	}

	// Clean up
	article.Title = strings.TrimSpace(article.Title)
	article.Summary = strings.TrimSpace(article.Summary)
	article.Content = strings.TrimSpace(article.Content)

	// Truncate summary if too long
	if len(article.Summary) > 500 {
		article.Summary = article.Summary[:500]
	}

	return article, nil
}

func extractContentFromURL(url string) (string, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return "", err
	}

	contentSelectors := []string{
		"article",
		"[role='main']",
		"main",
		".article-content",
		".post-content",
		".entry-content",
		".content",
		"#content",
		".story-body",
		".article-body",
	}

	var content string
	for _, sel := range contentSelectors {
		content = doc.Find(sel).First().Text()
		if len(content) > 100 {
			break
		}
	}

	if len(content) < 100 {
		ps := doc.Find("p")
		var paragraphs []string
		ps.Each(func(i int, s *goquery.Selection) {
			txt := strings.TrimSpace(s.Text())
			if len(txt) > 50 {
				paragraphs = append(paragraphs, txt)
			}
		})
		content = strings.Join(paragraphs, "\n\n")
	}

	return strings.TrimSpace(content), nil
}

func processEnrichment(ctx context.Context, noticia Noticia) bool {
	logger.Info().Str("title", noticia.Titulo).Msg("Enriching")

	content, err := extractContentFromURL(noticia.URL)
	if err != nil {
		logger.Error().Err(err).Str("url", noticia.URL).Msg("Error extracting content")
		return false
	}

	if len(content) < 50 {
		logger.Warn().Str("url", noticia.URL).Msg("Not enough content extracted")
		return false
	}

	if err := updateNoticiaResumen(ctx, noticia.ID, content); err != nil {
		logger.Error().Err(err).Str("id", noticia.ID).Msg("Error updating resumen")
		return false
	}

	logger.Info().Str("title", noticia.Titulo).Int("length", len(content)).Msg("Enriched")
	return true
}

func saveArticle(ctx context.Context, source URLSource, article *Article) (bool, error) {
	finalURL := article.URL
	if finalURL == "" {
		finalURL = source.URL
	}

	// Generate ID from URL
	articleID := fmt.Sprintf("%x", md5.Sum([]byte(finalURL)))

	// Check if exists
	var exists bool
	err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM noticias WHERE id = $1)", articleID).Scan(&exists)
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}

	title := article.Title
	if title == "" {
		title = "Sin título"
	}

	summary := article.Summary
	if summary == "" && article.Content != "" {
		summary = article.Content
		if len(summary) > 500 {
			summary = summary[:500]
		}
	}

	pubDate := time.Now()
	if article.PubDate != nil {
		pubDate = *article.PubDate
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO noticias (
			id, titulo, resumen, url, fecha, imagen_url, 
			fuente_nombre, categoria_id, pais_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT DO NOTHING
	`, articleID, title, summary, finalURL, pubDate, article.ImageURL,
		source.Nombre, source.CategoriaID, source.PaisID)

	if err != nil {
		return false, err
	}

	return true, nil
}

func processSource(ctx context.Context, source URLSource) {
	logger.Info().Str("nombre", source.Nombre).Str("url", source.URL).Msg("Processing")

	article, err := extractArticle(source)
	if err != nil {
		logger.Error().Err(err).Str("url", source.URL).Msg("Error extracting article")
		status := "ERROR"
		if strings.Contains(err.Error(), "HTTP") {
			status = "ERROR_HTTP"
		}
		updateSourceStatus(ctx, source.ID, status, err.Error()[:200], 0)
		return
	}

	if article.Title == "" {
		logger.Warn().Str("url", source.URL).Msg("No title found")
		updateSourceStatus(ctx, source.ID, "ERROR_PARSE", "No title extracted", 200)
		return
	}

	saved, err := saveArticle(ctx, source, article)
	if err != nil {
		logger.Error().Err(err).Msg("Error saving article")
		updateSourceStatus(ctx, source.ID, "ERROR_DB", err.Error()[:200], 0)
		return
	}

	if saved {
		logger.Info().Str("title", article.Title).Msg("Saved")
		updateSourceStatus(ctx, source.ID, "OK", "News created successfully", 200)
	} else {
		logger.Info().Str("title", article.Title).Msg("Already exists")
		updateSourceStatus(ctx, source.ID, "OK", "News already exists", 200)
	}
}

// enrichConcurrent enriches noticias in parallel with a bounded worker pool.
func enrichConcurrent(ctx context.Context, noticias []Noticia) {
	limiter := make(chan struct{}, scraperWorkers)
	var wg sync.WaitGroup
	for _, noticia := range noticias {
		wg.Add(1)
		limiter <- struct{}{}
		go func(n Noticia) {
			defer wg.Done()
			defer func() { <-limiter }()
			if processEnrichment(ctx, n) {
				time.Sleep(1 * time.Second)
			}
		}(noticia)
	}
	wg.Wait()
}

// scrapeSources processes fuentes_url in parallel with a bounded worker pool.
func scrapeSources(ctx context.Context, sources []URLSource) {
	limiter := make(chan struct{}, scraperWorkers)
	var wg sync.WaitGroup
	for _, source := range sources {
		wg.Add(1)
		limiter <- struct{}{}
		go func(src URLSource) {
			defer wg.Done()
			defer func() { <-limiter }()
			time.Sleep(1 * time.Second) // polite pacing across workers
			processSource(ctx, src)
		}(source)
	}
	wg.Wait()
}

func Main() {
	loadConfig()
	logger.Info().Msg("Starting Scraper Worker")

	cfg := workers.LoadDBConfig()
	if err := workers.Connect(cfg); err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to database")
	}
	pool = workers.GetPool()
	defer workers.Close()

	logger.Info().Msg("Connected to PostgreSQL")

	// Start health check HTTP server
	go func() {
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			if err := workers.HealthCheck(r.Context()); err != nil {
				http.Error(w, "unhealthy", http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		})
		logger.Info().Msg("Health check server listening on :8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			logger.Error().Err(err).Msg("Health check server error")
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())

	// Handle shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info().Msg("Shutting down gracefully...")
		cancel()
		time.Sleep(2 * time.Second)
		workers.Close()
		os.Exit(0)
	}()

	logger.Info().Msgf("Config: sleep=%ds, batch=%d, enrich=%d", sleepInterval, batchSize, enrichLimit)

	ticker := time.NewTicker(time.Duration(sleepInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("Context cancelled, stopping worker")
			return
		case <-ticker.C:
			noticias, err := getNoticiasToEnrich(ctx, enrichLimit)
			if err != nil {
				logger.Error().Err(err).Msg("Error fetching noticias to enrich")
			} else if len(noticias) > 0 {
				logger.Info().Msgf("Enriching %d noticias (%d workers)", len(noticias), scraperWorkers)
				enrichConcurrent(ctx, noticias)
			}

			sources, err := getActiveURLs(ctx)
			if err != nil {
				logger.Error().Err(err).Msg("Error fetching URLs")
				continue
			}

			if len(sources) == 0 {
				logger.Info().Msg("No active URLs to process")
				continue
			}

			logger.Info().Msgf("Processing %d sources (%d workers)", len(sources), scraperWorkers)
			scrapeSources(ctx, sources)
		}
	}
}

func main() {
	Main()
}
