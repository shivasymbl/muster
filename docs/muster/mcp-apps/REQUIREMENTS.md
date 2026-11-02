# Requirements: Muster inside Claude

## Discovery Summary

The user confirmed full phased scope: research and specify interactive UI for all three existing Muster workflows, read-only first, then explicit confirmed writes, with Claude compatibility, performance, and implementation-ready tests. Success is a coding-agent handoff grounded in current code and primary specifications; implementation, deployment, external publication, unrelated infrastructure, and closure of held issues were not requested.

The original discovery settled the App scope. The full-build amendment adds F7 and explicit owner choices in OWNER_DECISIONS.md; production revision, live Mongo indexes/topology, target-host behavior, and implementation performance remain unverified. Unapproved policy choices and missing execution evidence block their affected gates without removing any original requirement.

## Executive Summary

Give signed-in Muster users interactive views for picking a time, answering availability, and managing a group directly inside a supported MCP host. Preserve the existing text interface and server authorization, deliver read-only first, and enable each mutation only after its own correctness and recovery gates pass.

Scope was confirmed by the user on 2 November 2026. Architecture recommendations below are decisions for this handoff, not claims that these features already exist.

## Global constraints

- The implementation MUST use the stable MCP Apps extension and a single self-contained UI resource family.
- It MUST NOT introduce SSE, subscriptions, `listChanged: true`, a metrics platform, a new account/login system, calendar creation, or a new email-delivery subsystem.
- It MUST preserve all three Markdown workflows and existing conversational tools.
- It MUST read `CLAUDE.md`, current branch state, and related code before editing; if HEAD moved, record the delta and adjust the baseline without discarding concurrent changes.
- It MUST NOT deploy, publish a directory connector, close issues #11/#26, or perform destructive database cleanup without separate authorization.
- Unknown, hidden, unloaded, failed, truncated, empty, declined, and cleared MUST remain distinct. Unknown values MUST NOT be converted to zero or empty arrays merely to render a view.
- Each feature MUST have real acceptance evidence. A compiled bundle or mocked browser test is not evidence of rendering inside Claude.

## Null Handling and contract consumers

The full field-level Null Handling Contract is in UX_AND_CONTRACTS.md and the input schemas are in `contracts/`. These invariants apply to every feature:

| Boundary case | Required behavior |
|---|---|
| Unknown or hidden count | Null/hidden status, never zero |
| Successful legacy own-answer read with nil lists | Normalize to DTO `[]`; preserve raw nil/missing state for CAS |
| Failed/incomplete own-answer read | Noneditable unavailable state, never empty draft |
| No existing event in creation mode | Omit event ID; explicit null is invalid |
| Empty edit change-set | No-op, never clear |
| Null action payload | Validation error, never mutation |
| Absent scheduled interval | Null, distinct from archived-with-time |
| Missing mail outcome | Unknown, never sent |

New contracts affect the browser view, raw MCP route, host resource loader, guarded store helpers, and, through F7, the conversational tools. Compatibility means existing field meanings and default modes remain unchanged except for explicitly approved, tested corrections; additive legacy output fields are permitted. The known old-core structured-content extension is intentionally preserved for legacy tools, while new App responses follow the explicit bounded version policy.

Amendment null additions: `scheduled:null` means no stored interval; `closed` derives from that interval, not merely `is_archived`; `skipped:[]` means no skipped inputs after a complete processed request; `display_timezone:null` means unspecified, not UTC; hidden response totals are null, never zero. Full input and lifecycle decisions are specified in UX_AND_CONTRACTS and OWNER_DECISIONS.

## F1: Protocol and deployable UI resource

### Metadata

Classification: Functional, Security, Operational. Priority: Critical. Status: proposed implementation. Data classification: authenticated scheduling information.

### Evidence and impact

The current dispatcher only resolves Markdown skills and hardcodes a legacy core version; its successful results already have structured data ([current route](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/routes/mcp.go)). Impact includes route schemas, result serialization, asset loading, build packaging, existing non-UI clients, and host discovery.

### Requirements

- `REQ-APP-001`: Add model-visible read-only `muster_open_view`, with `_meta.ui.resourceUri`, `visibility: ["model","app"]`, title, and accurate read-only annotations.
- `REQ-APP-002`: Add exact allowlisted `ui://muster/view/<bundle-hash>.html` resources with MIME `text/html;profile=mcp-app`; HTML MUST contain no user data, credentials, trackers, or external script/font dependencies.
- `REQ-APP-003`: Return short useful text, model-safe `structuredContent` on supported core revisions, and the authorized view in `_meta.muster`. Existing tools MUST retain their legacy response semantics.
- `REQ-APP-004`: Support the three bounded core revisions in RESEARCH.md; stateless per-request protocol handling MUST NOT leak capabilities across users.
- `REQ-APP-005`: Feature flag `MCP_APPS_ENABLED` MUST default off; when on, missing/invalid assets MUST fail readiness before advertising the opener. Unknown UI resources MUST return a typed error, never SPA fallback HTML.
- `REQ-APP-006`: Authenticated GET MUST remain 405, unauthorized GET MUST challenge, failure accounting MUST apply to GET and POST, and list-change/subscription claims MUST remain false.
- `REQ-APP-007`: The release MUST preserve a text-only workflow when Apps are unsupported, denied, or fail to load. Include the canonical authorized website URL in the text fallback; when the App runs and the host supports it, use its link API for a user-clicked “Open in Muster” action.

### Acceptance scenarios

**A1:** GIVEN a signed-in supported host, WHEN it lists tools and reads the linked URI, THEN it receives the correct MIME, self-contained HTML, and consistent metadata; existing Markdown resources still resolve.

**A2:** GIVEN a legacy/non-rendering client or denied App consent, WHEN it invokes the opener, THEN the text response remains useful; unsupported or missing assets do not produce a blank “successful” widget or silently mutate state.

### Failure behavior and standards

Fail closed on auth, unknown resources, invalid schemas, and unsupported action versions. Preserve request IDs and sanitized errors; do not log raw tokens or snapshots. Standards alignment: stable MCP Apps 2026-01-26, supported MCP transport/auth revisions, OWASP ASVS access-control/input-validation principles; this is an engineering mapping, not a certification claim.

### What This Is NOT

This is not a Go SDK migration, SPA rewrite, or notification transport project. Existing website and text workflows remain in place.

### Anti-Patterns and AI Development Guidance

Not a Go SDK migration or SPA rewrite. MUST NOT cache user capabilities globally, embed website cookies, or serve arbitrary filesystem paths. Read and modify `server/routes/mcp.go`, add `mcp_apps.go`, pass asset options through `server/main.go`, and test using injected resource bytes rather than requiring a frontend build for ordinary Go unit tests.

## F2: Authorized snapshots and read-only workflows

### Metadata

Classification: Functional, Security. Priority: Critical. Status: proposed implementation.

### Evidence and impact

Existing response output is windowed, redacted, range-merged, and capped; finalization data and lossless own-answer data are not present in the needed shape ([response implementation](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/server.go), [existing DTO](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/dto.go)). Impact includes a new aggregate, a narrow own-response store method, App state, and redaction tests; legacy response shape stays unchanged.

### Requirements

- `REQ-DATA-001`: Implement the versioned snapshot in UX_AND_CONTRACTS.md, including section status, server-derived permissions, exact instants/reference-week semantics, scheduled interval, and explicit population definitions.
- `REQ-DATA-002`: Read own answer through a dedicated untruncated authenticated store method, before mutating any shared privacy-filtered response objects. Preserve manual/calendar layers and nonvisible marks.
- `REQ-DATA-003`: Blind non-owners MUST receive no other-person identities, emails, counts, derived rankings, or inference-enabling totals in any response field, including `_meta`.
- `REQ-DATA-004`: Emails MUST be owner-only in the new App DTO; row identifiers MUST be opaque event-scoped keys, not emails or Mongo user IDs. Duplicate names MUST remain distinct.
- `REQ-DATA-005`: The inline App MUST summarize; expanded or paginated detail MUST carry the grid, ranked candidates, own answer, and group people view. No UI mutation helpers are enabled in this phase.
- `REQ-DATA-006`: Permission failure after refresh MUST clear sensitive cached content and disable actions. Hydration, theme changes, resize, refresh, and polling MUST NOT perform business mutations.
- `REQ-DATA-007`: Candidate duration MUST be a positive multiple of the stored increment. “Not fully available” MUST distinguish conditional availability from definitely unavailable; unseen invitees MUST not silently enter the ranker's respondent population.

### Acceptance scenarios

**A3:** GIVEN owner/member/blind-member/stranger identities, WHEN each opens and refreshes a workflow, THEN each view reveals exactly its allowed fields; malicious row labels render as text, and hidden totals cannot be inferred from another section.

**A4:** GIVEN an own answer spanning outside the visible window, WHEN the App loads an editable view, THEN the complete base remains intact; an incomplete, oversized, or ambiguous record is explicitly noneditable rather than an empty draft.

### Failure behavior, edge cases, and guidance

Missing own record means `exists:false`; present empty means cleared; failed load means unavailable, not cleared. Undated groups and weekly reference dates are explicit UI states, not guessed calendar dates. Test no answers, if-needed-only fits, duplicates, archived versus finalized, stale permissions, and malformed schema versions.

Standards alignment: least privilege and allowlist serialization under OWASP ASVS access-control/output-encoding principles. No provider calendar reads, account-wide preload, or hidden-data “convenience” in UI metadata. Add `app.go`, `app_dto.go`, and a narrow own-answer read in the existing dependency/store layer.

### What This Is NOT

This is not provider-calendar synchronization or a rewrite of shared legacy privacy output. It adds a narrower App contract without broadening existing users' access.

### Anti-Patterns and AI Development Guidance

MUST NOT infer completeness from a truncated payload or identities from names. Read the current privacy mutator and store adapters before building `app.go`; implement A3/A4 and isolated source-copy tests before allowing editing.

## F3: Complete-answer drafts and guarded saves

### Metadata

Classification: Functional, Security, Operational. Priority: High. Status: later phase, disabled until verified.

### Evidence and impact

The existing save replaces both lists and updates response/attendee/counter records separately; the 16 KiB request cap cannot carry every possible full answer ([write service](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/writes.go), [persistence](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/db/mcp_writes.go), [route limit](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/routes/mcp.go)).

### Requirements

- `REQ-ANSWER-001`: Editing MUST be local until explicit review and commit. Provide available, if-needed, and unavailable states, with a visible unsaved-draft indicator.
- `REQ-ANSWER-002`: Prepare a bounded interval change-set against the complete own-answer fingerprint. The server MUST apply changes to the complete stored answer; it MUST preserve off-window and noneditable layers unless an explicit clear operation covers them.
- `REQ-ANSWER-003`: “Clear my answer” MUST be a distinct operation and confirmation. Omitted/null input MUST NOT mean clear; empty changes mean no-op, not destructive replacement.
- `REQ-ANSWER-004`: Prepared action records MUST bind actor, event, operation, canonical payload/hash, expected target state, expiry, and immutable confirmation summary.
- `REQ-ANSWER-005`: Commit MUST reauthorize, atomically claim a single execution attempt, and use conditional target writes. A relevant editable target state that differs from the prepared base MUST conflict without overwriting it; preserve unrelated manual/calendar updates with dotted field updates. This field-comparison contract does not detect an intervening edit that restores identical values (ABA), and MUST NOT be described as a universal revision history.
- `REQ-ANSWER-006`: Duplicate responses MUST be audited before a unique event/user index is applied; ambiguous existing data MUST disable editing, not choose an arbitrary record.
- `REQ-ANSWER-007`: Timeout/crash/partial side effects MUST produce pending/partial/outcome-unknown states until reconciled. No automatic retry of an uncertain mutation is permitted.

### Acceptance scenarios

**A5:** GIVEN complete marks across two windows, WHEN the user edits one window and explicitly saves, THEN only the intended editable marks change while hidden-window and manual/calendar fields survive; cancel sends no mutation.

**A6:** GIVEN a changed answer, duplicate submit, expired plan, or timeout after target update, WHEN the user commits or checks the action, THEN the result is a conflict, same receipt, or explicit uncertain outcome; it never silently overwrites changed target values or re-executes.

### Failure behavior, standards, and limits

Invalid input fails before claim; known conditional-write miss is rejected; uncertain database outcomes are not labeled failure-with-no-effect. A response CAS is atomic only for that response document: a concurrent event-grid or membership edit in another collection is not made atomic by a pre-read. Recheck related policy immediately before execution, reconcile afterward, and document that cross-document race; do not claim serializable authorization or grid invariants.

Standards alignment: authorization on each operation, output encoding, replay resistance, and explicit failure handling under OWASP ASVS principles. No arbitrary executor or global workflow platform. Add narrow prepared-action and guarded-response methods; do not call the unconditional legacy writer after a superficial precheck.

### What This Is NOT

This is not an exactly-once transaction layer or a global revision migration. Cross-document consistency limitations remain explicit.

### Anti-Patterns and AI Development Guidance

MUST NOT retry an uncertain write, silently rebase a stale draft, or validate untouched legacy marks as though they were newly submitted. Implement the target-specific conditional helpers and fault injection before enabling `edit_own_answer`.

## F4: Review and finalize a selected time

### Metadata

Classification: Functional, Security, Operational. Priority: High. Status: later phase.

### Evidence and impact

The existing finalize stores `scheduledEvent` and archive state but performs an unconditional event update after authorization ([current finalize and persistence](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/organizer.go), [event store](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/db/mcp_organizer.go)).

### Requirements

- `REQ-FINAL-001`: Review MUST show exact start/end, timezone, duration, conditional attendance, and “Records the time and archives this Muster; creates no calendar event and sends no email.”
- `REQ-FINAL-002`: Only the owner may prepare/commit. Commit MUST validate the current grid and atomically compare the event fields relevant to the preview, including owner, dates, duration/increment, archive and scheduled state.
- `REQ-FINAL-003`: On success, refresh and show the stored scheduled interval; archive-without-time MUST remain distinct.
- `REQ-FINAL-004`: Changing candidate, duration, event, or relevant event fields MUST invalidate confirmation. Reopened old cards MUST refresh before prepare; attendance is a read-time estimate and MUST be rechecked, but the event CAS does not atomically freeze separately stored respondent answers.

### Acceptance scenarios

**A7:** GIVEN an owner and valid ranked time, WHEN the owner explicitly confirms the prepared action, THEN exactly that interval is recorded; the receipt and refreshed view agree and claim neither calendar creation nor mail.

**A8:** GIVEN an old card or concurrently changed event, WHEN its prepared action is committed, THEN stale expected event state is rejected rather than replaced; non-owner invocation fails even if the tool is called directly.

### Failure behavior and guidance

Use target-specific `UpdateOne` predicates and check `MatchedCount`; do not rely on app-only revision numbers that website writers never update. Standards alignment: ASVS authorization/replay principles. This is not a new immutable-closed-poll policy for every existing interface; do not silently change legacy archive semantics.

### What This Is NOT

This is not calendar booking, mail delivery, or a universal closed-poll policy. It records the supported Muster finalization with a conditional target write.

### Anti-Patterns and AI Development Guidance

MUST NOT equate archived with finalized or present read-time attendance as atomically frozen. Read `spanOnGrid`, finalization, and event update persistence; implement A7/A8 against actual matched-count behavior.

## F5: Group management with truthful outcomes

### Metadata

Classification: Functional, Security, Operational. Priority: High. Status: independently gated mutation phase.

### Evidence and impact

Create, add, and remove currently span multiple records, and `email_sent` is not a delivery receipt; removal also deletes the person's response ([group persistence](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/db/mcp_organizer.go), [held mail issue](https://github.com/shivasymbl/muster/issues/26)).

### Requirements

- `REQ-GROUP-001`: Support creating a group, adding/removing invitees, editing the already-supported group settings, and declining one's own invitation through separate prepared-action types.
- `REQ-GROUP-002`: Review MUST show canonicalized recipients, skipped duplicates, effects on answers, and owner nonremovability. Display only configured/supported effects.
- `REQ-GROUP-003`: Create MUST reserve a stable event identity in its action record; replay MUST NOT generate another event. Add/remove MUST use stable target keys and an explicit step/result record.
- `REQ-GROUP-004`: Unique response/attendee indexes require a read-only duplicate audit and an approved migration; no silent deletion or deduplication.
- `REQ-GROUP-005`: Multi-record effects MUST expose completed, partial, and unknown outcomes; claim is at-most-once execution attempt, not atomic exactly-once completion. Uncertain mail MUST never be retried automatically.
- `REQ-GROUP-006`: Follow-up output is “people to follow up with,” not “reminders sent.” `email_sent:true` may be displayed only as the server-reported flag with “delivery unverified”; false MUST NOT become a successful-mail claim.
- `REQ-GROUP-007`: Full group writes MUST stay disabled until fault-injection tests for each operation pass. Read-only group views need not wait.

### Acceptance scenarios

**A9:** GIVEN a prepared group with duplicate/case-variant emails, WHEN creation is confirmed and the same action is replayed, THEN review and persisted targets agree and no second event is created; a changed payload requires a new preparation.

**A10:** GIVEN a failure between attendee and event writes or between deletion steps, WHEN the action result is inspected, THEN the UI reports partial/unknown state with available reconciliation detail, not “all done”; removing the owner is refused.

### Failure behavior and guidance

No replica-set or transaction prerequisite is assumed. Use the bounded operation records and per-target conditional helpers; if an operation cannot meet the stated duplicate/failure semantics on the current topology, leave that action disabled and return a specific blocker instead of inventing infrastructure.

Standards alignment: ASVS access-control, validation, and failure-handling principles. Keep Microsoft login and actual email-delivery work in their existing issues; do not add a mail queue or retry platform under this feature.

### What This Is NOT

This is not the deferred email-delivery, Microsoft login, or Mongo-topology project. Read-only group views do not depend on those projects.

### Anti-Patterns and AI Development Guidance

MUST NOT silently delete duplicates, retry uncertain mail, or generate a fresh event ID on receipt replay. Audit current indexes and use isolated fault-injection fixtures before enabling each group mutation.

## F6: Performance, accessibility, lifecycle, and release proof

### Metadata

Classification: Performance, Functional, Operational. Priority: High. Status: required throughout.

### Evidence and impact

Claude documents compact inline design, mobile host differences, and a large-result hydration failure mode; the performance and host gates below apply to the new browser entry, payload builder, and release evidence ([design guide](https://claude.com/docs/connectors/building/mcp-apps/design-guidelines), [troubleshooting](https://claude.com/docs/connectors/building/mcp-apps/troubleshooting)). These are new App requirements, not a claim that existing production performance is defective.

### Requirements

- `REQ-UX-001`: Support light/dark host themes, live host-context updates, 320px containers, keyboard-only operation, screen-reader labels, non-color status cues, and reduced motion.
- `REQ-UX-002`: Inline view MUST have no nested vertical-scroll trap; use compact summaries and at most two bottom actions, with a usable fallback if expanded mode is refused.
- `REQ-PERF-001`: Proposed budgets: common initial result ≤50,000 serialized characters; hard application guard ≤120,000 characters; HTML bundle target ≤300 KiB uncompressed and ≤100 KiB gzip-equivalent; no external runtime assets.
- `REQ-PERF-002`: Proposed measured targets after widget execution begins: skeleton ≤200ms, render ≤500ms after data arrival, visible-page local interaction ≤100ms, bounded read p95 ≤2s on the documented fixture. Host/model/network time outside the measured interval MUST be reported separately.
- `REQ-PERF-003`: Start with 20 candidates and 25 people per page; cancel/discard obsolete reads, coalesce filtering, avoid mount waterfalls and per-cell RPCs, and respect the existing 60/minute user limit.
- `REQ-LIFE-001`: Treat each card as a snapshot; no unconditional background polling. Refresh on explicit request and before prepare; never assume newest mount equals newest data.
- `REQ-LIFE-002`: Use an optional coalesced model-context update only after a meaningful selection or confirmed outcome, not every hover/keystroke. Failure of optional context APIs MUST NOT roll back a committed operation.
- `REQ-RELEASE-001`: Real Claude web/Desktop evidence, text-only fallback evidence, and separate mobile status MUST accompany release claims. No numeric host limit may be inferred from these proposed engineering budgets.
- `REQ-EFF-001`: Text reference tasks MUST obey the call ceilings in the amendment efficiency table, counting failed calls and retries; initial connection/discovery is recorded separately and excluded only when shared and amortized.
- `REQ-EFF-002`: Each release MUST record the exact serialized `tools/list` UTF-8 byte size, tool count, and comparison with a reviewed baseline. Proposed soft target is 24 KiB, hard ceiling 32 KiB, and growth over 10% raises a blocking review alarm unless the exact measured transition is approved. The final-tip 12,666-byte, 16-tool baseline (historically 12,472 bytes at 59b1a48b) is measured, not an approval of future growth.
- `REQ-EFF-003`: Designated typical reference-task/compact responses and model-safe summaries MUST be at most 2,000 Unicode code points each; a full legacy diagnostic result that cannot meet this without information loss MUST use an explicitly requested compact projection or pagination, not silent field removal. Every large-fixture text page MUST be at most 20,000 code points with valid pagination where needed.
- `REQ-EFF-004`: The App's 50,000/120,000-character and asset/render budgets remain unchanged and separate. Efficiency evidence MUST distinguish model-visible content, structured summaries, total wire bytes, and App bridge calls; moving required truth into `_meta` is not a text-path efficiency fix.
- `REQ-EFF-005`: Reference tasks MUST remain well below the existing 60-calls/minute bucket and MUST NOT evade throttling or retry hidden calls. Missing/stale/number-only measurement evidence blocks the efficiency gate.
- `REQ-EFF-006`: Short App-helper descriptions MUST explicitly say “App only”; commits without a valid actor-bound prepared action ID MUST fail server-side even if a host ignores visibility. The full-build gate measures all 20 tools, not only the model-filtered catalog; the earlier text-only gate measures its actual enabled 16/17/20-tool surface without requiring later App helpers to exist. Stage is selected by the runner, never self-declared to evade the full gate.

### Acceptance scenarios

**A11:** GIVEN narrow/wide containers, both themes, keyboard and touch input, WHEN users inspect and operate the workflow, THEN controls remain usable without clipping; performance tests report fixture/device and stay within budgets or block with measured evidence.

**A12:** GIVEN denied rendering, unsupported expansion, malformed/oversized data, restored old cards, or an expired token, WHEN the workflow is opened or resumed, THEN the user gets useful text or an explicit recoverable state and no hidden mutation.

### Failure behavior, guidance, and exclusions

Return recoverable loading/error/incomplete states rather than a blank iframe. Clear private state on denied refresh. Tool-result limits and UI guidance come from Claude's documentation, while these smaller budgets are project decisions ([troubleshooting](https://claude.com/docs/connectors/building/mcp-apps/troubleshooting), [design guide](https://claude.com/docs/connectors/building/mcp-apps/design-guidelines)).

Standards alignment: WCAG 2.2 AA as the proposed accessibility target, stable Apps lifecycle, and least-disclosure logging. No production analytics program or benchmark claim without measurements. Required agent work includes tests, bundle inspection, mock-host tests, and actual-host evidence with client versions.

### Performance

Measure the F6 budgets on documented fixtures and device conditions; distinguish local render time from model and host/network latency. Regression output must include serialized size and tested population/window, not just a pass label.

### Security and observability

Record sanitized test receipts, request IDs, and error categories only. Do not add production analytics or export raw private snapshots; these are release-proof artifacts, not a metrics program.

### What This Is NOT

This is not a claim of universal Claude support or a service-level agreement. Untested hosts remain explicitly unverified.

### Anti-Patterns and AI Development Guidance

MUST NOT replace real-host evidence with mock screenshots, accept empty test suites, or use host maximum limits as performance targets. Build the mock-host tests and real-client checklist independently, and keep their evidence labels distinct.

### Text efficiency acceptance

| Reference task | Maximum text tools/call operations | Required proof from that call |
|---|---:|---|
| Mark 15:00–17:00 on a known valid grid | 1 | Stored affected-window answer, not only submitted intent |
| Same task with unknown grid | 2 | One discovery read and one confirmed write |
| “When should we meet?” | 1 | Valid ranked candidates and the actual window/duration |
| Owner “How's it going?” | 1 | Owner-authorized counts in get_event |
| Create group with invitees/share link | 1 | Created event, canonical targets, honest outcomes, share URL |
| Finalize and confirm stored time | 1 | Stored scheduled interval in finalization receipt |
| Rename and confirm | 1 | Updated event echo |
| Archive and confirm | 1 | Updated event echo |

For these measurements, the intended target ID and user-authorized intent are provided in the fixture; entity discovery is not hidden inside setup where the task explicitly says unknown grid. Count all model-initiated tool attempts, not just successful ones; setup, initialization, catalog reads, and App bridge calls are retained as separate evidence.

The 2,000-character typical gate applies to the above task receipts and designated compact diagnostic projections on the 25-person/seven-day fixture. It does not authorize truncating a complete own-answer base or relabeling a full payload as a model-safe summary; full compatibility output still obeys its established completeness contract and the large-page bound.

## F7: Conversational tool parity and efficiency

### Metadata

Classification: Functional, Operational, with security-sensitive error/privacy/write subrequirements. Priority: High. Status: full-build amendment; implementation not performed. Owner decisions D1–D6 remain unapproved.

### Evidence and impact

The user reported 33/34 production checks passing on 2 November 2026, plus FE-1–FE-13 gaps; the final reviewed tip is cd089433, with the original 59b1a48b evidence retained. Code already includes flat structured errors, full grid reads, access checks, pagination and earlier regression fixes; the late delta adds skipped address strings and accurate rejection/rounding documentation, but not reason metadata or ranged/weekly inputs ([original route](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/routes/mcp.go), [late MCP delta](https://github.com/shivasymbl/muster/commit/cfaffa10)).

Affected consumers are Claude Code, the Messages API, denied/unverified App hosts, existing conversational clients, the App's shared domain helpers, and website writers competing with conditional operations. F7 does not wait for UI rendering, but its ranged-write primitives must meet the same F3 safety and byte/range limits.

### Requirements

- `REQ-TXT-001` / FE-1: `muster_set_availability` MUST add an authoritative stored-answer echo for the affected window, merged normal/if-needed ranges, `my_status`, `changed`, and the window used, alongside existing count/receipt fields. A receipt MUST distinguish `echo_complete` and uncertainty; never echo submitted input as if persistence were verified.
- `REQ-TXT-002` / FE-2: `muster_update_event` MUST add `event` in the same additive shape as `muster_get_event`, keeping `event_id` and `updated`. The returned event MUST reflect a post-write authoritative read, conditional return, or equivalent persisted result; no follow-up caller read is needed.
- `REQ-TXT-003` / FE-3: `muster_set_availability` MUST accept `ranges:[{start,end,state}]` and `mode:"replace"|"merge"` with default replace for compatibility. Range mode MUST reuse the F3 half-open interval engine, conflict validation, complete base handling, 100-range cap, 12,288 UTF-8 argument-byte cap, and 16,384-byte total-body cap. New ranged writes MUST use internal read-plus-target-CAS even without a caller fingerprint; a supplied fingerprint MUST reject stale state. Legacy array-only unconditional replace is an explicit compatibility exception awaiting D4, not an equivalent safety guarantee.
- `REQ-TXT-004` / FE-4: `muster_get_responses` and `muster_summarize_overlap` MUST allow both start/end omitted or null together, use the shared event-derived at-most-seven-day default, and return the actual window. One-sided or invalid windows MUST be rejected; undated events need an actionable error, not invented dates.
- `REQ-TXT-005` / FE-5: Tool execution errors MUST retain `isError:true` and existing flat structured fields, add canonical `data.code`, `message`, sanitized offending value/path/index, `what_to_do_next`, and authorized valid bounds where relevant, and serialize equivalent parseable error information in text content. Canonical codes include SLOT_OUTSIDE, SLOT_OFF_GRID, SLOT_IN_BOTH, WINDOW_TOO_LONG, BAD_ARGUMENTS, NOT_FOUND, NOT_GROUP, NOT_OWNER, OWNER_NOT_REMOVABLE, OVERLAP_HIDDEN; existing flat code values remain compatible aliases where they differ. Invalid IDs and inaccessible events stay nondisclosing.
- `REQ-TXT-006` / FE-6: Upgrade original descriptions and every input parameter's descriptions/formats/examples/replace-merge semantics; every tool MUST have truthful annotations. Reads are read-only/idempotent; set/update are destructive/idempotent only after repeated identical calls cause no extra domain effects; add-attendees is nondestructive/idempotent only after deduplication/no-mail-retry proof. Finalize/remove/decline are destructive; do not assert stronger idempotence without proof. Test all 16 base tools and the new four separately.
- `REQ-TXT-007` / FE-7: Add `scheduled:{start,end}|null` and `closed:boolean` to legacy event output, retaining every existing field. `closed` means a stored finalization interval exists, independent of manual archive; it does not itself silently impose a new lock policy. Finalize MUST also echo the stored interval so confirmation is one call.
- `REQ-TXT-008` / FE-8: Preserve the newly shipped `skipped:string[]` and add `skipped_details:[{email,reason,input_index,basis}]`, with reasons already_invited, owner, invalid and a basis distinguishing an existing attendee from an earlier duplicate input. Preserve atomic invalid-batch rejection with per-input diagnostics and no partial writes. A new recipient repeated in the input may occur in both legacy added and skipped; details MUST explain the skipped occurrence rather than falsely treating these arrays as disjoint.
- `REQ-TXT-009` / FE-9: Select omitted-both-list behavior through D2 and align code, instructions, and answer skill. Recommended is authenticated success no-op with `changed:false`; explicit empty replace remains clear, and null/omission is never an implicit clear. The alternative existing validation error is acceptable only if D2 explicitly selects it and all wording agrees.
- `REQ-TXT-010` / FE-10: Shared ranking MUST reject nonpositive, nonmultiple, and unsupported durations with BAD_ARGUMENTS instead of promotion/rounding. Preserve legacy `missing` meaning while adding disjoint `if_needed_names` and `unavailable_names`/counts; do not sum overlapping legacy categories or infer unique identity from duplicate display names.
- `REQ-TXT-011` / FE-13: Add status open|archived|all, needs_my_answer boolean, and projection full|compact to `muster_list_my_events`; recommended default status all and projection full preserve compatibility, subject to D3. Filter before paging/cursor advancement and state applied filters; compact is explicit, not silent removal of full-grid fields.
- `REQ-TXT-012`: Add owner-only response totals to get_event: responded, invited, declined, cleared, pending count, explicit population definitions and owner participation information. Nonowners/hidden views receive null, never fabricated zero. Pending follows the approved D1 rule; status categories and follow-up population MUST not be conflated.
- `REQ-TXT-013`: Weekly creation and set-availability MUST accept `{weekday,time}` and return both shorthand and canonical reference-week representation. Apply only to `dow` or an explicitly reference-week group; never convert actual dated/undated groups or guess the current week. Recommended clock is reference UTC, pending D6; dates retain the existing 2018 reference instants.
- `REQ-TXT-014`: Add optional stored `display_timezone` as a validated IANA name for presentation only. Missing create value means unspecified/null, omitted update preserves, explicit null clears; UTC stored instants, grid validation, and weekly reference mapping MUST not change when this label changes. Do not infer an IANA zone from a numeric user offset.
- `REQ-TXT-015` / FE-11: D1 MUST settle owner participation versus invitee/follow-up treatment; apply one documented rule to attendee rows/totals, remind output, owner counts, App follow-up lists, and REQ-GROUP-006. Preserve old field meanings unless D1 explicitly authorizes an identified behavior correction.
- `REQ-TXT-016` / FE-12: Model-visible descriptions/instructions MUST describe mail configuration and observed effects without third-party provider names. Keep truthful `email_sent` qualifications and the held delivery limitation.
- `REQ-TXT-017`: Explicitly test nonowner finalize/update/remind/remove, archived/finalized set/add, archive:false, and blind behavior across every text tool. Preserve current allowed lifecycle behavior unless D5 selects a tested correction; access failures and hidden aggregates use defined nondisclosing codes. Local tests are not a substitute for the missing second-account live check.

### Acceptance scenarios

**A13:** GIVEN a successful availability write, WHEN its receipt is read, THEN counts and authoritative affected-window stored ranges/status agree without another tool call; uncertain readback is explicit.

**A14:** GIVEN a rename or archive request, WHEN update completes, THEN its event echo matches get_event's additive contract and preserves all legacy receipt fields.

**A15:** GIVEN aligned ranges and a current base, WHEN replace or merge runs, THEN half-open state assignment is deterministic, conflicting overlaps fail, shared limits apply, merge preserves untouched values, and a stale target CAS never overwrites changed values.

**A16:** GIVEN a dated accessible event, WHEN responses or overlap is called with only event_id, THEN the bounded chosen window is returned and explicit legacy windows still work.

**A17:** GIVEN a repairable invalid slot/window/range, WHEN a text-only client sees the error, THEN it can parse canonical code, offending index/value, allowed bounds, and repair guidance in one retry without a discovery read; strangers learn no event bounds.

**A18:** GIVEN the complete base and App-helper catalogs, WHEN schemas/annotations and repeated-call effects are checked, THEN every parameter is documented, annotations are accurate, true idempotent claims have evidence, and catalog bytes are recorded.

**A19:** GIVEN manual archive versus finalization, WHEN event/finalize output is read, THEN scheduled and closed distinguish them while unchanged legacy fields retain their meaning.

**A20:** GIVEN existing, owner, duplicate, invalid, and new addresses, WHEN add-attendees executes, THEN it preserves skipped address strings, adds per-occurrence reason details, and never partially applies an invalid legacy batch.

**A21:** GIVEN both availability lists absent/null, WHEN the tool is called, THEN approved D2 behavior occurs without changing the answer and matches initialization instructions and the answer skill.

**A22:** GIVEN 20 minutes on a 30-minute grid and a conditional-only respondent, WHEN ranking is called, THEN the duration is rejected and valid-duration output separately names conditional and unavailable people without redefining missing.

**A23:** GIVEN mixed open/archived/finalized events and response states, WHEN list filters and compact projection are requested, THEN filtering precedes pagination, the approved default stays compatible, and needs-my-answer returns a short correctly scoped page.

**A24:** GIVEN owner and blind/nonowner callers, WHEN get_event runs, THEN owner counts and population labels match the approved owner rule while unauthorized totals remain null and no extra read is required.

**A25:** GIVEN a weekly shorthand on an eligible reference-week muster, WHEN create/set executes, THEN shorthand and canonical 2018 forms agree under the approved clock and actual dated/undated events are not silently remapped.

**A26:** GIVEN a valid/invalid/omitted/null IANA display timezone, WHEN create/update/read/App rendering runs, THEN label storage and null semantics are correct and all original UTC instants remain unchanged.

**A27:** GIVEN an unanswered owner and nonowner invitees, WHEN attendee totals, remind, get_event counts, and App follow-up render, THEN all follow the recorded D1 rule with explicit populations.

**A28:** GIVEN every model-visible tool description, WHEN provider-name and behavior checks run, THEN no third-party mail vendor is named and mail/delivery limits remain truthful.

**A29:** GIVEN nonowners, archived/finalized events, unarchive requests, and blind events, WHEN every affected text tool is tested, THEN the approved policy produces an explicit allowed result or defined error without unauthorized data or side effects.

### Failure behavior, standards, and tests

Validate authorization before revealing valid slot bounds; bound offending values and never echo credentials. Keep tool execution errors distinct from JSON-RPC transport/method errors; canonical aliases must not redefine existing flat codes. Use server conditional writes for new range mode and keep uncertain outcomes explicit.

Each requirement has its own `TestMCPTextParity/REQ_TXT_001` through `_017` test. `TestMCPFieldFindings` contains FE1–FE13 and W1–W8 subtests; changed behaviors FE1–FE10/FE12/FE13 need a recorded red baseline and green implementation using the same focused assertion. FE5 red asserts lost text error structure, not absence of existing structured codes; FE9 red tests the chosen correction only after D2 approval.

Preserve all prior Apps tests. Actual Mongo conditional/uniqueness tests are required before shipping ranged writes; schema tests, parent-only Go passes, or fake stores do not satisfy that gate. Efficiency is tested through text tools independently of App rendering.

### What This Is NOT

F7 is not permission to remove legacy fields, silently change existing defaults, make an App mandatory, introduce extra public tools for every shortcut, or bypass host/server consent. It does not close the held mail/auth/CI issues or claim the supplied live results were independently reproduced.

### Anti-Patterns and AI Development Guidance

Read the exact current route serialization, store methods, optional-window helper, ranker, and specs before editing. Add F7 primitives at T1.5 and reuse them at T2–T6; do not make ranged text writes depend on HTML or implement a second validator. Policy-specific tests stay blocked until OWNER_DECISIONS records explicit choices.
