package projects

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"cloud.google.com/go/firestore"
	"github.com/despliegue-facilito/backend/internal/detector"
	"github.com/despliegue-facilito/backend/internal/middleware"
	"github.com/go-chi/chi/v5"
)

// UserTokenFetcher abstracts fetching a user's GitHub access token.
type UserTokenFetcher interface {
	GetGitHubToken(ctx context.Context, userID string) (string, error)
}

// FirestoreUserTokenFetcher fetches the GitHub access token from Firestore.
type FirestoreUserTokenFetcher struct {
	Client *firestore.Client
}

// GetGitHubToken retrieves the GitHub access token for the given user from Firestore.
func (f *FirestoreUserTokenFetcher) GetGitHubToken(ctx context.Context, userID string) (string, error) {
	doc, err := f.Client.Collection("users").Doc(userID).Get(ctx)
	if err != nil {
		return "", err
	}

	data := doc.Data()
	token, ok := data["githubAccessToken"].(string)
	if !ok || token == "" {
		return "", nil
	}

	return token, nil
}

// CDNProvisioner abstracts provisioning a CDN distribution for a project.
type CDNProvisioner interface {
	ProvisionDistribution(ctx context.Context, project Project) error
}

// Handler handles HTTP requests for project endpoints.
type Handler struct {
	Store          ProjectStore
	Detector       detector.FrameworkDetector
	TokenFetcher   UserTokenFetcher
	BuildTrigger   BuildTrigger
	CDNProvisioner CDNProvisioner
}

// BuildTrigger abstracts triggering a build for a project.
type BuildTrigger interface {
	TriggerBuild(ctx context.Context, project Project) error
}

// CreateProjectRequest is the expected JSON body for creating a project.
type CreateProjectRequest struct {
	RepoURL        string `json:"repoUrl"`
	Subdomain      string `json:"subdomain"`
	PackageManager string `json:"packageManager"`
	BuildCommand   string `json:"buildCommand"`
	OutputDir      string `json:"outputDir"`
}

// HandleList handles GET /api/v1/projects.
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, errorResponse("unauthorized", "user not authenticated"))
		return
	}

	projects, err := h.Store.ListByUser(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse("internal_error", "failed to list projects"))
		return
	}

	if projects == nil {
		projects = []Project{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"projects": projects,
	})
}

// HandleGet handles GET /api/v1/projects/{project_id}.
func (h *Handler) HandleGet(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, errorResponse("unauthorized", "user not authenticated"))
		return
	}

	projectID := chi.URLParam(r, "project_id")
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse("bad_request", "project_id is required"))
		return
	}

	project, err := h.Store.GetByID(r.Context(), projectID)
	if err != nil || project == nil {
		writeJSON(w, http.StatusNotFound, errorResponse("not_found", "project not found"))
		return
	}
	if project.UserID != userID {
		writeJSON(w, http.StatusForbidden, errorResponse("forbidden", "you do not own this project"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"project": project,
	})
}

// HandleCreate handles POST /api/v1/projects.
func (h *Handler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, errorResponse("unauthorized", "user not authenticated"))
		return
	}

	var req CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse("bad_request", "invalid request body"))
		return
	}
	defer r.Body.Close()

	if req.RepoURL == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse("bad_request", "repoUrl is required"))
		return
	}

	if req.Subdomain == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse("bad_request", "subdomain is required"))
		return
	}

	// Check for duplicate repo URL
	existing, err := h.Store.GetByRepoURL(r.Context(), req.RepoURL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse("internal_error", "failed to check for duplicates"))
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusConflict, errorResponse("conflict", "repository is already linked to a project"))
		return
	}

	// Check for duplicate subdomain
	existingSub, err := h.Store.GetBySubdomain(r.Context(), req.Subdomain)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse("internal_error", "failed to check subdomain availability"))
		return
	}
	if existingSub != nil {
		writeJSON(w, http.StatusConflict, errorResponse("conflict", "subdomain is already taken"))
		return
	}

	// Fetch user's GitHub token
	token, err := h.TokenFetcher.GetGitHubToken(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse("internal_error", "failed to fetch GitHub token"))
		return
	}
	if token == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse("bad_request", "GitHub account not connected"))
		return
	}

	// Detect framework
	framework, err := h.Detector.Detect(r.Context(), req.RepoURL, token)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse("internal_error", "failed to detect framework"))
		return
	}

	// Use detected values as defaults, allow override from request
	buildCommand := framework.BuildCommand
	if req.BuildCommand != "" {
		buildCommand = req.BuildCommand
	}
	outputDir := framework.OutputDir
	if req.OutputDir != "" {
		outputDir = req.OutputDir
	}
	packageManager := "npm"
	if req.PackageManager != "" {
		packageManager = req.PackageManager
	}

	// Create project
	project := &Project{
		RepoURL:        req.RepoURL,
		Subdomain:      req.Subdomain,
		Framework:      framework.Name,
		PackageManager: packageManager,
		BuildCommand:   buildCommand,
		OutputDir:      outputDir,
	}

	if err := h.Store.Create(r.Context(), userID, project); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse("internal_error", "failed to create project"))
		return
	}

	// Trigger initial build and CDN provisioning in background
	if h.BuildTrigger != nil {
		p := *project
		go func() {
			// Start build first (doesn't block — it has its own polling goroutine)
			if err := h.BuildTrigger.TriggerBuild(context.Background(), p); err != nil {
				log.Printf("projects: failed to trigger initial build for %s: %v", p.ProjectID, err)
			}

			// Provision CDN (blocks until distribution is deployed)
			if h.CDNProvisioner != nil {
				if err := h.CDNProvisioner.ProvisionDistribution(context.Background(), p); err != nil {
					log.Printf("projects: failed to provision CDN for %s: %v", p.ProjectID, err)
				}
			}
		}()
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"project": project,
	})
}

// HandleDelete handles DELETE /api/v1/projects/{project_id}.
func (h *Handler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, errorResponse("unauthorized", "user not authenticated"))
		return
	}

	projectID := chi.URLParam(r, "project_id")
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse("bad_request", "project_id is required"))
		return
	}

	// Verify ownership
	project, err := h.Store.GetByID(r.Context(), projectID)
	if err != nil || project == nil {
		writeJSON(w, http.StatusNotFound, errorResponse("not_found", "project not found"))
		return
	}
	if project.UserID != userID {
		writeJSON(w, http.StatusForbidden, errorResponse("forbidden", "you do not own this project"))
		return
	}

	if err := h.Store.Delete(r.Context(), projectID); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse("internal_error", "failed to delete project"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// errorResponse creates a standard error response map.
func errorResponse(errType, message string) map[string]string {
	return map[string]string{
		"error":   errType,
		"message": message,
	}
}
