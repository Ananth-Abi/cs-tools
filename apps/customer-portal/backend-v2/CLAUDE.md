# Customer Portal Backend (v2)

Go HTTP server (`net/http`, Go 1.26+) that acts as a backend-for-frontend (BFF) for the customer
portal. It authenticates callers, forwards requests to `cs-tools/entity-service`, and shapes
responses for the frontend. This is a Go rewrite of the Ballerina backend at
`apps/customer-portal/backend`, modeled on `apps/csm-portal/backend`'s conventions — read that
backend's own CLAUDE.md too if something here is underspecified.

**Status: in progress.** 50 routes are wired up so far (`GET /health`, `GET`/`PATCH /users/me`,
`POST /accounts/search`, `GET /accounts/{id}`, `POST /projects/search`, `GET /projects/{id}`,
`POST /cases/search`, `GET /cases/{id}`, `POST /cases`, `PATCH /cases/{id}`,
`POST /cases/{id}/comments`, `POST /cases/{id}/activities/search`, `POST /deployments/search`,
`POST /deployments`, `PATCH /deployments/{id}`, `POST /deployed-products/search`,
`POST /deployed-products`, `PATCH /deployed-products/{id}`, `POST /attachments`,
`POST /attachments/search`, `GET /attachments/{id}/content`, `DELETE /attachments/{id}`,
`POST /products/search`, `POST /products/{id}/versions/search`,
`POST /products/vulnerabilities/search`, `GET /products/vulnerabilities/{id}`,
`POST /catalogs/search`, `GET /catalogs/{catalogId}/items/{catalogItemId}/variables`,
`POST /time-cards/search`, `POST /comments`, `POST /comments/search`, `POST /change-requests`,
`POST /change-requests/search`, `GET /change-requests/{id}`, `PATCH /change-requests/{id}`,
`GET /change-requests/{id}/approvals`, `POST /change-requests/{id}/approvals/decision`,
`POST /call-requests`, `POST /call-requests/search`, `PATCH /call-requests/{id}`,
`POST /cases/classify`, `POST /conversations/recommendations/search`,
`POST /projects/{id}/conversations/search`, `GET /conversations/{id}/messages`,
`POST /projects/{projectId}/conversations/{conversationId}/messages`,
`GET /projects/{id}/conversations/{conversationId}/summary`, `GET /ws`,
`POST /projects/{projectId}/deployments/{deploymentId}/license`, `POST /deployment-usages`,
`GET /updates/product-update-levels`, `POST /updates/levels/search`) across five upstream
services: entity-service, the WSO2 Updates service, SCIM, the AI chat agent, and the
product-consumption service (see "The AI chat agent" and "The product-consumption service" below —
unlike the other three, neither is entity-service-backed at all). The Ballerina backend exposes
~100 routes across many more modules (registry tokens, escalations, incidents, problems, task
SLAs, tasks, groups/service-offerings/configuration-items, account/project contacts, project
update, generic user search, project/case/deployment/conversation/time-card stats, case feedback,
instance search/metrics, global search, etc.) — none of those are ported yet, several because they
have no genuine equivalent on `cs-tools/entity-service` (confirmed by grepping
`entity-service/internal/server/routes.go` for each — stats, feedback, instance metrics, escalations,
and global search all come up empty) or aren't actually customer-portal features at all (see "Which
entity-service" below on how to tell the difference before porting one). Follow the recipe below to
add the next one.

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

**Existing on `cs-tools/entity-service` is necessary but not sufficient — also confirm the
Ballerina backend actually exposes it as a customer-portal feature.** `cs-tools/entity-service`
implements plenty of routes this backend should *not* port: some are genuinely customer-facing but
belong to a different portal (e.g. `POST /cases/{id}/github-issues` — filing an engineering bug
against an internal repo is a support-agent action with zero precedent anywhere in the Ballerina
customer-portal backend), and some read like customer features but the Ballerina backend actually
serves the equivalent from an entirely different, non-`cs-tools` microservice (e.g.
`POST /accounts/{id}/contacts/search` / `POST /projects/{id}/contacts/search` look like read
analogues of the Ballerina backend's project-contact endpoints, but those are actually backed by
the separate `user_management` module/microservice, not entity-service at all — porting the
`cs-tools/entity-service` version would expose a different, unrelated dataset under a
similar-looking URL). Before implementing anything new, grep
`apps/customer-portal/backend/modules/entity/entity.bal` (or the relevant sibling module) for the
Ballerina function that would call it, and check what it actually resolves to — a route only earns
a place in this backend once both checks pass.

## Other upstream services

Not everything comes from entity-service. Two more upstream service clients exist, each following
the same `Config{BaseURL, TokenURL, ClientID, ClientSecret, Scopes}` + `Client` + `NewClient` +
private `do()` shape as `internal/entity`:

- **`internal/updates`** — the WSO2 Updates service (product update levels, update descriptions
  between levels). Its own types are already portal-shaped camelCase (`internal/updates/types.go`
  defines both the upstream snake_case wire structs and the portal camelCase structs, translated by
  `internal/updates/mapper.go`) — so handlers write its return values directly via `writeJSONValue`
  with **no** further `internal/dto` mapping layer. This is the one deliberate exception to the
  "always map through dto" rule below, because the mapping already happened inside the client.
- **`internal/scim`** — the SCIM operations service, used only for a user's phone number
  (`SearchUser`, `UpdateUserPhone`). Its `UserInfo` return type is already a small portal-clean
  struct, so it's merged directly into `dto.UserMeResponse`/`dto.UserUpdateResponse` in
  `internal/handler/users.go` — again no separate mapping layer needed.

All four service clients (entity, updates, SCIM, the AI chat agent) authenticate as the same shared
OAuth2 client-credentials app in `cmd/server/main.go` — only each service's `*_BASE_URL`/`*_SCOPES`
env vars differ (confirmed against the Ballerina backend's `Config.toml`, where every module,
including the AI chat agent's WebSocket variant, declares the same `clientId`/`tokenUrl`).

## The AI chat agent

`internal/aichatagent` is a fourth upstream client, but unlike entity/updates/SCIM it talks to a
**separate Python service that has no relationship to `cs-tools/entity-service` at all** — see
`apps/customer-portal/backend`'s `modules/ai_chat_agent` for the Ballerina backend's equivalent
client. It has its own HTTP API (`internal/aichatagent/client.go`) and a distinct WebSocket
endpoint (`internal/aichatagent/ws.go`, using `github.com/gorilla/websocket` — the one third-party
dependency in this otherwise-stdlib-only backend, since `net/http` has no server-side WebSocket
support).

`internal/handler/ai_chat.go` (case classification, KB recommendations, conversation search,
conversation messages, conversation summary) and `internal/handler/websocket.go` (the real-time
chat proxy) both mix calls to the AI agent with calls to entity-service's conversation/comment
routes — mirroring the Ballerina backend's own design (a conversation thread lives in
entity-service; the AI agent only handles the live message exchange). Three entity-service gaps
block a full 1:1 port, all flagged with `TODO(entity-service)` at their call sites rather than
worked around — do not build a workaround for these; wait for the actual entity-service methods:

- **No `createConversation` / `updateConversation`** — only `POST /conversations/search` exists
  (see `internal/entity/conversations.go`). This blocks: starting a brand-new AI chat conversation
  (`POST /projects/{id}/conversations` from the Ballerina backend is not ported at all — there is no
  way to mint a conversation ID), and marking a conversation `resolved` when the AI agent reports
  `resolved: true` (skipped in both `internal/handler/ai_chat.go`'s `SendConversationMessage` and
  `internal/handler/websocket.go`).
- **No `createdBy` override on `entity.CreateCommentRequest`** — the Ballerina backend attributes
  the AI agent's own reply to a special "chat agent" identity when saving it as a comment;
  `cs-tools/entity-service` always attributes a created comment to the caller's own authenticated
  identity. Since there is no way to correctly attribute the AI's reply, this backend does **not**
  save it as a comment at all (only the customer's own message is saved, since that attribution is
  correct as-is) — saving it under the wrong identity would be worse than not saving it.

`GET /ws?sessionId={projectId}` keeps the Ballerina backend's query parameter name for wire
compatibility even though it actually carries the *project* ID, not a session ID — the AI agent's
own per-conversation session key is derived as `"{projectId}:{conversationId}"` inside the handler.
Because a brand-new conversation can't be created (see above), this endpoint only supports
*resuming* an existing conversation — the browser must supply a `conversationId` in its first
message, or the handler returns an `error` event rather than silently failing. One simplification
versus the Ballerina backend: Go's `http.Server` runs each upgraded connection in its own
goroutine, and the handler's `ReadMessage` → `handleMessage` loop is a single blocking sequence —
it does not read or process the next frame until `handleMessage` returns, and never starts a
concurrent read or upstream stream. So there's no need for the Ballerina implementation's explicit
"already streaming" busy-flag/mutex; the client can still send another frame at any time, this
handler simply won't look at it until the current one finishes.

Primary authorization on `GET /ws` is the same JWT middleware chain as every other route.
`WebSocketHandler`'s `gorilla/websocket.Upgrader.CheckOrigin` adds a defense-in-depth check against
cross-site WebSocket hijacking, restricting which browser `Origin`s may open the connection — set
via the optional `WS_ALLOWED_ORIGINS` env var (comma-separated; unset allows any origin, local
development only).

## The product-consumption service

`internal/productconsumption` is a fifth upstream client for **another separate service unrelated
to entity-service** — see `apps/customer-portal/backend`'s `modules/product_consumption_subscription`
and `modules/product_consumption_tracking` for the Ballerina backend's two client modules, which
this backend models as one Go package since both Ballerina modules point at the same upstream base
URL in practice (confirmed in the Ballerina backend's `config.toml`).

It backs two routes:
- `POST /projects/{projectId}/deployments/{deploymentId}/license` — provisions (or resumes
  provisioning) a WSO2 API Manager application/subscription/credentials for a deployment and
  returns the resulting license. `ProcessLicenseDownload` (`internal/productconsumption/subscription.go`)
  is a straight port of the Ballerina backend's `processLicenseDownload` state machine — it is
  **not idempotent-by-accident-safe to reimplement casually**: it can make up to 5 sequential
  upstream calls, several with side effects (creating an application, subscribing it, generating
  credentials), and each step only runs if the project's upstream-tracked status hasn't reached it
  yet. Read the whole function before touching it — a subtly wrong condition could create a
  duplicate WSO2 API Manager application. The handler first calls `entity.GetProject` purely as an
  access-control gate (mirroring the Ballerina backend), discarding the result — entity-service is
  still the actual authorization boundary for "does this caller own this project."
- `POST /deployment-usages` — imports a deployment-usage zip file. Unlike every other endpoint in
  this backend, the request body is **raw binary**, not JSON — see `readBinaryBody` in
  `internal/handler/response.go` (a `readJSONBody` counterpart with a larger size cap,
  `maxZipUploadBytes`) and the `Content-Type: application/zip`/`application/x-zip-compressed`
  check in the handler, both mirroring the Ballerina backend's `validateDeploymentUsageImportRequest`.
  The Go client base64-encodes the bytes before forwarding to the upstream service, matching its
  JSON contract exactly (`{"email": "...", "zip": "<base64>"}`).

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

**Data-source normalization is a second job of the DTO layer.** Unlike projects and cases,
entity-service's account endpoints (`GET /accounts/{id}`, `POST /accounts/search`) return a
genuinely different wire shape depending on whether it's deployed with `DATA_SOURCE=postgres` or
`DATA_SOURCE=servicenow` — see `internal/entity/types.go`'s `AccountDetail`/`AccountSummary`
comments for how the two shapes are unioned into one Go struct (their JSON keys never collide) and
`internal/dto/account.go` for how the DTO mapper picks whichever fields the active data source
populated (e.g. `Tier` prefers entity-service's `tier`, falling back to `classification`) to
produce one consistent contract for the frontend regardless of which data source is live. This is
a genuine advantage of the DTO-mapping convention over raw passthrough — `apps/csm-portal/backend`
has to model this as an OpenAPI `oneOf` in its spec and pass the ambiguity on to the frontend;
here, it's resolved once in the mapper. Also deliberately excluded from account DTOs: `ArrToday`
(annual recurring revenue — WSO2-internal financial data, never expose to the customer) and `Pod`
(WSO2-internal account routing). `POST /products/search` and `POST /products/{id}/versions/search`
(`internal/entity/types.go`'s `ProductView`/`ProductVersionView`) use the exact same superset-struct
technique — e.g. `ProductVersionView.ReleaseDate` is typed `*string` even though the Postgres shape
is `time.Time` and the ServiceNow shape is a plain string, because a Go `*string` field decodes a
JSON string value regardless of which the source type actually was.

**When the Ballerina backend exposes a thinner shape than entity-service's own struct, match the
Ballerina backend, not entity-service.** `POST /time-cards/search`'s `dto.TimeCardSummary` excludes
entity-service's per-category time breakdowns (`timeAnalyzing`, `timeSettingUp`, etc.),
`issueComplexity`, `workLogComment`, `rejectionReason`, and the eligible-approvers list — not
because any of them are individually dangerous, but because the Ballerina backend's own `TimeCard`
type (`apps/customer-portal/backend/modules/entity/types.bal`) never exposed them either. When
porting an endpoint, read the Ballerina backend's response type for the equivalent feature, not just
the request/response pair on `cs-tools/entity-service` — the Ballerina shape is itself a design
decision about what a customer should see, and entity-service's superset shouldn't leak through it
by default.

**`json.RawMessage` on a request field usually means "preserve three states," not "skip validation."**
`entity.UpdateDeployedProductRequest.Description` is `json.RawMessage` specifically so entity-service
can distinguish "field absent" (omit), `"description": null` (clear the value), and
`"description": "value"` (set it) — a plain `*string` can't represent "absent vs. explicitly null."
When a portal request decodes straight into a struct with such a field (no restricted DTO needed
here, since Cores/TPS/Description/Active are all customer-appropriate), the raw bytes the client
sent pass through unchanged and this three-state semantic is preserved automatically — don't
"simplify" the field to `*string` when porting a similar endpoint.

**Binary responses use a `doBinary`-shaped client method, not `getJSON`/`postJSON`.**
`GET /attachments/{id}/content` returns a raw file, not JSON — `entity.Client.doBinary` (added in
`internal/entity/client.go` alongside `do`) returns `(body []byte, contentType string, error)`, and
the handler (`internal/handler/attachments.go`) writes `Content-Type` from entity-service's own
(already-sanitized) header value and explicitly sets `Content-Disposition: attachment` itself —
never render an attachment inline, since entity-service's own allowlist coercion to
`application/octet-stream` for unrecognized types is a stored-XSS mitigation this backend must not
undo by, say, echoing a client-supplied filename into a `Content-Disposition` you construct
yourself.

Request bodies are usually the exception: incoming search/filter/create payloads are decoded
directly into the entity package's request structs (e.g. `entity.SearchProjectsRequest`,
`entity.CreateCaseRequest`) with no separate DTO layer, since those shapes are already what the
frontend needs to send and every field is customer-appropriate — there is nothing to hide.

**But when entity-service's request contract mixes customer-safe fields with internal-only ones,
build a restricted portal request DTO too.** `entity.UpdateCaseRequest` (`PATCH /cases/{id}`) is
the example: it has 18 optional fields, but `workState`, `assigneeEmail`, `parentId`/
`relatedCaseId`/`deploymentId`/`deployedProductId` (case relinking), `autocloseHoldUntil`, and the
`fixEta`/`bestCaseFixEta`/`mostLikelyFixEta`/`worstCaseFixEta` quartet are internal WSO2 support
operations, not things a customer should be able to set on their own case. `dto.UpdateCaseRequest`
(`internal/dto/case.go`) exposes only the customer-safe subset (state, severity, subject,
description, watchList, resolutionCode, cause, closeNotes), and
`dto.BuildEntityUpdateCaseRequest(id, req)` builds the full entity-service request from it, leaving
every excluded field zero/nil — so even if a client sends `{"workState": "ongoing"}`, it's silently
dropped (the portal struct has no such field) rather than forwarded. The same pattern applies to
`POST /cases/{id}/comments`: entity-service accepts `type: work_note|comment|activity`, but
`dto.BuildEntityCreateCaseCommentRequest` always forces `type: comment`, regardless of what the
client sends — a customer should never be able to create an internal work-note or system-activity
entry. When you add a write endpoint, check the entity-service request struct for fields that read
as "internal support operation" rather than "customer self-service action" before deciding whether
to pass it through directly or build a restricted DTO.

Two more examples, both in this same "restrict, don't mirror" category:
- `PATCH /change-requests/{id}` (`dto.ChangeRequestUpdateRequest`) excludes case/project/deployment
  relinking and `assignedEngineerId`/`assignedTeamId` (support assignment) for the same reasons as
  case updates, plus `state` specifically — state transitions go through the dedicated
  `isCustomerApproved`/`isCustomerReviewed`/`requestApproval` fields (which *are* exposed, since
  they're literally the customer's own approval actions) rather than letting the customer set an
  arbitrary ServiceNow workflow state directly.
- `PATCH /call-requests/{id}` (`dto.CallRequestUpdateRequest`) excludes `meetingDate`/`assignee`/
  `notes`/`plan`/`attendees`/`actionItems`/`actualDurationMin` — entity-service's own doc comment
  labels these "agent-side fields, set when an engineer schedules or concludes the call." They're
  still exposed on the *read* side (`dto.CallRequestSummary`) since the customer should be able to
  see the outcome of their own call, just not set it themselves.
- `POST /comments` / `POST /comments/search` (generic comments — distinct from `POST /cases/{id}/comments`,
  these attach to any reference entity: case, conversation, change_request, deployment, incident)
  restrict in *both* directions: `dto.BuildEntityCreateCommentRequest` forces `type: comment` on
  write for the same reason as case comments, and `dto.BuildEntitySearchCommentsRequest` forces
  `filters.type: comment` on **read** too — entity-service's search endpoint returns `work_note`
  entries verbatim unless the caller filters them out, and those are internal WSO2 annotations that
  must never reach the customer regardless of which reference entity they're attached to.

**Not every field worth restricting is a security decision — some are just an unenforced entity-service
scoping convenience.** `PATCH /deployed-products/{id}`'s `deploymentId` field looks similar to the
fields above (an id referencing another resource) but isn't restricted, because entity-service
documents it as an IDOR-style scope guard the caller supplies voluntarily (verify the deployed
product belongs to this deployment before mutating it), not a way to relink the resource. Read the
entity-service doc comment on the field before deciding which category it falls into — don't
pattern-match on field name alone (e.g. "any ID field" or "any assignee-shaped field").

**"Exactly one" vs. "at least one" is per-entity, read entity-service's own doc comment.**
`PATCH /cases/{id}` requires *exactly one* of its primary fields (entity-service's doc comment says
so explicitly); `PATCH /change-requests/{id}` requires *at least one* (its doc comment says "at
least one must be provided"). Don't assume one pattern generalizes to the other — copying the wrong
validation produces a portal that's stricter or looser than the upstream contract, and CodeRabbit
caught exactly this mismatch once already (see the case-update fix in this backend's PR history).

Where entity-service defines many enum-like fields (change request category/priority/impact/type/
state/risk, call request state, etc.) as named Go string types with const blocks, this file's
convention is to flatten them to plain `string` in the mirrored struct (matching how `CaseView`
already treats `Severity`/`State` as plain strings) — skip re-declaring the const blocks unless a
specific value needs to be checked in Go code (e.g. `ChangeRequestApprovalDecisionRequest.Decision`
validation in the handler compares against literal `"approved"`/`"rejected"` strings, not enum
constants).

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
8. **OpenAPI spec** (`openapi.yaml`) — add the path with `200`/`400`/`401`/`403`/`500` responses
   (`404` too for get-by-id, `413` too for endpoints with a request body); every endpoint must
   declare `403` since `mapUpstreamError` can return it.
9. **gosec** — run `gosec -fmt=text ./...` (must report 0 issues) before opening a PR.

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
