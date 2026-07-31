# Customer Portal Backend (v2)

Go rewrite of the Ballerina backend at `apps/customer-portal/backend`. It is a backend-for-frontend
(BFF) for the customer portal: it authenticates callers, forwards requests to
[`entity-service`](../../../entity-service) (this repo's `cs-tools/entity-service`, not the
`digiops-cs/entity-service` the Ballerina backend targets), and shapes the responses for the frontend.

This is a work in progress — only the 11 routes listed below are implemented so far, across
entity-service, the WSO2 Updates service, and SCIM. Everything else the Ballerina backend exposes
still needs a Go handler; add them following the pattern described in
[CLAUDE.md](./CLAUDE.md#adding-a-new-endpoint).

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
  - Outbound calls to entity-service, the Updates service, and SCIM: OAuth2 client credentials
    grant, shared across all three (optional — entity-service itself does not validate inbound
    credentials, see [CLAUDE.md](./CLAUDE.md))

## Prerequisites

- Go `1.26+` — [install](https://go.dev/doc/install)
- A running instance of `cs-tools/entity-service` (see its own README for `DATA_SOURCE` setup —
  `GET`/`PATCH /users/me` require `DATA_SOURCE=servicenow`)
- A running instance of the WSO2 Updates service and the SCIM operations service (for the
  `/updates/*` routes and phone-number fields on `/users/me`)

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

### Shared OAuth2 client credentials

Every upstream service client (entity-service, updates, SCIM) authenticates as the same OAuth2
client-credentials app — only each service's base URL and scopes differ.

| Variable | Description |
|---|---|
| `OAUTH2_CLIENT_ID` / `OAUTH2_CLIENT_SECRET` / `OAUTH2_TOKEN_URL` | Optional — only needed if a service sits behind a gateway requiring OAuth2 client-credentials auth |

### Entity service

| Variable | Description |
|---|---|
| `ENTITY_SERVICE_BASE_URL` | Base URL of `cs-tools/entity-service` |
| `ENTITY_SERVICE_SCOPES` | Comma-separated OAuth2 scopes (optional) |

### Updates service

| Variable | Description |
|---|---|
| `UPDATES_BASE_URL` | Base URL of the WSO2 Updates service |
| `UPDATES_SCOPES` | Comma-separated OAuth2 scopes (optional) |

### SCIM operations service

| Variable | Description |
|---|---|
| `SCIM_BASE_URL` | Base URL of the SCIM operations service |
| `SCIM_SCOPES` | Comma-separated OAuth2 scopes (optional) |

### Auth

| Variable | Description |
|---|---|
| `AUTH_JWKS_ENDPOINT` | JWKS endpoint used to verify JWT signatures |
| `AUTH_ISSUER` | Expected `iss` claim value |
| `AUTH_AUDIENCE` | Comma-separated accepted `aud` values |
| `AUTH_TOKEN_VALIDATOR_ENABLED` | `false` skips JWT signature verification — **local development only**; `.env.example` ships `false` for local convenience. Production **must** set this to `true` with a real `AUTH_JWKS_ENDPOINT`/`AUTH_ISSUER`/`AUTH_AUDIENCE` |

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
│   │   ├── client.go            # Config/Client/do()/getJSON()/postJSON()/patchJSON()
│   │   ├── types.go             # entity-service's wire-format structs (internal to this package)
│   │   ├── users.go             # GetMe, PatchMe
│   │   ├── accounts.go          # SearchAccounts, GetAccount
│   │   ├── projects.go          # SearchProjects, GetProject
│   │   └── cases.go             # SearchCases, GetCase
│   ├── updates/                 # OAuth2 HTTP client for the WSO2 Updates service
│   │   ├── client.go            # Config/Client/do()
│   │   ├── types.go             # upstream (snake_case) vs portal (camelCase) structs
│   │   ├── mapper.go            # snake_case <-> camelCase mapping
│   │   └── updates.go           # GetProductUpdateLevels, SearchUpdatesBetweenUpdateLevels
│   ├── scim/                    # OAuth2 HTTP client for the SCIM operations service
│   │   ├── client.go            # Config/Client/do()
│   │   ├── types.go
│   │   └── scim.go              # SearchUser, UpdateUserPhone
│   ├── dto/                     # Portal-facing response shapes + Map* functions from entity types
│   │   ├── user.go
│   │   ├── account.go
│   │   ├── project.go
│   │   └── case.go
│   ├── middleware/
│   │   ├── auth.go              # JWT validation; injects UserInfo into context
│   │   ├── correlation.go       # X-CSM-Correlation-ID propagation + slog enrichment
│   │   ├── logger.go            # Per-request access log
│   │   └── security_headers.go  # X-Content-Type-Options, CSP, HSTS on every response
│   └── handler/
│       ├── response.go          # writeJSON/writeError/mapUpstreamError shared helpers
│       ├── users.go             # GET/PATCH /users/me
│       ├── accounts.go          # POST /accounts/search, GET /accounts/{id}
│       ├── projects.go          # POST /projects/search, GET /projects/{id}
│       ├── cases.go             # POST /cases/search, GET /cases/{id}
│       └── updates.go           # GET /updates/product-update-levels, POST /updates/levels/search
├── .choreo/component.yaml
├── openapi.yaml
├── .env.example
└── go.mod
```

## API Endpoints

- `GET /health` — liveness check, no auth
- `GET /users/me` — current user's profile; name/timezone/roles from entity-service (requires
  `DATA_SOURCE=servicenow`), phone number from SCIM
- `PATCH /users/me` — update phone number (SCIM) and/or timezone (entity-service); at least one required
- `POST /accounts/search` — search accounts (normalizes entity-service's Postgres/ServiceNow shapes into one)
- `GET /accounts/{id}` — get account by ID (same normalization)
- `POST /projects/search` — search projects
- `GET /projects/{id}` — get project by ID
- `POST /cases/search` — search cases
- `GET /cases/{id}` — get case by ID
- `GET /updates/product-update-levels` — list product update levels
- `POST /updates/levels/search` — search update descriptions between two update levels

Full request/response schemas are documented in [openapi.yaml](./openapi.yaml). All entity-service
and SCIM response shapes are portal-owned DTOs (see `internal/dto`) — the raw upstream response is
never returned verbatim; see [CLAUDE.md](./CLAUDE.md#response-shaping) for what is deliberately
excluded from each and why. The `updates` client is the one exception — its own types are already
portal-shaped camelCase (translated from the upstream snake_case in `internal/updates/mapper.go`),
so handlers write its return values directly with no further DTO layer.

## Run Locally

```bash
go run ./cmd/server/main.go
```

With `AUTH_TOKEN_VALIDATOR_ENABLED=false` (the `.env.example` local default), pass any valid JWT as
the `x-jwt-assertion` header — its signature is not verified.

### Examples

```bash
JWT="<your-jwt-token>"

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/users/me

curl -X PATCH http://localhost:8080/users/me \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"timeZone":"Asia/Colombo"}'

curl -X POST http://localhost:8080/accounts/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"pagination":{"limit":10,"offset":0},"filters":{"searchQuery":"acme"}}'

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/accounts/<account-id>

curl -X POST http://localhost:8080/projects/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"pagination":{"limit":10,"offset":0},"searchQuery":"acme"}'

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/projects/<project-id>

curl -X POST http://localhost:8080/cases/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"pagination":{"limit":10,"offset":0},"filters":{"searchQuery":"login error"}}'

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/cases/<case-id>

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/updates/product-update-levels

curl -X POST http://localhost:8080/updates/levels/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"productName":"wso2am","productVersion":"4.2.0","startingUpdateLevel":1,"endingUpdateLevel":10}'
```
