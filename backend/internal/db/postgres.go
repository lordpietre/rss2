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

	// Al arrancar el host todos los contenedores levantan a la vez: el
	// Postgres puede tardar unos segundos en aceptar conexiones. Sin esta
	// espera el proceso muere y Docker lo reinicia con backoff (varios
	// reinicios por arranque). Se reintenta un numero acotado de veces.
	// Cada intento usa su propio contexto: un unico timeout global podria
	// expirar antes de agotar los intentos y los ultimos "reintentos" se
	// quedarian sin probar de verdad.
	for attempt := 1; attempt <= 30; attempt++ {
		pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = Pool.Ping(pingCtx)
		cancel()
		if err == nil {
			return nil
		}
		if attempt == 30 {
			break
		}
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("failed to ping database: %w", err)
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
