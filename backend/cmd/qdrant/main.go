package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rss2/backend/internal/workers"
)

var (
	logger     *log.Logger
	dbPool     *pgxpool.Pool
	qdrantURL  string
	collection = "news_vectors"
	sleepSec   = 30
	batchSize  = 100
)

func init() {
	logger = log.New(os.Stdout, "[QDRANT] ", log.LstdFlags)
}

func loadConfig() {
	sleepSec = getEnvInt("QDRANT_SLEEP", 30)
	batchSize = getEnvInt("QDRANT_BATCH", 100)
	qdrantHost := getEnv("QDRANT_HOST", "localhost")
	qdrantPort := getEnvInt("QDRANT_PORT", 6333)
	qdrantURL = fmt.Sprintf("http://%s:%d", qdrantHost, qdrantPort)
	collection = getEnv("QDRANT_COLLECTION", "news_vectors")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

type Translation struct {
	ID           int64
	NoticiaID    string
	Lang         string
	Titulo       string
	Resumen      string
	URL          string
	Fecha        *time.Time
	FuenteNombre string
	CategoriaID  *int64
	PaisID       *int64
	Embedding    []float64
}

func getPendingTranslations(ctx context.Context) ([]Translation, error) {
	rows, err := dbPool.Query(ctx, `
		SELECT 
			t.id as traduccion_id,
			t.noticia_id,
			TRIM(t.lang_to) as lang,
			t.titulo_trad as titulo,
			t.resumen_trad as resumen,
			n.url,
			n.fecha,
			n.fuente_nombre,
			n.categoria_id,
			n.pais_id,
			te.embedding::text
		FROM traducciones t
		INNER JOIN noticias n ON t.noticia_id = n.id
		INNER JOIN traduccion_embeddings te ON te.traduccion_id = t.id
		WHERE t.vectorized = FALSE
		AND t.status = 'done'
		ORDER BY t.created_at ASC
		LIMIT $1
	`, batchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var translations []Translation
	for rows.Next() {
		var t Translation
		var embText string
		if err := rows.Scan(
			&t.ID, &t.NoticiaID, &t.Lang, &t.Titulo, &t.Resumen,
			&t.URL, &t.Fecha, &t.FuenteNombre, &t.CategoriaID, &t.PaisID,
			&embText,
		); err != nil {
			logger.Printf("Error scanning translation row: %v", err)
			continue
		}
		emb, err := parseEmbedding(embText)
		if err != nil {
			logger.Printf("Error parsing embedding for translation %d: %v", t.ID, err)
			continue
		}
		t.Embedding = emb
		translations = append(translations, t)
	}
	return translations, nil
}

func parseEmbedding(s string) ([]float64, error) {
	s = strings.Trim(strings.TrimSpace(s), "{}")
	if s == "" {
		return nil, fmt.Errorf("empty embedding")
	}
	parts := strings.Split(s, ",")
	emb := make([]float64, len(parts))
	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return nil, err
		}
		emb[i] = v
	}
	return emb, nil
}

type QdrantPoint struct {
	ID      interface{}            `json:"id"`
	Vector  []float64              `json:"vector"`
	Payload map[string]interface{} `json:"payload"`
}

type QdrantUpsertRequest struct {
	Points []QdrantPoint `json:"points"`
}

func ensureCollection(ctx context.Context) error {
	req, err := http.NewRequest("GET", qdrantURL+"/collections/"+collection, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		logger.Printf("Collection %s already exists", collection)
		return nil
	}

	// Get embedding dimension from stored embeddings
	var dimension int
	err = dbPool.QueryRow(ctx, `SELECT dim FROM traduccion_embeddings LIMIT 1`).Scan(&dimension)
	if err != nil {
		return fmt.Errorf("failed to get embedding dimension: %w", err)
	}

	// Create collection
	createReq := map[string]interface{}{
		"vectors": map[string]interface{}{
			"size":     dimension,
			"distance": "Cosine",
		},
	}

	body, _ := json.Marshal(createReq)
	createURL := qdrantURL + "/collections/" + collection
	req2, err := http.NewRequest(http.MethodPut, createURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		return err
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp2.Body)
		return fmt.Errorf("Qdrant create collection returned %d: %s", resp2.StatusCode, string(respBody))
	}

	logger.Printf("Created collection %s with dimension %d", collection, dimension)
	return nil
}

func uploadToQdrant(translations []Translation, pointIDs []string) error {
	points := make([]QdrantPoint, 0, len(translations))

	for i, t := range translations {
		if t.Embedding == nil || len(t.Embedding) == 0 {
			continue
		}

		payload := map[string]interface{}{
			"news_id":       t.NoticiaID,
			"traduccion_id": t.ID,
			"titulo":        t.Titulo,
			"resumen":       t.Resumen,
			"url":           t.URL,
			"fuente_nombre": t.FuenteNombre,
			"lang":          t.Lang,
		}

		if t.Fecha != nil {
			payload["fecha"] = t.Fecha.Format(time.RFC3339)
		}
		if t.CategoriaID != nil {
			payload["categoria_id"] = *t.CategoriaID
		}
		if t.PaisID != nil {
			payload["pais_id"] = *t.PaisID
		}

		points = append(points, QdrantPoint{
			ID:      pointIDs[i],
			Vector:  t.Embedding,
			Payload: payload,
		})
	}

	if len(points) == 0 {
		return nil
	}

	reqBody := QdrantUpsertRequest{Points: points}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/collections/%s/points", qdrantURL, collection)
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 202 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Qdrant returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func updateTranslationStatus(ctx context.Context, translations []Translation, pointIDs []string) error {
	for i, t := range translations {
		if i >= len(pointIDs) || pointIDs[i] == "" {
			continue
		}

		_, err := dbPool.Exec(ctx, `
			UPDATE traducciones
			SET 
				vectorized = TRUE,
				vectorization_date = NOW(),
				qdrant_point_id = $1
			WHERE id = $2
		`, pointIDs[i], t.ID)

		if err != nil {
			logger.Printf("Error updating translation %d: %v", t.ID, err)
		}
	}

	return nil
}

func getStats(ctx context.Context) (total, vectorized, pending int, err error) {
	err = dbPool.QueryRow(ctx, `
		SELECT 
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE vectorized = TRUE) as vectorized,
			COUNT(*) FILTER (WHERE vectorized = FALSE AND status = 'done') as pending
		FROM traducciones
		WHERE lang_to = 'es' 
	`).Scan(&total, &vectorized, &pending)

	return total, vectorized, pending, err
}

func main() {
	loadConfig()
	logger.Println("Starting Qdrant Vectorization Worker")

	cfg := workers.LoadDBConfig()
	if err := workers.Connect(cfg); err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}
	dbPool = workers.GetPool()
	defer workers.Close()

	logger.Println("Connected to PostgreSQL")

	ctx := context.Background()

	if err := ensureCollection(ctx); err != nil {
		logger.Printf("Warning: Could not ensure collection: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Println("Shutting down...")
		os.Exit(0)
	}()

	logger.Printf("Config: qdrant=%s, collection=%s, sleep=%ds, batch=%d",
		qdrantURL, collection, sleepSec, batchSize)

	totalProcessed := 0

	for {
		select {
		case <-time.After(time.Duration(sleepSec) * time.Second):
			translations, err := getPendingTranslations(ctx)
			if err != nil {
				logger.Printf("Error fetching pending translations: %v", err)
				continue
			}

			if len(translations) == 0 {
				logger.Println("No pending translations to process")
				continue
			}

			logger.Printf("Processing %d translations...", len(translations))

			// Generate point IDs once (used both in Qdrant and DB)
			pointIDs := make([]string, len(translations))
			for i := range translations {
				pointIDs[i] = uuid.New().String()
			}

			// Upload to Qdrant
			if err := uploadToQdrant(translations, pointIDs); err != nil {
				logger.Printf("Error uploading to Qdrant: %v", err)
				continue
			}

			// Update DB status
			if err := updateTranslationStatus(ctx, translations, pointIDs); err != nil {
				logger.Printf("Error updating status: %v", err)
			}

			totalProcessed += len(translations)
			logger.Printf("Processed %d translations (total: %d)", len(translations), totalProcessed)

			total, vectorized, pending, err := getStats(ctx)
			if err == nil {
				logger.Printf("Stats: total=%d, vectorized=%d, pending=%d", total, vectorized, pending)
			}
		}
	}
}
