<!--
Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).

WSO2 LLC. licenses this file to you under the Apache License,
Version 2.0 (the "License"); you may not use this file except
in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
-->

# Customer Portal E2E (Playwright, local)

Scaffolding only — **no specs yet**. Runs locally against `pnpm run dev`
(:3000), authenticated by a **captured browser session** so no login page or
2FA is driven. See [`auth/README.md`](./auth/README.md) to capture one.

There is no mock backend here: specs will hit the same backend the dev server
is configured against (`public/config.js`), so anything a spec creates is a
**real record** in that environment. Tag created data so it stays identifiable.

## Run

Configuration lives in **`.env.e2e`** (at the webapp root, committed), so no env
vars need to be typed on the command line:

```bash
pnpm run test:e2e                          # all specs
node_modules/.bin/playwright test --ui     # author/debug interactively
node_modules/.bin/playwright show-report   # open the last HTML report
```

> Note: `pnpm exec playwright …` fails in this repo ("packages field missing");
> call the binary directly via `node_modules/.bin/playwright …`.

### Configuration

`playwright.config.ts` loads the env files itself (via node's built-in
`process.loadEnvFile`, no dotenv dependency). Precedence, highest first:

1. **real environment / CLI** — `E2E_BASE_URL=… pnpm run test:e2e`
2. **`.env.e2e.local`** — your personal overrides, git-ignored (`.env.*.local`)
3. **`.env.e2e`** — committed team defaults

| Var | Effect |
|---|---|
| `E2E_BASE_URL` | Environment under test. Default in `.env.e2e` is staging; falls back to `http://localhost:3000` if unset everywhere |
| `E2E_NO_WEBSERVER=1` | Don't boot the local dev server — required when `E2E_BASE_URL` points at a running deployment |
| `CI` | `forbidOnly`, and never reuse an already-running dev server |

No secrets belong in these files. Login is by replaying
`storageState/session.json` (git-ignored), not by credentials in env vars.

### Base URL must match the captured session

**The captured bundle decides which environment you can run against** — it only
restores into the origin it was captured from (see `fixtures/test.ts`).
`.env.e2e` ships pointing at staging because that is where the current
`session.json` was captured.

To run against the local dev server instead: recapture `session.json` while
signed in at `http://localhost:3000`, then create `.env.e2e.local` with

```bash
E2E_BASE_URL=http://localhost:3000
# Must be set empty, not omitted: keys absent from .env.e2e.local still come
# from .env.e2e, and an empty value reads as falsy so Playwright boots
# `pnpm run dev` itself.
E2E_NO_WEBSERVER=
```

`withSession()` skips (rather than fails) any test whose session bundle is
missing or captured against a different origin than the run targets, and the
skip message names the mismatch.

## Layout

| Path | Purpose |
|---|---|
| `auth/README.md` | How to capture a session bundle (localStorage + sessionStorage) |
| `fixtures/test.ts` | `withSession(test)` replays `storageState/session.json`, skipping each test that uses it when the bundle is absent or captured against a different origin (it skips from `beforeEach`, so tests are reported individually as skipped rather than the file being skipped as a unit); `openContextAs(browser, name)` opens a second authenticated context |
| `pages/` | Page objects — one per screen, no assertions inside |
| `specs/` | The specs, grouped in subfolders by feature area |
| `utils/` | Shared selectors / data-tagging helpers |
| `storageState/` | Captured session bundles — **git-ignored, real tokens** |

## Roles

One session (`session.json`) is captured today, so specs run as whatever that
account is. For role-gated coverage, capture additional bundles as
`storageState/<name>.json` and pass the name to `withSession(test, name)` or
`openContextAs(browser, name)`.

Each bundle should be captured from an account holding one role on the project
under test, so a spec can assert what that role can and cannot do:

- **admin** — manages users and registry service tokens in Settings.
- **lead** — a portal user who can also escalate a case past EL3.
- **portal** — the baseline: signs in, creates and manages cases.
- **security** — receives security advisories and raises security reports.

Project-level feature visibility (Operations, Security Center, Updates,
Engagements, Usage & Metrics …) is independent of all of these — it comes from
`GET /projects/{id}/features`. Pick the project a spec runs against
accordingly.
