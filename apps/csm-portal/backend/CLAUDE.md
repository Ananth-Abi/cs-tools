# CSM Portal Backend

Go HTTP server (`net/http`, Go 1.26+) that acts as a backend-for-frontend (BFF) for the CSM portal. It authenticates callers, forwards requests to upstream services, and shapes responses for the frontend.

## Middleware chain

`SecurityHeaders → CorrelationID → Auth → Logger → Mux`

- `SecurityHeaders` (`internal/middleware/security_headers.go`): sets `X-Content-Type-Options: nosniff`, `Content-Security-Policy: upgrade-insecure-requests`, and `Strict-Transport-Security: max-age=31536000; includeSubDomains` on every response; outermost so headers are present even on auth failures
- `CorrelationID` (`internal/middleware/correlation.go`): reads `X-CSM-Correlation-ID` from the incoming request or generates a UUID v4; stores the ID in context for the slog handler and for the entity client to forward; echoes the ID in the response header
- `Auth` (`internal/middleware/auth.go`): validates the `x-jwt-assertion` JWT and sets `UserInfo` in context
- `Logger` (`internal/middleware/logger.go`): logs every completed request (method, path, status, elapsed) via slog; runs after Auth so both `correlationID` and `userID` are present in every record

`middleware.ConfigureLogger()` must be called at startup — it wraps the default slog handler so that every `slog.*Context(r.Context(), …)` call anywhere in the codebase automatically includes `correlationID=<id>` when the context carries one.

## Upstream service modules

Each upstream service has its own client package under `internal/`:

| Package | Upstream | Notes |
|---------|----------|-------|
| `entity` | Multiple entity services (see below) | Hosts `CustomerEntityClient` (this repo's entity-service; most case/account/project endpoints, raw `[]byte` passthrough, plus `CreateEventPublishFailure` — see `eventpublisher` below) and `EngineeringEntityClient` (a separate internal engineering entity service; `CreateGitIssue`, typed request/response). **`EngineeringEntityClient` is not wired into `cmd/server/main.go` yet** — no handler calls it |
| `scim` | SCIM service | User/group lookups |
| `updates` | Updates service | Product update levels; returns typed structs (not raw passthrough) |

New upstream services get their own package under `internal/` following the same `Client` + `do()` pattern. `entity` is the exception: because it hosts multiple, separately-deployed/differently-authenticated services, it uses a `<Name>Config`/`<Name>Client` pair per service/file instead of one shared `Client` for the whole package — `CustomerEntityClient`/`EngineeringEntityClient`.

**Notifications (email, Google Chat, and future channels like SMS/voice-Twilio) have moved out of this backend** into `integrations/csm-notification-service`, a standalone Go service invoked over HTTP. This backend no longer constructs or calls any notification client directly — instead, `internal/eventbus`/`internal/events`/`internal/eventpublisher` handle Azure Event Hub's `case-events` topic, which that service (and now this backend itself) reacts to.

- **`internal/events`** — `Envelope{Type, EntityID, Payload}`, the wire shape of every record on `case-events`, kept in sync by hand with csm-notification-service's own `internal/events.Envelope` (separate Go modules, neither imports the other). Shared by both directions below.
- **`internal/eventbus`** — `Producer` (publish, wait for ack) and `Consumer` (join a named consumer group, poll, commit after `Handle` returns — see `Consumer.Run`'s doc comment for why it deliberately skips a retry/dead-letter policy, unlike csm-notification-service's own consumer). A distinct `groupID` gets its own full copy of every record, independent of any other consumer group on the same topic — that's how a second, unrelated reaction to the same events gets added without touching the producer or anything already consuming.
- **`internal/eventpublisher.Publisher.Publish(ctx, eventType, entityID, payload)`** — publishes a domain event, and if Event Hub doesn't acknowledge it, durably records the failure via `CustomerEntityClient.CreateEventPublishFailure` (`POST /event-publish-failures` on entity-service) rather than losing it silently. **Wired into `cmd/server/main.go`**, called asynchronously (see `CaseHandler.publishAsync` in `internal/handler/cases.go`) from `CreateCaseComment` (`case.comment_added`) and `PatchCase` when `state` changes (`case.status_changed`) — the only two event types anything currently consumes (see below).
- **`internal/recipientlinks.Resolver.ResolveLinks(ctx, emails, projectID, caseID)`** — for a `case.*` event's recipient list, resolves each recipient's *own* case-portal link based on their role (`CustomerEntityClient.SearchUsersByEmail` looks up each email's roles/userType), since a single event's recipients can be a mix of customer and CSM-role people who each need a different portal's link — a customer gets `<CUSTOMER_PORTAL_WEB_BASE_URL>/projects/{projectId}/support/cases/{caseId}`, everyone else gets `<CSM_PORTAL_WEB_BASE_URL>/cases/{caseId}`. Role-list membership (`CUSTOMER_ROLES`/`CSM_ROLES`) is the primary signal; `userType` is the fallback when a recipient's roles match neither configured list; the CSM link is the last-resort default (including when entity-service has no record for the recipient at all). **Not yet wired into `cmd/server/main.go`** — same status as `EngineeringEntityClient` above; the next step is calling it from whichever handlers create/update cases, comments, and incidents.
- **`internal/caseevents.Handler`** — the consumer side, **wired into `cmd/server/main.go`** (started whenever `EVENT_HUB_BROKER` is set, in its own consumer group — `EVENT_HUB_CONSUMER_GROUP`, suffixed `-replica-<id>` per process — the container/pod hostname when available (stable across a plain restart within the same pod; see `newReplicaID`), a random UUID only as fallback — so every replica sees every event rather than Kafka load-balancing them across replicas). Logs `type`/`entityId` for every event (never the payload — it can carry PII, e.g. comment text or recipient emails), and for `case.comment_added`/`case.status_changed` specifically also fans a minimal `{caseId, type, timestamp}` payload out to `internal/stream.BroadcastHub` — see "Case-activity SSE stream" below, which is what that scaffold grew into.

### Case-activity SSE stream (`GET /cases/{id}/activities/stream`, port `:9092`)

Real-time push for the case Activities tab (new comment / status change), so the frontend doesn't rely solely on its own query `staleTime`/manual refresh. Three pieces, all optional together — nothing here runs unless `EVENT_HUB_BROKER` is set, same gating as the rest of the Event Hub wiring above:

- **`internal/stream.BroadcastHub`** — in-process pub-sub, `map[string]map[chan string]bool` keyed by case ID. `Register`/`Unregister`/`Publish`, all mutex-guarded. Only knows about subscribers on *this* replica — see `internal/caseevents.Handler`'s per-replica consumer group above for why every replica needs its own full copy of every event to broadcast to whichever subscribers it happens to be holding.
- **`CaseHandler.StreamCaseActivities`** (`internal/handler/case_stream.go`) — the SSE handler: authorizes the caller can read the requested case (same upstream `GetCase` ACL check every other case-reading endpoint relies on) before registering with the hub, writes a `event: case_updated` frame per broadcast plus a `: ping` heartbeat every 15s, unregisters on `ctx.Done()`. Behind the same `middleware.Auth` chain as every other endpoint — **no separate ticket/token-exchange step**; a browser EventSource polyfill that supports custom headers (native `EventSource` cannot) connects with the same `x-jwt-assertion`/`x-user-id-token` headers `useAuthApiClient` already sends everywhere else.
- **Dedicated `:9092` listener** (`STREAM_PORT` env var, default `9092`; see `cmd/server/main.go`) — a long-lived SSE connection would be killed by the main `:8080` listener's `WriteTimeout`/`IdleTimeout` (30s/60s), so it gets its own `http.Server` with both set to `0`, same shape as `apps/customer-portal/backend-v2`'s WebSocket listener. Its own Choreo endpoint (`.choreo/component.yaml`, `openapi-stream.yaml` — kept separate from the main `openapi.yaml` since that spec describes the `:8080` surface only).
- **Both listeners run `middleware.CORS`** (`CORS_ALLOWED_ORIGINS`/`STREAM_CORS_ALLOWED_ORIGINS` env vars, comma-separated; unset allows any origin) — `SecurityHeaders` stays outermost so its headers are present on every response including a preflight, with `CORS` wrapping everything below it (still ahead of `Auth`, which a preflight's missing `x-jwt-assertion` would otherwise fail — see `middleware.CORS`'s doc comment). In a real deployment Choreo's gateway supplies CORS itself, making this in-app version a no-op there; it matters for local development, where the browser calls a listener directly with no gateway in front, and as defense-in-depth for the separately-declared stream endpoint, which isn't guaranteed to inherit the gateway's CORS handling the same way the long-established `:8080` one does.

**Shared OAuth2 credentials**: `CustomerEntityClient` and `EngineeringEntityClient` (both in `entity`), `updates`, and `scim` all authenticate as the same OAuth2 client-credentials app — `cmd/server/main.go` reads `OAUTH2_CLIENT_ID`/`OAUTH2_CLIENT_SECRET`/`OAUTH2_TOKEN_URL` once and passes them into every service's `Config`; only `<SERVICE>_BASE_URL`/`<SERVICE>_SCOPES` are per-service. `EngineeringEntityConfig` still has its own `ClientID`/`ClientSecret`/`TokenURL` fields (matching every other service's `Config` shape), but when it's wired into `main.go` those should be filled with the same shared `oauth2ClientID`/`oauth2ClientSecret`/`oauth2TokenURL` values, not new `ENGINEERING_ENTITY_*` env vars. Follow this pattern for any new upstream service client unless it genuinely uses a different OAuth2 app.

## Running locally

```bash
# from apps/csm-portal/backend
go run ./cmd/server/main.go
```

The server auto-loads `.env` from the working directory at startup (silently ignored if absent). No need to `source .env` manually.

## Commands

```bash
make setup   # wire up git hooks (once after clone)
make test    # vet + race-detector tests
make build   # runs tests then compiles ./cmd/server
```

Tests run automatically on `git push` via the pre-push hook.

## Adding a new endpoint

Follow these steps in order:

1. **Upstream client** (`internal/<module>/`) — add a method on `Client` that calls `c.do()`; use `url.PathEscape()` for every path parameter
2. **Handler interface** — extend the local interface in the relevant handler file (e.g. `entityCaseClient` in `cases.go`); keep it minimal — only methods that handler actually calls
3. **Handler func** — auth check → path/body guards → call client → `mapUpstreamErrorGeneric` on failure (see Handler conventions below for the one PATCH-handler exception) → write response
4. **Route** (`cmd/server/main.go`) — register using Go 1.22 method-prefixed patterns: `"POST /cases/{id}/comments"`
5. **OpenAPI spec** (`openapi.yaml`) — add the path with 200/400/401/403/404/500 responses; `403` is always required because `mapUpstreamError`/`mapUpstreamErrorGeneric` can return it
6. **Tests** — add handler tests; update the mock in `helpers_test.go` to satisfy the extended interface
7. **gosec** — run `gosec -fmt=text ./...` (see README's Security Scanning section) before opening the PR; it must report 0 issues

## Handler conventions

- **Auth**: always check `middleware.UserInfoFromContext(r.Context()) == nil` first → 401
- **Body size**: cap with `http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)` (1 MiB) before reading
- **Path params**: guard against empty string after `r.PathValue("id")`; if the param is a UUID, also validate format using the package-level `uuidRe` compiled regex and return 400 on mismatch — fail fast before calling the upstream
- **Field naming**: case create/patch use bare names without `Key`/`Keys` suffix — `state`, `severity`, `workState` (PATCH), `type`, `severity`, `issueType` (POST); search filters use `states`, `severities`, `types`, `issueTypes`, `engagementTypes`; deployment search uses `deploymentTypes`; case comments use `type` (not `typeKey`); case create accepts `type: "case"`, `"service_request"`, or `"security_report_analysis"` (ServiceNow only for the latter two)
- **Deployment ID injection**: two helpers exist in `deployments.go` — `injectDeploymentID` (injects `deploymentIds: [id]` array, used by search) and `injectDeploymentIDField` (injects `deploymentId: id` string, used by create/update). Use the correct one for the endpoint's upstream contract.
- **Upstream errors**: use `mapUpstreamErrorGeneric(w, err, "<fallback message>")` for every endpoint by default — never write custom status mappings inline. Only the ten PATCH/update handlers (`PatchCase`, `PatchCallRequest`, `PatchMe`, `UpdateProject`, `PatchDeployment`, `PatchDeployedProduct`, `PatchChangeRequest`, `UpdateTimeCard`, `UpdateTask`, `PatchIncident`) use `mapUpstreamError` instead, which surfaces the upstream 400/409/422 reason (e.g. "Invalid state transition") — appropriate there because the request body just submitted is what's being rejected. Every other endpoint (search/create/get/delete) forwards a payload that's only partially validated at this layer, so a 4xx from upstream isn't reliably something the caller could have avoided; `mapUpstreamErrorGeneric` returns the fixed fallback message for those instead of echoing upstream detail. Both log the full reason via the caller's `slog.ErrorContext(ctx, ..., "err", err)` regardless of which is used.
- **Response**: return raw `[]byte` with `writeJSON` for simple passthroughs; unmarshal into typed structs only when the response shape needs to change

## OpenAPI spec

**`openapi.yaml` must be updated whenever the API changes** — new endpoints, removed endpoints, changed request/response shapes, new error codes. It is the contract consumed by the frontend and other teams; an out-of-date spec is worse than no spec.

- Error responses use `$ref: '#/components/schemas/ErrorPayload'`
- Every endpoint must declare a `403` response
- Path parameters that expect UUIDs must declare `format: uuid` on the schema
- The `Case` schema includes a computed `nextStates` read-only field populated server-side from `state`
- Binary-download endpoints (e.g. attachment content) must document the `Content-Disposition: attachment` and `X-Content-Type-Options: nosniff` security headers in their description

## Response shape

- For **portal-owned/transformed** responses (typed structs constructed by the portal), all JSON fields must use **camelCase** (e.g. `createdAt`, `projectId`, `issueType`); use `json:"fieldName"` struct tags to enforce this
- **Raw passthrough** responses may retain upstream field naming as-is — do not reshape them unless there is an explicit requirement to do so
- The `nextStates` field is portal-constructed and follows camelCase like all other portal-owned fields

## Security

- **Never commit secrets** — API keys, tokens, passwords, and service URLs with credentials must not appear in source code or config files; use environment variables
- **No sensitive data in logs** — do not log request bodies, JWT payloads, or user PII; log only IDs and error summaries
- **JWT is the only auth mechanism** — all endpoints must validate the caller via `middleware.UserInfoFromContext`; there are no public endpoints
- **Audience** — `Config.Audiences` is `[]string`; a token is accepted if its `aud` claim contains **any** of the configured values (OR logic). Set via `AUTH_AUDIENCE` as a comma-separated string
- **Input validation** — validate and reject unexpected input at the boundary (path params, body size, JSON structure) before forwarding to upstream services
- **Error messages** — never leak upstream error details or stack traces to the caller; use the fixed `ErrMsg*` constants or a short fallback message. This is what `mapUpstreamErrorGeneric` enforces by default; see the Handler conventions section for the narrow PATCH-handler exception that uses `mapUpstreamError` instead
- **Security fixes in PRs** — when a change is made to fix a security issue (gosec findings, input sanitization, etc.), do not mention it in the PR title or description; describe the change in neutral functional terms only
- **Run gosec on every backend change** — `gosec -fmt=text ./...` (install once: `go install github.com/securego/gosec/v2/cmd/gosec@latest`) must report 0 issues before opening a PR touching this backend; fix the root cause of any finding rather than suppressing it, unless a `#nosec` annotation with a justification comment already covers that exact case

## Testing

- Mocks live in `internal/handler/helpers_test.go` — when you extend a handler interface, add the new field and method to the mock there
- `upstreamErrors(fallback)` is the error table for the ten `mapUpstreamError` PATCH-handler tests (surfaces the upstream 400/409/422 reason); `upstreamErrorsGeneric(fallback)` is its counterpart for every other handler's tests, which call `mapUpstreamErrorGeneric` and always expect `fallback` for those statuses instead
- `withUser()` injects a test user into the request context
- `decodeJSON[T]()` decodes response bodies in assertions
- Use real UUIDs (e.g. `"11111111-1111-1111-1111-111111111111"`) for UUID path param test values — not fake slugs like `"case-1"`
