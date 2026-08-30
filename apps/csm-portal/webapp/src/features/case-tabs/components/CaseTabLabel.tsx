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

import { useEffect } from "react";
import { useGetCsmCaseDetail } from "@features/csm-cases/api/useGetCsmCaseDetail";
import { caseIdLabel } from "@features/csm-cases/utils/caseIdentity";
import { useCaseTabsController } from "@context/case-tabs/CaseTabsContext";
import type { CaseTabState } from "@context/case-tabs/caseTabsTypes";

/**
 * Resolves a tab's display label from the SAME `useGetCsmCaseDetail` query
 * `CsmCaseDetailPage` itself uses — sharing its React Query cache entry, so
 * this is a cache hit (no extra network call) once that page has loaded, and
 * a normal loading state before it has. Reports the label back into
 * `CaseTabsContext` (`setTabLabel`) so the tab strip can render it without
 * every tab chip needing its own render-time data fetch call inline.
 *
 * Deliberately does not touch `CsmCaseDetailPage` itself — keeping the very
 * large existing page untouched avoids risking a regression in it for this
 * feature.
 */
export default function CaseTabLabel({ tab }: { tab: CaseTabState }): null {
  const { setTabLabel } = useCaseTabsController();
  const { data } = useGetCsmCaseDetail(tab.caseId);

  useEffect(() => {
    if (!data) return;
    const idPart = caseIdLabel(data);
    const label = idPart ? `${idPart} · ${data.subject}` : data.subject;
    setTabLabel(tab.id, label);
  }, [data, tab.id, setTabLabel]);

  return null;
}
