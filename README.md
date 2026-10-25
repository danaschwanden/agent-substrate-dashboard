# Agent Substrate Dashboard

A standalone, portable dashboard for one Agent Substrate atespace and worker pool. The repository contains an independently deployable Lit frontend and Go backend joined by a versioned OpenAPI contract.

The repository includes the versioned contract, a Lit component dashboard, and a Go backend with opt-in lifecycle operations.

## Development

Requirements: Go 1.27, Node.js 22, npm 10, and `kubectl` with Kustomize support.

```bash
npm install
npm run generate
npm run check
npm --workspace frontend run test:ui
```

Generated Go and TypeScript files are checked in. `npm run check:generated` regenerates them in a temporary directory and fails if they drift from `api/openapi.yaml`.

The first supported Agent Substrate version is `v0.6.1-0.20261023032142-4ebd668d62e3` (`4ebd668d62e3`).

## Backend

The backend calls the Agent Substrate Control API directly over TLS. It does not use a Kubernetes client or invoke `kubectl`.

Required environment variables:

- `DASHBOARD_ATESPACE`
- `DASHBOARD_WORKER_POOL`
- `DASHBOARD_CONTROL_TRUST_BUNDLE_FILE`
- `DASHBOARD_CONTROL_TOKEN_FILE`

The Control endpoint defaults to `api.ate-system.svc:443`, its certificate name to `api.ate-system.svc`, and the HTTP listener to `:8080`. These can be overridden with `DASHBOARD_CONTROL_ENDPOINT`, `DASHBOARD_CONTROL_SERVER_NAME`, and `DASHBOARD_LISTEN_ADDRESS`.

The process exposes `/api/v1/meta`, `/api/v1/snapshot`, `/api/v1/operations/resume`, `/api/v1/operations/suspend`, `/healthz`, and `/readyz`.

The backend defaults to observe-only. Lifecycle operations require `DASHBOARD_ENABLE_OPERATIONS=true` and a Control API token with matching editor permission for the configured atespace. Optional tuning variables are `DASHBOARD_OPERATION_CONCURRENCY` (default `4`) and `DASHBOARD_OPERATION_TIMEOUT` (default `10s`). Actor scope always comes from `DASHBOARD_ATESPACE`; HTTP clients can submit names but cannot supply or override an atespace.

## Frontend

Run `npm --workspace frontend run dev` for the dashboard. Add `?fixture=mixed` to use a canonical fixture without a backend, or open `/gallery.html` to render every card independently.

## Design

- [Architecture](./docs/ARCHITECTURE.md)
- [Deployment and AccessPolicy guide](./docs/DEPLOYMENT.md)
- [Implementation plan](./IMPLEMENTATION_PLAN.md)
- [Glossary](./GLOSSARY.md)
- [Architecture decisions](./docs/adr/)
