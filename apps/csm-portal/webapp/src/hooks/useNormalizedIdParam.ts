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
import { useLocation, useParams } from "react-router";
import { useNavTransition } from "@hooks/useNavTransition";

// Backend ids are 36-char UUIDs (8-4-4-4-12, dashes at positions 8/13/18/23,
// e.g. 56f49f0a-eb1e-c310-fcf5-f5dabad0cdab). Some inbound links carry the
// same 32 hex characters with the dashes stripped — this matches that exact
// shape only, nothing shorter/longer or non-hex.
const DASHLESS_ID_PATTERN = /^[0-9a-f]{32}$/i;

function toDashedId(dashless: string): string {
  return [
    dashless.slice(0, 8),
    dashless.slice(8, 12),
    dashless.slice(12, 16),
    dashless.slice(16, 20),
    dashless.slice(20, 32),
  ].join("-");
}

/**
 * Reads a route param that is expected to hold a backend UUID id, and
 * transparently repairs a dashless variant of it (32 hex chars, no
 * separators): the dashed id is returned immediately so the caller's first
 * render/fetch already uses the corrected value, while a `replace` navigation
 * to the dashed URL runs as a side effect so the address bar catches up.
 *
 * An already-dashed id, or anything that isn't exactly 32 hex chars, is
 * returned unchanged with no navigation — malformed ids are not validated or
 * rejected here, that stays whatever it does today (404/error state).
 */
export function useNormalizedIdParam(paramName: string): string | undefined {
  const params = useParams();
  const rawValue = params[paramName];
  const navigate = useNavTransition();
  const location = useLocation();

  const isDashless = !!rawValue && DASHLESS_ID_PATTERN.test(rawValue);
  const normalizedValue = isDashless && rawValue ? toDashedId(rawValue) : rawValue;

  useEffect(() => {
    if (!isDashless || !rawValue) return;
    const dashed = toDashedId(rawValue);
    const newPathname = location.pathname.replace(rawValue, dashed);
    navigate(
      { pathname: newPathname, search: location.search, hash: location.hash },
      { replace: true, state: location.state },
    );
  }, [
    isDashless,
    rawValue,
    location.pathname,
    location.search,
    location.hash,
    location.state,
    navigate,
  ]);

  return normalizedValue;
}
