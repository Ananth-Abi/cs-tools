# Customer Entity Service

## Tech Stack

| Layer     | Technology               |
| --------- | ------------------------ |
| Language  | Go 1.26.3                |
| Framework | Gin                      |
| Database  | PostgreSQL 15+           |
| Driver    | pgx v5 (connection pool) |

## Project Structure

```text
entity-service/
├── cmd/api/main.go              # Entry point — wires all layers and starts the server
├── internal/
│   ├── config/config.go         # Env-based config, builds PostgreSQL DSN
│   ├── db/
│   │   ├── postgres.go          # pgxpool setup and connection
│   │   └── migrate.go           # Schema migration runner
│   ├── domain/entity.go         # Shared domain types (Case, Page, inputs)
│   ├── events/events.go         # Envelope{Type, EntityID, Payload} — the case-events wire shape, kept in sync by hand with apps/csm-portal/backend and csm-notification-service's own copies
│   ├── eventbus/
│   │   ├── config.go            # Config + SASL/PLAIN setup for Azure Event Hub's Kafka-compatible endpoint
│   │   ├── producer.go          # Producer — publish a record, wait for ack
│   │   └── logger.go            # Bridges kafka-go's Logger/ErrorLogger to slog
│   ├── service/
│   │   ├── interfaces.go        # CaseRepository and CaseService interfaces
│   │   ├── entity_service.go    # Business logic — pagination, validation
│   │   ├── event_publisher_service.go # EventPublisherService.Publish — builds the envelope, publishes it, records a failure if Event Hub doesn't ack (wired in via routes.go; called from snCaseService.CreateCase and snIncidentService.CreateIncident)
│   │   └── sla_clock_service.go # SLAClockService — register/get/mark-tier-reached for a case's SLA clocks
│   ├── repository/
│   │   ├── entity_repo.go       # SQL queries against the "case" table
│   │   └── tx.go                # Transaction helper
│   ├── handler/
│   │   ├── entity_handler.go    # HTTP handler — bind JSON, call service, respond
│   │   └── health_handler.go    # /healthz and /readyz probes
│   ├── server/
│   │   ├── server.go            # Gin engine setup, middleware registration
│   │   └── routes.go            # URL → handler mapping
│   ├── middleware/
│   │   ├── logger.go            # Request logging
│   │   ├── recovery.go          # Panic recovery → 500
│   │   └── timeout.go           # Per-request context deadline
│   └── apierror/errors.go       # Sentinel errors and JSON error responder
├── migrations/                  # SQL migration files (up/down)
├── queries/                     # Raw SQL queries (sqlc source)
├── deploy/                      # Dockerfile and docker-compose
├── sqlc.yaml                    # sqlc code generation config
├── .env.example                 # Environment variable template
└── Makefile                     # Common dev targets
```

## Prerequisites

- Go 1.21+
- PostgreSQL 15+ (local via Docker or Azure)
- (Optional) [sqlc](https://sqlc.dev/) for query code generation

## Quick Start

### 1. Clone and install dependencies

```bash
git clone https://github.com/wso2-open-operations/cs-tools
cd cs-tools/entity-service
go mod download
```

### 2. Configure environment

```bash
cp .env.example .env
```

Edit `.env` with your database credentials:

```env
DB_HOST=localhost
DB_PORT=5434
DB_USER=your_user
DB_PASSWORD=your_password
DB_NAME=your_db
DB_SSLMODE=disable       # use "require" for Azure PostgreSQL
```

### 3. Run

```bash
go run cmd/api/main.go
```

Server starts at `http://localhost:8080`.

## Request Flow

```text
HTTP Request
  └── Gin Router
        └── Middleware (logger, recovery, timeout)
              └── Handler          — bind JSON, validate
                    └── Service    — business logic, pagination
                          └── Repository  — SQL query
                                └── PostgreSQL
```

## Environment Variables

| Variable    | Required | Default   | Description       |
| ----------- | -------- | --------- | ----------------- |
| DB_HOST     | Yes      | localhost | PostgreSQL host   |
| DB_PORT     | Yes      | 5432      | PostgreSQL port   |
| DB_USER     | Yes      | postgres  | Database user     |
| DB_PASSWORD | Yes      | —         | Database password |
| DB_NAME     | Yes      | postgres  | Database name     |
| DB_SSLMODE  | No       | require   | SSL mode          |

> `.env` file is loaded automatically if present. Absent `.env` is silently ignored; a malformed one causes a fatal startup error.

### Directory vocabularies — moved

`CSM_TEAM_REGISTRY` and `CSM_USER_ROLES` are **no longer read by this service**. The team registry
and the assignable-role allow-list are organisation vocabulary; they now live in the CSM portal
backend, which resolves them once at startup and serves `POST /teams/search` and
`POST /roles/search` from memory. This service holds no organisation vocabulary at all.

Configure them in `apps/csm-portal/backend/.env` — see that module's
[README](../apps/csm-portal/backend/README.md#directory-vocabularies). Setting them here has no
effect.

### Event Hub publishing

`internal/service.EventPublisherService` publishes domain events to Event Hub's Kafka-compatible
endpoint for `csm-notification-service` to consume. Constructed in `internal/server/routes.go`,
gated on `EVENT_HUB_BROKER` (not `DATA_SOURCE`) — left unset, nothing changes; `CreateCase`/
`CreateIncident` behave exactly as before this was wired in.

Currently published, both ServiceNow-data-source-only: `case.created` (from
`snCaseService.CreateCase`, re-enriched via `GetCaseByID` for the reporter's name/project
name/watch-list emails — `Recipients` is the watch list's emails only, and publishing is
skipped for a case with no watchers) and `incident.created` (from
`snIncidentService.CreateIncident`, built directly from the request — no enrichment call
needed). This service only publishes the fact that a case/incident was created — it builds
no portal link for either; `incident.created`'s "Open in Portal" button target is built by
csm-notification-service itself from the event's own entity id, the same way it already
builds case.created's portal link. See entity-service's `CLAUDE.md` ("Event Hub publishing")
for the full reasoning, including why both publish synchronously with a bounded timeout
rather than async.

| Variable | Description |
|---|---|
| `EVENT_HUB_BROKER` | Kafka bootstrap address: `<namespace>.servicebus.windows.net:9093` — the feature gate (optional) |
| `EVENT_HUB_CONNECTION_STRING` | The namespace's Shared Access Policy connection string — must be namespace-scoped (no `EntityPath`), not scoped to a single Event Hub (required once `EVENT_HUB_BROKER` is set) |
| `EVENT_HUB_TOPIC` | Event Hub (Kafka topic) name, e.g. `case-events` — must match `csm-notification-service`'s own `EVENT_HUB_TOPIC` (required once `EVENT_HUB_BROKER` is set) |

### SLA clocks

`sla_clocks` (migration `000011`) durably tracks per-case SLA timers — `caseId`/`clockType`,
`startedAt`/`dueAt`, and up to three tier-crossing timestamps (`reached50At`/`reached75At`/`reached100At`).
Has no ServiceNow equivalent — always backed by Postgres regardless of `DATA_SOURCE`, same as
`event_publish_failures`. `clockType` is a caller-defined string, not a fixed enum: which clock types
exist and what duration each gets is a policy decision made entirely by whatever publishes the
triggering event — this service only stores the result, it does not compute durations from case
severity or anything else.

Consumed by `csm-notification-service`'s SLA timer engine (`internal/slaengine`), which registers a
clock on `POST /cases/{caseId}/sla-clocks`, reads it back via `GET /cases/{caseId}/sla-clocks/{clockType}`
to check `pausedOn` before firing a tier, and records a crossed tier idempotently via
`PATCH /cases/{caseId}/sla-clocks/{clockType}/tiers/{tier}` with `{"status": "reached"}`.

## Security Scanning

Run [gosec](https://github.com/securego/gosec) to check for common security issues:

```bash
# Install gosec (once)
go install github.com/securego/gosec/v2/cmd/gosec@latest

# Run from entity-service
gosec -fmt=text ./...
```

The scan should report **0 issues**. If a new finding appears, fix the root cause before merging — do not suppress it without a code review.

Run [govulncheck](https://golang.org/x/vuln/cmd/govulncheck) to check for known vulnerabilities:

```bash
# Install govulncheck (once)
go install golang.org/x/vuln/cmd/govulncheck@latest

# Run from entity-service
govulncheck ./...
```

The scan should report **no vulnerabilities**. Most findings are Go standard-library CVEs tied to the toolchain patch pinned in `go.mod`'s `go` directive — bump it to the latest `1.26.x` patch and run `go mod tidy` to resolve them.

## License

Apache License 2.0 — see [LICENSE](LICENSE).
