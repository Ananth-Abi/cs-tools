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

package outbox

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
)

type fakePublisher struct {
	err        error
	callCount  int
	key, value []byte
}

func (f *fakePublisher) Publish(ctx context.Context, key, value []byte) error {
	f.callCount++
	f.key, f.value = key, value
	return f.err
}

type statusCall struct{ id, status string }

type fakeStatusUpdater struct {
	err     error
	errFunc func(status string) error // takes priority over err when set
	calls   []statusCall
}

func (f *fakeStatusUpdater) UpdateEventOutboxStatus(ctx context.Context, id, status string) error {
	f.calls = append(f.calls, statusCall{id, status})
	if f.errFunc != nil {
		return f.errFunc(status)
	}
	return f.err
}

func TestDispatch_NoEventID_PublishesUnconditionally(t *testing.T) {
	pub := &fakePublisher{}
	es := &fakeStatusUpdater{}
	published, err := Dispatch(t.Context(), pub, es, "", []byte("key"), []byte("value"))
	if err != nil {
		t.Fatalf("Dispatch returned err = %v, want nil", err)
	}
	if !published {
		t.Error("published = false, want true")
	}
	if pub.callCount != 1 {
		t.Errorf("Publish called %d times, want 1", pub.callCount)
	}
	if len(es.calls) != 0 {
		t.Errorf("expected no entity-service calls when eventID is empty, got %v", es.calls)
	}
}

func TestDispatch_NilStatusUpdater_PublishesUnconditionally(t *testing.T) {
	pub := &fakePublisher{}
	published, err := Dispatch(t.Context(), pub, nil, "outbox-1", []byte("key"), []byte("value"))
	if err != nil {
		t.Fatalf("Dispatch returned err = %v, want nil", err)
	}
	if !published {
		t.Error("published = false, want true")
	}
	if pub.callCount != 1 {
		t.Errorf("Publish called %d times, want 1", pub.callCount)
	}
}

func TestDispatch_ClaimsBeforePublishAndMarksDispatched(t *testing.T) {
	pub := &fakePublisher{}
	es := &fakeStatusUpdater{}
	published, err := Dispatch(t.Context(), pub, es, "outbox-1", []byte("CASE-1"), []byte("body"))
	if err != nil {
		t.Fatalf("Dispatch returned err = %v, want nil", err)
	}
	if !published {
		t.Error("published = false, want true")
	}
	if pub.callCount != 1 {
		t.Fatalf("Publish called %d times, want 1", pub.callCount)
	}
	want := []statusCall{
		{"outbox-1", StatusDispatching},
		{"outbox-1", StatusDispatched},
	}
	if len(es.calls) != len(want) {
		t.Fatalf("entity-service calls = %v, want %v", es.calls, want)
	}
	for i, c := range want {
		if es.calls[i] != c {
			t.Errorf("call %d = %v, want %v", i, es.calls[i], c)
		}
	}
}

func TestDispatch_ClaimConflict_SkipsPublish(t *testing.T) {
	pub := &fakePublisher{}
	es := &fakeStatusUpdater{err: &apierror.Error{StatusCode: http.StatusConflict, Body: "already claimed"}}
	published, err := Dispatch(t.Context(), pub, es, "outbox-1", []byte("CASE-1"), []byte("body"))
	if err != nil {
		t.Fatalf("Dispatch returned err = %v, want nil on conflict", err)
	}
	if published {
		t.Error("published = true, want false on conflict")
	}
	if pub.callCount != 0 {
		t.Errorf("Publish called %d times, want 0", pub.callCount)
	}
	if len(es.calls) != 1 || es.calls[0].status != StatusDispatching {
		t.Errorf("entity-service calls = %v, want a single claim attempt", es.calls)
	}
}

func TestDispatch_ClaimFails_ReturnsErrorWithoutPublishing(t *testing.T) {
	pub := &fakePublisher{}
	es := &fakeStatusUpdater{err: errors.New("entity-service unreachable")}
	published, err := Dispatch(t.Context(), pub, es, "outbox-1", []byte("CASE-1"), []byte("body"))
	if err == nil {
		t.Fatal("Dispatch returned nil err, want a non-nil error")
	}
	if published {
		t.Error("published = true, want false")
	}
	if pub.callCount != 0 {
		t.Errorf("Publish called %d times, want 0", pub.callCount)
	}
}

func TestDispatch_PublishFails_ReleasesClaimAndReturnsError(t *testing.T) {
	pub := &fakePublisher{err: errors.New("event hub unreachable")}
	es := &fakeStatusUpdater{}
	published, err := Dispatch(t.Context(), pub, es, "outbox-1", []byte("CASE-1"), []byte("body"))
	if err == nil {
		t.Fatal("Dispatch returned nil err, want a non-nil error")
	}
	if published {
		t.Error("published = true, want false")
	}
	want := []statusCall{
		{"outbox-1", StatusDispatching},
		{"outbox-1", StatusWaiting},
	}
	if len(es.calls) != len(want) {
		t.Fatalf("entity-service calls = %v, want %v", es.calls, want)
	}
	for i, c := range want {
		if es.calls[i] != c {
			t.Errorf("call %d = %v, want %v", i, es.calls[i], c)
		}
	}
}

func TestDispatch_PublishFails_ReleaseAlsoFails_StillReturnsPublishError(t *testing.T) {
	publishErr := errors.New("event hub unreachable")
	pub := &fakePublisher{err: publishErr}
	es := &fakeStatusUpdater{errFunc: func(status string) error {
		if status == StatusWaiting {
			return errors.New("entity-service unreachable during release")
		}
		return nil
	}}
	published, err := Dispatch(t.Context(), pub, es, "outbox-1", []byte("CASE-1"), []byte("body"))
	if !errors.Is(err, publishErr) {
		t.Fatalf("Dispatch err = %v, want it to wrap %v (the publish failure) even though the release also failed", err, publishErr)
	}
	if published {
		t.Error("published = true, want false")
	}
}

func TestDispatch_MarkDispatchedFails_StillReturnsSuccess(t *testing.T) {
	pub := &fakePublisher{}
	es := &fakeStatusUpdater{errFunc: func(status string) error {
		if status == StatusDispatched {
			return errors.New("entity-service unreachable while marking dispatched")
		}
		return nil
	}}
	published, err := Dispatch(t.Context(), pub, es, "outbox-1", []byte("CASE-1"), []byte("body"))
	if err != nil {
		t.Fatalf("Dispatch returned err = %v, want nil (publish succeeded; mark-dispatched failure is best-effort)", err)
	}
	if !published {
		t.Error("published = false, want true")
	}
}

func TestIsConflict(t *testing.T) {
	cases := map[string]struct {
		err  error
		want bool
	}{
		"conflict":       {&apierror.Error{StatusCode: http.StatusConflict}, true},
		"internal error": {&apierror.Error{StatusCode: http.StatusInternalServerError}, false},
		"plain error":    {errors.New("boom"), false},
		"nil":            {nil, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := IsConflict(c.err); got != c.want {
				t.Errorf("IsConflict(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}
