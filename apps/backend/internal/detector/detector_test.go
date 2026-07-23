package detector

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// helper to create a base64-encoded package.json GitHub API response.
func makeGitHubResponse(t *testing.T, pkg map[string]interface{}) []byte {
	t.Helper()
	pkgBytes, err := json.Marshal(pkg)
	if err != nil {
		t.Fatalf("failed to marshal package.json: %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString(pkgBytes)
	resp := gitHubContentResponse{
		Content:  encoded,
		Encoding: "base64",
	}
	respBytes, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal GitHub response: %v", err)
	}
	return respBytes
}

func TestDetect_Angular(t *testing.T) {
	pkg := map[string]interface{}{
		"name":         "my-angular-app",
		"dependencies": map[string]string{"@angular/core": "^17.0.0"},
	}
	body := makeGitHubResponse(t, pkg)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Authorization header 'Bearer test-token', got %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer server.Close()

	d := &GitHubDetector{
		Client:  server.Client(),
		BaseURL: server.URL,
	}

	fw, err := d.Detect(context.Background(), "https://github.com/owner/my-angular-app", "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fw.Name != "angular" {
		t.Errorf("expected framework 'angular', got %q", fw.Name)
	}
	if fw.BuildCommand != "npm run build" {
		t.Errorf("expected build command 'npm run build', got %q", fw.BuildCommand)
	}
	if fw.OutputDir != "dist/my-angular-app" {
		t.Errorf("expected output dir 'dist/my-angular-app', got %q", fw.OutputDir)
	}
}

func TestDetect_React(t *testing.T) {
	pkg := map[string]interface{}{
		"name":         "my-react-app",
		"dependencies": map[string]string{"react": "^18.2.0", "react-dom": "^18.2.0"},
	}
	body := makeGitHubResponse(t, pkg)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer server.Close()

	d := &GitHubDetector{Client: server.Client(), BaseURL: server.URL}

	fw, err := d.Detect(context.Background(), "https://github.com/owner/my-react-app", "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fw.Name != "react" {
		t.Errorf("expected framework 'react', got %q", fw.Name)
	}
	if fw.BuildCommand != "npm run build" {
		t.Errorf("expected build command 'npm run build', got %q", fw.BuildCommand)
	}
	if fw.OutputDir != "dist" {
		t.Errorf("expected output dir 'dist', got %q", fw.OutputDir)
	}
}

func TestDetect_Vue(t *testing.T) {
	pkg := map[string]interface{}{
		"name":         "my-vue-app",
		"dependencies": map[string]string{"vue": "^3.3.0"},
	}
	body := makeGitHubResponse(t, pkg)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer server.Close()

	d := &GitHubDetector{Client: server.Client(), BaseURL: server.URL}

	fw, err := d.Detect(context.Background(), "https://github.com/owner/my-vue-app", "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fw.Name != "vue" {
		t.Errorf("expected framework 'vue', got %q", fw.Name)
	}
	if fw.BuildCommand != "npm run build" {
		t.Errorf("expected build command 'npm run build', got %q", fw.BuildCommand)
	}
	if fw.OutputDir != "dist" {
		t.Errorf("expected output dir 'dist', got %q", fw.OutputDir)
	}
}

func TestDetect_Astro(t *testing.T) {
	pkg := map[string]interface{}{
		"name":            "my-astro-site",
		"devDependencies": map[string]string{"astro": "^4.0.0"},
	}
	body := makeGitHubResponse(t, pkg)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer server.Close()

	d := &GitHubDetector{Client: server.Client(), BaseURL: server.URL}

	fw, err := d.Detect(context.Background(), "https://github.com/owner/my-astro-site", "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fw.Name != "astro" {
		t.Errorf("expected framework 'astro', got %q", fw.Name)
	}
	if fw.BuildCommand != "npm run build" {
		t.Errorf("expected build command 'npm run build', got %q", fw.BuildCommand)
	}
	if fw.OutputDir != "dist" {
		t.Errorf("expected output dir 'dist', got %q", fw.OutputDir)
	}
}

func TestDetect_PriorityAngularOverReact(t *testing.T) {
	// When both Angular and React are present, Angular wins.
	pkg := map[string]interface{}{
		"name": "mixed-app",
		"dependencies": map[string]string{
			"@angular/core": "^17.0.0",
			"react":         "^18.2.0",
			"vue":           "^3.3.0",
		},
	}
	body := makeGitHubResponse(t, pkg)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer server.Close()

	d := &GitHubDetector{Client: server.Client(), BaseURL: server.URL}

	fw, err := d.Detect(context.Background(), "https://github.com/owner/mixed-app", "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fw.Name != "angular" {
		t.Errorf("expected framework 'angular' (highest priority), got %q", fw.Name)
	}
}

func TestDetect_PriorityReactOverVue(t *testing.T) {
	// When React and Vue are present but not Angular, React wins.
	pkg := map[string]interface{}{
		"name": "react-vue-app",
		"dependencies": map[string]string{
			"react": "^18.2.0",
			"vue":   "^3.3.0",
			"astro": "^4.0.0",
		},
	}
	body := makeGitHubResponse(t, pkg)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer server.Close()

	d := &GitHubDetector{Client: server.Client(), BaseURL: server.URL}

	fw, err := d.Detect(context.Background(), "https://github.com/owner/react-vue-app", "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fw.Name != "react" {
		t.Errorf("expected framework 'react' (higher priority than vue/astro), got %q", fw.Name)
	}
}

func TestDetect_PriorityVueOverAstro(t *testing.T) {
	pkg := map[string]interface{}{
		"name": "vue-astro-app",
		"dependencies": map[string]string{
			"vue":   "^3.3.0",
			"astro": "^4.0.0",
		},
	}
	body := makeGitHubResponse(t, pkg)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer server.Close()

	d := &GitHubDetector{Client: server.Client(), BaseURL: server.URL}

	fw, err := d.Detect(context.Background(), "https://github.com/owner/vue-astro-app", "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fw.Name != "vue" {
		t.Errorf("expected framework 'vue' (higher priority than astro), got %q", fw.Name)
	}
}

func TestDetect_HTMLWhenNoFramework(t *testing.T) {
	// Valid package.json but no recognized framework dependencies.
	pkg := map[string]interface{}{
		"name":         "simple-app",
		"dependencies": map[string]string{"express": "^4.18.0"},
	}
	body := makeGitHubResponse(t, pkg)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer server.Close()

	d := &GitHubDetector{Client: server.Client(), BaseURL: server.URL}

	fw, err := d.Detect(context.Background(), "https://github.com/owner/simple-app", "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fw.Name != "html" {
		t.Errorf("expected framework 'html', got %q", fw.Name)
	}
	if fw.BuildCommand != "" {
		t.Errorf("expected empty build command for html, got %q", fw.BuildCommand)
	}
	if fw.OutputDir != "." {
		t.Errorf("expected output dir '.', got %q", fw.OutputDir)
	}
}

func TestDetect_HTMLWhenMissingPackageJSON(t *testing.T) {
	// GitHub returns 404 when package.json doesn't exist.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message": "Not Found"}`))
	}))
	defer server.Close()

	d := &GitHubDetector{Client: server.Client(), BaseURL: server.URL}

	fw, err := d.Detect(context.Background(), "https://github.com/owner/static-site", "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fw.Name != "html" {
		t.Errorf("expected framework 'html' for missing package.json, got %q", fw.Name)
	}
	if fw.BuildCommand != "" {
		t.Errorf("expected empty build command, got %q", fw.BuildCommand)
	}
	if fw.OutputDir != "." {
		t.Errorf("expected output dir '.', got %q", fw.OutputDir)
	}
}

func TestDetect_HTMLWhenUnparseableJSON(t *testing.T) {
	// GitHub returns 200 but content is not valid JSON.
	invalidJSON := base64.StdEncoding.EncodeToString([]byte(`{not valid json`))
	ghResp := gitHubContentResponse{Content: invalidJSON, Encoding: "base64"}
	body, _ := json.Marshal(ghResp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer server.Close()

	d := &GitHubDetector{Client: server.Client(), BaseURL: server.URL}

	fw, err := d.Detect(context.Background(), "https://github.com/owner/bad-json-repo", "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fw.Name != "html" {
		t.Errorf("expected framework 'html' for unparseable JSON, got %q", fw.Name)
	}
}

func TestDetect_HTMLWhenInvalidBase64(t *testing.T) {
	// GitHub response has invalid base64 in content field.
	ghResp := gitHubContentResponse{Content: "!!!not-base64!!!", Encoding: "base64"}
	body, _ := json.Marshal(ghResp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer server.Close()

	d := &GitHubDetector{Client: server.Client(), BaseURL: server.URL}

	fw, err := d.Detect(context.Background(), "https://github.com/owner/bad-base64", "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fw.Name != "html" {
		t.Errorf("expected framework 'html' for invalid base64, got %q", fw.Name)
	}
}

func TestDetect_DevDependencies(t *testing.T) {
	// Framework detected from devDependencies instead of dependencies.
	pkg := map[string]interface{}{
		"name":            "react-dev",
		"devDependencies": map[string]string{"react": "^18.2.0"},
	}
	body := makeGitHubResponse(t, pkg)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer server.Close()

	d := &GitHubDetector{Client: server.Client(), BaseURL: server.URL}

	fw, err := d.Detect(context.Background(), "https://github.com/owner/react-dev", "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fw.Name != "react" {
		t.Errorf("expected framework 'react' from devDependencies, got %q", fw.Name)
	}
}

func TestDetect_RepoURLWithGitSuffix(t *testing.T) {
	pkg := map[string]interface{}{
		"name":         "my-app",
		"dependencies": map[string]string{"vue": "^3.3.0"},
	}
	body := makeGitHubResponse(t, pkg)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the URL path is correct (no .git in API call).
		expected := "/repos/owner/my-app/contents/package.json"
		if r.URL.Path != expected {
			t.Errorf("expected request path %q, got %q", expected, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer server.Close()

	d := &GitHubDetector{Client: server.Client(), BaseURL: server.URL}

	fw, err := d.Detect(context.Background(), "https://github.com/owner/my-app.git", "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fw.Name != "vue" {
		t.Errorf("expected framework 'vue', got %q", fw.Name)
	}
}

func TestParseRepoURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "standard HTTPS URL",
			url:       "https://github.com/owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:      "URL with .git suffix",
			url:       "https://github.com/owner/repo.git",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:      "URL with trailing slash",
			url:       "https://github.com/owner/repo/",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:    "empty string",
			url:     "",
			wantErr: true,
		},
		{
			name:    "single segment",
			url:     "repo",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := parseRepoURL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if owner != tt.wantOwner {
				t.Errorf("expected owner %q, got %q", tt.wantOwner, owner)
			}
			if repo != tt.wantRepo {
				t.Errorf("expected repo %q, got %q", tt.wantRepo, repo)
			}
		})
	}
}

func TestDetectFromPackageJSON_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		pkgJSON     string
		projectName string
		wantName    string
		wantCmd     string
		wantDir     string
	}{
		{
			name:        "angular in dependencies",
			pkgJSON:     `{"name":"my-app","dependencies":{"@angular/core":"^17.0.0"}}`,
			projectName: "my-app",
			wantName:    "angular",
			wantCmd:     "npm run build",
			wantDir:     "dist/my-app",
		},
		{
			name:        "angular in devDependencies",
			pkgJSON:     `{"name":"ng-app","devDependencies":{"@angular/core":"^16.0.0"}}`,
			projectName: "ng-app",
			wantName:    "angular",
			wantCmd:     "npm run build",
			wantDir:     "dist/ng-app",
		},
		{
			name:        "react in dependencies",
			pkgJSON:     `{"dependencies":{"react":"^18.2.0"}}`,
			projectName: "react-app",
			wantName:    "react",
			wantCmd:     "npm run build",
			wantDir:     "dist",
		},
		{
			name:        "vue in dependencies",
			pkgJSON:     `{"dependencies":{"vue":"^3.3.0"}}`,
			projectName: "vue-app",
			wantName:    "vue",
			wantCmd:     "npm run build",
			wantDir:     "dist",
		},
		{
			name:        "astro in devDependencies",
			pkgJSON:     `{"devDependencies":{"astro":"^4.0.0"}}`,
			projectName: "astro-site",
			wantName:    "astro",
			wantCmd:     "npm run build",
			wantDir:     "dist",
		},
		{
			name:        "no recognized frameworks",
			pkgJSON:     `{"dependencies":{"express":"^4.18.0","lodash":"^4.17.0"}}`,
			projectName: "backend-app",
			wantName:    "html",
			wantCmd:     "",
			wantDir:     ".",
		},
		{
			name:        "empty dependencies",
			pkgJSON:     `{"name":"empty"}`,
			projectName: "empty",
			wantName:    "html",
			wantCmd:     "",
			wantDir:     ".",
		},
		{
			name:        "invalid JSON",
			pkgJSON:     `{not valid`,
			projectName: "broken",
			wantName:    "html",
			wantCmd:     "",
			wantDir:     ".",
		},
		{
			name:        "priority angular over react and vue",
			pkgJSON:     `{"dependencies":{"@angular/core":"^17.0.0","react":"^18.0.0","vue":"^3.0.0"}}`,
			projectName: "multi",
			wantName:    "angular",
			wantCmd:     "npm run build",
			wantDir:     "dist/multi",
		},
		{
			name:        "priority react over vue and astro",
			pkgJSON:     `{"dependencies":{"react":"^18.0.0","vue":"^3.0.0","astro":"^4.0.0"}}`,
			projectName: "multi2",
			wantName:    "react",
			wantCmd:     "npm run build",
			wantDir:     "dist",
		},
		{
			name:        "priority vue over astro",
			pkgJSON:     `{"dependencies":{"vue":"^3.0.0","astro":"^4.0.0"}}`,
			projectName: "multi3",
			wantName:    "vue",
			wantCmd:     "npm run build",
			wantDir:     "dist",
		},
		{
			name:        "angular uses package name for output dir",
			pkgJSON:     `{"name":"@scope/cool-project","dependencies":{"@angular/core":"^17.0.0"}}`,
			projectName: "fallback-name",
			wantName:    "angular",
			wantCmd:     "npm run build",
			wantDir:     "dist/@scope/cool-project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fw, err := DetectFromPackageJSON([]byte(tt.pkgJSON), tt.projectName)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if fw.Name != tt.wantName {
				t.Errorf("Name: expected %q, got %q", tt.wantName, fw.Name)
			}
			if fw.BuildCommand != tt.wantCmd {
				t.Errorf("BuildCommand: expected %q, got %q", tt.wantCmd, fw.BuildCommand)
			}
			if fw.OutputDir != tt.wantDir {
				t.Errorf("OutputDir: expected %q, got %q", tt.wantDir, fw.OutputDir)
			}
		})
	}
}
