# AE Brand User Invitation System

Production-ready Go service for brand user invitation and self-service onboarding from the PRD.

The service exposes BPS-style HTTP APIs for:

- Creating brand users through an Admin API boundary
- Updating multiple personas per user
- Removing account access
- Listing account users with Admin API user details
- Querying a one-year audit trail

## API

All account-management endpoints require these gateway-injected headers:

```http
X-Actor-User-Id: actor-1
X-Actor-Email: actor@example.com
X-Actor-Personas: super_admin,ads_admin
X-Request-Id: req-123
```

`super_admin` is required for create, update, remove, list, and audit query operations.

### Create User

```http
POST /api/v1/account/{account_id}/users
Content-Type: application/json
```

```json
{
  "email": "brand.user@example.com",
  "name": "Brand User",
  "mobile": "+919876543210",
  "personas": ["ads_admin", "discount_analyst"]
}
```

### Update Personas Or Remove Users

```http
POST /api/v1/account/{account_id}/users:batch
Content-Type: application/json
```

```json
{
  "user_operations": [
    {
      "operation": "UPDATE_PERSONAS",
      "user_id": "USER_POOL_BRAND#abc123",
      "personas": ["super_admin", "ads_admin"]
    },
    {
      "operation": "REMOVE",
      "user_id": "USER_POOL_BRAND#def456"
    }
  ]
}
```

### List Users

```http
GET /api/v1/account/{account_id}/users
```

### Query Audit

```http
GET /api/v1/account/{account_id}/audit?actor_id=actor-1&from=2026-05-01T00:00:00Z&to=2026-05-21T23:59:59Z
```

OpenAPI is available at [api/openapi.yaml](api/openapi.yaml).

## Duplicate User Behavior

The PRD has one conflict: the flow diagram suggests recovering from an existing-email Admin API response, while the test matrix expects duplicate create to return `User already exists`.

This implementation returns `409 Conflict` when the email exists but is not already attached to the account. If the email maps to a user already attached to the same account, the create call is idempotent and returns success with `User already exists in account`.

## Configuration

Environment variables:

| Variable | Default | Description |
| --- | --- | --- |
| `HTTP_ADDR` | `:8080` | HTTP listen address |
| `DATA_FILE` | `data/app_state.json` | Atomic JSON state file used by the local repository |
| `ADMIN_API_BASE_URL` | empty | When set, use HTTP Admin API adapter instead of local file-backed Admin API |
| `AUDIT_RETENTION_DAYS` | `365` | Retention window applied to audit queries |
| `REQUEST_TIMEOUT` | `3s` | HTTP server and Admin detail lookup timeout |
| `SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown timeout |

Copy [.env.example](.env.example) when running locally.

## Local Development

```bash
make test
make run
```

Without `ADMIN_API_BASE_URL`, the service runs fully standalone using the file-backed Admin API and repository. This is useful for local development and integration tests. For production, set `ADMIN_API_BASE_URL` to the real Admin API gateway.

## Build

```bash
make build
docker build -t ae-brand-user-invitation .
```

## Production Notes

- The domain, service, Admin API client, and repository are separated so DynamoDB or Postgres can replace the file repository without changing handlers.
- User management is denied unless `X-Actor-Personas` includes `super_admin`.
- Passwords are never accepted or stored by this service.
- Audit entries include actor, account, target user, operation, old personas, new personas, request id, and timestamp.
- `GetUsersOfAccount` fetches user details concurrently. Missing Admin API details do not hide account users; the response includes the user with `user_details` omitted.
