package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcdole/gofeed"
	"github.com/rss2/backend/internal/logger"
	"github.com/rss2/backend/internal/workers"
)

var (
	dbPool                *pgxpool.Pool
	sleepSec              = 900  // 15 minutes
	batchSize             = 10
	maxFailures           = 5
	reactivationIntervalH  = 24  // hours
	domainCooldownMinutes = 30  // minutes
)

// DomainCircuitBreaker tracks domains that are temporarily down
type DomainCircuitBreaker struct {
	mu       sync.RWMutex
	domains  map[string]time.Time // domain -> cooldown until
	cooldown time.Duration
}

func NewDomainCircuitBreaker(cooldownMinutes int) *DomainCircuitBreaker {
	return &DomainCircuitBreaker{
		domains:  make(map[string]time.Time),
		cooldown: time.Duration(cooldownMinutes) * time.Minute,
	}
}

// IsDomainDown checks if a domain is currently in cooldown
func (cb *DomainCircuitBreaker) IsDomainDown(domain string) bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	if cooldownUntil, exists := cb.domains[domain]; exists {
		return time.Now().Before(cooldownUntil)
	}
	return false
}

// MarkDomainDown marks a domain as down until cooldown expires
func (cb *DomainCircuitBreaker) MarkDomainDown(domain string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.domains[domain] = time.Now().Add(cb.cooldown)
}

// Cleanup removes expired domain entries
func (cb *DomainCircuitBreaker) Cleanup() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	now := time.Now()
	for domain, cooldownUntil := range cb.domains {
		if now.After(cooldownUntil) {
			delete(cb.domains, domain)
		}
	}
}

var circuitBreaker *DomainCircuitBreaker

type URLSource struct {
	ID           int64
	Nombre       string
	URL          string
	CategoriaID  *int64
	PaisID       *int64
	Idioma       *string
	IsHealthy    bool
	FailuresCount int
}

func init() {
	logLevel := os.Getenv("LOG_LEVEL")
	logger.Init("discovery-worker", logLevel)
}

func loadConfig() {
	sleepSec = getEnvInt("DISCOVERY_INTERVAL", 900)
	batchSize = getEnvInt("DISCOVERY_BATCH", 10)
	maxFailures = getEnvInt("MAX_FAILURES", 5)
	reactivationIntervalH = getEnvInt("REACTIVATION_INTERVAL_HOURS", 24)
	domainCooldownMinutes = getEnvInt("DOMAIN_COOLDOWN_MINUTES", 30)
	circuitBreaker = NewDomainCircuitBreaker(domainCooldownMinutes)
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}



// getHealthyURLs returns only healthy feeds for processing
func getHealthyURLs(ctx context.Context) ([]URLSource, error) {
	rows, err := dbPool.Query(ctx, `
		SELECT id, nombre, url, categoria_id, pais_id, idioma, is_healthy, failures_count
		FROM fuentes_url
		WHERE active = TRUE AND is_healthy = TRUE
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
		if err := rows.Scan(&s.ID, &s.Nombre, &s.URL, &s.CategoriaID, &s.PaisID, &s.Idioma, &s.IsHealthy, &s.FailuresCount); err != nil {
			continue
		}
		sources = append(sources, s)
	}
	return sources, nil
}

// getInactiveURLsForReactivation returns inactive feeds that should be tried for reactivation
func getInactiveURLsForReactivation(ctx context.Context) ([]URLSource, error) {
	reactivationBatch := getEnvInt("REACTIVATION_BATCH", 5)
	
	rows, err := dbPool.Query(ctx, `
		SELECT id, nombre, url, categoria_id, pais_id, idioma, is_healthy, failures_count
		FROM fuentes_url
		WHERE active = FALSE
		ORDER BY last_check ASC NULLS FIRST
		LIMIT $1
	`, reactivationBatch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []URLSource
	for rows.Next() {
		var s URLSource
		if err := rows.Scan(&s.ID, &s.Nombre, &s.URL, &s.CategoriaID, &s.PaisID, &s.Idioma, &s.IsHealthy, &s.FailuresCount); err != nil {
			continue
		}
		sources = append(sources, s)
	}
	return sources, nil
}

// shouldCheckReactivation returns true if enough time has passed since last reactivation check
func shouldCheckReactivation(ctx context.Context) (bool, error) {
	var lastReactivation *time.Time
	err := dbPool.QueryRow(ctx, `
		SELECT last_reactivation_check FROM system_state WHERE id = 1
	`).Scan(&lastReactivation)
	
	if err != nil {
		// Table might not exist or no row, assume we should check
		return true, nil
	}
	
	if lastReactivation == nil {
		return true, nil
	}
	
	nextCheck := lastReactivation.Add(time.Duration(reactivationIntervalH) * time.Hour)
	return time.Now().After(nextCheck), nil
}

// updateReactivationCheck updates the last reactivation check timestamp
func updateReactivationCheck(ctx context.Context) error {
	_, err := dbPool.Exec(ctx, `
		INSERT INTO system_state (id, last_reactivation_check) VALUES (1, NOW())
		ON CONFLICT (id) DO UPDATE SET last_reactivation_check = NOW()
	`)
	return err
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

// recordFeedSuccess updates feed health after successful check
func recordFeedSuccess(ctx context.Context, urlID int64) error {
	_, err := dbPool.Exec(ctx, `
		UPDATE fuentes_url
		SET last_check = NOW(),
		    last_status = 'success',
		    status_message = 'Feeds discovered successfully',
		    last_http_code = 200,
		    failures_count = 0,
		    last_success = NOW(),
		    is_healthy = TRUE,
		    active = TRUE
		WHERE id = $1
	`, urlID)
	return err
}

// recordFeedFailure handles failure tracking and potential deactivation
func recordFeedFailure(ctx context.Context, urlID int64, errorMsg string, httpCode int) error {
	// First get current failures_count
	var failuresCount int
	err := dbPool.QueryRow(ctx, `SELECT failures_count FROM fuentes_url WHERE id = $1`, urlID).Scan(&failuresCount)
	if err != nil {
		return err
	}

	newFailuresCount := failuresCount + 1
	isHealthy := true
	active := true

	if newFailuresCount >= maxFailures {
		isHealthy = false
		active = false
		logger.Warn().Int64("urlID", urlID).Int("failures", newFailuresCount).Msg("Feed marked inactive due to consecutive failures")
	}

	_, err = dbPool.Exec(ctx, `
		UPDATE fuentes_url
		SET last_check = NOW(),
		    last_status = 'error',
		    status_message = $1,
		    last_http_code = $2,
		    failures_count = $3,
		    is_healthy = $4,
		    active = $5
		WHERE id = $6
	`, errorMsg[:min(200, len(errorMsg))], httpCode, newFailuresCount, isHealthy, active, urlID)
	return err
}

// recordReactivationSuccess marks an inactive feed as reactivated
func recordReactivationSuccess(ctx context.Context, urlID int64) error {
	_, err := dbPool.Exec(ctx, `
		UPDATE fuentes_url
		SET last_check = NOW(),
		    last_status = 'reactivated',
		    status_message = 'Feed reactivated successfully',
		    last_http_code = 200,
		    failures_count = 0,
		    last_success = NOW(),
		    is_healthy = TRUE,
		    active = TRUE
		WHERE id = $1
	`, urlID)
	return err
}

// extractDomain extracts the domain from a URL for circuit breaker
func extractDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		// Try to extract domain directly
		domain := strings.TrimPrefix(rawURL, "https://")
		domain = strings.TrimPrefix(domain, "http://")
		if idx := strings.Index(domain, "/"); idx != -1 {
			domain = domain[:idx]
		}
		return domain
	}
	return u.Host
}

// isRateLimitOrServerError checks if HTTP code indicates overloaded server
func isRateLimitOrServerError(httpCode int) bool {
	return httpCode == 429 || httpCode == 503
}

func discoverFeeds(pageURL string) ([]string, error) {
	domain := extractDomain(pageURL)
	
	// Check circuit breaker
	if circuitBreaker.IsDomainDown(domain) {
		logger.Warn().Str("domain", domain).Str("url", pageURL).Msg("Skipping domain due to circuit breaker")
		return nil, fmt.Errorf("domain %s is in circuit breaker cooldown", domain)
	}

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
		// Mark domain as down on timeout or connection errors
		circuitBreaker.MarkDomainDown(domain)
		return nil, err
	}
	defer resp.Body.Close()

	// Check for rate limit or server errors
	if isRateLimitOrServerError(resp.StatusCode) {
		logger.Warn().Int("statusCode", resp.StatusCode).Str("domain", domain).Msg("Rate limit or server error detected")
		circuitBreaker.MarkDomainDown(domain)
		return nil, fmt.Errorf("HTTP %d from domain %s", resp.StatusCode, domain)
	}

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
	domain := extractDomain(feedURL)
	
	// Check circuit breaker
	if circuitBreaker.IsDomainDown(domain) {
		return nil, fmt.Errorf("domain %s is in circuit breaker cooldown", domain)
	}

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
		// Mark domain as down on timeout or connection errors
		circuitBreaker.MarkDomainDown(domain)
		return nil, err
	}
	defer resp.Body.Close()

	// Check for rate limit or server errors
	if isRateLimitOrServerError(resp.StatusCode) {
		logger.Warn().Int("statusCode", resp.StatusCode).Str("domain", domain).Msg("Rate limit or server error detected")
		circuitBreaker.MarkDomainDown(domain)
		return nil, fmt.Errorf("HTTP %d from domain %s", resp.StatusCode, domain)
	}

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

// processURLSource processes a healthy URL source
func processURLSource(ctx context.Context, source URLSource) {
	logger.Info().Str("nombre", source.Nombre).Str("url", source.URL).Msg("Processing healthy source")

	// Try to find feeds on this URL
	feeds, err := discoverFeeds(source.URL)
	if err != nil {
		logger.Error().Err(err).Str("url", source.URL).Msg("Error discovering feeds")
		recordFeedFailure(ctx, source.ID, err.Error(), 0)
		return
	}

	if len(feeds) == 0 {
		logger.Warn().Str("url", source.URL).Msg("No feeds found")
		updateURLStatus(ctx, source.ID, "no_feeds", "No feeds found", 200)
		// No feeds found is not a failure per se, but we still track it
		recordFeedSuccess(ctx, source.ID)
		return
	}

	logger.Info().Int("count", len(feeds)).Str("url", source.URL).Msg("Found feeds")

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
			logger.Error().Err(err).Str("feedURL", feedURL).Msg("Error parsing feed")
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
				logger.Error().Err(err).Str("feedURL", feedURL).Msg("Error creating pending feed")
				errors++
			} else {
				pending++
			}
		} else {
			// Create feed directly
			createdFeed, err := createFeedDirectly(ctx, feedURL, &source.ID, source.CategoriaID, source.PaisID, source.Idioma)
			if err != nil {
				logger.Error().Err(err).Str("feedURL", feedURL).Msg("Error creating feed")
				errors++
			} else if createdFeed {
				created++
			} else {
				existing++
			}
		}

		time.Sleep(1 * time.Second) // Rate limiting
	}

	// Update status and record success
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

	// If we found feeds (created or pending), record as success
	if created > 0 || pending > 0 {
		recordFeedSuccess(ctx, source.ID)
	} else if existing > 0 {
		// Existing feeds means the source is working
		recordFeedSuccess(ctx, source.ID)
	}

	updateURLStatus(ctx, source.ID, status, message, 200)
	logger.Info().Str("url", source.URL).Int("created", created).Int("pending", pending).Int("existing", existing).Int("errors", errors).Msg("Processed")
}

// processInactiveSource attempts to reactivate an inactive feed
func processInactiveSource(ctx context.Context, source URLSource) {
	logger.Info().Str("nombre", source.Nombre).Str("url", source.URL).Msg("Attempting to reactivate inactive source")

	// Try to find feeds on this URL
	feeds, err := discoverFeeds(source.URL)
	if err != nil {
		logger.Warn().Err(err).Str("url", source.URL).Msg("Reactivation failed")
		// Don't record failure for inactive feeds during reactivation attempts
		// Just update last_check to avoid constant retries
		updateURLStatus(ctx, source.ID, "reactivation_failed", err.Error()[:200], 0)
		return
	}

	if len(feeds) == 0 {
		logger.Warn().Str("url", source.URL).Msg("No feeds found during reactivation attempt")
		updateURLStatus(ctx, source.ID, "no_feeds", "No feeds found", 200)
		return
	}

	logger.Info().Int("count", len(feeds)).Str("url", source.URL).Msg("Reactivation successful - found feeds")
	
	// Record reactivation success
	recordReactivationSuccess(ctx, source.ID)
	
	// Process the found feeds
	maxFeeds := getEnvInt("MAX_FEEDS_PER_URL", 5)
	if len(feeds) > maxFeeds {
		feeds = feeds[:maxFeeds]
	}

	autoApprove := source.CategoriaID != nil && source.PaisID != nil

	created := 0
	pending := 0
	existing := 0

	for _, feedURL := range feeds {
		title, description, language, entryCount, err := getFeedMetadata(feedURL)
		if err != nil {
			logger.Error().Err(err).Str("feedURL", feedURL).Msg("Error parsing reactivated feed")
			continue
		}

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
			if err := createPendingFeed(ctx, source.ID, feedURL, metadata); err != nil {
				logger.Error().Err(err).Str("feedURL", feedURL).Msg("Error creating pending feed from reactivated source")
			} else {
				pending++
			}
		} else {
			createdFeed, err := createFeedDirectly(ctx, feedURL, &source.ID, source.CategoriaID, source.PaisID, source.Idioma)
			if err != nil {
				logger.Error().Err(err).Str("feedURL", feedURL).Msg("Error creating feed from reactivated source")
			} else if createdFeed {
				created++
			} else {
				existing++
			}
		}

		time.Sleep(1 * time.Second) // Rate limiting
	}

	logger.Info().Str("url", source.URL).Int("created", created).Int("pending", pending).Int("existing", existing).Msg("Reactivated source processed")
}

func Main() {
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

	logger.Info().Msgf("Config: interval=%ds, batch=%d, maxFailures=%d, reactivationIntervalH=%d, domainCooldown=%dmin",
		sleepSec, batchSize, maxFailures, reactivationIntervalH, domainCooldownMinutes)

	ticker := time.NewTicker(time.Duration(sleepSec) * time.Second)
	defer ticker.Stop()

	// Cleanup circuit breaker periodically
	go func() {
		cleanupTicker := time.NewTicker(5 * time.Minute)
		defer cleanupTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-cleanupTicker.C:
				circuitBreaker.Cleanup()
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("Context cancelled, stopping worker")
			return
		case <-ticker.C:
			// Check circuit breaker status
			circuitBreaker.Cleanup()

			// Process healthy sources
			sources, err := getHealthyURLs(ctx)
			if err != nil {
				logger.Error().Err(err).Msg("Error fetching healthy URLs")
				continue
			}

			if len(sources) == 0 {
				logger.Info().Msg("No healthy URLs to process")
			} else {
				logger.Info().Msgf("Processing %d healthy sources", len(sources))
				for _, source := range sources {
					processURLSource(ctx, source)
					time.Sleep(2 * time.Second)
				}
			}

			// Check for reactivation
			shouldReactivate, err := shouldCheckReactivation(ctx)
			if err != nil {
				logger.Error().Err(err).Msg("Error checking reactivation eligibility")
			} else if shouldReactivate {
				logger.Info().Msg("Checking for feeds to reactivate")
				updateReactivationCheck(ctx)
				
				inactiveSources, err := getInactiveURLsForReactivation(ctx)
				if err != nil {
					logger.Error().Err(err).Msg("Error fetching inactive URLs for reactivation")
				} else if len(inactiveSources) > 0 {
					logger.Info().Msgf("Attempting to reactivate %d inactive sources", len(inactiveSources))
					for _, source := range inactiveSources {
						processInactiveSource(ctx, source)
						time.Sleep(5 * time.Second) // Longer delay between reactivation attempts
					}
				}
			}
		}
	}
}

func main() {
	Main()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
