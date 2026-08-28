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
	"github.com/rss2/backend/internal/logger"
	"github.com/rss2/backend/internal/workers"
)

var (
	dbPool   *pgxpool.Pool
	sleepSec = 10
	batchSz  = 500
)

type Topic struct {
	ID       int64
	Weight   int
	Keywords []string
}

type Country struct {
	ID       int64
	Name     string
	Keywords []string
}

func init() {
	logLevel := os.Getenv("LOG_LEVEL")
	logger.Init("topics-worker", logLevel)
}

func loadConfig() {
	sleepSec = getEnvInt("TOPICS_SLEEP", 10)
	batchSz = getEnvInt("TOPICS_BATCH", 500)
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func ensureSchema(ctx context.Context) error {
	_, err := dbPool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS topics (
			id SERIAL PRIMARY KEY,
			slug VARCHAR(50) UNIQUE NOT NULL,
			name VARCHAR(100) NOT NULL,
			weight INTEGER DEFAULT 1,
			keywords TEXT,
			group_name VARCHAR(50)
		);
	`)
	if err != nil {
		return err
	}

	_, err = dbPool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS news_topics (
			noticia_id VARCHAR(32) REFERENCES noticias(id) ON DELETE CASCADE,
			topic_id INTEGER REFERENCES topics(id) ON DELETE CASCADE,
			score INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT NOW(),
			PRIMARY KEY (noticia_id, topic_id)
		);
	`)
	if err != nil {
		return err
	}

	_, err = dbPool.Exec(ctx, `
		ALTER TABLE noticias ADD COLUMN IF NOT EXISTS topics_processed BOOLEAN DEFAULT FALSE;
	`)
	return err
}

func loadTopics(ctx context.Context) ([]Topic, error) {
	rows, err := dbPool.Query(ctx, "SELECT id, weight, keywords FROM topics")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var topics []Topic
	for rows.Next() {
		var t Topic
		var kwStr *string
		if err := rows.Scan(&t.ID, &t.Weight, &kwStr); err != nil {
			continue
		}
		if kwStr != nil {
			keywords := strings.Split(*kwStr, ",")
			for i := range keywords {
				keywords[i] = strings.ToLower(strings.TrimSpace(keywords[i]))
			}
			t.Keywords = keywords
		}
		topics = append(topics, t)
	}
	return topics, nil
}

func loadCountries(ctx context.Context) ([]Country, error) {
	rows, err := dbPool.Query(ctx, "SELECT id, nombre FROM paises")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	aliases := map[string][]string{
		"Estados Unidos": {"eeuu", "ee.uu.", "usa", "estadounidense", "washington"},
		"Rusia":          {"ruso", "rusa", "moscú", "kremlin"},
		"China":          {"chino", "china", "pekin", "beijing"},
		"Ucrania":        {"ucraniano", "kiev", "kyiv"},
		"Israel":         {"israelí", "tel aviv", "jerusalén"},
		"España":         {"español", "madrid"},
		"Reino Unido":    {"uk", "londres", "británico"},
		"Francia":        {"francés", "parís"},
		"Alemania":       {"alemán", "berlín"},
		"Palestina":      {"palestino", "gaza", "cisjordania"},
		"Irán":           {"iraní", "teherán"},
	}

	var countries []Country
	for rows.Next() {
		var c Country
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			continue
		}
		c.Keywords = []string{strings.ToLower(c.Name)}
		if kw, ok := aliases[c.Name]; ok {
			c.Keywords = append(c.Keywords, kw...)
		}
		countries = append(countries, c)
	}
	return countries, nil
}

type NewsItem struct {
	ID      string
	Titulo  *string
	Resumen *string
}

func fetchPendingNews(ctx context.Context, limit int) ([]NewsItem, error) {
	rows, err := dbPool.Query(ctx, `
		SELECT id, titulo, resumen 
		FROM noticias 
		WHERE topics_processed = FALSE 
		ORDER BY fecha DESC 
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []NewsItem
	for rows.Next() {
		var n NewsItem
		if err := rows.Scan(&n.ID, &n.Titulo, &n.Resumen); err != nil {
			continue
		}
		items = append(items, n)
	}
	return items, nil
}

func findTopics(text string, topics []Topic) []struct {
	TopicID int64
	Score   int
} {
	text = strings.ToLower(text)
	var matches []struct {
		TopicID int64
		Score   int
	}

	for _, topic := range topics {
		count := 0
		for _, kw := range topic.Keywords {
			if strings.Contains(text, kw) {
				count++
			}
		}
		if count > 0 {
			matches = append(matches, struct {
				TopicID int64
				Score   int
			}{topic.ID, topic.Weight * count})
		}
	}
	return matches
}

func findBestCountry(text string, countries []Country) *int64 {
	text = strings.ToLower(text)
	bestID := new(int64)
	bestCount := 0

	for _, c := range countries {
		count := 0
		for _, kw := range c.Keywords {
			if strings.Contains(text, kw) {
				count++
			}
		}
		if count > bestCount {
			bestCount = count
			*bestID = c.ID
		}
	}

	if bestCount > 0 {
		return bestID
	}
	return nil
}

func processBatch(ctx context.Context, topics []Topic, countries []Country) (int, error) {
	items, err := fetchPendingNews(ctx, batchSz)
	if err != nil {
		return 0, err
	}

	if len(items) == 0 {
		return 0, nil
	}

	type topicMatch struct {
		NoticiaID string
		TopicID   int64
		Score     int
	}

	type countryUpdate struct {
		PaisID    int64
		NoticiaID string
	}

	var topicMatches []topicMatch
	var countryUpdates []countryUpdate
	var processedIDs []string

	for _, item := range items {
		var text string
		if item.Titulo != nil {
			text += *item.Titulo
		}
		if item.Resumen != nil {
			text += " " + *item.Resumen
		}

		// Find topics
		matches := findTopics(text, topics)
		for _, m := range matches {
			topicMatches = append(topicMatches, topicMatch{item.ID, m.TopicID, m.Score})
		}

		// Find best country
		if countryID := findBestCountry(text, countries); countryID != nil {
			countryUpdates = append(countryUpdates, countryUpdate{*countryID, item.ID})
		}

		processedIDs = append(processedIDs, item.ID)
	}

	// Insert topic relations (batched, upsert)
	if len(topicMatches) > 0 {
		const chunkSize = 500
		for i := 0; i < len(topicMatches); i += chunkSize {
			end := i + chunkSize
			if end > len(topicMatches) {
				end = len(topicMatches)
			}
			chunk := topicMatches[i:end]
			values := make([]string, 0, len(chunk))
			args := make([]interface{}, 0, len(chunk)*3)
			for j, tm := range chunk {
				values = append(values, fmt.Sprintf("($%d, $%d, $%d)", j*3+1, j*3+2, j*3+3))
				args = append(args, tm.NoticiaID, tm.TopicID, tm.Score)
			}
			query := fmt.Sprintf(`
				INSERT INTO news_topics (noticia_id, topic_id, score)
				VALUES %s
				ON CONFLICT (noticia_id, topic_id) DO UPDATE SET score = EXCLUDED.score
			`, strings.Join(values, ","))
			if _, err := dbPool.Exec(ctx, query, args...); err != nil {
				logger.Printf("Error batch inserting topics: %v", err)
			}
		}
	}

	// Update country (single bulk UPDATE)
	if len(countryUpdates) > 0 {
		paisSlice := make([]int32, len(countryUpdates))
		idSlice := make([]string, len(countryUpdates))
		for i, cu := range countryUpdates {
			paisSlice[i] = int32(cu.PaisID)
			idSlice[i] = cu.NoticiaID
		}
		if _, err := dbPool.Exec(ctx, `
			UPDATE noticias n SET pais_id = u.pais
			FROM unnest($1::int[], $2::varchar[]) AS u(pais, noticia_id)
			WHERE n.id = u.noticia_id
		`, paisSlice, idSlice); err != nil {
			logger.Printf("Error bulk updating countries: %v", err)
		}
	}

	// Mark as processed
	if len(processedIDs) > 0 {
		_, err := dbPool.Exec(ctx, `
			UPDATE noticias SET topics_processed = TRUE WHERE id = ANY($1)
		`, processedIDs)
		if err != nil {
			return 0, err
		}
	}

	return len(items), nil
}

func main() {
	loadConfig()
	logger.Info().Msg("Starting Topics Worker")

	cfg := workers.LoadDBConfig()
	if err := workers.Connect(cfg); err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to database")
	}
	dbPool = workers.GetPool()
	defer workers.Close()

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

	// Ensure schema
	if err := ensureSchema(ctx); err != nil {
		logger.Error().Err(err).Msg("Error ensuring schema")
	}

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

	logger.Info().Msgf("Config: sleep=%ds, batch=%d", sleepSec, batchSz)

	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("Context cancelled, stopping worker")
			return
		case <-time.After(time.Duration(sleepSec) * time.Second):
			topics, err := loadTopics(ctx)
			if err != nil {
				logger.Error().Err(err).Msg("Error loading topics")
				continue
			}

			if len(topics) == 0 {
				logger.Info().Msg("No topics found in DB")
				time.Sleep(time.Duration(sleepSec) * time.Second)
				continue
			}

			countries, err := loadCountries(ctx)
			if err != nil {
				logger.Error().Err(err).Msg("Error loading countries")
				continue
			}

			count, err := processBatch(ctx, topics, countries)
			if err != nil {
				logger.Error().Err(err).Msg("Error processing batch")
				continue
			}

			if count > 0 {
				logger.Info().Msgf("Processed %d news items", count)
			}

			if count < batchSz {
				time.Sleep(time.Duration(sleepSec) * time.Second)
			}
		}
	}
}
