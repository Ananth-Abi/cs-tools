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

import { Box, Button, Tooltip } from "@wso2/oxygen-ui";
import { Check, Link2 } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useState, type JSX } from "react";
import { formatRelativeTime } from "@features/csm-dashboard/utils/abtDashboard";
import { formatAbsoluteForUser } from "@utils/dateTime";

/** How often mounted relative-time displays refresh themselves. Coarser than
 * a second-level ticker since the display granularity is minutes, but tight
 * enough that "1m ago" turning into "2m ago" feels prompt. */
const TICK_INTERVAL_MS = 20_000;

type TickListener = () => void;

/** Module-level subscriber set backing a single shared `setInterval`, so a
 * page with many relative timestamps mounted at once (e.g. a `CasesList`
 * table, or a long comment thread) re-renders off one timer instead of one
 * per instance. */
const tickListeners = new Set<TickListener>();
let tickIntervalId: ReturnType<typeof setInterval> | null = null;

function ensureTicking(): void {
  if (tickIntervalId !== null) return;
  tickIntervalId = setInterval(() => {
    tickListeners.forEach((listener) => listener());
  }, TICK_INTERVAL_MS);
}

function stopTickingIfIdle(): void {
  if (tickListeners.size === 0 && tickIntervalId !== null) {
    clearInterval(tickIntervalId);
    tickIntervalId = null;
  }
}

/**
 * Subscribes the calling component to the shared tick and returns the
 * timestamp (ms) as of the last tick, so it re-renders — and recomputes
 * whatever it derives from "now" — on the same cadence as every other
 * `RelativeTime` on the page. Used by {@link RelativeTime} itself, and by
 * the two "Last refreshed …" labels (`CaseSlaTable`, `RefreshButton`) that
 * format a relative string outside this component.
 *
 * Deliberately returns the timestamp itself, not an opaque counter: this
 * webapp runs the React Compiler, which auto-memoizes calls like
 * `formatRelativeTime(iso)` against the reactive values they visibly read.
 * A counter that's merely in scope wouldn't be picked up as a dependency —
 * the caller must pass this value in as the explicit `now` argument (see
 * `formatRelativeTime`'s second parameter) for the compiler to know the
 * result needs recomputing on every tick.
 */
// eslint-disable-next-line react-refresh/only-export-components -- hook is colocated with the shared ticker it manages, and reused by the two other relative-time call sites (fast-refresh DX only)
export function useRelativeTimeTick(): number {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    const listener = (): void => setNow(Date.now());
    tickListeners.add(listener);
    ensureTicking();
    return () => {
      tickListeners.delete(listener);
      stopTickingIfIdle();
    };
  }, []);

  return now;
}

interface RelativeTimeProps {
  /** Backend timestamp string (assumed UTC if no zone is present). */
  iso: string | null | undefined;
  /**
   * Optional permalink target. When provided, the time renders as an anchor
   * (Twitter/Facebook pattern: time = permalink to the entry). May be a hash
   * fragment (e.g. `#cmt-1001-2`) or a route.
   */
  href?: string;
  /** Optional className passthrough for layout tweaks. */
  className?: string;
}

/**
 * Small icon-button that copies the full absolute permalink URL for `href`
 * to the clipboard, so the permalink affordance is discoverable even for
 * someone who doesn't notice the timestamp itself is clickable. Follows the
 * same Copy/Check + 2-second-reset pattern as {@link QueryErrorState}'s
 * tracking-ID copy button, but rests on a chain-link icon to signal
 * "permalink" rather than a generic copy action.
 */
function CopyPermalinkButton({ href }: { href: string }): JSX.Element {
  const [copied, setCopied] = useState(false);

  const handleCopy = (): void => {
    const url = `${window.location.origin}${window.location.pathname}${href}`;
    if (!navigator.clipboard) return;
    navigator.clipboard.writeText(url).then(
      () => {
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
      },
      () => {
        // swallow — no toast surface available from this small button
      },
    );
  };

  const label = copied ? "Copied" : "Copy link to this entry";

  return (
    <Tooltip title={label} placement="top">
      <Button
        size="small"
        variant="text"
        color="inherit"
        onClick={handleCopy}
        sx={{ minWidth: 0, p: 0.5, color: "text.disabled" }}
        aria-label={label}
      >
        {copied ? <Check size={13} /> : <Link2 size={13} />}
      </Button>
    </Tooltip>
  );
}

/**
 * Renders a relative timestamp ("7h ago") with the full absolute datetime
 * (in the user's preferred zone) shown on hover. If `href` is provided, the
 * text becomes a permalink to that entry, and a copy-link icon-button follows
 * it so the permalink affordance doesn't rely on someone noticing the
 * timestamp itself is clickable.
 */
export default function RelativeTime({
  iso,
  href,
  className,
}: RelativeTimeProps): JSX.Element {
  // Subscribed only to trigger a re-render on each tick; the recomputed
  // relative string below always reads a fresh `Date.now()` internally.
  const now = useRelativeTimeTick();
  const relative = formatRelativeTime(iso, now);
  const absolute = formatAbsoluteForUser(iso) ?? "Unknown time";

  const inner = href ? (
    <a
      href={href}
      className={className}
      style={{
        color: "inherit",
        textDecoration: "none",
        whiteSpace: "nowrap",
        cursor: "pointer",
      }}
      onMouseEnter={(e) => {
        (e.currentTarget as HTMLAnchorElement).style.textDecoration =
          "underline";
      }}
      onMouseLeave={(e) => {
        (e.currentTarget as HTMLAnchorElement).style.textDecoration = "none";
      }}
    >
      {relative}
    </a>
  ) : (
    <span className={className} style={{ whiteSpace: "nowrap" }}>
      {relative}
    </span>
  );

  if (!href) {
    return (
      <Tooltip title={absolute} placement="top" arrow>
        {inner}
      </Tooltip>
    );
  }

  return (
    <Box component="span" sx={{ display: "inline-flex", alignItems: "center", gap: 0.25 }}>
      <Tooltip title={absolute} placement="top" arrow>
        {inner}
      </Tooltip>
      <CopyPermalinkButton href={href} />
    </Box>
  );
}
