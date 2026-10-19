# Auditoría de Seguridad y Código - Módulo de Suscripciones RSS2

**Fecha:** 2026-10-12  
**Auditor:** Code Review  
**Versión del código:** Subscription Module  

---

## 🔴 CRÍTICO - Bugs que requieren fix inmediato

### 1. Validación de Receipt Falsa (SEGURIDAD CRÍTICA)

**Archivo:** `backend/internal/handlers/subscription.go:49-102`

**Problema:**
```go
func ValidateSubscriptionReceipt(c *gin.Context) {
    // ...
    plan := getPlanFromProductID(req.ProductID)
    // NO SE VERIFICA EL RECEIPT TOKEN CON GOOGLE PLAY
    _, err := db.GetPool().Exec(c.Request.Context(), `
        UPDATE users SET plan = $1, ...
    `, plan, ...)
}
```

El endpoint acepta cualquier `receipt_token` sin validación. Un atacante puede:
1. Enviar `{"receipt_token": "fake_token", "product_id": "premium_monthly"}`
2. Obtener acceso Premium sin pagar

**Impacto:** Pérdida masiva de ingresos por suscripciones.

**Fix requerido:** Implementar verificación con Google Play Developer API:
```go
// Validar con Google Play
resp, err := http.PostForm("https://oauth2.googleapis.com/tokeninfo", 
    url.Values{"access_token": {googleAccessToken}})

// O usar la library oficial de Google
// "github.com/googleapis/google-api-go-generator"
```

---

### 2. Consultas SQL Mal Diseñadas (BUG FUNCIONAL)

**Archivos:**
- `backend/internal/handlers/subscription.go:246-250`
- `backend/internal/handlers/subscription.go:320-324`

**Problema:**
```go
// GetUsage - línea 246-250
SELECT COUNT(*) FROM traducciones t 
JOIN noticias n ON t.noticia_id = n.id 
WHERE n.user_id = $1 AND t.created_at >= CURRENT_DATE
```

La tabla `noticias` NO tiene columna `user_id`. Esto causa:
- Error en tiempo de ejecución (columna no existe)
- O conteo incorrecto de traducciones

**Fix requerido:** Las traducciones no pertenecen a usuarios directamente. Si el objetivo es contar traducciones totales del sistema, usar:
```sql
SELECT COUNT(*) FROM traducciones WHERE created_at >= CURRENT_DATE
```

---

## 🟠 ALTO - Bugs que afectan funcionalidad

### 3. Variable No Utilizada en Register (BUG)

**Archivo:** `backend/internal/handlers/auth.go:152-157`

**Problema:**
```go
// Línea 147: err ya fue asignado
err = db.GetPool().QueryRow(...).Scan(...)

// Línea 152-153: Nuevo bloque if err != nil
if err != nil {
    json.Unmarshal(limitsJSON, &user.Limits)  // Esto nunca ejecutará err != nil
    if err != nil {  // err aquí siempre es nil (de json.Unmarshal)
        c.JSON(http.StatusInternalServerError, ...)
    }
}
```

El segundo `if err != nil` es código muerto.

**Fix:**
```go
// Parse limits JSON
if err := json.Unmarshal(limitsJSON, &user.Limits); err != nil {
    c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to parse limits"})
    return
}
```

---

### 4. Race Condition en Verificación de Expiración

**Archivo:** `backend/internal/handlers/subscription.go:183-193`

**Problema:**
```go
// GetSubscriptionStatus
if user.SubscriptionStatus == "active" && user.SubscriptionEnd.Before(time.Now()) {
    // Window de carrera aquí - dos requests concurrentes
    db.GetPool().Exec(...) // Ambos intentarán hacer UPDATE
}
```

**Fix:** Usar transacción o UPDATE atómico:
```go
UPDATE users SET subscription_status = 'expired', plan = 'free'
WHERE id = $1 AND subscription_status = 'active' AND subscription_end < NOW()
RETURNING id
```
Si `RETURNING id` no devuelve rows, otro proceso ya hizo el update.

---

### 5. Rate Limiting Ausente

**Archivos:** `backend/cmd/server/main.go` (registro de rutas)

**Problema:** Los endpoints `/app/subscription/*` no tienen rate limiting específico.

Un atacante puede:
- Hacer brute force de validación de receipts
- Flood de requests para causar DoS

**Fix:** Añadir middleware de rate limiting específico para estos endpoints:
```go
app.POST("/subscription/validate", 
    middleware.RateLimitMiddleware(5), // 5 req/min
    handlers.ValidateSubscriptionReceipt)
```

---

## 🟡 MEDIO - Mejoras recomendadas

### 6. Sin Logs de Auditoría

**Problema:** No hay registro de cambios de suscripción.

Dificulta investigación de fraude y debugging.

**Fix:** Implementar logging estructurado:
```go
log.Info().
    Uint("user_id", uint(claims.UserID)).
    Str("action", "subscription_activated").
    Str("plan", plan).
    Msg("Subscription changed")
```

---

### 7. Límites No Aplicados en Endpoints Principales

**Problema:** `CheckPlanLimits` verifica límites pero estos no se aplican en los handlers reales de feeds, traducciones, etc.

Actualmente:
- `/app/limits/check?action=create_feed` → devuelve `allowed: true/false`
- Pero `POST /feeds` no llama a `CheckPlanLimits`

**Fix:** Crear middleware que aplique los límites:
```go
func PlanLimitMiddleware(action string) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Verificar límite
        // Si no tiene acceso, abortar con 402 Payment Required
    }
}

feeds.POST("", middleware.AuthRequired(), PlanLimitMiddleware("create_feed"), handlers.CreateFeed)
```

---

### 8. Inconsistencia en Tipos de Fecha

**Archivos:** `backend/internal/handlers/auth.go` y `subscription.go`

**Problema:**
- Login usa `*string` para `subscription_end` (línea 54) pero nunca lo usa
- GetSubscriptionStatus usa `*time.Time` directamente
- Register no devuelve `subscription_end` en la respuesta

**Fix:** Estandarizar a `*time.Time` en todos los endpoints.

---

## 📋 Resumen de Fixes Prioritarios

| Prioridad | Issue | Ubicación | 
|-----------|-------|-----------|
| 🔴 CRÍTICO | Validación receipt falsa | subscription.go:49 |
| 🔴 CRÍTICO | Queries con columnas inexistentes | subscription.go:246,320 |
| 🟠 ALTO | Código muerto en Register | auth.go:152-157 |
| 🟠 ALTO | Race condition expiración | subscription.go:183 |
| 🟠 ALTO | Sin rate limiting | main.go (rutas) |
| 🟡 MEDIO | Sin logs de auditoría | subscription.go |
| 🟡 MEDIO | Límites no aplicados | todas las rutas |

---

## 🧪 Tests Recomendados

1. **Test de seguridad:** Enviar receipt_token falso y verificar que es rechazado
2. **Test de race condition:** Concurrent calls a `GetSubscriptionStatus` con suscripción expirada
3. **Test de integración:** Verificar que límites se aplican correctamente en cada endpoint
4. **Test de rate limiting:** Flood de requests y verificar que se rechazan

---

## Recommendations

1. **Fix inmediato:** Implementar validación real de Google Play receipts
2. **Fix inmediato:** Corregir queries de conteo de traducciones
3. **Sprint 1:** Rate limiting + middleware de límites
4. **Sprint 2:** Logging de auditoría + tests
