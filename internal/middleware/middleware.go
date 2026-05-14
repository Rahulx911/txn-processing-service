package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rahuljain/txn-processing-service/pkg/idempotency"
	"github.com/rahuljain/txn-processing-service/pkg/logger"
)

type contextKey string

const RequestIDKey contextKey = "request_id"

// Middleware is a function that wraps an http.Handler.
type Middleware func(http.Handler) http.Handler

// Chain applies middleware in order: first listed = outermost wrapper.
func Chain(middlewares ...Middleware) Middleware {
	return func(final http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			final = middlewares[i](final)
		}
		return final
	}
}

// RequestID injects a unique correlation ID into each request context.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = uuid.New().String()
		}

		ctx := context.WithValue(r.Context(), RequestIDKey, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Logger logs incoming requests with duration and status code.
func Logger(log *logger.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			reqID, _ := r.Context().Value(RequestIDKey).(string)
			log.Info("request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", reqID,
			)
		})
	}
}

// Recovery catches panics and returns a 500 response.
func Recovery(log *logger.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					reqID, _ := r.Context().Value(RequestIDKey).(string)
					log.Error("panic recovered",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"request_id", reqID,
					)
					http.Error(w, `{"error":"Internal Server Error","message":"unexpected error"}`,
						http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// CORS adds Cross-Origin Resource Sharing headers.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Idempotency-Key, X-Request-ID")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Idempotency prevents duplicate processing of POST/PATCH requests
// by checking the Idempotency-Key header against a DynamoDB-backed store.
func Idempotency(store idempotency.Store, log *logger.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only enforce idempotency on mutating requests
			if r.Method != http.MethodPost && r.Method != http.MethodPatch {
				next.ServeHTTP(w, r)
				return
			}

			key := r.Header.Get("Idempotency-Key")
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Check if we've already processed this key
			if cached, err := store.Get(r.Context(), key); err == nil && cached != nil {
				log.Info("idempotent request detected, returning cached response", "idempotency_key", key)
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Idempotent-Replayed", "true")
				w.WriteHeader(cached.StatusCode)
				w.Write(cached.Body)
				return
			}

			// Capture the response
			rec := &responseRecorder{ResponseWriter: w, body: make([]byte, 0)}
			next.ServeHTTP(rec, r)

			// Cache the response for future duplicate requests
			if err := store.Set(r.Context(), key, &idempotency.CachedResponse{
				StatusCode: rec.status,
				Body:       rec.body,
			}); err != nil {
				log.Warn("failed to cache idempotent response", "key", key, "error", err)
			}
		})
	}
}

// statusRecorder wraps ResponseWriter to capture the status code.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// responseRecorder captures both status and body for idempotency caching.
type responseRecorder struct {
	http.ResponseWriter
	status int
	body   []byte
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body = append(r.body, b...)
	return r.ResponseWriter.Write(b)
}
