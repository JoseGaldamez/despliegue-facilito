package domains

import (
	"encoding/json"
	"net/http"

	"github.com/despliegue-facilito/backend/internal/middleware"
	"github.com/despliegue-facilito/backend/internal/projects"
	"github.com/go-chi/chi/v5"
)

// Handler handles HTTP requests for domain mapping endpoints.
type Handler struct {
	DomainStore  DomainStore
	ProjectStore projects.ProjectStore
}

// AddDomainRequest is the expected JSON body for adding a domain.
type AddDomainRequest struct {
	Hostname string `json:"hostname"`
}

// HandleAdd handles POST /api/v1/projects/{project_id}/domains.
func (h *Handler) HandleAdd(w http.ResponseWriter, r *http.Request) {
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

	// Check project ownership
	project, err := h.ProjectStore.GetByID(r.Context(), projectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse("internal_error", "failed to get project"))
		return
	}
	if project == nil {
		writeJSON(w, http.StatusNotFound, errorResponse("not_found", "project not found"))
		return
	}
	if project.UserID != userID {
		writeJSON(w, http.StatusForbidden, errorResponse("forbidden", "you do not own this project"))
		return
	}

	// Parse request body
	var req AddDomainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse("bad_request", "invalid request body"))
		return
	}
	defer r.Body.Close()

	// Validate hostname
	hostname, valid := ValidateHostname(req.Hostname)
	if !valid {
		writeJSON(w, http.StatusBadRequest, errorResponse("bad_request", "invalid hostname: must be a valid RFC 1123 hostname"))
		return
	}

	// Check for duplicate hostname
	existing, err := h.DomainStore.GetByHostname(r.Context(), hostname)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse("internal_error", "failed to check hostname uniqueness"))
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusConflict, errorResponse("conflict", "domain is already in use"))
		return
	}

	// Add domain
	domain, err := h.DomainStore.Add(r.Context(), projectID, hostname)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse("internal_error", "failed to add domain"))
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"domain": domain,
	})
}

// HandleList handles GET /api/v1/projects/{project_id}/domains.
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
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

	// Check project ownership
	project, err := h.ProjectStore.GetByID(r.Context(), projectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse("internal_error", "failed to get project"))
		return
	}
	if project == nil {
		writeJSON(w, http.StatusNotFound, errorResponse("not_found", "project not found"))
		return
	}
	if project.UserID != userID {
		writeJSON(w, http.StatusForbidden, errorResponse("forbidden", "you do not own this project"))
		return
	}

	// List domains
	domainsList, err := h.DomainStore.ListByProject(r.Context(), projectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse("internal_error", "failed to list domains"))
		return
	}

	if domainsList == nil {
		domainsList = []Domain{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"domains": domainsList,
	})
}

// HandleDelete handles DELETE /api/v1/projects/{project_id}/domains/{domain_id}.
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

	domainID := chi.URLParam(r, "domain_id")
	if domainID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse("bad_request", "domain_id is required"))
		return
	}

	// Check project ownership
	project, err := h.ProjectStore.GetByID(r.Context(), projectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse("internal_error", "failed to get project"))
		return
	}
	if project == nil {
		writeJSON(w, http.StatusNotFound, errorResponse("not_found", "project not found"))
		return
	}
	if project.UserID != userID {
		writeJSON(w, http.StatusForbidden, errorResponse("forbidden", "you do not own this project"))
		return
	}

	// Delete domain
	if err := h.DomainStore.Delete(r.Context(), projectID, domainID); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse("internal_error", "failed to delete domain"))
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
