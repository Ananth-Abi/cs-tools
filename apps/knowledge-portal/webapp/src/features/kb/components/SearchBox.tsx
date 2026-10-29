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

import { Box, TextField } from "@wso2/oxygen-ui";
import { Search } from "@wso2/oxygen-ui-icons-react";
import { useState, type FormEvent, type JSX } from "react";
import { MAX_SEARCH_QUERY_LENGTH } from "@api/backend/types";

interface SearchBoxProps {
  /** Accessible name; there is no visible label. */
  label: string;
  placeholder: string;
  initialValue?: string;
  large?: boolean;
  onSearch: (query: string) => void;
}

/** Search form. Remount with a new `key` to reset it to a new initial value. */
export default function SearchBox({
  label,
  placeholder,
  initialValue = "",
  large,
  onSearch,
}: SearchBoxProps): JSX.Element {
  const [draft, setDraft] = useState(initialValue);
  const submit = (e: FormEvent): void => {
    e.preventDefault();
    onSearch(draft);
  };
  return (
    <Box component="form" role="search" onSubmit={submit} sx={{ width: "100%" }}>
      <TextField
        fullWidth
        type="search"
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        placeholder={placeholder}
        slotProps={{
          htmlInput: { "aria-label": label, maxLength: MAX_SEARCH_QUERY_LENGTH },
          input: {
            startAdornment: (
              <Search size={large ? 20 : 18} style={{ marginRight: 10, opacity: 0.6 }} aria-hidden />
            ),
            sx: large ? { py: 0.5, fontSize: "1.05rem" } : undefined,
          },
        }}
      />
    </Box>
  );
}
