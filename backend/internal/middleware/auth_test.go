package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rss2/backend/internal/auth"
)

func init() {
	gin.SetMode(gin.TestMode)
	auth.SetJWTSecret("test-secret-key-12345")
}

func TestAuthRequiredNoHeader(t *testing.T) {
	router := gin.New()
	router.Use(AuthRequired())
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req, _ := http.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestAuthRequiredInvalidToken(t *testing.T) {
	router := gin.New()
	router.Use(AuthRequired())
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestAuthRequiredValidToken(t *testing.T) {
	router := gin.New()
	router.Use(AuthRequired())
	router.GET("/protected", func(c *gin.Context) {
		userVal, _ := c.Get("user")
		claims := userVal.(*auth.Claims)
		c.JSON(200, gin.H{"user_id": claims.UserID})
	})

	token, _ := auth.GenerateToken(42, "test@test.com", "testuser", false)

	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestAdminRequiredNotAdmin(t *testing.T) {
	router := gin.New()
	router.Use(AuthRequired())
	router.Use(AdminRequired())
	router.GET("/admin", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	token, _ := auth.GenerateToken(1, "test@test.com", "testuser", false)

	req, _ := http.NewRequest("GET", "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

func TestAdminRequiredIsAdmin(t *testing.T) {
	router := gin.New()
	router.Use(AuthRequired())
	router.Use(AdminRequired())
	router.GET("/admin", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	token, _ := auth.GenerateToken(1, "test@test.com", "adminuser", true)

	req, _ := http.NewRequest("GET", "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestCORSMiddleware(t *testing.T) {
	router := gin.New()
	router.Use(CORSMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("expected CORS header to be set")
	}
}

func TestCORSPreflight(t *testing.T) {
	router := gin.New()
	router.Use(CORSMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req, _ := http.NewRequest("OPTIONS", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204 for preflight, got %d", w.Code)
	}
}
