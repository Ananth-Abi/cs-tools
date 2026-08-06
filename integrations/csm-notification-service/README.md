# CSM Notification Service

Go HTTP server (`net/http`, Go 1.26+) that accepts notification requests (email, Google Chat, SMS, voice calls, and future channels) from other services — `apps/csm-portal/backend`, `apps/customer-portal/backend`, and eventually a Kafka-based event consumer.

This service was extracted from `apps/csm-portal/backend/internal/notifications`, which previously hosted the same email/Google Chat clients. That backend no longer constructs or calls any notification client directly.

## Why no `Auth` middleware

Like `integrations/csm-integration-service`, this service has no end-user identity to check. It's consumed by other backend services (and, later, a Kafka consumer) through Choreo's API Manager gateway, which owns the inbound trust boundary (subscription + client credentials) before a request ever reaches this app.

## Current scope — TODO

`POST /notifications` today only validates the request body and responds `202 Accepted` — it does **not** send an email, post to Google Chat, send an SMS, place a call, or publish anywhere yet. See the `TODO` comment on `NotificationHandler.PostNotification` (`internal/handler/notifications.go`): once the Kafka-based event backbone is in place, this handler should publish the notification event to the message queue (producer) instead, so a consumer can dispatch it asynchronously via `emailClient`/`googleChatClient`/`twilioClient`. No Kafka or Redis dependency exists in this repo yet — this is a deliberate, incremental first step.

## Middleware chain

`SecurityHeaders → CorrelationID → Logger → Mux`

- `SecurityHeaders` (`internal/middleware/security_headers.go`): sets `X-Content-Type-Options: nosniff`, `Content-Security-Policy: upgrade-insecure-requests`, and `Strict-Transport-Security: max-age=31536000; includeSubDomains` on every response
- `CorrelationID` (`internal/middleware/correlation.go`): reads `X-CSM-Correlation-ID` from the incoming request or generates a UUID v4; ensures the ID carries a `cns-` prefix (CSM Notification Service) either way; stores the ID in context for the slog handler; echoes the ID in the response header
- `Logger` (`internal/middleware/logger.go`): logs every completed request (method, path, status, elapsed) via slog

`middleware.ConfigureLogger()` must be called at startup — it wraps the default slog handler so every `slog.*Context(r.Context(), …)` call automatically includes `correlationID=<id>` when the context carries one.

## Notification channels

| Package | Notes |
|---------|-------|
| `notifications` | Hosts `EmailClient`/`SendEmail` (`email.go`, OAuth2 client-credentials auth), `GoogleChatClient`/`SendIncidentAlert` (`googlechat.go`, per-product incoming-webhook auth), and `TwilioClient`/`SendSMS`+`MakeCall` (`twilio.go`, HTTP Basic Auth — sms and call are two methods on one client, since both are the same Twilio account/auth) |

Each channel gets its own config/client pair in its own file, since channels differ in upstream auth scheme. A new channel follows the same pattern and adds a case to `PostNotification`'s `channel` switch. Like `emailClient`/`googleChatClient`, `twilioClient` is constructed and held on `NotificationHandler` but not yet called from `PostNotification` — see [Current scope — TODO](#current-scope--todo).

## Configuration

Copy `.env.example` to `.env` and fill in the values:

### Email notification channel

| Variable | Description |
|---|---|
| `EMAIL_BASE_URL` | Base URL of the email notification service (optional) |
| `EMAIL_TOKEN_URL` | OAuth2 token endpoint for the email service (optional) |
| `EMAIL_CLIENT_ID` | OAuth2 client ID for the email service (optional) |
| `EMAIL_CLIENT_SECRET` | OAuth2 client secret for the email service (optional) |
| `EMAIL_SCOPES` | Comma-separated OAuth2 scopes (optional) |
| `EMAIL_FROM_ADDRESS` | Fixed "From" address used for every outgoing email (optional) |

### Google Chat notification channel

| Variable | Description |
|---|---|
| `GOOGLE_CHAT_SPACES` | JSON array of `{"product","webhookUrl"}` objects, one per Google Chat space. Optional — left unset or malformed, Google Chat alerts are unavailable but startup and every other endpoint work normally |

### SMS and call notification channels (Twilio)

| Variable | Description |
|---|---|
| `TWILIO_ACCOUNT_SID` | Twilio Account SID (optional) |
| `TWILIO_AUTH_TOKEN` | Twilio Auth Token (optional) |
| `TWILIO_MESSAGING_SERVICE_SID` | Twilio Messaging Service SID — preferred for sms, since this is how our account actually sends (optional) |
| `TWILIO_FROM_NUMBER` | Fixed Twilio-provisioned sending number, E.164 format. Used for sms only if `TWILIO_MESSAGING_SERVICE_SID` is unset; **always required for the call channel** — Voice has no Messaging Service equivalent (optional overall, but the call channel won't work without it) |
| `TWILIO_VOICE` | Call channel only: TTS voice for `<Say>` (e.g. `Polly.Raveena`). Optional — empty uses Twilio's account default voice |
| `TWILIO_LANGUAGE` | Call channel only: TTS language/locale for `<Say>` (e.g. `en-IN`), affects pronunciation. Optional — empty uses Twilio's default for the selected voice |

### Server

| Variable | Description |
|---|---|
| `PORT` | Server listen port — a plain number, not an address (default `8080`) |

## Project Structure

```text
csm-notification-service/
├── cmd/
│   ├── server/main.go           # Entry point — routes + server startup
│   └── twiliocheck/main.go      # Manual live-verification CLI (real SMS/call, not a test — see below)
├── internal/
│   ├── apierror/               # Typed upstream error type (4xx/5xx passthrough)
│   ├── middleware/
│   │   ├── correlation.go      # X-CSM-Correlation-ID propagation + slog enrichment
│   │   ├── logger.go           # Per-request access log
│   │   └── security_headers.go # X-Content-Type-Options, CSP, HSTS on every response
│   ├── notifications/
│   │   ├── doc.go              # Package overview — one config/client pair per channel
│   │   ├── email.go            # EmailConfig/EmailClient/SendEmail
│   │   ├── googlechat.go       # GoogleChatConfig/GoogleChatClient/SendIncidentAlert
│   │   └── twilio.go           # TwilioConfig/TwilioClient/SendSMS+MakeCall
│   └── handler/
│       ├── notifications.go    # POST /notifications — validates + accepts (TODO: publish to queue)
│       └── response.go         # writeError/writeJSONValue helpers
├── .env                         # Local config (git-ignored)
└── go.mod
```

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

## Manual live verification (`cmd/twiliocheck`)

`internal/notifications`'s `go test` suite runs entirely against a local
mock server — it never talks to a real Twilio account. `cmd/twiliocheck` is
a small standalone CLI for the times you actually need to confirm
`TwilioClient` works against a **real** account: it sends one real SMS or
places one real voice call and prints whether Twilio accepted it.

**This is not an automated test.** It is never run by `go test` or CI, makes
a real, billed request, and needs real credentials passed as environment
variables (never commit them):

```bash
# SMS, via a Messaging Service (preferred — see Configuration above)
TWILIO_ACCOUNT_SID=... TWILIO_AUTH_TOKEN=... \
TWILIO_MESSAGING_SERVICE_SID=... TWILIO_TO_NUMBER=+1... \
go run ./cmd/twiliocheck -channel=sms

# Voice call — always needs TWILIO_FROM_NUMBER, a voice-capable Twilio number
TWILIO_ACCOUNT_SID=... TWILIO_AUTH_TOKEN=... \
TWILIO_FROM_NUMBER=+1... TWILIO_TO_NUMBER=+1... \
go run ./cmd/twiliocheck -channel=call -voice=Polly.Raveena -language=en-IN
```

A `-message` flag overrides the default test message; `-voice`/`-language`
(call only) override `TWILIO_VOICE`/`TWILIO_LANGUAGE` for one run, to try a
voice without changing `.env`.

**A `202`/`"accepted"` result only means Twilio queued the request** — it is
not proof of delivery. Cross-check the actual outcome via Twilio's own API
(`GET /Calls/{Sid}.json` — `status`, `duration`; `GET /Messages.json?To=...`
— `status`, `error_code`) before trusting a "succeeded" print from this
tool. Two upstream errors we've hit doing exactly this:

- `21215` on `-channel=call`: the destination country isn't enabled under
  the Twilio console's **Voice** Geo Permissions.
- `21612` on `-channel=sms`, persisting even after enabling **Messaging**
  Geo Permissions for that country: a trial account's sole sender (a plain
  long code) often can't complete SMS delivery to "High Risk"-flagged
  destinations regardless of that toggle — this is an account-tier
  limitation (upgrade from Trial), not a config or code issue. Voice and
  SMS use different carrier interconnects, so one channel working doesn't
  imply the other does.

## API Endpoints

- `GET /health` — Health check
- `POST /notifications` — Submit a notification for dispatch; body requires `channel` (`email` | `googleChat` | `sms` | `call`) plus the matching `email`/`googleChat`/`sms`/`call` payload object. Currently validates and responds `202 Accepted` only — see [Current scope — TODO](#current-scope--todo)

## Security

- **Never commit secrets** — client IDs/secrets, webhook URLs, and service URLs with credentials must not appear in source code or config files; use environment variables
- **No sensitive data in logs** — log only IDs and error summaries
- **No app-level inbound auth** — this is intentional (see above), not an oversight
- **Input validation** — validate and reject unexpected input at the boundary (body size, JSON structure, required fields) before any future dispatch
- **Security fixes in PRs** — describe security-related changes in neutral functional terms only, not called out as security fixes in the title/description
