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

package autoclosure

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeWork struct {
	due        map[string][]Candidate // by stage
	listErr    map[string]error
	advanceErr map[string]error // by ticket id
	outcome    map[string]Outcome
	listed     []CandidatesRequest
	advanced   []string // "stage:id"
	advanceReq []AdvanceRequest
}

func (f *fakeWork) Candidates(_ context.Context, req CandidatesRequest) ([]Candidate, error) {
	f.listed = append(f.listed, req)
	if err := f.listErr[req.Stage]; err != nil {
		return nil, err
	}
	return f.due[req.Stage], nil
}

func (f *fakeWork) Advance(_ context.Context, id string, req AdvanceRequest) (Outcome, error) {
	f.advanced = append(f.advanced, req.Stage+":"+id)
	f.advanceReq = append(f.advanceReq, req)
	if err := f.advanceErr[id]; err != nil {
		return "", err
	}
	if o, ok := f.outcome[id]; ok {
		return o, nil
	}
	return OutcomeAdvanced, nil
}

func ids(stage string, n ...string) []Candidate {
	var out []Candidate
	for _, id := range n {
		out = append(out, Candidate{ID: id, Number: "CS-" + id, State: "awaiting_info"})
	}
	return out
}

func TestRun_NothingDueIsASuccess(t *testing.T) {
	f := &fakeWork{}
	if err := Run(f, RunConfig{})(context.Background()); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(f.advanced) != 0 {
		t.Errorf("advanced %v, want none", f.advanced)
	}
}

func TestRun_StagesRunInOrderAndEveryDueTicketIsAdvanced(t *testing.T) {
	f := &fakeWork{due: map[string][]Candidate{
		StageReleaseHold:    ids("h", "h1"),
		StageClose:          ids("c", "c1", "c2"),
		StageSecondReminder: ids("s", "s1"),
		StageFirstReminder:  ids("f", "f1", "f2", "f3"),
	}}
	if err := Run(f, RunConfig{IntervalDays: 7, DailyLimit: 500})(context.Background()); err != nil {
		t.Fatalf("err = %v", err)
	}
	want := []string{
		"release_hold:h1", "close:c1", "close:c2", "second_reminder:s1",
		"first_reminder:f1", "first_reminder:f2", "first_reminder:f3",
	}
	if strings.Join(f.advanced, ",") != strings.Join(want, ",") {
		t.Errorf("advanced %v, want %v", f.advanced, want)
	}
	var stages []string
	for _, l := range f.listed {
		stages = append(stages, l.Stage)
		if l.DailyLimit != 500 || l.IntervalDays != 7 {
			t.Errorf("listing for %s sent %+v, want the configured limit and interval", l.Stage, l)
		}
	}
	if strings.Join(stages, ",") != "release_hold,close,second_reminder,first_reminder" {
		t.Errorf("stage order = %v", stages)
	}
}

func TestRun_ConfigReachesEntityService(t *testing.T) {
	f := &fakeWork{due: map[string][]Candidate{StageFirstReminder: ids("f", "f1")}}
	cfg := RunConfig{States: []string{"solution_proposed"}, Templates: map[string]string{"awaiting_info.closure": "bye"}}
	if err := Run(f, cfg)(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := f.listed[0].States; len(got) != 1 || got[0] != "solution_proposed" {
		t.Errorf("listing states = %v, want only solution_proposed", got)
	}
	if got := f.advanceReq[0]; got.Templates["awaiting_info.closure"] != "bye" || len(got.States) != 1 {
		t.Errorf("advance = %+v, want the same states and the template override", got)
	}
}

// One ticket failing must not stop the others, and the run still reports it so
// the engine alerts and retries.
func TestRun_OneFailureDoesNotStopTheRest(t *testing.T) {
	f := &fakeWork{
		due:        map[string][]Candidate{StageFirstReminder: ids("f", "f1", "f2", "f3")},
		advanceErr: map[string]error{"f2": errors.New("boom")},
	}
	err := Run(f, RunConfig{})(context.Background())
	if err == nil || !strings.Contains(err.Error(), "CS-f2") {
		t.Fatalf("err = %v, want it to name the failed ticket", err)
	}
	if len(f.advanced) != 3 {
		t.Errorf("advanced %v, want all three attempted", f.advanced)
	}
}

func TestRun_AFailedListingStillRunsTheOtherStages(t *testing.T) {
	f := &fakeWork{
		due:     map[string][]Candidate{StageFirstReminder: ids("f", "f1")},
		listErr: map[string]error{StageClose: errors.New("list boom")},
	}
	err := Run(f, RunConfig{})(context.Background())
	if err == nil || !strings.Contains(err.Error(), "close") {
		t.Fatalf("err = %v, want the failed listing named", err)
	}
	if len(f.advanced) != 1 || f.advanced[0] != "first_reminder:f1" {
		t.Errorf("advanced %v, want the first reminder still sent", f.advanced)
	}
}

func TestRun_ASkippedTicketIsNotAnError(t *testing.T) {
	f := &fakeWork{
		due:     map[string][]Candidate{StageClose: ids("c", "c1")},
		outcome: map[string]Outcome{"c1": OutcomeSkipped},
	}
	if err := Run(f, RunConfig{})(context.Background()); err != nil {
		t.Fatalf("a ticket another run handled first is expected, got err %v", err)
	}
}

func TestRun_FailureAlertIsBounded(t *testing.T) {
	var many []string
	failures := map[string]error{}
	for i := 0; i < 40; i++ {
		id := string(rune('a'+i%26)) + string(rune('a'+i/26))
		many = append(many, id)
		failures[id] = errors.New("boom")
	}
	f := &fakeWork{due: map[string][]Candidate{StageFirstReminder: ids("f", many...)}, advanceErr: failures}
	err := Run(f, RunConfig{})(context.Background())
	if err == nil {
		t.Fatal("want an error")
	}
	if lines := strings.Count(err.Error(), "\n") + 1; lines > maxErrorsReported+1 {
		t.Errorf("alert has %d lines, want at most %d", lines, maxErrorsReported+1)
	}
	if !strings.Contains(err.Error(), "and 30 more failures") {
		t.Errorf("alert does not say how many were left out: %v", err)
	}
}

func TestRun_StopsWhenTheContextEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f := &fakeWork{due: map[string][]Candidate{StageClose: ids("c", "c1")}}
	if err := Run(f, RunConfig{})(ctx); err == nil {
		t.Fatal("a cancelled run must not report success")
	}
	if len(f.advanced) != 0 {
		t.Errorf("advanced %v after cancellation", f.advanced)
	}
}
