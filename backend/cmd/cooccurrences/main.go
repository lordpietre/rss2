package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var db *sqlx.DB

func main() {
	// Parse flags
	interval := flag.Duration("interval", 24*time.Hour, "Interval between refreshes")
	once := flag.Bool("once", false, "Run once and exit")
	flag.Parse()

	// Connect to database
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://rss:rss@localhost:5432/rss?sslmode=disable"
	}

	var err error
	db, err = sqlx.Connect("postgres", dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	log.Printf("Cooccurrences worker started")
	log.Printf("Interval: %v", *interval)

	if *once {
		log.Println("Running once mode...")
		if err := refreshCooccurrences(); err != nil {
			log.Fatalf("Refresh failed: %v", err)
		}
		return
	}

	// Run initial refresh
	if err := refreshCooccurrences(); err != nil {
		log.Printf("Initial refresh failed: %v", err)
	}

	// Main loop
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Println("Running scheduled refresh...")
			if err := refreshCooccurrences(); err != nil {
				log.Printf("Scheduled refresh failed: %v", err)
			}
		}
	}
}

func refreshCooccurrences() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	log.Println("Starting cooccurrences refresh...")

	// Clear existing data
	_, err := db.ExecContext(ctx, "TRUNCATE entity_cooccurrences")
	if err != nil {
		return err
	}

	// Insert all co-occurrences from recent news (last 30 days)
	query := `
		INSERT INTO entity_cooccurrences (tag1_id, tag2_id, co_count)
		SELECT 
			LEAST(tn1.tag_id, tn2.tag_id) AS tag1_id,
			GREATEST(tn1.tag_id, tn2.tag_id) AS tag2_id,
			COUNT(DISTINCT tn1.noticia_id) AS co_count
		FROM tags_noticia tn1
		JOIN tags_noticia tn2 ON tn1.noticia_id = tn2.noticia_id AND tn1.tag_id < tn2.tag_id
		JOIN noticias n ON tn1.noticia_id = n.id
		WHERE n.fecha >= NOW() - INTERVAL '30 days'
		GROUP BY LEAST(tn1.tag_id, tn2.tag_id), GREATEST(tn1.tag_id, tn2.tag_id)
		HAVING COUNT(DISTINCT tn1.noticia_id) >= 3
	`

	result, err := db.ExecContext(ctx, query)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()

	// Update timestamp
	_, err = db.ExecContext(ctx, "UPDATE entity_cooccurrences SET last_updated = NOW()")
	if err != nil {
		log.Printf("Warning: failed to update timestamps: %v", err)
	}

	log.Printf("Cooccurrences refresh completed: %d records inserted", rowsAffected)
	return nil
}
