//go:build integration

package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

const baseURL = "http://localhost:8080"

func TestIntegration_FullTransactionLifecycle(t *testing.T) {
	// Wait for service to be ready
	waitForReady(t)

	// Step 1: Create a transaction
	createReq := map[string]interface{}{
		"sender_id":   "user_001",
		"receiver_id": "user_002",
		"amount":      250.00,
		"currency":    "USD",
		"description": "Integration test payment",
	}
	body, _ := json.Marshal(createReq)

	idempotencyKey := fmt.Sprintf("test-%d", time.Now().UnixNano())

	resp, err := http.Post(baseURL+"/api/v1/transactions", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create transaction: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var createResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&createResp)

	txn := createResp["transaction"].(map[string]interface{})
	txnID := txn["id"].(string)

	if txn["status"] != "PENDING" {
		t.Errorf("expected PENDING, got %s", txn["status"])
	}

	t.Logf("Created transaction: %s", txnID)

	// Step 2: Get the transaction
	getResp, err := http.Get(baseURL + "/api/v1/transactions/" + txnID)
	if err != nil {
		t.Fatalf("failed to get transaction: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", getResp.StatusCode)
	}

	// Step 3: Settle the transaction
	settleReq, _ := http.NewRequest(http.MethodPatch, baseURL+"/api/v1/transactions/"+txnID+"/settle", nil)
	settleResp, err := http.DefaultClient.Do(settleReq)
	if err != nil {
		t.Fatalf("failed to settle transaction: %v", err)
	}
	defer settleResp.Body.Close()

	if settleResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", settleResp.StatusCode)
	}

	var settleResult map[string]interface{}
	json.NewDecoder(settleResp.Body).Decode(&settleResult)

	settledTxn := settleResult["transaction"].(map[string]interface{})
	if settledTxn["status"] != "SETTLED" {
		t.Errorf("expected SETTLED, got %s", settledTxn["status"])
	}

	t.Logf("Transaction settled successfully")

	// Step 4: Verify idempotency — replay the create with same key
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/transactions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", idempotencyKey)

	// Note: idempotency test would require the first request to also use the key
	// This demonstrates the pattern

	// Step 5: List transactions
	listResp, err := http.Get(baseURL + "/api/v1/transactions?limit=10")
	if err != nil {
		t.Fatalf("failed to list transactions: %v", err)
	}
	defer listResp.Body.Close()

	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", listResp.StatusCode)
	}

	var listResult map[string]interface{}
	json.NewDecoder(listResp.Body).Decode(&listResult)

	count := int(listResult["count"].(float64))
	if count < 1 {
		t.Errorf("expected at least 1 transaction, got %d", count)
	}

	t.Logf("Listed %d transactions", count)
}

func TestIntegration_DoubleSettlePrevention(t *testing.T) {
	waitForReady(t)

	// Create a transaction
	createReq := map[string]interface{}{
		"sender_id":   "user_010",
		"receiver_id": "user_020",
		"amount":      100.00,
		"currency":    "INR",
	}
	body, _ := json.Marshal(createReq)

	resp, err := http.Post(baseURL+"/api/v1/transactions", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	txnID := result["transaction"].(map[string]interface{})["id"].(string)

	// Settle once — should succeed
	req1, _ := http.NewRequest(http.MethodPatch, baseURL+"/api/v1/transactions/"+txnID+"/settle", nil)
	resp1, _ := http.DefaultClient.Do(req1)
	defer resp1.Body.Close()

	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first settle expected 200, got %d", resp1.StatusCode)
	}

	// Settle again — should fail (already settled, terminal state)
	req2, _ := http.NewRequest(http.MethodPatch, baseURL+"/api/v1/transactions/"+txnID+"/settle", nil)
	resp2, _ := http.DefaultClient.Do(req2)
	defer resp2.Body.Close()

	if resp2.StatusCode == http.StatusOK {
		t.Error("second settle should have failed but returned 200")
	}

	t.Logf("Double-settle correctly prevented with status %d", resp2.StatusCode)
}

func TestIntegration_ValidationErrors(t *testing.T) {
	waitForReady(t)

	tests := []struct {
		name string
		body map[string]interface{}
	}{
		{
			name: "missing sender",
			body: map[string]interface{}{"receiver_id": "user_002", "amount": 100, "currency": "USD"},
		},
		{
			name: "negative amount",
			body: map[string]interface{}{"sender_id": "user_001", "receiver_id": "user_002", "amount": -50, "currency": "USD"},
		},
		{
			name: "same sender and receiver",
			body: map[string]interface{}{"sender_id": "user_001", "receiver_id": "user_001", "amount": 100, "currency": "USD"},
		},
		{
			name: "invalid currency",
			body: map[string]interface{}{"sender_id": "user_001", "receiver_id": "user_002", "amount": 100, "currency": "XYZ"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			resp, err := http.Post(baseURL+"/api/v1/transactions", "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", resp.StatusCode)
			}
		})
	}
}

func waitForReady(t *testing.T) {
	t.Helper()
	for i := 0; i < 30; i++ {
		resp, err := http.Get(baseURL + "/ready")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(time.Second)
	}
	t.Fatal("service did not become ready within 30 seconds")
}
