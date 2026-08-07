package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/example/stocker-evaluator/internal/config"
	"github.com/example/stocker-evaluator/internal/db"
	"github.com/example/stocker-evaluator/internal/evaluator"
	"github.com/example/stocker-evaluator/internal/finance"
	"github.com/example/stocker-evaluator/internal/grpcserver"
	"github.com/example/stocker-evaluator/internal/leadership"
	"github.com/example/stocker-evaluator/internal/sentiment"
	"github.com/example/stocker-evaluator/internal/typesentiment"

	stockerv1 "github.com/example/stocker-evaluator/gen/tsx/v1"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(log); err != nil {
		log.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	repo, err := db.NewRepository(ctx, cfg.PostgresDSN())
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer repo.Close()

	if err := repo.Migrate(ctx, db.InitSchemaSQL()); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	log.Info("database ready")

	// Financial data client (Yahoo Finance, no API key needed)
	finCli := finance.NewClient()

	// Sentiment analysis
	llmClient := sentiment.NewLLMClient(cfg.LLMBaseURL, cfg.LLMModel, cfg.LLMTimeout)
	sentEv := sentiment.NewEvaluator(llmClient, log)

	// Leadership analysis
	leadEv := leadership.NewEvaluator(leadership.NewYahooClient(), log)

	// Type sentiment analysis (sector/industry outlook via Yahoo Finance)
	typeSentProfileCli := typesentiment.NewProfileClient()
	typeSentEv := typesentiment.NewEvaluator(typeSentProfileCli, llmClient, log)

	// Background evaluator loop
	eval := evaluator.New(cfg, repo, finCli, sentEv, leadEv, typeSentEv, log)
	go eval.Run(ctx)

	// gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if err != nil {
		return fmt.Errorf("listen on port %d: %w", cfg.GRPCPort, err)
	}

	grpcSrv := grpc.NewServer()
	stockerv1.RegisterEvaluatorServiceServer(grpcSrv, grpcserver.New(repo, finCli, sentEv, leadEv, typeSentEv, log))
	reflection.Register(grpcSrv)

	go func() {
		<-ctx.Done()
		log.Info("shutting down gRPC server")
		grpcSrv.GracefulStop()
	}()

	log.Info("gRPC server listening", "port", cfg.GRPCPort)
	if err := grpcSrv.Serve(lis); err != nil {
		return fmt.Errorf("grpc serve: %w", err)
	}
	return nil
}
