# Architecture decisions

These decisions are single-author synthesis reviewed against parallel code, protocol, and host research. No external AI Council was used; they are implementation recommendations, not changes already made.

## ADR-Protocol: Official MCP Apps over generic embedded HTML

Decision ID: ADR-APP-001.

**Context:** The requirement is interactive UI inside Claude using the existing MCP connector.

**Decision:** Use stable MCP Apps, `ui://` HTML, tool UI metadata, and the browser Apps SDK. Keep the server's existing Go route; no Go SDK migration or proprietary website bridge.

**Consequences:** Real host support remains a release gate, and legacy text clients remain first-class. Generic HTML text, old vendor-specific widget MIME types, and iframe embedding of the SPA are rejected because they are not this standard ([official overview](https://modelcontextprotocol.io/extensions/apps/overview)).

## ADR-Build: Isolated browser entry and shared static asset location

Decision ID: ADR-APP-002.

**Context:** The SPA includes cookie auth, analytics, Vuex/router dependencies, and a different extension bridge ([frontend entry](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/frontend/src/main.js), [repository guidance](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/CLAUDE.md)).

**Decision:** Add `mcp-app/` with vanilla TypeScript, Apps SDK 1.7.5, a small state reducer, and single-file bundling to `frontend/dist/mcp-app/`. Reuse only audited pure utilities, not the website bootstrap.

**Consequences:** A separate build stage/runtime-compatible lockfile is necessary; the existing SPA remains unchanged. Use runtime asset loading through the resolved dist path, avoiding a generated-file dependency in ordinary Go builds.

## ADR-Discovery: One opener rather than UI on every tool

Decision ID: ADR-APP-003.

**Context:** Chained reads should not create several competing widgets.

**Decision:** One `muster_open_view` returns an explicit workflow snapshot. Refreshes stay in the current App through the host bridge; old text tools remain available and do not each render a new App.

**Consequences:** The new aggregate needs precise privacy/completeness semantics. It is not allowed to fabricate completeness by combining truncated legacy payloads.

## ADR-Disclosure: Model-safe summaries and authorized UI metadata

Decision ID: ADR-APP-004.

**Context:** Primary documentation differs on whether structured content reaches the model, and Claude Code can expose it as text ([Claude quickstart](https://claude.com/docs/connectors/building/mcp-apps/quickstart)).

**Decision:** Treat both `content` and `structuredContent` as model-visible. Put richer authorized UI data in `_meta.muster`, but exclude all data the user is not permitted to see.

**Consequences:** No secret/private-permission shortcut exists through `_meta`; host field handling is not an authorization boundary.

## ADR-Transport: No notification stream or new analytics program

Decision ID: ADR-APP-005.

**Context:** Existing deployment is stateless and read-only UI does not require push synchronization.

**Decision:** Keep authenticated GET405, false list-change claims, explicit refresh, and local performance tests.

**Consequences:** Old cards remain snapshots and refresh before action preparation. Deploy-time catalog behavior is tested separately; new tool discovery may require the host's supported reconnect flow.

## ADR-Writes: Narrow prepared actions, at-most-once attempt

Decision ID: ADR-APP-006.

**Context:** Existing writes lack conditional target updates and span separate Mongo records; topology and live indexes are unverified ([current persistence](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/db/mcp_writes.go)).

**Decision:** Add a closed operation enum with immutable preparation, atomic claim, target-specific conditional helpers, and explicit partial/unknown receipts. No automatic retry of uncertain side effects and no exactly-once-completion claim.

**Consequences:** More code than a client confirmation modal, but bounded to the approved actions. Single-document CAS does not create cross-document atomicity; stronger global guarantees or Mongo topology changes are excluded.

## ADR-Rollout: Staged support claims

Decision ID: ADR-APP-007.

**Context:** Documentation establishes platform capability, not actual Muster behavior.

**Decision:** Read-only web/Desktop evidence first, then per-action write gates, then separately verified mobile support. Cowork is secondary; the API/Claude Code text path is preserved without claiming App rendering.

**Consequences:** Screenshots, host versions, payload sizes, and failure evidence are required. A green mock-host test cannot substitute for a real-host test.

## Amendment: Text parity and review-gap decisions

This amendment adds F7 and T1.5 to the full T0–T7 scope without removing or relaxing any existing gate. The original seven ADRs above remain unchanged; recommendations labeled D1–D6 are pending owner decisions, not approvals granted by this document.

### ADR-Text-Parity: Text workflows are an independent delivery surface

Decision ID: ADR-TXT-PARITY. Review mapping: GAP1; F7, REQ-TXT-001 through REQ-TXT-017, T1.5.

- **Context:** The existing route already emits structured success objects and flat typed errors, but error text contains only the message, so FE5 is a text-serialization gap rather than missing error codes ([route](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/routes/mcp.go), [error DTO](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/dto.go)).
- **Decision:** Treat text parity as required full-build scope before UI rendering dependencies. Preserve flat structured error fields and existing flat code aliases, add the canonical nonrecursive `data` error object, and serialize equivalent parseable error information in text; replacing plain-message text with JSON is an intentional representation correction, not a claim of byte-for-byte compatibility.
- **Protocol boundary:** Domain errors remain tool results with `isError:true`; authentication and protocol failures retain their appropriate HTTP/JSON-RPC handling, because MCP distinguishes tool execution failures from failures to find or invoke the protocol method ([MCP error contract](https://raw.githubusercontent.com/modelcontextprotocol/modelcontextprotocol/main/schema/2025-11-25/schema.ts)).
- **Consequences:** Tests must cover both flat legacy consumers and canonical `data.code` consumers, nondisclosing access errors, and text-only hosts. Authoritative set/update/finalize echoes remove unnecessary caller reads, but must not present submitted input as confirmed persistence.
- **Rejected alternatives:** Do not claim the App's `_meta` repairs text parity, do not change all domain errors into JSON-RPC errors, and do not suppress errors into successful partial receipts. Invalid-email batches retain rejection before mutation, with sanitized per-input/skipped diagnostics; silently accepting valid addresses from an otherwise invalid legacy batch is rejected ([current batch validation](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/writes.go)).

D2 remains pending: recommend authenticated success with `changed:false` when both availability lists are absent/null, while preserving explicit empty-replace clearing; retaining the current validation error is the alternative if the owner selects it. Until that choice is recorded, neither the implementation nor the instructions may assert that the no-op correction is approved.

The designated typical-task and compact-projection bound is 2,000 Unicode code points, with large text pages bounded at 20,000 code points and valid pagination. These are not permission to truncate a complete own-answer base, silently remove fields from a full diagnostic result, or substitute a summary for required truth; existing App, wire-byte, and storage bounds remain separate.

### ADR-Range-Safety: Share transformations and guard new range writes

Decision ID: ADR-TXT-RANGES. Review mapping: GAP2; REQ-TXT-003, F3, D4.

- **Context:** The existing array setter is whole-answer replacement and the storage adapter performs an unconditional existing-response update after reading it; manual/calendar layers are preserved separately ([setter](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/writes.go), [storage](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/db/mcp_writes.go)).
- **Decision:** New ranged text writes and F3 use the same pure half-open range normalization, grid/conflict validation, complete-base transformation, and conditional target-write primitives. New ranged/merge text calls load current authorized state and use target CAS within one invocation even when `expected_state` is omitted; a supplied fingerprint additionally rejects a stale caller snapshot.
- **Compatibility recommendation:** D4 remains pending: recommend preserving raw-array unconditional replace as a documented legacy exception while making new range forms guarded. Omitted `mode` remains replace; an alternative that guards every legacy write is a deliberate behavior change requiring approval and regression evidence.
- **Consequences:** Keep the 100-range, 12,288 UTF-8 argument-byte, 16,384 total request-body-byte, and existing per-list expanded-mark limits. CAS conflicts do not trigger silent rebase/retry; unknown outcomes, duplicate source records, and multi-record partial effects retain F3/F5 safety handling.
- **Rejected alternatives:** A pre-read followed by the old unconditional writer is not CAS. Shared code is not a claim of shared legacy concurrency guarantees, and one response-document CAS does not make event-grid, membership, attendee, or counter effects atomic.

One call is the efficiency target when intent and grid are known, not a ban on internal reads. A necessary grid/intent read may make the text workflow two calls; guessing dates, increments, or a target to meet a call count is rejected.

### ADR-Event-Contract: Add lifecycle fields without rewriting old output

Decision ID: ADR-TXT-EVENT. Review mapping: GAP3; REQ-TXT-002, REQ-TXT-007, REQ-TXT-011, D3, D5.

- **Context:** The legacy `EventRow` contains existing event/grid/status fields but lacks the requested stored finalization shape ([event DTO](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/dto.go)).
- **Decision:** Permit additive legacy event fields, including `scheduled:{start,end}|null` and `closed:boolean`, while retaining every old field and its meaning. Return the same additive event shape in authoritative update echoes rather than introducing a second incompatible event schema.
- **Lifecycle recommendation:** D5 remains pending: recommend preserving currently allowed archived/finalized operations and `archive:false`, with an explicit statement that `closed` describes stored finalization, not a newly enforced lock. A global closed-event lock across text/UI/website writers requires separate owner approval and coordinated tests.
- **List recommendation:** D3 remains pending: recommend default `status:all` and `projection:full`, with explicit compact projection and filtering before pagination. This preserves existing broad listing behavior rather than silently hiding archived records or grid fields.
- **Consequences:** Existing compatibility statements forbidding shape changes mean “no removed/redefined legacy fields,” not “no additive fields ever.” New owner-only aggregates remain authorization-filtered, and new fields must not expose hidden totals.
- **Rejected alternatives:** Do not replace `archived` with `closed`, infer a scheduled interval from archive state, return only the submitted patch as a persisted event, or silently make compact output the full legacy default.

### ADR-Shared-Ranking: Reject unsupported duration at the common boundary

Decision ID: ADR-TXT-DURATION. Review mapping: GAP4; REQ-TXT-010 and the existing ranking prerequisite.

- **Context:** Ranking is shared domain behavior, while candidate output and duration validation are distinct concerns; its current algorithm evaluates stored respondents over the candidate span ([ranker](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/rank.go)).
- **Decision:** Put positive, supported, increment-multiple duration validation at the common service boundary used by text and App ranking. A 20-minute request on a 30-minute increment is `BAD_ARGUMENTS`, not rounding, promotion, or a successful empty result; a supported 30-minute request is evaluated normally.
- **Consequences:** Preserve the existing candidate-result maximum of 20 and other range/output limits. This count of returned candidates must not be confused with duration minutes or changed to 30 while fixing the 20/30-minute validation case.
- **Rejected alternatives:** Frontend-only validation leaves text calls inconsistent; silently substituting a duration changes user intent. Altering the unanimous overlap algorithm is not part of the duration correction.

### ADR-Rank-Categories: Preserve missing and add disjoint explanations

Decision ID: ADR-TXT-CATEGORIES. Review mapping: GAP5; REQ-TXT-010.

- **Context:** Ranking's legacy `missing` contains everyone not fully unconditionally available, including conditional respondents; it is not an unavailable-only category, and this field belongs to ranking rather than unanimous overlap ([ranker](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/rank.go), [overlap service](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/writes.go)).
- **Decision:** Preserve `missing` unchanged and add disjoint `if_needed_names` and `unavailable_names` with corresponding counts. Explain the stored-respondent population and derive membership from identities internally, never display-name set arithmetic.
- **Consequences:** For complete authorized output, available plus conditional plus unavailable partitions the population; legacy missing overlaps conditional and must not be added again. Duplicate names remain distinct people, and hidden/nonresponding invitees are not silently added to ranking.
- **Rejected alternatives:** Redefining old `missing` as unavailable would break meaning; changing a label alone without exposing disjoint categories would leave the original ambiguity. Hidden counts cannot be restored in error text, summaries, or `_meta`.

### ADR-Catalog-Discipline: Keep twenty bounded tools and truthful hints

Decision ID: ADR-TXT-CATALOG. Review mapping: GAP6; REQ-TXT-006, REQ-TXT-016 and the App action contract.

- **Context:** The reviewed catalog contains 16 base tools; the planned opener and prepare/commit/get-action helpers bring the complete enabled catalog to 20, without adding one public tool per operation ([base tool definitions](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/routes/mcp.go)).
- **Decision:** Improve every base tool and parameter description, while keeping the three App-only helper descriptions short and precise. Closed typed operations remain behind the existing generic action helpers, not a proliferation of new tool names.
- **Server gate:** App-only visibility is host discovery/access guidance, not authorization; commit/get-action require a valid actor-bound action ID and server-side state checks, and prepare enforces its closed operation schema and byte/range bounds ([stable visibility](https://raw.githubusercontent.com/modelcontextprotocol/ext-apps/main/specification/2026-01-26/apps.mdx)).
- **Hints:** Reads remain read-only; write hints must match implemented effects. MCP defines idempotence as repeated identical arguments causing no additional environmental effect, so requested true hints require no-op/deduplication corrections and tests covering domain state, observable timestamps, and mail; otherwise do not ship true ([annotation definition](https://raw.githubusercontent.com/modelcontextprotocol/modelcontextprotocol/main/schema/2025-11-25/schema.ts)).
- **Consequences:** Set/update/add idempotency is proved, not inferred from eventual primary values. Add remains nondestructive only for additive effects, external mail requires an appropriate open-world hint, prepare is a write, and commit replay may claim idempotency only when the same action ID never reexecutes.
- **Rejected alternatives:** Unqualified true-idempotent annotations, retry middleware based only on hints, an arbitrary tool-name executor, and descriptions containing provider branding instead of truthful mail configuration/effect are rejected.

Do not weaken invalid-batch rejection to make add-attendees appear efficient or idempotent. Canonicalization, owner/already-invited skips, input diagnostics, atomic validation-before-write, and mail/replay truth must agree across descriptions, receipts, and tests.

### ADR-Owner-Population: Separate participation from actionable follow-up

Decision ID: ADR-TXT-OWNER. Review mapping: GAP7; REQ-TXT-012, REQ-TXT-015, REQ-GROUP-006, D1.

- **Context:** Legacy attendee totals count attendee statuses, while follow-up excludes declined/nonempty-answer records without explicit owner exclusion; existing owner rows therefore require a policy rather than an assumption that they cannot exist ([attendee and follow-up service](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/organizer.go)).
- **Recommendation pending D1:** The owner remains a participant with existing roles/membership preserved, but is never an actionable follow-up target. Keep existing status totals labeled with their all-attendee population and add explicit actionable-pending/non-owner invitee fields instead of redefining all totals.
- **Intentional correction requiring D1 approval:** Excluding the owner from the existing `muster_remind.pending` field changes legacy behavior, even if it corrects the product intent. Apply that correction only after D1 is recorded as approved, then use the same actionable population in text follow-up, owner counts, and App follow-up.
- **Consequences:** Existing owner rows are not deleted, role flags are not rewritten, owner participation can still affect availability, and cleared/declined/no-answer remain distinct. Owner-only identities/counts stay hidden from unauthorized viewers.
- **Rejected alternatives:** Removing owner participation to fix reminders, silently relabeling all totals as non-owner totals, and claiming the pending-field correction is purely additive are rejected. Until the owner chooses D1, this ADR records a recommendation, not consent.

### ADR-Dependency-Order: Lift shared primitives, not the UI action protocol

Decision ID: ADR-TXT-SEQUENCE. Review mapping: GAP8; T1.5, F3, D6 and verification claims.

- **Decision:** Place T1.5 after the required T1 protocol baseline and before parallel T2/T3 work, with no browser-rendering dependency. Lift the pure F3 range engine and guarded storage prerequisites early enough to satisfy new text-range safety; do not force text callers through the later UI prepare/confirm/action-plan protocol.
- **Consequences:** The later App action protocol reuses those primitives and retains its own confirmation, expiry, actor binding, atomic claim, and uncertain-outcome gates. Full completion still requires T0–T7 plus T1.5 and all F7 requirement tests; an early text pass is not a substitute for actual-host/UI proof.
- **Rejected alternatives:** A second weaker text writer, completing T1.5 with only superficial range parsing, delaying all text work until rendering, or claiming that a green source test proves the live image are rejected.

D6 remains pending: recommend `{weekday,time}` as reference-UTC convenience syntax and `display_timezone` as validated presentation-only metadata, never a recurrence/DST conversion. Preserve raw timestamp validation's existing June 17 through June 24 reference-date acceptance; canonical generated shorthand maps Monday June 18 through Sunday June 24, 2018, without narrowing legacy raw inputs ([weekly validator and reference bounds](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/services/mcp/organizer.go)).

Reject conflicting shorthand/raw instants, unsupported actual-date/undated shorthand, and unannounced conversion to the current week. An IANA display-zone change must not alter stored UTC instants or the canonical reference mapping; any true local-time recurrence interpretation requires a separate explicit decision.

The reported 33/34 production-check result is user-reported evidence, not an independently rerun audit. A git SHA establishes the inspected source revision, not the live deployed image, database topology, real-host permissions, or the missing second-account production check; retain those limits in every completion claim.

### ADR-Late-Tip: Preserve newly shipped skip and documentation contracts

Decision ID: ADR-TXT-LATEST. Review mapping: FE-8, FE-9, FE-10; final reviewed tip cd089433.

The branch advanced during amendment: cfaffa10 adds `skipped:string[]`, explicit omitted-list rejection wording, and honest short-duration promotion documentation; cd089433 changes favicon metadata only ([MCP change](https://github.com/shivasymbl/muster/commit/cfaffa10), [final tip](https://github.com/shivasymbl/muster/commit/cd089433aa25eea7231047774e6235da3348c44d)). Preserve the historical 59b1a48b research and measurements, but use cd089433 as the effective field-regression baseline.

Reject literal replacement of the newly shipped skipped string array with reason objects: it would violate the amendment's additive compatibility rule. Adopt the same requested outcome through additive `skipped_details`, retain per-input indexes/basis, and account for duplicate-new addresses appearing in both old arrays.

The missing-both error/documentation agreement is now satisfied if D2 keeps validation_error; success no-op remains a proposed ergonomics change requiring approval. FE10 still conflicts with the approved rejection requirement despite now-honest documentation, so implement the shared duration rejection and update that guidance together.

The final catalog measures 12,666 bytes versus the original 12,472; neither is an approved future full-build size. Keep both observations, rebaseline only with explicit evidence/review, and never infer the production image from these commits.
