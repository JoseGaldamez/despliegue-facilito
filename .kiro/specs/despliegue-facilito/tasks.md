# Implementation Plan: Despliegue Facilito

## Overview

This plan implements a developer PaaS for automated static/frontend deployments on AWS. The project uses a simple multi-app repository structure without monorepo tooling — each app (backend, frontend, landing) is independent with its own dependencies and build process. A root-level script coordinates local development.

## Tasks

- [x] 1. Set up project structure
  - [x] 1.1 Create root-level project files
    - Create root `README.md` with project overview and quick start instructions
    - Create root `.gitignore` covering node_modules, build outputs, env files, credentials, OS/IDE files
    - Create `dev.ps1` script to start all services (backend, frontend, landing) concurrently in local
    - _Requirements: 1.1, 1.4, 1.5, 1.6_

- [x] 2. Implement Backend Service core setup
  - [x] 2.1 Scaffold Go backend service (`apps/backend`)
    - Initialize Go module with `go mod init`
    - Set up Chi router with middleware (CORS, logging, recovery)
    - Implement health check endpoint `GET /health` returning 200
    - Load `PORT` env var with default `8080`
    - Create `Dockerfile` for Cloud Run deployment
    - Create `apps/backend/package.json` with build/dev/test scripts
    - _Requirements: 12.1, 12.4, 12.5, 12.6_

  - [x] 2.2 Initialize Firestore and AWS SDK clients
    - Initialize Firestore client using `firebase.google.com/go/v4` on startup
    - Initialize AWS SDK v2 clients for CodeBuild, S3, and CloudWatch Logs
    - Implement graceful startup failure: log error and exit(1) if Firestore or AWS init fails
    - _Requirements: 12.2, 12.3, 12.7, 12.8_

  - [x] 2.3 Implement Firebase Auth middleware for token verification
    - Create middleware that extracts and verifies Firebase ID tokens from `Authorization` header
    - Return HTTP 401 for missing or invalid tokens
    - Attach user UID to request context on success
    - _Requirements: 5.5, 11.7_

- [x] 3. Implement Backend Service - Projects and GitHub
  - [x] 3.1 Implement ProjectStore (Firestore persistence)
    - Implement `Create`, `ListByUser`, `GetByRepoURL`, `UpdateStatus` methods
    - Store project records with all fields: projectId, userId, repoUrl, framework, buildCommand, outputDir, deploymentStatus, createdAt, updatedAt
    - _Requirements: 5.2, 5.3, 5.6_

  - [x] 3.2 Implement Framework Detector
    - Fetch `package.json` from GitHub repo using access token
    - Implement detection logic with priority: Angular > React > Vue > Astro > HTML
    - Assign build command and output directory per framework
    - Handle missing/unparseable `package.json` as pure HTML
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 6.7, 6.8, 6.9_

  - [ ]* 3.3 Write property test for Framework Detection (Property 1)
    - **Property 1: Framework detection correctness**
    - Generate random `package.json` objects with arbitrary deps including/excluding known frameworks
    - Verify highest-priority framework is selected or "html" for no match
    - Use `pgregory.net/rapid` with minimum 100 iterations
    - **Validates: Requirements 6.2, 6.3, 6.4, 6.5, 6.6, 6.7, 6.8**

  - [x] 3.4 Implement Projects REST endpoints
    - `GET /api/v1/projects` — list user projects (auth required)
    - `POST /api/v1/projects` — create project with repo URL (auth required), trigger framework detection, reject duplicate repos with HTTP 409
    - Wire routes through Chi router with auth middleware
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6_

  - [x] 3.5 Implement GitHub OAuth connection
    - `POST /api/v1/github/connect` — exchange OAuth code for access token
    - Store GitHub access token and username in Firestore user record
    - Handle token replacement for re-connection
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5_

- [x] 4. Checkpoint - Backend core
  - Ensure all tests pass, ask the user if questions arise.

- [x] 5. Implement Backend Service - Webhooks and Build Pipeline
  - [x] 5.1 Implement Webhook Handler
    - `POST /api/v1/webhooks/github` endpoint accepting up to 10 MB payloads
    - Validate `X-Hub-Signature-256` header using HMAC-SHA256 with configured secret
    - Parse payload, reject invalid JSON with HTTP 400
    - Filter non-push or non-main-branch events with HTTP 200 (no build triggered)
    - Look up project by repo URL, return HTTP 404 if not found
    - Trigger build pipeline on valid push to main, respond with HTTP 202
    - _Requirements: 9.1, 9.2, 9.3, 9.4, 9.5, 9.6, 9.7, 9.8_

  - [ ]* 5.2 Write property test for Webhook Signature Validation (Property 3)
    - **Property 3: Webhook signature validation**
    - Generate random byte payloads and secrets, compute valid/invalid HMAC
    - Verify acceptance only when signature matches
    - Use `pgregory.net/rapid` with minimum 100 iterations
    - **Validates: Requirements 9.2, 9.3**

  - [ ]* 5.3 Write property test for Webhook Invalid Payload Rejection (Property 4)
    - **Property 4: Webhook invalid payload rejection**
    - Generate random non-JSON strings, truncated JSON, and JSON missing required fields
    - Verify HTTP 400 response without triggering any build
    - Use `pgregory.net/rapid` with minimum 100 iterations
    - **Validates: Requirements 9.4**

  - [ ]* 5.4 Write property test for Webhook Non-Target Event Filtering (Property 5)
    - **Property 5: Webhook non-target event filtering**
    - Generate random GitHub event types and branch names excluding "push" on "main"
    - Verify HTTP 200 response and no build triggered
    - Use `pgregory.net/rapid` with minimum 100 iterations
    - **Validates: Requirements 9.5**

  - [x] 5.5 Implement Build Service (CodeBuild integration)
    - Implement `TriggerBuild` — create CodeBuild job with env vars (repo URL, build command, output dir, GitHub token, S3 path)
    - Implement `GetBuildStatus` — poll CodeBuild for job status
    - Update project status to "building" before starting, "deployed" on success, "failed" on failure with reason
    - Implement build queue: reject concurrent builds for same project, queue and execute sequentially
    - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5, 7.6, 7.7_

  - [ ]* 5.6 Write property test for Build Queue Sequential Execution (Property 6)
    - **Property 6: Build queue sequential execution**
    - Generate random sequences of 2–10 build triggers with random timing
    - Verify no two builds for the same project have overlapping execution windows
    - Use `pgregory.net/rapid` with minimum 100 iterations
    - **Validates: Requirements 7.7**

- [x] 6. Implement Backend Service - Log Streaming and Domains
  - [x] 6.1 Implement Log Streamer (CloudWatch + WebSocket)
    - Implement WebSocket endpoint `GET /api/v1/projects/{project_id}/logs`
    - Poll CloudWatch Logs every 2 seconds for new log chunks
    - Push log chunks to connected WebSocket client within 3 seconds
    - Send completion message and close connection when build finishes
    - Send error message on CloudWatch retrieval failure, retry on next cycle
    - _Requirements: 10.1, 10.2, 10.3, 10.4, 10.6_

  - [x] 6.2 Implement Domain Store and REST endpoints
    - Implement `DomainStore`: `Add`, `ListByProject`, `Delete`, `GetByHostname`
    - `POST /api/v1/projects/{project_id}/domains` — validate hostname (RFC 1123), reject duplicates with HTTP 409
    - `GET /api/v1/projects/{project_id}/domains` — list domains for project
    - `DELETE /api/v1/projects/{project_id}/domains/{domain_id}` — remove mapping
    - Enforce ownership: return HTTP 403 if user doesn't own the project
    - _Requirements: 11.1, 11.2, 11.3, 11.4, 11.6, 11.7, 11.8_

  - [ ]* 6.3 Write property test for Hostname Validation (Property 8)
    - **Property 8: Hostname validation**
    - Generate random strings mixing valid/invalid hostname characters
    - Verify only RFC 1123 conforming hostnames are accepted (labels 1–63 chars, total ≤ 253, no leading/trailing hyphens)
    - Use `pgregory.net/rapid` with minimum 100 iterations
    - **Validates: Requirements 11.1**

- [x] 7. Checkpoint - Backend complete
  - Ensure all tests pass, ask the user if questions arise.

- [x] 8. Implement Dashboard App
  - [x] 8.1 Scaffold React + Vite + Tailwind project (`apps/frontend`)
    - Initialize Vite project with React and TypeScript template
    - Configure Tailwind CSS
    - Create `apps/frontend/package.json` with own dependencies (no workspace references)
    - Set up routing (React Router)
    - Create `src/types/api.ts` with entity types (User, Project, Build, Domain)
    - Create `src/types/dtos.ts` with API request/response DTOs
    - Create `src/types/ws-messages.ts` with WebSocket message types
    - _Requirements: 3.1_

  - [x] 8.2 Implement Firebase Auth integration
    - Create `AuthContext` provider with Firebase Auth state and session persistence
    - Implement `ProtectedRoute` component redirecting unauthenticated users to login
    - Implement login page with Google OAuth and Email/Password forms
    - Implement sign-up page with Email/Password
    - Implement sign-out functionality clearing session and redirecting to login
    - Display error messages on auth failure (invalid credentials, connectivity issues)
    - _Requirements: 3.2, 3.3, 3.4, 3.5, 3.6, 3.7, 3.8, 3.9_

  - [x] 8.3 Implement API client and Projects page
    - Create API client implementing all endpoint calls with auth token injection using types from `src/types/`
    - Implement Projects list page showing project_id, repo URL, framework, status
    - Implement empty state when no projects exist
    - Implement create project flow (repo URL input)
    - _Requirements: 5.1, 5.7_

  - [x] 8.4 Implement GitHub connection UI
    - Create `GitHubConnect` component initiating OAuth flow
    - Display connected GitHub username when linked
    - Handle OAuth failure with error message and pre-connection state
    - _Requirements: 4.1, 4.3, 4.4_

  - [x] 8.5 Implement Build Logs WebSocket client
    - Create `BuildLogs` component connecting to WebSocket endpoint
    - Render log chunks in chronological order using types from `src/types/ws-messages.ts`
    - Implement exponential backoff reconnection (1s, 2s, 4s, 8s, 16s — max 5 attempts)
    - Display "connection lost" banner after max retries with manual retry button
    - Request logs from last received timestamp on reconnect
    - _Requirements: 10.2, 10.3, 10.4, 10.5_

  - [ ]* 8.6 Write property test for WebSocket Reconnection Backoff (Property 7)
    - **Property 7: WebSocket reconnection exponential backoff**
    - Generate random sequences of 1–5 disconnection events
    - Verify delay follows `2^(K-1)` seconds pattern and max 5 attempts
    - Use `fast-check` with minimum 100 iterations
    - **Validates: Requirements 10.5**

  - [x] 8.7 Implement Domain Management UI
    - Create `DomainManager` component for adding/listing/deleting custom domains
    - Display DNS configuration instructions (CNAME → CloudFront distribution) on add
    - _Requirements: 11.2, 11.3, 11.4, 11.5_

  - [x] 8.8 Implement main dashboard layout
    - Create layout shell with navigation sidebar, top header bar with user display name, and content area
    - Wire all pages into router with protected routes
    - _Requirements: 3.6_

- [~] 9. Checkpoint - Dashboard complete
  - Ensure all tests pass, ask the user if questions arise.

- [x] 10. Implement Landing App
  - [x] 10.1 Scaffold Astro SSG project (`apps/landing`)
    - Initialize Astro project with SSG output configuration
    - Configure Tailwind CSS integration
    - Create `apps/landing/package.json` with own dependencies (no workspace references)
    - _Requirements: 2.1_

  - [x] 10.2 Implement landing page content
    - Create hero section with headline, subheadline, and CTA button linking to Dashboard signup
    - Create features section listing at least three platform capabilities
    - Add navigation links to Dashboard login and signup
    - Ensure responsive design for 320px–1920px viewports
    - _Requirements: 2.2, 2.3, 2.4, 2.5, 2.6_

- [ ] 11. Implement Infrastructure Components
  - [~] 11.1 Create CloudFront Function (Wildcard Router)
    - Implement JavaScript function parsing host header to extract project_id
    - Rewrite `/` to `/apps/{project_id}/index.html`
    - Rewrite other paths to `/apps/{project_id}/{path}` preserving query strings
    - Pass through bare domain requests unchanged
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5_

  - [ ]* 11.2 Write property test for Wildcard Routing (Property 2)
    - **Property 2: Wildcard routing URL rewrite**
    - Generate random project_id strings, URL paths, and query strings
    - Verify correct rewriting for subdomain requests and pass-through for bare domain
    - Use `pgregory.net/rapid` or `fast-check` with minimum 100 iterations
    - **Validates: Requirements 8.3, 8.4, 8.5**

  - [~] 11.3 Create CodeBuild buildspec.yml template
    - Define buildspec with Node.js 20 runtime
    - Clone repo, run `npm ci`, execute build command via env var
    - Sync output directory to S3 path `s3://despliegue-facilito-apps/$PROJECT_ID/`
    - _Requirements: 7.2, 7.3, 7.6_

- [ ] 12. Integration and final verification
  - [~] 12.1 Verify all apps build independently
    - Verify backend builds with `go build ./...`
    - Verify frontend builds with `pnpm build`
    - Verify landing builds with `pnpm build`
    - Verify `dev.ps1` starts all services concurrently

  - [ ]* 12.2 Write integration tests for Backend API endpoints
    - Test project CRUD flows with mocked Firestore
    - Test webhook endpoint with valid/invalid signatures
    - Test domain management with ownership enforcement
    - Test auth middleware rejection of unauthenticated requests
    - _Requirements: 5.3, 5.4, 5.5, 9.1, 11.7, 11.8_

- [~] 13. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties from the design document using `pgregory.net/rapid` (Go) and `fast-check` (TypeScript)
- The Go backend uses Chi router, Firebase Admin SDK for auth verification, and AWS SDK v2
- The frontend uses Firebase SDK for auth and connects to backend via REST + WebSocket
- API types are defined directly in `apps/frontend/src/types/` — no shared package
- Each app manages its own dependencies independently (no workspace/monorepo tooling)
- The root `dev.ps1` script coordinates local development by starting all services in parallel

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "2.1"] },
    { "id": 1, "tasks": ["2.2"] },
    { "id": 2, "tasks": ["2.3", "3.1"] },
    { "id": 3, "tasks": ["3.2", "3.5"] },
    { "id": 4, "tasks": ["3.3", "3.4"] },
    { "id": 5, "tasks": ["5.1", "5.5"] },
    { "id": 6, "tasks": ["5.2", "5.3", "5.4", "5.6"] },
    { "id": 7, "tasks": ["6.1", "6.2"] },
    { "id": 8, "tasks": ["6.3"] },
    { "id": 9, "tasks": ["8.1", "10.1"] },
    { "id": 10, "tasks": ["8.2", "8.3", "10.2", "11.1", "11.3"] },
    { "id": 11, "tasks": ["8.4", "8.5", "8.7", "8.8", "11.2"] },
    { "id": 12, "tasks": ["8.6"] },
    { "id": 13, "tasks": ["12.1"] },
    { "id": 14, "tasks": ["12.2"] }
  ]
}
```
