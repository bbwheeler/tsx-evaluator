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

	"github.com/example/tsx-evaluator/internal/config"
	"github.com/example/tsx-evaluator/internal/db"
	"github.com/example/tsx-evaluator/internal/evaluator"
	"github.com/example/tsx-evaluator/internal/finance"
	"github.com/example/tsx-evaluator/internal/grpcserver"

	tsxv1 "github.com/example/tsx-evaluator/gen/tsx/v1"
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

	// Financial data client
	finCli := finance.NewClient(cfg.FMPBaseURL, cfg.FMPAPIKey)

	// Background evaluator loop
	eval := evaluator.New(cfg, repo, finCli, log)
	go eval.Run(ctx)

	// gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if err != nil {
		return fmt.Errorf("listen on port %d: %w", cfg.GRPCPort, err)
	}

	grpcSrv := grpc.NewServer()
	tsxv1.RegisterEvaluatorServiceServer(grpcSrv, grpcserver.New(repo, finCli, log))
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
