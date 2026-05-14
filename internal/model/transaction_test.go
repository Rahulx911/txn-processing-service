package model_test

import (
	"testing"

	"github.com/rahuljain/txn-processing-service/internal/model"
)

func TestCreateTransactionRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     model.CreateTransactionRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: model.CreateTransactionRequest{
				SenderID:   "user_001",
				ReceiverID: "user_002",
				Amount:     100.0,
				Currency:   "USD",
			},
			wantErr: false,
		},
		{
			name: "missing sender_id",
			req: model.CreateTransactionRequest{
				ReceiverID: "user_002",
				Amount:     100.0,
				Currency:   "USD",
			},
			wantErr: true,
		},
		{
			name: "missing receiver_id",
			req: model.CreateTransactionRequest{
				SenderID: "user_001",
				Amount:   100.0,
				Currency: "USD",
			},
			wantErr: true,
		},
		{
			name: "same sender and receiver",
			req: model.CreateTransactionRequest{
				SenderID:   "user_001",
				ReceiverID: "user_001",
				Amount:     100.0,
				Currency:   "USD",
			},
			wantErr: true,
		},
		{
			name: "zero amount",
			req: model.CreateTransactionRequest{
				SenderID:   "user_001",
				ReceiverID: "user_002",
				Amount:     0,
				Currency:   "USD",
			},
			wantErr: true,
		},
		{
			name: "negative amount",
			req: model.CreateTransactionRequest{
				SenderID:   "user_001",
				ReceiverID: "user_002",
				Amount:     -50.0,
				Currency:   "USD",
			},
			wantErr: true,
		},
		{
			name: "unsupported currency",
			req: model.CreateTransactionRequest{
				SenderID:   "user_001",
				ReceiverID: "user_002",
				Amount:     100.0,
				Currency:   "XYZ",
			},
			wantErr: true,
		},
		{
			name: "missing currency",
			req: model.CreateTransactionRequest{
				SenderID:   "user_001",
				ReceiverID: "user_002",
				Amount:     100.0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTransactionStatus_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name   string
		from   model.TransactionStatus
		to     model.TransactionStatus
		expect bool
	}{
		{"pending to processing", model.StatusPending, model.StatusProcessing, true},
		{"pending to cancelled", model.StatusPending, model.StatusCancelled, true},
		{"pending to failed", model.StatusPending, model.StatusFailed, true},
		{"pending to settled", model.StatusPending, model.StatusSettled, false},
		{"processing to settled", model.StatusProcessing, model.StatusSettled, true},
		{"processing to failed", model.StatusProcessing, model.StatusFailed, true},
		{"processing to pending", model.StatusProcessing, model.StatusPending, false},
		{"settled to any", model.StatusSettled, model.StatusFailed, false},
		{"failed to any", model.StatusFailed, model.StatusPending, false},
		{"cancelled to any", model.StatusCancelled, model.StatusProcessing, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.from.CanTransitionTo(tt.to)
			if result != tt.expect {
				t.Errorf("CanTransitionTo(%s -> %s) = %v, want %v", tt.from, tt.to, result, tt.expect)
			}
		})
	}
}
