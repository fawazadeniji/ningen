FROM python:3.10-slim-bookworm

WORKDIR /app

# Install system dependencies with retries to handle network timeouts
RUN apt-get update && \
    for i in 1 2 3; do apt-get install -y --no-install-recommends \
    build-essential \
    curl && break || sleep 5; done \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY main.py .

# Download model weights at build time — no runtime internet access needed.
# MODEL_DIR is /models, baked into the image layer.
RUN python -c "\
from huggingface_hub import hf_hub_download; \
[hf_hub_download(repo_id='optimum/all-MiniLM-L6-v2', filename=f, local_dir='/models') \
 for f in ['model.onnx', 'tokenizer.json']]"

EXPOSE 8000

CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
