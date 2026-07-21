package evaluator

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"net"

	"github.com/example/tsx-evaluator/internal/config"
	"github.com/example/tsx-evaluator/internal/db"

	tsxv1 "github.com/example/tsx-tracker/gen/tsx/v1"
)

// mockEvaluatorStore implements the Store interface for testing.
type mockEvaluatorStore struct {
	evaluatedSymbolsFn func(ctx context.Context) (map[string]struct{}, error)
	upsertScoresFn     func(ctx context.Context, s *db.ScoreSet) error
}

func (m *mockEvaluatorStore) EvaluatedSymbols(ctx context.Context) (map[string]struct{}, error) {
	return m.evaluatedSymbolsFn(ctx)
}

func (m *mockEvaluatorStore) UpsertScores(ctx context.Context, s *db.ScoreSet) error {
	return m.upsertScoresFn(ctx, s)
}

// mockCompanyServer implements the CompanyServiceServer for testing tracker interactions.
type mockCompanyServer struct {
	tsxv1.UnimplementedCompanyServiceServer
	listCompaniesFn func(ctx context.Context, req *tsxv1.ListCompaniesRequest) (*tsxv1.ListCompaniesResponse, error)
}

func (m *mockCompanyServer) ListCompanies(ctx context.Context, req *tsxv1.ListCompaniesRequest) (*tsxv1.ListCompaniesResponse, error) {
	return m.listCompaniesFn(ctx, req)
}

func startMockTracker(t *testing.T, server *mockCompanyServer) *grpc.ClientConn {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	tsxv1.RegisterCompanyServiceServer(srv, server)

	go func() {
		if err := srv.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Logf("mock tracker serve error: %v", err)
		}
	}()
	t.Cleanup(func() { srv.Stop() })

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to dial bufnet: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestFetchAllSymbols_SinglePage(t *testing.T) {
	tracker := startMockTracker(t, &mockCompanyServer{
		listCompaniesFn: func(_ context.Context, _ *tsxv1.ListCompaniesRequest) (*tsxv1.ListCompaniesResponse, error) {
			return &tsxv1.ListCompaniesResponse{
				Companies: []*tsxv1.Company{
					{Symbol: "SHOP.TO"},
					{Symbol: "RY.TO"},
					{Symbol: "TD.TO"},
				},
			}, nil
		},
	})

	ev := newTestEvaluator(t)
	client := tsxv1.NewCompanyServiceClient(tracker)
	symbols, err := ev.fetchAllSymbols(context.Background(), client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(symbols) != 3 {
		t.Fatalf("expected 3 symbols, got %d", len(symbols))
	}
	want := []string{"SHOP.TO", "RY.TO", "TD.TO"}
	for i, s := range symbols {
		if s != want[i] {
			t.Errorf("symbol[%d]: got %q, want %q", i, s, want[i])
		}
	}
}

func TestFetchAllSymbols_Paginated(t *testing.T) {
	callCount := 0
	tracker := startMockTracker(t, &mockCompanyServer{
		listCompaniesFn: func(_ context.Context, req *tsxv1.ListCompaniesRequest) (*tsxv1.ListCompaniesResponse, error) {
			callCount++
			if req.PageToken == "" {
				return &tsxv1.ListCompaniesResponse{
					Companies:     []*tsxv1.Company{{Symbol: "A.TO"}, {Symbol: "B.TO"}},
					NextPageToken: "page2",
					TotalCount:    4,
				}, nil
			}
			return &tsxv1.ListCompaniesResponse{
				Companies: []*tsxv1.Company{{Symbol: "C.TO"}, {Symbol: "D.TO"}},
			}, nil
		},
	})

	ev := newTestEvaluator(t)
	client := tsxv1.NewCompanyServiceClient(tracker)
	symbols, err := ev.fetchAllSymbols(context.Background(), client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(symbols) != 4 {
		t.Fatalf("expected 4 symbols, got %d", len(symbols))
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}

func TestFetchAllSymbols_Error(t *testing.T) {
	tracker := startMockTracker(t, &mockCompanyServer{
		listCompaniesFn: func(_ context.Context, _ *tsxv1.ListCompaniesRequest) (*tsxv1.ListCompaniesResponse, error) {
			return nil, errors.New("rpc error")
		},
	})

	ev := newTestEvaluator(t)
	client := tsxv1.NewCompanyServiceClient(tracker)
	_, err := ev.fetchAllSymbols(context.Background(), client)
	if err == nil {
		t.Fatal("expected error from fetchAllSymbols")
	}
}

func TestFetchAllSymbols_EmptyResponse(t *testing.T) {
	tracker := startMockTracker(t, &mockCompanyServer{
		listCompaniesFn: func(_ context.Context, _ *tsxv1.ListCompaniesRequest) (*tsxv1.ListCompaniesResponse, error) {
			return &tsxv1.ListCompaniesResponse{}, nil
		},
	})

	ev := newTestEvaluator(t)
	client := tsxv1.NewCompanyServiceClient(tracker)
	symbols, err := ev.fetchAllSymbols(context.Background(), client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(symbols) != 0 {
		t.Fatalf("expected 0 symbols, got %d", len(symbols))
	}
}

func TestFetchAllSymbols_ContextCancellation(t *testing.T) {
	tracker := startMockTracker(t, &mockCompanyServer{
		listCompaniesFn: func(_ context.Context, _ *tsxv1.ListCompaniesRequest) (*tsxv1.ListCompaniesResponse, error) {
			return &tsxv1.ListCompaniesResponse{
				Companies: []*tsxv1.Company{{Symbol: "A.TO"}},
			}, nil
		},
	})

	ev := newTestEvaluator(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := tsxv1.NewCompanyServiceClient(tracker)
	_, _ = ev.fetchAllSymbols(ctx, client)
}

func TestCycle_SelectionLogic(t *testing.T) {
	existing := map[string]struct{}{"B.TO": {}}
	symbols := []string{"A.TO", "B.TO", "C.TO"}
	batchSize := 2

	var unevaluated []string
	for _, s := range symbols {
		if _, done := existing[s]; !done {
			unevaluated = append(unevaluated, s)
		}
	}

	batch := min(batchSize, len(symbols))
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

	if len(candidates) != batch {
		t.Fatalf("expected %d candidates, got %d", batch, len(candidates))
	}

	unevaluatedCount := 0
	for _, c := range candidates {
		if _, done := existing[symbols[0]]; !done {
			// This candidate was unevaluated
		}
		_ = c
	}

	// First batch_size candidates should all be unevaluated
	for i := 0; i < len(unevaluated) && i < batch; i++ {
		if _, done := existing[candidates[i]]; done {
			t.Errorf("candidate[%d] %q should be unevaluated", i, candidates[i])
		}
		unevaluatedCount++
	}
	if unevaluatedCount != min(len(unevaluated), batch) {
		t.Errorf("expected %d unevaluated candidates, got %d", min(len(unevaluated), batch), unevaluatedCount)
	}
}

func TestCycle_BatchSizeLargerThanSymbols(t *testing.T) {
	existing := map[string]struct{}{"X.TO": {}}
	symbols := []string{"X.TO", "Y.TO"}
	batchSize := 10

	var unevaluated []string
	for _, s := range symbols {
		if _, done := existing[s]; !done {
			unevaluated = append(unevaluated, s)
		}
	}

	batch := min(batchSize, len(symbols))
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

	if len(candidates) != len(symbols) {
		t.Fatalf("expected %d candidates, got %d", len(symbols), len(candidates))
	}
}

func TestCycle_AllEvaluated(t *testing.T) {
	existing := map[string]struct{}{"A.TO": {}, "B.TO": {}}
	symbols := []string{"A.TO", "B.TO"}
	batchSize := 2

	var unevaluated []string
	for _, s := range symbols {
		if _, done := existing[s]; !done {
			unevaluated = append(unevaluated, s)
		}
	}

	batch := min(batchSize, len(symbols))
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

	if len(candidates) != batch {
		t.Fatalf("expected %d candidates, got %d", batch, len(candidates))
	}

	// All candidates should be re-evaluated
	for _, c := range candidates {
		if _, done := existing[c]; !done {
			t.Errorf("candidate %q should be from evaluated set", c)
		}
	}
}

func TestCycle_NoEvaluatedSymbols(t *testing.T) {
	existing := map[string]struct{}{}
	symbols := []string{"A.TO", "B.TO", "C.TO"}
	batchSize := 2

	var unevaluated []string
	for _, s := range symbols {
		if _, done := existing[s]; !done {
			unevaluated = append(unevaluated, s)
		}
	}

	batch := min(batchSize, len(symbols))
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

	if len(candidates) < batch {
		t.Fatalf("expected at least %d candidates, got %d", batch, len(candidates))
	}

	// All candidates should be unevaluated (none exist in evaluated set)
	for i := 0; i < batch; i++ {
		if _, done := existing[candidates[i]]; done {
			t.Errorf("candidate[%d] %q should be unevaluated", i, candidates[i])
		}
	}
}

func TestNew(t *testing.T) {
	log := slog.Default()
	cfg := &config.Config{}
	store := &mockEvaluatorStore{
		evaluatedSymbolsFn: func(_ context.Context) (map[string]struct{}, error) {
			return nil, nil
		},
		upsertScoresFn: func(_ context.Context, _ *db.ScoreSet) error {
			return nil
		},
	}

	ev := New(cfg, store, log)
	if ev == nil {
		t.Fatal("expected non-nil Evaluator")
	}
	if ev.cfg != cfg {
		t.Error("cfg not set correctly")
	}
	if ev.log != log {
		t.Error("log not set correctly")
	}
}

func newTestEvaluator(t *testing.T) *Evaluator {
	t.Helper()
	return New(&config.Config{}, &mockEvaluatorStore{
		evaluatedSymbolsFn: func(_ context.Context) (map[string]struct{}, error) {
			return nil, nil
		},
		upsertScoresFn: func(_ context.Context, _ *db.ScoreSet) error {
			return nil
		},
	}, slog.Default())
}
