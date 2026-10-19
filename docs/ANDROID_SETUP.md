# RSS2 Android App - Setup Instructions

## Prerequisites

1. **Node.js 18+** y **npm** (para Capacitor)
   ```bash
   # Instalar en Ubuntu/Debian
   curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
   sudo apt-get install -y nodejs

   # Verificar
   node --version  # v18.x.x
   npm --version   # 9.x.x
   ```

2. **Android SDK** (para build nativo)
   ```bash
   # Opción 1: Android Studio (recomendado)
   # Descargar desde https://developer.android.com/studio

   # Opción 2: Command line tools
   wget https://dl.google.com/android/repository/commandlinetools-linux-11076708_latest.zip
   unzip commandlinetools-linux-11076708_latest.zip
   mv cmdline-tools /opt/android-sdk/
   export ANDROID_HOME=/opt/android-sdk
   export PATH=$PATH:$ANDROID_HOME/cmdline-tools/latest/bin:$ANDROID_HOME/platform-tools
   ```

3. **Java 17+** (para Android Gradle)
   ```bash
   # Ubuntu/Debian
   sudo apt-get install openjdk-17-jdk
   ```

## Quick Start (Capacitor MVP)

### 1. Añadir Capacitor al Frontend

```bash
cd frontend

# Instalar dependencias de Capacitor
npm install @capacitor/core @capacitor/cli @capacitor/android

# Inicializar Capacitor
npx cap init RSS2App com.rss2.app --web-dir=dist

# Añadir plataforma Android
npx cap add android

# Build del frontend
npm run build

# Sync a Android
npx cap sync android
```

### 2. Configurar Google Play Billing

```bash
npm install @capacitor/payments
```

### 3. Build del APK

```bash
cd android

# Build debug APK
./gradlew assembleDebug

# El APK estará en: app/build/outputs/apk/debug/app-debug.apk
```

## Desarrollo Nativo (Kotlin + Jetpack Compose)

### Estructura del Proyecto

```
android/
├── app/
│   └── src/
│       └── main/
│           ├── java/com/rss2/app/
│           │   ├── MainActivity.kt
│           │   ├── data/
│           │   │   ├── api/
│           │   │   │   └── ApiClient.kt
│           │   │   ├── repository/
│           │   │   │   └── UserRepository.kt
│           │   │   └── model/
│           │   │       └── User.kt
│           │   ├── di/
│           │   │   └── AppModule.kt
│           │   └── ui/
│           │       ├── theme/
│           │       ├── screens/
│           │       └── viewmodels/
│           └── res/
├── build.gradle.kts
└── settings.gradle.kts
```

## Configuración de Google Play

### 1. Crear App en Google Play Console

1. Ve a https://play.google.com/console
2. Crea una nueva app
3. Configura la ficha de Play Store
4. Añade los productos de suscripción:
   - `pro_monthly` - $4.99/mes
   - `premium_monthly` - $9.99/mes

### 2. Integrar Google Play Billing

En `build.gradle`:
```groovy
dependencies {
    implementation 'com.android.billingclient:billing-ktx:6.0.0'
}
```

## Variables de Entorno para el Backend

Asegúrate de que tu backend tiene configuradas las siguientes variables:

```bash
# .env
POSTGRES_PASSWORD=x
REDIS_PASSWORD=x
SECRET_KEY=your-secret-key-here
DATABASE_URL=postgres://rss:x@db:5432/rss?sslmode=disable
```

## API Endpoints para Suscripciones

| Método | Endpoint | Descripción |
|--------|----------|-------------|
| POST | `/api/app/subscription/validate` | Valida receipt de Google Play |
| POST | `/api/app/subscription/cancel` | Cancela suscripción |
| GET | `/api/app/subscription/status` | Estado actual del plan |
| GET | `/api/app/usage` | Uso actual vs límites |
| GET | `/api/app/limits/check?action=X` | Verifica si puede realizar acción |

## Planes y Límites

| Plan | Fuentes | Traducciones/día | Búsqueda | Alertas |
|------|---------|-------------------|----------|---------|
| Free | 5 | 0 | No | No |
| Pro | 50 | 100 | Sí | 10 |
| Premium | ∞ | ∞ | Sí | ∞ |

## Troubleshooting

### Error: `SDK location not found`
```bash
export ANDROID_HOME=/path/to/android-sdk
echo "sdk.dir=$ANDROID_HOME" > android/local.properties
```

### Error: `Capacitor: Google Play Services not available`
Esto es normal en emuladores. El billing solo funciona en dispositivos reales.

### Error: `npm command not found`
```bash
# Instalar Node.js
curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
sudo apt-get install -y nodejs
```
