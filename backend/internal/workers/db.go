package workers

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rss2/backend/internal/db"
)

type Config struct {
	Host     string
	Port     int
	DBName   string
	User     string
	Password string
}

func LoadDBConfig() *Config {
	return &Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnvInt("DB_PORT", 5432),
		DBName:   getEnv("DB_NAME", "rss"),
		User:     getEnv("DB_USER", "rss"),
		Password: getEnv("DB_PASS", "rss"),
	}
}

func Connect(cfg *Config) error {
	// Just verify the main pool is accessible
	if db.GetPool() == nil {
		return fmt.Errorf("database pool not initialized")
	}
	return db.HealthCheck(context.Background())
}

func GetPool() *pgxpool.Pool {
	return db.GetPool()
}

func Close() {
	// No-op: main db package manages the pool
}

func HealthCheck(ctx context.Context) error {
	return db.HealthCheck(ctx)
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
