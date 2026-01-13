# Deployment Guide

This guide describes how to deploy the application to a new server.

## Prerequisites

*   **Linux Server** (Ubuntu 22.04+ recommended)
*   **NVIDIA GPU**: Required for translation, embeddings, and NER services.
*   **NVIDIA Container Toolkit**: Must be installed to allow Docker to access the GPU.
*   **Docker** & **Docker Compose**: Latest versions.
*   **Git**: To clone the repository.
*   **External Service**: An instance of [AllTalk](https://github.com/erew123/alltalk_tts) running externally or on the host (port 7851 by default).

## Deployment Steps

1.  **Clone the Repository**
    ```bash
    git clone <your-repo-url>
    cd <your-repo-name>
    ```

2.  **Configure Environment Variables**
    Copy the example configuration file:
    ```bash
    cp .env.example .env
    ```
    Edit `.env` and set secure passwords and configuration:
    ```bash
    nano .env
    ```
    *   Change `POSTGRES_PASSWORD` and `DB_PASS` to a strong unique password.
    *   Change `SECRET_KEY` to a long random string.
    *   Verify `ALLTALK_URL` points to your AllTalk instance (default assumes host machine access).

3.  **Start the Services**
    Run the following command to build and start the application:
    ```bash
    docker compose up -d --build
    ```

4.  **Database Initialization**
    The database will automatically initialize on the first run using the scripts in `init-db/`. This may take a few minutes. Check logs with:
    ```bash
    docker compose logs -f db
    ```

5.  **Verify Deployment**
    Access the application at `http://<your-server-ip>:8001`.

## Important Notes

*   **Models**: The application mounts `./models` and `./hf_cache` to persist AI models. On the first run, it will attempt to download necessary models (NLLB, BERT, etc.), which requires significant bandwidth and time.
*   **Data Persistence**: Database data is stored in `./pgdata` (mapped in docker-compose). Ensure this directory is backed up.
*   **Security**: Ensure port 5432 (Postgres) and 6379 (Redis) are firewall-protected and not exposed to the public internet unless intended (Docker maps them to the host network).
