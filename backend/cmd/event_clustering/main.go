package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rss2/backend/internal/logger"
	"github.com/rss2/backend/internal/workers"
)

var (
	dbPool       *pgxpool.Pool
	sleepSec     = 900  // 15 minutes
	batchSize    = 500  // News to process per batch
	minSimilitud = 0.85 // Cosine similarity threshold
	timeWindowH  = 72   // Hours window for event clustering
	entityTypes           = []string{"persona", "organizacion"}
)

// Configuración del modelo de embeddings (debe coincidir con el worker de embeddings)
const (
	embModel = "sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2"
	embDim   = 384
)

func init() {
	logLevel := os.Getenv("LOG_LEVEL")
	logger.Init("event-clustering", logLevel)
}

func loadConfig() {
	sleepSec = getEnvInt("CLUSTERING_SLEEP", 900)
	batchSize = getEnvInt("CLUSTERING_BATCH", 500)
	minSimilitud = getEnvFloat("CLUSTERING_MIN_SIM", 0.85)
	timeWindowH = getEnvInt("CLUSTERING_TIME_WINDOW", 72)
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

// ============================================================================
// Schema Setup
// ============================================================================

func ensureSchema(ctx context.Context) error {
	// Crear tablas si no existen
	queries := []string{
		`CREATE TABLE IF NOT EXISTS event_clusters (
			id SERIAL PRIMARY KEY,
			cluster_hash VARCHAR(64) NOT NULL,
			titulo_template TEXT,
			primer_noticia_id VARCHAR(32),
			fecha_inicio TIMESTAMP,
			fecha_fin TIMESTAMP,
			popularidad INTEGER DEFAULT 0,
			importancia VARCHAR(20) DEFAULT 'normal' CHECK (importancia IN ('breaking', 'major', 'normal', 'minor')),
			embedding_vector vector(384),
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS noticia_clusters (
			noticia_id VARCHAR(32) NOT NULL REFERENCES noticias(id) ON DELETE CASCADE,
			cluster_id INTEGER NOT NULL REFERENCES event_clusters(id) ON DELETE CASCADE,
			similitud DOUBLE PRECISION NOT NULL,
			es_principal BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP DEFAULT NOW(),
			PRIMARY KEY (noticia_id, cluster_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_event_clusters_hash ON event_clusters(cluster_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_event_clusters_fecha_inicio ON event_clusters(fecha_inicio DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_event_clusters_importancia ON event_clusters(importancia)`,
		`CREATE INDEX IF NOT EXISTS idx_noticia_clusters_noticia_id ON noticia_clusters(noticia_id)`,
		`CREATE INDEX IF NOT EXISTS idx_noticia_clusters_cluster_id ON noticia_clusters(cluster_id)`,
	}

	for _, q := range queries {
		if _, err := dbPool.Exec(ctx, q); err != nil {
			return fmt.Errorf("schema setup failed: %w", err)
		}
	}

	// Crear índice HNSW si no existe
	_, err := dbPool.Exec(ctx, `
		CREATE INDEX IF NOT EXISTS idx_event_clusters_vector 
		ON event_clusters USING hnsw (embedding_vector vector_cosine_ops)
		WITH (m = 16, ef_construction = 64)
	`)
	if err != nil {
		logger.Warn().Err(err).Msg("Could not create HNSW index (pgvector may not be installed)")
	}

	return nil
}

// ============================================================================
// Data Structures
// ============================================================================

type newsWithEmbedding struct {
	ID            string
	TraduccionID  int64
	Titulo        string
	Fecha         time.Time
	Embedding     []float64
	Entities      []string // Entity values for this news
}

type candidatePair struct {
	News1, News2     *newsWithEmbedding
	Similarity       float64
	SharedEntities   int
}

// ============================================================================
// Entity Extraction
// ============================================================================

func getNewsEntities(ctx context.Context, noticiaID string) ([]string, error) {
	rows, err := dbPool.Query(ctx, `
		SELECT DISTINCT LOWER(t.valor)
		FROM tags_noticia tn
		JOIN tags t ON tn.tag_id = t.id
		JOIN traducciones tr ON tn.traduccion_id = tr.id
		WHERE tr.noticia_id = $1 AND t.tipo = ANY($2)
	`, noticiaID, entityTypes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []string
	for rows.Next() {
		var val string
		if err := rows.Scan(&val); err != nil {
			continue
		}
		entities = append(entities, val)
	}
	return entities, nil
}

// ============================================================================
// Fetch Pending News
// ============================================================================

func fetchPendingNews(ctx context.Context, limit int) ([]newsWithEmbedding, error) {
	rows, err := dbPool.Query(ctx, `
		SELECT n.id, t.id, COALESCE(n.titulo, ''), n.fecha, e.embedding
		FROM noticias n
		JOIN traducciones t ON t.noticia_id = n.id AND t.lang_to = 'es' AND t.status = 'done'
		JOIN traduccion_embeddings e ON e.traduccion_id = t.id AND e.model = $1
		LEFT JOIN noticia_clusters nc ON nc.noticia_id = n.id
		WHERE e.embedding IS NOT NULL
		  AND e.embedding != '{}'
		  AND nc.noticia_id IS NULL
		  AND n.fecha >= NOW() - INTERVAL '7 days'
		GROUP BY n.id, t.id, n.titulo, n.fecha, e.embedding
		ORDER BY n.fecha DESC
		LIMIT $2
	`, embModel, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var newsList []newsWithEmbedding
	var skippedInvalid, skippedDim int
	for rows.Next() {
		var n newsWithEmbedding
		var embeddingStr sql.NullString
		if err := rows.Scan(&n.ID, &n.TraduccionID, &n.Titulo, &n.Fecha, &embeddingStr); err != nil {
			continue
		}
		if !embeddingStr.Valid {
			skippedInvalid++
			continue
		}
		n.Embedding = parseEmbedding(embeddingStr.String)
		if len(n.Embedding) != embDim {
			skippedDim++
			continue
		}
		newsList = append(newsList, n)
	}
	if skippedInvalid > 0 || skippedDim > 0 {
		logger.Warn().Int("skipped_invalid", skippedInvalid).Int("skipped_dim", skippedDim).Msg("Some news skipped due to invalid embeddings")
	}
	return newsList, nil
}

func parseEmbedding(s string) []float64 {
	// Parse PostgreSQL array format: {1.0,2.0,3.0,...}
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]float64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if f, err := strconv.ParseFloat(p, 64); err == nil {
			result = append(result, f)
		}
	}
	return result
}

// ============================================================================
// Similarity Calculation
// ============================================================================

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dotProd, normA, normB float64
	for i := range a {
		dotProd += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dotProd / (normA * normB)
}

// ============================================================================
// Clustering Algorithm (Incremental)
// ============================================================================

// findSimilarNews finds news similar to the given news using pgvector
func findSimilarNews(ctx context.Context, news *newsWithEmbedding, limit int) ([]struct {
	ID        string
	Score     float64
}, error) {
	vecStr := embeddingToVector(news.Embedding)

	rows, err := dbPool.Query(ctx, `
		SELECT n.id, 1 - (e.embedding_vector <=> $1::vector) AS score
		FROM traduccion_embeddings e
		JOIN traducciones t ON e.traduccion_id = t.id
		JOIN noticias n ON t.noticia_id = n.id
		WHERE e.embedding_vector IS NOT NULL
		  AND e.model = $2
		  AND n.id != $3
		  AND n.fecha >= $4
		  AND (1 - (e.embedding_vector <=> $1::vector)) >= $5
		ORDER BY e.embedding_vector <=> $1::vector
		LIMIT $6
	`, vecStr, embModel, news.ID, news.Fecha.Add(-time.Duration(timeWindowH)*time.Hour), minSimilitud, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []struct {
		ID    string
		Score float64
	}
	for rows.Next() {
		var r struct {
			ID    string
			Score float64
		}
		if err := rows.Scan(&r.ID, &r.Score); err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, nil
}

func embeddingToVector(emb []float64) string {
	if len(emb) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range emb {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(fmt.Sprintf("%g", v))
	}
	sb.WriteByte(']')
	return sb.String()
}

// ============================================================================
// Cluster Management
// ============================================================================

func calculateEntityHash(entities []string) string {
	if len(entities) == 0 {
		return ""
	}
	// Sort entities for consistent hash
	sorted := make([]string, len(entities))
	copy(sorted, entities)
	sort.Strings(sorted)
	
	combined := strings.Join(sorted, ",")
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:])
}

func getClusterHash(entities []string, fecha time.Time) string {
	entityHash := calculateEntityHash(entities)
	dateHash := fecha.Format("2006-01-02")
	// Use first 16 chars of hash, pad if needed
	if len(entityHash) < 16 {
		entityHash = entityHash + strings.Repeat("0", 16-len(entityHash))
	}
	return fmt.Sprintf("%s_%s", entityHash[:16], dateHash)
}

func findExistingCluster(ctx context.Context, clusterHash string) (int64, bool, error) {
	var id int64
	err := dbPool.QueryRow(ctx, `
		SELECT id FROM event_clusters 
		WHERE cluster_hash = $1 
		ORDER BY created_at DESC 
		LIMIT 1
	`, clusterHash).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

func createCluster(ctx context.Context, hash string, title string, fecha time.Time, embedding []float64) (int64, error) {
	var id int64
	vecStr := embeddingToVector(embedding)
	
	err := dbPool.QueryRow(ctx, `
		INSERT INTO event_clusters (cluster_hash, titulo_template, primer_noticia_id, fecha_inicio, fecha_fin, embedding_vector, importancia)
		VALUES ($1, $2, '', $3, $3, $4::vector, 'minor')
		RETURNING id
	`, hash, title, fecha, vecStr).Scan(&id)
	return id, err
}

func addToCluster(ctx context.Context, noticiaID string, clusterID int64, similarity float64, isPrincipal bool) error {
	_, err := dbPool.Exec(ctx, `
		INSERT INTO noticia_clusters (noticia_id, cluster_id, similitud, es_principal)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (noticia_id, cluster_id) DO UPDATE SET similitud = EXCLUDED.similitud
	`, noticiaID, clusterID, similarity, isPrincipal)
	return err
}

func updateClusterStats(ctx context.Context, clusterID int64) error {
	// Get the principal noticia (earliest with highest similarity)
	var principalID string
	err := dbPool.QueryRow(ctx, `
		SELECT n.id
		FROM noticia_clusters nc
		JOIN noticias n ON nc.noticia_id = n.id
		WHERE nc.cluster_id = $1
		ORDER BY nc.es_principal DESC, n.fecha ASC, nc.similitud DESC
		LIMIT 1
	`, clusterID).Scan(&principalID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	// Update dates and principal
	_, err = dbPool.Exec(ctx, `
		UPDATE event_clusters
		SET primer_noticia_id = COALESCE(NULLIF($2, ''), primer_noticia_id),
			fecha_inicio = (
				SELECT MIN(n.fecha) FROM noticia_clusters nc
				JOIN noticias n ON nc.noticia_id = n.id
				WHERE nc.cluster_id = $1
			),
			fecha_fin = (
				SELECT MAX(n.fecha) FROM noticia_clusters nc
				JOIN noticias n ON nc.noticia_id = n.id
				WHERE nc.cluster_id = $1
			)
		WHERE id = $1
	`, clusterID, principalID)
	if err != nil {
		return err
	}

	// Update importancia based on news count
	_, err = dbPool.Exec(ctx, `
		UPDATE event_clusters ec
		SET importancia = CASE
			WITH cnt AS (SELECT COUNT(*) as c FROM noticia_clusters WHERE cluster_id = $1)
			SELECT CASE 
				WHEN c >= 10 THEN 'breaking'
				WHEN c >= 5 THEN 'major'
				WHEN c >= 2 THEN 'normal'
				ELSE 'minor'
			END
		END,
		popularidad = (SELECT COUNT(*) FROM noticia_clusters WHERE cluster_id = $1)
		WHERE id = $1
	`, clusterID)
	return err
}

// ============================================================================
// Main Processing
// ============================================================================

func processBatch(ctx context.Context) (int, error) {
	// Fetch news without clusters
	newsList, err := fetchPendingNews(ctx, batchSize)
	if err != nil {
		return 0, fmt.Errorf("fetch pending news: %w", err)
	}

	if len(newsList) == 0 {
		return 0, nil
	}

	logger.Info().Msgf("Processing %d news for clustering", len(newsList))

	processed := 0

	for _, news := range newsList {
		select {
		case <-ctx.Done():
			return processed, ctx.Err()
		default:
		}

		// Get entities for this news
		entities, err := getNewsEntities(ctx, news.ID)
		if err != nil {
			logger.Warn().Err(err).Str("noticia_id", news.ID).Msg("Failed to get entities")
			entities = []string{}
		}

		// Calculate cluster hash
		clusterHash := getClusterHash(entities, news.Fecha)

		// Check if similar cluster exists
		clusterID, exists, err := findExistingCluster(ctx, clusterHash)
		if err != nil {
			logger.Warn().Err(err).Str("noticia_id", news.ID).Msg("Failed to find existing cluster")
			continue
		}

		if !exists {
			// Create new cluster
			title := news.Titulo
			if len(title) > 100 {
				title = title[:100] + "..."
			}
			clusterID, err = createCluster(ctx, clusterHash, title, news.Fecha, news.Embedding)
			if err != nil {
				logger.Warn().Err(err).Str("noticia_id", news.ID).Msg("Failed to create cluster")
				continue
			}
			logger.Info().Int64("cluster_id", clusterID).Str("hash", clusterHash).Msg("Created new cluster")
		}

		// Add to cluster as principal (if first) or regular member
		isPrincipal := !exists
		if err := addToCluster(ctx, news.ID, clusterID, 1.0, isPrincipal); err != nil {
			logger.Warn().Err(err).Str("noticia_id", news.ID).Int64("cluster_id", clusterID).Msg("Failed to add to cluster")
			continue
		}

		// Find similar news and add them too
		similarNews, err := findSimilarNews(ctx, &news, 10)
		if err != nil {
			logger.Warn().Err(err).Str("noticia_id", news.ID).Msg("Failed to find similar news")
		} else {
			for _, sim := range similarNews {
				// Check if already in a cluster
				var exists int
				err := dbPool.QueryRow(ctx, `
					SELECT 1 FROM noticia_clusters WHERE noticia_id = $1
				`, sim.ID).Scan(&exists)
				if err == sql.ErrNoRows {
					// Get entities for similar news
					simEntities, _ := getNewsEntities(ctx, sim.ID)
					simHash := getClusterHash(simEntities, news.Fecha)
					
					// Check if same cluster or create new one
					simClusterID, simExists, _ := findExistingCluster(ctx, simHash)
					if !simExists {
						// Could be same event, add to this cluster if shared entities
						if len(sharedEntities(entities, simEntities)) >= 2 && sim.Score >= minSimilitud {
							simClusterID = clusterID
						} else {
							continue
						}
					}
					
					// Add to cluster
					addToCluster(ctx, sim.ID, simClusterID, sim.Score, false)
				}
			}
		}

		// Update cluster stats
		if err := updateClusterStats(ctx, clusterID); err != nil {
			logger.Warn().Err(err).Int64("cluster_id", clusterID).Msg("Failed to update cluster stats")
		}

		processed++
	}

	return processed, nil
}

func sharedEntities(a, b []string) []string {
	bMap := make(map[string]bool)
	for _, v := range b {
		bMap[v] = true
	}
	var result []string
	for _, v := range a {
		if bMap[v] {
			result = append(result, v)
		}
	}
	return result
}

// ============================================================================
// Main Entry Point
// ============================================================================

func Main() {
	loadConfig()
	logger.Info().Msg("Starting Event Clustering Worker")

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

	logger.Info().Msgf("Config: sleep=%ds, batch=%d, minSim=%.2f, timeWindow=%dh",
		sleepSec, batchSize, minSimilitud, timeWindowH)

	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("Context cancelled, stopping worker")
			return
		case <-time.After(time.Duration(sleepSec) * time.Second):
			count, err := processBatch(ctx)
			if err != nil {
				logger.Error().Err(err).Msg("Error processing batch")
				continue
			}
			if count > 0 {
				logger.Info().Msgf("Clustered %d news articles", count)
			}
		}
	}
}

func main() {
	Main()
}
