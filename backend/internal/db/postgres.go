package db

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var Pool *pgxpool.Pool

func Connect(databaseURL string) error {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("failed to parse database URL: %w", err)
	}

	config.MaxConns = int32(getEnvInt("DB_MAX_CONNS", 25))
	config.MinConns = int32(getEnvInt("DB_MIN_CONNS", 5))
	config.MaxConnLifetime = getEnvDuration("DB_MAX_CONN_LIFETIME", time.Hour)
	config.MaxConnIdleTime = getEnvDuration("DB_MAX_CONN_IDLE_TIME", 30*time.Minute)
	config.HealthCheckPeriod = getEnvDuration("DB_HEALTH_CHECK_PERIOD", time.Minute)

	Pool, err = pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return fmt.Errorf("failed to create pool: %w", err)
	}

	if err = Pool.Ping(context.Background()); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	return nil
}

func Close() {
	if Pool != nil {
		Pool.Close()
	}
}

func GetPool() *pgxpool.Pool {
	return Pool
}

func HealthCheck(ctx context.Context) error {
	if Pool == nil {
		return fmt.Errorf("pool not initialized")
	}
	return Pool.Ping(ctx)
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
