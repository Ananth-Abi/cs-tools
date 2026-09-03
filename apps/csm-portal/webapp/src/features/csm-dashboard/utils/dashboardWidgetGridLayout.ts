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

import type { BeDashboardWidget } from "@api/backend/types";

/** 12-column grid, matching each widget's own `gridWidth`; on very small
 * screens there's only room for 4 columns, so a wide widget there wraps to
 * (at most) one extra row rather than overflowing. Shared by
 * `DashboardWidgetGrid` (the real grid) and every page that shows a
 * same-shaped loading skeleton before that grid's own data has resolved. */
export const WIDGET_GRID_SX = {
  display: "grid",
  gap: 1.5,
  gridTemplateColumns: {
    xs: "repeat(4, minmax(0, 1fr))",
    sm: "repeat(12, minmax(0, 1fr))",
  },
} as const;

export interface WidgetGroup {
  /** `undefined` for the untitled/default group — every widget with no
   * `section` set lands here, rendered exactly as before this field
   * existed (no heading). */
  section?: string;
  widgets: BeDashboardWidget[];
}

/** Groups widgets by `section`, preserving the order each distinct section
 * value (including the untitled default) first appears among `widgets` —
 * see `BeDashboardWidget.section`. Exported (in its own non-component
 * module, not `DashboardWidgetGrid.tsx` itself) so the dashboard builder
 * can derive the same section-name list a live dashboard would render,
 * without duplicating this grouping rule. */
export function groupWidgetsBySection(widgets: BeDashboardWidget[]): WidgetGroup[] {
  const groups: WidgetGroup[] = [];
  const indexBySection = new Map<string | undefined, number>();
  for (const widget of widgets) {
    const key = widget.section || undefined;
    let index = indexBySection.get(key);
    if (index === undefined) {
      index = groups.length;
      indexBySection.set(key, index);
      groups.push({ section: key, widgets: [] });
    }
    groups[index].widgets.push(widget);
  }
  return groups;
}
