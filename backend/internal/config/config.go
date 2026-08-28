package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	ServerPort         string
	DatabaseURL        string
	RedisURL           string
	QdrantHost         string
	QdrantPort         int
	SecretKey          string
	JWTExpiration      time.Duration
	TranslationURL     string
	OllamaURL          string
	SpacyURL           string
	DefaultLang        string
	NewsPerPage        int
	RateLimitPerMinute int
	DockerComposeDir   string
	WikiImagesPath     string
	AllowedOrigins     string
}

func Load() *Config {
	secretKey := os.Getenv("SECRET_KEY")
	if secretKey == "" {
		panic("SECRET_KEY environment variable is required")
	}

	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		allowedOrigins = "http://localhost:5173,http://localhost:3000"
	}

	return &Config{
		ServerPort:            getEnv("SERVER_PORT", "8080"),
		DatabaseURL:            getEnv("DATABASE_URL", "postgres://rss:rss@localhost:5432/rss"),
		RedisURL:              getEnv("REDIS_URL", "redis://localhost:6379"),
		QdrantHost:            getEnv("QDRANT_HOST", "localhost"),
		QdrantPort:            getEnvInt("QDRANT_PORT", 6333),
		SecretKey:             secretKey,
		JWTExpiration:         getEnvDuration("JWT_EXPIRATION", 24*time.Hour),
		TranslationURL:        getEnv("TRANSLATION_URL", "http://libretranslate:7790"),
		OllamaURL:             getEnv("OLLAMA_URL", "http://ollama:11434"),
		SpacyURL:             getEnv("SPACY_URL", "http://spacy:8000"),
		DefaultLang:          getEnv("DEFAULT_LANG", "es"),
		NewsPerPage:          getEnvInt("NEWS_PER_PAGE", 30),
		RateLimitPerMinute:   getEnvInt("RATE_LIMIT_PER_MINUTE", 60),
		DockerComposeDir:     getEnv("DOCKER_COMPOSE_DIR", "/datos/rss2"),
		WikiImagesPath:       getEnv("WIKI_IMAGES_PATH", "/app/data/wiki_images"),
		AllowedOrigins:       allowedOrigins,
	}
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

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}