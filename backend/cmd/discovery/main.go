package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcdole/gofeed"
	"github.com/rss2/backend/internal/logger"
	"github.com/rss2/backend/internal/workers"
)

var (
	pool      *workers.Config
	dbPool    *pgxpool.Pool
	sleepSec  = 900 // 15 minutes
	batchSize = 10
)

type URLSource struct {
	ID          int64
	Nombre      string
	URL         string
	CategoriaID *int64
	PaisID      *int64
	Idioma      *string
}

func init() {
	logLevel := os.Getenv("LOG_LEVEL")
	logger.Init("discovery-worker", logLevel)
}

func loadConfig() {
	sleepSec = getEnvInt("DISCOVERY_INTERVAL", 900)
	batchSize = getEnvInt("DISCOVERY_BATCH", 10)
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getPendingURLs(ctx context.Context) ([]URLSource, error) {
	rows, err := dbPool.Query(ctx, `
		SELECT id, nombre, url, categoria_id, pais_id, idioma
		FROM fuentes_url
		WHERE active = TRUE
		ORDER BY 
			CASE 
				WHEN last_check IS NULL THEN 1
				WHEN last_status = 'error' THEN 2
				WHEN last_status = 'no_feeds' THEN 3
				ELSE 4
			END,
			last_check ASC NULLS FIRST
		LIMIT $1
	`, batchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []URLSource
	for rows.Next() {
		var s URLSource
		if err := rows.Scan(&s.ID, &s.Nombre, &s.URL, &s.CategoriaID, &s.PaisID, &s.Idioma); err != nil {
			continue
		}
		sources = append(sources, s)
	}
	return sources, nil
}

func updateURLStatus(ctx context.Context, urlID int64, status, message string, httpCode int) error {
	_, err := dbPool.Exec(ctx, `
		UPDATE fuentes_url
		SET last_check = NOW(),
		    last_status = $1,
		    status_message = $2,
		    last_http_code = $3
		WHERE id = $4
	`, status, message, httpCode, urlID)
	return err
}

func discoverFeeds(pageURL string) ([]string, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; RSS2Bot/1.0)")
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml, text/html")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Try to parse as feed first
	parser := gofeed.NewParser()
	feed, err := parser.Parse(resp.Body)
	if err == nil && feed != nil && len(feed.Items) > 0 {
		// It's a valid feed
		return []string{pageURL}, nil
	}

	// If not a feed, try to find feeds in HTML
	return findFeedLinksInHTML(pageURL)
}

func findFeedLinksInHTML(baseURL string) ([]string, error) {
	// Simple feed link finder - returns empty for now
	// In production, use goquery to parse HTML and find RSS/Atom links
	return []string{}, nil
}

func parseFeed(feedURL string) (*gofeed.Feed, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", feedURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; RSS2Bot/1.0)")
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	parser := gofeed.NewParser()
	return parser.Parse(resp.Body)
}

func getFeedMetadata(feedURL string) (title, description, language string, entryCount int, err error) {
	feed, err := parseFeed(feedURL)
	if err != nil {
		return "", "", "", 0, err
	}

	title = feed.Title
	if title == "" {
		title = "Feed sin título"
	}

	description = feed.Description
	if len(description) > 500 {
		description = description[:500]
	}

	language = feed.Language
	entryCount = len(feed.Items)

	return title, description, language, entryCount, nil
}

func analyzeFeed(title, url, description string) (country, category string) {
	// Simple heuristics - in production use ML or API
	lowerTitle := strings.ToLower(title)
	lowerDesc := strings.ToLower(description)
	combined := lowerTitle + " " + lowerDesc

	// Detect country
	countries := map[string][]string{
		"España":      {"españa", "español", "madrid", "barcelona"},
		"Argentina":   {"argentino", "buenos aires"},
		"México":      {"méxico", "mexicano", "cdmx", "ciudad de méxico"},
		"Colombia":    {"colombiano", "bogotá"},
		"Chile":       {"chileno", "santiago"},
		"Perú":        {"peruano", "lima"},
		"EE.UU.":      {"estados unidos", "washington", "trump", "biden"},
		"Reino Unido": {"reino unido", "londres", "uk"},
		"Francia":     {"francia", "parís"},
		"Alemania":    {"alemania", "berlín"},
	}

	for country, keywords := range countries {
		for _, kw := range keywords {
			if strings.Contains(combined, kw) {
				return country, ""
			}
		}
	}

	return "", ""
}

func getCountryIDByName(ctx context.Context, countryName string) (*int64, error) {
	var id int64
	err := dbPool.QueryRow(ctx, "SELECT id FROM paises WHERE LOWER(nombre) = LOWER($1)", countryName).Scan(&id)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func getCategoryIDByName(ctx context.Context, categoryName string) (*int64, error) {
	var id int64
	err := dbPool.QueryRow(ctx, "SELECT id FROM categorias WHERE LOWER(nombre) = LOWER($1)", categoryName).Scan(&id)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func createPendingFeed(ctx context.Context, fuenteURLID int64, feedURL string, metadata map[string]interface{}) error {
	feedTitle := metadata["title"].(string)
	if feedTitle == "" {
		feedTitle = "Feed sin título"
	}

	description := ""
	if d, ok := metadata["description"].(string); ok {
		description = d
	}

	language := ""
	if l, ok := metadata["language"].(string); ok {
		language = l
	}

	entryCount := 0
	if c, ok := metadata["entry_count"].(int); ok {
		entryCount = c
	}

	detectedCountry := ""
	if dc, ok := metadata["detected_country"].(string); ok {
		detectedCountry = dc
	}

	var detectedCountryID *int64
	if detectedCountry != "" {
		if cid, err := getCountryIDByName(ctx, detectedCountry); err == nil {
			detectedCountryID = cid
		}
	}

	suggestedCategory := ""
	if sc, ok := metadata["suggested_category"].(string); ok {
		suggestedCategory = sc
	}

	var suggestedCategoryID *int64
	if suggestedCategory != "" {
		if caid, err := getCategoryIDByName(ctx, suggestedCategory); err == nil {
			suggestedCategoryID = caid
		}
	}

	_, err := dbPool.Exec(ctx, `
		INSERT INTO feeds_pending (
			fuente_url_id, feed_url, feed_title, feed_description,
			feed_language, feed_type, entry_count,
			detected_country_id, suggested_categoria_id,
			discovered_at
		)
		VALUES ($1, $2, $3, $4, $5, 'rss', $6, $7, $8, NOW())
		ON CONFLICT (feed_url) DO UPDATE
			SET feed_title = EXCLUDED.feed_title,
			    discovered_at = NOW()
	`, fuenteURLID, feedURL, feedTitle, description, language, entryCount, detectedCountryID, suggestedCategoryID)

	return err
}

func createFeedDirectly(ctx context.Context, feedURL string, fuenteURLID *int64, categoriaID, paisID *int64, idioma *string) (bool, error) {
	title, description, language, _, err := getFeedMetadata(feedURL)
	if err != nil {
		return false, err
	}

	if language == "" && idioma != nil {
		language = *idioma
	}

	var feedID int64
	err = dbPool.QueryRow(ctx, `
		INSERT INTO feeds (nombre, descripcion, url, categoria_id, pais_id, idioma, fuente_url_id, activo)
		VALUES ($1, $2, $3, $4, $5, $6, $7, TRUE)
		ON CONFLICT (url) DO NOTHING
		RETURNING id
	`, title, description, feedURL, categoriaID, paisID, language, fuenteURLID).Scan(&feedID)

	if err != nil {
		return false, err
	}

	return feedID > 0, nil
}

func processURLSource(ctx context.Context, source URLSource) {
	logger.Printf("Processing: %s (%s)", source.Nombre, source.URL)

	// Try to find feeds on this URL
	feeds, err := discoverFeeds(source.URL)
	if err != nil {
		logger.Printf("Error discovering feeds: %v", err)
		updateURLStatus(ctx, source.ID, "error", err.Error()[:200], 0)
		return
	}

	if len(feeds) == 0 {
		logger.Printf("No feeds found for: %s", source.URL)
		updateURLStatus(ctx, source.ID, "no_feeds", "No feeds found", 200)
		return
	}

	logger.Printf("Found %d feeds for %s", len(feeds), source.URL)

	maxFeeds := getEnvInt("MAX_FEEDS_PER_URL", 5)
	if len(feeds) > maxFeeds {
		feeds = feeds[:maxFeeds]
	}

	autoApprove := source.CategoriaID != nil && source.PaisID != nil

	created := 0
	pending := 0
	existing := 0
	errors := 0

	for _, feedURL := range feeds {
		// Get feed metadata
		title, description, language, entryCount, err := getFeedMetadata(feedURL)
		if err != nil {
			logger.Printf("Error parsing feed %s: %v", feedURL, err)
			errors++
			continue
		}

		// Analyze for country/category
		detectedCountry, suggestedCategory := analyzeFeed(title, feedURL, description)

		metadata := map[string]interface{}{
			"title":              title,
			"description":        description,
			"language":           language,
			"entry_count":        entryCount,
			"detected_country":   detectedCountry,
			"suggested_category": suggestedCategory,
		}

		if !autoApprove {
			// Create pending feed for review
			if err := createPendingFeed(ctx, source.ID, feedURL, metadata); err != nil {
				logger.Printf("Error creating pending feed: %v", err)
				errors++
			} else {
				pending++
			}
		} else {
			// Create feed directly
			createdFeed, err := createFeedDirectly(ctx, feedURL, &source.ID, source.CategoriaID, source.PaisID, source.Idioma)
			if err != nil {
				logger.Printf("Error creating feed: %v", err)
				errors++
			} else if createdFeed {
				created++
			} else {
				existing++
			}
		}

		time.Sleep(1 * time.Second) // Rate limiting
	}

	// Update status
	var status string
	var message string
	if created > 0 || pending > 0 {
		status = "success"
		parts := []string{}
		if created > 0 {
			parts = append(parts, fmt.Sprintf("%d created", created))
		}
		if pending > 0 {
			parts = append(parts, fmt.Sprintf("%d pending", pending))
		}
		message = strings.Join(parts, ", ")
	} else if existing > 0 {
		status = "existing"
		message = fmt.Sprintf("%d already existed", existing)
	} else {
		status = "error"
		message = fmt.Sprintf("%d errors", errors)
	}

	updateURLStatus(ctx, source.ID, status, message, 200)
	logger.Printf("Processed %s: created=%d, pending=%d, existing=%d, errors=%d",
		source.URL, created, pending, existing, errors)
}

func main() {
	loadConfig()
	logger.Info().Msg("Starting RSS Discovery Worker")

	cfg := workers.LoadDBConfig()
	if err := workers.Connect(cfg); err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to database")
	}
	dbPool = workers.GetPool()
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

	logger.Info().Msgf("Config: interval=%ds, batch=%d", sleepSec, batchSize)

	ticker := time.NewTicker(time.Duration(sleepSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("Context cancelled, stopping worker")
			return
		case <-ticker.C:
			sources, err := getPendingURLs(ctx)
			if err != nil {
				logger.Error().Err(err).Msg("Error fetching URLs")
				continue
			}

			if len(sources) == 0 {
				logger.Info().Msg("No pending URLs to process")
				continue
			}

			logger.Info().Msgf("Processing %d sources", len(sources))

			for _, source := range sources {
				processURLSource(ctx, source)
				time.Sleep(2 * time.Second)
			}
		}
	}
}
