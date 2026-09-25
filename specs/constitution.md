# Constitución del proyecto RSS2

Principios obligatorios para cualquier cambio (código, compose o docs).
Aprobados a partir del estado verificado del sistema en `docker-compose.yml`,
`backend/`, `frontend/`, `translator/`, `workers/` y `rss-ingestor-go/`.

## 1. Fuentes de verdad (orden de prioridad ante contradicciones)

1. **Esquema BD**: `init-db/*.sql` (p. ej. `noticias.id VARCHAR(32)`, sin columna
   `contenido`). El código que asuma otro esquema está mal, no la BD.
2. **Despliegue**: `docker-compose.yml` (servicios, redes, env). Lo que no tiene
   servicio no corre, aunque el binario exista en una imagen.
3. **Contratos HTTP**: rutas registradas en `backend/cmd/server/main.go` +
   tipos de `backend/internal/models` + `frontend/src/services/api.ts`.
4. **Este directorio `specs/`**: describe el comportamiento acordado.

## 2. Reglas de cambio

- **Evidencia antes que síntesis**: reproducir el fallo (log, curl, build) antes
  de afirmar la causa. No confirmar creencias sin comprobar.
- **Verificación por ejecución**: todo fix se demuestra compilando (`go build`,
  `tsc`), con `go vet` limpio y, si afecta a runtime, contra los contenedores
  (`curl` a `:8888`, conteos SQL, logs).
- **Diff mínimo**: no reformatear archivos enteros, no tocar lo que funciona.
- **Sin secretos en logs, diffs ni docs**: comparar con hashes o MATCH/MISMATCH.
- **Postgres**: `POSTGRES_PASSWORD` solo aplica al inicializar el volumen. Rotar
  requiere `ALTER USER` (ver `specs/08-operations/spec.md`). Nunca borrar
  `data/pgdata` en producción.
- **Caché Redis**: `cache.Set` serializa una vez; pasar siempre structs, nunca
  strings pre-serializados. Tras un fix de formato, `FLUSHDB` (solo hay caché).
- **Scans SQL**: ante `rows.Scan`, verificar orden, nulabilidad y tipos contra
  `init-db/` columna a columna. Prohibido `continue` silencioso sin log en
  código nuevo.
- **Frontend**: no asumir forma de respuesta; los endpoints cacheados pueden
  devolver cualquier forma histórica hasta purgar Redis.
- **Workers nuevos**: siempre con `DB_HOST/DB_PORT/DB_NAME/DB_USER/DB_PASS` y
  resto de env por `docker-compose.yml` (nunca hardcodear `DB_PASS=x` como
  valor efectivo). Healthcheck o log de arranque identificable.

## 3. Definición de terminado

Un cambio está terminado cuando: compila en local y en Docker, `vet` limpio,
verificado en vivo contra el stack (`docker compose`), y `specs/` actualizado
si cambia comportamiento, contrato o topología.
