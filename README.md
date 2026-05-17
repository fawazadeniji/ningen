# Ningen — DSN × BCT LLM Agent Challenge

**Team submission for the Data Science Nigeria × Bluechip Tech LLM Agent Hackathon.**
Deadline: 24 May 2026.

---

## What This Is

A fully containerized recommendation system built on real-world review data (Yelp, Amazon, Goodreads).
It handles two distinct tasks through one unified API:

- **Task A** — Simulate a user's star rating and written review for an unseen item based on their history.
- **Task B** — Conversational, context-aware recommendation with cold-start handling, cross-domain retrieval, and multi-turn dialogue.

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

| Service      | Description                                                                                    |
| ------------ | ---------------------------------------------------------------------------------------------- |
| `db`         | PostgreSQL 16 + pgvector extension                                                             |
| `embedder`   | Python FastAPI sidecar running `all-MiniLM-L6-v2` via ONNX Runtime (CPU-only, no Torch)        |
| `etl_worker` | One-shot Go binary — streams 100k reviews, embeds, bulk-inserts, builds HNSW index, then exits |
| `api`        | Long-running Go HTTP server exposing the recommendation API                                    |

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

| Field          | Type   | Required | Default         | Description                                                                               |
| -------------- | ------ | -------- | --------------- | ----------------------------------------------------------------------------------------- |
| `user_persona` | string | **yes**  | —               | Free-text description of the user.                                                        |
| `history`      | array  | no       | `[]`            | Alternating `user`/`assistant` turns. Empty history triggers a clarifying question.       |
| `cross_domain` | bool   | no       | `false`         | When `true`, LLM freely mixes categories (books, products, restaurants).                  |
| `limit`        | int    | no       | `10`            | Max items to return (1–50).                                                               |
| `provider`     | string | no       | first available | LLM backend: `"kimi"`, `"gemini"`, or `"openai"`. Falls back to any available if omitted. |

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

| Variable                   | Description                                                  |
| -------------------------- | ------------------------------------------------------------ |
| `MOONSHOT_API_KEY`         | Kimi (Moonshot AI) API key                                   |
| `GEMINI_API_KEY`           | Google Gemini API key                                        |
| `OPENAI_API_KEY`           | OpenAI API key                                               |
| `OPENAI_MODEL`             | Override non-Azure OpenAI model (default: `gpt-4o-mini`)     |
| `GEMINI_MODEL`             | Override Gemini model (default: `gemini-1.5-flash`)          |
| `AZURE_OPENAI_URL`         | Azure OpenAI endpoint URL (used instead of `OPENAI_API_KEY`) |
| `AZURE_OPENAI_KEY`         | Azure OpenAI key                                             |
| `AZURE_OPENAI_MODEL`       | Azure OpenAI deployment/model name                           |
| `AZURE_OPENAI_API_VERSION` | Azure OpenAI API version override (optional)                 |
| `DB_URL`                   | Postgres connection string (auto-set in docker-compose)      |
| `EMBEDDER_URL`             | Embedder sidecar URL (auto-set in docker-compose)            |
| `PORT`                     | API server port (default: `8080`)                            |

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

## Task A — User Modeling

### Objective

Given a user's historical reviews and a target product, the system predicts:

- A **rating** (float from 1.0 to 5.0)
- A **generated review** aligned with the user's inferred style
- A **rating reasoning** trace from the rater node
- The inferred **user profile** used by downstream steps

### Endpoint

```
POST /generate-review
```

Wired in [cmd/api/main.go](cmd/api/main.go):

```go
mux.HandleFunc("POST /generate-review", handlers.GenerateReviewHandler(deps))
```

### Request Schema

```json
{
  "user_history": [
    {
      "product_id": "h1",
      "product_name": "Wireless Earbuds",
      "product_category": "electronics",
      "star_rating": 4,
      "review_text": "Good sound for the price.",
      "review_date": "2026-05-01",
      "source": "amazon"
    },
    {
      "product_id": "h2",
      "product_name": "Laptop Stand",
      "product_category": "electronics",
      "star_rating": 3.5,
      "review_text": "Useful, but a little overpriced.",
      "review_date": "2026-05-10",
      "source": "amazon"
    }
  ],
  "target_product": {
    "product_id": "t1",
    "product_name": "Portable Speaker",
    "product_category": "electronics",
    "description": "Compact Bluetooth speaker with deep bass.",
    "price": 25000,
    "currency": "NGN",
    "source": "amazon",
    "features": ["bluetooth", "portable", "deep bass"],
    "rating": 4.4,
    "review_count": 152
  },
  "provider": "openai",
  "model_overrides": {
    "profiler": "gpt-5.4-mini",
    "rater": "gpt-5.4",
    "drafter": "gpt-5.4",
    "critic": "gpt-5.4"
  }
}
```

**Optional per-node model overrides:**

The `model_overrides` field (optional) allows specifying different LLM models for each pipeline node:

- `profiler` — override for the Profiler node (user behavioral analysis)
- `rater` — override for the Rater node (rating prediction)
- `drafter` — override for the Drafter node (review text generation)
- `critic` — override for the Critic node (behavioral fidelity QA)

If a node is omitted or the entire `model_overrides` field is absent, the provider's default model is used. This enables fine-grained control — for example, using a fast model like `gpt-5.4-mini` for profiling/drafting while using a more capable model like `gpt-5.4` for critical rating decisions.

Validation rules currently enforced in handler:

- `user_history` must be non-empty
- `target_product.product_id` is required
- `provider` defaults to `openai` if omitted
- unknown/unavailable provider returns `400`
- `model_overrides` is optional; if provided, model names must be valid for the provider

### Response Schema

```json
{
  "generated_review": "Revised review that sounds more natural and direct.",
  "predicted_rating": 4.2,
  "rating_reasoning": "This product fits the user's preferences well.",
  "user_profile": {
    "user_id": "u-1",
    "overall_tendency": "balanced",
    "average_rating": 3.8,
    "preferred_categories": ["electronics"]
  },
  "iterations": 2
}
```

### Architecture: Implemented Pipeline

Current flow in [internal/pipeline/graph.go](internal/pipeline/graph.go):

```
POST /generate-review
      │
      ▼
┌──────────────────────────────┐
│ Agent 1: Profiler            │ Structured JSON profile extraction
│                              │ via schema-constrained LLM output
└─────────────┬────────────────┘
              │
              ▼
┌──────────────────────────────┐
│ Agent 2: Rater               │ Predicts rating + chain_of_thought
│                              │ (parsed to rating_reasoning)
└─────────────┬────────────────┘
              │
              ▼
┌──────────────────────────────┐
│ Agent 3: Drafter             │ Localizes product context to Nigerian
│                              │ settings, then drafts the review text
└─────────────┬────────────────┘
              │
              ▼
┌──────────────────────────────┐
│ Agent 4: Critic             │ PASS/FAIL behavioral fidelity check
│                              │ with revision loop
└─────────────┬────────────────┘
              │
              ▼
Return response with generated_review, predicted_rating,
rating_reasoning, user_profile, iterations
```

Critic loop details:

- max 2 draft/critic iterations (`maxLoops = 2`)
- if critic returns `PASS`, current draft becomes `final_review`
- if loop cap is reached, latest draft is returned
- local fallback checks reject common AI-sounding phrasing when critic parsing fails

### Implementation Notes

Task A implementation currently lives in:

- [internal/handlers/task_a.go](internal/handlers/task_a.go) (endpoint contract + orchestration)
- [internal/pipeline/graph.go](internal/pipeline/graph.go) (node sequence + loop control)
- [internal/pipeline/nodes/](internal/pipeline/nodes/) (Profiler, Rater, Drafter, Critic, Localization)
- [internal/handlers/task_a_test.go](internal/handlers/task_a_test.go) (end-to-end handler test)

Notes on current behavior:

- Task A request/response structs are defined in handler/pipeline packages, not in [internal/models/schemas.go](internal/models/schemas.go).
- Task A currently uses provider `Complete` calls in pipeline nodes; it does not run a final `Humanize` pass like Task B.

### Per-Node Model Configuration (Model Overrides)

Each pipeline node (Profiler, Rater, Drafter, Critic) can use a different LLM model via the `model_overrides` field in the request. This is useful for:

- **Cost optimization**: Use fast, cheap models (e.g., `gpt-5.4-mini`) for less critical nodes like Drafter, and more capable models (e.g., `gpt-4o`) for critical decisions like Rater.
- **Latency tuning**: Route fast profiling to a lightweight model while keeping rating prediction on a more accurate one.
- **A/B testing**: Compare outputs across models without changing infrastructure.

**Implementation:**

- `AgentState.ModelOverrides` (map of node name → model string) is passed through the pipeline
- Each node queries `state.ModelFor("<nodename>")` to check for an override
- If present, the override is passed to the LLM provider via `llm.WithModel()` option
- The provider applies the override at the API call level without affecting the base provider configuration

**Example usage:**

```go
state := pipeline.AgentState{
    UserHistory: [...],
    TargetProduct: {...},
    ModelOverrides: map[string]string{
        "profiler": "gpt-5.4-mini",
        "rater":    "gpt-4o",         // More accurate for ratings
        "drafter":  "gpt-5.4-mini",   // Speed for drafting
        "critic":   "gpt-4o",         // Strict validation
    },
}
finalState, err := pipeline.ExecuteWorkflow(ctx, model, state)
```

All nodes support model overrides. See [internal/pipeline/nodes/model_overrides_test.go](internal/pipeline/nodes/model_overrides_test.go) for comprehensive test coverage.

---

## Design Decisions

**No web framework.** Go 1.22 stdlib `net/http` supports method+path routing natively (`POST /recommend`). Zero dependencies added.

**ONNX over Ollama.** Ollama requires a 4 GB+ image pull and a GPU-optimized runtime. The ONNX sidecar pulls ~90 MB of weights, runs on CPU with ONNX Runtime, and starts in under 10 seconds. Model weights are volume-cached after first download.

**50-candidate pool.** Retrieval always fetches 50 candidates from pgvector. The LLM synthesises over all 50 for better ranking; the response payload trims to `limit` (default 10). This directly improves NDCG@10 without increasing response size.

**Cold-start as an explicit state.** When `history` is empty the system has no intent signal and returning random items would score poorly. Instead it returns `requires_input: true` with a humanized clarifying question, making the multi-turn nature of the system explicit to the evaluator.

**Multi-LLM registry.** Providers are registered at startup based on which env vars are present. Switching providers requires only changing the `provider` field in the request — no code changes. Useful for ablation in the solution paper.
