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

# Create directory for models
RUN mkdir /models

EXPOSE 8000

CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
