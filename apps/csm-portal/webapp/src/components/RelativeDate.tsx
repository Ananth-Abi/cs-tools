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

import { Tooltip } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { formatDateOnlyForDisplay, formatRelativeDateOnly } from "@utils/dateTime";

interface RelativeDateProps {
  /** Date-only field value ("YYYY-MM-DD"), e.g. a time card's `workDate`. */
  value: string | null | undefined;
  /** Optional className passthrough for layout tweaks. */
  className?: string;
}

/**
 * Renders a date-only field ("YYYY-MM-DD", no time-of-day component) as a
 * calendar-day-relative label ("Today", "Yesterday", "3d ago"), with the
 * absolute date (e.g. "Aug 1, 2026") shown on hover.
 *
 * Sibling to {@link RelativeTime}, not a replacement for it: `RelativeTime`
 * is for a real timestamp (an instant, with a genuine time-of-day) and is
 * still correct for those. This is specifically for a field that only ever
 * carries a calendar date — using `RelativeTime` on one of those parses it as
 * UTC midnight and diffs it against the viewer's local "now" in hours/minutes,
 * which both reads oddly ("14h ago" for something logged today) and can land
 * on the wrong calendar day entirely depending on the viewer's timezone.
 */
export default function RelativeDate({
  value,
  className,
}: RelativeDateProps): JSX.Element {
  const relative = formatRelativeDateOnly(value);
  const absolute = formatDateOnlyForDisplay(value) ?? "Unknown date";

  return (
    <Tooltip title={absolute} placement="top" arrow>
      <span className={className} style={{ whiteSpace: "nowrap" }}>
        {relative}
      </span>
    </Tooltip>
  );
}
