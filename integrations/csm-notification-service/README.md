# CSM Notification Service

Go HTTP server (`net/http`, Go 1.26+) that accepts domain events (case created, comment added, status changed, case assigned, incident created) from other services via `POST /events`, publishes them to an event bus, and reacts asynchronously by sending email, Google Chat alerts, and voice calls. Both the producer (the `POST /events` ingest endpoint) and the consumer (a background poll loop) run inside this one process — see [Event-driven notifications](#event-driven-notifications).

This service was extracted from `apps/csm-portal/backend/internal/notifications`, which previously hosted the same email/Google Chat clients. That backend no longer constructs or calls any notification client directly.

## Why no `Auth` middleware

Like `integrations/csm-integration-service`, this service has no end-user identity to check. It's consumed by other backend services through Choreo's API Manager gateway, which owns the inbound trust boundary (subscription + client credentials) before a request ever reaches this app.

## Current scope — TODO

- **No dead-letter queue** — a record whose handler keeps failing after retries is logged loudly and dropped, not preserved anywhere for replay. See `eventbus.Consumer`'s doc comment.
- **SMS and direct call channels are unused.** `TwilioClient.SendSMS` has no caller — `MakeCall` is only invoked by `incident.created`.

This service deliberately has no database connection and never talks to one directly — deduplicating a caller that retries a `POST /events` call, or two upstream callers racing to publish the same logical event, is `entity-service`'s job, via its `event_outbox` table. This service participates in that by calling `entity-service`'s HTTP API (`internal/entityservice`), not by touching a database itself: when a request's `EventID` (an `event_outbox` row ID) is set and `ENTITY_SERVICE_BASE_URL` is configured, `PostEvent` claims that row before publishing and marks it dispatched (or releases the claim on failure) afterward. A background `outbox.Poller` also runs in that case, sweeping `entity-service` for rows still `"waiting"` — the fallback for a row whose immediate `POST /events` call never happened at all — and dispatching them the same way. See [Event-driven notifications](#event-driven-notifications). Left unset, either per-request or per-deployment (no `ENTITY_SERVICE_BASE_URL`), publishing is unconditional and no poller runs, exactly as before `event_outbox` existed.

Recipient resolution for `case.*` events is **not** a TODO in the same sense: the caller (e.g. `csm-portal-backend`) supplies a `recipients` array in the event payload itself, since this service has no `entity-service` client to resolve watchers/assignee/reporter on its own. That's still short of "real" resolution in the sense that the caller has to already know the audience, but it's not a fixed/hardcoded stand-in either — every event picks its own recipients.

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

Each channel gets its own config/client pair in its own file, since channels differ in upstream auth scheme. All three clients are constructed once in `cmd/server/main.go` and handed to `dispatch.NewDispatcher` — a new channel follows the same client pattern, then gets wired into `Dispatcher` for whichever event type should trigger it (see "Adding a new event type" in CLAUDE.md).

## Event-driven notifications

```text
csm-portal-backend ──┐
                      ├─POST /events─▶ EventsHandler ──▶ outbox.Dispatch ──▶ eventbus.Producer ──▶ Event Hub topic
customer-portal-backend ┘                                      ▲                                       │
                                                                 │                                       ▼
                                              outbox.Poller ─────┘                    eventbus.Consumer (consumer group)
                                        (sweeps entity-service                                            │
                                         for stale "waiting" rows)                                       ▼
                                                                                         dispatch.Dispatcher.Handle
                                                                             (render email or send incident alerts)
```

- **`internal/events`** — the event schema. `Envelope{Type, EntityID, EventID, Payload}` plus one payload struct per `Type` (`case.created`, `case.comment_added`, `case.status_changed`, `case.assigned`, `incident.created`), each carrying every value its matching reaction needs. `EntityID` is a case ID for the `case.*` types or an incident ID for `incident.created` — whatever this event is about; for the `case.*` types, it must match the payload's own `caseId`. `EventID`, when present, is an `entity-service` `event_outbox` row ID — see `internal/entityservice` below. Payloads are deliberately denormalized (names/titles/links, not just IDs) since there's no `entity-service` client here yet to look up display data (only to claim/dispatch/release outbox rows). `incident.created` is the one type with two independent reactions (a Google Chat alert *and* a voice call) rather than an email.
- **`internal/entityservice`** — an HTTP client for `entity-service`'s `event_outbox` endpoints: `UpdateEventOutboxStatus` (`PATCH /event-outbox/{id}`) and `SearchWaitingEventOutbox` (`POST /event-outbox/search`) — this service's only interaction with that data, since it holds no database connection itself.
- **`internal/outbox`** — the claim-before-publish sequence shared by both callers below it in the diagram, so they can't duplicate this logic or drift out of sync. `Dispatch(ctx, pub, es, eventID, key, value)` claims the `event_outbox` row named by `eventID` (skipping publish on a 409 — already claimed by the other caller), publishes via `pub`, then marks the row dispatched on success or releases the claim back to `"waiting"` on publish failure — all of it a no-op passthrough to `pub.Publish` when `eventID` is empty or `es` is nil. `Poller.Run` ticks every `Interval`, calling `SearchWaitingEventOutbox` and running `Dispatch` on every returned row old enough (`MinAge`) to no longer be a live race with an in-flight `POST /events` call for the same row — the fallback for a row whose immediate dispatch never happened at all.
- **`internal/eventbus`** — a thin wrapper around [`github.com/segmentio/kafka-go`](https://github.com/segmentio/kafka-go) (a pure-Go Kafka client, no cgo — keeps this service on Choreo's buildpack deploy, MIT licensed) for Azure Event Hub's Kafka-compatible endpoint. `Producer.Publish` does a synchronous produce; `Consumer.Run` polls a consumer group, retries a failing record `handleAttempts` (3) times with a fixed delay, then logs it at ERROR and commits anyway rather than blocking the partition forever. See CLAUDE.md for the franz-go → kafka-go swap rationale and its two known trade-offs.
- **`internal/dispatch`** — `Dispatcher.Handle` implements `eventbus.Handle`: decode the record as an `events.Envelope`, render the matching template, email the `recipients` list carried in that event's own payload.
- **`POST /events`** (`internal/handler/events.go`) — the producer-facing endpoint: validates the envelope and its type-specific payload, then hands off to `outbox.Dispatch` (see above) keyed by `entityId` (so every event about one case/incident stays ordered on the same partition).

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
| `TWILIO_API_BASE_URL` | Overrides Twilio's REST API base (default `https://api.twilio.com/2010-04-01`). Optional — only for a regional Twilio edge/API endpoint |

### Event bus (Azure Event Hub)

Required — unlike the channels above, this is this service's core purpose, so a missing value fails startup loudly.

| Variable | Description |
|---|---|
| `EVENT_HUB_BROKER` | Kafka bootstrap address: `<namespace>.servicebus.windows.net:9093` |
| `EVENT_HUB_CONNECTION_STRING` | The namespace's Shared Access Policy connection string (Namespace > Shared access policies > a policy's Primary Connection String); used as the SASL/PLAIN password. Never commit a real value |
| `EVENT_HUB_TOPIC` | Event Hub (Kafka topic) name, e.g. `case-events` |
| `EVENT_HUB_CONSUMER_GROUP` | Consumer group ID this service's instances join to split partitions between themselves. Optional — defaults to `csm-notification-service` |

### entity-service

| Variable | Description |
|---|---|
| `ENTITY_SERVICE_BASE_URL` | Base URL of `entity-service` (e.g. `http://localhost:8080` or its Choreo URL). Optional — left unset, `event_outbox` claim/dispatch/release is skipped entirely, `POST /events` publishes unconditionally, and the poller below does not start |
| `EVENT_OUTBOX_POLL_INTERVAL` | How often `outbox.Poller` sweeps `entity-service` for stale `"waiting"` rows (e.g. `30s`, `1m`). Optional — defaults to `30s` if unset or malformed. Only relevant when `ENTITY_SERVICE_BASE_URL` is set |

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
│   │   ├── twilio.go           # TwilioConfig/TwilioClient/SendSMS+MakeCall
│   │   └── templates/          # HTML email templates + templates.go's Render* functions
│   ├── events/
│   │   └── events.go           # Envelope + per-Type payload structs (the event schema)
│   ├── eventbus/
│   │   ├── config.go            # Config + SASL/PLAIN setup shared by producer/consumer
│   │   ├── producer.go          # Producer — publish a record, wait for ack
│   │   └── consumer.go          # Consumer — consumer-group poll loop, retry, commit
│   ├── dispatch/
│   │   └── dispatch.go          # Dispatcher.Handle — envelope → template → EmailClient
│   └── handler/
│       ├── events.go            # POST /events — validates + publishes to the event bus
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
voice without changing `.env`. `TWILIO_API_BASE_URL` points either binary at
something other than real Twilio — e.g. a local mock server, useful for
dry-running `twiliocheck` itself without spending anything.

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
- `POST /events` — Submit a domain event; body requires `type` (`case.created` | `case.comment_added` | `case.status_changed` | `case.assigned` | `incident.created`), `entityId`, and a `payload` object matching that type (see `internal/events/events.go`). Validates and publishes to the event bus, responding `202 Accepted` — the actual notification is sent asynchronously by this service's own consumer, not in this request. `incident.created` triggers both a Google Chat alert and a Twilio voice call; the other four each trigger one email, addressed to the `recipients` array in that event's own payload

## Security

- **Never commit secrets** — client IDs/secrets, webhook URLs, and service URLs with credentials must not appear in source code or config files; use environment variables
- **No sensitive data in logs** — log only IDs and error summaries
- **No app-level inbound auth** — this is intentional (see above), not an oversight
- **Input validation** — validate and reject unexpected input at the boundary (body size, JSON structure, required fields) before any future dispatch
- **Security fixes in PRs** — describe security-related changes in neutral functional terms only, not called out as security fixes in the title/description
