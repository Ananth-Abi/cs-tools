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
	"context"
	"errors"
	"testing"
	"time"
)

// testRetryDelay is short so exhaustion tests don't actually wait
// handleRetryDelay (2s) between attempts.
const testRetryDelay = time.Millisecond

func TestProcessRecord_SucceedsFirstAttempt(t *testing.T) {
	calls := 0
	handle := func(ctx context.Context, r Record) error {
		calls++
		return nil
	}
	ok := processRecord(context.Background(), Record{}, handle, nil, 3, testRetryDelay)
	if !ok {
		t.Error("processRecord() = false, want true")
	}
	if calls != 1 {
		t.Errorf("handle called %d times, want 1", calls)
	}
}

func TestProcessRecord_SucceedsAfterRetries(t *testing.T) {
	calls := 0
	handle := func(ctx context.Context, r Record) error {
		calls++
		if calls < 3 {
			return errors.New("transient failure")
		}
		return nil
	}
	ok := processRecord(context.Background(), Record{}, handle, nil, 3, testRetryDelay)
	if !ok {
		t.Error("processRecord() = false, want true")
	}
	if calls != 3 {
		t.Errorf("handle called %d times, want 3", calls)
	}
}

func TestProcessRecord_ExhaustedWithNilOnExhausted_StillCommits(t *testing.T) {
	calls := 0
	handle := func(ctx context.Context, r Record) error {
		calls++
		return errors.New("persistent failure")
	}
	ok := processRecord(context.Background(), Record{}, handle, nil, 3, testRetryDelay)
	if !ok {
		t.Error("processRecord() = false, want true (should still commit even when dropping)")
	}
	if calls != 3 {
		t.Errorf("handle called %d times, want 3 (handleAttempts)", calls)
	}
}

func TestProcessRecord_ExhaustedCallsOnExhausted(t *testing.T) {
	handle := func(ctx context.Context, r Record) error {
		return errors.New("persistent failure")
	}
	var gotRecord Record
	var gotErr error
	onExhausted := func(ctx context.Context, record Record, handleErr error) error {
		gotRecord = record
		gotErr = handleErr
		return nil
	}
	record := Record{Topic: "case-events", Partition: 2, Offset: 42}
	ok := processRecord(context.Background(), record, handle, onExhausted, 3, testRetryDelay)
	if !ok {
		t.Error("processRecord() = false, want true")
	}
	if gotRecord.Topic != "case-events" || gotRecord.Partition != 2 || gotRecord.Offset != 42 {
		t.Errorf("onExhausted got record = %+v, want the original record's identity preserved", gotRecord)
	}
	if !gotRecord.IsFinalAttempt {
		t.Error("onExhausted's record.IsFinalAttempt = false, want true")
	}
	if gotErr == nil || gotErr.Error() != "persistent failure" {
		t.Errorf("onExhausted handleErr = %v, want the last handle error", gotErr)
	}
}

func TestProcessRecord_OnExhaustedFailure_StillCommits(t *testing.T) {
	handle := func(ctx context.Context, r Record) error {
		return errors.New("persistent failure")
	}
	onExhausted := func(ctx context.Context, record Record, handleErr error) error {
		return errors.New("dead-letter topic unreachable")
	}
	ok := processRecord(context.Background(), Record{}, handle, onExhausted, 3, testRetryDelay)
	if !ok {
		t.Error("processRecord() = false, want true (nowhere lower to fall back to, so still commit)")
	}
}

func TestProcessRecord_ContextCanceledMidRetry_DoesNotCommit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	handle := func(ctx context.Context, r Record) error {
		calls++
		if calls == 1 {
			cancel()
		}
		return errors.New("failure")
	}
	ok := processRecord(ctx, Record{}, handle, nil, 3, 50*time.Millisecond)
	if ok {
		t.Error("processRecord() = true, want false when ctx is canceled mid-retry-wait")
	}
	if calls != 1 {
		t.Errorf("handle called %d times, want 1 (should stop retrying once ctx is canceled)", calls)
	}
}

func TestProcessRecord_IsFinalAttemptOnlyOnLastCall(t *testing.T) {
	var finalFlags []bool
	handle := func(ctx context.Context, r Record) error {
		finalFlags = append(finalFlags, r.IsFinalAttempt)
		return errors.New("keep failing")
	}
	processRecord(context.Background(), Record{}, handle, nil, 3, testRetryDelay)
	want := []bool{false, false, true}
	if len(finalFlags) != len(want) {
		t.Fatalf("got %d attempts, want %d", len(finalFlags), len(want))
	}
	for i, w := range want {
		if finalFlags[i] != w {
			t.Errorf("attempt %d: IsFinalAttempt = %v, want %v", i+1, finalFlags[i], w)
		}
	}
}
