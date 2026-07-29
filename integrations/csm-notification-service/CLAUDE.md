# CSM Notification Service

Go HTTP server (`net/http`, Go 1.26+) that other services call to request a notification (email, Google Chat, and future channels). Extracted from `apps/csm-portal/backend/internal/notifications` — that backend no longer owns or calls any notification client.

## Current scope — deliberately incomplete

`POST /notifications` (`internal/handler/notifications.go`, `PostNotification`) only validates the request body and responds `202 Accepted`. It does **not** call `EmailClient.SendEmail` or `GoogleChatClient.SendIncidentAlert` yet, and there is no Kafka/Redis dependency anywhere in this repo. This is intentional groundwork, not an oversight — see the `TODO` comment on `PostNotification`: once the Kafka-based event backbone (a separate, ongoing local POC) lands, this handler should publish the notification event to a message queue (producer) instead of a no-op, and a consumer (in this service or a peer) dispatches it via the same `emailClient`/`googleChatClient` fields already wired on `NotificationHandler`. Don't wire real sends into `PostNotification` without confirming whether the queue-based design has landed first.

## Why no `Auth` middleware

Like `integrations/csm-integration-service`, this service has no end-user identity to check — it's consumed by other backend services (and, later, a Kafka consumer) through Choreo's API Manager gateway, which owns the inbound trust boundary (subscription + client credentials) before a request ever reaches this app. Do not add inbound JWT/Bearer validation here without confirming that assumption no longer holds.

## Middleware chain

`SecurityHeaders → CorrelationID → Logger → Mux`

- `SecurityHeaders` (`internal/middleware/security_headers.go`): sets `X-Content-Type-Options: nosniff`, `Content-Security-Policy: upgrade-insecure-requests`, and `Strict-Transport-Security: max-age=31536000; includeSubDomains` on every response
- `CorrelationID` (`internal/middleware/correlation.go`): reads `X-CSM-Correlation-ID` from the incoming request or generates a UUID v4; ensures the ID carries a `cns-` prefix (CSM Notification Service) either way, without double-prefixing an ID that already has it; stores the ID in context for the slog handler; echoes the ID in the response header
- `Logger` (`internal/middleware/logger.go`): logs every completed request (method, path, status, elapsed) via slog

`middleware.ConfigureLogger()` must be called at startup — it wraps the default slog handler so every `slog.*Context(r.Context(), …)` call automatically includes `correlationID=<id>` when the context carries one.

## Notification channels

`internal/notifications` — one config/client pair per channel in its own file, since channels differ in upstream auth scheme:

- `EmailConfig`/`EmailClient`/`SendEmail` (`email.go`) — OAuth2 client-credentials auth against an external email notification service (`POST /send-email`)
- `GoogleChatConfig`/`GoogleChatClient`/`SendIncidentAlert` (`googlechat.go`) — a plain incoming-webhook URL per Google Chat space, no OAuth2 involved. `GoogleChatConfig.Spaces` is `[]GoogleChatSpace` (`{Product, WebhookURL}`), one space per product; `SendIncidentAlert` routes to the space matching `product` (case/whitespace-insensitive; unmatched returns an error, no fallback space)

Both clients are constructed in `cmd/server/main.go` with `os.Getenv` (never `mustEnv`) for their config, since neither channel is required for every deployment — a missing/invalid config only surfaces as an error the first time that channel is actually used.

A new channel (SMS/voice via Twilio is the next expected one, per the Kafka POC's architecture proposal) gets its own `<Name>Config`/`<Name>Client` file in `internal/notifications` following the same pattern, plus a new case in `PostNotification`'s `channel` switch (`internal/handler/notifications.go`) and a new payload struct alongside `emailNotificationPayload`/`googleChatNotificationPayload`.

## Request contract

`POST /notifications` takes a single discriminated-union body: `{"channel": "email"|"googleChat", "email": {...}, "googleChat": {...}}` — only the object matching `channel` needs to be populated. This shape (one endpoint, a `channel` field) is deliberate so a future Kafka consumer producing the same notification events doesn't need a new route — it can call the same internal validation/dispatch path `PostNotification` calls.

## Running locally

```bash
# from integrations/csm-notification-service
go run ./cmd/server/main.go
```

The server auto-loads `.env` from the working directory at startup (silently ignored if absent).

## Commands

```bash
go vet ./...              # vet
go test -race ./...       # vet + race-detector tests
go build -o server ./cmd/server   # compile
```

## Adding a new channel

1. **Client** (`internal/notifications/<channel>.go`) — add `<Name>Config`/`<Name>Client`/`Send<Thing>` following `email.go`'s or `googlechat.go`'s shape, whichever auth scheme is closer
2. **Handler struct** (`internal/handler/notifications.go`) — add a field on `NotificationHandler` for the new client, and a parameter on `NewNotificationHandler`
3. **Payload type** — add a `<channel>NotificationPayload` struct alongside the existing ones
4. **`sendNotificationRequest`** — add an `omitempty` field for the new payload, and a case in `PostNotification`'s `channel` switch validating its required fields
5. **`cmd/server/main.go`** — construct the new client from env vars (use `os.Getenv`, not `mustEnv`, unless the channel truly must be configured for the service to start)
6. **`openapi.yaml`** — add the new payload schema and extend `SendNotificationRequest`'s `channel` enum
7. **Tests** — add handler tests for the new channel's validation branch, following the existing `email`/`googleChat` cases

## Security

- **Never commit secrets** — client IDs/secrets, webhook URLs, and service URLs with credentials must not appear in source code or config files; use environment variables
- **No sensitive data in logs** — log only IDs and error summaries
- **No app-level inbound auth** — this is intentional (see above), not an oversight; don't "fix" it by bolting on JWT validation without confirming the Choreo gateway model has changed
- **Input validation** — validate and reject unexpected input at the boundary (body size, JSON structure, required fields) before any future dispatch or queue publish
- **Security fixes in PRs** — describe security-related changes in neutral functional terms only, not called out as security fixes in the title/description
