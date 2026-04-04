# 🚀 Despliegue RSS2 - 3 Comandos Simples

## ⚡ Flujo Rápido (3 pasos)

```bash
# 1. Generar credenciales seguras
./generate_secure_credentials.sh --force

# 2. Desplegar
docker compose up -d

# 3. Verificar
docker compose ps
```

**¡Listo!** La aplicación estará disponible en `http://localhost:8888`

---

## 🔐 ¿Por qué 3 pasos?

**Problema original:**
```bash
docker compose up -d
WARN: DB_PASS no definida → Contraseña vacía → ¡Inseguro!
```

**Solución:**
1. `./generate_secure_credentials.sh --force` → Crea `.env` con contraseñas de 32 caracteres
2. `docker compose up -d` → Docker carga `.env` automáticamente
3. `docker compose ps` → Verifica que todo esté bien

---

## 📋 Qué hace cada comando

### 1. `./generate_secure_credentials.sh --force`

- Crea contraseñas seguras de 32 caracteres aleatorios
- Genera archivo `.env` con todas las variables necesarias
- Guarda backup del `.env` anterior
- **Tiempo: 5 segundos**

**Salida:**
```
POSTGRES_PASSWORD: S5xwsI9HKjdaBWLpCkqp0mA0DJYZerpD
REDIS_PASSWORD:    cVF79QuJulPO5auEhA2nNqv7mqAruzYh
SECRET_KEY:       2eb7c221245bb8d896f83255188de614e59fae6b8dfbf16eee8a23c60dde4ffa
GRAFANA_PASSWORD: R4J61V6OGLTCesrITwEvUPTm
```

### 2. `docker compose up -d`

- Lee `.env` automáticamente (está configurado en `docker-compose.yml`)
- Inicia todos los servicios en modo detached (background)
- Espera a que estén listos
- **Tiempo: 2-5 minutos** (primera vez que descarga imágenes)

### 3. `docker compose ps`

- Muestra estado de todos los servicios
- Deberías ver `Up` para todos
- **Tiempo: 1 segundo**

**Salida esperada:**
```
Name               Status
rss2_db            Up (healthy)
rss2_redis         Up (healthy)
rss2_backend_go    Up (running)
rss2_nginx         Up (running)
...
```

---

## 🎯 Comandos Útiles

### Verificar credenciales
```bash
grep -E "^(POSTGRES_PASSWORD|REDIS_PASSWORD|DB_PASS)=" .env
```

### Ver logs en tiempo real
```bash
docker compose logs -f
```

### Reiniciar servicios
```bash
# Todos
docker compose restart

# Solo base de datos
docker compose restart db
```

### Verificar estado
```bash
docker compose ps
```

### Limpiar todo (PERDERÁS DATOS)
```bash
docker compose down -v
```

### Escalar workers
```bash
# Más traductores CPU
docker compose up -d --scale translator=3

# Más traductores GPU
docker compose up -d --scale translator-gpu=2
```

---

## 🔒 Seguridad

### Contraseñas por defecto = PELIGRO ❌

```bash
# MAL - Contraseñas vacías
docker compose up -d
WARN: DB_PASS no set
# Las contraseñas están vacías
```

### Contraseñas generadas = SEGURO ✅

```bash
# BIEN
./generate_secure_credentials.sh --force
docker compose up -d
# Contraseñas de 32 caracteres aleatorios
```

### Guardar tus credenciales

**Opción 1: Gestor de contraseñas (Recomendado)**
```bash
# Copia las contraseñas que muestra el script
# Guárdalas en: LastPass, 1Password, KeePass, Bitwarden
```

**Opción 2: Archivo seguro local**
```bash
# Crea un archivo fuera del proyecto
echo "POSTGRES_PASSWORD=..." > ~/rss2-credentials.txt
```

---

## 📊 Estado de los Servicios

| Servicio | Puerto | Descripción |
|----------|--------|-------------|
| `nginx` | 8888 | Web app |
| `backend-go` | 8080 | API |
| `db` | 5432 | PostgreSQL |
| `redis` | 6379 | Cache |
| `qdrant` | 6333 | Vector DB |

**DASHBOARDS:**
- Grafana: http://127.0.0.1:3001
- Prometheus: http://127.0.0.1:9090

---

## 🐛 Solución de Problemas

### WARN sobre contraseñas vacías
```bash
# Solución: Generar credenciales
./generate_secure_credentials.sh --force
```

### Port 8888 en uso
```bash
# Cambiar puerto en nginx
```

### Base de datos no inicia
```bash
# Ver logs
docker compose logs db

# Reiniciar
docker compose restart db
```

### Redis autenticación fallida
```bash
# Ver REDIS_PASSWORD
grep REDIS_PASSWORD .env
```

---

## ✅ Checklist Final

Antes de considerar tu despliegue listo:

- [ ] `.env` tiene contraseñas generadas (no por defecto)
- [ ] `docker compose ps` muestra todos "Up"
- [ ] `http://localhost:8888` abre la app
- [ ] `curl http://localhost:8888/api/health` responde

---

## 📚 Documentación

- [README.md](./README.md) - Documentación completa
- [DEPLOY.md](./DEPLOY.md) - Guía detallada
- [SECURITY_GUIDE.md](./SECURITY_GUIDE.md) - Seguridad

---

**RSS2** - *Transformando noticias en inteligencia con IA* 🚀
