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

import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import RelativeDate from "@components/RelativeDate";
import { formatDateOnly } from "@utils/dateTime";

describe("RelativeDate", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders 'Today' for today's date, not an hour-based value", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 0, 15, 23, 0, 0)); // 11 PM local

    render(<RelativeDate value={formatDateOnly(new Date(2026, 0, 15))} />);

    expect(screen.getByText("Today")).toBeInTheDocument();
    expect(screen.queryByText(/\d+h ago/)).not.toBeInTheDocument();
  });

  it("renders 'Yesterday' for the previous calendar day", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 0, 15, 9, 0, 0));

    render(<RelativeDate value={formatDateOnly(new Date(2026, 0, 14))} />);

    expect(screen.getByText("Yesterday")).toBeInTheDocument();
  });

  it("renders 'Nd ago' for older dates", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 0, 15, 9, 0, 0));

    render(<RelativeDate value={formatDateOnly(new Date(2026, 0, 12))} />);

    expect(screen.getByText("3d ago")).toBeInTheDocument();
  });

  it("renders the em dash for a missing value", () => {
    render(<RelativeDate value={null} />);
    expect(screen.getByText("—")).toBeInTheDocument();
  });
});
