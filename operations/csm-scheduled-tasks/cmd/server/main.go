// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

// The entry point for csm-scheduled-tasks. Unlike every other Go component
// in this repo, this is not a long-running server: Choreo invokes this
// binary fresh on its own Scheduled Task trigger, main runs exactly one
// Engine.Tick over the registered task list, and exits. There is
// deliberately no internal ticker/cron loop here — Choreo's own trigger IS
// the driver (see this component's own CLAUDE.md for the "driver cadence"
// concept).
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/adhocore/gronx"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/engine"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/ledger"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/registry"
)

func main() {
	loadDotEnv(".env")
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	driverInterval := envDuration("DRIVER_INTERVAL", time.Hour)

	// Shared OAuth2 client credentials — used by the entity-service client
	// below and the email client below it, and by any future service
	// client this component grows. Mirrors
	// integrations/csm-notification-service's own OAUTH2_CLIENT_ID/
	// OAUTH2_CLIENT_SECRET/OAUTH2_TOKEN_URL convention: the real deployments
	// these point at authenticate every caller through the same shared
	// gateway app, scoped per-client via each client's own *_SCOPES var, not
	// a separate per-consumer app. mustEnv even though the email client
	// alone is optional — entity-service is not, and both clients share
	// this one credential set.
	oauthTokenURL := mustEnv("OAUTH2_TOKEN_URL")
	oauthClientID := mustEnv("OAUTH2_CLIENT_ID")
	oauthClientSecret := mustEnv("OAUTH2_CLIENT_SECRET")

	// entity-service is this component's only durable state — see
	// internal/ledger's own doc comment — so a missing CUSTOMER_ENTITY_SERVICE_BASE_URL,
	// or either URL failing httpsec's https-only check, fails startup
	// loudly rather than surfacing as a per-task error later, unlike the
	// email client below.
	ledgerClient, err := ledger.NewClient(ledger.Config{
		BaseURL:      mustEnv("CUSTOMER_ENTITY_SERVICE_BASE_URL"),
		TokenURL:     oauthTokenURL,
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		Scopes:       splitComma(os.Getenv("CUSTOMER_ENTITY_SERVICE_SCOPES")),
	})
	if err != nil {
		slog.Error("failed to construct entity-service client", "err", err)
		os.Exit(1)
	}

	// Standing ops/on-call audience, emailed on every failure for every
	// task in addition to that task's own registry.Task.To/Cc — see
	// engine.Engine.AlertRecipients' own doc comment. Parsed before the
	// email client below specifically so the check right after it can run:
	// a non-empty list with no EMAIL_BASE_URL configured would otherwise
	// only surface the first time some task actually fails and tries to
	// send, as an opaque "invalid URL" error from a relative "/send-email"
	// path — much easier to catch here, at startup.
	alertRecipients := splitComma(os.Getenv("ALERT_RECIPIENTS"))
	emailBaseURL := os.Getenv("EMAIL_BASE_URL")
	if len(alertRecipients) > 0 && emailBaseURL == "" {
		slog.Error("ALERT_RECIPIENTS is set but EMAIL_BASE_URL is not; refusing to start since failure alerts could never actually send")
		os.Exit(1)
	}

	// Email itself is not required for every deployment (nothing calls it
	// while no registered task sets To and ALERT_RECIPIENTS is also empty,
	// per the check above) — EMAIL_BASE_URL is read with os.Getenv, not
	// mustEnv, matching integrations/csm-notification-service's own
	// internal/notifications.EmailClient. NewClient itself still fails
	// startup if EMAIL_BASE_URL is set but not https. Authenticates with
	// the same shared OAUTH2_* credentials as ledgerClient above, not its
	// own — only BaseURL/Scopes/FromAddress are specific to this client.
	emailClient, err := notify.NewClient(notify.Config{
		BaseURL:      emailBaseURL,
		TokenURL:     oauthTokenURL,
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		Scopes:       splitComma(os.Getenv("EMAIL_SCOPES")),
		FromAddress:  os.Getenv("EMAIL_FROM_ADDRESS"),
	})
	if err != nil {
		slog.Error("failed to construct email client", "err", err)
		os.Exit(1)
	}

	// No real sub-crons registered yet. See this component's own CLAUDE.md
	// ("Adding a sub-cron") for the steps to add one — including wiring
	// SUB_CRON_SCHEDULES/SUB_CRON_RECIPIENTS support in via
	// parseSubCronSchedules/scheduleFor and
	// parseSubCronRecipients/recipientsFor below, none of which have a
	// caller until then.
	tasks := []registry.Task{}

	for _, t := range tasks {
		if !gronx.IsValid(t.Schedule) {
			slog.Error("invalid cron schedule for registered task; refusing to start", "task", t.Name, "schedule", t.Schedule)
			os.Exit(1)
		}
	}

	eng := engine.New(tasks, ledgerClient, emailClient, driverInterval, alertRecipients)

	// No app-level execution timeout here — Choreo's own Scheduled Task
	// execution-time limit already bounds how long one invocation can run.
	// signal.NotifyContext instead cancels this context the moment Choreo
	// sends SIGTERM (whether that's from its own timeout firing, a
	// redeploy, or a manual stop), so in-flight HTTP calls to entity-service
	// abort promptly and this process can log/exit cleanly, rather than
	// being cut off mid-request with no chance to react.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	start := time.Now()
	eng.Tick(ctx, start)
	slog.Info("tick complete", "elapsed", time.Since(start).String())
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required environment variable is not set", "key", key)
		os.Exit(1)
	}
	return v
}

// parseSubCronSchedules decodes SUB_CRON_SCHEDULES, a JSON object mapping a
// registered task's Name to a cron schedule override — one shared config
// value for every task in the registry, rather than a dedicated env var per
// task name that would need inventing again for every new sub-cron added
// here. A missing or malformed value logs a warning and yields no
// overrides, so every task just falls back to its own hardcoded default
// schedule — mirrors integrations/csm-notification-service's own
// parseGoogleChatSpaces (same "optional JSON env var, log and fall back on
// a bad value" shape). No caller until the first real registry.Task exists
// — see "Adding a sub-cron" in this component's own CLAUDE.md, which calls
// this out explicitly as scaffolding for that step, not dead code.
//
//nolint:unused // see doc comment above
func parseSubCronSchedules(raw string) map[string]string {
	if raw == "" {
		return nil
	}
	var overrides map[string]string
	if err := json.Unmarshal([]byte(raw), &overrides); err != nil {
		slog.Error("failed to parse SUB_CRON_SCHEDULES; every task will use its own hardcoded default schedule", "err", err)
		return nil
	}
	return overrides
}

// scheduleFor returns overrides[taskName] if present and non-empty,
// otherwise def. def is still what ships when SUB_CRON_SCHEDULES doesn't
// mention taskName at all — every registered task keeps a sensible
// hardcoded default in code; SUB_CRON_SCHEDULES only ever overrides it.
//
//nolint:unused // see parseSubCronSchedules's own nolint comment above.
func scheduleFor(overrides map[string]string, taskName, def string) string {
	if s, ok := overrides[taskName]; ok && s != "" {
		return s
	}
	return def
}

// subCronRecipients is the per-task shape decoded from SUB_CRON_RECIPIENTS.
type subCronRecipients struct {
	To []string `json:"to"`
	Cc []string `json:"cc"`
}

// parseSubCronRecipients decodes SUB_CRON_RECIPIENTS, a JSON object mapping
// a registered task's Name to {"to": [...], "cc": [...]} — the config-driven
// counterpart to SUB_CRON_SCHEDULES, so a sub-cron's failure audience lives
// in .env next to its cadence, not hardcoded as registry.Task{To, Cc}
// literals in this file. A task not mentioned here just gets nil To/Cc —
// its failures still reach the standing ALERT_RECIPIENTS list (see
// engine.Engine.AlertRecipients' own doc comment), it just has no
// additional audience of its own. A missing or malformed value logs a
// warning and yields no per-task recipients at all, the same
// fail-safe-not-fail-closed shape parseSubCronSchedules uses.
//
// same situation as parseSubCronSchedules above.
//
//nolint:unused // no caller until the first real registry.Task exists —
func parseSubCronRecipients(raw string) map[string]subCronRecipients {
	if raw == "" {
		return nil
	}
	var overrides map[string]subCronRecipients
	if err := json.Unmarshal([]byte(raw), &overrides); err != nil {
		slog.Error("failed to parse SUB_CRON_RECIPIENTS; every task will have no per-task alert recipients", "err", err)
		return nil
	}
	return overrides
}

// recipientsFor returns overrides[taskName]'s To/Cc, or nil, nil if
// taskName isn't mentioned in overrides at all.
//
//nolint:unused // see parseSubCronRecipients's own nolint comment above.
func recipientsFor(overrides map[string]subCronRecipients, taskName string) (to, cc []string) {
	r := overrides[taskName]
	return r.To, r.Cc
}

// envDuration returns the given environment variable parsed with
// time.ParseDuration (e.g. "1h", "5m"), or def if unset or malformed.
func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		slog.Warn("environment variable is not a valid positive duration; using default", "key", key, "value", v, "default", def)
		return def
	}
	return d
}

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			result = append(result, t)
		}
	}
	return result
}

// loadDotEnv reads a .env file and sets any unset environment variables
// from it. Silently ignored if the file does not exist; logs a warning for
// any other error. Mirrors integrations/csm-notification-service's own
// cmd/server/main.go helper of the same name.
func loadDotEnv(path string) {
	f, err := os.Open(path) // #nosec G304 -- path is always the hardcoded literal ".env" at the only call site
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("loadDotEnv: failed to open .env file", "err", err)
		}
		return
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
			v = v[1 : len(v)-1]
		}
		if _, present := os.LookupEnv(k); !present {
			_ = os.Setenv(k, v)
		}
	}
	if err := scanner.Err(); err != nil {
		slog.Warn("loadDotEnv: error reading .env file", "err", err)
	}
}
