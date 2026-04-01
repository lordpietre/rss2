package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rss2/backend/internal/auth"
	"github.com/rss2/backend/internal/models"
)

func init() {
	gin.SetMode(gin.TestMode)
	auth.SetJWTSecret("test-secret-key-12345")
}

func TestLoginInvalidRequest(t *testing.T) {
	router := gin.New()
	router.POST("/auth/login", Login)

	body := []byte(`{}`)
	req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var resp models.ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Error != "Invalid request" {
		t.Errorf("expected 'Invalid request', got %s", resp.Error)
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	router := gin.New()
	router.POST("/auth/login", Login)

	body := []byte(`{"email":"invalid@test.com","password":"wrongpass"}`)
	req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestRegisterInvalidRequest(t *testing.T) {
	router := gin.New()
	router.POST("/auth/register", Register)

	body := []byte(`{}`)
	req, _ := http.NewRequest("POST", "/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCheckFirstUser(t *testing.T) {
	router := gin.New()
	router.GET("/auth/check-first-user", CheckFirstUser)

	req, _ := http.NewRequest("GET", "/auth/check-first-user", nil)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if _, ok := resp["is_first_user"]; !ok {
		t.Error("expected is_first_user in response")
	}
}

func TestGetCurrentUserUnauthorized(t *testing.T) {
	router := gin.New()
	router.GET("/auth/me", GetCurrentUser)

	req, _ := http.NewRequest("GET", "/auth/me", nil)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}
