package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/rss2/backend/internal/auth"
	"github.com/rss2/backend/internal/cache"
	"github.com/rss2/backend/internal/config"
	"github.com/rss2/backend/internal/db"
	"github.com/rss2/backend/internal/handlers"
	"github.com/rss2/backend/internal/middleware"
	"github.com/rss2/backend/internal/services"
)

func initDB() {
	ctx := context.Background()

	// Crear tabla entity_aliases si no existe
	_, err := db.GetPool().Exec(ctx, `
		CREATE TABLE IF NOT EXISTS entity_aliases (
			id SERIAL PRIMARY KEY,
			canonical_name VARCHAR(255) NOT NULL,
			alias VARCHAR(255) NOT NULL,
			tipo VARCHAR(50) NOT NULL CHECK (tipo IN ('persona', 'organizacion', 'lugar', 'tema')),
			created_at TIMESTAMP DEFAULT NOW(),
			UNIQUE(alias, tipo)
		)
	`)
	if err != nil {
		log.Printf("Warning: Could not create entity_aliases table: %v", err)
	} else {
		log.Println("Table entity_aliases ready")
	}

	// Añadir columna role a users si no existe
	_, err = db.GetPool().Exec(ctx, `
		ALTER TABLE users ADD COLUMN IF NOT EXISTS role VARCHAR(20) DEFAULT 'user'
	`)
	if err != nil {
		log.Printf("Warning: Could not add role column: %v", err)
	} else {
		log.Println("Column role ready")
	}

	// Crear tabla de configuración si no existe
	_, err = db.GetPool().Exec(ctx, `
		CREATE TABLE IF NOT EXISTS config (
			key VARCHAR(100) PRIMARY KEY,
			value TEXT,
			updated_at TIMESTAMP DEFAULT NOW()
		)
	`)
	if err != nil {
		log.Printf("Warning: Could not create config table: %v", err)
	} else {
		log.Println("Table config ready")
	}

	// Insertar configuración por defecto si no existe
	db.GetPool().Exec(ctx, `
		INSERT INTO config (key, value) VALUES ('translator_type', 'cpu')
		ON CONFLICT (key) DO NOTHING
	`)
	db.GetPool().Exec(ctx, `
		INSERT INTO config (key, value) VALUES ('translator_workers', '2')
		ON CONFLICT (key) DO NOTHING
	`)
	db.GetPool().Exec(ctx, `
		INSERT INTO config (key, value) VALUES ('translator_status', 'stopped')
		ON CONFLICT (key) DO NOTHING
	`)

	// Crear tabla de remote_workers si no existe
	_, err = db.GetPool().Exec(ctx, `
		CREATE TABLE IF NOT EXISTS remote_workers (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			api_key VARCHAR(64) UNIQUE NOT NULL,
			capabilities VARCHAR(50) DEFAULT 'cpu',
			status VARCHAR(20) DEFAULT 'offline',
			last_seen TIMESTAMP,
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	if err != nil {
		log.Printf("Warning: Could not create remote_workers table: %v", err)
	} else {
		log.Println("Table remote_workers ready")
	}

	// Añadir columnas a traducciones para workers remotos
	_, err = db.GetPool().Exec(ctx, `
		ALTER TABLE traducciones ADD COLUMN IF NOT EXISTS worker_id INTEGER REFERENCES remote_workers(id)
	`)
	if err != nil {
		log.Printf("Warning: Could not add worker_id column: %v", err)
	}

	_, err = db.GetPool().Exec(ctx, `
		ALTER TABLE traducciones ADD COLUMN IF NOT EXISTS assigned_at TIMESTAMP
	`)
	if err != nil {
		log.Printf("Warning: Could not add assigned_at column: %v", err)
	}
}

func main() {
	cfg := config.Load()

	if err := db.Connect(cfg.DatabaseURL); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("Connected to PostgreSQL")

	// Auto-setup DB tables
	initDB()

	if err := cache.Connect(cfg.RedisURL); err != nil {
		log.Printf("Warning: Failed to connect to Redis: %v", err)
	} else {
		defer cache.Close()
		log.Println("Connected to Redis")
	}

	services.Init(cfg)

	r := gin.Default()

	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.LoggerMiddleware())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api := r.Group("/api")
	{
		// Serve static images downloaded by wiki_worker
		api.StaticFS("/wiki-images", gin.Dir(cfg.WikiImagesPath, false))

		api.POST("/auth/login", handlers.Login)
		api.POST("/auth/register", handlers.Register)
		api.GET("/auth/check-first-user", handlers.CheckFirstUser)

		news := api.Group("/news")
		{
			news.GET("", handlers.GetNews)
			news.GET("/:id", handlers.GetNewsByID)
			news.DELETE("/:id", middleware.AuthRequired(), handlers.DeleteNews)
		}

		feeds := api.Group("/feeds")
		{
			feeds.GET("", handlers.GetFeeds)
			feeds.GET("/export", handlers.ExportFeeds)
			feeds.GET("/:id", handlers.GetFeedByID)
			feeds.POST("", middleware.AuthRequired(), handlers.CreateFeed)
			feeds.POST("/import", middleware.AuthRequired(), handlers.ImportFeeds)
			feeds.PUT("/:id", middleware.AuthRequired(), handlers.UpdateFeed)
			feeds.DELETE("/:id", middleware.AuthRequired(), handlers.DeleteFeed)
			feeds.POST("/:id/toggle", middleware.AuthRequired(), handlers.ToggleFeedActive)
			feeds.POST("/:id/reactivate", middleware.AuthRequired(), handlers.ReactivateFeed)
		}

		api.GET("/search", handlers.SearchNews)

		api.GET("/entities", handlers.GetEntities)

		api.GET("/stats", handlers.GetStats)

		api.GET("/categories", handlers.GetCategories)
		api.GET("/countries", handlers.GetCountries)

		admin := api.Group("/admin")
		admin.Use(middleware.AuthRequired(), middleware.AdminRequired())
		{
			admin.POST("/aliases", handlers.CreateAlias)
			admin.GET("/aliases/export", handlers.ExportAliases)
			admin.POST("/aliases/import", handlers.ImportAliases)
			admin.POST("/entities/retype", handlers.PatchEntityTipo)
			admin.GET("/backup", handlers.BackupDatabase)
			admin.GET("/backup/news", handlers.BackupNewsZipped)
			admin.POST("/restore", handlers.RestoreDatabase)
			admin.GET("/users", handlers.GetUsers)
			admin.POST("/users/:id/promote", handlers.PromoteUser)
			admin.POST("/users/:id/demote", handlers.DemoteUser)
			admin.POST("/reset-db", handlers.ResetDatabase)
			admin.GET("/workers/status", handlers.GetWorkerStatus)
			admin.POST("/workers/config", handlers.SetWorkerConfig)
			admin.POST("/workers/start", handlers.StartWorkers)
			admin.POST("/workers/stop", handlers.StopWorkers)

			admin.GET("/workers/remote", handlers.ListRemoteWorkers)
			admin.POST("/workers/remote", handlers.CreateRemoteWorker)
			admin.GET("/workers/remote/:id", handlers.GetRemoteWorker)
			admin.DELETE("/workers/remote/:id", handlers.DeleteRemoteWorker)
			admin.POST("/workers/remote/:id/toggle", handlers.ToggleRemoteWorker)
			admin.POST("/workers/remote/:id/regenerate-key", handlers.RegenerateAPIKey)
		}

		r.GET("/ws/worker", handlers.HandleWorkerWS)

		auth := api.Group("/auth")
		auth.Use(middleware.AuthRequired())
		{
			auth.GET("/me", handlers.GetCurrentUser)
		}
	}

	auth.SetJWTSecret(cfg.SecretKey)

	port := cfg.ServerPort
	addr := fmt.Sprintf(":%s", port)

	go func() {
		log.Printf("Server starting on %s", addr)
		if err := r.Run(addr); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	handlers.StartJobAssigner()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
}
