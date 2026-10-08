package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rss2/backend/internal/cache"
	"github.com/rss2/backend/internal/logger"
	"github.com/rss2/backend/internal/workers"
)

var (
	pool          *pgxpool.Pool
	sleepInterval = 30
	batchSize     = 40
	imagesDir     = "/app/data/wiki_images"
	maxImageBytes = int64(2 << 20)
)

type WikiSummary struct {
	Type        string `json:"type"`
	Title       string `json:"title"`
	DisplayTitle string `json:"displaytitle"`
	Extract     string `json:"extract"`
	ContentUrls struct {
		Desktop struct {
			Page string `json:"page"`
		} `json:"desktop"`
	} `json:"content_urls"`
	Thumbnail *struct {
		Source string `json:"source"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	} `json:"thumbnail"`
}

type Tag struct {
	ID    int64
	Valor string
	Tipo  string
}

func init() {
	logLevel := os.Getenv("LOG_LEVEL")
	logger.Init("wiki-worker", logLevel)
}

func getPendingTags(ctx context.Context) ([]Tag, error) {
	rows, err := pool.Query(ctx, `
		SELECT t.id, t.valor, t.tipo 
		FROM tags t
		LEFT JOIN (
			SELECT tag_id, COUNT(*) as cnt 
			FROM tags_noticia 
			GROUP BY tag_id
		) c ON c.tag_id = t.id
		WHERE t.tipo IN ('persona', 'organizacion') 
		  AND t.wiki_checked = FALSE 
		ORDER BY COALESCE(c.cnt, 0) DESC, t.id DESC
		LIMIT $1
	`, batchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Valor, &t.Tipo); err == nil {
			tags = append(tags, t)
		}
	}
	return tags, nil
}

func downloadImage(imgURL, destPath string) error {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", imgURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "RSS2-WikiWorker/1.0 (https://github.com/proyecto/rss2)")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	if resp.ContentLength > maxImageBytes {
		return fmt.Errorf("image too large (%d bytes)", resp.ContentLength)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, io.LimitReader(resp.Body, maxImageBytes))
	return err
}

func fetchWikipediaInfo(ctx context.Context, valor string) (*WikiSummary, error) {
	// Normalize the value to be wiki-compatible
	title := strings.ReplaceAll(strings.TrimSpace(valor), " ", "_")
	encodedTitle := url.PathEscape(title)

	// Check cache first
	cacheKey := fmt.Sprintf("wiki:summary:%s", encodedTitle)
	if cache.Client != nil {
		if cached, err := cache.Get(ctx, cacheKey); err == nil && cached != "" {
			var summary WikiSummary
			if err := json.Unmarshal([]byte(cached), &summary); err == nil {
				logger.Debug().Str("valor", valor).Msg("Cache hit for Wikipedia")
				return &summary, nil
			}
		}
	}

	apiURL := fmt.Sprintf("https://es.wikipedia.org/api/rest_v1/page/summary/%s", encodedTitle)

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	// Per MediaWiki API policy: https://meta.wikimedia.org/wiki/User-Agent_policy
	req.Header.Set("User-Agent", "RSS2-WikiWorker/1.0 (https://github.com/proyecto/rss2)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("HTTP 429: Too Many Requests (Rate Limited)")
	}
	if resp.StatusCode == 404 {
		return nil, nil // Not found, but handled successfully without error
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var summary WikiSummary
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		return nil, err
	}

	// Filter out disambiguation pages
	if summary.Type == "disambiguation" {
		return nil, nil // Treat as not found to strictly avoid incorrect tooltips
	}

	// Store in cache for 48 hours (Wikipedia data is relatively stable)
	if cache.Client != nil {
		if data, err := json.Marshal(summary); err == nil {
			cache.Set(ctx, cacheKey, string(data), 48*time.Hour)
			logger.Debug().Str("valor", valor).Msg("Cached Wikipedia result")
		}
	}

	return &summary, nil
}

func processTag(ctx context.Context, tag Tag) {
	logger.Info().Int64("tag_id", tag.ID).Str("valor", tag.Valor).Msg("Procesando tag")

	summary, err := fetchWikipediaInfo(ctx, tag.Valor)
	if err != nil {
		// No marcar como revisado ante errores transitorios (timeouts, 503, 504)
		// para que se reintenten en el siguiente ciclo
		errStr := err.Error()
		isTransient := strings.Contains(errStr, "context deadline exceeded") ||
			strings.Contains(errStr, "503") ||
			strings.Contains(errStr, "504") ||
			strings.Contains(errStr, "429")
		
		if isTransient {
			logger.Warn().Err(err).Str("valor", tag.Valor).Msg("Error transitorio al consultar Wikipedia, se reintentará")
		} else {
			// Marcar como revisado ante errores permanentes para no reintentar
			_, _ = pool.Exec(ctx, "UPDATE tags SET wiki_checked = TRUE WHERE id = $1", tag.ID)
			logger.Error().Err(err).Str("valor", tag.Valor).Msg("Error permanente al consultar Wikipedia (marcado como revisado)")
		}
		return
	}

	if summary == nil || summary.Extract == "" {
		// Not found or disambiguation
		_, _ = pool.Exec(ctx, "UPDATE tags SET wiki_checked = TRUE WHERE id = $1", tag.ID)
		logger.Warn().Str("valor", tag.Valor).Msg("No se encontraron resultados válidos en Wikipedia")
		return
	}

	var localImagePath *string
	if summary.Thumbnail != nil && summary.Thumbnail.Source != "" {
		ext := ".jpg"
		if strings.HasSuffix(strings.ToLower(summary.Thumbnail.Source), ".png") {
			ext = ".png"
		}
		fileName := fmt.Sprintf("wiki_%d%s", tag.ID, ext)
		destPath := filepath.Join(imagesDir, fileName)

		if err := downloadImage(summary.Thumbnail.Source, destPath); err != nil {
			logger.Error().Err(err).Str("valor", tag.Valor).Msg("Error descargando imagen")
			// Guardaremos la URL externa como fallback si falla la descarga
			src := summary.Thumbnail.Source
			localImagePath = &src
		} else {
			relativePath := "/api/wiki-images/" + fileName
			localImagePath = &relativePath
		}
	}

	wikiURL := summary.ContentUrls.Desktop.Page

	_, err = pool.Exec(ctx, `
		UPDATE tags 
		SET wiki_summary = $1, 
			wiki_url = $2, 
			image_path = $3, 
			wiki_checked = TRUE 
		WHERE id = $4
	`, summary.Extract, wikiURL, localImagePath, tag.ID)

	if err != nil {
		logger.Error().Err(err).Int64("tag_id", tag.ID).Msg("Error al actualizar la base de datos")
	} else {
		logger.Info().Str("valor", tag.Valor).Bool("imagen", localImagePath != nil).Msg("Actualizado con éxito")
	}
}

func Main() {
	var sleepVal, batchVal int
	if val := os.Getenv("WIKI_SLEEP"); val != "" {
		if _, err := fmt.Sscanf(val, "%d", &sleepVal); err == nil && sleepVal > 0 {
			sleepInterval = sleepVal
		}
	}
	if val := os.Getenv("WIKI_BATCH"); val != "" {
		if _, err := fmt.Sscanf(val, "%d", &batchVal); err == nil && batchVal > 0 {
			batchSize = batchVal
		}
	}

	logger.Info().Msg("Iniciando Wiki Worker...")

	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		logger.Fatal().Err(err).Msg("Error creando directorio de imágenes")
	}

	cfg := workers.LoadDBConfig()
	if err := workers.Connect(cfg); err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to database")
	}
	pool = workers.GetPool()
	defer workers.Close()

	// Connect to Redis for caching
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "redis"
	}
	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}
	redisPass := os.Getenv("REDIS_PASSWORD")
	var redisURL string
	if redisPass != "" {
		redisURL = fmt.Sprintf("redis://:%s@%s:%s", redisPass, redisHost, redisPort)
	} else {
		redisURL = fmt.Sprintf("redis://%s:%s", redisHost, redisPort)
	}
	if err := cache.Connect(redisURL); err != nil {
		logger.Warn().Err(err).Msg("Failed to connect to Redis, caching disabled")
	} else {
		logger.Info().Msg("Connected to Redis for caching")
		defer cache.Close()
	}

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
		// Give time for in-flight requests to complete
		time.Sleep(2 * time.Second)
		workers.Close()
		os.Exit(0)
	}()

	logger.Info().Msgf("Configuración: sleep=%ds, batch=%d", sleepInterval, batchSize)

	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("Context cancelled, stopping worker")
			return
		default:
			tags, err := getPendingTags(ctx)
			if err != nil {
				logger.Error().Err(err).Msg("Error recuperando tags pendientes")
				select {
				case <-ctx.Done():
					return
				case <-time.After(10 * time.Second):
				}
				continue
			}

			if len(tags) == 0 {
				logger.Info().Msgf("No hay tags pendientes. Durmiendo %d segundos...", sleepInterval)
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Duration(sleepInterval) * time.Second):
				}
				continue
			}

			logger.Info().Msgf("Recuperados %d tags para procesar...", len(tags))

			baseDelay := 10  // segundos base entre requests (más conservador para Wikipedia)
			maxDelay := 60   // máximo delay cuando hay errores
			currentDelay := baseDelay
		 consecutiveErrors := 0

			for _, tag := range tags {
				processTag(ctx, tag)
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Duration(currentDelay) * time.Second):
				}
				
				// Backoff exponencial: si hay errores recientes, incrementar delay
				// Resetear gradualmente cuando hay éxito
				if consecutiveErrors > 0 {
					consecutiveErrors--
					if currentDelay < maxDelay {
						currentDelay = currentDelay * 2
						if currentDelay > maxDelay {
							currentDelay = maxDelay
						}
					}
				} else if currentDelay > baseDelay {
					// Reducir gradualmente hacia el base
					currentDelay = currentDelay / 2
					if currentDelay < baseDelay {
						currentDelay = baseDelay
					}
				}
			}
		}
	}
}

func main() {
	Main()
}
