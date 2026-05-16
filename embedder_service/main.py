from contextlib import asynccontextmanager
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
import onnxruntime as ort
import numpy as np
from tokenizers import Tokenizer
import os

MODEL_DIR = os.getenv("MODEL_DIR", "/models")

tokenizer = None
session = None

def load_model():
    global tokenizer, session
    print(f"Loading model from {MODEL_DIR}...")
    tokenizer = Tokenizer.from_file(os.path.join(MODEL_DIR, "tokenizer.json"))
    tokenizer.enable_truncation(max_length=256)
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

    encoded = tokenizer.encode(req.text)
    input_ids = np.array([encoded.ids], dtype=np.int64)
    attention_mask = np.array([encoded.attention_mask], dtype=np.int64)

    model_inputs = {inp.name for inp in session.get_inputs()}
    inputs = {"input_ids": input_ids, "attention_mask": attention_mask}
    if "token_type_ids" in model_inputs:
        inputs["token_type_ids"] = np.array([encoded.type_ids], dtype=np.int64)

    outputs = session.run(None, inputs)
    embedding = outputs[0]  # [1, 384] (sentence) or [1, seq_len, 384] (token-level)

    if embedding.ndim == 3:
        mask = attention_mask.reshape(attention_mask.shape + (1,))
        embedding = np.sum(embedding * mask, axis=1) / np.clip(
            np.sum(mask, axis=1), a_min=1e-9, a_max=None
        )

    norm = np.linalg.norm(embedding, axis=1, keepdims=True)
    embedding = embedding / np.maximum(norm, 1e-9)

    return {"embedding": embedding[0].tolist()}
