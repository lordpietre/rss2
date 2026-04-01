package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rss2/backend/internal/auth"
	"github.com/rss2/backend/internal/db"
	"github.com/rss2/backend/internal/models"
	"golang.org/x/crypto/bcrypt"
)

func CheckFirstUser(c *gin.Context) {
	var count int
	err := db.GetPool().QueryRow(c.Request.Context(), "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check users"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"is_first_user": count == 0, "total_users": count})
}

func Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Message: err.Error()})
		return
	}

	var user models.User
	err := db.GetPool().QueryRow(c.Request.Context(), `
		SELECT id, email, username, password_hash, is_admin, created_at, updated_at
		FROM users WHERE email = $1`, req.Email).Scan(
		&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.IsAdmin,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Invalid credentials"})
		return
	}

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

	var userID int64
	err = db.GetPool().QueryRow(c.Request.Context(), `
		INSERT INTO users (email, username, password_hash, is_admin, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING id`,
		req.Email, req.Username, string(hashedPassword), isFirstUser,
	).Scan(&userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to create user", Message: err.Error()})
		return
	}

	var user models.User
	err = db.GetPool().QueryRow(c.Request.Context(), `
		SELECT id, email, username, is_admin, created_at, updated_at
		FROM users WHERE id = $1`, userID).Scan(
		&user.ID, &user.Email, &user.Username, &user.IsAdmin,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch user"})
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

func GetCurrentUser(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Not authenticated"})
		return
	}

	claims := userVal.(*auth.Claims)

	var user models.User
	err := db.GetPool().QueryRow(c.Request.Context(), `
		SELECT id, email, username, is_admin, created_at, updated_at
		FROM users WHERE id = $1`, claims.UserID).Scan(
		&user.ID, &user.Email, &user.Username, &user.IsAdmin,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "User not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}
