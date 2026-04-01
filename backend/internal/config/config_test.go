package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	os.Clearenv()
	
	cfg := Load()
	
	if cfg.ServerPort != "8080" {
		t.Errorf("expected ServerPort '8080', got %s", cfg.ServerPort)
	}
	if cfg.SecretKey != "change-this-secret-key" {
		t.Errorf("expected SecretKey 'change-this-secret-key', got %s", cfg.SecretKey)
	}
	if cfg.DefaultLang != "es" {
		t.Errorf("expected DefaultLang 'es', got %s", cfg.DefaultLang)
	}
	if cfg.NewsPerPage != 30 {
		t.Errorf("expected NewsPerPage 30, got %d", cfg.NewsPerPage)
	}
	if cfg.DockerComposeDir != "/datos/rss2" {
		t.Errorf("expected DockerComposeDir '/datos/rss2', got %s", cfg.DockerComposeDir)
	}
	if cfg.WikiImagesPath != "/app/data/wiki_images" {
		t.Errorf("expected WikiImagesPath '/app/data/wiki_images', got %s", cfg.WikiImagesPath)
	}
	if cfg.AllowedOrigins != "*" {
		t.Errorf("expected AllowedOrigins '*', got %s", cfg.AllowedOrigins)
	}
}

func TestLoadFromEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("SERVER_PORT", "9000")
	os.Setenv("DATABASE_URL", "postgres://user:pass@host:5432/db")
	os.Setenv("SECRET_KEY", "my-secret-key")
	os.Setenv("DEFAULT_LANG", "en")
	os.Setenv("NEWS_PER_PAGE", "50")
	os.Setenv("JWT_EXPIRATION", "48h")
	os.Setenv("RATE_LIMIT_PER_MINUTE", "100")
	os.Setenv("DOCKER_COMPOSE_DIR", "/custom/path")
	os.Setenv("WIKI_IMAGES_PATH", "/custom/images")
	
	cfg := Load()
	
	if cfg.ServerPort != "9000" {
		t.Errorf("expected ServerPort '9000', got %s", cfg.ServerPort)
	}
	if cfg.DatabaseURL != "postgres://user:pass@host:5432/db" {
		t.Errorf("expected DatabaseURL, got %s", cfg.DatabaseURL)
	}
	if cfg.SecretKey != "my-secret-key" {
		t.Errorf("expected SecretKey 'my-secret-key', got %s", cfg.SecretKey)
	}
	if cfg.DefaultLang != "en" {
		t.Errorf("expected DefaultLang 'en', got %s", cfg.DefaultLang)
	}
	if cfg.NewsPerPage != 50 {
		t.Errorf("expected NewsPerPage 50, got %d", cfg.NewsPerPage)
	}
	if cfg.JWTExpiration != 48*time.Hour {
		t.Errorf("expected JWTExpiration 48h, got %v", cfg.JWTExpiration)
	}
	if cfg.RateLimitPerMinute != 100 {
		t.Errorf("expected RateLimitPerMinute 100, got %d", cfg.RateLimitPerMinute)
	}
	if cfg.DockerComposeDir != "/custom/path" {
		t.Errorf("expected DockerComposeDir '/custom/path', got %s", cfg.DockerComposeDir)
	}
	if cfg.WikiImagesPath != "/custom/images" {
		t.Errorf("expected WikiImagesPath '/custom/images', got %s", cfg.WikiImagesPath)
	}
}

func TestLoadInvalidInt(t *testing.T) {
	os.Clearenv()
	os.Setenv("NEWS_PER_PAGE", "invalid")
	
	cfg := Load()
	
	if cfg.NewsPerPage != 30 {
		t.Errorf("expected default NewsPerPage 30 for invalid value, got %d", cfg.NewsPerPage)
	}
}

func TestLoadInvalidDuration(t *testing.T) {
	os.Clearenv()
	os.Setenv("JWT_EXPIRATION", "invalid")
	
	cfg := Load()
	
	if cfg.JWTExpiration != 24*time.Hour {
		t.Errorf("expected default JWTExpiration 24h for invalid value, got %v", cfg.JWTExpiration)
	}
}

func TestGetEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("TEST_KEY", "test_value")
	
	result := getEnv("TEST_KEY", "default")
	if result != "test_value" {
		t.Errorf("expected 'test_value', got %s", result)
	}
	
	result = getEnv("NON_EXISTENT", "default")
	if result != "default" {
		t.Errorf("expected 'default', got %s", result)
	}
}

func TestGetEnvInt(t *testing.T) {
	os.Clearenv()
	os.Setenv("TEST_INT", "123")
	
	result := getEnvInt("TEST_INT", 0)
	if result != 123 {
		t.Errorf("expected 123, got %d", result)
	}
	
	result = getEnvInt("NON_EXISTENT", 99)
	if result != 99 {
		t.Errorf("expected 99, got %d", result)
	}
}

func TestGetEnvDuration(t *testing.T) {
	os.Clearenv()
	os.Setenv("TEST_DURATION", "1h30m")
	
	result := getEnvDuration("TEST_DURATION", 0)
	if result != 90*time.Minute {
		t.Errorf("expected 90m, got %v", result)
	}
	
	result = getEnvDuration("NON_EXISTENT", 5*time.Minute)
	if result != 5*time.Minute {
		t.Errorf("expected 5m, got %v", result)
	}
}
