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
	// internal/ledger's own doc comment — so a missing ENTITY_SERVICE_BASE_URL
	// here fails startup loudly (mustEnv) rather than surfacing as a
	// per-task error later, unlike the email client below.
	ledgerClient := ledger.NewClient(ledger.Config{
		BaseURL:      mustEnv("ENTITY_SERVICE_BASE_URL"),
		TokenURL:     oauthTokenURL,
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		Scopes:       splitComma(os.Getenv("ENTITY_SERVICE_SCOPES")),
	})

	// Email itself is not required for every deployment (a task with no
	// Report/AlertRecipients never calls it) — EMAIL_BASE_URL is read with
	// os.Getenv, not mustEnv, matching
	// integrations/csm-notification-service's own
	// internal/notifications.EmailClient. A missing/invalid configuration
	// only surfaces as an error the first time a task with recipients
	// actually finishes. Authenticates with the same shared OAUTH2_*
	// credentials as ledgerClient above, not its own — only
	// BaseURL/Scopes/FromAddress are specific to this client.
	emailClient := notify.NewClient(notify.Config{
		BaseURL:      os.Getenv("EMAIL_BASE_URL"),
		TokenURL:     oauthTokenURL,
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		Scopes:       splitComma(os.Getenv("EMAIL_SCOPES")),
		FromAddress:  os.Getenv("EMAIL_FROM_ADDRESS"),
	})

	// No sub-crons registered yet. See this component's own CLAUDE.md
	// ("Adding a sub-cron") for the steps — in short: write a
	// func(ctx context.Context) error handler, append a
	// registry.Task{Name, Schedule, Handler, ...} entry below with a
	// sensible hardcoded default Schedule, and optionally let ops override
	// that schedule per-task-by-name via the SUB_CRON_SCHEDULES env var
	// (parseSubCronSchedules/scheduleFor below already implement that
	// lookup — call scheduleFor(parseSubCronSchedules(os.Getenv("SUB_CRON_SCHEDULES")),
	// "<task.Name>", "<default>") for each task's Schedule field once one exists).
	tasks := []registry.Task{}

	for _, t := range tasks {
		if !gronx.IsValid(t.Schedule) {
			slog.Error("invalid cron schedule for registered task; refusing to start", "task", t.Name, "schedule", t.Schedule)
			os.Exit(1)
		}
	}

	eng := engine.New(tasks, ledgerClient, emailClient, driverInterval)

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

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// parseSubCronSchedules decodes SUB_CRON_SCHEDULES, a JSON object mapping a
// registered task's Name to a cron schedule override — one shared config
// value for every task in the registry, rather than a dedicated env var per
// task name that would need inventing again for every new sub-cron added
// here. A missing or malformed value logs a warning and yields no
// overrides, so every task just falls back to its own hardcoded default
// schedule — mirrors integrations/csm-notification-service's own
// parseGoogleChatSpaces (same "optional JSON env var, log and fall back on
// a bad value" shape).
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
func scheduleFor(overrides map[string]string, taskName, def string) string {
	if s, ok := overrides[taskName]; ok && s != "" {
		return s
	}
	return def
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
