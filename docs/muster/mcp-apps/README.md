# Muster MCP Apps: implementation handoff

Research and repository review: 2 November 2026. Approved scope: all three existing workflows, delivered read-only first and then with explicitly confirmed writes.

## Full-build amendment

This package is amended, not replaced. T0–T7, F1–F6, the original acceptance/holdout gates, read-only-first delivery, per-action flags, safety limits, and release restrictions remain in force; F7 adds conversational-tool parity at T1.5, and F6 gains text-call/catalog/response efficiency gates.

The final development tip is `cd089433aa25eea7231047774e6235da3348c44d`, reviewed after two commits arrived during amendment; `59b1a48b` remains the original research baseline. The user supplied separate production field evidence from three archived probe musters and reported 33/34 checks passing; this amendment does not claim to have repeated those live checks or identified the live image.

The late delta adds `skipped:string[]` and corrects the omission/rounding descriptions; the subsequent commit changes only favicon metadata ([MCP delta](https://github.com/shivasymbl/muster/commit/cfaffa10), [latest tip](https://github.com/shivasymbl/muster/commit/cd089433aa25eea7231047774e6235da3348c44d)). The plan therefore preserves the new skipped array and adds `skipped_details` rather than changing its type; keeping omitted-list validation errors is now a documented, satisfied option, while the recommended no-op remains an owner choice.

Start with `AMENDMENT_CHANGELOG.md` for every FE/gap disposition and `OWNER_DECISIONS.md` for the unapproved policy choices. The plan is complete enough to hand to the coding agent, but affected behavior and final readiness remain blocked until the owner confirms those choices.

Updated dependency order:

```text
T0 -> T1 -> T1.5 conversational track
           |          |
           |          +-> shared range/CAS primitives reused by T4
           +-> T2 -> T3 read-only App -> T4 -> T5 -> T6 -> T7
```

T1.5 and T2 can proceed in parallel after T1; text value does not wait for App rendering or real-host App evidence. T1.5's ranged writes must first satisfy their shared storage/CAS tests rather than shipping a weaker temporary writer.

## Decision

Build a small **MCP App**, served by Muster's existing authenticated MCP endpoint and rendered inside Claude. Keep the Go server, existing text tools, Markdown skills, website, OAuth flow, and no-event-stream transport; add one visual opener, one versioned HTML resource, an authorized view contract, and later a narrowly bounded confirmation path.

MCP Apps is the official extension for associating a tool with an HTML resource through `_meta.ui.resourceUri`; it uses the host bridge rather than giving the iframe an MCP bearer token ([official overview](https://modelcontextprotocol.io/extensions/apps/overview), [Claude quickstart](https://claude.com/docs/connectors/building/mcp-apps/quickstart)). Claude's quickstart demonstrates a stateless server with GET returning 405, so this capability does not require a notification stream ([Claude quickstart](https://claude.com/docs/connectors/building/mcp-apps/quickstart)).

This is a specification and verification package, not a built UI or deployment approval. Machine-executable gates are supplied, but the implementation-dependent gates must fail or remain blocked until the coding agent builds the feature and supplies evidence.

## Repository baseline

Use branch `room/latest-work-completed-s5jq`, final checked tip `cd089433aa25eea7231047774e6235da3348c44d`, not the repository's older default branch. The original `59b1a48b` baseline contains the eight earlier regression tests and fixes to ranking, status, panic IDs, and the list projection ([original commit](https://github.com/shivasymbl/muster/commit/59b1a48b21060946c6ee11092f6a7d27b113bef7)).

The selected Go packages and all eight regression cases passed locally. Production image, deployment state, real Mongo topology/indexes, and actual Claude rendering remain unverified.

Two small prerequisites remain:

- **GET authentication accounting:** GET respects an existing block but does not increment the failed-auth budget for GET-only failures; add `NoteAuthFail` on the same failure conditions as POST ([current handler](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/routes/mcp.go)).
- **Duration validation:** The UI MUST offer positive multiples of the grid increment; add a clear `BAD_ARGUMENTS` check to `findBestTimes`, rather than treating an invalid duration as “no suitable times” ([current ranking changes](https://github.com/shivasymbl/muster/commit/59b1a48b21060946c6ee11092f6a7d27b113bef7)).

## What users will get

| Workflow | Read-only release | Confirmed-write release |
|---|---|---|
| Pick a time | Best options, counts, conditional availability, explainable ranking, responsive grid/list, permission-safe people detail | Review exact interval and consequences, finalize, then show stored receipt |
| Answer | Own answer status, complete stored answer, timezone/reference-week labels, accessible slot display | Local drafts, available/if-needed/unavailable choices, explicit clear, review diff, guarded save |
| Run a group | Group summary, authorized participant statuses, pending-follow-up list, actual finalized interval | Create group, add/remove invitees, edit supported settings, decline own invitation, each separately confirmed |

No calendar event creation, provider-calendar integration, new email-delivery promise, automatic reminders, subscriptions, or presence system is included. Existing issue #11 and issue #26 remain open ([issue 11](https://github.com/shivasymbl/muster/issues/11), [issue 26](https://github.com/shivasymbl/muster/issues/26)).

## Host support

Claude documents MCP Apps on web and Desktop, plus native WebViews on iOS and Android; Cowork is also documented, while Claude Code is text-only and the Messages API MCP connector does not itself render an App ([Claude getting started](https://claude.com/docs/connectors/building/mcp-apps/getting-started), [design guidance](https://claude.com/docs/connectors/building/mcp-apps/design-guidelines), [interactive connectors](https://claude.com/resources/articles/interactive-tools-in-claude), [quickstart](https://claude.com/docs/connectors/building/mcp-apps/quickstart), [API connector](https://platform.claude.com/docs/en/agents-and-tools/mcp-connector)).

Treat these as documented platform capabilities, not test results for Muster. Launch against Claude web/Desktop first; verify iOS/Android separately, and retain a complete text path throughout.

## Package order

1. `RESEARCH.md`: verified specifications, host constraints, code findings, and limits of evidence.
2. `REQUIREMENTS.md`: six bounded features with acceptance criteria and failure behavior.
3. `UX_AND_CONTRACTS.md`: interaction design, wire envelopes, view schema, and action states.
4. `IMPLEMENTATION_PLAN.md`: exact file/function changes, sequence, verification, and proposed commit boundaries.
5. `DECISIONS.md`: architecture decisions and rejected alternatives.
6. `SCENARIOS.md`: end-to-end reviewer scenarios; withhold this file and `evals/holdout` from an implementer if independent holdout evaluation is desired.
7. `SYSTEM_MAP.yaml`: components, boundaries, and impacted consumers.
8. `evals/`: executable convergence gates and host-evidence template.
9. `CLAUDE_CODE_PROMPT.md`: copy-ready instruction for the building agent.
10. `AMENDMENT_CHANGELOG.md` and `OWNER_DECISIONS.md`: complete field/gap traceability and explicit pending decisions.

## Delivery gates

- **Read-only:** Protocol, privacy, current-state fidelity, text fallback, accessible rendering, build packaging, and real Claude web/Desktop evidence pass.
- **Answer and finalization:** Target-specific conditional writes, durable action tracking, conflict handling, and unknown-outcome behavior pass before enabling each action.
- **Group mutations:** Multi-record partial-failure and duplicate-prevention tests pass separately; do not enable creation/invitation changes just because simpler writes passed.
- **Mobile:** Actual device evidence is recorded before describing the Muster UI as mobile-tested.
- **Conversational track:** F7/field findings, text-call efficiency, catalog growth, all six owner decisions, and the required conditional-store evidence pass independently of rendering; these are additional full-build gates.

No automated gate deploys, publishes, closes an issue, or approves a destructive migration. Those remain explicit human decisions.
