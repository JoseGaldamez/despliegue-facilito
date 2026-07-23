package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/despliegue-facilito/backend/internal/middleware"
)

// mockUserStore is a test double for UserStore.
type mockUserStore struct {
	savedUserID      string
	savedUsername    string
	savedAccessToken string
	err              error
}

func (m *mockUserStore) SaveGitHubConnection(_ context.Context, userID, username, accessToken string) error {
	m.savedUserID = userID
	m.savedUsername = username
	m.savedAccessToken = accessToken
	return m.err
}

// setupMockGitHub creates mock GitHub OAuth and API servers.
// Returns the handler configured with mock URLs.
func setupMockGitHub(t *testing.T, store UserStore, tokenHandler, userHandler http.HandlerFunc) *OAuthHandler {
	t.Helper()

	oauthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login/oauth/access_token" {
			tokenHandler(w, r)
			return
		}
		http.NotFound(w, r)
	}))

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user" {
			userHandler(w, r)
			return
		}
		http.NotFound(w, r)
	}))

	handler := &OAuthHandler{
		Store:            store,
		ClientID:         "test-client-id",
		ClientSecret:     "test-client-secret",
		GitHubBaseURL:    oauthServer.URL,
		GitHubAPIBaseURL: apiServer.URL,
	}

	t.Cleanup(func() {
		oauthServer.Close()
		apiServer.Close()
	})

	return handler
}

// createAuthenticatedRequest creates a request with the user ID set in context
// the same way the auth middleware does.
func createAuthenticatedRequest(t *testing.T, method, url string, body []byte, userID string) *http.Request {
	t.Helper()
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, url, bytes.NewReader(body))
	} else {
		req = httptest.NewRequest(method, url, nil)
	}
	req.Header.Set("Content-Type", "application/json")

	ctx := middleware.SetUserIDInContext(req.Context(), userID)
	return req.WithContext(ctx)
}

func TestHandleConnect_MissingAuth(t *testing.T) {
	handler := &OAuthHandler{
		Store:        &mockUserStore{},
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}

	body := []byte(`{"code": "test-code"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/github/connect", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.HandleConnect(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestHandleConnect_InvalidMethod(t *testing.T) {
	handler := &OAuthHandler{
		Store:        &mockUserStore{},
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/github/connect", nil)
	rr := httptest.NewRecorder()
	handler.HandleConnect(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}

func TestHandleConnect_EmptyCode(t *testing.T) {
	handler := &OAuthHandler{
		Store:        &mockUserStore{},
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}

	body := []byte(`{"code": ""}`)
	req := createAuthenticatedRequest(t, http.MethodPost, "/api/v1/github/connect", body, "user-123")

	rr := httptest.NewRecorder()
	handler.HandleConnect(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestHandleConnect_InvalidBody(t *testing.T) {
	handler := &OAuthHandler{
		Store:        &mockUserStore{},
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}

	body := []byte(`not json`)
	req := createAuthenticatedRequest(t, http.MethodPost, "/api/v1/github/connect", body, "user-123")

	rr := httptest.NewRecorder()
	handler.HandleConnect(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestHandleConnect_TokenExchangeFailure(t *testing.T) {
	store := &mockUserStore{}
	tokenHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}
	userHandler := func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("user endpoint should not be called")
	}

	handler := setupMockGitHub(t, store, tokenHandler, userHandler)

	body := []byte(`{"code": "bad-code"}`)
	req := createAuthenticatedRequest(t, http.MethodPost, "/api/v1/github/connect", body, "user-123")

	rr := httptest.NewRecorder()
	handler.HandleConnect(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Errorf("expected status %d, got %d", http.StatusBadGateway, rr.Code)
	}
}

func TestHandleConnect_GitHubOAuthError(t *testing.T) {
	store := &mockUserStore{}
	tokenHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"error":             "bad_verification_code",
			"error_description": "The code passed is incorrect or expired.",
		})
	}
	userHandler := func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("user endpoint should not be called")
	}

	handler := setupMockGitHub(t, store, tokenHandler, userHandler)

	body := []byte(`{"code": "expired-code"}`)
	req := createAuthenticatedRequest(t, http.MethodPost, "/api/v1/github/connect", body, "user-123")

	rr := httptest.NewRecorder()
	handler.HandleConnect(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Errorf("expected status %d, got %d", http.StatusBadGateway, rr.Code)
	}
}

func TestHandleConnect_GitHubUserFetchFailure(t *testing.T) {
	store := &mockUserStore{}
	tokenHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"access_token": "gho_valid_token",
			"token_type":   "bearer",
			"scope":        "repo",
		})
	}
	userHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("bad credentials"))
	}

	handler := setupMockGitHub(t, store, tokenHandler, userHandler)

	body := []byte(`{"code": "valid-code"}`)
	req := createAuthenticatedRequest(t, http.MethodPost, "/api/v1/github/connect", body, "user-123")

	rr := httptest.NewRecorder()
	handler.HandleConnect(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Errorf("expected status %d, got %d", http.StatusBadGateway, rr.Code)
	}
}

func TestHandleConnect_Success(t *testing.T) {
	store := &mockUserStore{}
	tokenHandler := func(w http.ResponseWriter, r *http.Request) {
		// Verify request parameters
		if err := r.ParseForm(); err != nil {
			t.Fatalf("failed to parse form: %v", err)
		}
		if r.FormValue("client_id") != "test-client-id" {
			t.Errorf("expected client_id 'test-client-id', got '%s'", r.FormValue("client_id"))
		}
		if r.FormValue("client_secret") != "test-client-secret" {
			t.Errorf("expected client_secret 'test-client-secret', got '%s'", r.FormValue("client_secret"))
		}
		if r.FormValue("code") != "valid-code" {
			t.Errorf("expected code 'valid-code', got '%s'", r.FormValue("code"))
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("expected Accept header 'application/json', got '%s'", r.Header.Get("Accept"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"access_token": "gho_valid_token_123",
			"token_type":   "bearer",
			"scope":        "repo",
		})
	}
	userHandler := func(w http.ResponseWriter, r *http.Request) {
		// Verify authorization header
		if r.Header.Get("Authorization") != "Bearer gho_valid_token_123" {
			t.Errorf("expected auth header 'Bearer gho_valid_token_123', got '%s'", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"login": "octocat",
		})
	}

	handler := setupMockGitHub(t, store, tokenHandler, userHandler)

	body := []byte(`{"code": "valid-code"}`)
	req := createAuthenticatedRequest(t, http.MethodPost, "/api/v1/github/connect", body, "user-123")

	rr := httptest.NewRecorder()
	handler.HandleConnect(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Verify response body
	var resp ConnectResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Username != "octocat" {
		t.Errorf("expected username 'octocat', got '%s'", resp.Username)
	}

	// Verify store was called correctly
	if store.savedUserID != "user-123" {
		t.Errorf("expected saved userID 'user-123', got '%s'", store.savedUserID)
	}
	if store.savedUsername != "octocat" {
		t.Errorf("expected saved username 'octocat', got '%s'", store.savedUsername)
	}
	if store.savedAccessToken != "gho_valid_token_123" {
		t.Errorf("expected saved token 'gho_valid_token_123', got '%s'", store.savedAccessToken)
	}
}

func TestHandleConnect_FirestoreWriteFailure(t *testing.T) {
	store := &mockUserStore{err: errors.New("firestore unavailable")}
	tokenHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"access_token": "gho_token",
			"token_type":   "bearer",
		})
	}
	userHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"login": "testuser"})
	}

	handler := setupMockGitHub(t, store, tokenHandler, userHandler)

	body := []byte(`{"code": "some-code"}`)
	req := createAuthenticatedRequest(t, http.MethodPost, "/api/v1/github/connect", body, "user-456")

	rr := httptest.NewRecorder()
	handler.HandleConnect(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}

func TestHandleConnect_VerifiesTokenExchangeHeaders(t *testing.T) {
	store := &mockUserStore{}
	var receivedContentType string
	var receivedAccept string

	tokenHandler := func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		receivedAccept = r.Header.Get("Accept")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"access_token": "gho_token",
			"token_type":   "bearer",
		})
	}
	userHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"login": "testuser"})
	}

	handler := setupMockGitHub(t, store, tokenHandler, userHandler)

	body := []byte(`{"code": "some-code"}`)
	req := createAuthenticatedRequest(t, http.MethodPost, "/api/v1/github/connect", body, "user-456")

	rr := httptest.NewRecorder()
	handler.HandleConnect(rr, req)

	if receivedContentType != "application/x-www-form-urlencoded" {
		t.Errorf("expected Content-Type 'application/x-www-form-urlencoded', got '%s'", receivedContentType)
	}
	if receivedAccept != "application/json" {
		t.Errorf("expected Accept 'application/json', got '%s'", receivedAccept)
	}
}

func TestHandleConnect_EmptyTokenResponse(t *testing.T) {
	store := &mockUserStore{}
	tokenHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"access_token": "",
			"token_type":   "bearer",
		})
	}
	userHandler := func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("user endpoint should not be called")
	}

	handler := setupMockGitHub(t, store, tokenHandler, userHandler)

	body := []byte(`{"code": "code-with-empty-token"}`)
	req := createAuthenticatedRequest(t, http.MethodPost, "/api/v1/github/connect", body, "user-789")

	rr := httptest.NewRecorder()
	handler.HandleConnect(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Errorf("expected status %d, got %d", http.StatusBadGateway, rr.Code)
	}
}

func TestHandleConnect_ReConnection(t *testing.T) {
	// Test that re-connection replaces the stored token
	store := &mockUserStore{}
	tokenHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"access_token": "gho_new_token",
			"token_type":   "bearer",
		})
	}
	userHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"login": "octocat"})
	}

	handler := setupMockGitHub(t, store, tokenHandler, userHandler)

	body := []byte(`{"code": "new-code"}`)
	req := createAuthenticatedRequest(t, http.MethodPost, "/api/v1/github/connect", body, "user-123")

	rr := httptest.NewRecorder()
	handler.HandleConnect(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Verify the new token was stored
	if store.savedAccessToken != "gho_new_token" {
		t.Errorf("expected saved token 'gho_new_token', got '%s'", store.savedAccessToken)
	}
	if store.savedUsername != "octocat" {
		t.Errorf("expected saved username 'octocat', got '%s'", store.savedUsername)
	}
}
