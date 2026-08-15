# CSM Notification Service

Go background worker (`net/http`, Go 1.26+) with no inbound API beyond a health check. `csm-portal-backend` and `customer-portal-backend` publish domain events (case created, comment added, status changed, case assigned, incident created) directly to Azure Event Hub; this service's own consumers read them back and react asynchronously by sending email, Google Chat alerts, and voice calls. See [Event-driven notifications](#event-driven-notifications).

This service was extracted from `apps/csm-portal/backend/internal/notifications`, which previously hosted the same email/Google Chat clients. That backend no longer constructs or calls any notification client directly. This service itself used to expose `POST /events` (backends called it, and it published to Event Hub on their behalf) — that producer-side hop was removed once the backends took over publishing directly; this service is a pure consumer now.

## Why no `Auth` middleware

The only route this service exposes is `GET /health` (Choreo's liveness probe) — there's no end-user or caller identity to check, since everything else is Kafka consumption, not an HTTP request.

## Current scope — TODO

- **SMS and direct call channels are unused.** `TwilioClient.SendSMS` has no caller — `MakeCall` is only invoked by `incident.created`.

A dead-letter queue exists (see [Event-driven notifications](#event-driven-notifications)) — a record that exhausts the main consumer's retries is published there and gets a fresh retry pass from a separate DLQ consumer, rather than being dropped immediately. There is still no third tier past that.

This service deliberately has no database connection and never talks to one directly. Deduplicating a publish, or recovering one Event Hub never acknowledged, is now entirely the publishing backend's job — `entity-service`'s `event_publish_failures` table exists for that, written to only when Event Hub doesn't ack a publish, and this service never reads or writes it.

Recipient resolution for `case.*` events is **not** a TODO in the same sense: the caller (e.g. `csm-portal-backend`) supplies a `recipients` array in the event payload itself, since this service has no way to resolve watchers/assignee/reporter on its own. That's still short of "real" resolution in the sense that the caller has to already know the audience, but it's not a fixed/hardcoded stand-in either — every event picks its own recipients. This service *does* now resolve which portal link each of those recipients gets (`internal/recipientlinks`, backed by `internal/entity`'s customer-entity-service client) — a distinct, smaller kind of resolution from audience resolution; see [Event-driven notifications](#event-driven-notifications).

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
                      ├──▶ Event Hub topic ──▶ main consumer group ──▶ dispatch.Dispatcher.Handle
customer-portal-backend ┘                           │                  (render email or send incident alerts)
                                                      │ (retries exhausted)
                                                      ▼
                                              Event Hub DLQ topic ──▶ DLQ consumer group ──▶ dispatch.Dispatcher.Handle
                                                                       (fresh retry pass, same Handle;
                                                                        exhausting it here just logs+drops)
```

Both backends publish directly to the main topic — there is no HTTP hop through this service. Two packages implement the bus→consumer side; `internal/notifications` (templates + channel clients) is the last-mile sender they call.

- **`internal/events`** — the event schema and its only remaining validation boundary. `Envelope{Type, EntityID, Payload}` plus one payload struct per `Type` (`case.created`, `case.comment_added`, `case.status_changed`, `case.assigned`, `incident.created`), each carrying every value its matching reaction needs. `EntityID` is a case ID for the `case.*` types or an incident ID for `incident.created` — whatever this event is about; for the `case.*` types, it must match the payload's own `caseId`. `Validate(entityID, type, payload)` decodes strictly and checks required fields — moved here from a since-removed HTTP handler, since there's no request boundary to validate at anymore; `dispatch.Dispatcher.Handle` calls it before rendering/sending anything. Payloads still carry denormalized display values (names/titles) this service has no other way to obtain, but no longer carry pre-built case/comment links — the four `case.*` payloads carry `projectId`/`caseId` (and `commentId`, for `case.comment_added`) instead, and `internal/dispatch` resolves each recipient's own portal-appropriate link itself via `internal/recipientlinks`. `incident.created` is the one type with two independent reactions (a Google Chat alert *and* a voice call) rather than an email, and doesn't go through link resolution at all — its Chat/call destinations are already in the payload.
- **`internal/entity`** — a minimal, read-only customer-entity-service client, implementing only `POST /users/search` (unlike `apps/csm-portal/backend`'s own entity client, a ~60-method passthrough surface). Backs `internal/recipientlinks`'s per-recipient role lookup. Not a notification channel — doesn't follow the `<Name>Config`/`<Name>Client`-in-`internal/notifications` pattern below, since it's an upstream data client, not something that sends a notification itself.
- **`internal/recipientlinks`** — `Resolver.ResolveLinks(ctx, emails, projectID, caseID)` looks up each email's role via `internal/entity` and returns the case link appropriate to their portal (customer vs CSM), with a role → role → userType → CSM-default fallback chain. A per-*recipient* decision, not per-event: the same `case.comment_added` notification can go to both a customer watcher and an internal CSM watcher at once, each needing a different link.
- **`internal/eventbus`** — a thin wrapper around [`github.com/segmentio/kafka-go`](https://github.com/segmentio/kafka-go) (a pure-Go Kafka client, no cgo — keeps this service on Choreo's buildpack deploy, MIT licensed) for Azure Event Hub's Kafka-compatible endpoint. `Producer.Publish` does a synchronous produce — this service's own use of it today is only for publishing to the dead-letter topic, since the main topic's producer side now lives in the backends. `Consumer.Run(ctx, handle, onExhausted)` polls a consumer group, retries a failing record `handleAttempts` (3) times with a fixed delay, then calls `onExhausted` (or logs at ERROR and drops, if `onExhausted` is nil) before committing either way. `PartitionCount(ctx, cfg)` reports a topic's real partition count, used at startup to sanity-check a configured consumer count. See CLAUDE.md for the franz-go → kafka-go swap rationale and its two known trade-offs.
- **`internal/dispatch`** — `Dispatcher.Handle` implements `eventbus.Handle`: decode the record as an `events.Envelope`, validate it (`events.Validate`), then for the four `case.*` types, resolve each recipient's own case link (`groupByLink`, via `internal/recipientlinks`), bucket recipients by the link they resolved to, and render+send one email per distinct link (`sendPerGroup`) — recipients sharing a link still batch into one `SendEmail` call. `incident.created` skips all of this and sends a Google Chat alert plus places a voice call directly from the payload's own fields.
- **Dead-letter queue** — when the main consumer's `Handle` call fails on all `handleAttempts` attempts, the record is published to `EVENT_HUB_DLQ_TOPIC` instead of being dropped. A second, independent consumer group runs against that topic, using the same `Dispatcher.Handle` — so a dead-lettered record gets its own fresh retry pass — but with nowhere further to escalate to: exhausting retries there just logs and drops. Provision `EVENT_HUB_DLQ_TOPIC` as its own Event Hub in Azure before deploying; this service doesn't create topics itself.
- **Configurable consumer counts** — `MAIN_CONSUMER_COUNT`/`DLQ_CONSUMER_COUNT` each start that many independent `eventbus.Consumer` instances, all joining the same consumer group; Kafka's own rebalancing splits a topic's partitions across however many are actually running. Keep each count at or below its topic's real partition count — a startup check logs a warning (not a hard failure) if it isn't, since excess consumers just sit idle rather than causing an error.

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

### Customer entity service

Backs `internal/recipientlinks`'s per-recipient role lookup (`POST /users/search` only). Optional per deployment like the channels above, but an unset `CUSTOMER_ENTITY_BASE_URL` makes every `case.*` email fail rather than just disabling one channel — a startup warning is logged when this happens. Unlike the channels above, this client authenticates with the **shared** `OAUTH2_CLIENT_ID`/`OAUTH2_CLIENT_SECRET`/`OAUTH2_TOKEN_URL` credentials (see below), the same OAuth2 app `apps/csm-portal/backend`'s own entity client uses — only `BaseURL`/`Scopes` are specific to this client.

| Variable | Description |
|---|---|
| `OAUTH2_CLIENT_ID` | Shared OAuth2 client ID, used only by the customer entity service client (optional) |
| `OAUTH2_CLIENT_SECRET` | Shared OAuth2 client secret (optional) |
| `OAUTH2_TOKEN_URL` | Shared OAuth2 token endpoint (optional) |
| `CUSTOMER_ENTITY_BASE_URL` | Base URL of this repo's entity-service (optional, see above) |
| `CUSTOMER_ENTITY_SCOPES` | Comma-separated OAuth2 scopes (optional) |

### Recipient portal links

| Variable | Description |
|---|---|
| `CSM_PORTAL_WEB_BASE_URL` | CSM portal webapp base URL — `<CSM_PORTAL_WEB_BASE_URL>/cases/{caseId}` for recipients classified CSM (optional) |
| `CUSTOMER_PORTAL_WEB_BASE_URL` | Customer portal webapp base URL — `<CUSTOMER_PORTAL_WEB_BASE_URL>/projects/{projectId}/support/cases/{caseId}` for recipients classified customer (optional) |
| `CUSTOMER_ROLES` | Comma-separated role names classified customer (optional) |
| `CSM_ROLES` | Comma-separated role names classified CSM (optional) |

Classification isn't just "role in `CUSTOMER_ROLES`" — it's a fallback chain, since neither role list needs to be exhaustive: a role in `CUSTOMER_ROLES` → customer; else a role in `CSM_ROLES` → CSM; else the recipient's entity-service `userType` (`customer`/`external` → customer, anything else → CSM); else — including when entity-service has no record for the recipient at all — CSM, as the last-resort default. See `Resolver.ResolveLinks`'s doc comment for the full reasoning.

### Event bus (Azure Event Hub)

Required — unlike the channels above, this is this service's core purpose, so a missing value fails startup loudly. This service is a pure consumer; the backends publish directly to `EVENT_HUB_TOPIC` themselves.

| Variable | Description |
|---|---|
| `EVENT_HUB_BROKER` | Kafka bootstrap address: `<namespace>.servicebus.windows.net:9093` |
| `EVENT_HUB_CONNECTION_STRING` | The namespace's Shared Access Policy connection string (Namespace > Shared access policies > a policy's Primary Connection String); used as the SASL/PLAIN password. Never commit a real value |
| `EVENT_HUB_TOPIC` | Event Hub (Kafka topic) name, e.g. `case-events` |
| `EVENT_HUB_CONSUMER_GROUP` | Consumer group ID the main consumer's instances join. Optional — defaults to `csm-notification-service` |
| `MAIN_CONSUMER_COUNT` | How many concurrent consumer instances to run for `EVENT_HUB_TOPIC`. Optional — defaults to `1`; keep at or below the topic's real partition count |

### Dead-letter queue

Required — a record that exhausts the main consumer's retries is published here rather than dropped; without a valid topic here, that would fail loudly at publish time instead.

| Variable | Description |
|---|---|
| `EVENT_HUB_DLQ_TOPIC` | A second Event Hub in the same namespace, provisioned separately in Azure |
| `EVENT_HUB_DLQ_CONSUMER_GROUP` | Consumer group ID the DLQ consumer's instances join. Optional — defaults to `csm-notification-service-dlq` |
| `DLQ_CONSUMER_COUNT` | How many concurrent consumer instances to run for `EVENT_HUB_DLQ_TOPIC`. Optional — defaults to `1`; same partition-count guidance as `MAIN_CONSUMER_COUNT` |

### Server

| Variable | Description |
|---|---|
| `PORT` | Server listen port — a plain number, not an address (default `8080`) |

## Project Structure

```text
csm-notification-service/
├── cmd/
│   ├── server/main.go           # Entry point — starts the HTTP health server + both consumer groups
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
│   │   ├── events.go           # Envelope + per-Type payload structs (the event schema)
│   │   └── validate.go         # Validate — the only remaining validation boundary
│   ├── entity/
│   │   └── customer.go         # Minimal read-only entity-service client (POST /users/search only)
│   ├── recipientlinks/
│   │   └── resolver.go         # Resolver.ResolveLinks — per-recipient customer/CSM portal link
│   ├── eventbus/
│   │   ├── config.go            # Config + SASL/PLAIN setup + PartitionCount, shared by producer/consumer
│   │   ├── producer.go          # Producer — publish a record, wait for ack
│   │   └── consumer.go          # Consumer — consumer-group poll loop, retry, OnExhausted, commit
│   └── dispatch/
│       └── dispatch.go          # Dispatcher.Handle — envelope → validate → resolve links → group → template → EmailClient
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

- `GET /health` — Health check (Choreo's liveness probe). This is the only inbound HTTP route this service has — everything else happens via the two Kafka consumer groups described in [Event-driven notifications](#event-driven-notifications).

## Security

- **Never commit secrets** — client IDs/secrets, webhook URLs, and service URLs with credentials must not appear in source code or config files; use environment variables
- **No sensitive data in logs** — log only IDs and error summaries
- **No app-level inbound auth** — this is intentional (see above), not an oversight
- **Input validation** — `events.Validate`, called from `dispatch.Dispatcher.Handle`, is the only validation boundary this service has left; keep rejecting unexpected input there rather than letting it reach a notification client
- **No recipient emails in logs** — `internal/recipientlinks`'s role-lookup warnings log `caseID`/`roles`/`userType`, never the recipient's email address (PII); keep it that way if this code changes
- **Security fixes in PRs** — describe security-related changes in neutral functional terms only, not called out as security fixes in the title/description
