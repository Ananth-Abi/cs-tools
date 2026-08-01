# Customer Portal Backend (v2)

Go rewrite of the Ballerina backend at `apps/customer-portal/backend`. It is a backend-for-frontend
(BFF) for the customer portal: it authenticates callers, forwards requests to
[`entity-service`](../../../entity-service) (this repo's `cs-tools/entity-service`, not the
`digiops-cs/entity-service` the Ballerina backend targets), and shapes the responses for the frontend.

This is a work in progress — only the 52 routes listed below are implemented so far, across
entity-service, the WSO2 Updates service, SCIM, the AI chat agent, and the product-consumption
service (two more separate services — see [CLAUDE.md](./CLAUDE.md#the-ai-chat-agent) and
[CLAUDE.md](./CLAUDE.md#the-product-consumption-service)). Everything else the Ballerina backend
exposes still needs a Go handler; add them following the pattern described in
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
  - Outbound calls to entity-service, the Updates service, SCIM, the AI chat agent, and the
    product-consumption service: OAuth2 client credentials grant, shared across all five (optional
    — entity-service itself does not validate inbound credentials, see [CLAUDE.md](./CLAUDE.md))

## Prerequisites

- Go `1.26+` — [install](https://go.dev/doc/install)
- A running instance of `cs-tools/entity-service` (see its own README for `DATA_SOURCE` setup —
  `GET`/`PATCH /users/me` require `DATA_SOURCE=servicenow`)
- A running instance of the WSO2 Updates service and the SCIM operations service (for the
  `/updates/*` routes and phone-number fields on `/users/me`)
- A running instance of the AI chat agent (a separate Python service, not entity-service — for
  `/cases/classify`, `/conversations/*`, `/projects/*/conversations/*`, and `/ws`)
- A running instance of the product-consumption service (a separate service, not entity-service —
  for `/projects/*/deployments/*/license` and `/deployment-usages`)

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

Every upstream service client (entity-service, updates, SCIM, the AI chat agent) authenticates as
the same OAuth2 client-credentials app — only each service's base URL and scopes differ.

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

### AI chat agent

A separate Python service (not entity-service) — see [CLAUDE.md](./CLAUDE.md#the-ai-chat-agent).

| Variable | Description |
|---|---|
| `AI_CHAT_AGENT_BASE_URL` | Base URL of the AI chat agent's HTTP API |
| `AI_CHAT_AGENT_SCOPES` | Comma-separated OAuth2 scopes (optional) |
| `AI_CHAT_AGENT_WS_BASE_URL` | Base URL of the AI chat agent's WebSocket endpoint |
| `AI_CHAT_AGENT_WS_SCOPES` | Comma-separated OAuth2 scopes (optional) |
| `WS_ALLOWED_ORIGINS` | Comma-separated browser Origins allowed to open `GET /ws` (optional — defense in depth against cross-site WebSocket hijacking; unset allows any origin, local development only) |

### Product-consumption service

A separate service (not entity-service) — see [CLAUDE.md](./CLAUDE.md#the-product-consumption-service).

| Variable | Description |
|---|---|
| `PRODUCT_CONSUMPTION_BASE_URL` | Base URL of the product-consumption service |
| `PRODUCT_CONSUMPTION_SCOPES` | Comma-separated OAuth2 scopes (optional) |

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
│   │   ├── cases.go             # SearchCases, GetCase, CreateCase, UpdateCase, CreateCaseComment, SearchCaseActivities
│   │   ├── deployments.go       # SearchDeployments, CreateDeployment
│   │   ├── deployed_products.go # SearchDeployedProducts, CreateDeployedProduct, UpdateDeployedProduct
│   │   ├── attachments.go       # CreateAttachment, SearchAttachments, GetAttachmentContent, DeleteAttachment
│   │   ├── products.go          # SearchProducts, SearchProductVersions
│   │   ├── change_requests.go   # create/search/get/update, approvals get/decide
│   │   ├── call_requests.go     # CreateCallRequest, SearchCallRequests, UpdateCallRequest
│   │   ├── comments.go          # CreateComment, SearchComments (generic, any reference entity)
│   │   ├── conversations.go     # SearchConversations
│   │   ├── product_vulnerabilities.go # SearchProductVulnerabilities, GetProductVulnerability
│   │   ├── catalogs.go          # SearchCatalogs, GetCatalogItemVariables
│   │   └── time_cards.go        # SearchTimeCards
│   ├── updates/                 # OAuth2 HTTP client for the WSO2 Updates service
│   │   ├── client.go            # Config/Client/do()
│   │   ├── types.go             # upstream (snake_case) vs portal (camelCase) structs
│   │   ├── mapper.go            # snake_case <-> camelCase mapping
│   │   └── updates.go           # GetProductUpdateLevels, SearchUpdatesBetweenUpdateLevels
│   ├── scim/                    # OAuth2 HTTP client for the SCIM operations service
│   │   ├── client.go            # Config/Client/do()
│   │   ├── types.go
│   │   └── scim.go              # SearchUser, UpdateUserPhone
│   ├── aichatagent/              # OAuth2 HTTP + WebSocket client for the AI chat agent (not entity-service)
│   │   ├── client.go            # Config/Client/do()/getJSON()/postJSON()
│   │   ├── types.go             # AI chat agent's wire-format structs
│   │   └── ws.go                # WSConfig/WSClient/StreamChat — proxies the upstream WebSocket
│   ├── productconsumption/       # OAuth2 HTTP client for the product-consumption service (not entity-service)
│   │   ├── client.go            # Config/Client/do()/postJSON()/patchJSON()
│   │   ├── types.go             # product-consumption service's wire-format structs
│   │   ├── subscription.go      # ProcessLicenseDownload — the deployment-license state machine
│   │   └── tracking.go          # ImportDeploymentUsage
│   ├── dto/                     # Portal-facing response shapes + Map* functions from entity types
│   │   ├── user.go
│   │   ├── account.go
│   │   ├── project.go
│   │   ├── case.go
│   │   ├── deployment.go
│   │   ├── deployed_product.go
│   │   ├── attachment.go
│   │   ├── product.go
│   │   ├── product_vulnerability.go
│   │   ├── catalog.go
│   │   ├── time_card.go
│   │   ├── comment.go
│   │   ├── ai_chat.go
│   │   ├── product_consumption.go
│   │   ├── change_request.go
│   │   └── call_request.go
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
│       ├── cases.go             # cases search/get/create/update/comment/activities
│       ├── deployments.go       # POST /deployments/search, POST /deployments, PATCH /deployments/{id}
│       ├── deployed_products.go # deployed-product search/create/update
│       ├── attachments.go       # attachment create/search/download/delete
│       ├── products.go          # POST /products/search, POST /products/{id}/versions/search
│       ├── product_vulnerabilities.go # vulnerability search/get
│       ├── catalogs.go          # catalog search, catalog item variables
│       ├── time_cards.go        # POST /time-cards/search
│       ├── comments.go          # generic comment create/search
│       ├── ai_chat.go           # case classification, recommendations, conversation search/messages/summary
│       ├── websocket.go         # GET /ws — real-time AI chat proxy
│       ├── product_consumption.go # deployment license provisioning, deployment usage import
│       ├── change_requests.go   # change-request create/search/get/update/approvals
│       ├── call_requests.go     # call-request create/search/update
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
- `POST /cases` — create a case
- `PATCH /cases/{id}` — update a case (restricted, customer-safe field subset — see CLAUDE.md)
- `POST /cases/{id}/comments` — add a comment to a case (always a plain customer comment)
- `POST /deployments/search` — search deployments
- `POST /deployments` — create a deployment (ServiceNow data source only)
- `PATCH /deployments/{id}` — update a deployment's name/type/description, or deactivate it
- `POST /deployed-products/search` — search deployed products
- `POST /deployed-products` — create a deployed product (ServiceNow data source only)
- `PATCH /deployed-products/{id}` — update a deployed product's cores/tps/description, or deactivate it (ServiceNow data source only)
- `POST /attachments` — create an attachment
- `POST /attachments/search` — search attachments
- `GET /attachments/{id}/content` — download an attachment's raw file content
- `DELETE /attachments/{id}` — delete an attachment
- `POST /cases/{id}/activities/search` — search a case's activity feed (comments, attachments, field changes)
- `POST /change-requests` — create a change request (ServiceNow data source only)
- `POST /change-requests/search` — search change requests (ServiceNow data source only)
- `GET /change-requests/{id}` — get change request by ID (ServiceNow data source only)
- `PATCH /change-requests/{id}` — update a change request (restricted, customer-safe field subset — see CLAUDE.md; ServiceNow data source only)
- `GET /change-requests/{id}/approvals` — get a change request's approval stages (ServiceNow data source only)
- `POST /change-requests/{id}/approvals/decision` — approve/reject the caller's own pending approval (ServiceNow data source only)
- `POST /call-requests` — create a call request (ServiceNow data source only)
- `POST /call-requests/search` — search call requests, scoped by caseId in the body (ServiceNow data source only)
- `PATCH /call-requests/{id}` — update a call request (restricted, excludes agent-only fields — see CLAUDE.md; ServiceNow data source only)
- `POST /products/search` — search products
- `POST /products/{id}/versions/search` — search a product's versions
- `POST /products/vulnerabilities/search` — search product vulnerabilities
- `GET /products/vulnerabilities/{id}` — get a product vulnerability by ID
- `POST /catalogs/search` — search service catalogs, scoped by deployedProductId
- `GET /catalogs/{catalogId}/items/{catalogItemId}/variables` — get a catalog item's form variables
- `POST /time-cards/search` — search time cards (read-only; ServiceNow data source only)
- `POST /comments` — add a comment to any reference entity (case, conversation, change_request, deployment, incident) — always a plain customer comment
- `POST /comments/search` — search comments on a reference entity — always filtered to plain customer comments
- `POST /cases/classify` — classify a chat transcript into a case type/severity via the AI chat agent
- `POST /conversations/recommendations/search` — get KB article recommendations via the AI chat agent
- `POST /projects/{id}/conversations/search` — search a project's AI chat conversations
- `GET /conversations/{id}/messages` — get a conversation's messages (backed by generic comment search)
- `POST /projects/{projectId}/conversations/{conversationId}/messages` — send a follow-up message on an existing conversation
- `GET /projects/{id}/conversations/{conversationId}/summary` — get a conversation's summary via the AI chat agent
- `GET /ws?sessionId={projectId}` — WebSocket: real-time AI chat proxy for an existing conversation (see CLAUDE.md — starting a brand-new conversation isn't supported yet)
- `POST /projects/{projectId}/deployments/{deploymentId}/license` — provision (or resume provisioning) and return a deployment's license via the product-consumption service
- `POST /deployment-usages` — import a deployment-usage zip file (raw binary body, `Content-Type: application/zip`) via the product-consumption service
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

curl -X POST http://localhost:8080/cases \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"type":"case","projectId":"<project-id>","deploymentId":"<deployment-id>","subject":"Login error","description":"...","severity":"high","issueType":"question"}'

curl -X PATCH http://localhost:8080/cases/<case-id> \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"state":"closed","closeNotes":"Resolved on our end, thanks!"}'

curl -X POST http://localhost:8080/cases/<case-id>/comments \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"content":"Any update on this?"}'

curl -X POST http://localhost:8080/deployments/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"pagination":{"limit":10,"offset":0}}'

curl -X POST http://localhost:8080/deployments \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"projectId":"<project-id>","name":"Production"}'

curl -X POST http://localhost:8080/deployed-products/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"pagination":{"limit":10,"offset":0},"deploymentIds":["<deployment-id>"]}'

curl -X POST http://localhost:8080/deployed-products \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"projectId":"<project-id>","deploymentId":"<deployment-id>","productId":"<product-id>","versionId":"<version-id>"}'

curl -X PATCH http://localhost:8080/deployed-products/<deployed-product-id> \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"cores":4}'

curl -X POST http://localhost:8080/attachments \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"referenceId":"<case-id>","referenceType":"case","name":"log.txt","type":"text/plain","file":"<base64>"}'

curl -X POST http://localhost:8080/attachments/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"referenceId":"<case-id>","referenceType":"case","pagination":{"limit":10,"offset":0}}'

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/attachments/<attachment-id>/content -o downloaded-file

curl -X DELETE -H "x-jwt-assertion: $JWT" http://localhost:8080/attachments/<attachment-id>

curl -X POST http://localhost:8080/cases/<case-id>/activities/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"pagination":{"limit":20,"offset":0}}'

curl -X POST http://localhost:8080/products/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"pagination":{"limit":10,"offset":0},"searchQuery":"wso2am"}'

curl -X POST http://localhost:8080/products/<product-id>/versions/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"pagination":{"limit":10,"offset":0}}'

curl -X POST http://localhost:8080/change-requests \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"subject":"Upgrade WSO2 API Manager to 4.3.0"}'

curl -X POST http://localhost:8080/change-requests/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"pagination":{"limit":10,"offset":0}}'

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/change-requests/<change-request-id>

curl -X PATCH http://localhost:8080/change-requests/<change-request-id> \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"isCustomerApproved":true}'

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/change-requests/<change-request-id>/approvals

curl -X POST http://localhost:8080/change-requests/<change-request-id>/approvals/decision \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"decision":"approved"}'

curl -X POST http://localhost:8080/call-requests \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"caseId":"<case-id>","reason":"Discuss workaround","utcTimes":["2026-08-05T10:00:00Z"],"durationInMinutes":30}'

curl -X POST http://localhost:8080/call-requests/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"caseId":"<case-id>","pagination":{"limit":10,"offset":0}}'

curl -X PATCH http://localhost:8080/call-requests/<call-request-id> \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"state":"customer_rejected","cancellationReason":"No longer needed"}'

curl -X PATCH http://localhost:8080/deployments/<deployment-id> \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"name":"Production (EU)"}'

curl -X POST http://localhost:8080/products/vulnerabilities/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"pagination":{"limit":10,"offset":0},"filters":{"productName":"wso2am"}}'

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/products/vulnerabilities/<vulnerability-id>

curl -X POST http://localhost:8080/catalogs/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"deployedProductId":"<deployed-product-id>","pagination":{"limit":10,"offset":0}}'

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/catalogs/<catalog-id>/items/<catalog-item-id>/variables

curl -X POST http://localhost:8080/time-cards/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"pagination":{"limit":10,"offset":0},"filters":{"projectIds":["<project-id>"]}}'

curl -X POST http://localhost:8080/comments \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"referenceId":"<change-request-id>","referenceType":"change_request","content":"Any update?"}'

curl -X POST http://localhost:8080/comments/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"referenceId":"<change-request-id>","referenceType":"change_request","pagination":{"limit":10,"offset":0}}'

curl -X POST http://localhost:8080/cases/classify \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"chatHistory":"user: my API gateway is down\n","envProducts":{},"region":"EU","tier":"gold","projectTypeId":"<project-type-id>"}'

curl -X POST http://localhost:8080/conversations/recommendations/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"chatHistory":[{"role":"user","content":"my API gateway is down","timestamp":"2026-08-01T10:00:00Z"}],"conversationData":{"chatHistory":"user: my API gateway is down","envProducts":{},"region":"EU","tier":"gold"}}'

curl -X POST http://localhost:8080/projects/<project-id>/conversations/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"filters":{},"sortBy":{},"pagination":{"limit":10,"offset":0}}'

curl -H "x-jwt-assertion: $JWT" "http://localhost:8080/conversations/<conversation-id>/messages?limit=20&offset=0"

curl -X POST http://localhost:8080/projects/<project-id>/conversations/<conversation-id>/messages \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"message":"It is still down","region":"EU","tier":"gold"}'

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/projects/<project-id>/conversations/<conversation-id>/summary

# WebSocket (real-time chat proxy) — resumes an existing conversation only, see CLAUDE.md.
# Using websocat (https://github.com/vi/websocat) as an example client:
websocat "ws://localhost:8080/ws?sessionId=<project-id>" -H "x-jwt-assertion: $JWT"
# then send: {"message":"still seeing the error","conversationId":"<conversation-id>"}

curl -X POST http://localhost:8080/projects/<project-id>/deployments/<deployment-id>/license \
  -H "x-jwt-assertion: $JWT"

curl -X POST http://localhost:8080/deployment-usages \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/zip" \
  --data-binary @deployment-usage.zip

curl -H "x-jwt-assertion: $JWT" http://localhost:8080/updates/product-update-levels

curl -X POST http://localhost:8080/updates/levels/search \
  -H "x-jwt-assertion: $JWT" -H "Content-Type: application/json" \
  -d '{"productName":"wso2am","productVersion":"4.2.0","startingUpdateLevel":1,"endingUpdateLevel":10}'
```
