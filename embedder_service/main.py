from contextlib import asynccontextmanager
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
import onnxruntime as ort
import numpy as np
from tokenizers import Tokenizer
import os
from huggingface_hub import hf_hub_download

MODEL_NAME = "optimum/all-MiniLM-L6-v2"
MODEL_DIR = os.getenv("MODEL_DIR", "/models")

tokenizer = None
session = None

def load_model():
    global tokenizer, session
    print(f"Loading model {MODEL_NAME} from {MODEL_DIR}...")
    files = ["model.onnx", "tokenizer.json"]
    for f in files:
        path = os.path.join(MODEL_DIR, f)
        if not os.path.exists(path):
            print(f"Downloading {f}...")
            hf_hub_download(repo_id=MODEL_NAME, filename=f, local_dir=MODEL_DIR, force_download=False)
    tokenizer = Tokenizer.from_file(os.path.join(MODEL_DIR, "tokenizer.json"))
    session = ort.InferenceSession(os.path.join(MODEL_DIR, "model.onnx"), providers=['CPUExecutionProvider'])
    print("Model loaded successfully.")

@asynccontextmanager
async def lifespan(app: FastAPI):
    load_model()
    yield

app = FastAPI(lifespan=lifespan)

class EmbedRequest(BaseModel):
    text: str

@app.get("/health")
async def health():
    return {"status": "ok"}

@app.post("/embed")
async def embed(req: EmbedRequest):
    if not session or not tokenizer:
        raise HTTPException(status_code=500, detail="Model not loaded")
    
    # Tokenize
    encoded = tokenizer.encode(req.text)
    input_ids = np.array([encoded.ids], dtype=np.int64)
    attention_mask = np.array([encoded.attention_mask], dtype=np.int64)
    token_type_ids = np.array([encoded.type_ids], dtype=np.int64)
    
    # Run inference
    inputs = {
        "input_ids": input_ids,
        "attention_mask": attention_mask,
        "token_type_ids": token_type_ids
    }
    outputs = session.run(None, inputs)
    
    # Mean pooling
    token_embeddings = outputs[0]
    mask = attention_mask.reshape(attention_mask.shape + (1,))
    token_embeddings *= mask
    sum_embeddings = np.sum(token_embeddings, axis=1)
    sum_mask = np.clip(np.sum(mask, axis=1), a_min=1e-9, a_max=None)
    embedding = sum_embeddings / sum_mask
    
    # Normalize
    norm = np.linalg.norm(embedding, axis=1, keepdims=True)
    embedding /= norm
    
    return {"embedding": embedding[0].tolist()}
