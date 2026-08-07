package grpcserver

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/example/stocker-evaluator/internal/analyzer"
	"github.com/example/stocker-evaluator/internal/db"
	"github.com/example/stocker-evaluator/internal/finance"
	"github.com/example/stocker-evaluator/internal/leadership"
	"github.com/example/stocker-evaluator/internal/sentiment"
	"github.com/example/stocker-evaluator/internal/typesentiment"

	stockerv1 "github.com/example/stocker-evaluator/gen/tsx/v1"
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
	stockerv1.UnimplementedEvaluatorServiceServer
	repo      Store
	finCli    *finance.Client
	sentEv    *sentiment.Evaluator
	leadEv    *leadership.Evaluator
	typeSentEv *typesentiment.Evaluator
	log       *slog.Logger
}

func New(repo Store, finCli *finance.Client, sentEv *sentiment.Evaluator, leadEv *leadership.Evaluator, typeSentEv *typesentiment.Evaluator, log *slog.Logger) *Server {
	return &Server{repo: repo, finCli: finCli, sentEv: sentEv, leadEv: leadEv, typeSentEv: typeSentEv, log: log}
}

func (s *Server) GetScores(ctx context.Context, req *stockerv1.GetScoresRequest) (*stockerv1.GetScoresResponse, error) {
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

	return &stockerv1.GetScoresResponse{Scores: toProto(score)}, nil
}

func (s *Server) ListScoredStocks(ctx context.Context, req *stockerv1.ListScoredStocksRequest) (*stockerv1.ListScoredStocksResponse, error) {
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
	case *stockerv1.ListScoredStocksRequest_Metric:
		col, ok := db.ScoreMetricToColumn(sort.Metric.String())
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "invalid metric: %v", sort.Metric)
		}
		orderExpr = col
	case *stockerv1.ListScoredStocksRequest_Weights:
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

	resp := &stockerv1.ListScoredStocksResponse{
		Scores:     make([]*stockerv1.ScoreSet, 0, len(scores)),
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

func (s *Server) EvaluateNow(ctx context.Context, req *stockerv1.EvaluateNowRequest) (*stockerv1.EvaluateNowResponse, error) {
	scores := analyzer.Analyze(ctx, s.finCli, s.sentEv, s.leadEv, s.typeSentEv, "DUMMY", s.log)
	if err := s.repo.UpsertScores(ctx, scores); err != nil {
		s.log.Error("EvaluateNow upsert failed", "error", err)
		return nil, status.Error(codes.Internal, "internal error")
	}
	return &stockerv1.EvaluateNowResponse{Scores: toProto(scores)}, nil
}

func toProto(s *db.ScoreSet) *stockerv1.ScoreSet {
	return &stockerv1.ScoreSet{
		Symbol:        s.Symbol,
		Financials:   s.Financials,
		Sentiment:    s.Sentiment,
		Leadership:   s.Leadership,
		TypeSentiment: s.TypeSentiment,
		EvaluatedAt:  timestamppb.New(s.EvaluatedAt),
	}
}
