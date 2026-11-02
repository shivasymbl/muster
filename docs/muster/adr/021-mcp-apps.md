# ADR 021: MCP Apps stay on the existing Muster server

Status: accepted for the shape. Five behavior choices are decided. Archived musters stay editable. The App UI is not live.

## Decision

Serve one read-only MCP App from the existing authenticated `POST /mcp` endpoint. Keep the Go server, the text tools, the Markdown skills, the website, and the OAuth flow. Do not add an event stream. Do not put a bearer token in the iframe. Writes, when they come, are prepared actions that the user confirms. Each write stays off until its own tests pass.

The full package is in [docs/muster/mcp-apps](../mcp-apps/README.md). This note is the decision, not a second spec.

## Why

Claude can render an HTML tool resource. Muster already has the text tools and the sign-in. A second server would split auth and the grid rules.

## Rejected

A generic embedded page with its own token. An event stream only so the host can refresh. Icons in `initialize`, which Claude has dropped connections for. Rounding a meeting length onto the next slot.

## Decided 2 Nov 2026

1. Reminders stay as they are today, including the owner when the owner still needs an answer.
2. Omitting both availability lists stays a rejection. The stored answer is unchanged.
3. muster_list_my_events defaults to needs_answer. status all returns every muster. The filter runs before the page is cut.
4. A whole-list replace stays unconditional. availability_ranges expands onto the grid and uses that same replace. if_match refuses the write when the stored answer differs. Omit if_match to replace anyway. This check is not a database transaction.
5. Not decided. Archived and finalized musters stay editable.
6. {weekday: Monday, time: 15:00} means 2018-06-18T15:00:00Z. New weekly dates use Monday 18 June through Sunday 24 June 2018. Answering uses the one stored date with that weekday and puts the clock on that day. Two stored dates with the same weekday are rejected. There is no timezone.

## Upgrade

`scripts/check-upstream-upgrade.sh` fails when a fetched upstream ref conflicts on MCP, auth, deploy, or spec paths. An upstream merge must not replace those files with the upstream copy.

## This slice

Failed GET `/mcp` attempts now count toward the same sign-in block as POST. `duration_minutes` that is not a multiple of `slot_minutes` is rejected. The App UI is not built and is not live.
