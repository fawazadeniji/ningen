# Ningen — DSN × BCT LLM Agent Challenge

**Team submission for the Data Science Nigeria × Bluechip Tech LLM Agent Hackathon.**
Deadline: 24 May 2026.

---

## What This Is

A fully containerized recommendation system built on real-world review data (Yelp, Amazon, Goodreads).
It handles two distinct tasks through one unified API:

- **Task A** *(partner's scope)* — Simulate a user's star rating and written review for an unseen item based on their history.
- **Task B** *(this codebase)* — Conversational, context-aware recommendation with cold-start handling, cross-domain retrieval, and multi-turn dialogue.

All outputs are post-processed through a Nigerian cultural humanizer so responses feel natural and grounded in everyday Nigerian life.

---

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│                     docker-compose                        │
│                                                          │
│  ┌──────────┐    ┌──────────────┐    ┌───────────────┐  │
│  │ Postgres │    │   Embedder   │    │   API Server  │  │
│  │ +pgvector│    │  (ONNX/CPU)  │    │   (Go HTTP)   │  │
│  │  :5432   │    │   :8000      │    │   :8080       │  │
│  └────┬─────┘    └──────┬───────┘    └──────┬────────┘  │
│       │                 │                    │           │
│  ┌────┴─────────────────┴────────────────────┴───────┐  │
│  │              ETL Worker (one-shot)                 │  │
│  │  stream → embed → bulk insert → HNSW index        │  │
│  └────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────┘
```

**Request flow for Task B (`POST /recommend`):**

```
Client
  │
  ▼
Cold-start gate
  │  no history → humanized clarifying question
  │
  ▼
Embed last user turn (ONNX sidecar)
  │
  ▼
pgvector HNSW cosine search (pool of 50)
  │  fallback → full-text ILIKE search
  │
  ▼
LLM synthesis (Kimi / Gemini / OpenAI)
  │
  ▼
Nigerian Humanizer pass
  │
  ▼
JSON response (top-N items + reasoning)
```

---

## Services

| Service | Description |
|---|---|
| `db` | PostgreSQL 16 + pgvector extension |
| `embedder` | Python FastAPI sidecar running `all-MiniLM-L6-v2` via ONNX Runtime (CPU-only, no Torch) |
| `etl_worker` | One-shot Go binary — streams 100k reviews, embeds, bulk-inserts, builds HNSW index, then exits |
| `api` | Long-running Go HTTP server exposing the recommendation API |

The embedder uses ONNX Runtime instead of Ollama — no extra 4 GB pull, cold starts in seconds, and model weights are cached in a Docker volume after first run.

---

## Quick Start

### Prerequisites

- Docker >= 24 and Docker Compose V2
- At least one LLM API key (Kimi, Gemini, or OpenAI)

### 1. Configure environment

```bash
cp .env.example .env
# Open .env and add at least one API key
```

### 2. Start everything

```bash
docker compose up --build
```

That single command:
1. Starts Postgres and waits for it to be healthy.
2. Starts the embedder sidecar and downloads the ONNX model on first run (~90 MB, cached to a volume).
3. Runs the ETL worker — streams 100k reviews from Yelp, Amazon, and Goodreads, embeds each one, bulk-inserts into Postgres, and builds the HNSW index. **This takes 20–40 minutes on first run** depending on network speed.
4. Starts the API server on port 8080.

The ETL worker is idempotent. If you restart the stack after a full ingest, it detects `COUNT(*) >= 100000` and skips straight to booting the API.

### 3. Verify

```bash
curl http://localhost:8080/health
# → 200 OK
```

---

## API Reference

### `POST /recommend` — Task B

Conversational recommendation. Pass a user persona and chat history; receive ranked items with a humanized reasoning narrative.

**Request body:**

```json
{
  "user_persona": "A Lagos-based tech worker in her 30s who loves sci-fi and good jollof rice.",
  "history": [
    { "role": "user", "content": "I want something to read on a long flight." }
  ],
  "cross_domain": false,
  "limit": 10,
  "provider": "gemini"
}
```

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `user_persona` | string | **yes** | — | Free-text description of the user. |
| `history` | array | no | `[]` | Alternating `user`/`assistant` turns. Empty history triggers a clarifying question. |
| `cross_domain` | bool | no | `false` | When `true`, LLM freely mixes categories (books, products, restaurants). |
| `limit` | int | no | `10` | Max items to return (1–50). |
| `provider` | string | no | first available | LLM backend: `"kimi"`, `"gemini"`, or `"openai"`. Falls back to any available if omitted. |

**Normal response:**

```json
{
  "recommendations": [
    {
      "item_id": "uuid",
      "domain": "goodreads",
      "search_text": "Review text excerpt...",
      "score": 0.312
    }
  ],
  "reasoning": "Omo, for that long flight you won't regret picking up..."
}
```

**Cold-start response** (empty `history`):

```json
{
  "requires_input": true,
  "question": "Abeg, tell me more — are you looking for something to read, eat, or buy?"
}
```

`score` is cosine distance — lower means more similar. Items are ordered by score ascending (most relevant first).

### `GET /health`

Returns `200 OK` when the API server is ready. Used by Docker health checks.

---

## Environment Variables

Copy `.env.example` to `.env`. At least one LLM key is required or the API server will refuse to start.

| Variable | Description |
|---|---|
| `MOONSHOT_API_KEY` | Kimi (Moonshot AI) API key |
| `GEMINI_API_KEY` | Google Gemini API key |
| `OPENAI_API_KEY` | OpenAI API key |
| `OPENAI_MODEL` | Override OpenAI model (default: `gpt-4o-mini`) |
| `GEMINI_MODEL` | Override Gemini model (default: `gemini-1.5-flash`) |
| `AZURE_OPENAI_URL` | Azure OpenAI endpoint URL (replaces `OPENAI_API_KEY`) |
| `AZURE_OPENAI_KEY` | Azure OpenAI key |
| `DB_URL` | Postgres connection string (auto-set in docker-compose) |
| `EMBEDDER_URL` | Embedder sidecar URL (auto-set in docker-compose) |
| `PORT` | API server port (default: `8080`) |

---

## Project Structure

```
ningen/
├── cmd/api/main.go          # API server entry point
├── main.go                  # ETL pipeline entry point
│
├── domain/
│   └── item.go              # Shared Item type
│
├── ingest/
│   ├── source.go            # Source interface
│   ├── amazon.go            # Amazon gzipped JSONL streamer
│   ├── goodreads.go         # Goodreads CSV streamer
│   └── yelp.go              # Yelp CSV streamer (unused — URL defunct)
│
├── embed/
│   └── embedder.go          # HTTP client for the ONNX sidecar
│
├── store/
│   └── postgres.go          # ETL-side DB: init, bulk insert, HNSW index
│
├── internal/
│   ├── handlers/
│   │   ├── deps.go          # Shared Deps struct + HTTP helpers
│   │   └── task_b.go        # POST /recommend handler
│   ├── llm/
│   │   ├── provider.go      # LLMProvider interface + humanizer system prompt
│   │   ├── openai.go        # Generic OpenAI-compatible client (Kimi/Gemini/Azure/OpenAI)
│   │   └── registry.go      # Build() — reads env, initialises available providers
│   ├── models/
│   │   └── schemas.go       # Request/response types
│   └── rag/
│       └── vector_store.go  # pgvector HNSW search + full-text fallback
│
└── embedder_service/
    ├── main.py              # FastAPI ONNX embedder service
    ├── requirements.txt
    └── Dockerfile
```

---

## ETL Pipeline

The pipeline runs once and exits. Data is streamed directly from source URLs — no large files written to disk.

**Sources (in order):**
1. Amazon Electronics — gzipped JSONL (~1.7M reviews)
2. Amazon Books — gzipped JSONL (~8M reviews, overflow if Electronics runs short)
3. Goodreads Book Reviews — CSV (cross-domain diversity)

**Architecture:** 10 embedding worker goroutines running in parallel, feeding a single writer goroutine that batches 5,000 items per `COPY FROM` call.

**Target:** Configurable via `TARGET_ITEM_COUNT` env var (default: `100000`). Set to `25000` for a quick evaluation run (~8 min). The HNSW index (`vector_cosine_ops`) is created after all inserts to avoid write amplification during bulk load.

**Resume behavior:** On restart, the pipeline counts existing DB rows and skips that many items from the beginning of the source stream before continuing. Safe to interrupt and resume.

---

## Task A — User Modeling (Partner's Implementation)

### Objective

Given a user's historical reviews and an unseen item, simulate:
- The **star rating** the user would give (1–5)
- The **written review** they would write

Evaluated on ROUGE/BERTScore (review quality), RMSE (rating accuracy), and human behavioral fidelity.

### Endpoint

```
POST /simulate
```

Wire it in [cmd/api/main.go](cmd/api/main.go):

```go
mux.HandleFunc("POST /simulate", handlers.SimulateHandler(deps))
```

### Request Schema

```json
{
  "user_persona": "A Lagos-based software engineer in his 40s, very critical of build quality.",
  "review_history": [
    {
      "item": "Logitech MX Master 3 Mouse",
      "rating": 5,
      "review": "Absolute beast of a mouse. The scroll wheel alone is worth the price..."
    },
    {
      "item": "Anker USB-C Hub",
      "rating": 2,
      "review": "Stopped working after 3 months. Anker quality has really dropped..."
    }
  ],
  "target_item": {
    "name": "Razer DeathAdder V3",
    "category": "Electronics",
    "description": "Ergonomic wired gaming mouse, 59g, optical sensor"
  },
  "provider": "gemini"
}
```

### Response Schema

```json
{
  "rating": 4,
  "review": "...",
  "reasoning": "..."
}
```

### Architecture: Behavioral Fidelity Pipeline

Do **not** just prompt the LLM with the history and ask it to guess. The rubric scores behavioral fidelity — the simulation must mimic this specific user's patterns, not a generic reviewer.

Use a three-agent pipeline:

```
POST /simulate
      │
      ▼
┌──────────────────────────────┐
│  Agent 1: Behavioral         │  LLM reads review_history and extracts
│  Profiler                    │  a structured profile:
│                              │  {
│                              │    avg_rating: 3.2,
│                              │    rating_variance: "high",
│                              │    review_length: "verbose",
│                              │    vocabulary: "technical",
│                              │    praises: ["build quality","longevity"],
│                              │    complaints: ["value for money","durability"],
│                              │    writing_quirks: ["uses ellipsis","starts with adjective"]
│                              │  }
└─────────────┬────────────────┘
              │
              ▼
┌──────────────────────────────┐
│  Agent 2: Fit Scorer         │  LLM compares target_item signals
│                              │  against user's known preferences.
│                              │  Output: predicted_rating (int 1–5)
│                              │  + rating_rationale (string)
└─────────────┬────────────────┘
              │
              ▼
┌──────────────────────────────┐
│  Agent 3: Voice Mimic        │  Few-shot: inject 2–3 of the user's
│                              │  actual reviews as examples.
│                              │  LLM generates a new review that:
│                              │  - matches extracted writing_quirks
│                              │  - references the item's specific traits
│                              │  - reflects the predicted_rating's sentiment
└─────────────┬────────────────┘
              │
              ▼
        Humanizer (Nigerian cultural pass — same as Task B)
              │
              ▼
    { rating: 4, review: "...", reasoning: "..." }
```

### Implementation Notes

**Shared infrastructure available:**

The `Deps` struct in [internal/handlers/deps.go](internal/handlers/deps.go) already holds:
- `deps.LLM` — all registered LLM providers. Call `deps.LLM.Get(req.Provider)` to get the chosen backend.
- `deps.Embed` — ONNX embedder sidecar (optional for Task A, but available).

**LLMProvider interface** in [internal/llm/provider.go](internal/llm/provider.go):
```go
provider.Complete(ctx, []llm.Message{...})  // send messages, get string back
provider.Humanize(ctx, rawText, userPersona) // Nigerian cultural pass
```

**Agent 1 tip:** Ask the LLM to respond with JSON only (use a system prompt that says "respond only with valid JSON, no markdown"). Then `json.Unmarshal` the result into a Go struct. This makes the profile reliable and composable.

**Agent 2 tip:** Pass the structured profile from Agent 1 directly into the Agent 2 prompt — don't re-read the history. Keep token cost low.

**Agent 3 tip:** Include the 2–3 shortest reviews from `review_history` as few-shot examples in the system prompt. Short reviews demonstrate style without bloating context.

**Add the request/response types** to [internal/models/schemas.go](internal/models/schemas.go) following the same pattern as `RecommendRequest`.

---

## Design Decisions

**No web framework.** Go 1.22 stdlib `net/http` supports method+path routing natively (`POST /recommend`). Zero dependencies added.

**ONNX over Ollama.** Ollama requires a 4 GB+ image pull and a GPU-optimized runtime. The ONNX sidecar pulls ~90 MB of weights, runs on CPU with ONNX Runtime, and starts in under 10 seconds. Model weights are volume-cached after first download.

**50-candidate pool.** Retrieval always fetches 50 candidates from pgvector. The LLM synthesises over all 50 for better ranking; the response payload trims to `limit` (default 10). This directly improves NDCG@10 without increasing response size.

**Cold-start as an explicit state.** When `history` is empty the system has no intent signal and returning random items would score poorly. Instead it returns `requires_input: true` with a humanized clarifying question, making the multi-turn nature of the system explicit to the evaluator.

**Multi-LLM registry.** Providers are registered at startup based on which env vars are present. Switching providers requires only changing the `provider` field in the request — no code changes. Useful for ablation in the solution paper.
