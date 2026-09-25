package workers

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"

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
	if cfg == nil {
		cfg = LoadDBConfig()
	}
	// Los workers solo reciben DB_HOST/DB_PORT/DB_NAME/DB_USER/DB_PASS por
	// env (ver docker-compose.yml); aquí se construye el DSN y se crea el
	// pool compartido. Sin esto el pool queda nil y todos los workers
	// mueren con "database pool not initialized".
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Path:   "/" + cfg.DBName,
	}
	q := u.Query()
	q.Set("sslmode", "disable")
	u.RawQuery = q.Encode()
	if err := db.Connect(u.String()); err != nil {
		return err
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
