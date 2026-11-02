# Grounded research and repository findings

## Standards and host behavior

The stable MCP Apps specification remains version `2026-01-26`; the core MCP protocol version is a separate negotiation, not the same version string ([stable Apps specification](https://github.com/modelcontextprotocol/ext-apps/blob/main/specification/2026-01-26/apps.mdx)). Implement this stable extension rather than an older proprietary `text/html+skybridge` interface or a generic iframe/plugin protocol ([official overview](https://modelcontextprotocol.io/extensions/apps/overview)).

The resource MIME is `text/html;profile=mcp-app`, the tool references it through `_meta.ui.resourceUri`, and the host fetches HTML with `resources/read`; the browser component uses the Apps SDK bridge for tool calls and host context ([Claude quickstart](https://claude.com/docs/connectors/building/mcp-apps/quickstart)). It is not enough to return arbitrary HTML in a normal text result.

Claude's current quickstart explicitly identifies `@modelcontextprotocol/ext-apps@1.7.5` as tested; use that exact browser SDK version for the first compatibility baseline, with a lockfile, rather than mixing the latest SDK API with old examples ([Claude quickstart](https://claude.com/docs/connectors/building/mcp-apps/quickstart)). A later upgrade needs its own protocol and host regression run, not a floating semver range.

### Protocol details that matter here

- **Capability advertisement:** Supporting hosts advertise `capabilities.extensions["io.modelcontextprotocol/ui"]` with supported MIME types; do not invent `capabilities.ui: true` or confuse this with tool-list notifications ([stable specification](https://github.com/modelcontextprotocol/ext-apps/blob/main/specification/2026-01-26/apps.mdx)).
- **Metadata location:** Tool UI metadata belongs under the tool's `_meta.ui`; resource CSP/domain preferences belong under each returned resource content item's `_meta.ui` ([stable specification](https://github.com/modelcontextprotocol/ext-apps/blob/main/specification/2026-01-26/apps.mdx)).
- **Visibility:** `visibility: ["app"]` hides a helper from model tool discovery in supporting hosts, but is not authorization enforcement; the server must authorize every call regardless of where it originated ([stable specification](https://github.com/modelcontextprotocol/ext-apps/blob/main/specification/2026-01-26/apps.mdx)).
- **No SSE requirement:** Apps bridge messaging is not the server's MCP GET notification stream; normal JSON POST responses and authenticated GET405 remain compatible with this architecture ([Claude quickstart](https://claude.com/docs/connectors/building/mcp-apps/quickstart), [MCP transport](https://modelcontextprotocol.io/specification/2025-03-26/basic/transports)).
- **Stateless discovery:** Do not retain the last client's initialization capabilities in process-global variables. Serve a stable, feature-flagged catalog with useful fallback text; unsupported clients ignore the UI metadata rather than changing another user's catalog.
- **Core compatibility:** Structured outputs were added in core `2025-06-18`; Muster currently hardcodes `2025-03-26` while emitting `structuredContent`, so compatibility needs a bounded fix rather than an unrelated upgrade to the newest core protocol ([core changelog](https://modelcontextprotocol.io/specification/2025-06-18/changelog), [current route](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/routes/mcp.go)).

Recommended core support is `2025-03-26`, `2025-06-18`, and `2025-11-25`: echo a supported requested initialization version, otherwise offer `2025-11-25`; invalid or unsupported subsequent `MCP-Protocol-Version` headers return HTTP400, and absent headers default to `2025-03-26` when no other version context exists ([lifecycle](https://modelcontextprotocol.io/specification/2025-11-25/basic/lifecycle), [transport](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports)). Preserve existing tools' historical `structuredContent` field on the old core as an explicitly documented compatibility extension; new App tools omit that field on old-core responses and retain useful text plus authorized metadata, while supported newer core responses include it. Do not globally strip or reformat existing tool data.

### A documentation discrepancy worth resolving conservatively

The Apps specification describes structured UI data as outside model context, but Claude's quickstart specifically says Claude Code can receive `structuredContent` serialized to the model instead of `content`. Therefore treat `structuredContent` as model-visible, and put only the authorized, model-safe summary there ([stable specification](https://github.com/modelcontextprotocol/ext-apps/blob/main/specification/2026-01-26/apps.mdx), [Claude quickstart](https://claude.com/docs/connectors/building/mcp-apps/quickstart)).

Use `_meta.muster` for the richer authorized UI snapshot, not for secrets or data the current user cannot see. Host omission from model context is not a data-access boundary.

## Claude support and constraints

| Surface | Verified platform documentation | Release treatment |
|---|---|---|
| Web | Interactive connector UI supported ([getting started](https://claude.com/docs/connectors/getting-started)) | Primary real-host gate |
| Desktop | Remote and local-development Apps documented ([Apps getting started](https://claude.com/docs/connectors/building/mcp-apps/getting-started)) | Test remote Muster separately |
| iOS / Android | Native WKWebView / WebView, safe-area and gesture differences ([design guide](https://claude.com/docs/connectors/building/mcp-apps/design-guidelines)) | Actual device tests, not only resized Chrome |
| Cowork | Interactive connectors documented ([announcement](https://claude.com/resources/articles/interactive-tools-in-claude)) | Secondary supported-host verification |
| Claude Code | No App UI; text/structured fallback ([quickstart](https://claude.com/docs/connectors/building/mcp-apps/quickstart)) | Required non-UI regression |
| Messages API | MCP tool calls, not a built-in App renderer ([API guide](https://platform.claude.com/docs/en/agents-and-tools/mcp-connector)) | Do not advertise inline UI here |

Custom connectors and directory connectors share the runtime; directory review is not required to test a private/custom MCP App, and an install link does not bypass user approval ([distribution guide](https://claude.com/docs/connectors/building/directory-vs-custom.md)). Document adding the custom connector on web/Desktop first because mobile onboarding documentation differs between the design guide and current help ([design guide](https://claude.com/docs/connectors/building/mcp-apps/design-guidelines), [connector help](https://support.claude.com/en/articles/11176164-use-connectors-to-extend-claude-s-capabilities)).

Claude documents approximately 150,000 characters per tool result and a 240-second tool timeout for web/Desktop; with the code-execution sandbox active, an oversized result may become a file pointer and fail to hydrate the App ([building guide](https://claude.com/docs/connectors/building), [troubleshooting](https://claude.com/docs/connectors/building/mcp-apps/troubleshooting)). Those are not a performance target, not independently verified mobile limits, and not an HTML-resource size limit.

Inline views should be compact, fit their contents, avoid nested vertical scrolling, and use at most two bottom actions; detailed interaction belongs in an expanded view where supported ([design guide](https://claude.com/docs/connectors/building/mcp-apps/design-guidelines)). Support 320px containers, host themes, large text, 44×44 touch targets, keyboard access, and safe-area insets ([design guide](https://claude.com/docs/connectors/building/mcp-apps/design-guidelines)).

No verified numeric cap for HTML resources, inline height, or JavaScript bundle size was found. The implementation budgets in this package are proposed engineering gates, not Claude guarantees.

### Consent and lifecycle

Rendering consent, host tool permissions, server authorization, and Muster's action preview are different controls; none substitutes for the others ([Apps getting started](https://claude.com/docs/connectors/building/mcp-apps/getting-started), [connector permissions](https://support.claude.com/en/articles/11176164-use-connectors-to-extend-claude-s-capabilities)).

Several old App instances may remain live, and a restored older conversation can mount after a newer widget; host mount order is not a trustworthy action version ([instance supersession](https://claude.com/docs/connectors/building/mcp-apps/instance-supersession.md)). Use server state checks and explicit refresh, not a global BroadcastChannel or client timestamp, to decide whether an action is still valid.

## Current Muster implementation

The reviewed branch has a raw Go JSON-RPC route, hand-written tool definitions, three Markdown resources, bearer authentication, and no retained client session. Successful results already contain text and `structuredContent`; this is not a missing-output-format project ([current route](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/routes/mcp.go)).

The website is Vue 2.7/Vuetify/Vuex, initializes analytics, uses cookie-authenticated HTTP, and has a separate extension bridge; importing the whole SPA would import the wrong authentication, dependencies, and messaging assumptions ([repository guidance](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/CLAUDE.md), [frontend package](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/frontend/package.json), [frontend entry](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/frontend/src/main.js)).

Recommendation: build a standalone TypeScript browser entry under `mcp-app/`, emit self-contained HTML into `frontend/dist/mcp-app/`, and serve it through the already-resolved frontend-dist path. Keep the website build intact, compile the app afterward without erasing the SPA output, and use a separate modern Node build stage if the pinned tooling requires it.

### Data gaps to solve before enabling controls

- **Whole-answer editing:** `get_responses` is limited to a seven-day window and a 4,096-byte payload; truncation can blank lists. It cannot safely initialize a whole-answer replacement editor ([response code](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/server.go)).
- **Request size:** POST is capped at 16,384 bytes, below a maximum-size full-list request. Use a bounded change-set with server-side preservation, not repeated replacement calls ([route limit](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/routes/mcp.go)).
- **Rank semantics:** `missing` includes if-needed people, not only unavailable people; do not sum these overlapping counts or label it “cannot attend” ([ranker](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/rank.go)).
- **Final state:** Existing event output lacks the stored finalized interval, so add a sanitized `scheduled` field to the new view DTO without changing legacy output ([existing DTO](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/dto.go)).
- **Privacy:** Build an allowlisted App DTO. Do not blindly copy existing nested response objects or assume emails that are absent from the participant table are absent everywhere ([privacy code](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/events/privacy.go)).
- **Mutations:** Existing writers lack target-state preconditions, durable operation identities, and atomic multi-record completion. A review button alone does not change those semantics ([response writes](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/db/mcp_writes.go), [organizer writes](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/db/mcp_organizer.go)).

## Evidence limits

Repository tests were run locally; source review does not prove deployment. Mongo uniqueness and transaction support, Claude permissions on the target account, actual App latency, and all real-host outcomes need implementation-stage evidence.

The Apps standards, SDK documentation, and Claude-specific documentation occasionally differ. The decisions here prefer the stable extension, the explicitly Claude-tested SDK, least disclosure, bounded compatibility, and host testing rather than treating one example as a universal guarantee.

## Field-evidence amendment: 2 November 2026

The user reports live production testing of three probe musters, subsequently archived: “MCP probe dates” (`jqNZbb`), “MCP probe group” (`9qfraj`), and “MCP probe weekly” (`xCuSFF`). All 16 tools were present and 33 of 34 checks passed; this is supplied field evidence, not a raw wire capture or a live test performed during this amendment.

The initial amendment audit used `59b1a48b21060946c6ee11092f6a7d27b113bef7`; the final tip advanced to `cd089433aa25eea7231047774e6235da3348c44d`. The intervening MCP change adds skipped strings and corrects descriptions, followed by favicon metadata only; local tests were rerun against the final tip ([MCP delta](https://github.com/shivasymbl/muster/commit/cfaffa10), [final tip](https://github.com/shivasymbl/muster/commit/cd089433aa25eea7231047774e6235da3348c44d)). None of these checks identifies the live image or supersedes the supplied client observations.

### Candidate disposition against the tip

| Item | Current code disposition | Amendment |
|---|---|---|
| FE-1 | Counts and replaced/duplicate receipt already work; no stored-answer echo ([writes.go](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/writes.go)) | Adopt TXT-001, preserving integer `availability`/`if_needed` fields and adding nested stored answer |
| FE-2 | Update returns only event ID and updated:true ([organizer.go](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/organizer.go)) | Adopt TXT-002, authoritative event echo |
| FE-3 | Inputs are RFC3339 slot strings; ranges are not accepted ([writes.go](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/writes.go)) | Adopt TXT-003, shared range engine and explicit replace/merge |
| FE-4 | Responses/overlap require windows; ranked-times alone supplies defaults ([read service](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/server.go), [organizer](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/organizer.go)) | Adopt TXT-004, shared defaults with actual window echoed |
| FE-5 | Structured flat errors already exist; text content is only Message ([serialization](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/routes/mcp.go)) | Satisfied in part; TXT-005 adds text parity, canonical data and repair detail, not a false claim that the wire lacks codes |
| FE-6 | All 16 have schemas/descriptions/annotations; original parameter descriptions/formats/examples are thin ([catalog](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/routes/mcp.go)) | Adopt TXT-006 documentation and truthful effect tests; preserve existing schemas' compatibility |
| FE-7 | Scheduled interval is persisted but absent from EventRow/list projection ([DTO](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/dto.go), [projection](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/db/mcp_events.go)) | Adopt TXT-007 additive readback and finalization echo |
| FE-8 | Final tip now returns `skipped:string[]`; per-input reasons/indexes remain absent ([late MCP delta](https://github.com/shivasymbl/muster/commit/cfaffa10)) | Partly satisfied; preserve strings and add TXT-008 skipped_details, retaining invalid-batch atomic rejection |
| FE-9 | Final descriptions/instructions now explicitly say rejection with no change; runtime remains BAD_ARGUMENTS ([late MCP delta](https://github.com/shivasymbl/muster/commit/cfaffa10)) | Agreement satisfied for the error option; recommended no-op remains a D2 product choice, not an unfixed documentation defect |
| FE-10 | 20 minutes on a 30-minute grid is still promoted to 30; locally reproduced ([ranker](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/rank.go)) | Adopt TXT-010 rejection; this fulfills existing DATA-007, not a new rounding preference |
| FE-11 | Group owner is stored as attendee and can appear in pending ([group store](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/db/mcp_organizer.go), [remind](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/organizer.go)) | Current semantics confirmed, desired semantics pending D1/TXT-015 |
| FE-12 | Provider names appear in add/create descriptions and group skill ([catalog](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/routes/mcp.go), [skills](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/routes/mcp_skills.go)) | Adopt TXT-016 vendor-neutral behavior language |
| FE-13 | Event listing already supports name/limit/cursor, but not status, needs-answer or compact projection ([list service](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/server.go), [list store](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/db/mcp_events.go)) | Adopt TXT-011 filter-before-pagination and explicit projection; do not confuse this with tools/list |

### Retained working evidence

| Eval | Behavior to retain | Tip evidence |
|---|---|---|
| W1 | Short accurate initialize instructions and all three skills | [Instructions/resources](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/routes/mcp_skills.go); amend only changed semantics |
| W2 | Off-grid/outside/overlap/over-seven-day rejection and owner-removal refusal | [Validation](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/writes.go), [owner removal](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/organizer.go) |
| W3 | find_best_times with only event_id | [optionalWindow](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/organizer.go) |
| W4 | get/list retain dates, duration, increment and weekdays | [Grid output](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/grid.go), [projection](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/db/mcp_events.go) |
| W5 | Display names and merged read ranges | [Read serialization](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/server.go) |
| W6 | Honest email_sent, duplicates_ignored, replaced and closed fields | [Writes](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/writes.go), [finalize](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/organizer.go) |
| W7 | Timestamped list_changes records | [Changes store](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/db/mcp_organizer.go); not a complete event log |
| W8 | Weekly reference instants and rejection of actual 2026 dates | [Grid rules](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/grid.go), [weekly validation](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/organizer.go) |

Current source accepts reference dates from 2018-06-17 inclusive to 2018-06-25 exclusive, including June 24; the shorthand generator must not narrow that existing raw-date acceptance ([reference-date validator](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/organizer.go)). New shorthand generation proposes Monday June 18 through Sunday June 24 under an explicit clock contract; existing raw instants remain valid.

### Policy checks and measurement

Nonowner finalize/update/remind/remove were locally verified to return NOT_OWNER for an invited member; source already implements ownership checks. A local availability-after-finalization call and archive:false call also succeeded; archived/finalized mutability is therefore a policy choice to document, not proof of a missing ownership check ([organizer](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/organizer.go), [availability](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/writes.go)).

Blind read protections have existing tests, but the full all-tool/second-account/live-host matrix requested by the user remains an implementation acceptance gate. Live Mongo atomicity, live indexes, and email delivery were not tested.

Measured locally: the complete compact JSON-RPC `tools/list` response using ID 1 is **12,472 UTF-8 bytes for 16 tools**. This is a byte measurement, not a token estimate; future 20-tool catalog size and amended response sizes remain null/unmeasured until implementation, with proposed 24/32 KiB catalog targets and an explicit reviewed-growth alarm.

Final-tip remeasurement is **12,666 UTF-8 bytes for the same 16 tools**, an increase of 194 bytes from description changes. The final-tip focused suites and isolated token test passed; the old 12,472 figure is historical, and neither measurement approves a future baseline transition.
