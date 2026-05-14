package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rahuljain/txn-processing-service/internal/config"
	"github.com/rahuljain/txn-processing-service/internal/handler"
	"github.com/rahuljain/txn-processing-service/internal/middleware"
	"github.com/rahuljain/txn-processing-service/internal/repository"
	"github.com/rahuljain/txn-processing-service/internal/service"
	"github.com/rahuljain/txn-processing-service/pkg/idempotency"
	"github.com/rahuljain/txn-processing-service/pkg/logger"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialise structured logger
	log := logger.New(cfg.LogLevel)
	log.Info("starting transaction processing service", "port", cfg.Port, "env", cfg.Environment)

	// Initialise repository layer
	txnRepo, err := repository.NewDynamoDBRepository(cfg)
	if err != nil {
		log.Error("failed to initialise DynamoDB repository", "error", err)
		os.Exit(1)
	}

	// Initialise idempotency store
	idempotencyStore, err := idempotency.NewDynamoDBStore(cfg)
	if err != nil {
		log.Error("failed to initialise idempotency store", "error", err)
		os.Exit(1)
	}

	// Initialise service layer
	txnService := service.NewTransactionService(txnRepo, log)

	// Initialise HTTP handlers
	txnHandler := handler.NewTransactionHandler(txnService, log)

	// Build router
	mux := http.NewServeMux()

	// Health & readiness endpoints
	mux.HandleFunc("GET /health", handler.HealthCheck)
	mux.HandleFunc("GET /ready", handler.ReadinessCheck(txnRepo))

	// Transaction endpoints
	mux.HandleFunc("POST /api/v1/transactions", txnHandler.CreateTransaction)
	mux.HandleFunc("GET /api/v1/transactions/{id}", txnHandler.GetTransaction)
	mux.HandleFunc("GET /api/v1/transactions", txnHandler.ListTransactions)
	mux.HandleFunc("PATCH /api/v1/transactions/{id}/settle", txnHandler.SettleTransaction)

	// Apply middleware chain
	chain := middleware.Chain(
		middleware.Recovery(log),
		middleware.RequestID,
		middleware.Logger(log),
		middleware.CORS,
		middleware.Idempotency(idempotencyStore, log),
	)

	// Configure server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      chain(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Info("server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	log.Info("shutdown signal received", "signal", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("forced shutdown", "error", err)
		os.Exit(1)
	}

	log.Info("server stopped gracefully")
}
