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

package eventbus

import (
	"fmt"
	"log/slog"
	"strings"
)

// logWarn and logError bridge kafka.Writer/kafka.Reader's Logger/ErrorLogger
// (a bare Printf-style func) to slog, so broker warnings (e.g. disconnects,
// rebalances) and errors show up in this service's structured logs instead
// of nowhere — kafka-go, like the Kafka client used before it, logs nothing
// at all without one of these explicitly set.
func logWarn(msg string, args ...any) {
	slog.Warn(fmt.Sprintf(msg, args...))
}

func logError(msg string, args ...any) {
	slog.Error(fmt.Sprintf(msg, args...))
}

// idleFetchTimeoutPrefix is the exact prefix of the message kafka.Reader logs
// every time a long-poll fetch against an idle partition times out (once per
// partition per MaxWait interval, ~every 10s by default) — confirmed against
// the real Event Hub namespace: with 4 partitions and low traffic, this alone
// produced a WARN log line roughly every 2-3 seconds. It's expected protocol
// behavior (Kafka error code 7, "REQUEST_TIMED_OUT"), not something worth
// paging on, so readerLogWarn demotes it to DEBUG instead of dropping it
// outright — it's still visible if the log level is ever lowered.
const idleFetchTimeoutPrefix = "no messages received from kafka within the allocated time"

// readerLogWarn is kafka.Reader's Logger — unlike kafka.Writer, which has no
// equivalent routine noise, Reader logs the idle-timeout message above at the
// same level as genuinely useful warnings (rebalances, disconnects), so it
// needs its own filter rather than reusing logWarn directly.
func readerLogWarn(msg string, args ...any) {
	if strings.HasPrefix(msg, idleFetchTimeoutPrefix) {
		slog.Debug(fmt.Sprintf(msg, args...))
		return
	}
	slog.Warn(fmt.Sprintf(msg, args...))
}
