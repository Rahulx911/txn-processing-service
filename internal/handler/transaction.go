package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/rahuljain/txn-processing-service/internal/model"
	"github.com/rahuljain/txn-processing-service/internal/repository"
	"github.com/rahuljain/txn-processing-service/internal/service"
	"github.com/rahuljain/txn-processing-service/pkg/logger"
)

// TransactionHandler handles HTTP requests for transaction operations.
type TransactionHandler struct {
	service *service.TransactionService
	log     *logger.Logger
}

// NewTransactionHandler creates a new handler with injected service.
func NewTransactionHandler(svc *service.TransactionService, log *logger.Logger) *TransactionHandler {
	return &TransactionHandler{
		service: svc,
		log:     log,
	}
}

// CreateTransaction handles POST /api/v1/transactions
func (h *TransactionHandler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	var req model.CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	defer r.Body.Close()

	txn, err := h.service.CreateTransaction(r.Context(), &req)
	if err != nil {
		if errors.Is(err, repository.ErrConflict) {
			writeError(w, http.StatusConflict, "transaction already exists")
			return
		}
		// Check for validation errors
		if isValidationError(err) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, model.TransactionResponse{Transaction: txn})
}

// GetTransaction handles GET /api/v1/transactions/{id}
func (h *TransactionHandler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "transaction ID is required")
		return
	}

	txn, err := h.service.GetTransaction(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "transaction not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, model.TransactionResponse{Transaction: txn})
}

// ListTransactions handles GET /api/v1/transactions
func (h *TransactionHandler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	lastKey := r.URL.Query().Get("last_key")

	transactions, nextKey, err := h.service.ListTransactions(r.Context(), limit, lastKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, model.ListTransactionsResponse{
		Transactions:  transactions,
		Count:         len(transactions),
		LastEvaluated: nextKey,
	})
}

// SettleTransaction handles PATCH /api/v1/transactions/{id}/settle
func (h *TransactionHandler) SettleTransaction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "transaction ID is required")
		return
	}

	txn, err := h.service.SettleTransaction(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "transaction not found")
			return
		}
		if errors.Is(err, repository.ErrConflict) {
			writeError(w, http.StatusConflict, "transaction was modified concurrently, retry")
			return
		}
		if errors.Is(err, repository.ErrInvalidTransition) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, model.TransactionResponse{Transaction: txn})
}

// HealthCheck handles GET /health
func HealthCheck(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "healthy",
		"service": "txn-processing-service",
	})
}

// ReadinessCheck handles GET /ready and verifies downstream dependencies.
func ReadinessCheck(repo repository.TransactionRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := repo.Ping(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "not ready",
				"reason": "database unavailable",
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}

// --- Helpers ---

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{
		Error:   http.StatusText(status),
		Message: message,
	})
}

func isValidationError(err error) bool {
	// Validation errors from the model layer are wrapped with "validation failed:"
	return err != nil && len(err.Error()) > 18 && err.Error()[:18] == "validation failed:"
}
