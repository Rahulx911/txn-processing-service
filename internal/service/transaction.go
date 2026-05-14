package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rahuljain/txn-processing-service/internal/model"
	"github.com/rahuljain/txn-processing-service/internal/repository"
	"github.com/rahuljain/txn-processing-service/pkg/logger"
)

// TransactionService encapsulates all transaction business logic.
type TransactionService struct {
	repo repository.TransactionRepository
	log  *logger.Logger
}

// NewTransactionService creates a new service with injected dependencies.
func NewTransactionService(repo repository.TransactionRepository, log *logger.Logger) *TransactionService {
	return &TransactionService{
		repo: repo,
		log:  log,
	}
}

// CreateTransaction validates and persists a new transaction.
// Returns the created transaction in PENDING status.
func (s *TransactionService) CreateTransaction(ctx context.Context, req *model.CreateTransactionRequest) (*model.Transaction, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	now := time.Now().UTC()
	txn := &model.Transaction{
		ID:          fmt.Sprintf("txn_%s", uuid.New().String()),
		SenderID:    req.SenderID,
		ReceiverID:  req.ReceiverID,
		Amount:      req.Amount,
		Currency:    req.Currency,
		Status:      model.StatusPending,
		Description: req.Description,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.repo.Create(ctx, txn); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			s.log.Warn("duplicate transaction creation attempt", "txn_id", txn.ID)
			return nil, err
		}
		s.log.Error("failed to create transaction", "error", err)
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	s.log.Info("transaction created",
		"txn_id", txn.ID,
		"sender", txn.SenderID,
		"receiver", txn.ReceiverID,
		"amount", txn.Amount,
		"currency", txn.Currency,
	)

	return txn, nil
}

// GetTransaction retrieves a transaction by ID.
func (s *TransactionService) GetTransaction(ctx context.Context, id string) (*model.Transaction, error) {
	txn, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, err
		}
		s.log.Error("failed to get transaction", "txn_id", id, "error", err)
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}
	return txn, nil
}

// ListTransactions returns a paginated list of transactions.
func (s *TransactionService) ListTransactions(ctx context.Context, limit int, lastKey string) ([]*model.Transaction, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	transactions, nextKey, err := s.repo.List(ctx, limit, lastKey)
	if err != nil {
		s.log.Error("failed to list transactions", "error", err)
		return nil, "", fmt.Errorf("failed to list transactions: %w", err)
	}

	return transactions, nextKey, nil
}

// SettleTransaction transitions a transaction through PROCESSING → SETTLED.
// Uses optimistic locking to prevent race conditions when multiple consumers
// attempt to settle the same transaction concurrently.
func (s *TransactionService) SettleTransaction(ctx context.Context, id string) (*model.Transaction, error) {
	// Fetch current state
	txn, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Validate state transition: PENDING → PROCESSING
	if !txn.Status.CanTransitionTo(model.StatusProcessing) {
		s.log.Warn("invalid state transition attempted",
			"txn_id", id,
			"current_status", txn.Status,
			"target_status", model.StatusProcessing,
		)
		return nil, fmt.Errorf("cannot process transaction in %s status: %w",
			txn.Status, repository.ErrInvalidTransition)
	}

	// Phase 1: Move to PROCESSING (optimistic lock check)
	if err := s.repo.UpdateStatus(ctx, id, model.StatusProcessing, txn.Version); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			s.log.Warn("optimistic lock conflict during processing", "txn_id", id)
			return nil, fmt.Errorf("concurrent modification detected: %w", err)
		}
		return nil, err
	}

	s.log.Info("transaction moved to PROCESSING", "txn_id", id)

	// Phase 2: Execute settlement logic
	// In a production system, this would involve calling external payment
	// processors, ledger updates, compliance checks, etc.
	if err := s.executeSettlement(ctx, txn); err != nil {
		// Rollback: mark as FAILED
		_ = s.repo.UpdateStatus(ctx, id, model.StatusFailed, txn.Version+1)
		s.log.Error("settlement failed, marked as FAILED", "txn_id", id, "error", err)
		return nil, fmt.Errorf("settlement failed: %w", err)
	}

	// Phase 3: Move to SETTLED
	if err := s.repo.UpdateStatus(ctx, id, model.StatusSettled, txn.Version+1); err != nil {
		s.log.Error("failed to mark as settled", "txn_id", id, "error", err)
		return nil, err
	}

	s.log.Info("transaction settled successfully", "txn_id", id, "amount", txn.Amount)

	// Fetch and return updated transaction
	return s.repo.GetByID(ctx, id)
}

// executeSettlement simulates the actual settlement process.
// In production, this would integrate with payment rails, ledger systems, etc.
func (s *TransactionService) executeSettlement(ctx context.Context, txn *model.Transaction) error {
	// Simulate processing delay
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(100 * time.Millisecond):
		// Settlement logic would go here:
		// 1. Validate sender has sufficient balance
		// 2. Debit sender account
		// 3. Credit receiver account
		// 4. Record in ledger
		// 5. Trigger compliance checks
		return nil
	}
}
