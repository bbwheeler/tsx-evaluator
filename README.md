# TSX Evaluator

A Go microservice that evaluates financial health and sentiment of TSX-listed stocks using the Piotroski F-Score, custom financial metrics, and LLM-powered sentiment analysis. Exposes results via gRPC.

## Prerequisites

- Go 1.25+
- PostgreSQL 16+
- [buf](https://buf.build) (for protobuf generation)
- Docker & Docker Compose (optional, for containerized setup)
- A [Financial Modeling Prep](https://financialmodelingprep.com) API key
- [Ollama](https://ollama.ai) with a pulled model (e.g., `ollama pull llama3`)

## Quick Start

### Docker Compose (recommended)

```bash
cp .env.example .env
# Edit .env and set FMP_API_KEY

docker compose up --build
```

This starts the evaluator, tsx-tracker, and PostgreSQL.

### Local Development

1. Start PostgreSQL and create the database:

```bash
createdb tsx_evaluator
```

2. Set environment variables (see [Configuration](#configuration)) or copy the example:

```bash
cp .env.example .env
# Edit .env
source .env
```

3. Generate protobuf code and run:

```bash
make run
```

## Build

```bash
make build
# Binary: bin/tsx-evaluator
```

## Configuration

| Variable | Default | Description |
|---|---|---|
| `GRPC_PORT` | `50052` | gRPC server port |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `postgres` | PostgreSQL user |
| `DB_PASSWORD` | `postgres` | PostgreSQL password |
| `DB_NAME` | `tsx_evaluator` | PostgreSQL database |
| `DB_SSLMODE` | `disable` | PostgreSQL SSL mode |
| `TRACKER_ADDR` | `localhost:50051` | tsx-tracker gRPC address |
| `EVAL_INTERVAL` | `5m` | Time between evaluation cycles |
| `EVAL_BATCH_SIZE` | `1` | Stocks evaluated per cycle |
| `FMP_API_KEY` | | FMP API key (required) |
| `FMP_BASE_URL` | `https://financialmodelingprep.com` | FMP API base URL |
| `LLM_BASE_URL` | `http://localhost:11434` | Ollama API endpoint |
| `LLM_MODEL` | `llama3` | LLM model for sentiment analysis |
| `LLM_TIMEOUT` | `60s` | Timeout for LLM requests |

## gRPC API

The service exposes three RPCs on the port specified by `GRPC_PORT`.

### GetScores

Returns evaluation scores for a single symbol.

```protobuf
rpc GetScores(GetScoresRequest) returns (GetScoresResponse);

message GetScoresRequest {
  string symbol = 1;
}

message GetScoresResponse {
  ScoreSet scores = 1;
}
```

### ListScoredStocks

Lists all evaluated stocks with sorting and pagination. Sort by a single metric or a weighted composite.

```protobuf
rpc ListScoredStocks(ListScoredStocksRequest) returns (ListScoredStocksResponse);

message ListScoredStocksRequest {
  oneof sort_by {
    ScoreMetric metric = 1;
    ScoreWeights weights = 2;
  }
  int32 page_size = 3;
  string page_token = 4;
  bool descending = 5;
}
```

### EvaluateNow

Triggers an immediate evaluation (placeholder implementation).

```protobuf
rpc EvaluateNow(EvaluateNowRequest) returns (EvaluateNowResponse);

message EvaluateNowRequest {}
message EvaluateNowResponse {
  ScoreSet scores = 1;
}
```

## Scoring

Each stock receives four metrics in the range `[-1, 1]`:

| Metric | Description |
|---|---|
| `financials` | 60% Piotroski F-Score + 40% custom health metrics |
| `sentiment` | LLM-analyzed sentiment from news and social media |
| `leadership` | 60% average executive tenure + 40% CEO/CFO stability |
| `type_sentiment` | Sector/industry outlook sentiment based on company classification |

The `financials` score is composed of:

- **Piotroski F-Score (0–9)**: Profitability, leverage/liquidity, and efficiency signals comparing current and prior year financials
- **Custom metrics**: Debt-to-equity ratio, current ratio, revenue growth, net margin

The `sentiment` score is computed by:

1. Fetching news headlines from Yahoo Finance RSS, Google News RSS, and Reddit posts
2. Sending the collected text to an LLM (Ollama) for analysis
3. Parsing the LLM's response to extract a score between -1.0 (extremely negative) and 1.0 (extremely positive)

The `leadership` score is computed by:

1. Fetching executive data from the FMP Key Executives API
2. Calculating average tenure across all listed executives (60% weight)
3. Calculating stability score from CEO and CFO individual tenures (40% weight)
4. Combining into a final score in [-1, 1]

The `type_sentiment` score is computed by:

1. Fetching the company's profile from FMP to determine sector and industry
2. Searching for sector-level news using Google News RSS (e.g., "Technology sector stocks outlook")
3. Sending the collected articles to the LLM with a sector-focused prompt
4. Returning a score reflecting the sector's overall outlook and sentiment

## Project Structure

```
├── cmd/server/          Entry point
├── internal/
│   ├── analyzer/        Orchestrates all four scoring modules per symbol
│   ├── config/          Environment-based configuration
│   ├── db/              PostgreSQL repository, migrations, schema
│   ├── evaluator/       Background evaluation loop
│   ├── finance/         FMP API client, Piotroski calculation, scoring
│   ├── grpcserver/      gRPC service implementation
│   ├── leadership/      Executive tenure/stability scoring via FMP
│   ├── sentiment/       News aggregation + LLM sentiment analysis
│   └── typesentiment/   Sector/industry outlook scoring via FMP + LLM
├── proto/               Protobuf definitions
├── gen/                 Generated protobuf/gRPC code
├── docker-compose.yml   Container orchestration
└── Makefile             Build and development commands
```

## Sentiment Analysis

The sentiment scoring system uses free data sources and a local LLM:

**Data Sources:**
- Yahoo Finance RSS: `https://feeds.finance.yahoo.com/rss/2.0/headline?s={symbol}`
- Google News RSS: `https://news.google.com/rss/search?q={symbol}+stock`
- Reddit JSON: Posts from r/wallstreetbets, r/stocks, r/investing

**LLM Integration:**
- Compatible with Ollama API (`/api/chat` endpoint)
- Default model: `llama3`
- Sends ~20-35 articles per symbol for analysis
- Returns JSON with score, reasoning, and confidence

## Make Targets

| Target | Description |
|---|---|
| `make proto` | Generate protobuf code with buf |
| `make proto-protoc` | Generate protobuf code with protoc |
| `make tidy` | Run `go mod tidy` |
| `make build` | Build binary to `bin/tsx-evaluator` |
| `make run` | Build and run the server |
| `make docker-up` | Start all services with Docker Compose |
| `make docker-down` | Stop and remove all services |
