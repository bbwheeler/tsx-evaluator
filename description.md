# TSX Evaluator

## Overall Purpose

The TSX Evaluator is a Go microservice that evaluates stocks listed on the Toronto Stock Exchange (TSX) across four dimensions: financial health, sentiment, leadership quality, and sector/industry outlook. It combines quantitative metrics (Piotroski F-Score, custom health indicators, executive tenure analysis) with LLM-powered news sentiment analysis and exposes results through a gRPC API.

---

## System Capabilities

### 1. Background Evaluation Loop

The evaluator runs as a persistent background process that periodically:

1. Connects to a companion `tsx-tracker` gRPC service to fetch the full list of tracked stock symbols
2. Selects a configurable batch of symbols to evaluate each cycle (prioritizing symbols that haven't been evaluated yet)
3. Fetches real financial data for each symbol from the Financial Modeling Prep (FMP) API
4. Computes financial health scores and persists them to PostgreSQL

The evaluation interval and batch size are configurable via environment variables (`EVAL_INTERVAL`, `EVAL_BATCH_SIZE`).

### 2. Financial Health Scoring

Each stock receives four score components (all in the range `[-1, 1]`):

| Metric | Description |
|---|---|---|
| **Financials** | Composite score from Piotroski F-Score (60%) and custom health metrics (40%) |
| **Sentiment** | LLM-analyzed sentiment from news articles and RSS feeds |
| **Leadership** | Leadership quality score based on executive tenure (60%) and CEO/CFO stability (40%) |
| **TypeSentiment** | Sector/industry outlook sentiment based on the company's business classification |

#### Piotroski F-Score Calculation

The Piotroski F-Score is a 0–9 accounting-based score that evaluates financial strength across three categories:

- **Profitability**: Positive ROA, positive cash flow proxy, increasing ROA, quality earnings
- **Leverage & Liquidity**: Decreasing leverage, increasing liquidity, no share dilution
- **Efficiency**: Increasing gross margin, increasing asset turnover

The raw score (0–9) is normalized to `[-1, 1]` where 4.5 maps to 0 (neutral).

#### Custom Health Metrics

Supplementary metrics that complement the Piotroski analysis:

- **Debt-to-Equity Ratio** — Lower values score higher (0 → +1, 1.5 → 0, 3+ → -1)
- **Current Ratio** — Higher values score higher (0 → -1, 1.5 → 0, 3+ → +1)
- **Revenue Growth** — Year-over-year change (+50% → +1, -50% → -1)
- **Net Margin** — Profitability relative to revenue (20% → +1, -20% → -1)

#### Final Score Composition

```
final = 0.6 × piotroski_normalized + 0.4 × custom_metrics_score
```

### 3. Leadership Quality Scoring

The leadership evaluator assesses the quality and stability of a company's executive team using data from the Financial Modeling Prep (FMP) API.

#### Data Source

- **FMP Key Executives API** — Returns executive names, titles, ages, gender, compensation, and tenure since year

#### Scoring Components

| Component | Weight | Description |
|---|---|---|
| **Tenure Score** | 60% | Average executive tenure across all listed executives |
| **Stability Score** | 40% | CEO and CFO individual tenure stability |

#### Tenure Score Thresholds

- **< 2 years**: -1.0 to -0.5 (new team, high risk)
- **2–5 years**: 0.0 to 0.5 (established)
- **5+ years**: 0.5 to 1.0 (stable, experienced)

#### Stability Score Thresholds

- **< 1 year**: -1.0 (very new)
- **1–3 years**: -0.5 to 0.0
- **3–7 years**: 0.0 to 0.5
- **7+ years**: 0.5 to 1.0

#### Final Score

```
final = 0.6 × tenure_score + 0.4 × stability_score
```

**Note:** Insider sentiment scoring is skipped for TSX/Canadian companies because the SEC EDGAR database (used for insider data) only covers US-listed companies.

### 4. Type Sentiment (Sector/Industry Outlook)

The type sentiment evaluator measures the overall sentiment and outlook for the sector or industry that a company operates in. Two companies might have identical financials, but one in Technology and one in Energy could perform very differently based on sector-level trends.

#### Data Sources

- **FMP Company Profile API** — Retrieves the company's `sector` and `industry` classification
- **Google News RSS** — Fetches sector-level news (e.g., "Technology sector stocks outlook", "Energy stocks market trend")

#### How It Works

1. **Profile Lookup** — Fetches the company's profile to determine its sector and industry
2. **Sector News Collection** — Searches for news about the sector/industry using multiple queries
3. **LLM Analysis** — The same LLM analyzes sector-level sentiment with a sector-focused prompt
4. **Score Return** — Returns a score in `[-1, 1]` reflecting the sector outlook

#### Score Interpretation

- `-1.0` = Extremely bearish sector outlook (major decline, regulatory crackdown, obsolescence risk)
- `-0.5` = Bearish outlook (slowing growth, headwinds, competitive pressure)
- `0.0` = Neutral/mixed outlook
- `0.5` = Bullish outlook (strong growth, favorable trends, tailwinds)
- `1.0` = Extremely bullish sector outlook (boom conditions, massive growth, favorable regulation)

### 5. Sentiment Analysis

The sentiment scoring system collects news and social media data from multiple free sources, then uses a configurable LLM (Ollama) to analyze the overall sentiment.

#### Data Sources

| Source | Type | Rate Limit | Description |
|---|---|---|---|
| **Yahoo Finance RSS** | RSS Feed | None | Company-specific news headlines |
| **Google News RSS** | RSS Feed | None | Aggregated news from multiple sources |

All data sources are **free** and require no API keys.

#### LLM Integration

The system sends collected headlines and snippets to an Ollama-compatible LLM endpoint for analysis:

1. **Data Collection** — Fetches 20-35 articles per symbol from all sources concurrently
2. **Prompt Construction** — Formats articles into a structured prompt for the LLM
3. **Sentiment Scoring** — LLM returns a JSON response with score, reasoning, and confidence
4. **Score Normalization** — Response is parsed and clamped to `[-1, 1]`

**Score Interpretation:**
- `-1.0` = Extremely negative (lawsuits, bankruptcy, major losses)
- `-0.5` = Negative (missed earnings, declining sales)
- `0.0` = Neutral/mixed sentiment
- `0.5` = Positive (good earnings, growth, upgrades)
- `1.0` = Extremely positive (record profits, major deals)

### 6. Data Source Integration

The system integrates with the **Financial Modeling Prep (FMP) API** to fetch:

- Income statements (revenue, gross profit, net income, EBITDA, etc.)
- Balance sheet data (total assets, debt, equity, current assets/liabilities, etc.)
- Cash flow statements (operating cash flow, free cash flow, dividends paid)

Data is fetched for the current and prior year to enable year-over-year comparisons required by the Piotroski calculation.

### 7. Data Persistence

Evaluation results are stored in a PostgreSQL database with the following schema:

```sql
CREATE TABLE evaluations (
    symbol          TEXT PRIMARY KEY,
    financials      DOUBLE PRECISION NOT NULL,
    sentiment       DOUBLE PRECISION NOT NULL,
    leadership      DOUBLE PRECISION NOT NULL,
    type_sentiment  DOUBLE PRECISION NOT NULL,
    evaluated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Each symbol has exactly one row, updated via upsert semantics (INSERT ... ON CONFLICT DO UPDATE).

### 8. gRPC API

The service exposes three RPC endpoints:

| RPC | Description |
|---|---|
| `GetScores` | Retrieve evaluation scores for a single stock symbol |
| `ListScoredStocks` | List all evaluated stocks with sorting and pagination |
| `EvaluateNow` | Trigger an immediate evaluation (on-demand) |

#### Sorting and Composite Scores

`ListScoredStocks` supports sorting by:

- **Single metric** — Sort by any individual score component (e.g., `FINANCIALS`, `SENTIMENT`)
- **Weighted composite** — Sort by a user-defined weighted combination of all four metrics (e.g., `w_f*financials + w_s*sentiment + w_l*leadership + w_t*type_sentiment`)

Results are paginated with cursor-based pagination (using the symbol as the cursor).

### 9. Service Integration

The evaluator is designed to run as part of a larger stock analysis stack:

- **tsx-tracker** — Companion service that tracks TSX stock listings; the evaluator fetches the list of symbols to evaluate from this service via gRPC
- **PostgreSQL** — Shared or dedicated database instance
- **Docker Compose** — Provided for local development, orchestrating the evaluator, tracker, and database

### 10. Configuration

All configuration is via environment variables:

| Variable | Default | Description |
|---|---|---|
| `GRPC_PORT` | `50052` | gRPC server listen port |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `postgres` | PostgreSQL user |
| `DB_PASSWORD` | `postgres` | PostgreSQL password |
| `DB_NAME` | `tsx_evaluator` | PostgreSQL database name |
| `DB_SSLMODE` | `disable` | PostgreSQL SSL mode |
| `TRACKER_ADDR` | `localhost:50051` | gRPC address of the tsx-tracker service |
| `EVAL_INTERVAL` | `5m` | Time between evaluation cycles |
| `EVAL_BATCH_SIZE` | `1` | Number of stocks to evaluate per cycle |
| `FMP_API_KEY` | (none) | Financial Modeling Prep API key |
| `FMP_BASE_URL` | `https://financialmodelingprep.com` | FMP API base URL |
| `LLM_BASE_URL` | `http://localhost:11434` | Ollama API endpoint |
| `LLM_MODEL` | `llama3` | LLM model for sentiment analysis |
| `LLM_TIMEOUT` | `60s` | Timeout for LLM requests |
