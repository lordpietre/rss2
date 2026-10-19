package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rss2/backend/internal/auth"
	"github.com/rss2/backend/internal/db"
	"github.com/rss2/backend/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// CheckFirstUser checks if this is the first user registration
// @Summary Check if first user
// @Description Returns whether this is the first user being registered (no users exist yet)
// @Tags auth
// @Produce json
// @Success 200 {object} map[string]interface{} "is_first_user and total_users"
// @Failure 500 {object} models.ErrorResponse
// @Router /auth/check-first-user [get]
func CheckFirstUser(c *gin.Context) {
	var count int
	err := db.GetPool().QueryRow(c.Request.Context(), "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check users"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"is_first_user": count == 0, "total_users": count})
}

// Login authenticates a user and returns a JWT token
// @Summary User login
// @Description Authenticate with email and password to receive a JWT token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body models.LoginRequest true "Login credentials"
// @Success 200 {object} models.AuthResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /auth/login [post]
func Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Message: err.Error()})
		return
	}

	var user models.User
	var limitsJSON []byte
	var subscriptionEnd *string
	err := db.GetPool().QueryRow(c.Request.Context(), `
		SELECT id, email, username, password_hash, is_admin, role, plan, subscription_status, subscription_end,
			   COALESCE(limits_json, '{"feeds": 5, "translations_per_day": 0, "search_enabled": false, "alerts_limit": 0}'::jsonb) as limits_json,
			   created_at, updated_at
		FROM users WHERE email = $1`, req.Email).Scan(
		&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.IsAdmin, &user.Role,
		&user.Plan, &user.SubscriptionStatus, &subscriptionEnd,
		&limitsJSON, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Invalid credentials"})
		return
	}

	// Parse limits JSON
	json.Unmarshal(limitsJSON, &user.Limits)

	tokenString, err := auth.GenerateToken(user.ID, user.Email, user.Username, user.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, models.AuthResponse{
		Token: tokenString,
		User:  user,
	})
}

// Register creates a new user account
// @Summary Register new user
// @Description Create a new user account (first user becomes admin automatically)
// @Tags auth
// @Accept json
// @Produce json
// @Param request body models.RegisterRequest true "Registration details"
// @Success 201 {object} models.AuthResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /auth/register [post]
func Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Message: err.Error()})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to hash password"})
		return
	}

	var userCount int
	db.GetPool().QueryRow(c.Request.Context(), "SELECT COUNT(*) FROM users").Scan(&userCount)
	isFirstUser := userCount == 0

	// Default limits for new users
	defaultLimits := `{"feeds": 5, "translations_per_day": 0, "search_enabled": false, "alerts_limit": 0}`

	var userID int64
	err = db.GetPool().QueryRow(c.Request.Context(), `
		INSERT INTO users (email, username, password_hash, is_admin, plan, subscription_status, limits_json, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'free', 'none', $5::jsonb, NOW(), NOW())
		RETURNING id`,
		req.Email, req.Username, string(hashedPassword), isFirstUser, defaultLimits,
	).Scan(&userID)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			c.JSON(http.StatusConflict, models.ErrorResponse{Error: "Email or username already registered"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to create user", Message: err.Error()})
		return
	}

	var user models.User
	var limitsJSON []byte
	err = db.GetPool().QueryRow(c.Request.Context(), `
		SELECT id, email, username, is_admin, role, plan, subscription_status,
			   COALESCE(limits_json, $1::jsonb) as limits_json, created_at, updated_at
		FROM users WHERE id = $2`, defaultLimits, userID).Scan(
		&user.ID, &user.Email, &user.Username, &user.IsAdmin, &user.Role,
		&user.Plan, &user.SubscriptionStatus, &limitsJSON,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch user"})
		return
	}

	// Parse limits JSON
	if err := json.Unmarshal(limitsJSON, &user.Limits); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to parse user limits"})
		return
	}

	tokenString, err := auth.GenerateToken(user.ID, user.Email, user.Username, user.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, models.AuthResponse{
		Token:       tokenString,
		User:        user,
		IsFirstUser: isFirstUser,
	})
}

// GetCurrentUser returns the current authenticated user
// @Summary Get current user
// @Description Returns the currently authenticated user's profile
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.User
// @Failure 401 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /auth/me [get]
func GetCurrentUser(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Not authenticated"})
		return
	}

	claims := userVal.(*auth.Claims)

	var user models.User
	var limitsJSON []byte
	err := db.GetPool().QueryRow(c.Request.Context(), `
		SELECT id, email, username, is_admin, role, plan, subscription_status, subscription_end,
			   COALESCE(limits_json, '{"feeds": 5, "translations_per_day": 0, "search_enabled": false, "alerts_limit": 0}'::jsonb) as limits_json,
			   created_at, updated_at
		FROM users WHERE id = $1`, claims.UserID).Scan(
		&user.ID, &user.Email, &user.Username, &user.IsAdmin, &user.Role,
		&user.Plan, &user.SubscriptionStatus, &user.SubscriptionEnd,
		&limitsJSON, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "User not found"})
		return
	}

	// Parse limits JSON
	json.Unmarshal(limitsJSON, &user.Limits)

	c.JSON(http.StatusOK, user)
}
