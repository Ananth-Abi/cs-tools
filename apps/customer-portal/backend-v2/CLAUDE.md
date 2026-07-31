# Customer Portal Backend (v2)

Go HTTP server (`net/http`, Go 1.26+) that acts as a backend-for-frontend (BFF) for the customer
portal. It authenticates callers, forwards requests to `cs-tools/entity-service`, and shapes
responses for the frontend. This is a Go rewrite of the Ballerina backend at
`apps/customer-portal/backend`, modeled on `apps/csm-portal/backend`'s conventions — read that
backend's own CLAUDE.md too if something here is underspecified.

**Status: in progress.** 26 routes are wired up so far (`GET /health`, `GET`/`PATCH /users/me`,
`POST /accounts/search`, `GET /accounts/{id}`, `POST /projects/search`, `GET /projects/{id}`,
`POST /cases/search`, `GET /cases/{id}`, `POST /cases`, `PATCH /cases/{id}`,
`POST /cases/{id}/comments`, `POST /cases/{id}/activities/search`, `POST /deployments/search`,
`POST /deployments`, `POST /deployed-products/search`, `POST /deployed-products`,
`PATCH /deployed-products/{id}`, `POST /attachments`, `POST /attachments/search`,
`GET /attachments/{id}/content`, `DELETE /attachments/{id}`, `POST /products/search`,
`POST /products/{id}/versions/search`, `GET /updates/product-update-levels`,
`POST /updates/levels/search`) across three upstream services: entity-service, the WSO2 Updates
service, and SCIM. The Ballerina backend exposes ~100 routes across many more modules (generic
comments, conversations, change requests, call requests, catalogs, time cards, registry tokens,
contacts, escalations, product vulnerabilities, AI chat/websocket, etc.) — none of those are ported
yet. Follow the recipe below to add the next one.

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

All three service clients (entity, updates, SCIM) authenticate as the same shared OAuth2
client-credentials app in `cmd/server/main.go` — only each service's `*_BASE_URL`/`*_SCOPES` env
vars differ.

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
