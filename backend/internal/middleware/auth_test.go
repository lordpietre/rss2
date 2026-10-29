package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rss2/backend/internal/auth"
)

func init() {
	gin.SetMode(gin.TestMode)
	auth.SetJWTSecret("test-secret-key-12345")
	os.Setenv("SECRET_KEY", "test-secret-key-12345")
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
	// CORSMiddleware() lee la configuracion, que exige SECRET_KEY, y solo
	// devuelve la cabecera CORS para origenes permitidos.
	os.Setenv("SECRET_KEY", "test-secret-key")
	os.Setenv("ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:3000")

	router := gin.New()
	router.Use(CORSMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Origen permitido: debe devolverse la cabecera.
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("expected CORS header for allowed origin, got %q", w.Header().Get("Access-Control-Allow-Origin"))
	}

	// Origen no permitido: NO debe devolverse la cabecera (CORS no es
	// "*": con credenciales, permitir cualquier origen seria inseguro).
	req2, _ := http.NewRequest("GET", "/test", nil)
	req2.Header.Set("Origin", "https://sitio-malicioso.example")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("expected no CORS header for disallowed origin, got %q", w2.Header().Get("Access-Control-Allow-Origin"))
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
