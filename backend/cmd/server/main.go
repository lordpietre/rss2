package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/rss2/backend/internal/auth"
	"github.com/rss2/backend/internal/cache"
	"github.com/rss2/backend/internal/config"
	"github.com/rss2/backend/internal/db"
	"github.com/rss2/backend/internal/handlers"
	"github.com/rss2/backend/internal/logger"
	"github.com/rss2/backend/internal/middleware"
	"github.com/rss2/backend/internal/services"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	_ "github.com/rss2/backend/docs"
)

func initDB() {
	ctx := context.Background()
	log := logger.GetLogger()

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
		log.Warn().Err(err).Msg("Could not create entity_aliases table")
	} else {
		log.Info().Msg("Table entity_aliases ready")
	}

	// Añadir columna role a users si no existe
	_, err = db.GetPool().Exec(ctx, `
		ALTER TABLE users ADD COLUMN IF NOT EXISTS role VARCHAR(20) DEFAULT 'user'
	`)
	if err != nil {
		log.Warn().Err(err).Msg("Could not add role column")
	} else {
		log.Info().Msg("Column role ready")
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

	// Crear tabla de alertas si no existe
	_, err = db.GetPool().Exec(ctx, `
		CREATE TABLE IF NOT EXISTS alertas (
			id SERIAL PRIMARY KEY,
			valor VARCHAR(255) NOT NULL,
			tipo VARCHAR(32) NOT NULL,
			periodo DATE NOT NULL,
			hits INT NOT NULL,
			baseline DOUBLE PRECISION NOT NULL,
			ratio DOUBLE PRECISION NOT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'nueva',
			created_at TIMESTAMP DEFAULT NOW(),
			UNIQUE(valor, tipo, periodo)
		)
	`)
	if err != nil {
		log.Printf("Warning: Could not create alertas table: %v", err)
	} else {
		log.Println("Table alertas ready")
	}

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

	// Crear índices para optimizar consultas críticas
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_noticias_fecha ON noticias(fecha DESC)",
		"CREATE INDEX IF NOT EXISTS idx_noticias_fuente_nombre ON noticias(fuente_nombre)",
		"CREATE INDEX IF NOT EXISTS idx_noticias_categoria_id ON noticias(categoria_id)",
		"CREATE INDEX IF NOT EXISTS idx_noticias_pais_id ON noticias(pais_id)",
		"CREATE INDEX IF NOT EXISTS idx_noticias_lang ON noticias(lang)",
		"CREATE INDEX IF NOT EXISTS idx_traducciones_noticia_id_lang_to ON traducciones(noticia_id, lang_to)",
		"CREATE INDEX IF NOT EXISTS idx_traducciones_status ON traducciones(status)",
		"CREATE INDEX IF NOT EXISTS idx_tags_noticia_noticia_id_tag_id ON tags_noticia(noticia_id, tag_id)",
		"CREATE INDEX IF NOT EXISTS idx_tags_noticia_traduccion_id ON tags_noticia(traduccion_id)",
		"CREATE INDEX IF NOT EXISTS idx_tags_valor_tipo ON tags(valor, tipo)",
		"CREATE INDEX IF NOT EXISTS idx_entity_aliases_alias_tipo ON entity_aliases(alias, tipo)",
		"CREATE INDEX IF NOT EXISTS idx_alertas_valor_tipo_periodo ON alertas(valor, tipo, periodo)",
		"CREATE INDEX IF NOT EXISTS idx_alertas_status ON alertas(status)",
		"CREATE INDEX IF NOT EXISTS idx_user_search_tags_user_id ON user_search_tags(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_feeds_activo ON feeds(activo)",
		"CREATE INDEX IF NOT EXISTS idx_feeds_categoria_id ON feeds(categoria_id)",
		"CREATE INDEX IF NOT EXISTS idx_feeds_pais_id ON feeds(pais_id)",
	}
	for _, idx := range indexes {
		_, err := db.GetPool().Exec(ctx, idx)
		if err != nil {
			log.Warn().Err(err).Msg("Could not create index")
		} else {
			log.Info().Msg("Index created/verified")
		}
	}
}

func main() {
	cfg := config.Load()

	// Initialize structured logger
	logLevel := os.Getenv("LOG_LEVEL")
	logger.Init("rss2-api", logLevel)
	log := logger.GetLogger()

	if err := db.Connect(cfg.DatabaseURL); err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()
	log.Info().Msg("Connected to PostgreSQL")

	// Auto-setup DB tables
	initDB()

	if err := cache.Connect(cfg.RedisURL); err != nil {
		log.Warn().Err(err).Msg("Failed to connect to Redis")
	} else {
		defer cache.Close()
		log.Info().Msg("Connected to Redis")
	}

	services.Init(cfg)

	r := gin.Default()

	r.Use(middleware.RequestIDMiddleware())
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.LoggerMiddleware())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Swagger documentation
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := r.Group("/api")
	{
		// Serve static images downloaded by wiki_worker
		api.StaticFS("/wiki-images", gin.Dir(cfg.WikiImagesPath, false))

		// Stricter rate limiting for auth endpoints
		authGroup := api.Group("/auth")
		authGroup.Use(middleware.RateLimitMiddleware(10)) // 10 req/min for auth
		{
			authGroup.POST("/login", handlers.Login)
			authGroup.POST("/register", handlers.Register)
			authGroup.GET("/check-first-user", handlers.CheckFirstUser)
		}

		// General rate limiting for API
		api.Use(middleware.RateLimitMiddleware(cfg.RateLimitPerMinute))

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
		api.GET("/search/suggestions", middleware.AuthRequired(), handlers.SearchSuggestions)
		api.POST("/searchlog", middleware.AuthRequired(), handlers.LogSearch)

		api.GET("/entities", handlers.GetEntities)
		api.GET("/entities/news", handlers.GetEntityNews)
		api.GET("/entities/mentions", handlers.GetEntityMentions)

		api.GET("/alerts", handlers.GetAlertas)
		api.POST("/alerts/:id/read", middleware.AuthRequired(), handlers.MarkAlertaRead)
		api.POST("/alerts/read-all", middleware.AuthRequired(), handlers.MarkAllAlertasRead)

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
			admin.POST("/alerts/scan", handlers.ScanAlertasAdmin)
			admin.GET("/workers/status", handlers.GetWorkerStatus)
			admin.GET("/workers/stats", handlers.GetTranslationStats)
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
		log.Info().Str("addr", addr).Msg("Server starting")
		if err := r.Run(addr); err != nil {
			log.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	handlers.StartJobAssigner()
	handlers.StartAlertScanner()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down server...")
}
