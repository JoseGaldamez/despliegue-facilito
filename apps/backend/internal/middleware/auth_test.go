package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"firebase.google.com/go/v4/auth"
)

// mockTokenVerifier implements TokenVerifier for testing.
type mockTokenVerifier struct {
	token *auth.Token
	err   error
}

func (m *mockTokenVerifier) VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error) {
	return m.token, m.err
}

// dummyHandler writes the user ID from context to verify it was attached.
func dummyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid := GetUserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(uid))
	})
}

func TestAuth_MissingAuthorizationHeader(t *testing.T) {
	verifier := &mockTokenVerifier{}
	handler := Auth(verifier)(dummyHandler())

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body["error"] != "unauthorized" {
		t.Errorf("expected error 'unauthorized', got '%s'", body["error"])
	}
	if body["message"] != "missing authorization header" {
		t.Errorf("expected message 'missing authorization header', got '%s'", body["message"])
	}
}

func TestAuth_MalformedAuthorizationHeader(t *testing.T) {
	verifier := &mockTokenVerifier{}
	handler := Auth(verifier)(dummyHandler())

	testCases := []struct {
		name   string
		header string
	}{
		{"no Bearer prefix", "Token abc123"},
		{"Bearer with no token", "Bearer "},
		{"just Bearer", "Bearer"},
		{"wrong scheme", "Basic abc123"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			req.Header.Set("Authorization", tc.header)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("expected status 401, got %d", rec.Code)
			}

			var body map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}
			if body["error"] != "unauthorized" {
				t.Errorf("expected error 'unauthorized', got '%s'", body["error"])
			}
		})
	}
}

func TestAuth_InvalidToken(t *testing.T) {
	verifier := &mockTokenVerifier{
		token: nil,
		err:   errors.New("token verification failed"),
	}
	handler := Auth(verifier)(dummyHandler())

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-value")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body["message"] != "invalid or expired token" {
		t.Errorf("expected message 'invalid or expired token', got '%s'", body["message"])
	}
}

func TestAuth_ValidToken(t *testing.T) {
	verifier := &mockTokenVerifier{
		token: &auth.Token{UID: "user-123"},
		err:   nil,
	}
	handler := Auth(verifier)(dummyHandler())

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	if rec.Body.String() != "user-123" {
		t.Errorf("expected body 'user-123', got '%s'", rec.Body.String())
	}
}

func TestGetUserIDFromContext_NotSet(t *testing.T) {
	ctx := context.Background()
	uid := GetUserIDFromContext(ctx)
	if uid != "" {
		t.Errorf("expected empty string, got '%s'", uid)
	}
}

func TestGetUserIDFromContext_Set(t *testing.T) {
	ctx := context.WithValue(context.Background(), userIDContextKey, "test-uid")
	uid := GetUserIDFromContext(ctx)
	if uid != "test-uid" {
		t.Errorf("expected 'test-uid', got '%s'", uid)
	}
}
