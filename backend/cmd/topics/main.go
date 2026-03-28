package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rss2/backend/internal/workers"
)

var (
	logger   *log.Logger
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
	logger = log.New(os.Stdout, "[TOPICS] ", log.LstdFlags)
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

	// Insert topic relations
	if len(topicMatches) > 0 {
		for _, tm := range topicMatches {
			_, err := dbPool.Exec(ctx, `
				INSERT INTO news_topics (noticia_id, topic_id, score)
				VALUES ($1, $2, $3)
				ON CONFLICT (noticia_id, topic_id) DO UPDATE SET score = EXCLUDED.score
			`, tm.NoticiaID, tm.TopicID, tm.Score)
			if err != nil {
				logger.Printf("Error inserting topic: %v", err)
			}
		}
	}

	// Update country
	if len(countryUpdates) > 0 {
		for _, cu := range countryUpdates {
			_, err := dbPool.Exec(ctx, `
				UPDATE noticias SET pais_id = $1 WHERE id = $2
			`, cu.PaisID, cu.NoticiaID)
			if err != nil {
				logger.Printf("Error updating country: %v", err)
			}
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
	logger.Println("Starting Topics Worker")

	cfg := workers.LoadDBConfig()
	if err := workers.Connect(cfg); err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}
	dbPool = workers.GetPool()
	defer workers.Close()

	ctx := context.Background()

	// Ensure schema
	if err := ensureSchema(ctx); err != nil {
		logger.Printf("Error ensuring schema: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Println("Shutting down...")
		os.Exit(0)
	}()

	logger.Printf("Config: sleep=%ds, batch=%d", sleepSec, batchSz)

	for {
		select {
		case <-time.After(time.Duration(sleepSec) * time.Second):
			topics, err := loadTopics(ctx)
			if err != nil {
				logger.Printf("Error loading topics: %v", err)
				continue
			}

			if len(topics) == 0 {
				logger.Println("No topics found in DB")
				time.Sleep(time.Duration(sleepSec) * time.Second)
				continue
			}

			countries, err := loadCountries(ctx)
			if err != nil {
				logger.Printf("Error loading countries: %v", err)
				continue
			}

			count, err := processBatch(ctx, topics, countries)
			if err != nil {
				logger.Printf("Error processing batch: %v", err)
				continue
			}

			if count > 0 {
				logger.Printf("Processed %d news items", count)
			}

			if count < batchSz {
				time.Sleep(time.Duration(sleepSec) * time.Second)
			}
		}
	}
}
