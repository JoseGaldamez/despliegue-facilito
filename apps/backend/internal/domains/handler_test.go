package domains

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/despliegue-facilito/backend/internal/middleware"
	"github.com/despliegue-facilito/backend/internal/projects"
	"github.com/go-chi/chi/v5"
)

// mockProjectStore implements projects.ProjectStore for testing.
type mockProjectStore struct {
	project *projects.Project
	err     error
}

func (m *mockProjectStore) Create(ctx context.Context, userID string, project projects.Project) error {
	return m.err
}
func (m *mockProjectStore) ListByUser(ctx context.Context, userID string) ([]projects.Project, error) {
	return nil, m.err
}
func (m *mockProjectStore) GetByID(ctx context.Context, projectID string) (*projects.Project, error) {
	return m.project, m.err
}
func (m *mockProjectStore) GetByRepoURL(ctx context.Context, repoURL string) (*projects.Project, error) {
	return nil, m.err
}
func (m *mockProjectStore) UpdateStatus(ctx context.Context, projectID string, status string, reason string) error {
	return m.err
}

// mockDomainStore implements DomainStore for testing.
type mockDomainStore struct {
	domains        []Domain
	domainByHost   *Domain
	addedDomain    Domain
	addErr         error
	listErr        error
	deleteErr      error
	getByHostErr   error
}

func (m *mockDomainStore) Add(ctx context.Context, projectID string, hostname string) (Domain, error) {
	if m.addErr != nil {
		return Domain{}, m.addErr
	}
	d := Domain{
		DomainID:  "domain-123",
		ProjectID: projectID,
		Hostname:  hostname,
		CreatedAt: time.Now().UTC(),
	}
	m.addedDomain = d
	return d, nil
}
func (m *mockDomainStore) ListByProject(ctx context.Context, projectID string) ([]Domain, error) {
	return m.domains, m.listErr
}
func (m *mockDomainStore) Delete(ctx context.Context, projectID string, domainID string) error {
	return m.deleteErr
}
func (m *mockDomainStore) GetByHostname(ctx context.Context, hostname string) (*Domain, error) {
	return m.domainByHost, m.getByHostErr
}

// helper to create a request with chi URL params and auth context.
func newRequest(method, path string, body interface{}, userID string, urlParams map[string]string) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")

	// Set auth context
	if userID != "" {
		ctx := middleware.SetUserIDInContext(req.Context(), userID)
		req = req.WithContext(ctx)
	}

	// Set chi URL params
	if len(urlParams) > 0 {
		rctx := chi.NewRouteContext()
		for key, val := range urlParams {
			rctx.URLParams.Add(key, val)
		}
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		req = req.WithContext(ctx)
	}

	return req
}

func TestHandleAdd_Success(t *testing.T) {
	h := &Handler{
		DomainStore: &mockDomainStore{},
		ProjectStore: &mockProjectStore{
			project: &projects.Project{ProjectID: "proj-1", UserID: "user-1"},
		},
	}

	req := newRequest("POST", "/api/v1/projects/proj-1/domains",
		AddDomainRequest{Hostname: "app.example.com"}, "user-1",
		map[string]string{"project_id": "proj-1"})
	w := httptest.NewRecorder()

	h.HandleAdd(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("HandleAdd() status = %d, want %d", w.Code, http.StatusCreated)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	domain, ok := resp["domain"].(map[string]interface{})
	if !ok {
		t.Fatal("HandleAdd() response missing domain object")
	}
	if domain["hostname"] != "app.example.com" {
		t.Errorf("HandleAdd() hostname = %v, want app.example.com", domain["hostname"])
	}
}

func TestHandleAdd_InvalidHostname(t *testing.T) {
	h := &Handler{
		DomainStore: &mockDomainStore{},
		ProjectStore: &mockProjectStore{
			project: &projects.Project{ProjectID: "proj-1", UserID: "user-1"},
		},
	}

	req := newRequest("POST", "/api/v1/projects/proj-1/domains",
		AddDomainRequest{Hostname: "invalid"}, "user-1",
		map[string]string{"project_id": "proj-1"})
	w := httptest.NewRecorder()

	h.HandleAdd(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("HandleAdd() status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleAdd_DuplicateHostname(t *testing.T) {
	h := &Handler{
		DomainStore: &mockDomainStore{
			domainByHost: &Domain{DomainID: "existing-domain", Hostname: "app.example.com"},
		},
		ProjectStore: &mockProjectStore{
			project: &projects.Project{ProjectID: "proj-1", UserID: "user-1"},
		},
	}

	req := newRequest("POST", "/api/v1/projects/proj-1/domains",
		AddDomainRequest{Hostname: "app.example.com"}, "user-1",
		map[string]string{"project_id": "proj-1"})
	w := httptest.NewRecorder()

	h.HandleAdd(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("HandleAdd() status = %d, want %d", w.Code, http.StatusConflict)
	}
}

func TestHandleAdd_Forbidden(t *testing.T) {
	h := &Handler{
		DomainStore: &mockDomainStore{},
		ProjectStore: &mockProjectStore{
			project: &projects.Project{ProjectID: "proj-1", UserID: "other-user"},
		},
	}

	req := newRequest("POST", "/api/v1/projects/proj-1/domains",
		AddDomainRequest{Hostname: "app.example.com"}, "user-1",
		map[string]string{"project_id": "proj-1"})
	w := httptest.NewRecorder()

	h.HandleAdd(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("HandleAdd() status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestHandleAdd_Unauthenticated(t *testing.T) {
	h := &Handler{
		DomainStore:  &mockDomainStore{},
		ProjectStore: &mockProjectStore{},
	}

	req := newRequest("POST", "/api/v1/projects/proj-1/domains",
		AddDomainRequest{Hostname: "app.example.com"}, "",
		map[string]string{"project_id": "proj-1"})
	w := httptest.NewRecorder()

	h.HandleAdd(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("HandleAdd() status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleList_Success(t *testing.T) {
	h := &Handler{
		DomainStore: &mockDomainStore{
			domains: []Domain{
				{DomainID: "d1", ProjectID: "proj-1", Hostname: "app.example.com"},
				{DomainID: "d2", ProjectID: "proj-1", Hostname: "www.example.com"},
			},
		},
		ProjectStore: &mockProjectStore{
			project: &projects.Project{ProjectID: "proj-1", UserID: "user-1"},
		},
	}

	req := newRequest("GET", "/api/v1/projects/proj-1/domains", nil, "user-1",
		map[string]string{"project_id": "proj-1"})
	w := httptest.NewRecorder()

	h.HandleList(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("HandleList() status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	domainsList, ok := resp["domains"].([]interface{})
	if !ok {
		t.Fatal("HandleList() response missing domains array")
	}
	if len(domainsList) != 2 {
		t.Errorf("HandleList() domains count = %d, want 2", len(domainsList))
	}
}

func TestHandleList_Forbidden(t *testing.T) {
	h := &Handler{
		DomainStore: &mockDomainStore{},
		ProjectStore: &mockProjectStore{
			project: &projects.Project{ProjectID: "proj-1", UserID: "other-user"},
		},
	}

	req := newRequest("GET", "/api/v1/projects/proj-1/domains", nil, "user-1",
		map[string]string{"project_id": "proj-1"})
	w := httptest.NewRecorder()

	h.HandleList(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("HandleList() status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestHandleDelete_Success(t *testing.T) {
	h := &Handler{
		DomainStore: &mockDomainStore{},
		ProjectStore: &mockProjectStore{
			project: &projects.Project{ProjectID: "proj-1", UserID: "user-1"},
		},
	}

	req := newRequest("DELETE", "/api/v1/projects/proj-1/domains/domain-1", nil, "user-1",
		map[string]string{"project_id": "proj-1", "domain_id": "domain-1"})
	w := httptest.NewRecorder()

	h.HandleDelete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("HandleDelete() status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

func TestHandleDelete_Forbidden(t *testing.T) {
	h := &Handler{
		DomainStore: &mockDomainStore{},
		ProjectStore: &mockProjectStore{
			project: &projects.Project{ProjectID: "proj-1", UserID: "other-user"},
		},
	}

	req := newRequest("DELETE", "/api/v1/projects/proj-1/domains/domain-1", nil, "user-1",
		map[string]string{"project_id": "proj-1", "domain_id": "domain-1"})
	w := httptest.NewRecorder()

	h.HandleDelete(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("HandleDelete() status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestHandleAdd_ProjectNotFound(t *testing.T) {
	h := &Handler{
		DomainStore: &mockDomainStore{},
		ProjectStore: &mockProjectStore{
			project: nil,
		},
	}

	req := newRequest("POST", "/api/v1/projects/proj-1/domains",
		AddDomainRequest{Hostname: "app.example.com"}, "user-1",
		map[string]string{"project_id": "proj-1"})
	w := httptest.NewRecorder()

	h.HandleAdd(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("HandleAdd() status = %d, want %d", w.Code, http.StatusNotFound)
	}
}
