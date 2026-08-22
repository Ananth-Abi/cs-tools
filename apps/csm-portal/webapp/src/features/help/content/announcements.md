# Announcements

The Announcements section lists customer-facing announcements published across
projects and tiers. Under the hood, an announcement is a case of type
`announcement`, so it shows up here read-only, using the same search and
detail infrastructure as regular cases, just trimmed down to what makes
sense for a broadcast.

## Viewing the list

The list shows one row per announcement: number, subject, project, state,
who created it, and when it was last updated. Rows are real links, so you can
middle-click or cmd-click to open an announcement in a new tab, or copy its
URL.

You can narrow the list with:

- **Search**: matches subject or number.
- **State**: filter to one or more states (open, work in progress, solution
  proposed, awaiting info, waiting on WSO2, closed).
- **Project**: filter to one or more projects.

All filters default to "show all"; changing any filter resets the list back
to the first page. Use **Clear filters** to reset state and project in one
click.

## Opening an announcement

Selecting a row opens the announcement's detail page. It reuses the same
case detail view as regular cases, but with the parts that don't apply to a
broadcast hidden:

- No action bar: announcements aren't assigned, acknowledged, or
  transitioned by an engineer from this page.
- No reply or work-note composer: announcements are read-only; there's
  nothing to reply to.
- No Related, Watchers, SLA, Time, or Call Requests tabs, since none of
  those concepts apply to an announcement.

The announcement's body is shown as its Description, rendered as rich text
(the same HTML editor content used elsewhere in the portal), so formatting,
links, and lists in the original announcement are preserved.

## What you can't do yet

Creating, targeting, or unpublishing announcements isn't available from the
CSM portal yet: it depends on a dedicated announcements backend that hasn't
been built. Until then, this section is view-only.
