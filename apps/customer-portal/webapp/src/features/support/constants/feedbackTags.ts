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

/**
 * Predefined reason tags offered after rating a Novera answer.
 *
 * Deliberately duplicated rather than fetched: the vocabulary changes rarely,
 * and the backend is the authority — it validates against its own allow-list in
 * `claude_chatbot/feedback_tags.py` and silently drops anything it does not
 * recognise. So a client that drifts loses its tags, it cannot corrupt the data.
 * Keep the slugs here in step with that file; the labels are ours alone.
 */

export interface FeedbackTagOption {
  /** Wire value. Must match the backend allow-list exactly. */
  slug: string;
  /** What the customer reads. */
  label: string;
}

export const NEGATIVE_FEEDBACK_TAGS: readonly FeedbackTagOption[] = [
  { slug: "inaccurate", label: "Inaccurate" },
  { slug: "incomplete", label: "Incomplete" },
  { slug: "irrelevant", label: "Irrelevant" },
  { slug: "outdated", label: "Outdated" },
  { slug: "too_slow", label: "Too slow" },
];

export const POSITIVE_FEEDBACK_TAGS: readonly FeedbackTagOption[] = [
  { slug: "accurate", label: "Accurate" },
  { slug: "complete", label: "Complete" },
  { slug: "clear", label: "Clear" },
  { slug: "saved_time", label: "Saved time" },
];

/** Mirrors MAX_TAGS in claude_chatbot/feedback_tags.py — extra tags are dropped. */
export const MAX_FEEDBACK_TAGS = 4;

/** Prompt shown above the chips, which differs by rating. */
export function feedbackTagPrompt(rating: 1 | -1): string {
  return rating === 1 ? "What was good about it?" : "What went wrong?";
}

export function feedbackTagsFor(rating: 1 | -1): readonly FeedbackTagOption[] {
  return rating === 1 ? POSITIVE_FEEDBACK_TAGS : NEGATIVE_FEEDBACK_TAGS;
}
