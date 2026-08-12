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

import { afterEach, describe, expect, it, vi } from "vitest";
import { formatRelativeDateOnly, isPastDateOnly, isPastDateTime } from "./dateTime";

describe("isPastDateTime", () => {
  it("is true for an instant strictly before now", () => {
    expect(isPastDateTime(new Date(Date.now() - 60_000))).toBe(true);
  });

  it("is false for an instant in the future", () => {
    expect(isPastDateTime(new Date(Date.now() + 60_000))).toBe(false);
  });

  it("is false for null (an empty/unset field isn't 'in the past')", () => {
    expect(isPastDateTime(null)).toBe(false);
  });

  it("is false for an invalid Date", () => {
    expect(isPastDateTime(new Date("not-a-date"))).toBe(false);
  });
});

describe("isPastDateOnly", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("is false for today's local-midnight date, regardless of current time", () => {
    // Freeze the clock so `today` (built here) and the "now" that
    // isPastDateOnly constructs internally can't disagree across an actual
    // local-midnight boundary crossed between this line and the call below.
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 0, 15, 12, 0, 0));
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    expect(isPastDateOnly(today)).toBe(false);
  });

  it("is true for yesterday", () => {
    const yesterday = new Date();
    yesterday.setHours(0, 0, 0, 0);
    yesterday.setDate(yesterday.getDate() - 1);
    expect(isPastDateOnly(yesterday)).toBe(true);
  });

  it("is false for tomorrow", () => {
    const tomorrow = new Date();
    tomorrow.setHours(0, 0, 0, 0);
    tomorrow.setDate(tomorrow.getDate() + 1);
    expect(isPastDateOnly(tomorrow)).toBe(false);
  });

  it("is false for null/invalid", () => {
    expect(isPastDateOnly(null)).toBe(false);
    expect(isPastDateOnly(new Date("not-a-date"))).toBe(false);
  });
});

describe("formatRelativeDateOnly", () => {
  const now = new Date(2026, 0, 15, 12, 0, 0); // Jan 15, 2026, local noon

  it("labels today's date as 'Today'", () => {
    expect(formatRelativeDateOnly("2026-01-15", now)).toBe("Today");
  });

  it("labels yesterday as 'Yesterday'", () => {
    expect(formatRelativeDateOnly("2026-01-14", now)).toBe("Yesterday");
  });

  it("labels tomorrow as 'Tomorrow'", () => {
    expect(formatRelativeDateOnly("2026-01-16", now)).toBe("Tomorrow");
  });

  it("labels 2+ days ago/from now as 'Nd ago' / 'Nd from now'", () => {
    expect(formatRelativeDateOnly("2026-01-13", now)).toBe("2d ago");
    expect(formatRelativeDateOnly("2026-01-08", now)).toBe("7d ago");
    expect(formatRelativeDateOnly("2026-01-17", now)).toBe("2d from now");
  });

  it("returns 'Today' near local midnight regardless of the value/now split", () => {
    // This is the actual bug: the old (hour-diff, UTC-parsed) approach could
    // read a date-only value as being on the wrong calendar day -- or even
    // hours "in the future" -- once the viewer's timezone offset shifted
    // local midnight away from UTC midnight. This function never touches UTC
    // at all (parseDateOnly + local Date field comparisons only), so it can't
    // regress into that: "today", one minute after local midnight, is still
    // unambiguously "Today".
    const justAfterMidnight = new Date(2026, 0, 15, 0, 1, 0);
    expect(formatRelativeDateOnly("2026-01-15", justAfterMidnight)).toBe("Today");
  });

  it("returns '—' for null/undefined/empty", () => {
    expect(formatRelativeDateOnly(null, now)).toBe("—");
    expect(formatRelativeDateOnly(undefined, now)).toBe("—");
    expect(formatRelativeDateOnly("", now)).toBe("—");
  });

  it("returns '—' for an unparseable value", () => {
    expect(formatRelativeDateOnly("not-a-date", now)).toBe("—");
  });
});
