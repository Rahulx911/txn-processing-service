package handler_test

import (
"encoding/json"
"net/http"
"net/http/httptest"
"testing"

"github.com/rahuljain/txn-processing-service/internal/handler"
)

func TestHealthCheck(t *testing.T) {
req := httptest.NewRequest(http.MethodGet, "/health", nil)
w := httptest.NewRecorder()

handler.HealthCheck(w, req)

if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d", w.Code)
}

var resp map[string]string
json.NewDecoder(w.Body).Decode(&resp)

if resp["status"] != "healthy" {
t.Errorf("expected status 'healthy', got '%s'", resp["status"])
}
}
