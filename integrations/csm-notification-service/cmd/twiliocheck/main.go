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

// Command twiliocheck manually verifies TwilioClient against a real Twilio
// account: it sends one real SMS or places one real voice call. This is NOT
// an automated test — it makes a real, billed request to Twilio's live API
// and is never run by `go test` or CI. Use it only for one-off manual
// verification after changing twilio.go, e.g.:
//
//	TWILIO_ACCOUNT_SID=... \
//	TWILIO_AUTH_TOKEN=... \
//	TWILIO_MESSAGING_SERVICE_SID=... \ # sms, preferred; or TWILIO_FROM_NUMBER
//	TWILIO_TO_NUMBER=+1... \
//	go run ./cmd/twiliocheck -channel=sms
//
//	TWILIO_ACCOUNT_SID=... \
//	TWILIO_AUTH_TOKEN=... \
//	TWILIO_FROM_NUMBER=+1... \ # always required for call — no MessagingServiceSid equivalent
//	TWILIO_TO_NUMBER=+1... \
//	go run ./cmd/twiliocheck -channel=call -voice=Polly.Raveena -language=en-IN
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

func main() {
	loadDotEnv(".env")

	channel := flag.String("channel", "", `channel to test: "sms" or "call"`)
	message := flag.String("message", "Test notification from csm-notification-service's twiliocheck tool.", "message text to send, or speak on a call")
	voice := flag.String("voice", "", `-channel=call only: TTS voice override (e.g. "Polly.Raveena"); empty uses the account/env default`)
	language := flag.String("language", "", `-channel=call only: TTS language override (e.g. "en-IN"); empty uses the account/env default`)
	flag.Parse()

	to := os.Getenv("TWILIO_TO_NUMBER")
	if to == "" {
		fmt.Fprintln(os.Stderr, "TWILIO_TO_NUMBER is required (E.164, e.g. +14155552671)")
		os.Exit(1)
	}

	cfg := notifications.TwilioConfig{
		AccountSID:          os.Getenv("TWILIO_ACCOUNT_SID"),
		AuthToken:           os.Getenv("TWILIO_AUTH_TOKEN"),
		FromNumber:          os.Getenv("TWILIO_FROM_NUMBER"),
		MessagingServiceSid: os.Getenv("TWILIO_MESSAGING_SERVICE_SID"),
		Voice:               envOrFlag("TWILIO_VOICE", *voice),
		Language:            envOrFlag("TWILIO_LANGUAGE", *language),
		APIBaseURL:          os.Getenv("TWILIO_API_BASE_URL"),
	}
	client := notifications.NewTwilioClient(cfg)

	var err error
	switch *channel {
	case "sms":
		err = client.SendSMS(context.Background(), to, *message)
	case "call":
		err = client.MakeCall(context.Background(), to, *message)
	default:
		fmt.Fprintln(os.Stderr, `-channel must be "sms" or "call"`)
		os.Exit(1)
	}

	if err != nil {
		fmt.Println(*channel, "failed:", err)
		os.Exit(1)
	}
	fmt.Println(*channel, "succeeded.")
}

// envOrFlag prefers an explicitly-passed flag value, falling back to the
// named environment variable so a fixed TWILIO_VOICE/TWILIO_LANGUAGE in
// .env doesn't need to be repeated on the command line for every run.
func envOrFlag(envKey, flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return os.Getenv(envKey)
}

// loadDotEnv reads a .env file and sets any unset environment variables from
// it, matching cmd/server/main.go's own loadDotEnv — so credentials/config
// set once in a shared .env work for both. Silently ignored if the file does
// not exist; logs a warning for any other error.
func loadDotEnv(path string) {
	f, err := os.Open(path) // #nosec G304 -- path is always the hardcoded literal ".env" at the only call site
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("loadDotEnv: failed to open .env file", "err", err)
		}
		return
	}
	defer f.Close()
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
		// Strip surrounding quotes from value.
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
