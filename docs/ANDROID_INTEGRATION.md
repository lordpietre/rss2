# RSS2 Android App - Integración de Cliente API Multi-Servidor

## Archivos Creados

### 1. Cliente API (`frontend/src/services/apiClient.ts`)

Cliente API modular que soporta:
- **Modo Cloud**: Se conecta a servidor central (configurable)
- **Modo Local**: Se conecta a servidor local en el teléfono

#### Características:
- Detección automática de servidor local
- Cambio dinámico entre servidores sin reiniciar la app
- Tipos TypeScript completos para todas las respuestas
- Manejo de errores (NO_CONNECTION, TOKEN_EXPIRED)
- Persistencia de configuración en localStorage

#### Uso básico:
```typescript
import { apiClient, serverConfig } from './services/apiClient'

// Verificar estado del servidor
const isLocalAvailable = await serverConfig.checkLocalServer()

// Login
const response = await apiClient.login(email, password)

// Obtener estado de suscripción
const sub = await apiClient.getSubscriptionStatus()
```

### 2. Context Provider (`frontend/src/contexts/AppContext.tsx`)

Estado global de la aplicación que maneja:
- Estado de autenticación
- Configuración del servidor (local vs cloud)
- Información del plan y límites
- Sincronización automática de datos

#### Uso:
```typescript
import { AppProvider, useApp } from './contexts/AppContext'

// En tu App.tsx
function App() {
  return (
    <AppProvider>
      <YourRoutes />
    </AppProvider>
  )
}

// En cualquier componente
function MyComponent() {
  const { 
    serverMode,      // 'cloud' | 'local'
    user,            // Usuario actual
    subscription,   // Info del plan
    login,           // Función de login
    setMode          // Cambiar entre cloud/local
  } = useApp()
}
```

### 3. Página de Onboarding (`frontend/src/pages/OnboardingPage.tsx`)

Pantalla inicial que permite elegir entre:
- **Modo Cloud**: Conexión al servidor central
- **Modo Local**: Servidor desplegado en el teléfono

Incluye:
- Verificación automática del servidor local
- Configuración manual de URL:puerto
- Estado visual de conexión

### 4. Página de Login (`frontend/src/pages/LoginPage.tsx`)

Login/registro con:
- Toggle entre Cloud y Local
- Indicador de servidor actual
- Manejo de errores de conexión
- Info del plan (Cloud)

### 5. Página de Settings (`frontend/src/pages/SettingsPage.tsx`)

Configuración completa:
- Cambio de modo Cloud/Local
- Configuración de URL local
- Info de cuenta y plan
- Suscripción y uso
- Modal de upgrade

---

## Integración Paso a Paso

### Paso 1: Agregar el AppProvider

Modificar `frontend/src/main.tsx`:

```typescript
import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import App from './App'
import { AppProvider } from './contexts/AppContext'
import './index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter>
      <AppProvider>
        <App />
      </AppProvider>
    </BrowserRouter>
  </React.StrictMode>,
)
```

### Paso 2: Actualizar App.tsx

```typescript
import { Routes, Route } from 'react-router-dom'
import { AppProvider, useApp } from './contexts/AppContext'
import { OnboardingPage } from './pages/OnboardingPage'
import { LoginPage } from './pages/LoginPage'
import { SettingsPage } from './pages/SettingsPage'
// ... otras páginas existentes

function AppRoutes() {
  const { isAuthenticated, isLoading } = useApp()
  
  if (isLoading) {
    return <LoadingSpinner />
  }
  
  return (
    <Routes>
      {/* Rutas públicas */}
      <Route path="/onboarding" element={<OnboardingPage />} />
      <Route path="/login" element={<LoginPage />} />
      
      {/* Rutas protegidas */}
      <Route path="/" element={<ProtectedLayout />}>
        <Route index element={<HomePage />} />
        <Route path="settings" element={<SettingsPage />} />
        {/* ... otras rutas */}
      </Route>
    </Routes>
  )
}

function App() {
  return (
    <AppProvider>
      <AppRoutes />
    </AppProvider>
  )
}
```

### Paso 3: Configurar Capacitor

```bash
cd frontend

# Instalar Capacitor
npm install @capacitor/core @capacitor/cli @capacitor/android
npm install @capacitor/payments

# Inicializar
npx cap init RSS2App com.rss2.app --web-dir=dist

# Añadir Android
npx cap add android

# Build y sync
npm run build
npx cap sync android
```

### Paso 4: Configurar URLs de Servidor

Modificar `.env` o configurar en runtime:

```typescript
// Para modo cloud
serverConfig.setCloudUrl('https://tu-dominio.com')

// Para modo local
serverConfig.setLocalUrl('http://localhost', 8080)
```

---

## API Endpoints Utilizados

### Autenticación
| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/api/auth/check-first-user` | ¿Hay usuarios? |
| POST | `/api/auth/login` | Login |
| POST | `/api/auth/register` | Registro |
| GET | `/api/auth/me` | Usuario actual |

### Suscripciones (Cloud only)
| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/api/app/subscription/status` | Estado del plan |
| POST | `/api/app/subscription/validate` | Activar suscripción |
| POST | `/api/app/subscription/cancel` | Cancelar |
| GET | `/api/app/usage` | Uso vs límites |
| GET | `/api/app/limits/check?action=X` | Verificar límite |

### News/Feeds
| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/api/news` | Lista de noticias |
| GET | `/api/news/:id` | Detalle |
| GET | `/api/feeds` | Lista de feeds |
| POST | `/api/feeds` | Crear feed |
| DELETE | `/api/feeds/:id` | Eliminar |
| GET | `/api/search` | Búsqueda |

---

## Plan de Suscripción (Límites)

| Plan | Fuentes | Traducciones/día | Búsqueda | Alertas |
|------|---------|------------------|----------|---------|
| Free | 5 | 0 | ❌ | 0 |
| Pro | 50 | 100 | ✅ | 10 |
| Premium | ∞ | ∞ | ✅ | ∞ |

---

## Estado del Servidor Local

El servidor local debe estar ejecutándose en el teléfono. Para Android:

1. El usuario instala Termux
2. Ejecuta scripts de instalación de PostgreSQL + Redis
3. Corre el servidor Go compilado para ARM64
4. La app se conecta a `http://localhost:8080`

### Verificación de Conexión

```typescript
const isAvailable = await serverConfig.checkLocalServer()
// Returns true si el servidor responde en /health
```

---

## Flujo de Usuario

```
1. Usuario abre la app
   ↓
2. OnboardingPage - elige Cloud o Local
   ↓
3a. Cloud → Login/Register → Home (con plan)
   ↓
3b. Local → Verificar servidor → Login/Register → Home (sin plan)
```

---

## Testing

### Probar Cloud Mode
```bash
# Asegúrate de que el backend esté corriendo
curl http://localhost:8080/health
```

### Probar Local Mode
```bash
# Iniciar servidor local
cd backend && go run ./cmd/server/main.go
# O compilado para ARM64 en Android
```

### Verificar Tipos
```bash
cd frontend
npx tsc --noEmit
```

---

## Documentación Adicional

- `docs/MONETIZATION_PLAN.md` - Plan de monetización completo
- `docs/AUDIT_SUBSCRIPTION.md` - Auditoría de seguridad
- `docs/ANDROID_SETUP.md` - Setup de Android SDK
