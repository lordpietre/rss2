FROM python:3.11-slim-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends \
    patchelf gcc git curl wget \
    && rm -rf /var/lib/apt/lists/*

ENV PYTHONUNBUFFERED=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    TOKENIZERS_PARALLELISM=false \
    HF_HOME=/root/.cache/huggingface

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir --upgrade pip

RUN pip install --no-cache-dir torch==2.1.0 torchvision==0.16.0 --index-url https://download.pytorch.org/whl/cu118

RUN pip install --no-cache-dir \
    ctranslate2==3.24.0 \
    sentencepiece \
    transformers==4.36.0 \
    protobuf==3.20.3 \
    "numpy<2" \
    websocket-client

RUN find /usr/local/lib/python3.11/site-packages/ctranslate2* \
    -name "libctranslate2-*.so.*" -o -name "libctranslate2.so*" | \
    xargs -I {} patchelf --clear-execstack {} || true

COPY worker.py /app/worker.py

# CT2_DEVICE y CT2_COMPUTE_TYPE se configuran via variables de entorno
# Valores por defecto: cuda + float16 (requiere GPU NVIDIA con 6GB+ VRAM)
# Para CPU: CT2_DEVICE=cpu CT2_COMPUTE_TYPE=int8
ENV CT2_DEVICE=${CT2_DEVICE:-cpu}
ENV CT2_COMPUTE_TYPE=${CT2_COMPUTE_TYPE:-int8}

CMD ["python", "/app/worker.py"]