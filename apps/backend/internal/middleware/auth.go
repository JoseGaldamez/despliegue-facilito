package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"firebase.google.com/go/v4/auth"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const userIDContextKey contextKey = "userID"

// TokenVerifier is an interface for verifying Firebase ID tokens.
// This allows injecting a mock in tests.
type TokenVerifier interface {
	VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error)
}

// Auth returns an HTTP middleware that verifies Firebase ID tokens.
// It extracts the Bearer token from the Authorization header, verifies it,
// and attaches the user UID to the request context.
func Auth(verifier TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeAuthError(w, "missing authorization header")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
				writeAuthError(w, "invalid authorization header format")
				return
			}

			token, err := verifier.VerifyIDToken(r.Context(), parts[1])
			if err != nil {
				writeAuthError(w, "invalid or expired token")
				return
			}

			// Attach user UID to context
			ctx := context.WithValue(r.Context(), userIDContextKey, token.UID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserIDFromContext retrieves the authenticated user's UID from the request context.
// Returns an empty string if no user ID is present.
func GetUserIDFromContext(ctx context.Context) string {
	uid, ok := ctx.Value(userIDContextKey).(string)
	if !ok {
		return ""
	}
	return uid
}

// SetUserIDInContext sets the user ID in the given context.
// This is useful for testing handlers that depend on the auth middleware.
func SetUserIDInContext(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDContextKey, userID)
}

// writeAuthError writes a JSON 401 Unauthorized response.
func writeAuthError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   "unauthorized",
		"message": message,
	})
}
