# 🚀 Guía de Despliegue RSS2

## ⚡ Despliegue Rápido (5 minutos)

### Paso 1: Clonar el Proyecto

```bash
git clone https://github.com/tu-usuario/rss2.git
cd rss2
```

### Paso 2: Generar Credenciales Seguras

```bash
./pre-deploy.sh --generate
```

**Salida esperada:**
```
🔑 Generando credenciales seguras...

⚠️  IMPORTANTE: Guarda estas credenciales en un lugar seguro

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
POSTGRES_PASSWORD: S5xwsI9HKjdaBWLpCkqp0mA0DJYZerpD
REDIS_PASSWORD:    cVF79QuJulPO5auEhA2nNqv7mqAruzYh
SECRET_KEY:        2eb7c221245bb8d896f83255188de614e59fae6b8dfbf16eee8a23c60dde4ffa
GRAFANA_PASSWORD:  R4J61V6OGLTCesrITwEvUPTm
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

**⚠️ IMPORTANTE:** Copia estas contraseñas y guárdalas en un gestor de contraseñas. ¡Nunca se volverán a mostrar!

### Paso 3: Validar y Desplegar

```bash
./pre-deploy.sh
```

**El script te preguntará:**
```
¿Deseas desplegar ahora?
   1) Desplegar automáticamente (recomendado)
   2) Solo validar y salir (modo manual)
   Elige una opción:
```

**Opción A: Despliegue automático (Opción 1)**
- Elige `1` para que el script haga todo automáticamente
- Espera a que termine la construcción de las imágenes
- Espera a que inicien los servicios

**Opción B: Despliegue manual (Opción 2)**
- Elige `2` para validar solo
- Luego ejecuta manualmente:
  ```bash
  docker compose up -d
  ```

### Paso 4: Verificar Despliegue

```bash
# Verificar estado de los servicios
docker compose ps

# Deberías ver algo como:
# Name                    Status
# rss2_db                Up (healthy)
# rss2_redis             Up (healthy)
# rss2_backend_go        Up (running)
# ...
```

### Paso 5: Acceder a la Aplicación

```bash
# Aplicación web
http://localhost:8888

# API (prueba con curl)
curl http://localhost:8888/api/feeds
```

---

## 🔐 Seguridad

### ¿Por qué es importante?

**Sin seguridad (❌ MAL):**
```bash
docker compose up -d
WARN: The "DB_PASS" variable is not set. Defaulting to a blank string.
# Las contraseñas están vacías ¡PELIGRO!
```

**Con seguridad (✅ BIEN):**
```bash
./pre-deploy.sh --generate
./pre-deploy.sh
docker compose up -d
# ✅ Todo funciona correctamente
```

### Variables Críticas

| Variable | Qué es | Por qué es importante |
|----------|--------|----------------------|
| `POSTGRES_PASSWORD` | Contraseña de PostgreSQL | Base de datos principal |
| `REDIS_PASSWORD` | Contraseña de Redis | Cache y colas |
| `DB_PASS` | Contraseña para workers | Conexiones de workers |
| `SECRET_KEY` | Key secreta de la app | JWT, encriptación |
| `GRAFANA_PASSWORD` | Contraseña de Grafana | Dashboard de monitoring |

### Guardar tus Credenciales

**Opción 1: Gestor de contraseñas (Recomendado)**
```bash
# Copia las credenciales y guárdalas en:
# - LastPass
# - 1Password
# - KeePass
# - Bitwarden
```

**Opción 2: Archivo seguro local**
```bash
# Crea un archivo seguro fuera del proyecto
echo "POSTGRES_PASSWORD=..." > ~/rss2-credentials.txt
echo "REDIS_PASSWORD=..." >> ~/rss2-credentials.txt
```

---

## 🛠️ Comandos Útiles

### Verificar Credenciales
```bash
# Ver que las contraseñas están definidas
grep -E "^(POSTGRES_PASSWORD|REDIS_PASSWORD|DB_PASS)=" .env
```

### Reiniciar Servicios
```bash
# Reiniciar todos los servicios
docker compose restart

# Reiniciar solo base de datos
docker compose restart db
```

### Escalar Workers
```bash
# Añadir más traductores (CPU)
docker compose up -d --scale translator=3

# Añadir más traductores (GPU)
docker compose up -d --scale translator-gpu=2
```

### Ver Logs
```bash
# Ver logs en tiempo real
docker compose logs -f

# Ver logs específicos
docker compose logs -f db
docker compose logs -f redis
docker compose logs -f backend-go
```

### Limpiar y Reiniciar
```bash
# Detener y eliminar todo (incluye datos)
docker compose down -v

# Eliminar solo contenedores (mantiene datos)
docker compose down

# Eliminar volumen específico
docker volume rm rss2_db
docker volume rm rss2_redis
```

---

## 🐛 Solución de Problemas

### Problema: WARN sobre variables no definidas

```
WARN: The "DB_PASS" variable is not set. Defaulting to a blank string.
```

**Solución:**
```bash
# Ejecutar pre-deploy para generar credenciales
./pre-deploy.sh --generate
```

### Problema: Contraseña vacía en healthcheck

```
redis-cli: Authentication failed
```

**Solución:**
```bash
# Verificar REDIS_PASSWORD
grep REDIS_PASSWORD .env

# Si está vacío, regenerar
./pre-deploy.sh --generate
```

### Problema: Port 8888 en uso

```
ERROR: failed to start service: port 8888 already in use
```

**Solución:**
```bash
# Cambiar puerto en docker-compose.yml
# O matar el proceso que usa el puerto
lsof -ti:8888 | xargs kill

# O usar otro puerto
docker compose up -d --scale nginx=1
```

### Problema: Base de datos no inicia

```
postgres: cannot connect to database
```

**Solución:**
```bash
# Ver logs de la base de datos
docker compose logs db

# Reiniciar base de datos
docker compose restart db

# Si persiste, eliminar y recrear (¡PERDERÁS DATOS!)
docker compose down -v
docker compose up -d db
```

---

## 📊 Estado de los Servicios

### Servicios Críticos

| Servicio | Puerto | Descripción |
|----------|--------|-------------|
| `nginx` | 8888 | Aplicación web |
| `backend-go` | 8080 | API interna |
| `db` | 5432 | PostgreSQL |
| `redis` | 6379 | Redis cache |
| `qdrant` | 6333 | Vector database |

### Dashboard de Monitorización

| Herramienta | Puerto | Acceso |
|-------------|--------|--------|
| Grafana | 3001 | http://127.0.0.1:3001 |
| Prometheus | 9090 | http://127.0.0.1:9090 |

**Nota:** Accede a Grafana desde `127.0.0.1` (localhost) por seguridad.

---

## 🔄 Actualización (Upgrade)

### Actualizar a Nueva Versión

```bash
# 1. Pull las nuevas imágenes
docker compose pull

# 2. Backup de la base de datos
docker compose exec db pg_dump -U rss rss > backup.sql

# 3. Parar servicios
docker compose down

# 4. Actualizar código
git pull

# 5. Reiniciar
docker compose up -d

# 6. Verificar
curl http://localhost:8888/api/health
```

### Migrar Contraseñas

Si tienes credenciales existentes y quieres mantenerlas:
```bash
# 1. Verificar .env existente
grep POSTGRES_PASSWORD .env

# 2. Si quieres nuevas credenciales
./pre-deploy.sh --generate

# 3. Si quieres mantener las existentes
# No hagas nada, las credenciales seguirán funcionando
```

---

## 📝 Archivos Importantes

| Archivo | Propósito |
|---------|-----------|
| `.env` | Contraseñas y configuración (NO subir a Git) |
| `.env.example` | Plantilla (SÍ subir a Git) |
| `pre-deploy.sh` | Script de validación (SÍ subir a Git) |
| `generate_secure_credentials.sh` | Script de generación (SÍ subir a Git) |
| `docker-compose.yml` | Configuración de Docker (SÍ subir a Git) |

---

## ✅ Checklist Final

Antes de considerar tu despliegue como "listo", verifica:

- [ ] `.env` tiene credenciales generadas (no por defecto)
- [ ] `.env` está en `.gitignore`
- [ ] Puedes acceder a http://localhost:8888
- [ ] `docker compose ps` muestra todos los servicios "Up"
- [ ] La base de datos muestra "healthy"
- [ ] Los logs de la base de datos no tienen errores
- [ ] Redis responde a ping
- [ ] Qdrant está funcionando

**¡Listo! Tu RSS2 está desplegado y funcionando.** 🎉

---

## 📚 Documentación Adicional

- [README.md](./README.md) - Documentación general
- [SECURITY_GUIDE.md](./SECURITY_GUIDE.md) - Guía de seguridad
- [DOCKER.md](./DOCKER.md) - Configuración avanzada de Docker

---

**RSS2** - *Transformando noticias en inteligencia con IA.*
