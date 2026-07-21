package grpcserver

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/example/tsx-evaluator/internal/analyzer"
	"github.com/example/tsx-evaluator/internal/db"

	tsxv1 "github.com/example/tsx-evaluator/gen/tsx/v1"
)

const defaultPageSize = 50
const maxPageSize = 500

type Store interface {
	GetBySymbol(ctx context.Context, symbol string) (*db.ScoreSet, error)
	ListOrdered(ctx context.Context, orderExpr string, descending bool, afterSymbol string, pageSize int) ([]db.ScoreSet, error)
	CountAll(ctx context.Context) (int, error)
	UpsertScores(ctx context.Context, s *db.ScoreSet) error
}

type Server struct {
	tsxv1.UnimplementedEvaluatorServiceServer
	repo Store
	log  *slog.Logger
}

func New(repo Store, log *slog.Logger) *Server {
	return &Server{repo: repo, log: log}
}

func (s *Server) GetScores(ctx context.Context, req *tsxv1.GetScoresRequest) (*tsxv1.GetScoresResponse, error) {
	if req.GetSymbol() == "" {
		return nil, status.Error(codes.InvalidArgument, "symbol is required")
	}

	score, err := s.repo.GetBySymbol(ctx, req.GetSymbol())
	if err != nil {
		if err == db.ErrNotFound {
			return nil, status.Errorf(codes.NotFound, "no evaluation found for symbol %q", req.GetSymbol())
		}
		s.log.Error("GetScores failed", "symbol", req.GetSymbol(), "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	return &tsxv1.GetScoresResponse{Scores: toProto(score)}, nil
}

func (s *Server) ListScoredStocks(ctx context.Context, req *tsxv1.ListScoredStocksRequest) (*tsxv1.ListScoredStocksResponse, error) {
	pageSize := int(req.GetPageSize())
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	descending := req.GetDescending()

	var orderExpr string
	switch sort := req.GetSortBy().(type) {
	case *tsxv1.ListScoredStocksRequest_Metric:
		col, ok := db.ScoreMetricToColumn(sort.Metric.String())
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "invalid metric: %v", sort.Metric)
		}
		orderExpr = col
	case *tsxv1.ListScoredStocksRequest_Weights:
		w := sort.Weights
		orderExpr = db.BuildCompositeExpr(
			w.GetFinancials(), w.GetSentiment(),
			w.GetLeadership(), w.GetTypeSentiment(),
		)
	default:
		orderExpr = "financials"
		descending = true
	}

	scores, err := s.repo.ListOrdered(ctx, orderExpr, descending, req.GetPageToken(), pageSize)
	if err != nil {
		s.log.Error("ListScoredStocks failed", "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	total, err := s.repo.CountAll(ctx)
	if err != nil {
		s.log.Error("CountAll failed", "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	resp := &tsxv1.ListScoredStocksResponse{
		Scores:     make([]*tsxv1.ScoreSet, 0, len(scores)),
		TotalCount: int32(total),
	}
	for i := range scores {
		resp.Scores = append(resp.Scores, toProto(&scores[i]))
	}
	if len(scores) == pageSize {
		resp.NextPageToken = scores[len(scores)-1].Symbol
	}
	return resp, nil
}

func (s *Server) EvaluateNow(ctx context.Context, req *tsxv1.EvaluateNowRequest) (*tsxv1.EvaluateNowResponse, error) {
	// Trigger an immediate evaluation on a random symbol.
	// In a production system this would accept a symbol parameter;
	// here we just re-evaluate any existing symbol or generate a dummy one.
	scores := analyzer.Analyze("DUMMY")
	if err := s.repo.UpsertScores(ctx, scores); err != nil {
		s.log.Error("EvaluateNow upsert failed", "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}
	return &tsxv1.EvaluateNowResponse{Scores: toProto(scores)}, nil
}

func toProto(s *db.ScoreSet) *tsxv1.ScoreSet {
	return &tsxv1.ScoreSet{
		Symbol:       s.Symbol,
		Financials:   s.Financials,
		Sentiment:    s.Sentiment,
		Leadership:   s.Leadership,
		TypeSentiment: s.TypeSentiment,
		EvaluatedAt:  timestamppb.New(s.EvaluatedAt),
	}
}
