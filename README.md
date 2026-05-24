# Ningen — DSN × BCT LLM Agent Challenge

**Team submission for the Data Science Nigeria × Bluechip Tech LLM Agent Hackathon.**
Deadline: 24 May 2026.

---

## What This Is

A fully containerized recommendation system built on real-world review data (Yelp, Amazon, Goodreads).
It handles two distinct tasks through one unified API:

- **Task A** — Simulate a user's star rating and written review for an unseen item based on their history.
- **Task B** — Conversational, context-aware recommendation with cold-start handling, cross-domain retrieval, and multi-turn dialogue.

By default, all outputs are post-processed through a Nigerian cultural humanizer so responses feel natural and grounded in everyday Nigerian life. Both endpoints accept `nigerian_flavor: false` to receive neutral English instead.

---

## Live Data (Production)

As of submission (24 May 2026), the production instance at `ningen.firebcorps.online` holds:

| Domain | Items | Named |
| ------ | ----- | ----- |
| Amazon Electronics | ~249,000 | ~249,000 (100% — backfilled from SNAP metadata) |
| Amazon Books | ~250,000 | ~250,000 (100% — backfilled from SNAP metadata) |
| Yelp restaurant reviews | ~270,000 | 0 (dataset strips business names — see Limitations) |
| **Total** | **~770,000** | **~500,000** |

**Amazon name resolution:** After ETL, a one-shot backfill script re-streams the SNAP metadata files (`meta_Electronics.json.gz`, `meta_Books.json.gz`), joins on ASIN, recomputes the same deterministic item_ids, and `UPDATE`s `metadata->>'name'` for each matching row. The `/recommend` endpoint returns `"name"` in the response when available.

**Yelp limitation:** The public Yelp dataset used (SetFit/yelp_review_full) deliberately strips business metadata — only star labels and review text are provided. Business names cannot be recovered without the full Yelp Open Dataset (requires academic license). This is documented as a known limitation.

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

**SIGNAL pipeline — request flow for `POST /recommend`:**

```
Client
  │
  ▼
Cold-start gate
  │  empty history → humanized clarifying question
  │
  ▼
Stage 0 — Corpus pre-search
  │  embed last user turn → top-5 DB samples (grounds LLM in what actually exists)
  │
  ▼
Stage 1 — Signal Extractor (LLM)
  │  persona + history + corpus examples → UserSignal
  │  {intent, domain, search_queries, mood, constraints}
  │  clarify_needed=true → humanized follow-up question
  │
  ▼
Stage 2 — Multi-vector Retrieval
  │  embed each search_query → pgvector HNSW cosine search (pool of 50)
  │  union results, deduplicate by search_text, resolved name, and entity fingerprint
  │  fallback → full-text search on intent phrase
  │
  ▼
Stage 3 — Quality Gate (LLM)
  │  ACCEPT → continue
  │  REFINE → rewrite queries, re-retrieve
  │  ASK    → humanized clarifying question to client
  │
  ▼
Stage 4 — Psychographic Reranker (LLM)
  │  top-20 candidates ranked by mood + constraints + domain fit
  │
  ▼
Nigerian Humanizer pass (overall reasoning)
  │
  ▼
JSON response (top-N items + per-item reasoning + narrative)
```

Each LLM stage runs with a 25-second context timeout. The pipeline is fault-tolerant: Gate parse failures default to ACCEPT, Reranker parse failures fall back to retrieval order.

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
3. Runs the ETL worker — streams 100k reviews from Yelp, Amazon, and Goodreads, embeds each one, bulk-inserts into Postgres with dedup, and builds the HNSW index. **This takes 20–40 minutes on first run** depending on network speed.
4. Starts the API server on port 8080.

The ETL worker is idempotent. If you restart the stack after a full ingest, it detects `COUNT(*) >= 100000` and skips straight to booting the API. Partial runs can be resumed — existing rows are skipped via deterministic IDs and `ON CONFLICT DO NOTHING`.

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

| Field             | Type   | Required | Default         | Description                                                                                                               |
| ----------------- | ------ | -------- | --------------- | ------------------------------------------------------------------------------------------------------------------------- |
| `user_persona`    | string | **yes**  | —               | Free-text description of the user.                                                                                        |
| `history`         | array  | no       | `[]`            | Alternating `user`/`assistant` turns. Empty history triggers a clarifying question.                                       |
| `cross_domain`    | bool   | no       | `false`         | When `true`, LLM freely mixes categories (books, products, restaurants).                                                  |
| `limit`           | int    | no       | `10`            | Max items to return (1–50).                                                                                               |
| `provider`        | string | no       | first available | LLM backend: `"kimi"`, `"gemini"`, or `"openai"`. Falls back to any available if omitted.                                |
| `nigerian_flavor` | bool   | no       | `true`          | When `true` (default), reasoning is humanized in Nigerian English. Set `false` for neutral warm English instead.          |

**Normal response:**

```json
{
  "recommendations": [
    {
      "item_id": "uuid",
      "domain": "amazon",
      "name": "Sony WH-1000XM4 Wireless Noise Canceling Headphones",
      "search_text": "Great noise-cancelling headphones, perfect for long flights...",
      "score": 0.182,
      "reasoning": "Aligns with your productivity focus and commute lifestyle."
    }
  ],
  "reasoning": "Omo, for that long flight you won't regret picking up..."
}
```

**Clarifying question response** (empty `history` or ambiguous signal):

```json
{
  "requires_input": true,
  "question": "Abeg, tell me more — are you looking for something to read, eat, or buy?"
}
```

`score` is cosine distance — lower means more similar. Items are ordered by psychographic rank (not raw score).

`name` is the resolved product name from SNAP metadata. Present for Amazon items after the backfill; absent (field omitted) for Yelp items.

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
├── cmd/
│   ├── api/main.go              # API server entry point
│   ├── backfill_amazon_names/
│   │   └── main.go              # One-shot: join SNAP metadata → UPDATE metadata->>'name'
│   └── holdout_eval/
│       ├── main.go              # Offline evaluation: NDCG@10, Hit@10, MRR
│       └── metrics_test.go      # Unit tests for ranking metrics
├── main.go                      # ETL pipeline entry point
│
├── domain/
│   └── item.go                  # Shared Item type
│
├── ingest/
│   ├── source.go                # Source interface
│   ├── id.go                    # Deterministic UUID v5 for dedup-safe IDs
│   ├── amazon.go                # Amazon gzipped JSONL streamer
│   ├── goodreads.go             # Goodreads CSV streamer
│   └── yelp.go                  # Yelp JSONL streamer
│
├── embed/
│   └── embedder.go              # HTTP client for the ONNX sidecar
│
├── store/
│   └── postgres.go              # ETL-side DB: init, bulk insert (ON CONFLICT DO NOTHING), HNSW index
│
├── internal/
│   ├── agents/
│   │   ├── extractor.go         # Stage 1: LLM signal extraction
│   │   ├── gate.go              # Stage 3: LLM quality gate (ACCEPT/REFINE/ASK)
│   │   ├── reranker.go          # Stage 4: LLM psychographic reranker
│   │   └── agents_test.go       # Unit tests (fully mocked, no network calls)
│   ├── handlers/
│   │   ├── deps.go              # Shared Deps struct + HTTP helpers
│   │   ├── recommend.go         # POST /recommend — full SIGNAL pipeline
│   │   └── recommend_test.go    # Unit tests for dedup and result ordering
│   ├── llm/
│   │   ├── provider.go          # LLMProvider interface + humanizer system prompt
│   │   ├── openai.go            # Generic OpenAI-compatible client (Kimi/Gemini/Azure/OpenAI)
│   │   └── registry.go          # Build() — reads env, initialises available providers
│   ├── models/
│   │   ├── schemas.go           # Request/response types
│   │   └── signal.go            # UserSignal — shared contract across all pipeline stages
│   └── rag/
│       └── vector_store.go      # pgvector HNSW search, multi-vector union, full-text fallback
│
├── scripts/
│   ├── dev.sh                   # Local dev: sources .env, overrides DB/embedder to localhost, runs air
│   ├── eval.sh                  # Smoke-test harness: cold-start, single-turn, multi-turn, all providers
│   ├── spot_eval.sh             # 2-scenario deep inspection: full JSON + reasoning visible
│   └── ablation.sh              # Runs all 6 ablation variants, writes results to OUT_FILE
│
└── embedder_service/
    ├── main.py                  # FastAPI ONNX embedder service
    ├── requirements.txt
    └── Dockerfile
```

---

## ETL Pipeline

The pipeline runs once and exits. Data is streamed directly from source URLs — no large files written to disk.

**Sources (in order):**

1. Yelp restaurant reviews — JSONL from HuggingFace (SetFit/yelp_review_full), 50% of target
2. Amazon Electronics — gzipped JSONL from SNAP (~1.7M reviews), 25% of target
3. Amazon Books — gzipped JSONL from SNAP (~8M reviews), 25% of target

**Dedup:** Item IDs are UUID v5 derived from `domain + review_text`. Reingest of the same content produces the same ID and is silently skipped via `ON CONFLICT (item_id) DO NOTHING`.

**Target:** Configurable via `TARGET_ITEM_COUNT` env var (default: `1,000,000` in production). Per-source limits (50/25/25) ensure domain diversity regardless of target size.

**Pre-filter:** At startup the ETL worker loads all existing Yelp item IDs into memory (`ExistingIDs` query). Items already in the DB are skipped before embedding — avoids re-embedding known duplicates and saves embedder CPU on restarts.

**Local file fallback:** Set `YELP_FILE=/data/yelp_train.jsonl` to read from a local file instead of streaming over HTTP. The volume mount `YELP_DATA_DIR` exposes the host path to the container.

**Resume:** On restart the pipeline counts existing rows. If `count >= TARGET_ITEM_COUNT`, it exits immediately. Otherwise it resumes from the next un-ingested item (Yelp pre-filter handles dedup; Amazon uses `ON CONFLICT DO NOTHING`).

### Amazon Name Backfill

After ETL, run `cmd/backfill_amazon_names` inside the ETL container to resolve product names from SNAP metadata files. Safe to run alongside the API server — only updates `metadata->>'name'` for rows where it is `NULL`.

```bash
# Electronics (run after Electronics ETL)
docker run --rm --env-file .env --network <stack>_default \
  --entrypoint go <etl-image> \
  run ./cmd/backfill_amazon_names

# Books (run with overridden URLs)
docker run --rm --env-file .env --network <stack>_default \
  -e META_URL=https://snap.stanford.edu/data/amazon/productGraph/categoryFiles/meta_Books.json.gz \
  -e REVIEWS_URL=https://snap.stanford.edu/data/amazon/productGraph/categoryFiles/reviews_Books.json.gz \
  --entrypoint go <etl-image> \
  run ./cmd/backfill_amazon_names
```

Env vars: `DRY_RUN=true` prints matches without writing. `DB_URL`, `META_URL`, `REVIEWS_URL` are all overrideable.

**Verification:** Before any writes, the script streams 500 reviews, computes their item_ids, and confirms ≥10/20 exist in the DB. Aborts if the ID formula doesn't match — prevents silently writing names to wrong items.

---

## Offline Evaluation

```bash
# Scored metrics against holdout sets from the source datasets
SEEDS_PER_DOMAIN=20 go run ./cmd/holdout_eval

# 2-scenario deep inspection with full LLM output visible
./scripts/spot_eval.sh
```

The holdout evaluator streams items from the end of each source dataset (never ingested), embeds them, finds ground-truth DB neighbors by cosine distance, queries the API, and reports NDCG@10, Hit@10, and MRR per domain. It handles `requires_input` responses by synthesizing a follow-up from the seed text.

### Ablation Studies

Run all 6 pipeline variants (A–F) in the background and save results to a file:

```bash
nohup OUT_FILE=ablation_results.txt \
      PROVIDER=gemini \
      SEEDS_PER_DOMAIN=10 \
      API_URL=http://localhost:8080 \
      ./scripts/ablation.sh > ablation.log 2>&1 &

echo "PID: $!  —  tail -f ablation_results.txt to follow"
```

Check progress:

```bash
tail -f ablation_results.txt
```

Variants tested:

| Variant | `SKIP_STAGES` value | Stages disabled |
| ------- | ------------------- | --------------- |
| A — Full SIGNAL pipeline | *(none)* | All 5 stages active |
| B — Pure RAG | `gate,reranker` | No quality gate, no reranker |
| C — RAG + Reranker | `gate` | No quality gate |
| D — RAG + Gate | `reranker` | No reranker |
| E — Single-query retrieval | `multi_vector` | Only 1 search query embedded |
| F — No corpus grounding | `grounding` | Stage 0 corpus pre-search skipped |

You can also run a single variant manually:

```bash
SKIP_STAGES=gate,reranker SEEDS_PER_DOMAIN=5 PROVIDER=gemini go run ./cmd/holdout_eval
```

The `debug_skip` field is also accepted directly in the `POST /recommend` request body for one-off testing:

```json
{
  "user_persona": "...",
  "history": [...],
  "provider": "gemini",
  "debug_skip": ["gate", "reranker"]
}
```

---

## Task A — User Modeling

### Objective

Given a user's historical reviews and a target product, the system predicts:

- A **rating** (float from 1.0 to 5.0)
- A **generated review** aligned with the user's inferred style
- A **critic verdict** showing whether the draft passed the fidelity check
- The **number of iterations** the critic/drafter loop used
- Per-node **execution timing** for profiling, rating, drafting, and critique

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
  "nigerian_flavor": true,
  "model_overrides": {
    "profiler": "gpt-5.4-mini",
    "rater": "gpt-5.4",
    "drafter": "gpt-5.4",
    "critic": "gpt-5.4"
  }
}
```

`provider` is optional and defaults to `openai`. If the requested provider is not available in the current environment, the handler returns `400` with the list of available providers.

`nigerian_flavor` is optional and defaults to `true`. When `true`, the Drafter localizes product context to Nigerian equivalents (Amazon → Jumia, USD → NGN, etc.) and injects Nigerian vernacular into the review. Set to `false` for a neutral, culturally generic review in standard English.

**Optional per-node model overrides:**

The `model_overrides` field (optional) allows specifying different LLM models for each pipeline node:

- `profiler` — override for the Profiler node (user behavioral analysis)
- `rater` — override for the Rater node (rating prediction)
- `drafter` — override for the Drafter node (review text generation)
- `critic` — override for the Critic node (behavioral fidelity QA)

If a node is omitted or the entire `model_overrides` field is absent, the provider's default model is used. This enables fine-grained control — for example, using a fast model like `gpt-5.4-mini` for profiling/drafting while using a more capable model like `gpt-5.4` for critical rating decisions.

Validation rules currently enforced in handler:

- `user_history` must contain at least 2 prior reviews and at most 50
- every history item must include `product_id`, `review_text`, and a `star_rating` between 1.0 and 5.0
- `target_product.product_id` is required
- `target_product.price` cannot be negative
- `target_product.rating` must be between 0.0 and 5.0
- `target_product.review_count` cannot be negative
- `provider` defaults to `openai` if omitted
- unknown/unavailable provider returns `400`
- `model_overrides` is optional; if provided, model names must be valid for the provider

### Response Schema

```json
{
  "generated_review": "Revised review that sounds more natural and direct.",
  "predicted_rating": 4.2,
  "critic_verdict": "PASS",
  "iterations_used": 2,
  "execution_timing": {
    "profiler_ms": 42,
    "rater_ms": 38,
    "drafter_ms": 51,
    "critic_ms": 47
  }
}
```

`critic_verdict` is either `PASS` or `MAX_ITERATIONS`. The handler returns the latest draft as `generated_review` and the final predicted rating as `predicted_rating`.

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
│ Agent 2: Rater               │ Predicts rating and stores reasoning
└─────────────┬────────────────┘
              │
              ▼
┌──────────────────────────────┐
│ Agent 3: Drafter             │ Optionally localizes product context to Nigerian
│                              │ settings (controlled by nigerian_flavor), then
│                              │ drafts the review text in the user's voice
└─────────────┬────────────────┘
              │
              ▼
┌──────────────────────────────┐
│ Agent 4: Critic              │ PASS/FAIL behavioral fidelity check
│                              │ with revision loop
└─────────────┬────────────────┘
              │
              ▼
Return response with generated_review, predicted_rating,
critic_verdict, iterations_used, execution_timing
```

Critic loop details:

- max 2 draft/critic iterations (`maxLoops = 2`)
- if critic returns `PASS`, current draft becomes `final_review`
- if loop cap is reached, latest draft is returned
- local fallback checks reject common AI-sounding phrasing before the LLM critic runs
- the HTTP handler enforces a 90 second workflow timeout

### Implementation Notes

Task A implementation currently lives in:

- [internal/handlers/task_a.go](internal/handlers/task_a.go) (endpoint contract + orchestration)
- [internal/pipeline/graph.go](internal/pipeline/graph.go) (node sequence + loop control)
- [internal/pipeline/nodes/](internal/pipeline/nodes/) (Profiler, Rater, Drafter, Critic, Localization)
- [internal/handlers/task_a_test.go](internal/handlers/task_a_test.go) (end-to-end handler test)

Notes on current behavior:

- Task A request/response structs are defined in handler/pipeline packages, not in [internal/models/schemas.go](internal/models/schemas.go).
- Task A uses provider `Complete` calls in pipeline nodes; it does not run a final `Humanize` pass like Task B.
- The handler surfaces `critic_verdict`, `iterations_used`, and `execution_timing` directly in the response.

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

**2-pass extraction.** Before the LLM generates search queries, we embed the last user turn and retrieve 5 representative corpus samples. These are fed to the Extractor as examples, grounding its queries in what actually exists in the DB. Prevents queries for items the corpus doesn't contain.

**50-candidate pool, 20-candidate reranker.** Retrieval fetches 50 candidates from pgvector (2× per-query fetch for better diversity across multi-vector union). The Gate sees the top 10 for quality evaluation. The Reranker sees the top 20 — enough for accurate psychographic ranking while bounding LLM output size and latency. The response trims to `limit` (default 10).

**Cold-start as an explicit state.** When `history` is empty the system has no intent signal and returning random items would score poorly. Instead it returns `requires_input: true` with a humanized clarifying question, making the multi-turn nature of the system explicit to the evaluator.

**Multi-LLM registry.** Providers are registered at startup based on which env vars are present. Switching providers requires only changing the `provider` field in the request — no code changes. Useful for ablation in the solution paper.

**Deterministic item IDs.** UUIDs are derived via SHA-1 from `domain + review_text`. The same review always gets the same ID across runs, enabling `ON CONFLICT DO NOTHING` dedup without a separate uniqueness check.

**search_text vs item_id:** The ETL worker truncates `search_text` to 1,000 characters for the embedding (long reviews degrade retrieval quality), but the item_id is computed from the *full* untruncated text before truncation. This means you cannot recompute item_id from `search_text` alone — the backfill script addresses this by re-streaming the original reviews file rather than querying the DB's search_text.

---

## Limitations

| Limitation | Detail |
| ---------- | ------ |
| **Yelp business names** | The SetFit/yelp_review_full dataset strips business names — only star labels and review text are available. The full Yelp Open Dataset (business names included) requires an academic license. `/recommend` omits `name` for all Yelp items. |
| **Amazon Books metadata** | The SNAP `meta_Books.json.gz` is ~2 GB and uses Python-literal dict format. The backfill parses it via regex fallback, which may miss titles containing apostrophes. |
| **Single embedder thread** | The ONNX embedder sidecar runs on CPU. On a weak VM (2 vCPU), docker resource limits (`--cpus 1.0`) prevent embedder from consuming all cores and triggering Azure VM deallocation. This caps throughput to ~820 items/min. |
| **Yelp HTTP streaming** | The 485 MB Yelp JSONL file streams over HTTP. Very slow networks may time out at the TCP level even without an application timeout. Use `YELP_FILE` env var + volume mount for reliable offline operation. |
