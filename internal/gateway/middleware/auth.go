package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/exchange/internal/user"
)

type contextKey string

const UserIDKey contextKey = "user_id"

// JWTAuth validates JWT tokens for web users.
func JWTAuth(authSvc *user.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				http.Error(w, `{"error":"invalid authorization format"}`, http.StatusUnauthorized)
				return
			}

			userID, err := authSvc.VerifyJWT(parts[1])
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// HMACAuth validates HMAC-SHA256 signatures for trading API.
func HMACAuth(keyGetter func(ctx context.Context, apiKey string) (secret string, userID string, err error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")
			timestamp := r.Header.Get("X-Timestamp")
			signature := r.Header.Get("X-Signature")

			if apiKey == "" || timestamp == "" || signature == "" {
				http.Error(w, `{"error":"missing required headers: X-API-Key, X-Timestamp, X-Signature"}`, http.StatusUnauthorized)
				return
			}

			// Verify timestamp is within 5 seconds
			if !user.VerifyTimestamp(timestamp, 5*time.Second) {
				http.Error(w, `{"error":"timestamp expired"}`, http.StatusUnauthorized)
				return
			}

			secret, userID, err := keyGetter(r.Context(), apiKey)
			if err != nil {
				http.Error(w, `{"error":"invalid API key"}`, http.StatusUnauthorized)
				return
			}

			// Verify HMAC signature
			if !user.VerifyHMACSignature(secret, timestamp, r.Method, r.URL.Path, r.URL.RawQuery, "", signature) {
				http.Error(w, `{"error":"invalid signature"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID extracts the user ID from the request context.
func GetUserID(r *http.Request) string {
	userID, _ := r.Context().Value(UserIDKey).(string)
	return userID
}
