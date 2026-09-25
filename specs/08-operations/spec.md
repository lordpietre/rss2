# Spec 08 — Operaciones (credenciales, despliegue, datos)

## Credenciales (lección aprendida, procedimiento)

- `.env`: `POSTGRES_PASSWORD`, `REDIS_PASSWORD`, `DB_PASS`(legacy),
  `SECRET_KEY`, `GRAFANA_PASSWORD`, `TARGET_LANGS`. Genera con
  `generate_secure_credentials.sh` (ojo: crea `.env.generated`, hay que
  aplicarlo a `.env`).
- **Postgres**: el password solo aplica al inicializar `data/pgdata`. Tras
  rotar: `ALTER USER rss WITH PASSWORD '<.env>'` por socket (trust local)
  y `--force-recreate` de los consumidores (pueden quedar con env vieja si
  se crearon en el mismo minuto). Nunca borrar `pgdata`.
- **Redis**: aplica `requirepass` al arrancar; basta recrear consumidores.
- **SECRET_KEY**: rotar invalida JWTs → re-login obligatorio. Sin reset de
  password en app (reset manual: bcrypt + `UPDATE users`, ver historial).
- Verificar sin exponer secretos: hashes/MATCH-MIZMATCH, `SELECT 1` por TCP
  (`-h 127.0.0.1` es trust; probar desde otra IP/contenedor para SCRAM).

## Despliegue

- `docker compose up -d --build <svc>`; gateway único `:8888`; `backend`
  sin puertos publicados (solo vía gateway o `docker exec`).
- No construir dos servicios contra la misma etiqueta de imagen en un solo
  comando (`AlreadyExists`): construir uno y arrancar el resto `--no-build`.
- Contexto raíz (scheduler/ner): `.dockerignore` debe excluir `data/`,
  `models/`, `hf_cache/`, `backups/` (`pgdata` es 0700 e ilegible).
- Sin NVIDIA: `translator-gpu` queda en `profiles:[gpu]` (esta máquina: Intel).
- Sin Ollama: `?semantic=true` → 500 (decisión vigente).

## Datos y recuperaciones verificadas

- `noticias.id = md5(url)`, `UNIQUE(url)` → re-ingestas idempotentes.
- Sospecha de env vieja: comparar `docker inspect Created` con la hora del
  cambio + `--force-recreate`.
- Backend crash-loop `Failed to connect to database` → revisar password
  efectivo (TCP, no socket) antes que nada.
- Limpieza disco 2026-09-14 (raíz al 100%): `~/.cache/pip`,
  `~/.cache/google-chrome`, `~/.cache/go-build`, `~/.cache/tracker3`
  (cachés regenerables, ~11G); `docker container prune` (12.4G de
  contenedores parados); eliminada imagen `rss2-translator-gpu` (8.3G, sin
  NVIDIA en esta máquina; reconstruible con `Dockerfile.translator-gpu`).
  NO tocar: `data/pgdata`, `backups/`, `hf_cache/`, `models/`,
  `~/.cache/huggingface` (modelos de otro proyecto).

## Tasks

- [x] Procedimiento de rotación documentado y probado.
- [x] Retención backups (`backup.sh`): 30 días + elimina dumps fallidos
  (<1024 bytes) en vez de acumularlos (2026-09-14: 10 ficheros de 20 B
  eliminados). 2026-09-15: `set -o pipefail` (un pg_dump fallido quedaba
  enmascarado por gzip; el dump de 12 KB del día 13 pasó el filtro).
- [x] User-Agent wiki genérico (antes email personal en cada petición).
- [x] `.dockerignore` raíz: +`node_modules`, `frontend/node_modules`,
  `PM/` (el contexto superaba 750 MB y ralentizaba cada build).
- [x] Healthchecks en ingestor/scheduler/langdetect/ner/translator/wiki/
  topics (`grep -q <proceso> /proc/1/cmdline`; todos `healthy`
  verificado 2026-09-14).
- [ ] Backup automático de `pgdata` antes de cualquier rotación (hoy manual).
