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

import { Button } from "@wso2/oxygen-ui";
import { Plus } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";

import CsmIssuesView from "@features/csm-cases/components/CsmIssuesView";
import { useNavTransition } from "@hooks/useNavTransition";

/** All-cases list — the shared issues view across every case type. */
export default function CsmCasesPage(): JSX.Element {
  const navigate = useNavTransition();

  return (
    <CsmIssuesView
      title="Cases"
      entityNoun="cases"
      // Cases list defaults to support cases (`caseTypes: ["case"]`) but,
      // unlike the other issue-type pages (Operations/Security Center/
      // Engagements, which exist purely to be locked to one type and hide
      // the control), is the one unlocked, multi-type `CsmIssuesView`: the
      // type control is left visible, and `CsmIssuesView` only applies this
      // `lockedFilters.caseTypes` value to its severity-filter-visibility /
      // column-default hints, not to the query itself (see its own
      // `hideTypeFilter` handling) — so picking a different type here
      // genuinely narrows the results, per digiops-cs#2907. Clearing the
      // control back to no selection falls through to "every type" (see
      // `CsmIssuesView`'s own `ALL_CASE_TYPES` fallback), not back to "Case
      // only" — a deliberate choice to keep the empty state meaning "no
      // filter applied" consistently with every other filter on this bar,
      // rather than a hidden implicit default the user can't see or
      // override. Flag to the product owner if "Case only" should instead be
      // the sticky default even once cleared.
      lockedFilters={{ caseTypes: ["case"] }}
      enableColumnCustomization
      columnsViewId="cases"
      actions={
        <Button
          variant="contained"
          color="primary"
          size="small"
          startIcon={<Plus size={16} />}
          onClick={() => navigate("/cases/new")}
        >
          Create case
        </Button>
      }
    />
  );
}
