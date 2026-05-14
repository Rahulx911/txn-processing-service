package model

import (
	"errors"
	"time"
)

// TransactionStatus represents the state machine for a transaction.
type TransactionStatus string

const (
	StatusPending    TransactionStatus = "PENDING"
	StatusProcessing TransactionStatus = "PROCESSING"
	StatusSettled    TransactionStatus = "SETTLED"
	StatusFailed     TransactionStatus = "FAILED"
	StatusCancelled  TransactionStatus = "CANCELLED"
)

// Transaction represents a financial transaction in the system.
type Transaction struct {
	ID          string            `json:"id" dynamodbav:"PK"`
	SenderID    string            `json:"sender_id" dynamodbav:"sender_id"`
	ReceiverID  string            `json:"receiver_id" dynamodbav:"receiver_id"`
	Amount      float64           `json:"amount" dynamodbav:"amount"`
	Currency    string            `json:"currency" dynamodbav:"currency"`
	Status      TransactionStatus `json:"status" dynamodbav:"status"`
	Description string            `json:"description,omitempty" dynamodbav:"description"`
	Version     int               `json:"version" dynamodbav:"version"` // Optimistic locking
	CreatedAt   time.Time         `json:"created_at" dynamodbav:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at" dynamodbav:"updated_at"`
	SettledAt   *time.Time        `json:"settled_at,omitempty" dynamodbav:"settled_at,omitempty"`
}

// CreateTransactionRequest is the DTO for creating a new transaction.
type CreateTransactionRequest struct {
	SenderID    string  `json:"sender_id"`
	ReceiverID  string  `json:"receiver_id"`
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	Description string  `json:"description,omitempty"`
}

// Validate checks the request payload for correctness.
func (r *CreateTransactionRequest) Validate() error {
	if r.SenderID == "" {
		return errors.New("sender_id is required")
	}
	if r.ReceiverID == "" {
		return errors.New("receiver_id is required")
	}
	if r.SenderID == r.ReceiverID {
		return errors.New("sender and receiver cannot be the same")
	}
	if r.Amount <= 0 {
		return errors.New("amount must be greater than zero")
	}
	if r.Currency == "" {
		return errors.New("currency is required")
	}
	if !isValidCurrency(r.Currency) {
		return errors.New("unsupported currency code")
	}
	return nil
}

// ListTransactionsRequest holds pagination parameters.
type ListTransactionsRequest struct {
	Limit         int    `json:"limit"`
	LastEvaluated string `json:"last_evaluated,omitempty"`
}

// TransactionResponse wraps a transaction for API responses.
type TransactionResponse struct {
	Transaction *Transaction `json:"transaction"`
}

// ListTransactionsResponse wraps paginated transaction results.
type ListTransactionsResponse struct {
	Transactions  []*Transaction `json:"transactions"`
	Count         int            `json:"count"`
	LastEvaluated string         `json:"last_evaluated,omitempty"`
}

// ValidTransitions defines allowed state transitions for the transaction state machine.
var ValidTransitions = map[TransactionStatus][]TransactionStatus{
	StatusPending:    {StatusProcessing, StatusCancelled, StatusFailed},
	StatusProcessing: {StatusSettled, StatusFailed},
	StatusSettled:    {}, // Terminal state
	StatusFailed:     {}, // Terminal state
	StatusCancelled:  {}, // Terminal state
}

// CanTransitionTo checks if a status transition is valid.
func (s TransactionStatus) CanTransitionTo(target TransactionStatus) bool {
	allowed, exists := ValidTransitions[s]
	if !exists {
		return false
	}
	for _, t := range allowed {
		if t == target {
			return true
		}
	}
	return false
}

func isValidCurrency(code string) bool {
	supported := map[string]bool{
		"USD": true, "EUR": true, "GBP": true,
		"INR": true, "JPY": true, "AUD": true,
		"CAD": true, "SGD": true, "AED": true,
	}
	return supported[code]
}
