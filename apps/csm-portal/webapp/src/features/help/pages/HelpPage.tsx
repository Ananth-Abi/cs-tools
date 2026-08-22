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

import {
  Box,
  Divider,
  IconButton,
  List,
  ListItemButton,
  ListItemText,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowUp } from "@wso2/oxygen-ui-icons-react";
import { type JSX, useEffect, useState } from "react";
import { navNodeById } from "@config/csmNavItems";
import { enabledNavChildren } from "@config/featureFlags";
import HelpTopicSection from "@features/help/components/HelpTopicSection";

/** Bare topic id (e.g. `"operations"`) from a `help.<id>` nav node id — the
 * key `HELP_TOPIC_CONTENT` and every anchor target on this page share. */
function bareTopicId(nodeId: string): string {
  return nodeId.replace(/^help\./, "");
}

/** Scroll-Y, in px, past which the "back to top" button becomes visible. */
const BACK_TO_TOP_THRESHOLD = 240;

/**
 * Floating "back to top" control. Hidden near the top of the page, fades in
 * once the user has scrolled past the table of contents, and smooth-scrolls
 * back to the top on click — a plain scroll listener + `window.scrollTo` is
 * enough for this, no scroll library needed.
 */
function BackToTopButton(): JSX.Element {
  const [visible, setVisible] = useState(
    () => window.scrollY > BACK_TO_TOP_THRESHOLD,
  );

  useEffect(() => {
    const onScroll = (): void => {
      setVisible(window.scrollY > BACK_TO_TOP_THRESHOLD);
    };
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  if (!visible) return <></>;

  return (
    <Tooltip title="Back to top">
      <IconButton
        aria-label="Back to top"
        onClick={() => window.scrollTo({ top: 0, behavior: "smooth" })}
        sx={{
          position: "fixed",
          bottom: 24,
          right: 24,
          zIndex: (theme) => theme.zIndex.tooltip,
          bgcolor: "background.paper",
          border: 1,
          borderColor: "divider",
          boxShadow: 3,
          "&:hover": { bgcolor: "action.hover" },
        }}
      >
        <ArrowUp size={20} />
      </IconButton>
    </Tooltip>
  );
}

/**
 * The whole Help section: one scrollable page with a table of contents up
 * top, every topic rendered below it in nav order, and a floating
 * "back to top" button. Replaces the earlier sidebar-plus-per-topic-route
 * layout (`HelpLayout` + `HelpTopicPage`, one route per topic) — a long-form
 * doc reads better as one page you can skim/search/print than as a set of
 * separate routes, and a floating in-page TOC gets the same "jump to a
 * topic" behaviour natively, via anchor links, with no router involved.
 *
 * Topics are still declared once, in `csmNavItems.ts`'s `help` node, and
 * filtered here to the ones this deployment has enabled via
 * `CSM_PORTAL_FEATURE_OVERRIDES` — the same mechanism every other section's
 * tab strip uses (`enabledNavChildren`), just applied to a plain rendered
 * list instead of a route guard, since there is no longer a per-topic route
 * to guard.
 */
/**
 * Scrolls the section matching the current `#<topic>` hash into view, both on
 * first mount (a direct link like `/help#operations`) and on any later hash
 * change (the TOC's own anchor links, which the browser can otherwise leave
 * unhandled since every topic's section is a plain DOM node rendered by this
 * same component, not a route the router would scroll for on navigation).
 */
function useScrollToHashOnMount(): void {
  useEffect(() => {
    const scrollToHash = (): void => {
      const id = window.location.hash.slice(1);
      if (!id) return;
      document.getElementById(id)?.scrollIntoView();
    };
    scrollToHash();
    window.addEventListener("hashchange", scrollToHash);
    return () => window.removeEventListener("hashchange", scrollToHash);
  }, []);
}

export default function HelpPage(): JSX.Element {
  const helpSection = navNodeById("help");
  const topics = helpSection ? enabledNavChildren(helpSection) : [];
  useScrollToHashOnMount();

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Typography variant="h5">Help</Typography>

      <Box
        component="nav"
        aria-label="Help topics"
        sx={{
          border: 1,
          borderColor: "divider",
          borderRadius: 1,
          overflow: "hidden",
          maxWidth: 420,
        }}
      >
        <List dense disablePadding>
          {topics.map((topic) => {
            const id = bareTopicId(topic.id);
            return (
              <ListItemButton key={topic.id} component="a" href={`#${id}`}>
                <ListItemText primary={topic.label} />
              </ListItemButton>
            );
          })}
        </List>
      </Box>

      {topics.map((topic, index) => {
        const id = bareTopicId(topic.id);
        return (
          <Box key={topic.id}>
            {index > 0 && <Divider sx={{ mb: 3 }} />}
            <Box component="section" id={id} sx={{ minWidth: 0 }}>
              <HelpTopicSection topicId={id} />
            </Box>
          </Box>
        );
      })}

      <BackToTopButton />
    </Box>
  );
}
