# AE Brand User Invitation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a production-ready Go HTTP service for brand user creation, persona updates, user removal, user display, and audit querying from the PRD.

**Architecture:** The service is a standalone BPS-style gateway with a domain/service layer, pluggable Admin API client, durable repository abstraction, and HTTP handlers matching the PRD endpoints. The default local runtime uses an atomic JSON repository so the service runs without external infrastructure; production can point `ADMIN_API_BASE_URL` at the real Admin API.

**Tech Stack:** Go 1.26, standard-library HTTP server, structured JSON logging, table-driven tests, Docker, GitHub Actions.

---

## PRD Coverage Notes

- Create user: `POST /api/v1/account/{account_id}/users`
- Edit personas and remove user access: `POST /api/v1/account/{account_id}/users:batch`
- Display users: `GET /api/v1/account/{account_id}/users`
- Audit trail query: `GET /api/v1/account/{account_id}/audit`
- Security: user management endpoints require `super_admin` in `X-Actor-Personas`.
- Audit retention: audit query defaults to entries within `AUDIT_RETENTION_DAYS`, default `365`.
- Admin API duplicate conflict: the PRD contains two behaviors. The implementation returns conflict when email already exists, except when the same user is already attached to the account, where create is idempotent.

## File Structure

- `cmd/server/main.go`: application entrypoint, config load, dependency wiring, graceful shutdown.
- `internal/config/config.go`: environment configuration and defaults.
- `internal/domain/models.go`: domain entities, request DTOs, personas, audit operations.
- `internal/domain/errors.go`: typed errors mapped by HTTP handlers.
- `internal/domain/validation.go`: account, email, mobile, and persona validation.
- `internal/admin/admin.go`: Admin API client interface and errors.
- `internal/admin/file_client.go`: local file-backed Admin API implementation for development and tests.
- `internal/admin/http_client.go`: production HTTP Admin API adapter.
- `internal/store/store.go`: account, user, and audit repository interfaces.
- `internal/store/filestore.go`: atomic JSON repository implementation.
- `internal/service/service.go`: create, update personas, remove, list users, and audit query logic.
- `internal/httpapi/router.go`: route registration and request dispatch.
- `internal/httpapi/handlers.go`: endpoint handlers and JSON response helpers.
- `internal/httpapi/auth.go`: actor extraction and super-admin enforcement.
- `internal/httpapi/middleware.go`: request id, logging, panic recovery, timeouts.
- `api/openapi.yaml`: REST contract.
- `README.md`: local run, configuration, API examples, production notes.
- `Dockerfile`, `.dockerignore`, `Makefile`, `.github/workflows/ci.yml`, `.env.example`: operational assets.

## Tasks

### Task 1: Bootstrap Module And Domain Tests

**Files:**
- Create: `go.mod`
- Create: `internal/domain/models.go`
- Create: `internal/domain/errors.go`
- Create: `internal/domain/validation.go`
- Test: `internal/domain/validation_test.go`

- [ ] Step 1: Write validation tests covering valid personas, invalid email, blank account id, blank persona, and blank name.
- [ ] Step 2: Run `go test ./internal/domain` and verify it fails because the package is missing.
- [ ] Step 3: Add domain models and validation functions.
- [ ] Step 4: Run `go test ./internal/domain` and verify it passes.
- [ ] Step 5: Commit with `git commit -m "feat: add domain model and validation"`.

### Task 2: Add Store And Admin Adapters

**Files:**
- Create: `internal/admin/admin.go`
- Create: `internal/admin/file_client.go`
- Create: `internal/admin/http_client.go`
- Create: `internal/store/store.go`
- Create: `internal/store/filestore.go`
- Test: `internal/store/filestore_test.go`

- [ ] Step 1: Write file store tests for add, list, update personas, remove, audit append, audit filtering, and duplicate email.
- [ ] Step 2: Run `go test ./internal/store` and verify it fails because store code is missing.
- [ ] Step 3: Implement interfaces, atomic JSON persistence, and local Admin client behavior.
- [ ] Step 4: Run `go test ./internal/store ./internal/admin` and verify it passes.
- [ ] Step 5: Commit with `git commit -m "feat: add persistence and admin adapters"`.

### Task 3: Add Service Layer

**Files:**
- Create: `internal/service/service.go`
- Test: `internal/service/service_test.go`

- [ ] Step 1: Write service tests for create success, duplicate email conflict, create idempotency, missing super admin, update personas audit, remove audit, and list users with details fallback.
- [ ] Step 2: Run `go test ./internal/service` and verify it fails because service code is missing.
- [ ] Step 3: Implement service orchestration and audit entries.
- [ ] Step 4: Run `go test ./internal/service` and verify it passes.
- [ ] Step 5: Commit with `git commit -m "feat: implement invitation service"`.

### Task 4: Add HTTP API

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/httpapi/router.go`
- Create: `internal/httpapi/handlers.go`
- Create: `internal/httpapi/auth.go`
- Create: `internal/httpapi/middleware.go`
- Create: `cmd/server/main.go`
- Test: `internal/httpapi/handlers_test.go`

- [ ] Step 1: Write handler tests for create, create forbidden, batch update, batch remove, get users, audit query, malformed JSON, and method not allowed.
- [ ] Step 2: Run `go test ./internal/httpapi` and verify it fails because handlers are missing.
- [ ] Step 3: Implement router, middleware, config, and server main.
- [ ] Step 4: Run `go test ./internal/httpapi` and verify it passes.
- [ ] Step 5: Commit with `git commit -m "feat: expose brand user HTTP API"`.

### Task 5: Add Operational Assets

**Files:**
- Create: `README.md`
- Create: `api/openapi.yaml`
- Create: `.env.example`
- Create: `.dockerignore`
- Create: `Dockerfile`
- Create: `Makefile`
- Create: `.github/workflows/ci.yml`

- [ ] Step 1: Add documentation covering setup, headers, endpoint examples, configuration, duplicate-user assumption, and production notes.
- [ ] Step 2: Add OpenAPI paths for the four service endpoints.
- [ ] Step 3: Add Docker, make, and CI assets.
- [ ] Step 4: Run `go test ./...` and `go test -race ./...`.
- [ ] Step 5: Commit with `git commit -m "chore: add docs and operational tooling"`.

### Task 6: Final Verification

**Files:**
- Modify only files needed for fixes discovered by verification.

- [ ] Step 1: Run `gofmt -w` on all Go files.
- [ ] Step 2: Run `go test ./...`.
- [ ] Step 3: Run `go test -race ./...`.
- [ ] Step 4: Run `go build ./cmd/server`.
- [ ] Step 5: Commit final fixes if any with a focused message.
