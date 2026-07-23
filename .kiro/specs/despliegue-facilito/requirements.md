# Requirements Document

## Introduction

Despliegue Facilito is a developer PaaS for automated static and frontend application deployments on AWS. The platform provides a monorepo-based architecture with a landing page, a React SPA dashboard, and a Go backend service. It enables developers to connect their GitHub accounts, detect frontend frameworks, trigger AWS CodeBuild jobs, and serve deployed applications via wildcard subdomains on CloudFront. Real-time build logs are streamed to the dashboard via WebSocket.

## Glossary

- **Monorepo**: A single repository containing multiple applications and shared packages managed with pnpm workspaces and Turborepo.
- **Landing_App**: The Astro SSG website (`apps/landing`) that explains the platform and links to login/signup. Deployed on AWS S3 + CloudFront.
- **Dashboard_App**: The React SPA (`apps/frontend`) built with Vite and Tailwind CSS. Provides authenticated access to project management, build logs, and domain mappings.
- **Backend_Service**: The Go REST service (`apps/backend`) deployed on Google Cloud Run. Manages Firestore data, triggers AWS CodeBuild, and streams CloudWatch Logs.
- **Shared_Package**: The shared TypeScript package (`packages/shared`) containing API DTO definitions and shared types.
- **Framework_Detector**: The component within the Backend_Service responsible for identifying the frontend framework (React, Angular, Vue, Astro, or pure HTML) of a connected project.
- **Build_Pipeline**: The AWS CodeBuild-based pipeline that compiles and deploys frontend application artifacts to S3.
- **Wildcard_Router**: The CloudFront Function that routes requests from `{project_id}.despliegue-facilito.app` to the corresponding S3 path `/apps/{project_id}/index.html`.
- **Log_Streamer**: The component within the Backend_Service that fetches build log chunks from CloudWatch Logs and streams them to the Dashboard_App via WebSocket.
- **Firebase_Auth**: The Firebase Authentication service used by Dashboard_App supporting Google OAuth and Email/Password sign-in methods.
- **Project**: A user-linked frontend application deployment unit identified by a unique `project_id`.
- **Webhook_Handler**: The endpoint in Backend_Service that receives GitHub `push` events on the `main` branch and triggers the Build_Pipeline.

## Requirements

### Requirement 1: Monorepo Structure

**User Story:** As a developer, I want the project structured as a pnpm monorepo with Turborepo, so that all applications and shared packages are managed in a single repository with efficient builds.

#### Acceptance Criteria

1. THE Monorepo SHALL contain a root `package.json` with a `packageManager` field specifying pnpm and a `workspaces` or equivalent pnpm workspace reference.
2. THE Monorepo SHALL contain a `pnpm-workspace.yaml` file declaring `apps/*` and `packages/*` as workspace members.
3. THE Monorepo SHALL contain a `turbo.json` configuration file at the repository root defining `build`, `dev`, and `lint` pipeline tasks, where the `build` task declares topological dependencies so that Shared_Package builds before dependent applications.
4. THE Monorepo SHALL contain the Landing_App at the path `apps/landing`.
5. THE Monorepo SHALL contain the Dashboard_App at the path `apps/frontend`.
6. THE Monorepo SHALL contain the Backend_Service at the path `apps/backend`.
7. THE Monorepo SHALL contain the Shared_Package at the path `packages/shared`.
8. Each workspace member declared in the Monorepo SHALL contain its own `package.json` with a `name` field using the `@despliegue-facilito` scope prefix.

### Requirement 2: Landing Page Application

**User Story:** As a visitor, I want to see a landing page explaining the platform with clear calls to action, so that I understand the value of Despliegue Facilito and can navigate to sign up or log in.

#### Acceptance Criteria

1. THE Landing_App SHALL be built using Astro with static site generation (SSG) output.
2. THE Landing_App SHALL include navigation links to the Dashboard_App login and signup pages.
3. THE Landing_App SHALL be deployable to AWS S3 with CloudFront distribution for content delivery.
4. THE Landing_App SHALL include a hero section containing a headline describing the platform purpose, a subheadline explaining the key benefit, and a primary CTA button linking to the Dashboard_App signup page.
5. THE Landing_App SHALL render correctly on viewports from 320px to 1920px width.
6. THE Landing_App SHALL include a features section listing at least three platform capabilities with titles and descriptions.

### Requirement 3: Dashboard Application Setup

**User Story:** As a developer, I want a React SPA dashboard with authentication, so that I can securely manage my deployment projects.

#### Acceptance Criteria

1. THE Dashboard_App SHALL be built using React with Vite as the build tool and Tailwind CSS for styling.
2. THE Dashboard_App SHALL integrate Firebase_Auth for user authentication with session persistence across page refreshes.
3. THE Dashboard_App SHALL support Google OAuth sign-in via Firebase_Auth.
4. THE Dashboard_App SHALL support Email and Password sign-in and sign-up via Firebase_Auth.
5. WHEN an unauthenticated user attempts to access any protected route of the Dashboard_App, THE Dashboard_App SHALL redirect the user to the login page.
6. WHEN a user successfully authenticates, THE Dashboard_App SHALL display the main dashboard layout shell containing a navigation sidebar, a top header bar with the authenticated user's display name, and a content area.
7. IF a user submits invalid credentials or an authentication attempt fails, THEN THE Dashboard_App SHALL display an error message indicating the reason for failure and remain on the login page.
8. WHEN an authenticated user requests to sign out, THE Dashboard_App SHALL clear the authentication session and redirect the user to the login page.
9. IF the Firebase_Auth service is unreachable during an authentication attempt, THEN THE Dashboard_App SHALL display an error message indicating a connectivity problem and allow the user to retry.

### Requirement 4: GitHub Account Connection

**User Story:** As a developer, I want to connect my GitHub account to the platform, so that the system can access my repositories for deployment.

#### Acceptance Criteria

1. WHEN an authenticated user requests to connect a GitHub account, THE Dashboard_App SHALL initiate the GitHub OAuth authorization flow requesting repository read-access scope.
2. WHEN GitHub OAuth authorization completes successfully, THE Backend_Service SHALL store the GitHub access token associated with the user in Firestore and return the connected GitHub username to the Dashboard_App.
3. IF the user has a connected GitHub account, THEN THE Dashboard_App SHALL display the connected GitHub username on the dashboard.
4. IF GitHub OAuth authorization fails or the user denies the authorization request, THEN THE Dashboard_App SHALL display an error message indicating the reason for the failure and return the user to the pre-connection state without storing any token.
5. IF the user already has a connected GitHub account and requests to connect again, THEN THE Backend_Service SHALL replace the previously stored access token with the new one and THE Dashboard_App SHALL display the updated GitHub username.

### Requirement 5: Project Listing and Management

**User Story:** As a developer, I want to list and manage my deployment projects, so that I can track and control my deployed applications.

#### Acceptance Criteria

1. WHEN an authenticated user navigates to the projects section, THE Dashboard_App SHALL display a list of all Projects belonging to that user, showing for each Project the project_id, repository URL, detected framework, and deployment status.
2. WHEN a user creates a new Project by selecting a GitHub repository, THE Backend_Service SHALL store the Project record in Firestore with the repository URL, detected framework, a unique `project_id`, and an initial deployment status of "pending".
3. THE Backend_Service SHALL expose a REST endpoint `GET /api/v1/projects` that returns all Projects for the authenticated user, including for each Project the project_id, repository URL, detected framework, and deployment status.
4. THE Backend_Service SHALL expose a REST endpoint `POST /api/v1/projects` that accepts a repository URL and creates a new Project for the authenticated user.
5. IF an unauthenticated request is made to any Projects endpoint, THEN THE Backend_Service SHALL reject the request with HTTP status 401 without processing it.
6. IF a user attempts to create a Project with a repository URL that is already associated with an existing Project for that user, THEN THE Backend_Service SHALL reject the request with an error message indicating the repository is already linked to a project.
7. IF an authenticated user has no Projects, THEN THE Dashboard_App SHALL display an empty state message prompting the user to create their first project.

### Requirement 6: Framework Detection

**User Story:** As a developer, I want the system to automatically detect the frontend framework of my project, so that it uses the correct build configuration without manual setup.

#### Acceptance Criteria

1. WHEN a new Project is created, THE Framework_Detector SHALL read the `package.json` file at the root of the connected GitHub repository to identify the frontend framework.
2. THE Framework_Detector SHALL detect React projects by the presence of `react` in the `dependencies` or `devDependencies` fields of `package.json`.
3. THE Framework_Detector SHALL detect Angular projects by the presence of `@angular/core` in the `dependencies` or `devDependencies` fields of `package.json`.
4. THE Framework_Detector SHALL detect Vue projects by the presence of `vue` in the `dependencies` or `devDependencies` fields of `package.json`.
5. THE Framework_Detector SHALL detect Astro projects by the presence of `astro` in the `dependencies` or `devDependencies` fields of `package.json`.
6. IF multiple recognized framework dependencies are found, THEN THE Framework_Detector SHALL select the framework with the highest priority in this order: Angular, React, Vue, Astro.
7. IF no `package.json` file exists at the repository root or the file cannot be parsed, THEN THE Framework_Detector SHALL classify the project as pure HTML.
8. IF no recognized framework dependency is found in a valid `package.json`, THEN THE Framework_Detector SHALL classify the project as pure HTML.
9. WHEN detection completes, THE Framework_Detector SHALL assign the build command and output directory according to the detected framework: React (`npm run build`, `dist`), Angular (`npm run build`, `dist/{project-name}`), Vue (`npm run build`, `dist`), Astro (`npm run build`, `dist`), pure HTML (no build command, `.`).

### Requirement 7: Build Pipeline Execution

**User Story:** As a developer, I want my project to be automatically built and deployed to S3, so that my application is available on the platform after every push.

#### Acceptance Criteria

1. WHEN a build is triggered for a Project, THE Backend_Service SHALL update the Project deployment status to "building" in Firestore before starting the CodeBuild job.
2. WHEN a build is triggered for a Project, THE Build_Pipeline SHALL create an AWS CodeBuild job with environment variables containing the repository URL, build command, target S3 path, and the GitHub access token for repository access.
3. THE Build_Pipeline SHALL upload build artifacts to the S3 path `s3://despliegue-facilito-apps/{project_id}/`.
4. WHEN a CodeBuild job completes successfully, THE Backend_Service SHALL update the Project deployment status to "deployed" in Firestore.
5. IF a CodeBuild job fails, THEN THE Backend_Service SHALL update the Project deployment status to "failed" in Firestore and store the failure reason.
6. THE Build_Pipeline SHALL use the build command and output directory determined by the Framework_Detector.
7. IF a build is triggered for a Project that already has a build in progress, THEN THE Backend_Service SHALL queue the new build and execute it after the current build completes.

### Requirement 8: Wildcard Subdomain Routing

**User Story:** As a developer, I want my deployed application accessible via a unique subdomain, so that each project has its own URL without manual DNS configuration.

#### Acceptance Criteria

1. THE Wildcard_Router SHALL be implemented as a CloudFront Function attached to a single CloudFront Distribution.
2. THE Wildcard_Router SHALL be associated with the wildcard domain `*.despliegue-facilito.app`.
3. WHEN a request arrives with host header `{project_id}.despliegue-facilito.app` and path `/`, THE Wildcard_Router SHALL rewrite the origin request path to `/apps/{project_id}/index.html`.
4. WHEN a request arrives with host header `{project_id}.despliegue-facilito.app` and a path other than `/`, THE Wildcard_Router SHALL rewrite the origin request path to `/apps/{project_id}/{requested_path}` stripping the leading slash from requested_path and preserving query string parameters.
5. WHEN a request arrives with the bare domain `despliegue-facilito.app` (no subdomain), THE Wildcard_Router SHALL pass the request through to the origin without path rewriting.
6. IF the requested S3 object does not exist, THEN CloudFront SHALL return an HTTP 404 response to the client.

### Requirement 9: GitHub Webhook Integration

**User Story:** As a developer, I want automatic deployments triggered by pushes to the main branch, so that my application stays up to date without manual intervention.

#### Acceptance Criteria

1. THE Backend_Service SHALL expose a REST endpoint `POST /api/v1/webhooks/github` that accepts GitHub webhook payloads up to 10 MB in size.
2. WHEN a webhook request is received, THE Webhook_Handler SHALL validate the request signature using the GitHub webhook secret before processing the payload, and respond within 10 seconds.
3. IF the webhook request signature is missing or invalid, THEN THE Webhook_Handler SHALL reject the request with HTTP status 401 without further processing.
4. IF the webhook payload cannot be parsed as valid JSON or does not conform to the expected GitHub event structure, THEN THE Webhook_Handler SHALL respond with HTTP status 400.
5. IF the webhook payload is not a `push` event on `main`, THEN THE Webhook_Handler SHALL respond with HTTP status 200 without triggering a build.
6. WHEN a `push` event is received for the `main` branch, THE Webhook_Handler SHALL identify the corresponding Project in Firestore by repository URL.
7. IF the webhook payload does not correspond to a registered Project, THEN THE Webhook_Handler SHALL respond with HTTP status 404.
8. WHEN the corresponding Project is identified from a valid `push` event on `main`, THE Webhook_Handler SHALL trigger the Build_Pipeline for that Project and respond with HTTP status 202.

### Requirement 10: Real-Time Build Log Streaming

**User Story:** As a developer, I want to see real-time build logs in the dashboard during deployment, so that I can monitor build progress and debug failures.

#### Acceptance Criteria

1. WHILE a CodeBuild job is in progress, THE Log_Streamer SHALL poll AWS CloudWatch Logs for new log chunks every 2 seconds and deliver them to connected clients in chronological order.
2. THE Backend_Service SHALL expose a WebSocket endpoint that the Dashboard_App connects to for receiving log chunks.
3. WHEN new log chunks are available, THE Log_Streamer SHALL push the log chunks to the connected Dashboard_App via the WebSocket connection within 3 seconds of retrieval.
4. WHEN the CodeBuild job completes, THE Log_Streamer SHALL send a completion message and close the WebSocket connection.
5. IF the WebSocket connection is interrupted, THEN THE Dashboard_App SHALL attempt to reconnect using exponential backoff starting at 1 second, up to a maximum of 5 attempts, and request logs from the last received timestamp.
6. IF the Log_Streamer fails to retrieve logs from AWS CloudWatch Logs, THEN THE Log_Streamer SHALL send an error message to the connected Dashboard_App indicating the retrieval failure and retry on the next polling cycle.

### Requirement 11: Domain Mapping Management

**User Story:** As a developer, I want to manage custom domain mappings for my projects, so that I can serve my application from my own domain in addition to the platform subdomain.

#### Acceptance Criteria

1. WHEN an authenticated user adds a custom domain to a Project, THE Backend_Service SHALL validate that the domain is a well-formed hostname and store the domain mapping in Firestore with a unique `domain_id`.
2. THE Backend_Service SHALL expose a REST endpoint `POST /api/v1/projects/{project_id}/domains` that accepts a domain hostname string and adds a custom domain mapping.
3. THE Backend_Service SHALL expose a REST endpoint `GET /api/v1/projects/{project_id}/domains` to list all domain mappings for a Project.
4. THE Backend_Service SHALL expose a REST endpoint `DELETE /api/v1/projects/{project_id}/domains/{domain_id}` to remove a domain mapping.
5. WHEN a custom domain is added, THE Dashboard_App SHALL display DNS configuration instructions specifying a CNAME record pointing the custom domain to the CloudFront Distribution domain name.
6. IF a user attempts to add a domain that is already mapped to any Project in the system, THEN THE Backend_Service SHALL reject the request with an error indicating the domain is already in use.
7. IF an unauthenticated request is made to any domain mapping endpoint, THEN THE Backend_Service SHALL reject the request with HTTP status 401.
8. IF a user attempts to manage domains for a Project they do not own, THEN THE Backend_Service SHALL reject the request with HTTP status 403.

### Requirement 12: Backend Service Setup

**User Story:** As a developer, I want the Go backend properly scaffolded with Firestore and AWS SDK integration, so that the service can manage data and trigger cloud operations.

#### Acceptance Criteria

1. THE Backend_Service SHALL be implemented in Go using a standard HTTP router (net/http, Fiber, or Chi).
2. THE Backend_Service SHALL initialize a Firestore client using the `firebase.google.com/go/v4` package on startup.
3. THE Backend_Service SHALL initialize AWS SDK for Go v2 clients for CodeBuild, S3, and CloudWatch Logs on startup.
4. THE Backend_Service SHALL be containerized with a Dockerfile and deployable to Google Cloud Run.
5. THE Backend_Service SHALL read the listening port from the `PORT` environment variable, defaulting to `8080` if not set.
6. THE Backend_Service SHALL expose a health check endpoint `GET /health` that returns HTTP status 200 when the service is ready to accept requests.
7. IF Firestore client initialization fails on startup, THEN THE Backend_Service SHALL log the error and terminate with a non-zero exit code.
8. IF AWS SDK client initialization fails on startup, THEN THE Backend_Service SHALL log the error and terminate with a non-zero exit code.

### Requirement 13: Shared Package

**User Story:** As a developer, I want shared TypeScript types and API DTOs in a common package, so that the frontend applications maintain type safety and consistency with the backend API contract.

#### Acceptance Criteria

1. THE Shared_Package SHALL export TypeScript type definitions for all API request and response DTOs corresponding to the Backend_Service REST endpoints: `GET /api/v1/projects`, `POST /api/v1/projects`, `POST /api/v1/projects/{project_id}/domains`, `GET /api/v1/projects/{project_id}/domains`, `DELETE /api/v1/projects/{project_id}/domains/{domain_id}`, and `POST /api/v1/webhooks/github`.
2. THE Shared_Package SHALL export TypeScript type definitions for Project, User, Build, and Domain entities.
3. THE Shared_Package SHALL be importable by both the Landing_App and the Dashboard_App as a workspace dependency.
4. WHEN a DTO definition is updated in the Shared_Package, THE Landing_App and THE Dashboard_App SHALL receive the updated type at build time without publishing the package to a registry.
5. THE Shared_Package SHALL declare a TypeScript entry point in its `package.json` via the `types` or `exports` field so that consuming applications resolve type definitions without additional configuration.
6. THE Shared_Package SHALL compile without TypeScript errors when the `tsc --noEmit` command is executed in the package directory.
