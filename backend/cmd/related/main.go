package main

import (
	"context"
	"fmt"
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
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return defaultValue
}

func ensureSchema(ctx context.Context) error {
	// Enable pgvector extension
	_, err := dbPool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS vector CASCADE;`)
	if err != nil {
		logger.Warn().Err(err).Msg("Could not create vector extension (may already exist)")
	}

	// Create related_noticias table
	_, err = dbPool.Exec(ctx, `
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

	// Ensure traduccion_embeddings table exists with pgvector column
	_, err = dbPool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS traduccion_embeddings (
			id SERIAL PRIMARY KEY,
			traduccion_id INTEGER NOT NULL REFERENCES traducciones(id) ON DELETE CASCADE,
			model TEXT NOT NULL,
			dim INTEGER NOT NULL,
			embedding DOUBLE PRECISION[] NOT NULL,
			embedding_vector vector(384),
			created_at TIMESTAMP DEFAULT NOW(),
			UNIQUE (traduccion_id, model)
		);
	`)
	if err != nil {
		return err
	}

	// Create standard indexes
	_, err = dbPool.Exec(ctx, `
		CREATE INDEX IF NOT EXISTS idx_tr_emb_model ON traduccion_embeddings(model);
	`)
	if err != nil {
		return err
	}

	_, err = dbPool.Exec(ctx, `
		CREATE INDEX IF NOT EXISTS idx_tr_emb_traduccion_id ON traduccion_embeddings(traduccion_id);
	`)
	if err != nil {
		return err
	}

	// Create HNSW index for pgvector search (best for cosine similarity)
	_, err = dbPool.Exec(ctx, `
		CREATE INDEX IF NOT EXISTS idx_tr_emb_vector_hnsw 
		ON traduccion_embeddings 
		USING hnsw (embedding_vector vector_cosine_ops)
		WITH (m = 16, ef_construction = 64);
	`)
	if err != nil {
		logger.Warn().Err(err).Msg("Could not create HNSW index (pgvector may not be installed)")
	}

	// Create the find_related_translations function for pgvector search
	_, err = dbPool.Exec(ctx, `
		CREATE OR REPLACE FUNCTION find_related_translations(
			query_embedding vector(384),
			match_count int DEFAULT 10,
			match_threshold float DEFAULT 0.0,
			exclude_id int DEFAULT NULL
		) RETURNS TABLE (
			traduccion_id int,
			score float
		) AS $$
		BEGIN
			RETURN QUERY
			SELECT 
				te.traduccion_id,
				1 - (te.embedding_vector <=> query_embedding) AS score
			FROM traduccion_embeddings te
			WHERE te.embedding_vector IS NOT NULL
			  AND te.embedding_vector != '0'::vector(384)
			  AND (1 - (te.embedding_vector <=> query_embedding)) > match_threshold
			  AND (exclude_id IS NULL OR te.traduccion_id != exclude_id)
			ORDER BY te.embedding_vector <=> query_embedding
			LIMIT match_count;
		END;
		$$ LANGUAGE plpgsql STABLE;
	`)
	if err != nil {
		logger.Warn().Err(err).Msg("Could not create find_related_translations function")
	}

	return nil
}

// fetchPendingIDs returns IDs that need related news generated
func fetchPendingIDs(ctx context.Context, model string, limit int) ([]int64, error) {
	rows, err := dbPool.Query(ctx, `
		SELECT t.id
		FROM traducciones t
		JOIN traduccion_embeddings e ON e.traduccion_id = t.id AND e.model = $1
		LEFT JOIN related_noticias r ON r.traduccion_id = t.id
		WHERE t.lang_to = 'es'
		  AND t.status = 'done'
		  AND e.embedding_vector IS NOT NULL
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

// getEmbedding retrieves the embedding vector for a specific translation
func getEmbedding(ctx context.Context, traduccionID int64) ([]float64, error) {
	var embedding []float64
	err := dbPool.QueryRow(ctx, `
		SELECT e.embedding
		FROM traduccion_embeddings e
		WHERE e.traduccion_id = $1
	`, traduccionID).Scan(&embedding)
	if err != nil {
		return nil, err
	}
	return embedding, nil
}

// embeddingToVector converts a float64 slice to pgvector format string
func embeddingToVector(emb []float64) string {
	if len(emb) == 0 {
		return "[]"
	}
	vec := make([]string, len(emb))
	for i, v := range emb {
		vec[i] = fmt.Sprintf("%g", v)
	}
	result := vec[0]
	for i := 1; i < len(vec); i++ {
		result += "," + vec[i]
	}
	return "[" + result + "]"
}

// findRelatedWithPgvector uses pgvector's find_related_translations function
func findRelatedWithPgvector(ctx context.Context, embedding []float64, excludeID int64, k int, minScore float64) ([]struct {
	ID    int64
	Score float64
}, error) {
	vecStr := embeddingToVector(embedding)

	rows, err := dbPool.Query(ctx, `
		SELECT traduccion_id, score
		FROM find_related_translations($1::vector, $2, $3, $4)
	`, vecStr, k, minScore, excludeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []struct {
		ID    int64
		Score float64
	}
	for rows.Next() {
		var r struct {
			ID    int64
			Score float64
		}
		if err := rows.Scan(&r.ID, &r.Score); err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, nil
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
			logger.Error().Err(err).Int64("traduccion_id", traduccionID).Int64("related_id", r.ID).Msg("Error inserting related")
		}
	}
	return nil
}

// processBatchWithPgvector uses pgvector for efficient vector search
func processBatchWithPgvector(ctx context.Context, model string) (int, error) {
	// Get pending IDs that have embeddings
	pendingIDs, err := fetchPendingIDs(ctx, model, batchSz)
	if err != nil {
		return 0, err
	}

	if len(pendingIDs) == 0 {
		return 0, nil
	}

	processed := 0

	for _, tradID := range pendingIDs {
		// Get the embedding for this translation
		embedding, err := getEmbedding(ctx, tradID)
		if err != nil {
			logger.Warn().Int64("traduccion_id", tradID).Err(err).Msg("Could not get embedding")
			continue
		}

		// Use pgvector to find similar translations
		related, err := findRelatedWithPgvector(ctx, embedding, tradID, topK, minScore)
		if err != nil {
			logger.Warn().Int64("traduccion_id", tradID).Err(err).Msg("Pgvector search failed")
			continue
		}

		if err := insertRelated(ctx, tradID, related); err != nil {
			logger.Error().Err(err).Int64("traduccion_id", tradID).Msg("Error inserting related")
			continue
		}

		processed++
	}

	return processed, nil
}

func Main() {
	loadConfig()
	logger.Info().Msg("Starting Related News Worker (pgvector mode)")

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

	// Ensure schema (creates tables, indexes, and pgvector function)
	if err := ensureSchema(ctx); err != nil {
		logger.Error().Err(err).Msg("Error ensuring schema")
	}

	model := os.Getenv("EMB_MODEL")
	if model == "" {
		model = "sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2"
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

	logger.Info().Msgf("Config: sleep=%ds, topK=%d, batch=%d, model=%s, minScore=%.2f", 
		sleepSec, topK, batchSz, model, minScore)

	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("Context cancelled, stopping worker")
			return
		case <-time.After(time.Duration(sleepSec) * time.Second):
			count, err := processBatchWithPgvector(ctx, model)
			if err != nil {
				logger.Error().Err(err).Msg("Error processing batch")
				continue
			}

			if count > 0 {
				logger.Info().Msgf("Generated related news for %d translations using pgvector", count)
			}
		}
	}
}

func main() {
	Main()
}
