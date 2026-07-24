package evaluator

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/example/tsx-evaluator/internal/analyzer"
	"github.com/example/tsx-evaluator/internal/config"
	"github.com/example/tsx-evaluator/internal/db"
	"github.com/example/tsx-evaluator/internal/finance"
	"github.com/example/tsx-evaluator/internal/leadership"
	"github.com/example/tsx-evaluator/internal/sentiment"
	"github.com/example/tsx-evaluator/internal/typesentiment"

	tsxv1 "github.com/example/tsx-tracker/gen/tsx/v1"
)

type Store interface {
	EvaluatedSymbols(ctx context.Context) (map[string]struct{}, error)
	UpsertScores(ctx context.Context, s *db.ScoreSet) error
}

type Evaluator struct {
	cfg       *config.Config
	repo      Store
	finCli    *finance.Client
	sentEv    *sentiment.Evaluator
	leadEv    *leadership.Evaluator
	typeSentEv *typesentiment.Evaluator
	log       *slog.Logger
}

func New(cfg *config.Config, repo Store, finCli *finance.Client, sentEv *sentiment.Evaluator, leadEv *leadership.Evaluator, typeSentEv *typesentiment.Evaluator, log *slog.Logger) *Evaluator {
	return &Evaluator{cfg: cfg, repo: repo, finCli: finCli, sentEv: sentEv, leadEv: leadEv, typeSentEv: typeSentEv, log: log}
}

func (e *Evaluator) Run(ctx context.Context) {
	e.log.Info("evaluator starting",
		"interval", e.cfg.EvalInterval,
		"batch_size", e.cfg.EvalBatchSize,
		"tracker", e.cfg.TrackerAddr)

	// Run an immediate evaluation pass, then on interval.
	e.cycle(ctx)

	ticker := time.NewTicker(e.cfg.EvalInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			e.log.Info("evaluator stopping")
			return
		case <-ticker.C:
			e.cycle(ctx)
		}
	}
}

func (e *Evaluator) cycle(ctx context.Context) {
	conn, err := grpc.NewClient(e.cfg.TrackerAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		e.log.Error("connect to tracker", "error", err)
		return
	}
	defer conn.Close()

	client := tsxv1.NewCompanyServiceClient(conn)

	// Fetch all companies (paginate through full list).
	symbols, err := e.fetchAllSymbols(ctx, client)
	if err != nil {
		e.log.Error("fetch symbols from tracker", "error", err)
		return
	}
	if len(symbols) == 0 {
		e.log.Warn("no companies returned from tracker")
		return
	}

	// Pick random symbols to evaluate this cycle.
	batch := min(e.cfg.EvalBatchSize, len(symbols))
	evaluated := 0

	// Get already-evaluated symbols to prioritise un-evaluated ones.
	existing, _ := e.repo.EvaluatedSymbols(ctx)

	var unevaluated []string
	for _, s := range symbols {
		if _, done := existing[s]; !done {
			unevaluated = append(unevaluated, s)
		}
	}

	// Evaluate unevaluated symbols first, then re-evaluate old ones.
	candidates := unevaluated
	if len(candidates) < batch {
		remaining := batch - len(candidates)
		for _, s := range symbols {
			if remaining <= 0 {
				break
			}
			if _, done := existing[s]; done {
				candidates = append(candidates, s)
				remaining--
			}
		}
	}

	rand.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	for _, symbol := range candidates[:batch] {
		scores := analyzer.Analyze(ctx, e.finCli, e.sentEv, e.leadEv, e.typeSentEv, symbol, e.log)
		if err := e.repo.UpsertScores(ctx, scores); err != nil {
			e.log.Error("store scores", "symbol", symbol, "error", err)
			continue
		}
		evaluated++
		e.log.Info("evaluated", "symbol", symbol,
			"financials", scores.Financials,
			"sentiment", scores.Sentiment,
			"leadership", scores.Leadership,
			"type_sentiment", scores.TypeSentiment)
	}

	e.log.Info("evaluation cycle complete",
		"total_symbols", len(symbols),
		"evaluated", evaluated)
}

func (e *Evaluator) fetchAllSymbols(ctx context.Context, client tsxv1.CompanyServiceClient) ([]string, error) {
	var symbols []string
	var pageToken string

	for {
		resp, err := client.ListCompanies(ctx, &tsxv1.ListCompaniesRequest{
			PageSize:  500,
			PageToken: pageToken,
		})
		if err != nil {
			return nil, fmt.Errorf("list companies: %w", err)
		}
		for _, c := range resp.Companies {
			symbols = append(symbols, c.Symbol)
		}
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}
	return symbols, nil
}
