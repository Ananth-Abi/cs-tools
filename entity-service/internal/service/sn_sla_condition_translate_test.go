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

package service

import (
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/slacondition"
)

func TestTranslateSLACondition(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "strips the trailing EQ terminator",
			raw:  "state=1^priority=10^EQ",
			want: "state=OPEN^priority=S1",
		},
		{
			name: "translates case_type sys_id and strips u_ prefix",
			raw:  "u_case_type=8d4b87bd1b18f010cb6898aebd4bcb59^state=1^EQ",
			want: "case_type=case^state=OPEN",
		},
		{
			name: "translates product sys_ids across an OR run",
			raw:  "product=691b4f861b746010cb6898aebd4bcbc0^ORproduct=720bf91d1bf92010cb6898aebd4bcb10^priority=10^EQ",
			want: "product=WSO2 Developer Platform^ORproduct=Asgardeo^priority=S1",
		},
		{
			name: "translates project.u_project_type dot-walk field and sys_id",
			raw:  "project.u_project_type=cb4a87bd1b18f010cb6898aebd4bcb4f^ORproject.u_project_type=87a8676c1b347410cb6898aebd4bcb76^EQ",
			want: "project_type=Subscription^ORproject_type=Managed Cloud Subscription",
		},
		{
			name: "renames u_wso2_response_comment",
			raw:  "state=1^u_wso2_response_comment=false^EQ",
			want: "state=OPEN^cs_engineer_response_comment=false",
		},
		{
			name: "renames u_fix_eta_shared and keeps ISNOTEMPTY suffix",
			raw:  "state=3^ORu_fix_eta_sharedISNOTEMPTY^EQ",
			want: "state=CLOSED^ORfix_eta_sharedISNOTEMPTY",
		},
		{
			name: "drops a clause naming the dead case_type sys_id",
			raw:  "case_type!=3f5b47bd1b18f010cb6898aebd4bcbc2^case_type!=8d4b87bd1b18f010cb6898aebd4bcb59^EQ",
			want: "case_type!=case",
		},
		{
			name: "keeps a malformed, value-less trailing clause as a false disjunct rather than dropping it",
			raw:  "priority!=S4^ORstate=",
			want: "priority!=S4^ORstate=",
		},
		{
			name: "strips the javascript date-literal wrapper",
			raw:  "sys_created_on>=javascript:gs.dateGenerate('2022-08-26','00:00:00')^EQ",
			want: "sys_created_on>=2022-08-26",
		},
		{
			name: "translates IN lists member by member",
			raw:  "stateINWORK_IN_PROGRESS,AWAITING_INFO^EQ",
			want: "stateINWORK_IN_PROGRESS,AWAITING_INFO",
		},
		{
			name: "translates numeric IN list members",
			raw:  "stateIN10,18,1009^EQ",
			want: "stateINWORK_IN_PROGRESS,AWAITING_INFO,1009",
		},
		{
			name: "preserves NQ disjunction boundaries",
			raw:  "state=1^priority=10^NQstate=1^u_case_type=ab36479047ccf510a0a29cd3846d43ee^EQ",
			want: "state=OPEN^priority=S1^NQstate=OPEN^case_type=security_report_analysis",
		},
		{
			name: "is idempotent on already-neutral text",
			raw:  "state=OPEN^case_type=case^priority=S1",
			want: "state=OPEN^case_type=case^priority=S1",
		},
		{
			name: "passes through an unmapped value unchanged",
			raw:  "state=OPEN^product=unknown-sys-id",
			want: "state=OPEN^product=unknown-sys-id",
		},
		{
			name: "empty input stays empty",
			raw:  "",
			want: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := translateSLACondition(c.raw)
			if got != c.want {
				t.Errorf("translateSLACondition(%q) = %q, want %q", c.raw, got, c.want)
			}
		})
	}
}

// TestTranslateSLACondition_RealRowsParseWithNoUnknownClauses runs every raw
// shape actually observed in the real sla_policy table (migrations
// 0250-0260's own backup/UPDATE statements are the source for these) through
// translateSLACondition then slacondition.Parse, and asserts no unknown
// clause survives translation -- the regression test for the exact blocker a
// PR review caught: an untranslated "^EQ" terminator made every one of these
// evaluate to false.
func TestTranslateSLACondition_RealRowsParseWithNoUnknownClauses(t *testing.T) {
	raws := []string{
		"u_case_type=8d4b87bd1b18f010cb6898aebd4bcb59^state=1^priority=10^u_wso2_response_comment=false^EQ",
		"product=691b4f861b746010cb6898aebd4bcbc0^ORproduct=720bf91d1bf92010cb6898aebd4bcb10^ORproduct=6436091d1b8ae210a002c9d3604bcb45^ORproduct=7bf5811d1b8ae210a002c9d3604bcbc8^state=1^priority=10^EQ",
		"project.u_project_type=cb4a87bd1b18f010cb6898aebd4bcb4f^ORproject.u_project_type=87a8676c1b347410cb6898aebd4bcb76^u_case_type=8d4b87bd1b18f010cb6898aebd4bcb59^priority=10^EQ",
		"stateINAWAITING_INFO,SOLUTION_PROPOSED^ORu_workaround_providedISNOTEMPTY^EQ",
		"u_workaround_providedISEMPTY^stateINWORK_IN_PROGRESS,WAITING_ON_WSO2,REOPENED^EQ",
		"priority=S1^u_case_type=8d4b87bd1b18f010cb6898aebd4bcb59^ORu_case_type=bfa1473c1bbcb410cb6898aebd4bcb52^sys_created_on>=javascript:gs.dateGenerate('2022-08-26','00:00:00')^EQ",
		"u_case_type!=3f5b47bd1b18f010cb6898aebd4bcbc2^u_case_type!=8d4b87bd1b18f010cb6898aebd4bcb59^state=1^u_case_typeISNOTEMPTY^EQ",
		"state=1^priority=10^u_case_type=8d4b87bd1b18f010cb6898aebd4bcb59^u_wso2_response_comment=false^NQstate=1^u_case_type=ab36479047ccf510a0a29cd3846d43ee^u_wso2_response_comment=false^EQ",
		"product!=691b4f861b746010cb6898aebd4bcbc0^product=720bf91d1bf92010cb6898aebd4bcb10^product=6436091d1b8ae210a002c9d3604bcb45^product=7bf5811d1b8ae210a002c9d3604bcbc8^ORpriority!=13^ORstate=",
	}
	for _, raw := range raws {
		translated := translateSLACondition(raw)
		cond := slacondition.Parse(translated)
		if unknown := cond.UnknownClauses(); len(unknown) > 0 {
			t.Errorf("translateSLACondition(%q) = %q, left unknown clauses: %v", raw, translated, unknown)
		}
	}
}
