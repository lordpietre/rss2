FROM python:3.11-slim

# CUDA o CPU
ARG TORCH_CUDA=cu121

WORKDIR /app

# --------------------------------------------------------
# Dependencias del sistema
# --------------------------------------------------------
RUN apt-get update && apt-get install -y --no-install-recommends \
    libpq-dev \
    gcc \
    git \
    libcairo2 \
    libpango-1.0-0 \
    libpangocairo-1.0-0 \
    libgdk-pixbuf-2.0-0 \
    libffi-dev \
    shared-mime-info \
    && rm -rf /var/lib/apt/lists/*

ENV PYTHONUNBUFFERED=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    TOKENIZERS_PARALLELISM=false \
    HF_HUB_DISABLE_SYMLINKS_WARNING=1 \
    HF_HOME=/root/.cache/huggingface

# --------------------------------------------------------
# Instalación de requirements
# --------------------------------------------------------
COPY requirements.txt .
RUN python -m pip install --no-cache-dir --upgrade pip setuptools wheel

# Instalar PyTorch según GPU/CPU
RUN if [ "$TORCH_CUDA" = "cu121" ]; then \
    pip install --no-cache-dir --index-url https://download.pytorch.org/whl/cu121 \
    torch==2.4.1 torchvision==0.19.1 torchaudio==2.4.1 ; \
    else \
    pip install --no-cache-dir --index-url https://download.pytorch.org/whl/cpu \
    torch==2.4.1 torchvision==0.19.1 torchaudio==2.4.1 ; \
    fi

RUN pip install --no-cache-dir -r requirements.txt

# Instalar ctranslate2 con soporte CUDA
RUN if [ "$TORCH_CUDA" = "cu121" ]; then \
    pip install --no-cache-dir ctranslate2 ; \
    else \
    pip install --no-cache-dir ctranslate2 ; \
    fi

# Descargar modelo spaCy ES
RUN python -m spacy download es_core_news_md || true

# --------------------------------------------------------
# Copiar TODO el proyecto rss2/
# --------------------------------------------------------
COPY . .

# --------------------------------------------------------
# Puede descargar modelos NLLB o Sentence-BERT si existe
# --------------------------------------------------------
RUN python download_models.py || true

EXPOSE 8000

