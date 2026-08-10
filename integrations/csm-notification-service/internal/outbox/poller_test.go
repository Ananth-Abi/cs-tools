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
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/entityservice"
)

// fakeEntityServiceSearcher is a test double for EntityServiceSearcher.
type fakeEntityServiceSearcher struct {
	rows        []entityservice.EventOutboxRow
	searchErr   error
	statusErr   error
	statusCalls []statusCall
}

func (f *fakeEntityServiceSearcher) SearchWaitingEventOutbox(ctx context.Context, limit int) ([]entityservice.EventOutboxRow, error) {
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	return f.rows, nil
}

func (f *fakeEntityServiceSearcher) UpdateEventOutboxStatus(ctx context.Context, id, status string) error {
	f.statusCalls = append(f.statusCalls, statusCall{id, status})
	return f.statusErr
}

func TestPollOnce_DispatchesRowsOldEnough(t *testing.T) {
	pub := &fakePublisher{}
	es := &fakeEntityServiceSearcher{
		rows: []entityservice.EventOutboxRow{
			{ID: "outbox-1", EventType: "case.created", EntityID: "CASE-1", Payload: []byte(`{"a":1}`), CreatedOn: time.Now().Add(-1 * time.Minute)},
		},
	}
	p := &Poller{Publisher: pub, EntityService: es, MinAge: 10 * time.Second, Limit: 50}
	p.pollOnce(t.Context())

	if pub.callCount != 1 {
		t.Fatalf("Publish called %d times, want 1", pub.callCount)
	}
	if string(pub.key) != "CASE-1" {
		t.Errorf("publish key = %q, want %q", pub.key, "CASE-1")
	}
	want := []statusCall{
		{"outbox-1", StatusDispatching},
		{"outbox-1", StatusDispatched},
	}
	if len(es.statusCalls) != len(want) {
		t.Fatalf("entity-service calls = %v, want %v", es.statusCalls, want)
	}
	for i, c := range want {
		if es.statusCalls[i] != c {
			t.Errorf("call %d = %v, want %v", i, es.statusCalls[i], c)
		}
	}
}

func TestPollOnce_StopsAtFirstRowYoungerThanMinAge(t *testing.T) {
	pub := &fakePublisher{}
	es := &fakeEntityServiceSearcher{
		rows: []entityservice.EventOutboxRow{
			{ID: "outbox-old", EventType: "case.created", EntityID: "CASE-1", Payload: []byte(`{}`), CreatedOn: time.Now().Add(-1 * time.Minute)},
			{ID: "outbox-fresh", EventType: "case.created", EntityID: "CASE-2", Payload: []byte(`{}`), CreatedOn: time.Now()},
		},
	}
	p := &Poller{Publisher: pub, EntityService: es, MinAge: 10 * time.Second, Limit: 50}
	p.pollOnce(t.Context())

	if pub.callCount != 1 {
		t.Fatalf("Publish called %d times, want 1 (only the old-enough row)", pub.callCount)
	}
	if string(pub.key) != "CASE-1" {
		t.Errorf("publish key = %q, want %q (the old row, not the fresh one)", pub.key, "CASE-1")
	}
}

func TestPollOnce_SearchError_DoesNotPanicOrDispatch(t *testing.T) {
	pub := &fakePublisher{}
	es := &fakeEntityServiceSearcher{searchErr: errors.New("entity-service unreachable")}
	p := &Poller{Publisher: pub, EntityService: es, MinAge: 10 * time.Second, Limit: 50}
	p.pollOnce(t.Context())

	if pub.callCount != 0 {
		t.Errorf("Publish called %d times, want 0", pub.callCount)
	}
}

func TestPollOnce_ClaimConflict_SkipsWithoutError(t *testing.T) {
	pub := &fakePublisher{}
	es := &fakeEntityServiceSearcher{
		rows: []entityservice.EventOutboxRow{
			{ID: "outbox-1", EventType: "case.created", EntityID: "CASE-1", Payload: []byte(`{}`), CreatedOn: time.Now().Add(-1 * time.Minute)},
		},
		statusErr: &apierror.Error{StatusCode: http.StatusConflict, Body: "already claimed"},
	}
	p := &Poller{Publisher: pub, EntityService: es, MinAge: 10 * time.Second, Limit: 50}
	p.pollOnce(t.Context())

	if pub.callCount != 0 {
		t.Errorf("Publish called %d times, want 0 (row already claimed elsewhere)", pub.callCount)
	}
}

func TestPoller_Run_StopsOnContextCancel(t *testing.T) {
	pub := &fakePublisher{}
	es := &fakeEntityServiceSearcher{}
	p := &Poller{Publisher: pub, EntityService: es, Interval: 5 * time.Millisecond, MinAge: 0, Limit: 50}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		p.Run(ctx)
		close(done)
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}
