# ADR 021: MCP Apps stay on the existing Muster server

Status: accepted for the shape. Six behavior choices are still open. Not live.

## Decision

Serve one read-only MCP App from the existing authenticated `POST /mcp` endpoint. Keep the Go server, the text tools, the Markdown skills, the website, and the OAuth flow. Do not add an event stream. Do not put a bearer token in the iframe. Writes, when they come, are prepared actions that the user confirms. Each write stays off until its own tests pass.

The full package is in [docs/muster/mcp-apps](../mcp-apps/README.md). This note is the decision, not a second spec.

## Why

Claude can render an HTML tool resource. Muster already has the text tools and the sign-in. A second server would split auth and the grid rules.

## Rejected

A generic embedded page with its own token. An event stream only so the host can refresh. Icons in `initialize`, which Claude has dropped connections for. Rounding a meeting length onto the next slot.

## Open choices

These are not approved. Do not implement them as if they were.

1. Owner follow-up: keep the owner as a participant, or keep today's reminder list.
2. Omitting both availability lists: keep today's rejection, or return a successful no-op.
3. Event list default: keep every event, or default to ones that need an answer.
4. Text writes: guard new ranges, and leave old whole-list replaces unconditional.
5. Archived and finalized musters: keep them editable, or lock them.
6. Weekly shorthand: map Monday 15:00 to the stored reference week in UTC.

## Upgrade

`scripts/check-upstream-upgrade.sh` fails when a fetched upstream ref conflicts on MCP, auth, deploy, or spec paths. An upstream merge must not replace those files with the upstream copy.

## This slice

Failed GET `/mcp` attempts now count toward the same sign-in block as POST. `duration_minutes` that is not a multiple of `slot_minutes` is rejected. The App UI is not built and is not live.
