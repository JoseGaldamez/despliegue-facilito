package projects

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/despliegue-facilito/backend/internal/detector"
	"github.com/despliegue-facilito/backend/internal/middleware"
)

// --- Mocks ---

type mockProjectStore struct {
	projects  []Project
	listErr   error
	createErr error
	getByRepo *Project
	getErr    error
}

func (m *mockProjectStore) Create(ctx context.Context, userID string, project Project) error {
	return m.createErr
}

func (m *mockProjectStore) ListByUser(ctx context.Context, userID string) ([]Project, error) {
	return m.projects, m.listErr
}

func (m *mockProjectStore) GetByRepoURL(ctx context.Context, repoURL string) (*Project, error) {
	return m.getByRepo, m.getErr
}

func (m *mockProjectStore) UpdateStatus(ctx context.Context, projectID string, status string, reason string) error {
	return nil
}

func (m *mockProjectStore) GetByID(ctx context.Context, projectID string) (*Project, error) {
	return nil, nil
}

type mockDetector struct {
	framework detector.Framework
	err       error
}

func (m *mockDetector) Detect(ctx context.Context, repoURL string, token string) (detector.Framework, error) {
	return m.framework, m.err
}

type mockTokenFetcher struct {
	token string
	err   error
}

func (m *mockTokenFetcher) GetGitHubToken(ctx context.Context, userID string) (string, error) {
	return m.token, m.err
}

// --- Helper ---

func newAuthenticatedRequest(method, path string, body interface{}) *http.Request {
	var reqBody *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")

	ctx := middleware.SetUserIDInContext(req.Context(), "test-user-123")
	return req.WithContext(ctx)
}

func newUnauthenticatedRequest(method, path string, body interface{}) *http.Request {
	var reqBody *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	return req
}

// --- Tests: HandleList ---

func TestHandleList_EmptyProjects(t *testing.T) {
	h := &Handler{
		Store: &mockProjectStore{projects: nil},
	}

	req := newAuthenticatedRequest(http.MethodGet, "/api/v1/projects", nil)
	rr := httptest.NewRecorder()

	h.HandleList(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp map[string][]Project
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp["projects"]) != 0 {
		t.Errorf("expected empty array, got %d projects", len(resp["projects"]))
	}
}

func TestHandleList_WithProjects(t *testing.T) {
	now := time.Now()
	h := &Handler{
		Store: &mockProjectStore{
			projects: []Project{
				{
					ProjectID:        "proj-1",
					UserID:           "test-user-123",
					RepoURL:          "https://github.com/user/repo1",
					Framework:        "react",
					BuildCommand:     "npm run build",
					OutputDir:        "dist",
					DeploymentStatus: "pending",
					CreatedAt:        now,
					UpdatedAt:        now,
				},
				{
					ProjectID:        "proj-2",
					UserID:           "test-user-123",
					RepoURL:          "https://github.com/user/repo2",
					Framework:        "vue",
					BuildCommand:     "npm run build",
					OutputDir:        "dist",
					DeploymentStatus: "deployed",
					CreatedAt:        now,
					UpdatedAt:        now,
				},
			},
		},
	}

	req := newAuthenticatedRequest(http.MethodGet, "/api/v1/projects", nil)
	rr := httptest.NewRecorder()

	h.HandleList(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp map[string][]map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp["projects"]) != 2 {
		t.Errorf("expected 2 projects, got %d", len(resp["projects"]))
	}
}

func TestHandleList_Unauthenticated(t *testing.T) {
	h := &Handler{
		Store: &mockProjectStore{},
	}

	req := newUnauthenticatedRequest(http.MethodGet, "/api/v1/projects", nil)
	rr := httptest.NewRecorder()

	h.HandleList(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestHandleList_StoreError(t *testing.T) {
	h := &Handler{
		Store: &mockProjectStore{listErr: errors.New("firestore error")},
	}

	req := newAuthenticatedRequest(http.MethodGet, "/api/v1/projects", nil)
	rr := httptest.NewRecorder()

	h.HandleList(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

// --- Tests: HandleCreate ---

func TestHandleCreate_Success(t *testing.T) {
	h := &Handler{
		Store: &mockProjectStore{
			getByRepo: nil, // no duplicate
		},
		Detector: &mockDetector{
			framework: detector.Framework{
				Name:         "react",
				BuildCommand: "npm run build",
				OutputDir:    "dist",
			},
		},
		TokenFetcher: &mockTokenFetcher{token: "ghp_test_token"},
	}

	body := CreateProjectRequest{RepoURL: "https://github.com/user/my-app"}
	req := newAuthenticatedRequest(http.MethodPost, "/api/v1/projects", body)
	rr := httptest.NewRecorder()

	h.HandleCreate(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d. body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	project := resp["project"]
	if project["repoUrl"] != "https://github.com/user/my-app" {
		t.Errorf("expected repoUrl to be set, got %v", project["repoUrl"])
	}
	if project["framework"] != "react" {
		t.Errorf("expected framework 'react', got %v", project["framework"])
	}
}

func TestHandleCreate_DuplicateRepo(t *testing.T) {
	h := &Handler{
		Store: &mockProjectStore{
			getByRepo: &Project{
				ProjectID: "existing-proj",
				RepoURL:   "https://github.com/user/my-app",
			},
		},
		Detector:     &mockDetector{},
		TokenFetcher: &mockTokenFetcher{token: "ghp_test_token"},
	}

	body := CreateProjectRequest{RepoURL: "https://github.com/user/my-app"}
	req := newAuthenticatedRequest(http.MethodPost, "/api/v1/projects", body)
	rr := httptest.NewRecorder()

	h.HandleCreate(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", rr.Code)
	}
}

func TestHandleCreate_MissingRepoURL(t *testing.T) {
	h := &Handler{
		Store:        &mockProjectStore{},
		Detector:     &mockDetector{},
		TokenFetcher: &mockTokenFetcher{token: "ghp_test_token"},
	}

	body := CreateProjectRequest{RepoURL: ""}
	req := newAuthenticatedRequest(http.MethodPost, "/api/v1/projects", body)
	rr := httptest.NewRecorder()

	h.HandleCreate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestHandleCreate_MissingGitHubToken(t *testing.T) {
	h := &Handler{
		Store: &mockProjectStore{
			getByRepo: nil,
		},
		Detector:     &mockDetector{},
		TokenFetcher: &mockTokenFetcher{token: ""},
	}

	body := CreateProjectRequest{RepoURL: "https://github.com/user/my-app"}
	req := newAuthenticatedRequest(http.MethodPost, "/api/v1/projects", body)
	rr := httptest.NewRecorder()

	h.HandleCreate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d. body: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreate_TokenFetchError(t *testing.T) {
	h := &Handler{
		Store: &mockProjectStore{
			getByRepo: nil,
		},
		Detector:     &mockDetector{},
		TokenFetcher: &mockTokenFetcher{err: errors.New("firestore error")},
	}

	body := CreateProjectRequest{RepoURL: "https://github.com/user/my-app"}
	req := newAuthenticatedRequest(http.MethodPost, "/api/v1/projects", body)
	rr := httptest.NewRecorder()

	h.HandleCreate(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

func TestHandleCreate_Unauthenticated(t *testing.T) {
	h := &Handler{
		Store:        &mockProjectStore{},
		Detector:     &mockDetector{},
		TokenFetcher: &mockTokenFetcher{},
	}

	body := CreateProjectRequest{RepoURL: "https://github.com/user/my-app"}
	req := newUnauthenticatedRequest(http.MethodPost, "/api/v1/projects", body)
	rr := httptest.NewRecorder()

	h.HandleCreate(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestHandleCreate_DetectorError(t *testing.T) {
	h := &Handler{
		Store: &mockProjectStore{
			getByRepo: nil,
		},
		Detector:     &mockDetector{err: errors.New("github api error")},
		TokenFetcher: &mockTokenFetcher{token: "ghp_test_token"},
	}

	body := CreateProjectRequest{RepoURL: "https://github.com/user/my-app"}
	req := newAuthenticatedRequest(http.MethodPost, "/api/v1/projects", body)
	rr := httptest.NewRecorder()

	h.HandleCreate(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}
