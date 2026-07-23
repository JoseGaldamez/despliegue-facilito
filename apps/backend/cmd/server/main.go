package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/despliegue-facilito/backend/internal/build"
	"github.com/despliegue-facilito/backend/internal/cdn"
	"github.com/despliegue-facilito/backend/internal/clients"
	"github.com/despliegue-facilito/backend/internal/detector"
	"github.com/despliegue-facilito/backend/internal/domains"
	"github.com/despliegue-facilito/backend/internal/github"
	"github.com/despliegue-facilito/backend/internal/logs"
	"github.com/despliegue-facilito/backend/internal/middleware"
	"github.com/despliegue-facilito/backend/internal/projects"
	"github.com/despliegue-facilito/backend/internal/webhooks"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if present (local development).
	// In Cloud Run, env vars are injected directly.
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Initialize external service clients (Firestore + AWS SDK)
	ctx := context.Background()
	c := clients.Init(ctx)
	defer c.Firestore.Close()

	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check endpoint (public)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Initialize handlers
	projectsHandler := &projects.Handler{
		Store:    projects.NewFirestoreProjectStore(c.Firestore),
		Detector: detector.NewGitHubDetector(),
		TokenFetcher: &projects.FirestoreUserTokenFetcher{
			Client: c.Firestore,
		},
		BuildTrigger: nil, // Will be wired after build service is initialized
	}

	githubHandler := github.NewOAuthHandler(
		c.Firestore,
		os.Getenv("GITHUB_CLIENT_ID"),
		os.Getenv("GITHUB_CLIENT_SECRET"),
	)

	webhookHandler := &webhooks.Handler{
		Secret:       os.Getenv("GITHUB_WEBHOOK_SECRET"),
		Store:        projects.NewFirestoreProjectStore(c.Firestore),
		BuildTrigger: nil, // Will be wired to a real BuildTrigger in task 5.5
	}

	// Initialize build service and log streamer
	projectStore := projects.NewFirestoreProjectStore(c.Firestore)
	buildService := build.NewCodeBuildService(c.CodeBuild, projectStore)
	logsHandler := logs.NewHandler(projectStore, buildService, c.CloudWatchLogs)

	// Wire build trigger to projects handler
	buildTriggerAdapter := &build.BuildTriggerAdapter{
		Service:      buildService,
		TokenFetcher: &projects.FirestoreUserTokenFetcher{Client: c.Firestore},
	}
	projectsHandler.BuildTrigger = buildTriggerAdapter

	// Wire CDN provisioner
	cdnService := cdn.NewService(c.CloudFront, projectStore)
	projectsHandler.CDNProvisioner = cdnService

	// Initialize domain handler
	domainsHandler := &domains.Handler{
		DomainStore:  domains.NewFirestoreDomainStore(c.Firestore),
		ProjectStore: projectStore,
	}

	// Public webhook route (validated via HMAC signature, not auth middleware)
	r.Post("/api/v1/webhooks/github", webhookHandler.HandleWebhook)

	// Protected API routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.Auth(c.FirebaseAuth))
		r.Get("/projects", projectsHandler.HandleList)
		r.Post("/projects", projectsHandler.HandleCreate)
		r.Get("/projects/{project_id}", projectsHandler.HandleGet)
		r.Delete("/projects/{project_id}", projectsHandler.HandleDelete)
		r.Post("/github/connect", githubHandler.HandleConnect)
		r.Get("/github/status", githubHandler.HandleStatus)
		r.Get("/github/repos", githubHandler.HandleRepos)
		r.Get("/projects/{project_id}/logs", logsHandler.HandleLogStream)
		r.Post("/projects/{project_id}/domains", domainsHandler.HandleAdd)
		r.Get("/projects/{project_id}/domains", domainsHandler.HandleList)
		r.Delete("/projects/{project_id}/domains/{domain_id}", domainsHandler.HandleDelete)
	})

	log.Printf("Backend service starting on port %s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
