CREATE TABLE IF NOT EXISTS evaluations (
    symbol              TEXT PRIMARY KEY,
    financials          DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    sentiment           DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    leadership          DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    type_sentiment      DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    growth_momentum     DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    valuation_fairness  DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    evaluated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_evaluations_evaluated_at ON evaluations (evaluated_at);
CREATE INDEX IF NOT EXISTS idx_evaluations_financials      ON evaluations (financials DESC);
CREATE INDEX IF NOT EXISTS idx_evaluations_growth_momentum  ON evaluations (growth_momentum DESC);
CREATE INDEX IF NOT EXISTS idx_evaluations_valuation_fairness ON evaluations (valuation_fairness DESC);
