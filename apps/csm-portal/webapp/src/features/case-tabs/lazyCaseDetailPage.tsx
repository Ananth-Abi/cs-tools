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

import { lazy } from "react";

/**
 * A second `lazy()` wrapper around the same `CsmCaseDetailPage` module
 * `App.tsx` already lazy-loads for its own (now-unused-for-rendering, see
 * `CaseDetailRouteSync`) case-detail routes. A separate `lazy()` call still
 * shares the SAME underlying dynamic `import()` — and so the same network
 * chunk — as `App.tsx`'s; it does not double the bundle or the fetch. Kept
 * as its own module (rather than exporting `App.tsx`'s local const) so this
 * feature doesn't reach into the router config module for an implementation
 * detail.
 */
const LazyCsmCaseDetailPage = lazy(
  () => import("@features/csm-cases/pages/CsmCaseDetailPage"),
);

export default LazyCsmCaseDetailPage;
