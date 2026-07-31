# Customer Portal Backend (v2)

Go HTTP server (`net/http`, Go 1.26+) that acts as a backend-for-frontend (BFF) for the customer
portal. It authenticates callers, forwards requests to `cs-tools/entity-service`, and shapes
responses for the frontend. This is a Go rewrite of the Ballerina backend at
`apps/customer-portal/backend`, modeled on `apps/csm-portal/backend`'s conventions — read that
backend's own CLAUDE.md too if something here is underspecified.

**Status: in progress.** Only 6 routes are wired up so far (`GET /health`, `GET /users/me`,
`POST /projects/search`, `GET /projects/{id}`, `POST /cases/search`, `GET /cases/{id}`). The
Ballerina backend exposes ~100 routes across many more modules (accounts, deployments, deployed
products, attachments, comments, conversations, change requests, call requests, catalogs, time
cards, updates, registry tokens, contacts, escalations, product vulnerabilities, AI chat/websocket,
etc.) — none of those are ported yet. Follow the recipe below to add the next one.

## Which entity-service

This backend targets **`cs-tools/entity-service`** (this repo, `../../../entity-service`), *not*
the `digiops-cs/entity-service` that `apps/customer-portal/backend` (the Ballerina original) calls.
The two are different services with overlapping but not identical APIs — before porting an
endpoint, verify it actually exists on `cs-tools/entity-service` (check
`entity-service/internal/server/routes.go` and `entity-service/openapi.yaml`), and note that many
of its routes are **ServiceNow-only** (registered only when the service runs with
`DATA_SOURCE=servicenow`; see `entity-service/internal/config/config.go`) — a Postgres-mode
deployment will 404 on those. If the Ballerina backend has an endpoint with no `cs-tools/entity-service`
equivalent at all (e.g. `GET /metadata` — entity-service has no metadata endpoint), do not invent
one; add a code comment at the call site noting the gap and flag it instead of fabricating a
response.

## Middleware chain

`SecurityHeaders → CorrelationID → Auth → Logger → Mux`

Identical to `apps/csm-portal/backend`'s chain — see that backend's CLAUDE.md for the rationale of
each layer. `middleware.ConfigureLogger()` must be called at startup.

## Response shaping — the "wrapper" pattern

**Never return an entity-service response struct directly to the frontend.** This is the one
deliberate difference from `apps/csm-portal/backend` (which does raw `[]byte` passthrough for most
entity responses) — it mirrors what the Ballerina backend does with its `types` module +
`utils.bal` mapper functions (`mapCaseResponse`, `mapProjectsResponse`, etc.), which reshape every
`entity:*Response` into a portal-owned DTO before it reaches the frontend.

Concretely:

- `internal/entity` decodes entity-service's raw JSON into typed Go structs that mirror its wire
  format 1:1 (see `internal/entity/types.go`) — these types are internal to that package.
- `internal/dto` defines the portal's own response structs and one `Map*` function per entity type
  that translates entity → portal, dropping fields the customer portal has no business showing:
  - Salesforce/internal IDs (e.g. `ProjectDetailsView.SfID`)
  - Internal feature-flags (e.g. `ProjectAccountRef.AgentEnabled`/`KbReferencesEnabled`)
  - CSM/WSO2-internal-only fields that entity-service itself documents as such (e.g.
    `CaseView`'s `BestCaseFixEta`/`MostLikelyFixEta`/`WorstCaseFixEta`, `WatchList` — see the
    comments in `entity-service/internal/domain/entity.go` on `CaseView`)
  - WSO2-internal team routing (e.g. `AccountRef.CreTeam`/`SreTeam`)
  - Internal opaque identifiers not meaningful to a customer (e.g. `SearchCaseView.InternalID`)
- `internal/handler` calls the entity client, passes the result through the matching `dto.Map*`
  function, and writes the DTO with `writeJSONValue`.

When you add a new endpoint, add the equivalent trimming — read the field carefully before
including it; when in doubt whether a field is customer-appropriate, leave it out and note why in
a comment on the DTO struct (see `internal/dto/case.go` for examples).

Request bodies are the exception: incoming search/filter payloads are decoded directly into the
entity package's request structs (e.g. `entity.SearchProjectsRequest`) with no separate DTO layer,
since those shapes are already what the frontend needs to send — there is nothing to hide on the
request side.

## Adding a new endpoint

1. **Confirm the route exists on `cs-tools/entity-service`** — check `routes.go` and note whether
   it's Postgres-only, ServiceNow-only, or both (see "Which entity-service" above). If it doesn't
   exist, stop and add a comment instead of faking it.
2. **Entity types** (`internal/entity/types.go`) — add the request/response structs, copied field-
   for-field (name, type, `json` tag) from `entity-service/internal/domain/entity.go`. Don't guess
   — read the actual struct.
3. **Entity client method** (`internal/entity/<feature>.go`) — add a method on `Client` using
   `c.getJSON`/`c.postJSON`; `url.PathEscape()` every path parameter.
4. **Portal DTO** (`internal/dto/<feature>.go`) — add the trimmed response struct and a `Map*`
   function. See "Response shaping" above for what to exclude.
5. **Handler** (`internal/handler/<feature>.go`) — extend or add a local interface naming only the
   entity-client methods this handler needs; handler method sequence: auth check → path/body
   guards → call entity client → `mapUpstreamError` on failure → map to DTO → `writeJSONValue`.
6. **Route** (`cmd/server/main.go`) — register using Go 1.22 method-prefixed patterns:
   `"POST /cases/{id}/comments"`.
7. **README** — add the endpoint under "API Endpoints" in `README.md`.
8. **gosec** — run `gosec -fmt=text ./...` (must report 0 issues) before opening a PR.

## Handler conventions

- **Auth**: always check `middleware.UserInfoFromContext(r.Context()) == nil` first → 401.
- **Body size**: use the shared `readJSONBody(w, r)` helper (`internal/handler/response.go`) — caps
  at `maxRequestBodyBytes` (1 MiB) and validates the body is well-formed JSON.
- **Path params**: guard against empty string after `r.PathValue("id")`; validate UUID-shaped IDs
  with the package-level `uuidRe` and return 400 on mismatch before calling entity-service.
- **Upstream errors**: always use `mapUpstreamError(w, err, "<fallback message>")` — never write
  custom status mappings inline.
- **Logging**: use `slog.ErrorContext` with `summarizeErr(err)`, never the raw error — an
  unrecognized error can stringify with the full request URL including query params.

## Security

- **Never commit secrets** — `.env`, `Config.toml`, or any file with real credentials must never be
  staged.
- **No sensitive data in logs** — do not log request bodies, JWT payloads, or PII such as email/name. The opaque `userID` claim (`UserInfo.UserID`) is not PII and may be logged for correlation/support purposes, as every handler does today.
- **JWT is the only inbound auth mechanism** — every non-health endpoint must go through
  `middleware.Auth`.
- **Run gosec on every change** — `gosec -fmt=text ./...` must report 0 issues before opening a PR.
