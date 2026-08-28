package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rss2/backend/internal/logger"
	"github.com/rss2/backend/internal/workers"
)

var (
	dbPool   *pgxpool.Pool
	sleepSec = 10
	topK     = 10
	batchSz  = 200
	minScore = 0.0
)

func init() {
	logLevel := os.Getenv("LOG_LEVEL")
	logger.Init("related-worker", logLevel)
}

func loadConfig() {
	sleepSec = getEnvInt("RELATED_SLEEP", 10)
	topK = getEnvInt("RELATED_TOPK", 10)
	batchSz = getEnvInt("RELATED_BATCH", 200)
	minScore = getEnvFloat("RELATED_MIN_SCORE", 0.0)
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			return floatVal
		}
	}
	return defaultValue
}

type Translation struct {
	ID        int64
	Titulo    string
	Resumen   string
	Embedding []float64
}

func ensureSchema(ctx context.Context) error {
	_, err := dbPool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS related_noticias (
			traduccion_id INTEGER REFERENCES traducciones(id) ON DELETE CASCADE,
			related_traduccion_id INTEGER REFERENCES traducciones(id) ON DELETE CASCADE,
			score FLOAT NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT NOW(),
			PRIMARY KEY (traduccion_id, related_traduccion_id)
		);
	`)
	if err != nil {
		return err
	}

	// Ensure traduccion_embeddings table exists
	_, err = dbPool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS traduccion_embeddings (
			id SERIAL PRIMARY KEY,
			traduccion_id INTEGER NOT NULL REFERENCES traducciones(id) ON DELETE CASCADE,
			model TEXT NOT NULL,
			dim INTEGER NOT NULL,
			embedding DOUBLE PRECISION[] NOT NULL,
			created_at TIMESTAMP DEFAULT NOW(),
			UNIQUE (traduccion_id, model)
		);
	`)
	if err != nil {
		return err
	}

	_, err = dbPool.Exec(ctx, `
		CREATE INDEX IF NOT EXISTS idx_tr_emb_model ON traduccion_embeddings(model);
	`)
	if err != nil {
		return err
	}

	_, err = dbPool.Exec(ctx, `
		CREATE INDEX IF NOT EXISTS idx_tr_emb_traduccion_id ON traduccion_embeddings(traduccion_id);
	`)

	return err
}

func fetchAllEmbeddings(ctx context.Context, model string) ([]Translation, error) {
	rows, err := dbPool.Query(ctx, `
		SELECT e.traduccion_id, 
		       COALESCE(NULLIF(t.titulo_trad,''), ''), 
		       COALESCE(NULLIF(t.resumen_trad,''), ''),
		       e.embedding
		FROM traduccion_embeddings e
		JOIN traducciones t ON t.id = e.traduccion_id
		WHERE e.model = $1
		  AND t.status = 'done'
		  AND t.lang_to = 'es'
	`, model)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var translations []Translation
	for rows.Next() {
		var t Translation
		if err := rows.Scan(&t.ID, &t.Titulo, &t.Resumen, &t.Embedding); err != nil {
			continue
		}
		translations = append(translations, t)
	}
	return translations, nil
}

func fetchPendingIDs(ctx context.Context, model string, limit int) ([]int64, error) {
	rows, err := dbPool.Query(ctx, `
		SELECT t.id
		FROM traducciones t
		JOIN traduccion_embeddings e ON e.traduccion_id = t.id AND e.model = $1
		LEFT JOIN related_noticias r ON r.traduccion_id = t.id
		WHERE t.lang_to = 'es'
		  AND t.status = 'done'
		GROUP BY t.id
		HAVING COUNT(r.related_traduccion_id) = 0
		ORDER BY t.id DESC
		LIMIT $2
	`, model, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	normA = sqrt(normA)
	normB = sqrt(normB)

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (normA * normB)
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	// Simple Newton-Raphson
	z := x
	for i := 0; i < 20; i++ {
		z = (z + x/z) / 2
	}
	return z
}

func findTopK(query Embedding, candidates []Translation, k int, minScore float64) []struct {
	ID    int64
	Score float64
} {
	type sim struct {
		id    int64
		score float64
	}

	var similarities []sim

	for _, c := range candidates {
		if int64(c.ID) == query.ID {
			continue
		}

		score := cosineSimilarity(query.Embedding, c.Embedding)
		if score <= minScore {
			continue
		}

		similarities = append(similarities, sim{int64(c.ID), score})
	}

	// Sort by score descending
	for i := 0; i < len(similarities)-1; i++ {
		for j := i + 1; j < len(similarities); j++ {
			if similarities[j].score > similarities[i].score {
				similarities[i], similarities[j] = similarities[j], similarities[i]
			}
		}
	}

	if len(similarities) > k {
		similarities = similarities[:k]
	}

	result := make([]struct {
		ID    int64
		Score float64
	}, len(similarities))
	for i, s := range similarities {
		result[i] = struct {
			ID    int64
			Score float64
		}{s.id, s.score}
	}

	return result
}

type Embedding struct {
	ID        int64
	Embedding []float64
}

func findEmbeddingByID(embeddings []Embedding, id int64) *Embedding {
	for i := range embeddings {
		if embeddings[i].ID == id {
			return &embeddings[i]
		}
	}
	return nil
}

func insertRelated(ctx context.Context, traduccionID int64, related []struct {
	ID    int64
	Score float64
}) error {
	if len(related) == 0 {
		return nil
	}

	for _, r := range related {
		if r.Score <= 0 {
			continue
		}
		_, err := dbPool.Exec(ctx, `
			INSERT INTO related_noticias (traduccion_id, related_traduccion_id, score)
			VALUES ($1, $2, $3)
			ON CONFLICT (traduccion_id, related_traduccion_id) 
			DO UPDATE SET score = EXCLUDED.score
		`, traduccionID, r.ID, r.Score)
		if err != nil {
			logger.Printf("Error inserting related: %v", err)
		}
	}
	return nil
}

func processBatch(ctx context.Context, model string) (int, error) {
	// Fetch all embeddings once
	allTranslations, err := fetchAllEmbeddings(ctx, model)
	if err != nil {
		return 0, err
	}

	if len(allTranslations) == 0 {
		return 0, nil
	}

	// Convert to Embedding format for easier lookup
	var allEmbeddings []Embedding
	for _, t := range allTranslations {
		if t.Embedding != nil {
			allEmbeddings = append(allEmbeddings, Embedding{ID: t.ID, Embedding: t.Embedding})
		}
	}

	// Get pending IDs
	pendingIDs, err := fetchPendingIDs(ctx, model, batchSz)
	if err != nil {
		return 0, err
	}

	if len(pendingIDs) == 0 {
		return 0, nil
	}

	processed := 0

	for _, tradID := range pendingIDs {
		emb := findEmbeddingByID(allEmbeddings, tradID)
		if emb == nil {
			continue
		}

		topRelated := findTopK(*emb, allTranslations, topK, minScore)

		if err := insertRelated(ctx, tradID, topRelated); err != nil {
			logger.Printf("Error inserting related for %d: %v", tradID, err)
			continue
		}

		processed++
	}

	return processed, nil
}

func main() {
	loadConfig()
	logger.Info().Msg("Starting Related News Worker")

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

	model := os.Getenv("EMB_MODEL")
	if model == "" {
		model = "mxbai-embed-large"
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

	logger.Info().Msgf("Config: sleep=%ds, topK=%d, batch=%d, model=%s", sleepSec, topK, batchSz, model)

	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("Context cancelled, stopping worker")
			return
		case <-time.After(time.Duration(sleepSec) * time.Second):
			count, err := processBatch(ctx, model)
			if err != nil {
				logger.Error().Err(err).Msg("Error processing batch")
				continue
			}

			if count > 0 {
				logger.Info().Msgf("Generated related news for %d translations", count)
			}
		}
	}
}
