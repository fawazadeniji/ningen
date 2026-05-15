# Ningen

AI-powered product review simulation and recommendation engine.

## Overview

Ningen uses LLM-based agents to:
- **Task A** (`POST /api/v1/simulate-review`): Generate realistic product reviews from a user persona.
- **Task B** (`POST /api/v1/recommend`): Produce contextual product recommendations for a given persona.

## Prerequisites

- Go 1.23+
- Docker & Docker Compose (for containerized runs)

## Setup

1. **Clone the repository:**
   ```bash
   git clone https://github.com/Firebreather-heart/ningen.git
   cd ningen
   ```

2. **Configure environment variables:**
   ```bash
   cp .env.example .env
   # Edit .env with your API keys
   ```

3. **Install dependencies:**
   ```bash
   go mod download
   ```

4. **Run locally:**
   ```bash
   go run ./cmd/api
   ```

5. **Run with Docker:**
   ```bash
   docker compose up --build
   ```

## API Endpoints

### `POST /api/v1/simulate-review`

Generate a simulated product review.

**Request Body:**
```json
{
  "persona": {
    "name": "Jane Doe",
    "age": 28,
    "interests": ["tech", "gaming"],
    "tech_savvy": "high"
  },
  "product": {
    "id": "prod-001",
    "name": "Wireless Headphones",
    "category": "Electronics",
    "price": 79.99
  }
}
```

### `POST /api/v1/recommend`

Get personalized product recommendations.

**Request Body:**
```json
{
  "persona": {
    "name": "Jane Doe",
    "age": 28,
    "interests": ["tech", "gaming"],
    "budget_range": "$50-200"
  }
}
```

## Project Structure

```
ningen/
├── cmd/api/          # Application entry point
├── internal/
│   ├── api/          # HTTP handlers and routing
│   ├── agents/       # LLM agent workflows
│   ├── models/       # Shared data structures
│   └── config/       # Environment and configuration
├── docs/             # Documentation and solution paper
├── data/             # Sample datasets (gitignored)
├── Dockerfile
├── docker-compose.yml
└── .env.example
```

## License

See [LICENSE](LICENSE).