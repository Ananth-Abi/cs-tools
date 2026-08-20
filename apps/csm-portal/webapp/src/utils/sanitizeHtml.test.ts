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
import { describe, expect, it } from "vitest";
import {
  escapeHtml,
  isBlankHtml,
  plainTextToHtml,
  sanitizeDescriptionHtml,
  sanitizeRichTextHtml,
  stripHtmlTags,
  stripHtmlTagsPreservingLineBreaks,
  stripLightModeInlineStyles,
} from "./sanitizeHtml";

describe("sanitizeRichTextHtml", () => {
  it("forces rel=noopener noreferrer on target=_blank anchors when target is preserved", () => {
    // DOMPurify's default ALLOWED_ATTR doesn't include `target`, so a bare
    // `sanitizeRichTextHtml()` call already strips it (and with it, the need
    // for a hardening `rel`) — this exercises the module-load hook itself via
    // a config that keeps `target`, which is what any caller opting into
    // `target="_blank"` output (e.g. a future ADD_ATTR override) would rely on.
    const out = DOMPurify.sanitize(
      '<a href="https://example.com" target="_blank">link</a>',
      { ADD_ATTR: ["target"] },
    );
    expect(out).toContain('rel="noopener noreferrer"');
  });

  it("strips the target attribute by default (so no dangling target=_blank ships unhardened)", () => {
    const out = sanitizeRichTextHtml(
      '<a href="https://example.com" target="_blank">link</a>',
    );
    expect(out).not.toContain("target=");
  });

  it("strips script tags", () => {
    const out = sanitizeRichTextHtml("<p>hi</p><script>alert(1)</script>");
    expect(out).not.toContain("<script>");
  });

  it("keeps tables and code blocks (permissive comment policy)", () => {
    const out = sanitizeRichTextHtml("<table><tr><td>x</td></tr></table><code>y</code>");
    expect(out).toContain("<table>");
    expect(out).toContain("<code>");
  });
});

describe("sanitizeDescriptionHtml", () => {
  it("strips tables and code blocks", () => {
    const out = sanitizeDescriptionHtml(
      "<p>desc</p><table><tr><td>x</td></tr></table><pre><code>y</code></pre>",
    );
    expect(out).toContain("<p>desc</p>");
    expect(out).not.toContain("<table>");
    expect(out).not.toContain("<pre>");
    expect(out).not.toContain("<code>");
  });
});

describe("isBlankHtml", () => {
  it("is true for an empty paragraph", () => {
    expect(isBlankHtml("<p></p>")).toBe(true);
  });

  it("is false for a description with text", () => {
    expect(isBlankHtml("<p>Some description</p>")).toBe(false);
  });
});

describe("stripLightModeInlineStyles", () => {
  it("removes a pure-white inline background declaration", () => {
    const out = stripLightModeInlineStyles(
      '<div style="background-color: #ffffff; color: red;">x</div>',
    );
    expect(out).not.toContain("background-color");
    expect(out).toContain("color: red");
  });

  it("removes a near-white rgb background", () => {
    const out = stripLightModeInlineStyles(
      '<div style="background: rgb(250, 250, 250);">x</div>',
    );
    expect(out).not.toContain("rgb(250");
  });

  it("removes a dark text color declaration", () => {
    const out = stripLightModeInlineStyles('<span style="color: #000000;">x</span>');
    expect(out).not.toContain("color");
  });

  it("leaves other declarations untouched", () => {
    const out = stripLightModeInlineStyles(
      '<div style="border: 1px solid #333; padding: 4px;">x</div>',
    );
    expect(out).toContain("border: 1px solid #333");
    expect(out).toContain("padding: 4px");
  });

  it("handles single-quoted style attributes the same as double-quoted", () => {
    const out = stripLightModeInlineStyles(
      "<div style='background: #ffffff; color: red;'>x</div>",
    );
    expect(out).not.toContain("background");
    expect(out).toContain("color: red");
  });

  it("does not treat a bright #1xxxxx/#2xxxxx color as dark", () => {
    const blue = stripLightModeInlineStyles('<span style="color: #1f75fe;">x</span>');
    expect(blue).toContain("color: #1f75fe");
    const cyan = stripLightModeInlineStyles('<span style="color: #2fffff;">x</span>');
    expect(cyan).toContain("color: #2fffff");
  });

  it("removes a light pastel background (e.g. a ServiceNow call-note highlight)", () => {
    const out = stripLightModeInlineStyles(
      '<div style="background-color: #bce4e8; padding: 0.01em 16px;">x</div>',
    );
    expect(out).not.toContain("background-color");
    expect(out).toContain("padding: 0.01em 16px");
  });

  it("removes a mid-gray background whose contrast against light text is below WCAG AA", () => {
    // #808080 (luminance ~0.216) contrasts with white text at ~3.95:1, below
    // the 4.5:1 AA minimum for normal text — a fixed 0.55 luminance cutoff
    // missed this; the contrast-derived threshold must catch it.
    const out = stripLightModeInlineStyles(
      '<div style="background-color: #808080;">x</div>',
    );
    expect(out).not.toContain("background-color");
  });

  it("still removes near-white backgrounds (no regression)", () => {
    const hex = stripLightModeInlineStyles('<div style="background-color: #f4f4f4;">x</div>');
    expect(hex).not.toContain("background-color");
    const rgb = stripLightModeInlineStyles('<div style="background: rgb(250, 250, 250);">x</div>');
    expect(rgb).not.toContain("rgb(250");
  });

  it("leaves a dark/saturated background alone", () => {
    const out = stripLightModeInlineStyles(
      '<div style="background-color: #1a1a1a; color: red;">x</div>',
    );
    expect(out).toContain("background-color: #1a1a1a");
    expect(out).toContain("color: red");
  });
});

describe("stripHtmlTagsPreservingLineBreaks", () => {
  it("keeps the boundary between two paragraphs (regression: used to collapse to 'AB')", () => {
    expect(stripHtmlTags("<p>A</p><p>B</p>")).toBe("AB");
    expect(stripHtmlTagsPreservingLineBreaks("<p>A</p><p>B</p>")).toBe("A\n\nB");
  });

  it("turns <br> into a single newline", () => {
    expect(stripHtmlTagsPreservingLineBreaks("first<br />second")).toBe(
      "first\nsecond",
    );
  });

  it("keeps list items on separate lines", () => {
    expect(
      stripHtmlTagsPreservingLineBreaks("<ul><li>one</li><li>two</li></ul>"),
    ).toBe("one\n\ntwo");
  });

  it("does not double up when the source HTML is itself newline-formatted", () => {
    expect(
      stripHtmlTagsPreservingLineBreaks("<p>A</p>\n  <p>B</p>\n"),
    ).toBe("A\n\nB");
  });

  it("still strips markup and decodes entities like the single-line variant", () => {
    expect(
      stripHtmlTagsPreservingLineBreaks("<p>a &amp; b<script>x()</script></p>"),
    ).toBe("a & b");
  });

  it("leaves plain text with comparison operators intact", () => {
    expect(stripHtmlTagsPreservingLineBreaks("x < y > z")).toBe("x < y > z");
  });

  it("returns an empty string for markup with no visible text", () => {
    expect(stripHtmlTagsPreservingLineBreaks("<p></p>")).toBe("");
  });
});

describe("escapeHtml", () => {
  it("escapes the HTML-significant characters", () => {
    expect(escapeHtml(`&<>"'`)).toBe("&amp;&lt;&gt;&quot;&#039;");
  });

  it("escapes the ampersand first, so an escaped entity is not double-escaped wrongly", () => {
    expect(escapeHtml("a & <b>")).toBe("a &amp; &lt;b&gt;");
  });
});

describe("plainTextToHtml", () => {
  it("wraps a single line in a paragraph", () => {
    expect(plainTextToHtml("Superseded by a later change.")).toBe(
      "<p>Superseded by a later change.</p>",
    );
  });

  it("keeps line breaks as <br />", () => {
    expect(plainTextToHtml("first\nsecond")).toBe("<p>first<br />second</p>");
  });

  it("escapes markup-significant characters so no text is dropped on render", () => {
    expect(plainTextToHtml("latency < 5ms & rising")).toBe(
      "<p>latency &lt; 5ms &amp; rising</p>",
    );
  });

  it("normalizes CRLF so a pasted reason does not gain blank lines", () => {
    expect(plainTextToHtml("first\r\nsecond")).toBe("<p>first<br />second</p>");
  });

  it("returns an empty string for blank input", () => {
    expect(plainTextToHtml("   \n  ")).toBe("");
  });

  it("round-trips a reason containing <, & and a newline back to the typed text", () => {
    const typed = "Rolled back: latency < 5ms & error rate spiked.\nOwner: Jane Doe";
    const posted = plainTextToHtml(typed);
    expect(stripHtmlTagsPreservingLineBreaks(posted)).toBe(typed);
  });

  it("survives the render-side sanitizer with the text and the break intact", () => {
    const typed = "a < b & c\nsecond line";
    const rendered = sanitizeRichTextHtml(plainTextToHtml(typed));
    expect(stripHtmlTagsPreservingLineBreaks(rendered)).toBe(typed);
  });
});
