package detector

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Framework holds detection result with build configuration.
type Framework struct {
	Name         string // "react", "angular", "vue", "astro", "html"
	BuildCommand string // e.g., "npm run build"
	OutputDir    string // e.g., "dist"
}

// FrameworkDetector identifies the frontend framework from package.json.
type FrameworkDetector interface {
	Detect(ctx context.Context, repoURL string, token string) (Framework, error)
}

// HTTPClient abstracts the HTTP client for testability.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// GitHubDetector implements FrameworkDetector using the GitHub API.
type GitHubDetector struct {
	Client  HTTPClient
	BaseURL string // Override for testing; defaults to "https://api.github.com"
}

// NewGitHubDetector creates a new GitHubDetector with the default HTTP client and GitHub API base URL.
func NewGitHubDetector() *GitHubDetector {
	return &GitHubDetector{
		Client:  http.DefaultClient,
		BaseURL: "https://api.github.com",
	}
}

// Detect fetches the package.json from the given GitHub repository URL,
// parses its dependencies, and returns the detected framework with its build configuration.
func (d *GitHubDetector) Detect(ctx context.Context, repoURL string, token string) (Framework, error) {
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return htmlFramework(), nil
	}

	baseURL := d.BaseURL
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}

	url := fmt.Sprintf("%s/repos/%s/%s/contents/package.json", baseURL, owner, repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return htmlFramework(), fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := d.Client.Do(req)
	if err != nil {
		return htmlFramework(), fmt.Errorf("failed to fetch package.json: %w", err)
	}
	defer resp.Body.Close()

	// If the file doesn't exist (404) or any non-200 response, treat as pure HTML.
	if resp.StatusCode != http.StatusOK {
		return htmlFramework(), nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return htmlFramework(), fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse the GitHub API response to extract the base64-encoded content.
	var ghResp gitHubContentResponse
	if err := json.Unmarshal(body, &ghResp); err != nil {
		return htmlFramework(), nil
	}

	// Decode the base64 content of package.json.
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(ghResp.Content, "\n", ""))
	if err != nil {
		return htmlFramework(), nil
	}

	return DetectFromPackageJSON(decoded, repo)
}

// DetectFromPackageJSON detects the framework from raw package.json bytes.
// Exported for use in property-based tests.
func DetectFromPackageJSON(data []byte, projectName string) (Framework, error) {
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return htmlFramework(), nil
	}

	// If the project has a name in package.json, use it for Angular output dir.
	name := projectName
	if pkg.Name != "" {
		name = pkg.Name
	}

	// Priority order: Angular > React > Vue > Astro
	if hasDependency(pkg, "@angular/core") {
		return Framework{
			Name:         "angular",
			BuildCommand: "npm run build",
			OutputDir:    fmt.Sprintf("dist/%s", name),
		}, nil
	}

	if hasDependency(pkg, "react") {
		return Framework{
			Name:         "react",
			BuildCommand: "npm run build",
			OutputDir:    "dist",
		}, nil
	}

	if hasDependency(pkg, "vue") {
		return Framework{
			Name:         "vue",
			BuildCommand: "npm run build",
			OutputDir:    "dist",
		}, nil
	}

	if hasDependency(pkg, "astro") {
		return Framework{
			Name:         "astro",
			BuildCommand: "npm run build",
			OutputDir:    "dist",
		}, nil
	}

	// No recognized framework found.
	return htmlFramework(), nil
}

// htmlFramework returns the default Framework for pure HTML projects.
func htmlFramework() Framework {
	return Framework{
		Name:         "html",
		BuildCommand: "",
		OutputDir:    ".",
	}
}

// hasDependency checks if a package exists in dependencies or devDependencies.
func hasDependency(pkg packageJSON, name string) bool {
	if _, ok := pkg.Dependencies[name]; ok {
		return true
	}
	if _, ok := pkg.DevDependencies[name]; ok {
		return true
	}
	return false
}

// parseRepoURL extracts the owner and repo name from a GitHub URL.
// Supports formats like:
//   - https://github.com/owner/repo
//   - https://github.com/owner/repo.git
func parseRepoURL(repoURL string) (owner, repo string, err error) {
	// Remove trailing slash and .git suffix.
	repoURL = strings.TrimSuffix(repoURL, "/")
	repoURL = strings.TrimSuffix(repoURL, ".git")

	// Try to parse the URL path.
	parts := strings.Split(repoURL, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid repo URL: %s", repoURL)
	}

	repo = parts[len(parts)-1]
	owner = parts[len(parts)-2]

	if owner == "" || repo == "" {
		return "", "", fmt.Errorf("invalid repo URL: could not extract owner/repo from %s", repoURL)
	}

	return owner, repo, nil
}

// gitHubContentResponse represents the GitHub API response for file contents.
type gitHubContentResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

// packageJSON represents the relevant fields of a package.json file.
type packageJSON struct {
	Name            string            `json:"name"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}
