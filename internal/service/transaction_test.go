package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/rahuljain/txn-processing-service/internal/model"
	"github.com/rahuljain/txn-processing-service/internal/repository"
	"github.com/rahuljain/txn-processing-service/internal/service"
	"github.com/rahuljain/txn-processing-service/pkg/logger"
)

// mockRepo implements repository.TransactionRepository for testing.
type mockRepo struct {
	transactions map[string]*model.Transaction
	createErr    error
	updateErr    error
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		transactions: make(map[string]*model.Transaction),
	}
}

func (m *mockRepo) Create(_ context.Context, txn *model.Transaction) error {
	if m.createErr != nil {
		return m.createErr
	}
	if _, exists := m.transactions[txn.ID]; exists {
		return repository.ErrConflict
	}
	m.transactions[txn.ID] = txn
	return nil
}

func (m *mockRepo) GetByID(_ context.Context, id string) (*model.Transaction, error) {
	txn, exists := m.transactions[id]
	if !exists {
		return nil, repository.ErrNotFound
	}
	return txn, nil
}

func (m *mockRepo) List(_ context.Context, limit int, _ string) ([]*model.Transaction, string, error) {
	result := make([]*model.Transaction, 0)
	for _, txn := range m.transactions {
		if len(result) >= limit {
			break
		}
		result = append(result, txn)
	}
	return result, "", nil
}

func (m *mockRepo) UpdateStatus(_ context.Context, id string, status model.TransactionStatus, expectedVersion int) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	txn, exists := m.transactions[id]
	if !exists {
		return repository.ErrNotFound
	}
	if txn.Version != expectedVersion {
		return repository.ErrConflict
	}
	txn.Status = status
	txn.Version = expectedVersion + 1
	return nil
}

func (m *mockRepo) Ping(_ context.Context) error {
	return nil
}

func TestCreateTransaction_Success(t *testing.T) {
	repo := newMockRepo()
	log := logger.New("error")
	svc := service.NewTransactionService(repo, log)

	req := &model.CreateTransactionRequest{
		SenderID:    "user_001",
		ReceiverID:  "user_002",
		Amount:      250.00,
		Currency:    "USD",
		Description: "Test payment",
	}

	txn, err := svc.CreateTransaction(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if txn.Status != model.StatusPending {
		t.Errorf("expected status PENDING, got %s", txn.Status)
	}
	if txn.Amount != 250.00 {
		t.Errorf("expected amount 250.00, got %f", txn.Amount)
	}
	if txn.Version != 1 {
		t.Errorf("expected version 1, got %d", txn.Version)
	}
	if txn.SenderID != "user_001" {
		t.Errorf("expected sender user_001, got %s", txn.SenderID)
	}
}

func TestCreateTransaction_ValidationFailure(t *testing.T) {
	repo := newMockRepo()
	log := logger.New("error")
	svc := service.NewTransactionService(repo, log)

	req := &model.CreateTransactionRequest{
		SenderID:   "user_001",
		ReceiverID: "user_001", // same as sender
		Amount:     100.0,
		Currency:   "USD",
	}

	_, err := svc.CreateTransaction(context.Background(), req)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestGetTransaction_NotFound(t *testing.T) {
	repo := newMockRepo()
	log := logger.New("error")
	svc := service.NewTransactionService(repo, log)

	_, err := svc.GetTransaction(context.Background(), "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSettleTransaction_Success(t *testing.T) {
	repo := newMockRepo()
	log := logger.New("error")
	svc := service.NewTransactionService(repo, log)

	// Create a transaction first
	req := &model.CreateTransactionRequest{
		SenderID:   "user_001",
		ReceiverID: "user_002",
		Amount:     500.00,
		Currency:   "INR",
	}
	txn, err := svc.CreateTransaction(context.Background(), req)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Settle it
	settled, err := svc.SettleTransaction(context.Background(), txn.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if settled.Status != model.StatusSettled {
		t.Errorf("expected SETTLED status, got %s", settled.Status)
	}
}

func TestSettleTransaction_OptimisticLockConflict(t *testing.T) {
	repo := newMockRepo()
	log := logger.New("error")
	svc := service.NewTransactionService(repo, log)

	// Create a transaction
	req := &model.CreateTransactionRequest{
		SenderID:   "user_001",
		ReceiverID: "user_002",
		Amount:     100.0,
		Currency:   "USD",
	}
	txn, _ := svc.CreateTransaction(context.Background(), req)

	// Simulate a concurrent modification by bumping the version
	repo.transactions[txn.ID].Version = 99

	// Settle should fail due to version mismatch
	_, err := svc.SettleTransaction(context.Background(), txn.ID)
	if err == nil {
		t.Fatal("expected optimistic lock conflict, got nil")
	}
}

func TestListTransactions_DefaultLimit(t *testing.T) {
	repo := newMockRepo()
	log := logger.New("error")
	svc := service.NewTransactionService(repo, log)

	// Create several transactions
	for i := 0; i < 5; i++ {
		req := &model.CreateTransactionRequest{
			SenderID:   "user_001",
			ReceiverID: "user_002",
			Amount:     float64(i+1) * 10.0,
			Currency:   "USD",
		}
		svc.CreateTransaction(context.Background(), req)
	}

	txns, _, err := svc.ListTransactions(context.Background(), 0, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(txns) != 5 {
		t.Errorf("expected 5 transactions, got %d", len(txns))
	}
}
