FROM python:3.11-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    libpq-dev gcc git curl \
    && rm -rf /var/lib/apt/lists/*

ENV PYTHONUNBUFFERED=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    TOKENIZERS_PARALLELISM=false \
    HF_HOME=/root/.cache/huggingface

COPY requirements.txt .
RUN pip install --no-cache-dir --upgrade pip

RUN pip install --no-cache-dir torch==2.1.0 torchvision==0.16.0 --index-url https://download.pytorch.org/whl/cu121

RUN pip install --no-cache-dir \
    ctranslate2 \
    sentencepiece \
    transformers==4.44.0 \
    protobuf==3.20.3 \
    "numpy<2" \
    psycopg2-binary \
    redis \
    requests \
    beautifulsoup4 \
    lxml \
    langdetect \
    nltk \
    scikit-learn \
    pandas \
    sentence-transformers \
    spacy

RUN python -m spacy download es_core_news_lg

COPY workers/ ./workers/
COPY init-db/ ./init-db/
COPY migrations/ ./migrations/
COPY entity_config.json .

ENV DB_HOST=db
ENV DB_PORT=5432
ENV DB_NAME=rss
ENV DB_USER=rss
ENV DB_PASS=x

CMD ["python", "-m", "workers.embeddings_worker"]
