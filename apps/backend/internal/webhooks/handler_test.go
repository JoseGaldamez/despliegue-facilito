package webhooks

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/despliegue-facilito/backend/internal/projects"
)

// mockProjectStore is a test double for ProjectStore.
type mockProjectStore struct {
	project *projects.Project
	err     error
}

func (m *mockProjectStore) Create(ctx context.Context, userID string, project projects.Project) error {
	return nil
}

func (m *mockProjectStore) ListByUser(ctx context.Context, userID string) ([]projects.Project, error) {
	return nil, nil
}

func (m *mockProjectStore) GetByRepoURL(ctx context.Context, repoURL string) (*projects.Project, error) {
	return m.project, m.err
}

func (m *mockProjectStore) UpdateStatus(ctx context.Context, projectID string, status string, reason string) error {
	return nil
}

func (m *mockProjectStore) GetByID(ctx context.Context, projectID string) (*projects.Project, error) {
	return m.project, m.err
}

// mockBuildTrigger records whether TriggerBuild was called.
type mockBuildTrigger struct {
	called  bool
	project projects.Project
	err     error
}

func (m *mockBuildTrigger) TriggerBuild(ctx context.Context, project projects.Project) error {
	m.called = true
	m.project = project
	return m.err
}

const testSecret = "test-webhook-secret"

func newTestHandler(store projects.ProjectStore, trigger BuildTrigger) *Handler {
	return &Handler{
		Secret:       testSecret,
		Store:        store,
		BuildTrigger: trigger,
	}
}

func createPushPayload(ref, htmlURL, cloneURL string) []byte {
	payload := GitHubPushEvent{
		Ref: ref,
		Repository: GitHubRepository{
			HTMLURL:  htmlURL,
			CloneURL: cloneURL,
		},
	}
	data, _ := json.Marshal(payload)
	return data
}

func TestHandleWebhook_MissingSignature(t *testing.T) {
	h := newTestHandler(&mockProjectStore{}, nil)

	body := []byte(`{"ref":"refs/heads/main"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/github", strings.NewReader(string(body)))
	// No X-Hub-Signature-256 header
	rec := httptest.NewRecorder()

	h.HandleWebhook(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestHandleWebhook_InvalidSignature(t *testing.T) {
	h := newTestHandler(&mockProjectStore{}, nil)

	body := []byte(`{"ref":"refs/heads/main"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/github", strings.NewReader(string(body)))
	req.Header.Set("X-Hub-Signature-256", "sha256=invalidsignature")
	rec := httptest.NewRecorder()

	h.HandleWebhook(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestHandleWebhook_InvalidJSON(t *testing.T) {
	h := newTestHandler(&mockProjectStore{}, nil)

	body := []byte(`not valid json{{{`)
	sig := ComputeSignature(body, testSecret)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/github", strings.NewReader(string(body)))
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("X-GitHub-Event", "push")
	rec := httptest.NewRecorder()

	h.HandleWebhook(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestHandleWebhook_NonPushEvent(t *testing.T) {
	h := newTestHandler(&mockProjectStore{}, nil)

	body := createPushPayload("refs/heads/main", "https://github.com/owner/repo", "")
	sig := ComputeSignature(body, testSecret)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/github", strings.NewReader(string(body)))
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("X-GitHub-Event", "pull_request") // Not a push event
	rec := httptest.NewRecorder()

	h.HandleWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 for non-push event, got %d", rec.Code)
	}
}

func TestHandleWebhook_PushToNonMainBranch(t *testing.T) {
	h := newTestHandler(&mockProjectStore{}, nil)

	body := createPushPayload("refs/heads/develop", "https://github.com/owner/repo", "")
	sig := ComputeSignature(body, testSecret)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/github", strings.NewReader(string(body)))
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("X-GitHub-Event", "push")
	rec := httptest.NewRecorder()

	h.HandleWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 for non-main branch, got %d", rec.Code)
	}
}

func TestHandleWebhook_ProjectNotFound(t *testing.T) {
	store := &mockProjectStore{project: nil}
	h := newTestHandler(store, nil)

	body := createPushPayload("refs/heads/main", "https://github.com/owner/unknown-repo", "")
	sig := ComputeSignature(body, testSecret)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/github", strings.NewReader(string(body)))
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("X-GitHub-Event", "push")
	rec := httptest.NewRecorder()

	h.HandleWebhook(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestHandleWebhook_ValidPushTriggerssBuild(t *testing.T) {
	testProject := &projects.Project{
		ProjectID: "proj-123",
		UserID:    "user-456",
		RepoURL:   "https://github.com/owner/repo",
	}
	store := &mockProjectStore{project: testProject}
	trigger := &mockBuildTrigger{}
	h := newTestHandler(store, trigger)

	body := createPushPayload("refs/heads/main", "https://github.com/owner/repo", "https://github.com/owner/repo.git")
	sig := ComputeSignature(body, testSecret)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/github", strings.NewReader(string(body)))
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("X-GitHub-Event", "push")
	rec := httptest.NewRecorder()

	h.HandleWebhook(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d", rec.Code)
	}

	if !trigger.called {
		t.Error("expected BuildTrigger.TriggerBuild to be called")
	}

	if trigger.project.ProjectID != "proj-123" {
		t.Errorf("expected project ID 'proj-123', got %q", trigger.project.ProjectID)
	}
}

func TestHandleWebhook_ValidPushWithNilBuildTrigger(t *testing.T) {
	testProject := &projects.Project{
		ProjectID: "proj-789",
		UserID:    "user-101",
		RepoURL:   "https://github.com/owner/my-app",
	}
	store := &mockProjectStore{project: testProject}
	h := newTestHandler(store, nil) // nil BuildTrigger

	body := createPushPayload("refs/heads/main", "https://github.com/owner/my-app", "")
	sig := ComputeSignature(body, testSecret)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/github", strings.NewReader(string(body)))
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("X-GitHub-Event", "push")
	rec := httptest.NewRecorder()

	h.HandleWebhook(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("expected status 202 even with nil BuildTrigger, got %d", rec.Code)
	}
}

func TestComputeSignature(t *testing.T) {
	payload := []byte("hello world")
	secret := "my-secret"

	sig := ComputeSignature(payload, secret)

	if !strings.HasPrefix(sig, "sha256=") {
		t.Errorf("expected signature to start with 'sha256=', got %q", sig)
	}

	// Verify the signature is valid hex after the prefix
	hexPart := strings.TrimPrefix(sig, "sha256=")
	if len(hexPart) != 64 { // SHA-256 produces 32 bytes = 64 hex chars
		t.Errorf("expected 64 hex characters, got %d", len(hexPart))
	}
}

func TestVerifySignature_ValidSignature(t *testing.T) {
	h := &Handler{Secret: testSecret}
	payload := []byte(`{"test": "data"}`)
	sig := ComputeSignature(payload, testSecret)

	if !h.verifySignature(payload, sig) {
		t.Error("expected valid signature to be accepted")
	}
}

func TestVerifySignature_WrongSecret(t *testing.T) {
	h := &Handler{Secret: testSecret}
	payload := []byte(`{"test": "data"}`)
	sig := ComputeSignature(payload, "wrong-secret")

	if h.verifySignature(payload, sig) {
		t.Error("expected signature with wrong secret to be rejected")
	}
}

func TestVerifySignature_MalformedSignature(t *testing.T) {
	h := &Handler{Secret: testSecret}
	payload := []byte(`{"test": "data"}`)

	tests := []struct {
		name string
		sig  string
	}{
		{"no prefix", "abcdef1234567890"},
		{"wrong prefix", "md5=abcdef1234567890"},
		{"empty", ""},
		{"only prefix", "sha256="},
		{"invalid hex", "sha256=zzzzzz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if h.verifySignature(payload, tt.sig) {
				t.Errorf("expected malformed signature %q to be rejected", tt.sig)
			}
		})
	}
}
