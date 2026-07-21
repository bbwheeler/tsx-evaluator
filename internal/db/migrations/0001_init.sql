CREATE TABLE IF NOT EXISTS evaluations (
    symbol          TEXT PRIMARY KEY,
    financials      DOUBLE PRECISION NOT NULL,
    sentiment       DOUBLE PRECISION NOT NULL,
    leadership      DOUBLE PRECISION NOT NULL,
    type_sentiment  DOUBLE PRECISION NOT NULL,
    evaluated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_evaluations_evaluated_at ON evaluations (evaluated_at);
