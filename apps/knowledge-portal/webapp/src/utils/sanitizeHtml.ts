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

import DOMPurify from "dompurify";

/**
 * Article bodies are authored rich text served by a public, unauthenticated
 * API and rendered with dangerouslySetInnerHTML, so the policy is stricter
 * than DOMPurify's defaults: no inline styles (CSS can overlay or exfiltrate),
 * and nothing interactive (forms, inputs, frames, embedded objects) that could
 * stage a phishing form or load third-party content inside the page.
 */
const SANITIZE_CONFIG = {
  FORBID_TAGS: [
    "style",
    "form",
    "input",
    "button",
    "textarea",
    "select",
    "iframe",
    "object",
    "embed",
  ],
  FORBID_ATTR: ["style"],
};

/** True when `href` points at a different origin than the page. */
export function isExternalHref(href: string | null, pageOrigin: string): boolean {
  if (!href) return false;
  // Protocol-relative (//host/path) and absolute http(s) URLs only. Relative
  // paths, fragments, mailto: and tel: stay in the same tab.
  if (!/^(https?:)?\/\//i.test(href.trim())) return false;
  try {
    return new URL(href.trim(), pageOrigin).origin !== pageOrigin;
  } catch {
    return false;
  }
}

// External links open in a new tab and must not hand the new page a reference
// to this one (reverse tabnabbing). DOMPurify strips `target` itself, so it is
// set here, after sanitization, once the href is known to have survived.
// Registered once at module load.
if (typeof window !== "undefined") {
  DOMPurify.addHook("afterSanitizeAttributes", (node) => {
    if (node.tagName === "A" && isExternalHref(node.getAttribute("href"), window.location.origin)) {
      node.setAttribute("target", "_blank");
      node.setAttribute("rel", "noopener noreferrer");
    }
  });
}

/** Sanitize an article body for rendering via dangerouslySetInnerHTML. */
export function sanitizeArticleHtml(html: string): string {
  return DOMPurify.sanitize(html, SANITIZE_CONFIG);
}
