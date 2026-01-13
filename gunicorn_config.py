"""
Configuración de Gunicorn optimizada para alta capacidad de proceso
"""
import multiprocessing
import os

# Bind
bind = "0.0.0.0:8000"

# Workers
# Fórmula recomendada: (2 x $num_cores) + 1
workers = int(os.getenv("GUNICORN_WORKERS", multiprocessing.cpu_count() * 2 + 1))

# Worker class - sync como fallback si gevent no está disponible
# Para máximo rendimiento, cambiar a "gevent" después de instalar gevent
worker_class = "sync"
worker_connections = 1000

# Timeouts
timeout = 300  # 5 minutos para queries pesadas
graceful_timeout = 30
keepalive = 5

# Reiniciar workers después de N requests para prevenir memory leaks
max_requests = 0 # Desactivado para evitar matar hilos de backup
max_requests_jitter = 0

# Logging
accesslog = "-"
errorlog = "-"
loglevel = "info"
access_log_format = '%(h)s %(l)s %(u)s %(t)s "%(r)s" %(s)s %(b)s "%(f)s" "%(a)s" %(D)s'

# Preload app para compartir memoria entre workers
preload_app = True

# Threading
threads = 2

# Process naming
proc_name = "rss2_gunicorn"

# Server mechanics
daemon = False
pidfile = None
umask = 0
user = None
group = None
tmp_upload_dir = None

# SSL (no usado, NGINX maneja SSL)
keyfile = None
certfile = None

# Configuración de seguridad
limit_request_line = 4094
limit_request_fields = 100
limit_request_field_size = 8190

def on_starting(server):
    """Callback cuando el servidor arranca"""
    server.log.info("Starting RSS2 Gunicorn server with %d workers", workers)

def on_reload(server):
    """Callback cuando el servidor recarga"""
    server.log.info("Reloading RSS2 Gunicorn server")

def worker_int(worker):
    """Callback cuando un worker recibe SIGINT o SIGQUIT"""
    worker.log.info("Worker received INT or QUIT signal")

def worker_abort(worker):
    """Callback cuando un worker es abortado"""
    worker.log.info("Worker received SIGABRT signal")
