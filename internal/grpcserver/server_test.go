package grpcserver

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	stockerv1 "github.com/example/stocker-evaluator/gen/tsx/v1"
	"github.com/example/stocker-evaluator/internal/db"
	"github.com/example/stocker-evaluator/internal/finance"
	"github.com/example/stocker-evaluator/internal/leadership"
	"github.com/example/stocker-evaluator/internal/sentiment"
	"github.com/example/stocker-evaluator/internal/typesentiment"
)

// mockStore implements the Store interface for testing.
type mockStore struct {
	getBySymbolFn  func(ctx context.Context, symbol string) (*db.ScoreSet, error)
	listOrderedFn  func(ctx context.Context, orderExpr string, descending bool, afterSymbol string, pageSize int) ([]db.ScoreSet, error)
	countAllFn     func(ctx context.Context) (int, error)
	upsertScoresFn func(ctx context.Context, s *db.ScoreSet) error
}

func (m *mockStore) GetBySymbol(ctx context.Context, symbol string) (*db.ScoreSet, error) {
	return m.getBySymbolFn(ctx, symbol)
}

func (m *mockStore) ListOrdered(ctx context.Context, orderExpr string, descending bool, afterSymbol string, pageSize int) ([]db.ScoreSet, error) {
	return m.listOrderedFn(ctx, orderExpr, descending, afterSymbol, pageSize)
}

func (m *mockStore) CountAll(ctx context.Context) (int, error) {
	return m.countAllFn(ctx)
}

func (m *mockStore) UpsertScores(ctx context.Context, s *db.ScoreSet) error {
	return m.upsertScoresFn(ctx, s)
}

func newTestServer(store Store) *Server {
	llmClient := sentiment.NewLLMClient("http://localhost:11434", "llama3", 60)
	sentEv := sentiment.NewEvaluator(llmClient, slog.Default())
	leadEv := leadership.NewEvaluator(leadership.NewYahooClient(), slog.Default())
	typeSentEv := typesentiment.NewEvaluator(typesentiment.NewProfileClient(), llmClient, slog.Default())
	return New(store, testFinanceClient(), sentEv, leadEv, typeSentEv, slog.Default())
}

func testFinanceClient() *finance.Client {
	return finance.NewClientWithBaseURL("http://localhost:0")
}

func TestGetScores_EmptySymbol(t *testing.T) {
	srv := newTestServer(&mockStore{})
	_, err := srv.GetScores(context.Background(), &stockerv1.GetScoresRequest{Symbol: ""})
	if err == nil {
		t.Fatal("expected error for empty symbol")
	}
	if c := status.Code(err); c != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", c)
	}
}

func TestGetScores_NotFound(t *testing.T) {
	srv := newTestServer(&mockStore{
		getBySymbolFn: func(_ context.Context, _ string) (*db.ScoreSet, error) {
			return nil, db.ErrNotFound
		},
	})
	_, err := srv.GetScores(context.Background(), &stockerv1.GetScoresRequest{Symbol: "MISSING"})
	if err == nil {
		t.Fatal("expected error for missing symbol")
	}
	if c := status.Code(err); c != codes.NotFound {
		t.Errorf("expected NotFound, got %v", c)
	}
}

func TestGetScores_InternalError(t *testing.T) {
	srv := newTestServer(&mockStore{
		getBySymbolFn: func(_ context.Context, _ string) (*db.ScoreSet, error) {
			return nil, context.Canceled
		},
	})
	_, err := srv.GetScores(context.Background(), &stockerv1.GetScoresRequest{Symbol: "ERR"})
	if c := status.Code(err); c != codes.Internal {
		t.Errorf("expected Internal, got %v", c)
	}
}

func TestGetScores_Success(t *testing.T) {
	now := time.Now()
	srv := newTestServer(&mockStore{
		getBySymbolFn: func(_ context.Context, symbol string) (*db.ScoreSet, error) {
			return &db.ScoreSet{
				Symbol:        symbol,
				Financials:    0.5,
				Sentiment:     -0.3,
				Leadership:    0.8,
				TypeSentiment: 0.1,
				EvaluatedAt:   now,
			}, nil
		},
	})
	resp, err := srv.GetScores(context.Background(), &stockerv1.GetScoresRequest{Symbol: "SHOP.TO"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Scores.Symbol != "SHOP.TO" {
		t.Errorf("symbol: got %q, want %q", resp.Scores.Symbol, "SHOP.TO")
	}
	if resp.Scores.Financials != 0.5 {
		t.Errorf("financials: got %v, want 0.5", resp.Scores.Financials)
	}
	if resp.Scores.Sentiment != -0.3 {
		t.Errorf("sentiment: got %v, want -0.3", resp.Scores.Sentiment)
	}
	if resp.Scores.Leadership != 0.8 {
		t.Errorf("leadership: got %v, want 0.8", resp.Scores.Leadership)
	}
	if resp.Scores.TypeSentiment != 0.1 {
		t.Errorf("type_sentiment: got %v, want 0.1", resp.Scores.TypeSentiment)
	}
}

func TestListScoredStocks_DefaultPageSize(t *testing.T) {
	var capturedPageSize int
	srv := newTestServer(&mockStore{
		listOrderedFn: func(_ context.Context, _ string, _ bool, _ string, pageSize int) ([]db.ScoreSet, error) {
			capturedPageSize = pageSize
			return nil, nil
		},
		countAllFn: func(_ context.Context) (int, error) {
			return 0, nil
		},
	})
	_, err := srv.ListScoredStocks(context.Background(), &stockerv1.ListScoredStocksRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedPageSize != defaultPageSize {
		t.Errorf("page size: got %d, want %d", capturedPageSize, defaultPageSize)
	}
}

func TestListScoredStocks_CapsAtMaxPageSize(t *testing.T) {
	var capturedPageSize int
	srv := newTestServer(&mockStore{
		listOrderedFn: func(_ context.Context, _ string, _ bool, _ string, pageSize int) ([]db.ScoreSet, error) {
			capturedPageSize = pageSize
			return nil, nil
		},
		countAllFn: func(_ context.Context) (int, error) {
			return 0, nil
		},
	})
	_, err := srv.ListScoredStocks(context.Background(), &stockerv1.ListScoredStocksRequest{PageSize: 1000})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedPageSize != maxPageSize {
		t.Errorf("page size: got %d, want %d", capturedPageSize, maxPageSize)
	}
}

func TestListScoredStocks_ClapsToMin(t *testing.T) {
	var capturedPageSize int
	srv := newTestServer(&mockStore{
		listOrderedFn: func(_ context.Context, _ string, _ bool, _ string, pageSize int) ([]db.ScoreSet, error) {
			capturedPageSize = pageSize
			return nil, nil
		},
		countAllFn: func(_ context.Context) (int, error) {
			return 0, nil
		},
	})
	_, err := srv.ListScoredStocks(context.Background(), &stockerv1.ListScoredStocksRequest{PageSize: -5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedPageSize != defaultPageSize {
		t.Errorf("page size: got %d, want %d", capturedPageSize, defaultPageSize)
	}
}

func TestListScoredStocks_SortByMetric(t *testing.T) {
	var capturedOrderExpr string
	var capturedDescending bool
	srv := newTestServer(&mockStore{
		listOrderedFn: func(_ context.Context, orderExpr string, descending bool, _ string, _ int) ([]db.ScoreSet, error) {
			capturedOrderExpr = orderExpr
			capturedDescending = descending
			return nil, nil
		},
		countAllFn: func(_ context.Context) (int, error) {
			return 0, nil
		},
	})
	_, err := srv.ListScoredStocks(context.Background(), &stockerv1.ListScoredStocksRequest{
		SortBy:     &stockerv1.ListScoredStocksRequest_Metric{Metric: stockerv1.ScoreMetric_SCORE_METRIC_SENTIMENT},
		Descending: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedOrderExpr != "sentiment" {
		t.Errorf("orderExpr: got %q, want %q", capturedOrderExpr, "sentiment")
	}
	if !capturedDescending {
		t.Error("expected descending=true")
	}
}

func TestListScoredStocks_SortByWeights(t *testing.T) {
	var capturedOrderExpr string
	srv := newTestServer(&mockStore{
		listOrderedFn: func(_ context.Context, orderExpr string, _ bool, _ string, _ int) ([]db.ScoreSet, error) {
			capturedOrderExpr = orderExpr
			return nil, nil
		},
		countAllFn: func(_ context.Context) (int, error) {
			return 0, nil
		},
	})
	_, err := srv.ListScoredStocks(context.Background(), &stockerv1.ListScoredStocksRequest{
		SortBy: &stockerv1.ListScoredStocksRequest_Weights{
			Weights: &stockerv1.ScoreWeights{
				Financials:    0.3,
				Sentiment:     0.2,
				Leadership:    0.4,
				TypeSentiment: 0.1,
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := db.BuildCompositeExpr(0.3, 0.2, 0.4, 0.1)
	if capturedOrderExpr != want {
		t.Errorf("orderExpr:\n  got  %q\n  want %q", capturedOrderExpr, want)
	}
}

func TestListScoredStocks_DefaultSortByDescending(t *testing.T) {
	var capturedDescending bool
	srv := newTestServer(&mockStore{
		listOrderedFn: func(_ context.Context, _ string, descending bool, _ string, _ int) ([]db.ScoreSet, error) {
			capturedDescending = descending
			return nil, nil
		},
		countAllFn: func(_ context.Context) (int, error) {
			return 0, nil
		},
	})
	_, err := srv.ListScoredStocks(context.Background(), &stockerv1.ListScoredStocksRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !capturedDescending {
		t.Error("default sort should set descending=true")
	}
}

func TestListScoredStocks_PaginationToken(t *testing.T) {
	srv := newTestServer(&mockStore{
		listOrderedFn: func(_ context.Context, _ string, _ bool, _ string, pageSize int) ([]db.ScoreSet, error) {
			scores := make([]db.ScoreSet, pageSize)
			for i := range scores {
				scores[i] = db.ScoreSet{Symbol: "SYM"}
			}
			return scores, nil
		},
		countAllFn: func(_ context.Context) (int, error) {
			return 100, nil
		},
	})
	resp, err := srv.ListScoredStocks(context.Background(), &stockerv1.ListScoredStocksRequest{PageSize: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.NextPageToken != "SYM" {
		t.Errorf("NextPageToken: got %q, want %q", resp.NextPageToken, "SYM")
	}
	if resp.TotalCount != 100 {
		t.Errorf("TotalCount: got %d, want 100", resp.TotalCount)
	}
}

func TestListScoredStocks_NoNextPageWhenFewerResults(t *testing.T) {
	srv := newTestServer(&mockStore{
		listOrderedFn: func(_ context.Context, _ string, _ bool, _ string, _ int) ([]db.ScoreSet, error) {
			return []db.ScoreSet{{Symbol: "A"}}, nil
		},
		countAllFn: func(_ context.Context) (int, error) {
			return 1, nil
		},
	})
	resp, err := srv.ListScoredStocks(context.Background(), &stockerv1.ListScoredStocksRequest{PageSize: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.NextPageToken != "" {
		t.Errorf("expected empty NextPageToken, got %q", resp.NextPageToken)
	}
}

func TestListScoredStocks_InternalError(t *testing.T) {
	srv := newTestServer(&mockStore{
		listOrderedFn: func(_ context.Context, _ string, _ bool, _ string, _ int) ([]db.ScoreSet, error) {
			return nil, context.Canceled
		},
		countAllFn: func(_ context.Context) (int, error) {
			return 0, nil
		},
	})
	_, err := srv.ListScoredStocks(context.Background(), &stockerv1.ListScoredStocksRequest{})
	if c := status.Code(err); c != codes.Internal {
		t.Errorf("expected Internal, got %v", c)
	}
}

func TestListScoredStocks_CountAllError(t *testing.T) {
	srv := newTestServer(&mockStore{
		listOrderedFn: func(_ context.Context, _ string, _ bool, _ string, _ int) ([]db.ScoreSet, error) {
			return nil, nil
		},
		countAllFn: func(_ context.Context) (int, error) {
			return 0, context.Canceled
		},
	})
	_, err := srv.ListScoredStocks(context.Background(), &stockerv1.ListScoredStocksRequest{})
	if c := status.Code(err); c != codes.Internal {
		t.Errorf("expected Internal, got %v", c)
	}
}

func TestEvaluateNow_Success(t *testing.T) {
	var upserted *db.ScoreSet
	srv := newTestServer(&mockStore{
		upsertScoresFn: func(_ context.Context, s *db.ScoreSet) error {
			upserted = s
			return nil
		},
	})
	resp, err := srv.EvaluateNow(context.Background(), &stockerv1.EvaluateNowRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Scores == nil {
		t.Fatal("expected non-nil scores in response")
	}
	if upserted == nil {
		t.Fatal("expected UpsertScores to be called")
	}
	if upserted.Symbol != "DUMMY" {
		t.Errorf("expected symbol DUMMY, got %q", upserted.Symbol)
	}
}

func TestEvaluateNow_UpsertError(t *testing.T) {
	srv := newTestServer(&mockStore{
		upsertScoresFn: func(_ context.Context, _ *db.ScoreSet) error {
			return context.Canceled
		},
	})
	_, err := srv.EvaluateNow(context.Background(), &stockerv1.EvaluateNowRequest{})
	if c := status.Code(err); c != codes.Internal {
		t.Errorf("expected Internal, got %v", c)
	}
}

func TestToProto(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	s := &db.ScoreSet{
		Symbol:        "SHOP.TO",
		Financials:    0.5,
		Sentiment:     -0.3,
		Leadership:    0.8,
		TypeSentiment: 0.1,
		EvaluatedAt:   now,
	}
	p := toProto(s)

	if p.Symbol != "SHOP.TO" {
		t.Errorf("symbol: got %q, want %q", p.Symbol, "SHOP.TO")
	}
	if p.Financials != 0.5 {
		t.Errorf("financials: got %v, want 0.5", p.Financials)
	}
	if p.Sentiment != -0.3 {
		t.Errorf("sentiment: got %v, want -0.3", p.Sentiment)
	}
	if p.Leadership != 0.8 {
		t.Errorf("leadership: got %v, want 0.8", p.Leadership)
	}
	if p.TypeSentiment != 0.1 {
		t.Errorf("type_sentiment: got %v, want 0.1", p.TypeSentiment)
	}
	if p.EvaluatedAt.AsTime() != now {
		t.Errorf("evaluated_at: got %v, want %v", p.EvaluatedAt.AsTime(), now)
	}
}
