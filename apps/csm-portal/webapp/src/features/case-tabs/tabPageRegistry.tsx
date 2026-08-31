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

import { lazy, type ComponentType } from "react";
import type { CaseRouteKind } from "@context/case-tabs/caseTabsTypes";

/**
 * Which page component renders for which open-tab kind. The five case-like
 * kinds all share `CsmCaseDetailPage`; `incident` and `change_request` each
 * have their own dedicated page. Every entry is its own `lazy()` — a second
 * `lazy()` wrapper around a module `App.tsx` also lazy-imports still shares
 * the same underlying dynamic `import()` (same network chunk), it doesn't
 * double the bundle — see the (now-superseded, single-page) version of this
 * file's own note for the same reasoning.
 */
const LazyCsmCaseDetailPage = lazy(
  () => import("@features/csm-cases/pages/CsmCaseDetailPage"),
);
const LazyCsmIncidentDetailPage = lazy(
  () => import("@features/csm-operations/pages/CsmIncidentDetailPage"),
);
const LazyCsmChangeRequestDetailPage = lazy(
  () => import("@features/csm-operations/pages/CsmChangeRequestDetailPage"),
);

/** Returns the lazy-loaded page component an open tab of this kind renders. */
export function pageComponentForKind(kind: CaseRouteKind): ComponentType {
  switch (kind) {
    case "incident":
      return LazyCsmIncidentDetailPage;
    case "change_request":
      return LazyCsmChangeRequestDetailPage;
    case "case":
    case "service_request":
    case "engagement":
    case "announcement":
    case "security_report_analysis":
      return LazyCsmCaseDetailPage;
  }
}
