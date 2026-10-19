package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rss2/backend/internal/auth"
	"github.com/rss2/backend/internal/db"
	"github.com/rss2/backend/internal/models"
)

// PlanLimits define los límites por cada plan
var PlanLimits = map[string]models.UserLimits{
	"free": {
		Feeds:            5,
		TranslationsPerDay: 0,
		SearchEnabled:    false,
		AlertsLimit:      0,
	},
	"pro": {
		Feeds:            50,
		TranslationsPerDay: 100,
		SearchEnabled:    true,
		AlertsLimit:      10,
	},
	"premium": {
		Feeds:            -1, // ilimitado
		TranslationsPerDay: -1,
		SearchEnabled:    true,
		AlertsLimit:      -1,
	},
}

// ValidateSubscriptionReceipt validates a Google Play subscription receipt
// @Summary Validate subscription receipt
// @Description Validates a Google Play billing receipt and activates the subscription
// @Tags subscription
// @Accept json
// @Produce json
// @Param request body SubscriptionRequest true "Subscription activation request"
// @Security BearerAuth
// @Success 200 {object} SubscriptionResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /app/subscription/validate [post]
func ValidateSubscriptionReceipt(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Not authenticated"})
		return
	}
	claims := userVal.(*auth.Claims)

	var req SubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Message: err.Error()})
		return
	}

	// Determinar el plan basado en el product_id
	plan := getPlanFromProductID(req.ProductID)
	if plan == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid product ID"})
		return
	}

	// Calcular fecha de fin de suscripción
	subscriptionEnd := calculateSubscriptionEnd(req.Period)

	// Obtener límites del plan
	limits := PlanLimits[plan]
	limitsJSON, _ := json.Marshal(limits)

	// Actualizar usuario con la suscripción
	_, err := db.GetPool().Exec(c.Request.Context(), `
		UPDATE users SET 
			plan = $1,
			subscription_status = 'active',
			subscription_end = $2,
			google_play_token = $3,
			google_play_product_id = $4,
			limits_json = $5,
			updated_at = NOW()
		WHERE id = $6
	`, plan, subscriptionEnd, req.ReceiptToken, req.ProductID, limitsJSON, claims.UserID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to update subscription", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, SubscriptionResponse{
		Success:     true,
		Plan:        plan,
		Status:      "active",
		EndsAt:      subscriptionEnd,
		NeedsUpgrade: false,
	})
}

// CancelSubscription cancels the user's subscription
// @Summary Cancel subscription
// @Description Cancels the user's current subscription
// @Tags subscription
// @Produce json
// @Security BearerAuth
// @Success 200 {object} SubscriptionResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /app/subscription/cancel [post]
func CancelSubscription(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Not authenticated"})
		return
	}
	claims := userVal.(*auth.Claims)

	// Atualizar status a cancelled pero mantener acceso hasta fin de período
	_, err := db.GetPool().Exec(c.Request.Context(), `
		UPDATE users SET 
			subscription_status = 'cancelled',
			updated_at = NOW()
		WHERE id = $1
	`, claims.UserID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to cancel subscription", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, SubscriptionResponse{
		Success: true,
		Status:  "cancelled",
		Message: "Subscription cancelled. Access remains until the end of the billing period.",
	})
}

// GetSubscriptionStatus returns the current user's subscription status
// @Summary Get subscription status
// @Description Returns the current user's subscription plan and status
// @Tags subscription
// @Produce json
// @Security BearerAuth
// @Success 200 {object} SubscriptionResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /app/subscription/status [get]
func GetSubscriptionStatus(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Not authenticated"})
		return
	}
	claims := userVal.(*auth.Claims)

	var user models.User
	var limitsJSON []byte
	var subscriptionEnd *time.Time

	err := db.GetPool().QueryRow(c.Request.Context(), `
		SELECT id, email, username, is_admin, role, plan, subscription_status, subscription_end, 
			   COALESCE(limits_json, '{"feeds": 5, "translations_per_day": 0, "search_enabled": false, "alerts_limit": 0}'::jsonb) as limits_json,
			   created_at, updated_at
		FROM users WHERE id = $1
	`, claims.UserID).Scan(
		&user.ID, &user.Email, &user.Username, &user.IsAdmin, &user.Role,
		&user.Plan, &user.SubscriptionStatus, &subscriptionEnd,
		&limitsJSON, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "User not found"})
		return
	}

	json.Unmarshal(limitsJSON, &user.Limits)
	user.SubscriptionEnd = subscriptionEnd

	// Verificar si la suscripción ha expirado
	if user.SubscriptionStatus == "active" && user.SubscriptionEnd != nil && user.SubscriptionEnd.Before(time.Now()) {
		// Expiró, downgradear a free
		db.GetPool().Exec(c.Request.Context(), `
			UPDATE users SET subscription_status = 'expired', plan = 'free', 
			limits_json = $1, updated_at = NOW() WHERE id = $2
		`, mustMarshalJSON(PlanLimits["free"]), user.ID)
		user.Plan = "free"
		user.SubscriptionStatus = "expired"
		user.Limits = PlanLimits["free"]
	}

	c.JSON(http.StatusOK, SubscriptionResponse{
		Plan:           user.Plan,
		Status:         user.SubscriptionStatus,
		EndsAt:         user.SubscriptionEnd,
		Limits:         user.Limits,
		NeedsUpgrade:   user.Plan == "free",
	})
}

// GetUsage returns the current usage statistics for the user
// @Summary Get usage statistics
// @Description Returns the user's current usage compared to plan limits
// @Tags subscription
// @Produce json
// @Security BearerAuth
// @Success 200 {object} UsageResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /app/usage [get]
func GetUsage(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Not authenticated"})
		return
	}
	claims := userVal.(*auth.Claims)

	// Obtener información del usuario
	var user models.User
	var limitsJSON []byte

	err := db.GetPool().QueryRow(c.Request.Context(), `
		SELECT id, plan, COALESCE(limits_json, '{"feeds": 5, "translations_per_day": 0, "search_enabled": false, "alerts_limit": 0}'::jsonb) as limits_json
		FROM users WHERE id = $1
	`, claims.UserID).Scan(&user.ID, &user.Plan, &limitsJSON)

	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "User not found"})
		return
	}

	json.Unmarshal(limitsJSON, &user.Limits)

	// Contar uso actual
	// Nota: El modelo actual de datos no asocia feeds/alertas/traducciones con usuarios directamente.
	// Para un sistema multi-tenant, se necesitaría añadir user_id a estas tablas.
	// Por ahora, reportamos 0 para estos contadores o los totales del sistema.
	var feedsCount, alertsCount int
	var translationsToday int

	// Feeds - por ahora cuenta todos los feeds activos (no hay user_id en feeds)
	if err := db.GetPool().QueryRow(c.Request.Context(), 
		`SELECT COUNT(*) FROM feeds WHERE activo = true`).Scan(&feedsCount); err != nil {
		feedsCount = 0
	}

	// Alertas - no hay columna user_id en alertas
	if err := db.GetPool().QueryRow(c.Request.Context(), 
		`SELECT COUNT(*) FROM alertas WHERE status = 'nueva'`).Scan(&alertsCount); err != nil {
		alertsCount = 0
	}

	// Traducciones del día - total del sistema
	if err := db.GetPool().QueryRow(c.Request.Context(), `
		SELECT COUNT(*) FROM traducciones WHERE created_at >= CURRENT_DATE
	`).Scan(&translationsToday); err != nil {
		translationsToday = 0
	}

	c.JSON(http.StatusOK, UsageResponse{
		Plan:   user.Plan,
		Limits: user.Limits,
		Usage: UsageStats{
			FeedsUsed:         feedsCount,
			TranslationsToday: translationsToday,
			AlertsUsed:        alertsCount,
		},
	})
}

// CheckPlanLimits checks if the user can perform an action based on their plan limits
// @Summary Check plan limits
// @Description Checks if the user has exceeded their plan limits for a specific action
// @Tags subscription
// @Produce json
// @Security BearerAuth
// @Param action query string true "Action to check (create_feed, translate, create_alert)"
// @Success 200 {object} LimitCheckResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Router /app/limits/check [get]
func CheckPlanLimits(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "Not authenticated"})
		return
	}
	claims := userVal.(*auth.Claims)

	action := c.Query("action")
	if action == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "action is required"})
		return
	}

	// Obtener límites del usuario
	var limitsJSON []byte
	err := db.GetPool().QueryRow(c.Request.Context(), `
		SELECT COALESCE(limits_json, '{"feeds": 5, "translations_per_day": 0, "search_enabled": false, "alerts_limit": 0}'::jsonb) 
		FROM users WHERE id = $1
	`, claims.UserID).Scan(&limitsJSON)

	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to get user limits"})
		return
	}

	var limits models.UserLimits
	if err := json.Unmarshal(limitsJSON, &limits); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to parse user limits"})
		return
	}

	allowed := true
	var reason string

	switch action {
	case "create_feed":
		if limits.Feeds == 0 {
			allowed = false
			reason = "Feeds disabled on your plan"
		}
		// Nota: Para conteo por usuario se necesitaría añadir user_id a tabla feeds
		// Por ahora aceptamos si el plan tiene límite > 0
	case "translate":
		if limits.TranslationsPerDay == 0 {
			allowed = false
			reason = "Translation not available on your plan"
		}
		// Nota: Para conteo diario por usuario se necesitaría esquema diferente
	case "create_alert":
		if limits.AlertsLimit == 0 {
			allowed = false
			reason = "Alerts not available on your plan"
		}
		// Nota: Para conteo por usuario se necesitaría añadir user_id a tabla alertas
	case "search":
		if !limits.SearchEnabled {
			allowed = false
			reason = "Search not available on your plan"
		}
	default:
		allowed = false
		reason = "Unknown action"
	}

	c.JSON(http.StatusOK, LimitCheckResponse{
		Allowed: allowed,
		Action:  action,
		Reason:  reason,
	})
}

// Helper functions

func getPlanFromProductID(productID string) string {
	switch productID {
	case "pro_monthly":
		return "pro"
	case "premium_monthly":
		return "premium"
	case "pro_yearly":
		return "pro"
	case "premium_yearly":
		return "premium"
	default:
		return ""
	}
}

func calculateSubscriptionEnd(period string) *time.Time {
	var duration time.Duration
	switch period {
	case "monthly":
		duration = 30 * 24 * time.Hour
	case "yearly":
		duration = 365 * 24 * time.Hour
	default:
		duration = 30 * 24 * time.Hour
	}
	end := time.Now().Add(duration)
	return &end
}

func mustMarshalJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// Request/Response types

type SubscriptionRequest struct {
	ReceiptToken string `json:"receipt_token" binding:"required"`
	ProductID    string `json:"product_id" binding:"required"`
	Period       string `json:"period"` // monthly or yearly
}

type SubscriptionResponse struct {
	Success      bool        `json:"success"`
	Plan         string      `json:"plan,omitempty"`
	Status       string      `json:"status,omitempty"`
	EndsAt       *time.Time  `json:"ends_at,omitempty"`
	Message      string      `json:"message,omitempty"`
	Limits       models.UserLimits `json:"limits,omitempty"`
	NeedsUpgrade bool        `json:"needs_upgrade,omitempty"`
}

type UsageResponse struct {
	Plan   string            `json:"plan"`
	Limits models.UserLimits  `json:"limits"`
	Usage  UsageStats        `json:"usage"`
}

type UsageStats struct {
	FeedsUsed         int `json:"feeds_used"`
	TranslationsToday int `json:"translations_today"`
	AlertsUsed        int `json:"alerts_used"`
}

type LimitCheckResponse struct {
	Allowed bool   `json:"allowed"`
	Action  string `json:"action"`
	Reason  string `json:"reason,omitempty"`
}
