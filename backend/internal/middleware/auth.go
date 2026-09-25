package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rss2/backend/internal/auth"
	"github.com/rss2/backend/internal/config"
	"github.com/rss2/backend/internal/logger"
	"golang.org/x/time/rate"
)

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Bearer token required"})
			c.Abort()
			return
		}

		claims, err := auth.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		c.Set("user", claims)
		c.Next()
	}
}

func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		userVal, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
			c.Abort()
			return
		}

		claims := userVal.(*auth.Claims)
		if !claims.IsAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func CORSMiddleware() gin.HandlerFunc {
	cfg := config.Load()
	allowedOrigins := strings.Split(cfg.AllowedOrigins, ",")

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		allowed := false
		for _, o := range allowedOrigins {
			o = strings.TrimSpace(o)
			if o == origin {
				allowed = true
				break
			}
		}

		if allowed {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = c.GetString("request_id")
		}

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method

		if raw != "" {
			path = path + "?" + raw
		}

		log := logger.GetLogger().With().
			Str("method", method).
			Str("path", path).
			Int("status", status).
			Str("client_ip", clientIP).
			Dur("latency", latency).
			Logger()

		if requestID != "" {
			log = log.With().Str("request_id", requestID).Logger()
		}

		if status >= 500 {
			log.Error().Msg("Server error")
		} else if status >= 400 {
			log.Warn().Msg("Client error")
		} else {
			log.Info().Msg("Request completed")
		}
	}
}

type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	clientLimiters = make(map[string]*clientLimiter)
	limitersMu     sync.Mutex
	// Observabilidad del rate limit (Fase 3): conteo de 429 totales y por
	// configuración (requestsPerMinute). Sin IPs: solo agregados.
	rateLimitHitsTotal atomic.Int64
	rateLimitHitsByCfg = make(map[int]*atomic.Int64)
	rateLimitHitsMu    sync.Mutex
)

// RateLimitStats devuelve los 429 observados desde el arranque.
func RateLimitStats() (int64, map[int]int64) {
	byCfg := make(map[int]int64)
	rateLimitHitsMu.Lock()
	for k, v := range rateLimitHitsByCfg {
		byCfg[k] = v.Load()
	}
	rateLimitHitsMu.Unlock()
	return rateLimitHitsTotal.Load(), byCfg
}

func bumpRateLimitHit(requestsPerMinute int) {
	rateLimitHitsTotal.Add(1)
	rateLimitHitsMu.Lock()
	ctr, ok := rateLimitHitsByCfg[requestsPerMinute]
	if !ok {
		ctr = &atomic.Int64{}
		rateLimitHitsByCfg[requestsPerMinute] = ctr
	}
	rateLimitHitsMu.Unlock()
	ctr.Add(1)
}

func RateLimitMiddleware(requestsPerMinute int) gin.HandlerFunc {
	limit := rate.Limit(requestsPerMinute) / 60
	burst := requestsPerMinute

	return func(c *gin.Context) {
		ip := c.ClientIP()
		// Cada configuración (requestsPerMinute) tiene su propio bucket por IP.
		// Antes se compartía un único bucket global y el limitador estricto
		// de /auth (10 req/min) secuestraba a toda la API causando 429s
		// espurios en /api/news con navegación normal.
		key := fmt.Sprintf("%s|%d", ip, requestsPerMinute)

		limitersMu.Lock()
		cl, exists := clientLimiters[key]
		if !exists || time.Since(cl.lastSeen) > 10*time.Minute {
			cl = &clientLimiter{limiter: rate.NewLimiter(limit, burst)}
			clientLimiters[key] = cl
		}
		cl.lastSeen = time.Now()
		limiter := cl.limiter
		limitersMu.Unlock()

		if !limiter.Allow() {
			bumpRateLimitHit(requestsPerMinute)
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Rate limit exceeded"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = fmt.Sprintf("%d", time.Now().UnixNano())
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}
