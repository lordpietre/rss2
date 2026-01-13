package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"html/template"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

var db *sql.DB

func initDB() {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_USER"), os.Getenv("DB_PASS"), os.Getenv("DB_NAME"))
	
	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err = db.Ping(); err != nil {
		log.Fatalf("Cannot connect to DB: %v", err)
	}
	log.Println("Connected to Database")
}

// Template Functions (to replace Jinja filters)
var funcMap = template.FuncMap{
	"safe_html": func(s string) template.HTML {
		return template.HTML(s)
	},
	"format_date": func(t time.Time) string {
		return t.Format("02/01/2006")
	},
	"country_flag": func(code string) string {
		// Placeholder logic, real logic needs country mapping
		return "🏳️"
	},
}

func main() {
	// Debug mode for now
	gin.SetMode(gin.DebugMode)

	initDB()

	r := gin.Default()

	// Load Templates with FuncMap
	// We need to support the template structure.
	// For now, let's try to load them directly, but likely we need to adapt syntax.
	// r.SetFuncMap(funcMap)
	// r.LoadHTMLGlob("templates/*") 

	// Static Files
	r.Static("/static", "./static")

	// Routes
	r.GET("/", homeHandler)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "engine": "golang"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8001"
	}

	log.Printf("Starting Server on port %s", port)
	if err := r.Run("0.0.0.0:" + port); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func homeHandler(c *gin.Context) {
	// Simple query for testing connectivity
	rows, err := db.Query("SELECT titulo, url FROM noticias ORDER BY fecha DESC LIMIT 10")
	if err != nil {
		c.String(500, "DB Error: %v", err)
		return
	}
	defer rows.Close()

	var news []map[string]string
	for rows.Next() {
		var titulo, url string
		if err := rows.Scan(&titulo, &url); err != nil {
			continue
		}
		news = append(news, map[string]string{"titulo": titulo, "url": url})
	}

	// For now, return JSON to prove it works before porting the complex HTML
	c.JSON(200, gin.H{
		"message": "Welcome to RSS2 Go Web Server",
		"news": news,
	})
}
