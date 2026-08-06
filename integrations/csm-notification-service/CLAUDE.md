# CSM Notification Service

Go HTTP server (`net/http`, Go 1.26+) that other services call to request a notification (email, Google Chat, SMS, voice calls, and future channels). Extracted from `apps/csm-portal/backend/internal/notifications` — that backend no longer owns or calls any notification client.

## Current scope — deliberately incomplete

`POST /notifications` (`internal/handler/notifications.go`, `PostNotification`) only validates the request body and responds `202 Accepted`. It does **not** call `EmailClient.SendEmail`, `GoogleChatClient.SendIncidentAlert`, `TwilioClient.SendSMS`, or `TwilioClient.MakeCall` yet, and there is no Kafka/Redis dependency anywhere in this repo. This is intentional groundwork, not an oversight — see the `TODO` comment on `PostNotification`: once the Kafka-based event backbone (a separate, ongoing local POC) lands, this handler should publish the notification event to a message queue (producer) instead of a no-op, and a consumer (in this service or a peer) dispatches it via the same `emailClient`/`googleChatClient`/`twilioClient` fields already wired on `NotificationHandler`. Don't wire real sends into `PostNotification` without confirming whether the queue-based design has landed first.

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
- `TwilioConfig`/`TwilioClient`/`SendSMS`+`MakeCall` (`twilio.go`) — HTTP Basic Auth (Twilio Account SID/Auth Token; Twilio has no OAuth2 flow). One client, two methods, since sms and call are the same Twilio account/auth, just different REST resources (`Messages.json` vs `Calls.json`) via a shared private `do()`:
  - `SendSMS(ctx, to, body)` sends via the client's configured `MessagingServiceSid` if set (our account's real setup — a sender pool with opt-out/compliance handling), else its fixed `FromNumber`. If both are set, `MessagingServiceSid` wins
  - `MakeCall(ctx, to, message)` places a voice call that reads `message` aloud via a `<Say>` TwiML document built locally (`sayTwiML`, no external TwiML hosting needed) — always requires `FromNumber` as caller ID, since Voice has no `MessagingServiceSid` equivalent, regardless of whether `MessagingServiceSid` is set for sms. `<Say voice="..." language="...">` come from `TwilioConfig.Voice`/`Language` (`TWILIO_VOICE`/`TWILIO_LANGUAGE`, both optional — empty omits the attribute and uses Twilio's account default)
  - `TwilioConfig.APIBaseURL` overrides Twilio's REST API base (`TWILIO_API_BASE_URL`, defaults to `defaultTwilioAPIBaseURL` when empty — resolved once in `NewTwilioClient`, not read per-request). Not just a test seam: tests set it to an `httptest.Server` URL the same way a real deployment could point it at a regional Twilio edge endpoint

All three clients are constructed in `cmd/server/main.go` with `os.Getenv` (never `mustEnv`) for their config, since none is required for every deployment — a missing/invalid config only surfaces as an error the first time that channel is actually used. Like `emailClient`/`googleChatClient`, `twilioClient` is held on `NotificationHandler` and validated against in `PostNotification`'s `channel` switch, but not yet called — see "Current scope" above.

A new channel gets its own `<Name>Config`/`<Name>Client` file in `internal/notifications` following the same pattern (or a new method on an existing client, if it shares that client's auth — as call does with sms), plus a new case in `PostNotification`'s `channel` switch (`internal/handler/notifications.go`) and a new payload struct alongside `emailNotificationPayload`/`googleChatNotificationPayload`/`smsNotificationPayload`/`callNotificationPayload`.

## Request contract

`POST /notifications` takes a single discriminated-union body: `{"channel": "email"|"googleChat"|"sms"|"call", "email": {...}, "googleChat": {...}, "sms": {...}, "call": {...}}` — only the object matching `channel` may be populated; `PostNotification` rejects a request that also carries a non-selected payload (e.g. `channel: "email"` with a `googleChat`, `sms`, and/or `call` object present), and requires the decoded body to end exactly after the JSON object (no trailing values). `openapi.yaml` mirrors this as a `oneOf`/`discriminator` schema (`SendEmailNotificationRequest`/`SendGoogleChatNotificationRequest`/`SendSmsNotificationRequest`/`SendCallNotificationRequest`), each with `additionalProperties: false`. This shape (one endpoint, a `channel` field) is deliberate so a future Kafka consumer producing the same notification events doesn't need a new route — it can call the same internal validation/dispatch path `PostNotification` calls.

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
- **TwiML injection** — `MakeCall`'s spoken message is caller-supplied text, not trusted markup. `sayTwiML` (`twilio.go`) builds the `<Say>` document by `encoding/xml`-marshaling a typed struct (`twimlResponse`/`twimlSay`), never raw string concatenation — a message containing TwiML-shaped text (e.g. `</Say><Redirect>...`) must render as literal spoken text, not inject a different verb. `Voice`/`Language` go through the same struct as `xml:"...,attr"` fields, so they get correct attribute escaping too, even though today they're only ever operator-supplied config, not request input. Keep using `encoding/xml` struct marshaling (not string concatenation) for any future TwiML-building code in this package
- **Security fixes in PRs** — describe security-related changes in neutral functional terms only, not called out as security fixes in the title/description
