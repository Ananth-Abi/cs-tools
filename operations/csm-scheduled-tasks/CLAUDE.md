# CSM Scheduled Tasks

A single Choreo Scheduled Task that internally fans out to any number of independently-scheduled
sub-crons on one shared driver cadence. Go 1.26+, no HTTP surface at all — Choreo invokes this
binary fresh on its own trigger, `main` runs exactly one `engine.Engine.Tick` over the registered
task list, and exits. There is no internal ticker/cron loop; Choreo's own trigger IS the driver.

This process has no database of its own — durable claim/retry state lives in entity-service's
`scheduled_task_run` table (see that repo's own `CLAUDE.md`, "Scheduled task runs"), the same
division of labor `integrations/csm-notification-service`'s `internal/slaengine` uses.

## The core mechanism: period keys

For a sub-cron's cron expression `E` and a point in time `now`, `internal/schedule.PeriodKey(E,
now)` returns `E`'s most recent scheduled firing time ≤ `now` (via
`github.com/adhocore/gronx.PrevTickBefore`). A daily expression returns the *same* timestamp for
every call between that firing and its next one — every tick in between re-derives the identical
key, so retries land on the same ledger row instead of a new one, no matter how many hours or days
pass.

A row keeps retrying on every eligible tick until either it succeeds (done, forever, for that
period), or the *next* period comes due while it's still unresolved — at which point entity-service
marks it **superseded** (abandoned; only the newest period gets chased from then on). This is what
bounds the ledger to at most one open row per task at any time, and is why there is no
`max_attempts` anywhere in this design: a broken job just keeps retrying — and alerting — until
either it's fixed or its own next period supersedes it.

## Engine tick — one pass, one rule

`engine.Engine.Tick` does exactly one thing per registered task, once per invocation:

1. Compute the task's current period key.
2. Call `POST /scheduled-tasks/attempts` (`internal/ledger.Client.Attempt`) — entity-service
   atomically decides allow/deny: a period this task hasn't seen before first supersedes any other
   still-open row for the same task, then claims fresh; an existing row whose retry time has
   arrived is bumped and claimed; anything else is denied.
3. If allowed, run `Task.Handler`, then report back via `PATCH /scheduled-tasks/attempts/{id}`
   (one endpoint for both outcomes, `internal/ledger.Client.Complete`/`Fail` on the Go side) —
   `Complete` (success) or `Fail` (failure —
   sets `nextRetryOn`, never a permanent give-up state).

There is deliberately no second "sweep" pass here: an earlier design had one whose job was to
rescue an old failed period after the calendar moved past it, but once the rule became "only ever
chase the newest period," there is nothing left to rescue — an unresolved old period is meant to be
abandoned, not resurrected. See `internal/engine/engine.go`'s own package doc for the full
reasoning, and the design artifact linked from the PR that introduced this component for the
worked examples that led here.

## Adding a sub-cron

There are no sub-crons registered yet — `tasks := []registry.Task{}` in `cmd/server/main.go` is
empty. To add one:

1. Write the handler: `func(ctx context.Context) error` — return nil on success, a non-nil error
   (with enough detail to be useful in an alert email) on failure. It does not need to know
   anything about retries, periods, or the ledger — the engine handles all of that.
2. Add a `registry.Task{Name, Schedule, Handler, ...}` entry to the `tasks` slice in
   `cmd/server/main.go`, with a sensible default `Schedule` hardcoded in code. `Name` is the
   entity-service ledger key **and** the key both `SUB_CRON_SCHEDULES` and `SUB_CRON_RECIPIENTS`
   below look it up by — treat a rename like a breaking API change: it orphans the task's prior
   history in entity-service, and silently stops matching any existing schedule *or* recipients
   override in either config (they're two separate JSON maps — see step 4 — but both keyed on this
   same exact `Name`, so a rename has to be updated in both at once).
3. Leave `To`/`Cc` unset in the struct literal itself — they're populated from config, not
   hardcoded (see step 4 and "Alerting" below).
4. Wire up both config surfaces for it, by exact `Name`:
   - Schedule: `scheduleFor(parseSubCronSchedules(os.Getenv("SUB_CRON_SCHEDULES")), "<task.Name>", "<default>")`
     for the task's `Schedule` field.
   - Recipients: `recipientsFor(parseSubCronRecipients(os.Getenv("SUB_CRON_RECIPIENTS")), "<task.Name>")`
     returns `(to, cc []string)` for the task's `To`/`Cc` fields — leave both unset (call returns
     `nil, nil`) unless `SUB_CRON_RECIPIENTS` actually mentions this task.
   (All four helper functions already exist in `cmd/server/main.go`, just currently unreferenced
   since there's nothing to call them for yet.)

Every task's cadence and per-task recipients can both be set without a code change:
`SUB_CRON_SCHEDULES` is `{"<task.Name>": "<cron expression>"}`; `SUB_CRON_RECIPIENTS` is
`{"<task.Name>": {"to": [...], "cc": [...]}}` — both single JSON objects shared by the whole
registry, rather than a dedicated env var per task that would need inventing again for every new
sub-cron added here. `scheduleFor`/`recipientsFor` look up each task's override by its exact
`Name`; anything not mentioned just keeps its own hardcoded default (schedule) or gets nil (To/Cc).

## Retry backoff

`registry.Task.RetryBackoff` defaults to the driver's own interval (`DRIVER_INTERVAL`, see below)
when zero — there is no point backing off shorter than how often this process even runs. Set it
explicitly only if a specific task needs to back off harder than "every tick."

## Alerting

Two layers, combined:

- `ALERT_RECIPIENTS` (env var) is a standing ops/on-call audience — emailed on every failed
  attempt, for every task, unconditionally. Most tasks will leave `To`/`Cc` unset and rely on this
  alone.
- `registry.Task.To`/`Task.Cc` are additional recipients specific to that one task, populated from
  `SUB_CRON_RECIPIENTS` (see below) — set only for the sub-crons that need a different or extra
  audience, e.g. a billing job's own on-call team alongside the standing list. `To` gets merged
  into the same `To` line as `AlertRecipients` (not `Cc`), so a task's own recipients and the
  standing list both land as real primary recipients, not one buried in Cc; `Cc` stays purely
  per-task.

A task's failures send no email only if **both** `ALERT_RECIPIENTS` and that task's own `To` are
empty — it still fails and retries normally either way, just silently as far as email goes.

**Every single failed attempt sends an alert, not just a final give-up** — there is no "fully
failed" or exhausted state in this design (see "The core mechanism" above), so this fires each time
a handler returns an error, however many times that happens before the task either succeeds or gets
superseded by its own next period. A task retrying every tick for hours alerts every tick too; if
that's too noisy for a given task in practice, the fix is a notification-frequency policy layered
on top later, not a retry cap.

The email itself (`internal/notify/templates/alert.html`, rendered by `notify.RenderAlertEmail`) is
a plain HTML template in the same table-based, WSO2-branded style
`integrations/csm-notification-service`'s own templates use (`escapeHTML`/`escapeMultiline` mirror
that service's own functions of the same name) — task name, period, attempt count, next retry time,
and the failure's error message.

There is currently no success email — see "Future: per-task report emails" below for why.

## Environment variables

| Variable | Required | Description |
|---|---|---|
| `OAUTH2_CLIENT_ID` / `OAUTH2_CLIENT_SECRET` / `OAUTH2_TOKEN_URL` | Yes | Shared OAuth2 client credentials — used by the entity-service client and the email client below, and by any future service client this component grows. Mirrors `integrations/csm-notification-service`'s own `OAUTH2_*` convention: the real deployments these point at authenticate every caller through the same shared gateway app, scoped per-client via each client's own `*_SCOPES` var, not a separate app per consumer |
| `CUSTOMER_ENTITY_SERVICE_BASE_URL` | Yes | entity-service base URL |
| `CUSTOMER_ENTITY_SERVICE_SCOPES` | No | Comma-separated OAuth2 scopes for the entity-service client, using the shared credentials above |
| `EMAIL_BASE_URL` | No | Internal email notification service base URL (`POST /send-email`) — same service `integrations/csm-notification-service` uses. Authenticates with the same shared `OAUTH2_*` credentials, not its own |
| `EMAIL_SCOPES` | No | Comma-separated OAuth2 scopes for the email client, using the shared credentials above |
| `EMAIL_FROM_ADDRESS` | No | Fixed "From" address for every email this component sends |
| `ALERT_RECIPIENTS` | No | Comma-separated email addresses alerted on every failed sub-cron attempt, for every task — see "Alerting" above |
| `DRIVER_INTERVAL` | No (default `1h`) | This component's own expected invocation cadence — must match the cron trigger configured on the Choreo Scheduled Task component itself |
| `SUB_CRON_SCHEDULES` | No | JSON object `{"<task.Name>": "<cron expression>"}` overriding any registered task's schedule by name — see "Adding a sub-cron" above. A task not mentioned keeps its own hardcoded default |

No app-level execution timeout is configured here — Choreo's own Scheduled Task execution-time
limit already bounds how long one invocation can run. `cmd/server/main.go` instead cancels its
context via `signal.NotifyContext` on `SIGTERM`, so however that signal arrives (Choreo's own
timeout firing, a redeploy, a manual stop), in-flight HTTP calls to entity-service abort promptly
instead of being cut off mid-request with no chance to react.

`.env` is auto-loaded from the working directory at startup if present (silently ignored if
absent), matching `integrations/csm-notification-service`'s own convention.

## Future: per-task report emails

Not built yet, deliberately: `registry.Task` has no report-recipients field and `engine.recordSuccess`
only updates the ledger, never sends anything. A generic one-size-fits-all "task succeeded" template
was tried and dropped — different sub-crons will want genuinely different report content (a usage
report reads nothing like a billing summary), so one shared template would either stay generic to
the point of being useless or grow special cases per task. The intended shape when this is built:
each real sub-cron owns its own template (in `internal/notify/templates/`, following `alert.html`'s
pattern) and supplies its own `ReportRecipients`, rather than the engine rendering one shared shape
for every task the way it does for alerts today.

## Future: events

Not built yet. `engine.Engine`'s `recordSuccess`/`recordFailure` are the one place a status
transition (succeeded/failed/superseded) is known — a future `onTransition(task, period, status)`
hook there, publishing to Event Hub, would be a small addition rather than a rework, and would let
email delivery move from "this component sends it" to "this component publishes,
`csm-notification-service` sends it" — the same division of labor the rest of this system already
uses for `case.*` events. `superseded` transitions aren't currently observable at all from this
component's own code (entity-service's `Attempt` response doesn't report whether it just
superseded something) — that's a real gap if a "period X was abandoned" notice is wanted later; it
would need a small addition to the `ClaimScheduledTaskRunResponse` contract, not just to this
component.

## Running locally

```bash
# from operations/csm-scheduled-tasks
go run ./cmd/server
```

Runs exactly one tick against whatever `CUSTOMER_ENTITY_SERVICE_BASE_URL` points at, then exits — there is
no server to leave running.

## Commands

```bash
go vet ./...              # vet
go test -race ./...       # race-detector tests
```
