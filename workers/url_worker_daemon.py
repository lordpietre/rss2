import time
import logging
import sys
from workers.url_worker import main as run_once

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
    stream=sys.stdout
)
logger = logging.getLogger("url_worker_daemon")

INTERVAL = 300  # 5 minutes

def main():
    logger.info("Starting URL Worker Daemon")
    logger.info(f"Check interval: {INTERVAL} seconds")
    
    while True:
        try:
            logger.info("Running job cycle...")
            run_once()
            logger.info("Cycle completed.")
        except Exception as e:
            logger.exception(f"Error in job cycle: {e}")
        
        time.sleep(INTERVAL)

if __name__ == "__main__":
    main()
