# Revisión de Código - Módulo de Suscripciones y App Android

**Fecha:** 2026-10-12  
**Revisor:** Code Audit  
**Versión analizada:** Backend (Go) + Frontend (React/TypeScript)

---

## 📋 RESUMEN EJECUTIVO

| Componente | Estado | Problemas |
|------------|--------|----------|
| Backend Go | ✅ Compila | 3 bugs, 2 vulnerabilidades |
| Frontend TS | ⚠️ Sin verificar | Requiere npm para verificar |
| Migración SQL | ✅ OK | Correcta |
| Documentación | ✅ Completa | 4 documentos |

---

## 🔴 PROBLEMAS CRÍTICOS - Backend Go

### 1. Validación de Receipt Falsa (SEGURIDAD CRÍTICA)

**Archivo:** `backend/internal/handlers/subscription.go:49-102`

**Problema:**
```go
func ValidateSubscriptionReceipt(c *gin.Context) {
    // ...
    plan := getPlanFromProductID(req.ProductID)  // Solo mapea product_id a plan
    // NO SE VALIDA EL RECEIPT TOKEN CON GOOGLE PLAY API
    _, err := db.GetPool().Exec(...)
}
```

**Impacto:** Cualquier usuario puede enviar `{"receipt_token": "fake", "product_id": "premium_monthly"}` y obtener acceso premium sin pagar.

**Solución requerida:** Integrar con Google Play Developer API:
```go
// Validar con Google Play
resp, err := http.PostForm("https://oauth2.googleapis.com/tokeninfo", 
    url.Values{"access_token": {googleAccessToken}})
// Verificar signature del receipt
```

---

### 2. Variable `subscriptionEnd` No Utilizada en Login

**Archivo:** `backend/internal/handlers/auth.go:54`

**Problema:**
```go
var user models.User
var limitsJSON []byte
var subscriptionEnd *string  // ❌ DECLARADA PERO NUNCA USADA
err := db.GetPool().QueryRow(c.Request.Context(), `
    SELECT ... subscription_end,
    ...
`).Scan(
    ..., &subscriptionEnd,  // Se asigna aquí
    ...
)
```

**Impacto:** Waste de memoria, confusión en código. El valor de `subscription_end` se ignora completamente después de asignarlo.

**Nota:** En `GetCurrentUser` (línea 198) se usa correctamente `&user.SubscriptionEnd`.

**Solución:** Eliminar la variable si no se usa, o usarla si era necesario.

---

### 3. Campos `Success` y `Message` en Response pero No Devueltos

**Archivo:** `backend/internal/handlers/subscription.go:195-201`

**Problema:**
```go
type SubscriptionResponse struct {
    Success      bool        `json:"success,omitempty"`
    Plan         string      `json:"plan,omitempty"`
    Status       string      `json:"status,omitempty"`
    EndsAt       *time.Time `json:"ends_at,omitempty"`
    Message      string      `json:"message,omitempty"`  // ❌ Definido pero no en todos los casos
    NeedsUpgrade bool        `json:"needs_upgrade,omitempty"`
}

func GetSubscriptionStatus(c *gin.Context) {
    c.JSON(http.StatusOK, SubscriptionResponse{
        Plan:           user.Plan,
        Status:         user.SubscriptionStatus,
        EndsAt:         user.SubscriptionEnd,
        Limits:         user.Limits,
        NeedsUpgrade:   user.Plan == "free",
        // ❌ Falta: Success, Message
    })
}
```

**Impacto:** Inconsistencia en la API - algunos endpoints devuelven `success`, otros no.

**Solución:** Estandarizar respuestas.

---

### 4. Race Condition en Verificación de Expiración

**Archivo:** `backend/internal/handlers/subscription.go:183-193`

**Problema:**
```go
if user.SubscriptionStatus == "active" && user.SubscriptionEnd.Before(time.Now()) {
    // DOS requests concurrentes pueden entrar aquí
    db.GetPool().Exec(...)  // Ambos intentarán hacer UPDATE
}
```

**Impacto:** Posible doble ejecución de UPDATE, logs confusos.

**Solución:** Usar UPDATE atómico con condición:
```go
UPDATE users SET subscription_status = 'expired', plan = 'free'
WHERE id = $1 AND subscription_status = 'active' AND subscription_end < NOW()
RETURNING id
// Si no devuelve rows, otro proceso ya lo hizo
```

---

## 🟠 PROBLEMAS MEDIOS - Backend Go

### 5. No Se Verifica Error en `json.Unmarshal` de Limits

**Archivo:** `backend/internal/handlers/auth.go:75`

**Problema:**
```go
json.Unmarshal(limitsJSON, &user.Limits)  // ❌ Error ignorado
```

**Impacto:** Si el JSON está corrupto en la BD, el error se ignora silenciosamente.

**Ocurrencias:**
- `auth.go:75` (Login)
- `auth.go:207` (GetCurrentUser)
- `subscription.go:180` (GetSubscriptionStatus)
- `subscription.go:236` (GetUsage)
- `subscription.go:313` (CheckPlanLimits)

---

### 6. Hardcoded Default Limits en Múltiples Lugares

**Problema:** Los límites por defecto están hardcoded como strings en:
- `auth.go:118` (Register)
- `auth.go:57` (Login query)
- `auth.go:194` (GetCurrentUser)
- `subscription.go:166` (GetSubscriptionStatus)
- `subscription.go:227` (GetUsage)
- `subscription.go:303` (CheckPlanLimits)

**Impacto:** Inconsistencia si se cambian los defaults.

**Solución:** Definir constante global:
```go
const DefaultLimitsJSON = `{"feeds": 5, "translations_per_day": 0, "search_enabled": false, "alerts_limit": 0}`
```

---

### 7. Índices Creados Sin Verificar Errores

**Archivo:** `backend/cmd/server/main.go:113-121`

**Problema:**
```go
_, err = db.GetPool().Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_users_plan ON users(plan)`)
// ❌ Error verificado pero solo logging, no se retorna
```

**Impacto:** Si falla la creación del índice, el servidor sigue funcionando pero puede tener problemas de performance.

---

## 🟡 PROBLEMAS BAJOS - Frontend

### 8. Frontend TypeScript No Verificado

**Estado:** ⚠️ No se puede verificar sin npm

**Problemas potenciales:**
- Tipos definidos pueden no coincidir con respuestas reales del API
- El código no ha sido probado con `tsc --noEmit`
- Las páginas (OnboardingPage, LoginPage, SettingsPage) no están integradas en App.tsx

### 9. API Client No Expone Todos los Endpoints

**Archivo:** `frontend/src/services/apiClient.ts`

**Faltan:**
- `getFeeds` (listar feeds)
- `createFeed` (crear feed)
- `deleteFeed` (eliminar feed)
- `toggleFeed` (activar/desactivar)
- `search` (búsqueda)
- `getCategories` (categorías)
- `getCountries` (países)

### 10. AppContext No Está Integrado en App.tsx

**Problema:** Las nuevas páginas y el AppProvider no están integrados en el flujo principal de rutas.

**Archivos creados pero no integrados:**
- `frontend/src/services/apiClient.ts`
- `frontend/src/contexts/AppContext.tsx`
- `frontend/src/pages/OnboardingPage.tsx`
- `frontend/src/pages/LoginPage.tsx`
- `frontend/src/pages/SettingsPage.tsx`

---

## ✅ FUNCIONALIDADES CORRECTAS

### Backend

1. **Migración SQL** - Correcta
   - Columnas añadidas correctamente
   - Índices creados
   - Defaults establecidos

2. **Modelo User** - Correcto
   - Campos de suscripción añadidos
   - UserLimits struct definido

3. **Login/Register** - Funcional
   - Devuelve plan y límites
   - Token JWT generado correctamente

4. **CheckPlanLimits** - Corregido después de auditoría
   - Ya no usa columnas inexistentes
   - Manejo de errores añadido

5. **GetUsage** - Corregido después de auditoría
   - Queries corregidas
   - No intenta usar user_id inexistente

### Frontend

1. **apiClient.ts** - Estructura correcta
   - Soporte cloud/local
   - Tipos TypeScript definidos
   - Manejo de errores

2. **ServerConfigManager** - Bien diseñado
   - Cambio dinámico de servidor
   - Persistencia en localStorage
   - Listeners para cambios

---

## 📝 CHECKLIST DE CORRECCIONES

### Backend (Go)

| # | Problema | Gravedad | Status |
|---|----------|----------|--------|
| 1 | Validación receipt falsa | 🔴 CRÍTICO | ❌ Pendiente |
| 2 | subscriptionEnd no usada | 🟠 MEDIO | ❌ Pendiente |
| 3 | Success/Message inconsistentes | 🟠 MEDIO | ❌ Pendiente |
| 4 | Race condition expiración | 🟠 MEDIO | ❌ Pendiente |
| 5 | json.Unmarshal errors ignorados | 🟡 BAJO | ❌ Pendiente |
| 6 | Default limits hardcoded | 🟡 BAJO | ❌ Pendiente |
| 7 | Índices sin verificación | 🟡 BAJO | ❌ Pendiente |

### Frontend (React/TypeScript)

| # | Problema | Gravedad | Status |
|---|----------|----------|--------|
| 8 | TS no verificado | ⚠️ N/A | ❌ Pendiente |
| 9 | Endpoints faltantes en apiClient | 🟡 BAJO | ❌ Pendiente |
| 10 | Pages no integradas en App.tsx | 🟡 BAJO | ❌ Pendiente |

---

## 🔧 PRÓXIMOS PASOS PARA CORREGIR

### 1. Fix Inmediato - Validación de Google Play

Implementar validación real de receipts:
```go
// En subscription.go
func validateGooglePlayReceipt(token string) (bool, error) {
    // Llamar a Google Play Developer API
    // Verificar signature del receipt
    // Retornar si es válido
}
```

### 2. Fix Inmediato - Limpiar Código Muerto

Eliminar `subscriptionEnd` no usada en auth.go:54

### 3. Fix Rápido - Estandarizar Respuestas

Asegurar que todas las respuestas SubscriptionResponse incluyan `Success` y `Message` consistentes.

### 4. Frontend - Integrar y Testear

1. Agregar AppProvider a main.tsx
2. Integrar nuevas páginas en App.tsx
3. Correr `npm install && npm run build`
4. Verificar con `npx tsc --noEmit`

---

## 📁 ARCHIVOS MODIFICADOS/CREADOS

### Backend
```
backend/
├── internal/
│   ├── handlers/
│   │   ├── subscription.go     # NUEVO - 428 líneas
│   │   └── auth.go            # MODIFICADO - añadido campos subscription
│   └── models/
│       └── models.go          # MODIFICADO - añadido UserLimits, campos a User
├── cmd/server/main.go         # MODIFICADO - migraciones auto + rutas
init-db/
└── 56-add-subscription-fields.sql  # NUEVO
```

### Frontend
```
frontend/src/
├── services/
│   └── apiClient.ts           # NUEVO - cliente API modular
├── contexts/
│   └── AppContext.tsx        # NUEVO - estado global
└── pages/
    ├── OnboardingPage.tsx    # NUEVO
    ├── LoginPage.tsx        # NUEVO
    └── SettingsPage.tsx      # NUEVO
```

### Documentación
```
docs/
├── MONETIZATION_PLAN.md      # Plan de monetización
├── AUDIT_SUBSCRIPTION.md     # Auditoría previa
├── ANDROID_SETUP.md          # Setup Android SDK
├── ANDROID_INTEGRATION.md    # Guía de integración
└── REVISION_CODIGO.md       # Este documento
```

---

## ⚠️ NOTA IMPORTANTE

El servidor backend está **corriendo con la versión corregida** después de la auditoría previa. Sin embargo:

1. **Validación de receipts sigue siendo falsa** - Necesita integración real con Google Play
2. **Modelo de datos no soporta multi-tenant** - feeds, alertas, traducciones no tienen user_id
3. **El frontend no está completamente integrado** - Requiere npm y trabajo adicional

---

*Documento generado: 2026-10-12*
*Versión: 1.1*
