# RSS2 — Plan de Monetización con App Android

## Resumen Ejecutivo

Crear una app Android que monetice RSS2 ofreciendo dos modalidades de uso:
1. **Modo Cloud**: El usuario se conecta a tu servidor central (suscription-based)
2. **Modo Local**: El usuario despliega un servidor completo en su teléfono (one-time purchase o freemium)

---

## 1. Arquitectura Propuesta

```
┌─────────────────────────────────────────────────────────────────┐
│                         MODO CLOUD                               │
│  ┌─────────────┐     ┌─────────────┐     ┌──────────────────┐  │
│  │  Android    │────▶│  Tu Servidor│────▶│  PostgreSQL      │  │
│  │  App         │     │  (Docker)    │     │  + pgvector      │  │
│  │  (Kotlin)   │     │  :8080       │     │                  │  │
│  └─────────────┘     └─────────────┘     └──────────────────┘  │
│                            │                                        │
│                            ▼                                        │
│                      ┌─────────────┐                              │
│                      │  Redis       │                              │
│                      └─────────────┘                              │
│                                                                  │
│  Monetización: Suscripción mensual/anual                        │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                         MODO LOCAL                               │
│  ┌─────────────┐     ┌─────────────────────────────┐           │
│  │  Android    │────▶│  Servidor embebido          │           │
│  │  App         │     │  (Todo en el teléfono)       │           │
│  │  (Kotlin)   │     │  ┌─────────┐ ┌───────────┐  │           │
│  │             │     │  │Backend  │ │PostgreSQL │  │           │
│  │  Puerto     │     │  │Go :8080 │ │SQLite or  │  │           │
│  │  localhost  │     │  └─────────┘ │PostgreSQL │  │           │
│  └─────────────┘     │  ┌─────────┐ │Mini       │  │           │
│                       │  │Redis    │ └───────────┘  │           │
│                       │  │Lite     │                 │           │
│                       │  └─────────┘                 │           │
│                       └─────────────────────────────┘           │
│                                                                  │
│  Monetización: Compra única o version gratuita limitada         │
└─────────────────────────────────────────────────────────────────┘
```

---

## 2. Stack Tecnológico para Android

### Cliente Android
- **Lenguaje**: Kotlin
- **UI**: Jetpack Compose (modermo, declarativo)
- **Arquitectura**: MVVM + Clean Architecture
- **DI**: Hilt (Dagger)
- **Networking**: Retrofit + OkHttp
- **Persistencia local**: Room (caché)
- **State Management**: StateFlow / Compose State
- **Navegación**: Jetpack Navigation Compose
- **WebSocket**: OkHttp WebSocket (para remote workers)
- **Publishing**: Google Play Store

### Servidor Android Embebido (Modo Local)
- **Backend Go**: Compilado como binary ARM64/Android
- **Base de datos**: PostgreSQL portable o SQLite con pgvectorlite
- **Redis**: Valkey o Redis menor (posible compilación estática)
- **Inicio**: El app lanza el servidor en background service

### Alternativa: wrappers
- **Capacitor/Cordova**: Envolver el frontend React existente
  - ✅ Reusa todo el frontend existente
  - ❌ Menor rendimiento, experiencia menos nativa
  - ❌ No permite servidor embebido easily

### Decisión recomendada
**Desarrollar cliente Kotlin nativo** que consuma la API REST existente del backend.

---

## 3. Modalidad Cloud (Suscripciones)

### Funcionamiento
1. El usuario descarga la app y crea cuenta
2. Selecciona plan (Free / Pro / Premium)
3. La app se conecta a tu servidor via `https://api.tusrss.com`
4. Todos los datos se almacenan en tu infraestructura

### Estructura de Planes

| Característica          | Free      | Pro ($4.99/mes) | Premium ($9.99/mes) |
|-------------------------|-----------|-----------------|---------------------|
| Fuentes RSS             | 5         | 50              | Ilimitadas          |
| Traducciones            | No        | 100/día         | Ilimitadas          |
| Búsqueda semántica      | No        | Sí              | Sí                  |
| Alertas                 | No        | 10              | Ilimitadas          |
| Sin anuncios             | No        | Sí              | Sí                  |
| Soporte                 | No        | Email           | Prioridad           |

### Implementación
- **Auth**: JWT existente del backend (extender con refresh tokens)
- **Pagos**: Google Play Billing Library (subscriptions)
- **Gestión de usuarios**: Usar tabla `users` existente, añadir `subscription_status`, `plan`, `subscription_end`
- **Límites**: Implementar rate limiting por plan en el backend

### API endpoints a añadir/modificar
```go
// Nuevos endpoints
POST   /api/app/register      // Registro con plan
POST   /api/app/subscription  // Webhook de Google Play
GET    /api/app/plan          // Plan actual del usuario
POST   /api/app/cancel        // Cancelar suscripción
```

---

## 4. Modalidad Local (Servidor en el Teléfono)

### Funcionamiento
1. El usuario descarga la app
2. Selecciona "Desplegar servidor local"
3. La app descarga/instala los componentes necesarios:
   - Backend Go compilado para ARM64
   - PostgreSQL Mini (o SQLite con extensiones)
   - Redis (versión ligera)
4. El servidor arranca como foreground service
5. La app se conecta a `http://localhost:8080`

### Componentes a portar

#### Opción A: Compilación nativa para Android
```bash
# Compilar backend Go para Android ARM64
GOOS=linux GOARCH=arm64 CGO_ENABLED=1 \
  go build -o rss2-backend-android ./backend/cmd/server

# Requiere:
# - PostgreSQL compilado para Android (complicado)
# - Redis compilado para Android (complicado)
```

#### Opción B: Usar Termux como runtime
- La app detecta si Termux está instalado
- Si no, guía al usuario a instalarlo
- Usa scripts para instalar paquetes via `pkg install postgresql redis`
- Ejecuta los workers via `termux-exec`

#### Opción C: Binarios pre-compilados (RECOMENDADA)
```bash
# Descargar binarios pre-compilados para ARM64
# - postgresql-mobile (PaaS)
# - redis-android
# - backend-go-android

# Estructura de directorios en el teléfono:
# /data/data/com.rss2.app/
#   ├── bin/
#   │   ├── backend
#   │   ├── postgres
#   │   └── redis
#   ├── data/
#   │   ├── pgdata/
#   │   └── redis-data/
#   └── logs/
```

#### Opción D: Container runtime (UserLAnd / Linux Deploy)
- El usuario instala Linux en su Android
- Luego instala RSS2 como si fuera un VPS
- La app se conecta al servidor Linux

### Desafíos técnicos del modo local

| Componente | Desafío | Solución propuesta |
|------------|---------|-------------------|
| PostgreSQL | Android no soporta PostgreSQL nativo | Usar **SQLite** + extensión **sqlite-vss** (vector search) o **PostgreSQL portable** |
| pgvector | No disponible en SQLite |sqlite-vss para embeddings o deshabilitar búsqueda semántica en modo local |
| Redis | Android no tiene Redis nativo | Usar **freeredis** o **badger** (DB de key-value de Dgraph) |
| CPU/RAM | Teléfonos limitados | Workers más lentos, batch sizes reducidos |
| Almacenamiento | Espacio limitado | Límite de noticias stored (ej: 10,000) |
| Batería | Workers consume batería | Modo " background refresh" configurable |
| Red | IP dinámica | Descubrimiento local via **mDNS** o **Hardcoded localhost** |

### Modelo de monetización modo local

| Versión | Precio | Limitaciones |
|---------|--------|--------------|
| Local Free | Gratis | Máx 3 fuentes, sin traducción, con ads |
| Local Pro | $9.99 (one-time) | Ilimitadas fuentes, traducción, sin ads |
| Local Premium | $19.99 (one-time) | Todo + exportación + widgets |

---

## 5. Base de datos para modo local

### Opción: SQLite + sqlite-vss

```sql
--Noticias (equivalente simplificado)
CREATE TABLE noticias (
    id TEXT PRIMARY KEY,
    titulo TEXT,
    resumen TEXT,
    url TEXT UNIQUE,
    fecha TIMESTAMP,
    fuente_nombre TEXT,
    lang TEXT,
    contenido TEXT,
    imagen_url TEXT
);

--Feeds
CREATE TABLE feeds (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    nombre TEXT,
    url TEXT UNIQUE,
    activo INTEGER DEFAULT 1
);

--Traducciones (simplificado, sin pgvector)
CREATE TABLE traducciones (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    noticia_id TEXT,
    lang_to TEXT,
    titulo_trad TEXT,
    resumen_trad TEXT,
    status TEXT DEFAULT 'pending',
    FOREIGN KEY (noticia_id) REFERENCES noticias(id)
);

--Users (local)
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    username TEXT,
    is_admin INTEGER DEFAULT 0
);

--Índices para búsqueda full-text
CREATE VIRTUAL TABLE noticias_fts USING fts5(titulo, resumen, contenido);
```

### Cambios en el backend para soportar SQLite

Crear un nuevo "driver" en `backend/internal/db/` que soporte tanto PostgreSQL como SQLite:

```go
// backend/internal/db/database.go
type Database interface {
    GetNews(opts NewsOptions) ([]News, error)
    Search(query string) ([]News, error)
    // ... otros métodos
}

type PostgresDB struct { /* ... */ }
type SQLiteDB struct { /* ... */ }
```

---

## 6. Estrategia de Monetización Completa

### Flujo de usuario

```
Usuario instala app
       │
       ▼
┌──────────────────┐
│  Onboarding      │
│  "Cloud" vs      │
│  "Local"         │
└────────┬─────────┘
         │
    ┌────┴────┐
    ▼         ▼
┌────────┐ ┌────────┐
│ Cloud  │ │ Local  │
│ Mode   │ │ Mode   │
└───┬────┘ └───┬────┘
    │          │
    ▼          ▼
┌────────┐ ┌────────────┐
│Login/  │ │Setup       │
│Register│ │Server      │
└───┬────┘ └─────┬──────┘
    │            │
    ▼            ▼
┌─────────────────────────┐
│   Selección de Plan     │
│  Free / Pro / Premium   │
└─────────────────────────┘
```

### Modelo de negocio

#### Cloud (B2C SaaS)
- **Free tier**: Atraer usuarios, upsell a Pro
- **Pro/Premium**: Ingreso recurrente mensual/anual
- **Costo estimado por usuario**: ~$0.50-1/mes (server costs)
- **Margen**: 70-80%

#### Local (B2C One-time)
- **Local Pro**: $9.99 - Compra única
- **Local Premium**: $19.99 - Compra única
- **Revenue share Google Play**: 15% (primer $1M), 30% después

#### Fuentes de ingreso adicionales
1. **Anuncios** en versión free (AdMob)
2. **Affiliate feeds**: Promocionar feeds premium de terceros
3. **Data insights**: Vender análisis agregados (anonimizados) a medios

---

## 7. Plan de Desarrollo por Fases

### Fase 1: Cliente Android Básico (Semanas 1-4)
**Objetivo**: App Android que se conecte al backend cloud existente

- [ ] Configurar proyecto Android con Kotlin + Jetpack Compose
- [ ] Implementar cliente API (conectar a API existente)
- [ ] Pantallas básicas: Login, Register, Home, News List, News Detail
- [ ] Autenticación JWT funcional
- [ ] Búsqueda básica
- [ ] Integración Google Sign-In (opcional)

**Entregable**: APK que funciona contra el servidor cloud

### Fase 2: Suscripciones Cloud (Semanas 5-7)
**Objetivo**: Monetización del modo cloud

- [ ] Integrar Google Play Billing Library
- [ ] Crear productos de suscripción (Free, Pro, Premium)
- [ ] Modificar backend para manejar planes y límites
- [ ] Implementar paywall / upgrade flow
- [ ] Webhook handler para Google Play
- [ ] Gestión de suscripción cancelaciones

**Entregable**: App con suscripciones funcionales

### Fase 3: Servidor Local (Semanas 8-12)
**Objetivo**: Modo local desplegado en el teléfono

- [ ] Compilar backend Go para Android ARM64
- [ ] Crear servicio de instalación de dependencias
- [ ] Implementar PostgreSQL portable o SQLite con FTS
- [ ] Implementar Redis ligero o Badger
- [ ] Frontend service para gestionar servidor
- [ ] Descubrimiento de servidor local
- [ ] Sincronización de datos (import/export)

**Entregable**: App que puede desplegar servidor local

### Fase 4: Versión Local Monetizada (Semanas 13-15)
**Objetivo**: Monetización del modo local

- [ ] Integrar Google Play Billing para compras one-time
- [ ] Implementar limitaciones para versión Free
- [ ] Pantalla de upgrade/paywall
- [ ] Eliminar ads en versión Pro
- [ ] Testing completo en múltiples dispositivos

**Entregable**: App con monetization local funcional

### Fase 5: Polish & Launch (Semanas 16-20)
**Objetivo**: Preparar para Google Play

- [ ] Testing beta con usuarios selectos
- [ ] Optimizar rendimiento y batería
- [ ] Crear assets de Play Store (screenshots, descripción)
- [ ] Configurar Remote Config para feature flags
- [ ] Analytics (Firebase Analytics)
- [ ] Crash reporting (Firebase Crashlytics)
- [ ] Publicar en Google Play

---

## 8. Cambios Requeridos en el Backend

### Nuevo schema (añadir a `users`)

```sql
-- Añadir a la tabla users existente
ALTER TABLE users ADD COLUMN plan VARCHAR(20) DEFAULT 'free';
ALTER TABLE users ADD COLUMN subscription_status VARCHAR(20) DEFAULT 'none';
ALTER TABLE users ADD COLUMN subscription_end TIMESTAMP;
ALTER TABLE users ADD COLUMN google_play_token TEXT;
ALTER TABLE users ADD COLUMN limits_json JSONB;
```

### Rate limiting por plan

```go
// backend/internal/middleware/ratelimit.go
var planLimits = map[string]int{
    "free":     100,    // requests per minute
    "pro":      1000,
    "premium":  unlimited,
}

func RateLimitMiddleware() gin.HandlerFunc {
    // Implementar límite según plan del usuario
}
```

### Nuevos endpoints

```go
// Cloud subscription management
POST   /api/app/subscription/validate     // Validar receipt de Google Play
POST   /api/app/subscription/cancel        // Cancelar
GET    /api/app/usage                      // Usage actual vs límites
POST   /api/app/subscription/upgrade       // Upgrade plan

// Local mode
POST   /api/local/setup                     // Inicializar servidor local
GET    /api/local/status                    // Estado del servidor local
POST   /api/local/backup                   // Exportar datos
POST   /api/local/restore                  // Importar datos
```

---

## 9. Costos Estimados

### Desarrollo
- **Desarrollador Android**: 20 semanas × 40h = 800h
- **Desarrollador Backend adicional**: 4 semanas × 40h = 160h (extensiones backend)

### Infraestructura Cloud (estimación)
| Recurso | Free Tier | Por usuario adicional |
|---------|-----------|----------------------|
| Server (4 vCPU, 8GB) | $20/mes | Sirve ~100 usuarios |
| PostgreSQL (4GB) | Incluido | - |
| Redis (512MB) | $5/mes | - |
| Ancho de banda | $10/mes | $0.10/GB |
| **Total por ~100 usuarios** | ~$35/mes | ~$0.35/usuario |

### Costos de app store
- **Google Play**: $25 una vez
- **Apple Developer**: $99/año (si iOS)

---

## 10. Riesgos y Mitigaciones

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|--------------|---------|------------|
| Google Play reject | Media | Alto | Revisar políticas de subscriptions, usar billing library oficial |
| Usuarios local no pagan | Alta | Medio | Limitar features free severamente |
| Performance mobile bajo | Media | Medio | Optimizar queries, caching agresivo |
| Modelos ML muy pesados | Alta | Alto | Deshabilitar NER/embeddings en local o usar versiones tiny |
| Compatibilidad Android fragmentada | Media | Medio | Min SDK 26 (Android 8), testear en emuladores |
| Servidor local consume batería | Alta | Bajo | Permitir configuración de refresh interval |

---

## 11. Recomendaciones Finales

### Prioridad de desarrollo
1. **Empezar por Cloud**: Es más simple de monetizar (suscripciones = revenue recurrente)
2. **Reusar frontend React existente**: Considerar Capacitor para MVP rápido
3. **Modo local como segundo producto**: Más complejo, pero diferenciador

### Alternativa rápida (Capacitor)
Si quieres un MVP en 2-3 semanas:
```bash
# 1. Instalar Capacitor en el frontend existente
npm install @capacitor/core @capacitor/cli @capacitor/android
npx cap init RSS2App com.rss2.app
npx cap add android

# 2. Build y sync
npm run build
npx cap sync android

# 3. Resultado: APK funcional
```
- ✅ Rápido de implementar
- ✅ Reusa todo el frontend
- ❌ No permite servidor local (solo cloud)
- ❌ Menos experiencia nativa

### Siguiente paso recomendado
Comenzar con un **MVP de Cloud + Capacitor** para validar el modelo de negocio, luego migrar a app nativa si hay traction.

---

## 12. Checklist de Implementación

### Backend
- [ ] Añadir campos `plan`, `subscription_status` a `users`
- [ ] Implementar rate limiting por plan
- [ ] Crear endpoint `/api/app/subscription/validate`
- [ ] Crear webhook receiver para Google Play
- [ ] Añadir logging de uso por usuario

### Android (Capacitor MVP)
- [ ] Instalar Capacitor en frontend
- [ ] Configurar Android project
- [ ] Implementar Google Sign-In
- [ ] Integrar Google Play Billing
- [ ] Crear paywall UI
- [ ] Build APK

### Android (Nativo Completo)
- [ ] Crear proyecto Kotlin + Compose
- [ ] Diseñar arquitectura Clean Architecture
- [ ] Implementar cliente API
- [ ] Crear todas las pantallas
- [ ] Integrar Play Billing
- [ ] Implementar modo local server
- [ ] Testing y polish

---

*Documento creado: $(date)*
*Versión: 1.0*

## Estado de Implementación

### ✅ Completado (Backend)

- [x] **Migración SQL** (`56-add-subscription-fields.sql`)
  - Añade columnas: `plan`, `subscription_status`, `subscription_end`, `google_play_token`, `google_play_product_id`, `limits_json`
  - Índices para consultas por plan/status

- [x] **Modelo User actualizado** (`backend/internal/models/models.go`)
  - Nuevos campos: `Plan`, `SubscriptionStatus`, `SubscriptionEnd`, `GooglePlayProductID`, `Limits`
  - Nueva estructura `UserLimits` con límites por plan

- [x] **Handler de Suscripciones** (`backend/internal/handlers/subscription.go`)
  - `ValidateSubscriptionReceipt` - Valida receipts de Google Play
  - `CancelSubscription` - Cancela suscripción
  - `GetSubscriptionStatus` - Obtiene estado actual del plan
  - `GetUsage` - Estadísticas de uso
  - `CheckPlanLimits` - Verifica si puede realizar una acción

- [x] **Auth Handlers actualizados** (`backend/internal/handlers/auth.go`)
  - Login/Register/GetCurrentUser devuelven info de plan y límites
  - Nuevos usuarios empiezan con plan `free`

- [x] **Migraciones auto** (`backend/cmd/server/main.go`)
  - Las columnas se crean automáticamente al iniciar el servidor
  - Índices creados automáticamente

- [x] **Rutas API** (`backend/cmd/server/main.go`)
  - `/api/app/subscription/*` - Gestión de suscripciones
  - `/api/app/usage` - Uso actual
  - `/api/app/limits/check` - Verificación de límites

### 📋 Pendiente (Android)

- [ ] **Capacitor**: Instalar y configurar `@capacitor/core`, `@capacitor/cli`, `@capacitor/android`
- [ ] **Google Play Billing**: Integrar librería de pagos
- [ ] **UI de Paywall**: Pantalla para mostrar planes y upgrade
- [ ] **Verificación de Receipts**: Implementar validación de purchases
- [ ] **Rate Limiting UI**: Mostrar cuando se alcanzan límites

### 🚀 Próximos Pasos

1. **Backend listo** - Ya se puede testar con curl:
   ```bash
   # Registro
   curl -X POST http://localhost:8080/api/auth/register \
     -H "Content-Type: application/json" \
     -d '{"email":"test@test.com","username":"test","password":"test123"}'

   # Ver estado de suscripción (necesita token)
   curl http://localhost:8080/api/app/subscription/status \
     -H "Authorization: Bearer <token>"
   ```

2. **Android**: Seguir instrucciones en `docs/ANDROID_SETUP.md`

3. **Google Play Console**: Configurar productos de suscripción
