# ACP Closure Service

Go port of Phase 1 of the Account Closure Process (ACP) — subscription
end-date closure only. Ports the decision/action logic from
`docs/legacy-servicenow-reference/ACPMainProcess.js` and
`ACPActionModules.js` (repo root); invoice/compliance logic (Phase 2) in
those files is reference-only and out of scope here.

## Shape: run-to-completion CLI, not a server

Every other Go component in this repo (`entity-service`,
`apps/csm-portal/backend`, `integrations/csm-integration-service`) is an
HTTP server. This one isn't: `cmd/acp-closure/main.go` performs one full
sweep and exits. A Choreo Task component's cron owns the schedule — there is
no in-process ticker, no long-running state, no health endpoint.

## Calls csm-integration-service, not entity-service directly

`internal/entity` is an HTTP client for `csm-integration-service`
(`integrations/csm-integration-service`), not entity-service. This was a
deliberate choice, not an oversight: `csm-integration-service` was built
with this automation specifically in mind (its `UpdateProject` client method
carries a comment to that effect), and the API team pointed us at its
`openapi.yaml` directly when asked how to call the API. Do not add a direct
entity-service client here without revisiting that decision.

`internal/entity.Client` deliberately does not implement the
`x-user-id-token` pass-through that `csm-integration-service`'s own entity
client has — this component is a headless batch job with no end-user
session, ever, so that code path would be permanently dead here.

## Package layout and the pure/I-O split

- `internal/closure` — pure decision logic. `Decide(now, endDate,
  lastNoticeWindow) Decision` reports what's due (which notice window, and
  whether suspend applies) given the confirmed 90/60/30/15/7/0 cascading
  thresholds. No I/O, no sequencing, no recipient logic. `Decision.ShouldSuspend`
  has no idempotency signal of its own — it fires on every day-0 evaluation
  regardless of `lastNoticeWindow`, because the real suspend-idempotency
  signal (`closureStatus`) isn't a parameter this function receives at all;
  callers must check it themselves (`sweep.suspend` does).
- `internal/recipients` — pure customer-contact and Account-Manager-email
  resolution. `ResolveCustomerContact` implements the three-tier fallback
  (business-contact-role Project Contact → account-level Primary Contact →
  signal to nudge the Account Manager instead). `AccountManagerEmail`
  extracts an email from an already-fetched `PersonRef`, treating "no AM
  assigned" and "AM assigned but no email" both as legitimate absence
  (`""`), not errors — many real accounts have incomplete role assignments.
  `Contact` (`{Name, Email}`) is the resolved-recipient shape shared with
  `notify.Recipients`.
- `internal/suspensionstate` — translates between
  `suspensionProcessState`'s real wire shape (see below) and
  `closure.NoticeWindow`. `WithSubscriptionEndDateState` only ever touches
  the `based_on_subscription_end_date` key; `based_on_due_invoices` and
  `based_on_compliance` (Phase 2, legacy-owned) must survive every write
  byte-for-byte untouched — this is covered by a dedicated regression test
  using a realistic multi-section payload, not a trivial empty case.
- `internal/notify` — `Notice` shape + `LoggingNotifier`. Real email sending
  does not exist anywhere yet (deferred pending message-queue design on the
  entity-service side) — `LoggingNotifier` is not a temporary stand-in for
  this component specifically, it's genuinely the only option available.
- `internal/sweep` — orchestration. `Run` paginates `/projects/search` (or,
  when `TEST_PROJECT_ID` is set, fetches exactly one project via
  `GetProject` and skips pagination entirely); `processProject` evaluates
  and acts on a single project.

Each pure package has an I/O counterpart living in `sweep` (e.g.
`resolveAccountContacts` does the `GetAccount` call and DTO parsing, then
hands parsed data to `recipients.AccountManagerEmail`). Keep new decision
logic in the pure packages and I/O in `sweep` — this split is what makes the
decision logic cheaply testable without mocks.

## Dry-run is an injection choice, not a branch

`DRY_RUN` never appears as an `if` inside `processProject` or `Run`. Both
have exactly two side-effecting dependencies — `projectUpdater` (writes) and
`notifier` (sends) — expressed as small interfaces. `main.go` decides which
concrete implementation to inject based on `DRY_RUN`:
`sweep.DryRunProjectUpdater` (logs, never calls `UpdateProject`) vs. the real
`*entity.Client`. Reads (`SearchProjects`, `GetAccount`,
`SearchProjectContacts`, `SearchAccountContacts`) are never dry-run-gated —
fetching and deciding has no write effect to protect against.

If you add a new side-effecting call, give it the same treatment: define a
minimal interface, inject the real implementation and a logging one, and
never branch on `DRY_RUN` inside the orchestration logic itself.

## TEST_PROJECT_ID scoping

`Run`'s `projectID` parameter, when non-empty, makes the broad
`SearchProjects` pagination loop structurally unreachable for that
invocation — it returns after evaluating the one fetched project, before the
loop's `offset := 0` line. This is what backs safe testing against a single
dedicated project without risk of touching every open project in an
environment.

## Notice audience matrix and content (redesigned per Chamara's direct request)

Confirmed audience rule, unchanged: 90/60/30-day windows are internal-only;
15/7/0-day windows are both internal and customer (`needsCustomerAudience`
in `sweep.go`). What changed is notice *shape*, not audience — this is a
deliberate redesign (superseding the original `Kind`
internal/customer/am_nudge model), decided directly with Chamara and
recorded here so the reasoning isn't lost:

- **One consolidated day-count reminder `Notice` per firing window**, not
  separate `Kind`-tagged sends. `Recipients` (`AccountOwner`,
  `RenewalManager`, `TechnicalOwner`, always populated; `Customer`, only for
  a resolved 15/7/0 window) replaces the old single `Recipient` string
  entirely. There is no more `Kind` field — audience is implied by which
  `Recipients` fields are populated and by `Subject`'s wording, not stated
  explicitly, per Chamara's request that the log stop saying
  "internal"/"external".
- **`Subject`** (`reminderSubject` in `sweep.go`) is `"{N} Days Reminder of
  Project for {ProjectName} of {AccountName}"`, `[ACP] `-prefixed only for
  the internal-only 90/60/30 windows. Confirmed against two real examples
  from Chamara — do not change the template without re-confirming; in
  particular, `ProjectName` itself often already contains the word
  "Subscription" (e.g. `"TICKETNETWORK - Subscription"`), which is why a
  literal subject can visually resemble "...Subscription of TicketNetwork"
  without "Subscription of" being separate template wording.
- **The no-business-contact case** (three-tier customer-contact fallback
  lands on `NeedsAMNudge`: no business contact, no primary contact) sends a
  **second, separate** `Notice` alongside the day-count reminder — not
  instead of it, and with no suppression logic collapsing the two. This
  notice has a fixed `Subject` (`"[Urgent] [ACP] No Business Contacts
  Specified for Project {ProjectName}"`), a `Recipients.AccountOwner`-only
  audience, and a fixed `Body` template (`noBusinessContactBody`) confirmed
  verbatim from a real existing notice. Sending both is a deliberate
  simplification the user confirmed rather than inventing a new suppression
  rule for this shape — revisit if it proves too noisy in practice. (The
  previous design's `shouldSuppressInternalNotice`, which collapsed a
  same-recipient internal+nudge pair into one send, no longer applies: there
  is no separate "internal" notice to suppress anymore, just the one
  reminder plus this second notice.)

## suspensionProcessState's real shape

Free-form JSON written by an existing, live ServiceNow suspension flow —
**not** something this component's design invented. Confirmed via a real
write against the dedicated test project
(`e3e87599-1bc7-6650-182c-0dc5604bcb68`):

```json
{
  "based_on_subscription_end_date": {"event_type": "30_days_notice", "actionSendEmailNotification": "SUCCESSFUL"},
  "based_on_due_invoices": {"event_type": "7_days_notice", "actionSendEmailNotification": "SUCCESSFUL", "actionServicePortalAnnouncement": "SUCCESSFUL"},
  "based_on_compliance": {"event_type": "open"}
}
```

This matches legacy's exact structure (`event_type` + per-action
`SUCCESSFUL`/`FAILED`/`IGNORED` results, three top-level dimensions). Phase 1
only ever reads/writes `based_on_subscription_end_date` — the other two
dimensions belong to Phase 2 / legacy and must never be touched.

## Known discrepancies between documented/coded behavior and live behavior

Confirmed via direct Postman testing against staging — each of these is a
case where reading a sibling service's source or docs would have given the
wrong answer:

- **Page size.** entity-service's own `maxLimit` constant
  (`entity-service/internal/service/user_service.go`) states `100`. The
  real, live maximum for `/projects/search` is **50** — `limit: 51` returns
  a 400. `internal/sweep/run.go`'s `pageSize` is set to `50` with this
  documented inline. If entity-service's constant is ever corrected, verify
  live behavior again before changing this — don't just copy the new
  constant.
- **PATCH /projects/{id} under M2M-only auth.** `csm-integration-service`'s
  own `CLAUDE.md` states this endpoint "currently receives a mapped 401 from
  `mapUpstreamError`, unconditionally" under M2M-only auth. Confirmed via
  direct, repeated testing (including after a fresh merge, to rule out
  staleness) that this is not true in practice: the endpoint accepts
  M2M-only writes successfully, including real writes to
  `suspensionProcessState`. Flagging discrepancies like this rather than
  silently trusting either source is a deliberate practice on this
  component — verify against real behavior before code changes that depend
  on an assumption from documentation or source reading alone.
- **`account` on `/projects/search` items.** For a period during this
  component's development, entity-service's `ProjectView` type had no
  account reference at all on search results (only the single-project
  detail endpoint carried one). That gap was closed
  (`domain.ProjectView.Account`) partway through this component's build.
  `internal/sweep/types.go`'s `project.Account` has always expected the
  nested `{id, name}` shape; only the doc comment needed correcting once the
  broader `SearchProjects` gap closed.

## Open dependencies

- **Business-contact role string** (`internal/recipients`'s
  `businessContactRole` constant, marked `PLACEHOLDER`) — exact
  ServiceNow-side literal still unconfirmed with the API team. Broad-sweep
  testing against real data shows this role is rarely configured in
  practice regardless — most real resolutions land on `primary_contact` or
  `am_nudge`, not `business_contact`.
- **Real email-sending mechanism** — deferred pending message-queue design
  on the entity-service side, not blocked on this component's own work.

## Testing conventions

- Hand-rolled mocks (function-field structs, e.g. `mockEntityReader`,
  `mockProjectUpdater`, `mockNotifier` in `internal/sweep/helpers_test.go`),
  matching `csm-integration-service`'s own test convention — no mocking
  library.
- Prefer real, previously-confirmed response shapes as test fixtures over
  synthetic/trivial ones where the exact shape matters (e.g.
  `TestProject_ParsesNestedAccountFromRealGetProjectResponse`,
  `TestWithSubscriptionEndDateState_PreservesOtherSectionsByteForByte` use
  the literal JSON confirmed via Postman against the dedicated test
  project/account) — this catches shape mismatches that a hand-written
  trivial fixture would silently paper over.
- TDD throughout: red before green, one seam at a time. Seams under test:
  `closure.Decide`, `recipients.ResolveCustomerContact` /
  `AccountManagerEmail`, `suspensionstate.LastNoticeWindow` /
  `WithSubscriptionEndDateState`, `sweep.processProject`, `sweep.Run`.
  `main.go` and the two logging-only implementations
  (`notify.LoggingNotifier`, `sweep.DryRunProjectUpdater`) are deliberately
  untested, matching this repo's convention that wiring-only code and
  behaviorless placeholders don't need dedicated tests.
