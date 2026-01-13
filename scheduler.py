import time
import logging
import atexit
from datetime import datetime, timedelta
import sys

from apscheduler.schedulers.background import BackgroundScheduler

# Carga de la app Flask
from app import app

# Import correcto de operaciones de traduccion
from translation_ops import run_producer_cycle


logging.basicConfig(
    stream=sys.stdout,
    level=logging.INFO,
    format='[%(asctime)s] %(levelname)s in %(module)s: %(message)s'
)

scheduler = BackgroundScheduler(
    daemon=True,
    timezone="UTC"
)


def shutdown_scheduler():
    """Detiene el planificador al salir del proceso de forma segura."""
    try:
        if scheduler.running:
            scheduler.shutdown(wait=False)
            logging.info("Scheduler detenido correctamente.")
    except Exception as e:
        logging.error(f"Error al detener el scheduler: {e}")


# Registrar apagado limpio
atexit.register(shutdown_scheduler)


if __name__ == "__main__":

    # Entrar al contexto de Flask (necesario para partes del proyecto)
    with app.app_context():
        try:
            # Job 1: RSS Fetching -> MOVIDO A GO (rss-ingestor-go)
            # Este scheduler ya no maneja la ingesta de noticias.

            # Job 2: Translation Producer
            scheduler.add_job(
                run_producer_cycle,
                trigger="interval",
                minutes=1,
                id="translation_producer_job",
                next_run_time=datetime.utcnow() + timedelta(seconds=5),
                max_instances=1,
                coalesce=True,
            )

            # Job 3: Precache Entities
            from scripts.precache_entities import run_precache
            scheduler.add_job(
                run_precache,
                trigger="interval",
                hours=6,
                id="precache_entities_job",
                next_run_time=datetime.utcnow() + timedelta(seconds=20),
                max_instances=1,
                coalesce=True,
            )

            scheduler.start()
            logging.info("Scheduler iniciado correctamente.")
            logging.info("Tareas activas: translation_producer_job, precache_entities_job")

        except Exception as e:
            logging.exception(f"Error inicializando el scheduler: {e}")
            sys.exit(1)

    # Mantener proceso vivo (necesario para Docker)
    try:
        while True:
            time.sleep(60)

    except (KeyboardInterrupt, SystemExit):
        logging.info("Apagando el scheduler worker...")
