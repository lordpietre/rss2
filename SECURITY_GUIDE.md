# 🔒 GUÍA DE SEGURIDAD Y MIGRACIÓN - RSS2 Application

## ⚠️ RESUMEN DE VULNERABILIDADES ENCONTRADAS

### CRÍTICAS (Arreglar INMEDIATAMENTE)

1. **Credenciales débiles en .env**
   - PostgreSQL password: `x`
   - Flask SECRET_KEY: `secret`
   - Grafana password: `admin`
   
2. **Servicios expuestos públicamente sin autenticación**
   - Qdrant (puertos 6333, 6334) - Base de datos vectorial
   - Prometheus (puerto 9090) - Métricas del sistema
   - cAdvisor (puerto 8081) - Estadísticas de contenedores
   
3. **Redis sin autenticación**
   - Accesible por todos los contenedores sin password

### ALTO RIESGO

4. **Ausencia de segmentación de red**
   - Todos los servicios en una única red Docker
   
5. **Ausencia de límites de recursos**
   - Contenedores sin límites de CPU/memoria (riesgo de DoS)

6. **Volúmenes con permisos excesivos**
   - Código fuente montado con permisos de escritura

---

## 🛠️ SOLUCIONES IMPLEMENTADAS

### 1. Archivo `docker-compose.secure.yml`

**Mejoras de seguridad implementadas:**

#### 🔹 Redes Segmentadas
```yaml
networks:
  frontend:     # Solo nginx y rss2_web
  backend:      # BD, workers, redis, qdrant (interna)
  monitoring:   # Prometheus, Grafana, cAdvisor (interna)
```

#### 🔹 Puertos Internalizados
- ❌ **Eliminados puertos públicos de:**
  - Qdrant (6333, 6334) → Solo acceso interno
  - Prometheus (9090) → Solo acceso interno
  - cAdvisor (8081) → Solo acceso interno
  - rss-web-go (8002) → Servicio comentado (duplicado)

- ✅ **Único puerto público:**
  - Nginx (8001) → Proxy reverso con seguridad

- ✅ **Puerto localhost únicamente:**
  - Grafana (127.0.0.1:3001) → Acceso solo local o via túnel SSH

#### 🔹 Autenticación en Redis
```yaml
redis:
  command: >
    redis-server
    --requirepass ${REDIS_PASSWORD}
```

#### 🔹 Límites de Recursos
Todos los contenedores tienen límites de CPU y memoria:
```yaml
deploy:
  resources:
    limits:
      cpus: '2'
      memory: 2G
    reservations:
      memory: 512M
```

#### 🔹 Volúmenes Read-Only
Código fuente montado en modo lectura donde sea posible:
```yaml
volumes:
  - ./app.py:/app/app.py:ro
  - ./routers:/app/routers:ro
  - ./templates:/app/templates:ro
```

---

## 📋 PASOS DE MIGRACIÓN

### Opción A: Migración Gradual (RECOMENDADO para producción)

#### Paso 1: Generar Credenciales Seguras

```bash
# 1. Generar password para PostgreSQL
POSTGRES_PASSWORD=$(openssl rand -base64 32)
echo "POSTGRES_PASSWORD=$POSTGRES_PASSWORD"

# 2. Generar password para Redis
REDIS_PASSWORD=$(openssl rand -base64 32)
echo "REDIS_PASSWORD=$REDIS_PASSWORD"

# 3. Generar SECRET_KEY para Flask
SECRET_KEY=$(python3 -c "import secrets; print(secrets.token_hex(32))")
echo "SECRET_KEY=$SECRET_KEY"

# 4. Generar password para Grafana
GRAFANA_PASSWORD=$(openssl rand -base64 24)
echo "GRAFANA_PASSWORD=$GRAFANA_PASSWORD"

# Guardar estos valores en un lugar seguro (gestor de passwords)
```

#### Paso 2: Copiar y Configurar .env Seguro

```bash
# Copiar el ejemplo seguro
cp .env.secure.example .env

# Editar .env y pegar las contraseñas generadas
nano .env  # o usa tu editor preferido
```

#### Paso 3: Backup de Datos

```bash
# Backup de PostgreSQL
docker exec rss2_db pg_dump -U rss rss > backup_$(date +%Y%m%d_%H%M%S).sql

# Backup de Qdrant
tar -czf qdrant_backup_$(date +%Y%m%d_%H%M%S).tar.gz qdrant_storage/

# Backup de Redis (opcional)
docker exec rss2_redis redis-cli --rdb /data/dump.rdb
cp redis-data/dump.rdb redis_backup_$(date +%Y%m%d_%H%M%S).rdb
```

#### Paso 4: Detener Servicios Actuales

```bash
docker-compose down
```

#### Paso 5: Migrar a Configuración Segura

```bash
# Renombrar archivo actual (backup)
mv docker-compose.yml docker-compose.yml.insecure.bak

# Usar la versión segura
cp docker-compose.secure.yml docker-compose.yml

# Verificar configuración
docker-compose config
```

#### Paso 6: Iniciar con Nueva Configuración

```bash
# Iniciar servicios
docker-compose up -d

# Verificar logs
docker-compose logs -f

# Verificar que todos los contenedores están corriendo
docker-compose ps
```

#### Paso 7: Verificar Conectividad

```bash
# Test web app
curl http://localhost:8001

# Test Redis (desde dentro de un contenedor)
docker exec rss2_web bash -c 'python3 -c "import redis; r = redis.Redis(host=\"redis\", port=6379, password=\"$REDIS_PASSWORD\"); print(r.ping())"'

# Verificar logs de workers
docker-compose logs rss2_tasks_py | tail -20
```

---

### Opción B: Migración Directa (Para desarrollo/testing)

```bash
# 1. Backup de datos (como en Opción A, Paso 3)

# 2. Detener todo
docker-compose down -v  # CUIDADO: -v elimina volúmenes

# 3. Generar credenciales y configurar .env
cp .env.secure.example .env
# Editar .env con credenciales generadas

# 4. Restaurar datos si es necesario
# (Restaurar dump SQL, qdrant_storage, etc.)

# 5. Iniciar con configuración segura
cp docker-compose.secure.yml docker-compose.yml
docker-compose up -d
```

---

## 🔐 ACCESO A SERVICIOS PROTEGIDOS

### Grafana (Monitoring)

Ahora solo accesible en localhost. Para acceso remoto:

```bash
# Opción 1: Túnel SSH (RECOMENDADO)
ssh -L 3001:localhost:3001 usuario@servidor

# Luego acceder en tu navegador local:
# http://localhost:3001
# Usuario: admin
# Password: El que configuraste en GRAFANA_PASSWORD
```

### Qdrant (Base de Datos Vectorial)

Ya no es accesible públicamente. Para acceso de desarrollo:

```bash
# Opción 1: Temporalmente exponer puerto (SOLO para debug)
# Editar docker-compose.yml y descomentar:
#   ports:
#     - "127.0.0.1:6333:6333"

# Opción 2: Acceder desde dentro de la red Docker
docker exec -it rss2_qdrant_worker bash
curl http://qdrant:6333/collections
```

### Prometheus (Métricas)

```bash
# Acceso via túnel SSH
ssh -L 9090:localhost:9090 usuario@servidor

# O exponer temporalmente en localhost:
# En docker-compose.yml, prometheus service:
# ports:
#   - "127.0.0.1:9090:9090"
```

---

## 🧪 TESTING DE SEGURIDAD

### Verificar que puertos NO son accesibles públicamente:

```bash
# Desde FUERA del servidor (desde tu máquina local)

# Estos NO deberían responder:
curl http://servidor:6333  # Qdrant - debe fallar
curl http://servidor:9090  # Prometheus - debe fallar
curl http://servidor:8081  # cAdvisor - debe fallar

# Este SÍ debe responder:
curl http://servidor:8001  # Nginx - debe funcionar
```

### Verificar segmentación de redes:

```bash
# Los contenedores NO deberían poder acceder a servicios fuera de su red

# Desde un worker backend, NO debe alcanzar nginx:
docker exec rss2_cluster_py curl http://nginx  # Debería fallar

# Desde monitoring, NO debe alcanzar db:
docker exec rss2_prometheus curl http://db:5432  # Debería fallar
```

---

## 📊 COMPARATIVA: ANTES vs DESPUÉS

| Aspecto | ANTES (Inseguro) | DESPUÉS (Seguro) |
|---------|------------------|------------------|
| **Puertos expuestos** | 7 puertos públicos | 1 puerto público + 1 localhost |
| **Autenticación Redis** | ❌ Sin password | ✅ Autenticado |
| **PostgreSQL Password** | `x` (débil) | 32+ caracteres aleatorios |
| **Flask SECRET_KEY** | `secret` | 64 caracteres hex |
| **Segmentación de red** | ❌ Una red única | ✅ 3 redes aisladas |
| **Límites de recursos** | ❌ Sin límites | ✅ CPU y RAM limitados |
| **Volúmenes** | Read-Write | Read-Only donde posible |
| **Qdrant público** | ⚠️ Sí (puerto 6333) | ✅ Solo interno |
| **Prometheus público** | ⚠️ Sí (puerto 9090) | ✅ Solo interno |

---

## 🚨 CHECKLIST FINAL DE SEGURIDAD

Antes de poner en producción, verifica:

- [ ] Todas las contraseñas generadas aleatoriamente (min 32 caracteres)
- [ ] Archivo `.env` NO está en el repositorio (revisar .gitignore)
- [ ] Solo puerto 8001 expuesto públicamente
- [ ] Grafana accesible solo en localhost
- [ ] Redis requiere autenticación
- [ ] Todos los workers pueden conectarse a Redis
- [ ] Todos los workers pueden conectarse a PostgreSQL
- [ ] Web app funciona correctamente
- [ ] Búsqueda semántica (Qdrant) funciona
- [ ] Backups automáticos configurados
- [ ] Monitoring (Grafana) accessible via SSH tunnel
- [ ] Firewall del servidor configurado (solo permitir 8001, 22)

---

## 🔧 TROUBLESHOOTING

### Error: Redis authentication failed

```bash
# Verificar que REDIS_PASSWORD está en .env
grep REDIS_PASSWORD .env

# Verificar que los workers tienen la variable
docker exec rss2_web env | grep REDIS

# Reiniciar servicios
docker-compose restart
```

### Error: No puedo acceder a Grafana desde mi máquina

```bash
# Asegurarte de que el túnel SSH está activo
ssh -L 3001:localhost:3001 usuario@servidor

# Verificar que Grafana está corriendo
docker-compose ps | grep grafana

# Logs de Grafana
docker-compose logs grafana
```

### Workers no pueden conectarse a Qdrant

```bash
# Verificar que Qdrant está en la red backend
docker network inspect rss2_backend | grep qdrant

# Verificar logs de Qdrant
docker-compose logs qdrant

# Test de conectividad desde un worker
docker exec rss2_qdrant_worker curl http://qdrant:6333/collections
```

---

## 📚 RECURSOS ADICIONALES

- [Docker Networks Security Best Practices](https://docs.docker.com/network/network-tutorial-standalone/)
- [Redis Security](https://redis.io/docs/management/security/)
- [PostgreSQL Security](https://www.postgresql.org/docs/current/security.html)
- [OWASP Docker Security Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Docker_Security_Cheat_Sheet.html)

---

## 📝 NOTAS IMPORTANTES

1. **Backup Regular**: Configura backups automáticos ANTES de migrar
2. **Testing**: Prueba en un entorno de desarrollo primero
3. **Downtime**: Planifica una ventana de mantenimiento
4. **Monitoring**: Verifica que Grafana funciona después de migrar
5. **Documentation**: Documenta las contraseñas en un gestor seguro

---

**Última actualización**: 2026-01-12
**Autor**: Auditoría de Seguridad Automatizada
