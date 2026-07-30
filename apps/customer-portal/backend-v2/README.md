# Customer Portal Backend (v2)

Go rewrite of the Ballerina backend at `apps/customer-portal/backend`. It is a backend-for-frontend
(BFF) for the customer portal: it authenticates callers, forwards requests to
[`entity-service`](../../../entity-service) (this repo's `cs-tools/entity-service`, not the
`digiops-cs/entity-service` the Ballerina backend targets), and shapes the responses for the frontend.

This is a work in progress — only the first 5 endpoints are implemented so far (see below).
Everything else the Ballerina backend exposes still needs a Go handler; add them following the
pattern described in [CLAUDE.md](./CLAUDE.md#adding-a-new-endpoint).

## Quick Start

```bash
# from apps/customer-portal/backend-v2
go run ./cmd/server/main.go
```

The server automatically loads `.env` from the working directory on startup (silently ignored if absent).

Backend starts at `http://localhost:8080`.

## Overview

- Default port: `8080`
- Runtime: Go `1.26+`
- Entry point: `cmd/server/main.go`
- Authentication:
  - Incoming requests: JWT Bearer token; pass as `x-jwt-assertion` header when testing locally
  - Outbound calls to entity-service: OAuth2 client credentials grant (optional — entity-service
    itself does not validate inbound credentials, see [CLAUDE.md](./CLAUDE.md))

## Prerequisites

- Go `1.26+` — [install](https://go.dev/doc/install)
- A running instance of `cs-tools/entity-service` (see its own README for `DATA_SOURCE` setup —
  `GET /users/me` requires `DATA_SOURCE=servicenow`)

## Testing

```bash
go test ./...
go test -race ./...
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

Or use `make`:

```bash
make test    # vet + test
make build   # vet + test + compile
```

### Run tests before every push (recommended)

```bash
git config core.hooksPath .githooks   # from the repo root, once
# or: make setup   # from this directory
```

## Security Scanning

```bash
go install github.com/securego/gosec/v2/cmd/gosec@latest
gosec -fmt=text ./...
```

The scan must report **0 issues** before opening a PR touching this backend.

## Configuration

Copy `.env.example` to `.env` and fill in the values.

### Entity service

| Variable | Description |
|---|---|
| `ENTITY_SERVICE_BASE_URL` | Base URL of `cs-tools/entity-service` |
| `OAUTH2_CLIENT_ID` / `OAUTH2_CLIENT_SECRET` / `OAUTH2_TOKEN_URL` | Optional — only needed if entity-service sits behind a gateway requiring OAuth2 client-credentials auth |
| `ENTITY_SERVICE_SCOPES` | Comma-separated OAuth2 scopes (optional) |

### Auth

| Variable | Description |
|---|---|
| `AUTH_JWKS_ENDPOINT` | JWKS endpoint used to verify JWT signatures |
| `AUTH_ISSUER` | Expected `iss` claim value |
| `AUTH_AUDIENCE` | Comma-separated accepted `aud` values |
| `AUTH_TOKEN_VALIDATOR_ENABLED` | Set to `false` for local development to skip signature verification (default `true`) |

### Server

| Variable | Description |
|---|---|
| `PORT` | Server listen port — a plain number, not an address (default `8080`) |

## Project Structure

```text
backend-v2/
├── cmd/server/main.go           # Entry point — routes + server startup
├── internal/
│   ├── apierror/                # Typed upstream error type (4xx/5xx passthrough)
│   ├── entity/                  # OAuth2 HTTP client for cs-tools/entity-service
│   │   ├── client.go            # Config/Client/do()/getJSON()/postJSON()
│   │   ├── types.go             # entity-service's wire-format structs (internal to this package)
│   │   ├── users.go             # GetMe
│   │   ├── projects.go          # SearchProjects, GetProject
│   │   └── cases.go             # SearchCases, GetCase
│   ├── dto/                     # Portal-facing response shapes + Map* functions from entity types
│   │   ├── user.go
│   │   ├── project.go
│   │   └── case.go
│   ├── middleware/
│   │   ├── auth.go              # JWT validation; injects UserInfo into context
│   │   ├── correlation.go       # X-CSM-Correlation-ID propagation + slog enrichment
│   │   ├── logger.go            # Per-request access log
│   │   └── security_headers.go  # X-Content-Type-Options, CSP, HSTS on every response
│   └── handler/
│       ├── response.go          # writeJSON/writeError/mapUpstreamError shared helpers
│       ├── users.go             # GET /users/me
│       ├── projects.go          # POST /projects/search, GET /projects/{id}
│       └── cases.go             # POST /cases/search, GET /cases/{id}
├── .env.example
└── go.mod
```

## API Endpoints

- `GET /health` — liveness check, no auth
- `GET /users/me` — current user's profile (requires entity-service running with `DATA_SOURCE=servicenow`)
- `POST /projects/search` — search projects
- `GET /projects/{id}` — get project by ID
- `POST /cases/search` — search cases
- `GET /cases/{id}` — get case by ID

All response shapes are portal-owned DTOs (see `internal/dto`) — entity-service's raw response is
never returned verbatim; see [CLAUDE.md](./CLAUDE.md#response-shaping) for what is deliberately
excluded from each and why.

## Run Locally

```bash
go run ./cmd/server/main.go
```

When `AUTH_TOKEN_VALIDATOR_ENABLED=false` (default for local), pass any valid JWT as the
`x-jwt-assertion` header.

### Examples

```bash
JWT="<your-jwt-token>"

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/users/me

curl -X POST http://localhost:8080/projects/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"pagination":{"limit":10,"offset":0},"searchQuery":"acme"}'

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/projects/<project-id>

curl -X POST http://localhost:8080/cases/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"pagination":{"limit":10,"offset":0},"filters":{"searchQuery":"login error"}}'

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/cases/<case-id>
```
