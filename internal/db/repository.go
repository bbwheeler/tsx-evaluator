package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("evaluation not found")

type ScoreSet struct {
	Symbol       string
	Financials   float64
	Sentiment    float64
	Leadership   float64
	TypeSentiment float64
	EvaluatedAt  time.Time
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(ctx context.Context, dsn string) (*Repository, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Repository{pool: pool}, nil
}

func (r *Repository) Close() {
	r.pool.Close()
}

func (r *Repository) Migrate(ctx context.Context, migrationSQL string) error {
	_, err := r.pool.Exec(ctx, migrationSQL)
	if err != nil {
		return fmt.Errorf("run migration: %w", err)
	}
	return nil
}

func (r *Repository) UpsertScores(ctx context.Context, s *ScoreSet) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO evaluations (symbol, financials, sentiment, leadership, type_sentiment, evaluated_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (symbol) DO UPDATE SET
			financials = EXCLUDED.financials,
			sentiment = EXCLUDED.sentiment,
			leadership = EXCLUDED.leadership,
			type_sentiment = EXCLUDED.type_sentiment,
			evaluated_at = now()
	`, s.Symbol, s.Financials, s.Sentiment, s.Leadership, s.TypeSentiment)
	if err != nil {
		return fmt.Errorf("upsert scores: %w", err)
	}
	return nil
}

func (r *Repository) GetBySymbol(ctx context.Context, symbol string) (*ScoreSet, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT symbol, financials, sentiment, leadership, type_sentiment, evaluated_at
		 FROM evaluations WHERE upper(symbol) = upper($1)`, symbol)
	return scanScore(row)
}

// ListOrdered returns evaluations sorted by a computed expression.
// orderExpr must be a safe SQL expression referencing the evaluations columns,
// e.g. "financials" or "0.3*financials + 0.2*sentiment + 0.4*leadership + 0.1*type_sentiment".
func (r *Repository) ListOrdered(ctx context.Context, orderExpr string, descending bool, afterSymbol string, pageSize int) ([]ScoreSet, error) {
	dir := "ASC"
	if descending {
		dir = "DESC"
	}

	query := fmt.Sprintf(
		`SELECT symbol, financials, sentiment, leadership, type_sentiment, evaluated_at
		 FROM evaluations
		 WHERE upper(symbol) > upper($1)
		 ORDER BY %s %s, symbol ASC
		 LIMIT $2`, orderExpr, dir)

	rows, err := r.pool.Query(ctx, query, afterSymbol, pageSize)
	if err != nil {
		return nil, fmt.Errorf("list ordered: %w", err)
	}
	defer rows.Close()

	var out []ScoreSet
	for rows.Next() {
		s, err := scanScore(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

func (r *Repository) CountAll(ctx context.Context) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM evaluations`).Scan(&n)
	return n, err
}

// EvaluatedSymbols returns all symbols that have been evaluated.
func (r *Repository) EvaluatedSymbols(ctx context.Context) (map[string]struct{}, error) {
	rows, err := r.pool.Query(ctx, `SELECT symbol FROM evaluations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]struct{})
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out[s] = struct{}{}
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanScore(row rowScanner) (*ScoreSet, error) {
	var s ScoreSet
	err := row.Scan(
		&s.Symbol, &s.Financials, &s.Sentiment, &s.Leadership,
		&s.TypeSentiment, &s.EvaluatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}

// BuildCompositeExpr returns a SQL expression for a weighted combination of scores.
func BuildCompositeExpr(w Financials, s Sentiment, l Leadership, t TypeSentiment) string {
	return fmt.Sprintf(
		"(%.6f*financials + %.6f*sentiment + %.6f*leadership + %.6f*type_sentiment)",
		w, s, l, t,
	)
}

type (
	Financials  = float64
	Sentiment   = float64
	Leadership  = float64
	TypeSentiment = float64
)

// ScoreMetricToColumn maps a metric enum name to its column.
// It accepts both short names (e.g. "SENTIMENT") and full proto enum
// names (e.g. "SCORE_METRIC_SENTIMENT").
func ScoreMetricToColumn(metric string) (string, bool) {
	cols := map[string]string{
		"FINANCIALS":              "financials",
		"SENTIMENT":               "sentiment",
		"LEADERSHIP":              "leadership",
		"TYPE_SENTIMENT":          "type_sentiment",
		"SCORE_METRIC_FINANCIALS": "financials",
		"SCORE_METRIC_SENTIMENT":  "sentiment",
		"SCORE_METRIC_LEADERSHIP": "leadership",
		"SCORE_METRIC_TYPE_SENTIMENT": "type_sentiment",
	}
	c, ok := cols[strings.ToUpper(metric)]
	return c, ok
}
