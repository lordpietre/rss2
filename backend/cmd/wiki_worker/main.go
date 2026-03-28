package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rss2/backend/internal/workers"
)

var (
	logger        *log.Logger
	pool          *pgxpool.Pool
	sleepInterval = 30
	batchSize     = 50
	imagesDir     = "/app/data/wiki_images"
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
	logger = log.New(os.Stdout, "[WIKI_WORKER] ", log.LstdFlags)
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

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func fetchWikipediaInfo(valor string) (*WikiSummary, error) {
	// Normalize the value to be wiki-compatible
	title := strings.ReplaceAll(strings.TrimSpace(valor), " ", "_")
	encodedTitle := url.PathEscape(title)

	apiURL := fmt.Sprintf("https://es.wikipedia.org/api/rest_v1/page/summary/%s", encodedTitle)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	// Per MediaWiki API policy: https://meta.wikimedia.org/wiki/User-Agent_policy
	req.Header.Set("User-Agent", "RSS2-WikiWorker/1.0 (pietrelinux@gmail.com)")

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

	return &summary, nil
}

func processTag(ctx context.Context, tag Tag) {
	logger.Printf("Procesando tag %d: %s", tag.ID, tag.Valor)

	summary, err := fetchWikipediaInfo(tag.Valor)
	if err != nil {
		logger.Printf("Error al consultar Wikipedia para %s: %v", tag.Valor, err)
		return
	}

	if summary == nil || summary.Extract == "" {
		// Not found or disambiguation
		_, _ = pool.Exec(ctx, "UPDATE tags SET wiki_checked = TRUE WHERE id = $1", tag.ID)
		logger.Printf("No se encontraron resultados válidos en Wikipedia para: %s", tag.Valor)
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
			logger.Printf("Error descargando imagen para %s: %v", tag.Valor, err)
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
		logger.Printf("Error al actualizar la base de datos para tag %d: %v", tag.ID, err)
	} else {
		logger.Printf("Actualizado con éxito: %s (Imagen: %v)", tag.Valor, localImagePath != nil)
	}
}

func main() {
	if val := os.Getenv("WIKI_SLEEP"); val != "" {
		if sleep, err := fmt.Sscanf(val, "%d", &sleepInterval); err == nil && sleep > 0 {
			sleepInterval = sleep
		}
	}

	logger.Println("Iniciando Wiki Worker...")

	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		logger.Fatalf("Error creando directorio de imágenes: %v", err)
	}

	cfg := workers.LoadDBConfig()
	if err := workers.Connect(cfg); err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}
	pool = workers.GetPool()
	defer workers.Close()

	ctx := context.Background()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Println("Cerrando gracefully...")
		workers.Close()
		os.Exit(0)
	}()

	logger.Printf("Configuración: sleep=%ds, batch=%d", sleepInterval, batchSize)

	for {
		tags, err := getPendingTags(ctx)
		if err != nil {
			logger.Printf("Error recuperando tags pendientes: %v", err)
			time.Sleep(10 * time.Second)
			continue
		}

		if len(tags) == 0 {
			logger.Printf("No hay tags pendientes. Durmiendo %d segundos...", sleepInterval)
			time.Sleep(time.Duration(sleepInterval) * time.Second)
			continue
		}

		logger.Printf("Recuperados %d tags para procesar...", len(tags))

		for _, tag := range tags {
			processTag(ctx, tag)
			time.Sleep(3 * time.Second) // Increased delay to avoid Wikipedia Rate Limits (429)
		}
	}
}
