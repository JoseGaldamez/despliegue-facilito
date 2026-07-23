package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/despliegue-facilito/backend/internal/middleware"
)

// DefaultGitHubBaseURL is the base URL for the GitHub OAuth endpoints.
const DefaultGitHubBaseURL = "https://github.com"

// DefaultGitHubAPIBaseURL is the base URL for the GitHub REST API.
const DefaultGitHubAPIBaseURL = "https://api.github.com"

// UserStore defines the interface for persisting GitHub connection data.
type UserStore interface {
	SaveGitHubConnection(ctx context.Context, userID, username, accessToken string) error
	GetGitHubConnection(ctx context.Context, userID string) (username string, accessToken string, err error)
}

// FirestoreUserStore implements UserStore using Firestore.
type FirestoreUserStore struct {
	Client *firestore.Client
}

// SaveGitHubConnection stores the GitHub username and access token in the user's Firestore document.
func (s *FirestoreUserStore) SaveGitHubConnection(ctx context.Context, userID, username, accessToken string) error {
	_, err := s.Client.Collection("users").Doc(userID).Set(ctx, map[string]interface{}{
		"githubUsername":    username,
		"githubAccessToken": accessToken,
		"connectedAt":       time.Now(),
	}, firestore.MergeAll)
	return err
}

// GetGitHubConnection retrieves the GitHub username and access token for a user.
func (s *FirestoreUserStore) GetGitHubConnection(ctx context.Context, userID string) (string, string, error) {
	doc, err := s.Client.Collection("users").Doc(userID).Get(ctx)
	if err != nil {
		return "", "", fmt.Errorf("user not found: %w", err)
	}

	data := doc.Data()
	username, _ := data["githubUsername"].(string)
	accessToken, _ := data["githubAccessToken"].(string)

	if username == "" || accessToken == "" {
		return "", "", fmt.Errorf("github not connected")
	}

	return username, accessToken, nil
}

// OAuthHandler handles GitHub OAuth connection requests.
type OAuthHandler struct {
	Store            UserStore
	ClientID         string
	ClientSecret     string
	GitHubBaseURL    string // Base URL for OAuth token exchange (configurable for testing)
	GitHubAPIBaseURL string // Base URL for GitHub API (configurable for testing)
	HTTPClient       *http.Client
}

// NewOAuthHandler creates an OAuthHandler backed by Firestore.
func NewOAuthHandler(firestoreClient *firestore.Client, clientID, clientSecret string) *OAuthHandler {
	return &OAuthHandler{
		Store:        &FirestoreUserStore{Client: firestoreClient},
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}
}

// ConnectRequest is the expected JSON body for the connect endpoint.
type ConnectRequest struct {
	Code string `json:"code"`
}

// ConnectResponse is the JSON response returned on successful connection.
type ConnectResponse struct {
	Username string `json:"username"`
}

// gitHubTokenResponse represents the JSON response from GitHub OAuth token exchange.
type gitHubTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

// gitHubUserResponse represents the relevant fields from GitHub's /user endpoint.
type gitHubUserResponse struct {
	Login string `json:"login"`
}

// getHTTPClient returns the configured HTTP client or the default one.
func (h *OAuthHandler) getHTTPClient() *http.Client {
	if h.HTTPClient != nil {
		return h.HTTPClient
	}
	return http.DefaultClient
}

// getGitHubBaseURL returns the configured GitHub base URL or the default.
func (h *OAuthHandler) getGitHubBaseURL() string {
	if h.GitHubBaseURL != "" {
		return h.GitHubBaseURL
	}
	return DefaultGitHubBaseURL
}

// getGitHubAPIBaseURL returns the configured GitHub API base URL or the default.
func (h *OAuthHandler) getGitHubAPIBaseURL() string {
	if h.GitHubAPIBaseURL != "" {
		return h.GitHubAPIBaseURL
	}
	return DefaultGitHubAPIBaseURL
}

// HandleConnect handles POST /api/v1/github/connect.
// It exchanges the OAuth code for an access token, fetches the GitHub username,
// and stores both in the user's Firestore document.
func (h *OAuthHandler) HandleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract user UID from context (set by auth middleware)
	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		writeErrorResponse(w, http.StatusUnauthorized, "user not authenticated")
		return
	}

	// Parse request body
	var req ConnectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}
	defer r.Body.Close()

	if req.Code == "" {
		writeErrorResponse(w, http.StatusBadRequest, "code is required")
		return
	}

	// Exchange OAuth code for access token
	accessToken, err := h.exchangeCodeForToken(r.Context(), req.Code)
	if err != nil {
		writeErrorResponse(w, http.StatusBadGateway, fmt.Sprintf("failed to exchange code for token: %v", err))
		return
	}

	// Fetch GitHub username
	username, err := h.fetchGitHubUsername(r.Context(), accessToken)
	if err != nil {
		writeErrorResponse(w, http.StatusBadGateway, fmt.Sprintf("failed to fetch GitHub user: %v", err))
		return
	}

	// Store/update user record (merge to handle both new and re-connection)
	if err := h.Store.SaveGitHubConnection(r.Context(), userID, username, accessToken); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("failed to store GitHub connection: %v", err))
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ConnectResponse{Username: username})
}

// exchangeCodeForToken exchanges the OAuth authorization code for an access token.
func (h *OAuthHandler) exchangeCodeForToken(ctx context.Context, code string) (string, error) {
	url := h.getGitHubBaseURL() + "/login/oauth/access_token"

	body := fmt.Sprintf("client_id=%s&client_secret=%s&code=%s", h.ClientID, h.ClientSecret, code)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := h.getHTTPClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var tokenResp gitHubTokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}

	if tokenResp.Error != "" {
		return "", fmt.Errorf("GitHub OAuth error: %s - %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("empty access token in response")
	}

	return tokenResp.AccessToken, nil
}

// fetchGitHubUsername fetches the authenticated user's GitHub username.
func (h *OAuthHandler) fetchGitHubUsername(ctx context.Context, accessToken string) (string, error) {
	url := h.getGitHubAPIBaseURL() + "/user"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := h.getHTTPClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var userResp gitHubUserResponse
	if err := json.Unmarshal(respBody, &userResp); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}

	if userResp.Login == "" {
		return "", fmt.Errorf("empty username in response")
	}

	return userResp.Login, nil
}

// StatusResponse is the JSON response returned for the status endpoint.
type StatusResponse struct {
	Connected bool   `json:"connected"`
	Username  string `json:"username,omitempty"`
}

// RepoInfo represents a GitHub repository summary.
type RepoInfo struct {
	Name        string `json:"name"`
	FullName    string `json:"fullName"`
	HTMLURL     string `json:"htmlUrl"`
	Description string `json:"description"`
	Private     bool   `json:"private"`
}

// ReposResponse is the JSON response returned for the repos endpoint.
type ReposResponse struct {
	Repos []RepoInfo `json:"repos"`
}

// HandleStatus handles GET /api/v1/github/status.
// It checks if the user has a connected GitHub account and returns the status.
func (h *OAuthHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		writeErrorResponse(w, http.StatusUnauthorized, "user not authenticated")
		return
	}

	username, _, err := h.Store.GetGitHubConnection(r.Context(), userID)
	if err != nil {
		// No connection found
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(StatusResponse{Connected: false})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(StatusResponse{Connected: true, Username: username})
}

// HandleRepos handles GET /api/v1/github/repos.
// It fetches the user's GitHub repositories using their stored access token.
func (h *OAuthHandler) HandleRepos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		writeErrorResponse(w, http.StatusUnauthorized, "user not authenticated")
		return
	}

	_, accessToken, err := h.Store.GetGitHubConnection(r.Context(), userID)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "GitHub account not connected")
		return
	}

	repos, err := h.fetchUserRepos(r.Context(), accessToken)
	if err != nil {
		writeErrorResponse(w, http.StatusBadGateway, fmt.Sprintf("failed to fetch repos: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ReposResponse{Repos: repos})
}

// fetchUserRepos fetches the authenticated user's repositories from GitHub.
func (h *OAuthHandler) fetchUserRepos(ctx context.Context, accessToken string) ([]RepoInfo, error) {
	url := h.getGitHubAPIBaseURL() + "/user/repos?sort=updated&per_page=30"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := h.getHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var ghRepos []struct {
		Name        string `json:"name"`
		FullName    string `json:"full_name"`
		HTMLURL     string `json:"html_url"`
		Description string `json:"description"`
		Private     bool   `json:"private"`
	}
	if err := json.Unmarshal(respBody, &ghRepos); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	repos := make([]RepoInfo, 0, len(ghRepos))
	for _, r := range ghRepos {
		repos = append(repos, RepoInfo{
			Name:        r.Name,
			FullName:    r.FullName,
			HTMLURL:     r.HTMLURL,
			Description: r.Description,
			Private:     r.Private,
		})
	}

	return repos, nil
}

// writeErrorResponse writes a JSON error response.
func writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   http.StatusText(statusCode),
		"message": message,
	})
}
