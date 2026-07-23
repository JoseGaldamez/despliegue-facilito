package webhooks

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/despliegue-facilito/backend/internal/projects"
)

// BuildTrigger is the interface for triggering builds on valid push events.
type BuildTrigger interface {
	TriggerBuild(ctx context.Context, project projects.Project) error
}

// Handler handles incoming GitHub webhook requests.
type Handler struct {
	Secret       string
	Store        projects.ProjectStore
	BuildTrigger BuildTrigger
}

// GitHubPushEvent represents the relevant fields from a GitHub push event payload.
type GitHubPushEvent struct {
	Ref        string           `json:"ref"`
	Repository GitHubRepository `json:"repository"`
}

// GitHubRepository represents the repository fields in a GitHub push event.
type GitHubRepository struct {
	HTMLURL  string `json:"html_url"`
	CloneURL string `json:"clone_url"`
}

// maxPayloadSize is the maximum allowed webhook payload size (10 MB).
const maxPayloadSize = 10 * 1024 * 1024

// HandleWebhook processes incoming GitHub webhook requests.
// It validates the HMAC signature, parses the payload, filters non-push/non-main events,
// looks up the project by repo URL, and triggers a build if everything checks out.
func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	// Limit request body to 10 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxPayloadSize)

	// Read the raw body for signature verification
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return
	}

	// Validate X-Hub-Signature-256 header
	signature := r.Header.Get("X-Hub-Signature-256")
	if signature == "" {
		http.Error(w, "missing signature", http.StatusUnauthorized)
		return
	}

	if !h.verifySignature(body, signature) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	// Parse the JSON payload
	var payload GitHubPushEvent
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Check X-GitHub-Event header — only process "push" events
	eventType := r.Header.Get("X-GitHub-Event")
	if eventType != "push" {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "event ignored, not a push event",
		})
		return
	}

	// Check if the push is to the main branch
	if payload.Ref != "refs/heads/main" {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "push ignored, not targeting main branch",
		})
		return
	}

	// Extract repo URL — try html_url first, fallback to clone_url
	repoURL := payload.Repository.HTMLURL
	if repoURL == "" {
		repoURL = payload.Repository.CloneURL
	}

	// Look up the project by repo URL
	project, err := h.Store.GetByRepoURL(r.Context(), repoURL)
	if err != nil {
		log.Printf("webhook: error looking up project by repo URL %q: %v", repoURL, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if project == nil {
		// Try clone_url as fallback if html_url didn't match
		if payload.Repository.CloneURL != "" && payload.Repository.CloneURL != repoURL {
			project, err = h.Store.GetByRepoURL(r.Context(), payload.Repository.CloneURL)
			if err != nil {
				log.Printf("webhook: error looking up project by clone URL %q: %v", payload.Repository.CloneURL, err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
		}
	}
	if project == nil {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}

	// Trigger build if BuildTrigger is available
	if h.BuildTrigger != nil {
		if err := h.BuildTrigger.TriggerBuild(r.Context(), *project); err != nil {
			log.Printf("webhook: error triggering build for project %s: %v", project.ProjectID, err)
			http.Error(w, "failed to trigger build", http.StatusInternalServerError)
			return
		}
	} else {
		log.Printf("webhook: build trigger not configured, acknowledging push for project %s", project.ProjectID)
	}

	// Respond 202 Accepted
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"message":    "build triggered",
		"project_id": project.ProjectID,
	})
}

// verifySignature validates the HMAC-SHA256 signature of the payload.
// The expected format is "sha256=<hex-encoded-signature>".
func (h *Handler) verifySignature(payload []byte, signature string) bool {
	parts := strings.SplitN(signature, "=", 2)
	if len(parts) != 2 || parts[0] != "sha256" {
		return false
	}

	expectedMAC, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(h.Secret))
	mac.Write(payload)
	computedMAC := mac.Sum(nil)

	return hmac.Equal(computedMAC, expectedMAC)
}

// ComputeSignature computes the HMAC-SHA256 signature for a payload with the given secret.
// Returns the signature in the format "sha256=<hex>". Useful for testing.
func ComputeSignature(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return fmt.Sprintf("sha256=%s", hex.EncodeToString(mac.Sum(nil)))
}
