# Despliegue Facilito

Developer PaaS for automated static and frontend application deployments on AWS.

## Project Structure

```
├── apps/
│   ├── backend/    # Go REST API + WebSocket (Cloud Run)
│   ├── frontend/   # React SPA Dashboard (Vite + Tailwind)
│   └── landing/    # Astro SSG Marketing Site
├── dev.ps1         # Script to run all services locally
└── README.md
```

## Quick Start

### Prerequisites

- Go 1.22+
- Node.js 20+
- pnpm (for frontend apps)

### Run all services locally

```powershell
.\dev.ps1
```

This starts the backend, frontend dev server, and landing dev server concurrently.

### Run individually

```bash
# Backend
cd apps/backend
go run ./cmd/server

# Frontend (Dashboard)
cd apps/frontend
pnpm install
pnpm dev

# Landing
cd apps/landing
pnpm install
pnpm dev
```

## Architecture

- **Backend**: Go + Chi router, Firebase Auth, Firestore, AWS SDK (CodeBuild, S3, CloudWatch)
- **Frontend**: React + Vite + Tailwind CSS, Firebase Auth SDK
- **Landing**: Astro SSG, Tailwind CSS
- **Infrastructure**: AWS CloudFront + S3 (hosting), Google Cloud Run (backend)
