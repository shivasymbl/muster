# Implementation sequence for the building agent

Work in the current MCP development branch or an isolated feature branch based on its verified tip. Do not reset, overwrite, or transplant old review files onto newer code.

This plan is deliberately additive. The read-only UI can be accepted before mutation infrastructure is complete; every write operation stays disabled until its own tests pass.

## Dependency order

```text
T0 baseline
  -> T1 protocol/resource
  -> T1.5 conversational parity and efficiency
       -> shared range/CAS primitives reused by T4
  -> T2 authorized snapshots (parallel with T1.5 where independent)
  -> T3 read-only browser UI and packaging
  -> read-only real-host review
  -> T4 complete-answer editing and bounded action records
  -> T5 finalize
  -> T6 group mutations
  -> T7 full performance, lifecycle, accessibility, host review
```

Run relevant performance, security, and accessibility checks throughout rather than postponing all quality work until T7. The final gate requires the entire agreed scope; a successful read-only slice is not permission to claim full write support.

The amendment adds T1.5 without removing or shortening T0–T7. Text parity can ship before App rendering, but its new ranged-write engine must first pass the same target-CAS/byte/validation prerequisites later used by F3; do not make the text track depend on browser confirmation or a ready UI asset.

## Preparation and file map

Read `CLAUDE.md`, `server/routes/mcp.go`, `mcp_skills.go`, MCP service/store files, current tests, both Docker build paths, and relevant active specs. The repository uses raw Go handlers and a Vue website, so use the existing extension points rather than introducing a parallel server ([current route](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/server/routes/mcp.go), [repository guidance](https://github.com/shivasymbl/muster/blob/59b1a48b21060946c6ee11092f6a7d27b113bef7/CLAUDE.md)).

| Existing/new file | Exact responsibility |
|---|---|
| `server/routes/mcp.go` | Core version handling, UI-aware tool definitions, route delegation, result `_meta` support, GET failure accounting |
| `server/routes/mcp_apps.go` new | UI resource catalog/read handler, immutable manifest, CSP, options, asset readiness |
| `server/routes/mcp_skills.go` | Short optional visual opener instruction; retain full text workflows |
| `server/services/mcp/server.go` | Dispatch new opener and later fixed action methods |
| `server/services/mcp/app.go`, `app_dto.go` new | Authenticated snapshot builder, section states, explicit redaction, scheduled interval |
| `server/services/mcp/deps.go`, `server/db/mcp_deps.go` | Narrow typed dependency additions |
| `server/db/mcp_apps.go` new | Complete own-response query and ambiguity detection |
| `server/services/mcp/app_actions.go` new | Closed prepare/commit/receipt state machine and typed per-operation validators |
| `server/db/mcp_app_actions.go` new | Prepared records, atomic claims, guarded target writes, durable step outcomes |
| `server/main.go`, `server/.env.template` | Resolved UI asset directory and read/write flags |
| `mcp-app/` new | Isolated TypeScript browser App, schemas/state/views, unit/mock-host/performance tests |
| Root `Dockerfile`, `frontend/Dockerfile`, build orchestration | Build/copy standalone resource under existing dist; preserve SPA output |
| Go route/service tests; isolated DB fixture tests | Contract, privacy, CAS, uniqueness, partial-failure and replay assertions |
| Existing spec index | Add the feature as not done until proof exists; preserve held issue scope |

The source currently contains the earlier regression fixes; do not reimplement them or assert they still fail ([latest inspected commit](https://github.com/shivasymbl/muster/commit/59b1a48b21060946c6ee11092f6a7d27b113bef7)).

## Tasks

### Task T0: Confirm baseline and commit the approved specification

**Requirements:** Global constraints and all baseline assumptions.

1. Fetch/read the current development branch and record its SHA; retain the original `59b1a48b` research provenance and inspect any delta after the final reviewed `cd089433`.
2. Confirm tracked/untracked work before modifying anything. Preserve user changes.
3. Run the focused existing suites and the eight `TestReviewRegression` cases.
4. Confirm actual build routes, live-index assumptions, and the absence of an App implementation. Do not inspect production secrets or infer the live image from git.
5. Copy this spec/evaluation package into the repository's spec location, reconcile with its existing index, and commit the spec separately from implementation if committing is within the agent's authorized workflow.

**Verify:** `PROJECT_ROOT=/path/to/repo bash evals/T0.sh`; additionally run `go test ./services/mcp ./routes -run '^TestReviewRegression' -count=1 -v` from `server`. Record local results and mark production/host state unverified.

**Proposed commit:** `docs: specify bounded MCP Apps workflows and verification gates`.

### Task T1: Add the Apps protocol surface without enabling business writes

**Requirements:** F1, A1–A2.

1. Add explicit supported-core-version helpers used by initialize and subsequent requests. Do not store “last client version” globally.
2. Complete failed-auth GET accounting using the same failure conditions and limiter as POST; test GET-only attempts as well as mixed GET/POST.
3. Add an options object for UI asset loading and feature flags. Preserve the current `InitMcp` test injection pattern rather than broad dependency refactoring.
4. Add exact hash-URI dispatch alongside Markdown resources, MIME, CSP, and manifest validation. Missing bundle while enabled must fail readiness; when disabled, do not advertise an unusable opener.
5. Add a dedicated result-writing path that can carry authorized `_meta` without accidentally nesting the entire legacy result under it. Keep ordinary tool output unchanged.
6. Define the opener with full schema from `contracts/opener-input.schema.json`, read-only annotations, and resource metadata. Set legacy tools' Apps visibility to model-only while retaining their normal tool availability.
7. Add no app mutation tools in the read-only release. Later helpers are separately flagged and use a closed operation enum.
8. Unit-test resource bytes via an injected loader, not production build files.

**Verify:** Implement top-level `TestMCPAppsProtocol` with A1/A2 subtests, version/header isolation, GET401/405/429, resource auth, missing/unknown URI, Markdown preservation, result/meta separation, request ID retention, and legacy fallback. Run `evals/T1.sh`.

**Proposed commit:** `feat: add read-only MCP Apps resource and protocol contract`.

### Task T2: Build a truthful authorized snapshot

**Requirements:** F2, A3–A4.

1. Add typed snapshot/section DTOs matching UX_AND_CONTRACTS; strictly distinguish complete, partial, hidden, unavailable, and not requested.
2. Implement a narrow complete own-response store query using authenticated actor identity; return absent, unique, duplicate, and error distinctly. Copy before invoking any mutating privacy filter.
3. Construct output using an explicit field allowlist. Email is owner-only; hidden views expose no other-user totals; opaque row keys avoid name collisions and identity leakage.
4. Include stored finalized start/end separately from `archived`. F7 explicitly permits additive legacy `EventRow` fields with old-field contract tests; do not change existing field meanings or replace the legacy shape merely to satisfy the App.
5. Call existing shared ranking/grid logic directly, not internal HTTP. Constrain ranking duration to positive multiples and add a clear service validation error for invalid values.
6. Carry the whole own-answer base separately from the visible seven-day window. Include unsupported/off-grid layers as preserved/noneditable; never synthesize an empty editable answer from truncation.
7. Bound initial participants to 25 and candidates to the existing maximum 20; expose authorized completeness/cursors. Return creation-mode defaults without persisting a group.
8. Compute a canonical state fingerprint for conditional comparisons. It is not an auth token or proof of a transactionally consistent multi-query snapshot.

**Verify:** Implement `TestMCPAppsSnapshot`: owner/member/blind/stranger, hidden counts, duplicate names, no-answer/cleared/declined, window preservation, duplicate own records, scheduled interval, invalid duration, reference-week labels, and size boundaries. Run `evals/T2.sh`; preserve all existing ranking tests.

**Proposed commit:** `feat: provide authorized lossless snapshots for MCP views`.

### Task T3: Ship the read-only browser App

**Requirements:** F2, F6, A3–A4 and A11–A12 read-only portions.

1. Create independent `mcp-app/package.json`, exact Apps SDK `1.7.5`, lockfile, TypeScript, minimal bundler/single-file output, and fail-on-empty test scripts. Do not install the SPA's bootstrap or cookie API client.
2. Use a dedicated modern Node build stage compatible with all pinned tooling, rather than silently running it in the existing Node 18 SPA stage. Confirm package engine requirements before pinning and preserve both Docker/Compose artifact paths.
3. Emit HTML plus a content-hash manifest under `frontend/dist/mcp-app/`; configure output so it cannot erase the existing SPA dist. Route reads use the already-resolved dist path.
4. Implement a typed state reducer, strict snapshot parser, and renderers for inline summary, ranked comparisons, own answer, and group participants. Use loading/hidden/incomplete/error states before cosmetic polish.
5. Register SDK handlers before connecting; validate every result; handle input partials, cancellation, host context, display-mode refusal, and teardown. Ignore obsolete read responses.
6. Add keyboard/list alternatives, host theming, 320px layout, safe areas, UTC/timezone/reference-week labeling, and compact text fallback.
7. Keep edits and mutation controls absent or clearly unavailable. Local sorting/timezone changes must not write; no invisible background refresh loop.
8. Add `build`, `test:unit`, `test:perf`, and `test:e2e` scripts under `mcp-app`, with a mock host and deterministic fixtures. Tests must run actual assertions and fail on zero discovered tests.
9. Capture actual Claude web/Desktop read-only evidence using test data and the custom connector. Do not publish or deploy without authorization.

**Verify:** `evals/T3.sh`, relevant F1/F2 gates, bundle CSP/external-origin scan, and `evals/run.sh read-only`. Run root-image and Compose-build smoke checks with the feature both disabled and enabled; verify ordinary Go tests do not depend on a generated HTML file.

**Proposed commit:** `feat: render read-only Muster workflows as an MCP App`.

### Task T4: Add complete-answer drafts and bounded action execution

**Requirements:** F3, A5–A6.

1. Add fixed prepare/commit/get-action tools, App-only visibility, truthful write annotations, and per-operation feature gates. `prepare` creates an expiring action record but changes no scheduling state.
2. Implement actor-bound immutable payloads and action states. Commit accepts only `action_id`, atomically claims once, and never reruns an already claimed action.
3. Implement edit/clear/decline validators. Apply grid-aligned change ranges to the complete current answer, preserve noneditable/off-window marks, and show exact canonical review effects.
4. Enforce at most 100 ranges and 12,288 canonical UTF-8 argument bytes before sending and on the server; preserve the route's full raw JSON-RPC body limit of 16,384 UTF-8 bytes. Reject large plans rather than issuing several destructive replacements.
5. Add guarded response writes: match immutable response ID, actor/event identity and original editable values; `$set` only editable fields, preserving manual/calendar fields. Check matched counts.
6. For creating a first answer, audit duplicates and introduce required uniqueness only through an approved migration. Do not silently pick or delete conflicting records.
7. Persist per-effect status for separate decline/counter updates. On uncertain error, return pending/partial/outcome_unknown, not `retryable:true` as a license to replay.
8. UI draft changes invalidate prior preparation. Add review, explicit save/clear, stable receipt display, draft preservation after conflict, and reauthentication behavior.
9. Do not claim that response CAS atomically fences separately stored event-grid or membership updates. Recheck and reconcile, and document the remaining cross-record race.

**Verify:** `TestMCPAppsAnswer` must cover A5/A6 plus null/empty distinctions, unchanged-window preservation, oversized plans, new-answer duplicates, target CAS, double click, expired plan, payload tampering, revoked access, panic/timeout after claim, and uncertain partial effects. Use an isolated Mongo fixture for actual conditional-write behavior; a fake-only pass is insufficient for the write gate. Run `evals/T4.sh` with fixture-backed coverage as documented.

**Proposed commit:** `feat: add reviewed and conditionally committed availability edits`.

### Task T5: Enable guarded finalization

**Requirements:** F4, A7–A8.

1. Add `finalize_time` to the allowed action registry only after its tests pass.
2. Store exact interval and prior event fields during prepare; preview actual archive/no-mail/no-calendar consequences.
3. In commit, reauthorize owner, validate span, and atomically match event ID, owner, nondeleted state, and relevant prior grid/archive/scheduled values.
4. Return a conflict for a conditional miss; preserve the old card's selection but require a new preparation.
5. On success, read the stored interval and display the authoritative receipt. Do not label an archive without a scheduled interval as finalized.

**Verify:** `TestMCPAppsFinalize` with A7/A8, competing website/legacy writes, stale two-card flow, owner changes, invalid spans, and timeout after event update; run `evals/T5.sh` and reviewer S2.

**Proposed commit:** `feat: confirm and conditionally finalize selected Muster times`.

### Task T6: Enable group actions individually

**Requirements:** F5, A9–A10.

1. Implement create/add/remove/update/decline through fixed validators reusing current domain rules. Creation is `group`, not a general arbitrary-event factory.
2. Reserve event/target IDs in the immutable action record. Do not generate a second event on repeated commit.
3. Add canonical email normalization, duplicate audit, and unique-target protections; the read-only audit must run before any index migration. Request approval for data cleanup.
4. Add per-step outcome records for multi-document effects. Do not claim a separate action insert and target write form a transaction.
5. Removal preview states answer deletion; owner removal remains forbidden. Update exposes only currently supported fields.
6. Never automatically retry uncertain mail. Surface `email_sent` as the current server's reported value with delivery unverified; issue #26 remains open.
7. Fault-inject before and after each persistence boundary, including response lost after create. If safe semantics cannot be established, keep that operation disabled with a specific blocker while the rest remains usable.

**Verify:** `TestMCPAppsGroup`, A9/A10, actual isolated Mongo duplicate races/conditional behavior, partial creation/removal, stable ID replay, and read-only reminder behavior; run `evals/T6.sh` and S4. No real recipients or production database in tests.

**Proposed commit:** `feat: add bounded reviewed group-management actions`.

### Task T7: Complete performance and host acceptance

**Requirements:** F6, A11–A12 and all end-to-end scenarios.

1. Test small, typical, and large authorized fixtures: empty/one response; 25 people/7 days; 100 people paginated, long labels, and maximum safe own-answer payload. Keep synthetic fixture sizes explicit rather than claiming they represent measured production traffic.
2. Measure actual initial/result bytes, bundle bytes, skeleton/render/local interaction time, backend reads, and p95 bounded read time using the F6 methodology.
3. Run keyboard/screen-reader, theme change, 320px, zoom, touch/list fallback, safe-area, unknown result, denied resource, and stale-conversation tests.
4. Test app-initiated and model-initiated tool permissions separately. Record actual prompts instead of assuming all hosts handle annotations identically.
5. Capture web/Desktop, then actual iOS/Android evidence with exact client versions. Cowork is secondary; Code must remain useful without UI; API tests verify a text tool contract, not a renderer.
6. Verify disabled flags remove only the new surface, old resources fail safely, and rollback preserves existing tools. A new connector/new chat may be required to exercise the host's current catalog; do not label that protocol notification support.
7. Run all verification gates and review the actual artifacts. Only then report ready for human release review, not “deployed” or “production verified.”

**Verify:** `evals/T7.sh`, `evals/run-all.sh`, reviewer scenarios, real-host evidence, isolated DB coverage, and a clean intentional diff. Missing evidence blocks the corresponding claim.

**Proposed commit:** `test: verify MCP Apps workflows across hosts and failure modes`.

## Convergence and delivery

For a failing test, change only the behavior tied to its requirement, rerun the focused check, then rerun dependent gates. Do not weaken assertions, skip tests, add empty named tests, or replace missing host evidence with mock screenshots.

Final agent delivery must include: commit SHA; changed files; each feature's status; executed test commands and results; host evidence; known limitations; migrations requiring approval; and exact deployment steps awaiting authorization. Keep the existing open issues open unless their owners separately authorize closure.

## Rollback plan

Before any separately authorized release, record the prior image and verify that disabling App writes leaves text tools and read-only views intact. If the App resource or hydration fails, disable the new surface and retain useful conversational output; do not remove unrelated MCP capabilities or change OAuth.

Do not delete action records during rollback: pending/partial/unknown receipts may still be needed to prevent repeated effects and explain outcomes. Preserve supported old resource hashes where possible, keep additive indexes/data compatible, and record any migration that would make an old image unsafe before deployment approval.

Use existing sanitized request/tool logs and the attached acceptance evidence for diagnosis. This rollback verification adds no metrics collection platform or ongoing analytics project.

## Full-build amendment tasks

### Task T1.5: Conversational parity before the renderer

**Requirements:** F7/TXT-001–017, EFF-001–006, A13–A29. **Dependency:** T1 protocol baseline, not T3 or real-host App evidence.

**Owner gate:** Read OWNER_DECISIONS before selecting policy-specific behavior. Implement policy-independent helpers/tests first; do not convert pending recommendations into approvals.

**Evidence-first work:**

1. Add `TestMCPFieldFindings` with FE1–FE13 and W1–W8 direct subtests. Use effective baseline cd089433 and capture red for the precise changed assertion in FE1–FE10/FE12/FE13, not for features that already exist; FE5 targets text representation, FE8 targets reason/index details rather than absent skipped strings, and FE9 depends on D2 because its rejection is now documented. Keep FE11 policy-dependent and every W assertion green.
2. Add `TestMCPTextParity` with one direct `REQ_TXT_001`–`REQ_TXT_017` subtest. For permission/lifecycle/blind matrices, add nested cases beneath the relevant direct child; none may be skipped to obtain a pass.
3. Preserve fixtures and test source hashes across baseline/candidate comparison; avoid fake-store shortcuts that erase scheduled state or fail to persist blindness. State clearly which tests are route fakes and which use actual isolated Mongo.

**Shared primitives first:**

4. Add `server/services/mcp/availability_changes.go` for range expansion/state assignment and `weekly_slots.go` for shorthand normalization; put type/semantic bounds in one helper set. Reject invalid duration in shared ranking validation before rounding and update both service and low-level ranker tests.
5. Lift F3's complete own-read and target-CAS helpers early into the existing dependency/store layers. New text ranges call them directly within one authorized tool call, while App T4 later wraps them in preparation/commit/receipts; do not build a second persistence algorithm.
6. Add conditional-write tests with a real isolated Mongo fixture before range mode is enabled. Preserve D4's approved legacy exception and the existing cross-document race caveat; no replica-set migration or production data change is authorized.

**Text contracts and truthful receipts:**

7. Extend `setAvailability` with exclusive legacy-list/range parsing, default replace, explicit merge, optional expected-state handling, repeated-intent no-op semantics needed for truthful idempotence, and nested stored-answer echo. Do not replace the existing count fields with arrays.
8. Add additive EventRow fields in `dto.go`/`eventRow`, owner-authorized totals and scheduled/display-zone projection fields in `server/db/mcp_events.go`; reuse the builder in get/list/create/update/finalize. Add a golden test proving original fields and types remain unchanged.
9. Change `updateEvent`/finalize receipt construction to return authoritative stored event/interval, with matched-write checking and explicit uncertainty if readback fails. Internal database reads are allowed; the caller must not need a second MCP call.
10. Use the shared optional-window helper for responses/overlap/ranking, echo the chosen window, and preserve explicit-window validation.
11. Add parseable text errors plus canonical `data` aliases in `ToolError` and `writeToolResult`; retain existing flat code fields, `isError`, and HTTP/JSON-RPC distinctions. Add offending index/path, safe valid range, and one-retry repair tests.
12. Preserve `skipped:string[]` added at cfaffa10 and add per-input `skipped_details`, including the case where a new address appears in both added and skipped due to a duplicate input. Do not change atomic invalid-batch rejection; reconcile storage-level concurrent skips before claiming complete reasons, and prove no repeated mail/record/timestamp effects before true idempotent hints.
13. Extend list input/store query with status/needs-answer/projection. Filter before limit/cursor and index only after measured query need; do not invent a tools/list filter protocol. Keep default all/full unless D3 approves otherwise.
14. Add `displayTimezone`/equivalent optional model storage and validated conversion in create/update; include it in DB projection/readback and App DTO. Omitted fields must not blank existing data. Ensure the runtime image contains IANA data (for example the standard Go `time/tzdata` embedding) and test named-zone validation in the built container, not only on the developer machine.
15. Add approved owner pending/count semantics through a single helper used by attendees, remind, owner totals and the App; keep lifecycle/weekly policies tied to D1/D5/D6.
16. Upgrade the original parameter documentation and all truthful annotations, use concise examples, and remove mail-provider names from model-visible descriptions/skills. Retain guidance that prevents invalid grids, wrong weeks, ownership violations and false mail claims.

**Efficiency evidence:**

17. Add `TestMCPTextEfficiency/REQ_EFF_001`–`REQ_EFF_006`; run the eight reference tasks through the real route/transport against isolated fixtures. Record all calls including repairs/retries and resources reads; do not hide a business read as setup.
18. Emit current-run trace evidence, exact `tools/list` bytes, text/summary character counts, page completeness and actual outcomes. Record the final-tip 12,666-byte/16-tool catalog (and historical 12,472-byte value) separately from the future 20-tool result; do not prefill future measurements. Text-only gates use the actual enabled catalog without depending on later App helpers, while full-build gates require the complete 20-tool set.
19. Use 25-person/seven-day typical fixtures plus the existing large paginated fixture. A 2,000-character target must not drop truth or move it only into `_meta`; preserve explicit compact/full contracts and the 20,000-character page limit.

**Verify:** `evals/tasks/T1.5.sh`, `evals/acceptance/F7.sh`, `evals/run.sh text-only`; required named subtests, red/green artifacts, current-run efficiency evidence, and approved owner decisions must all be checked. The original focused Go baseline and eight earlier regressions still run.

**Proposed bounded commits:** field regression tests; shared range/weekly/rank primitives and conditional store; additive receipts/defaults/errors; filters/counts/labels/schema; efficiency fixtures/evidence. Keep the spec commit separate and never push/deploy without the applicable authorization.

### Amendment integration into T2–T7

| Original task | Additions, with original work retained | Verification |
|---|---|---|
| T2 | Reuse F7 event builder/scheduled/timezone/count semantics, complete own-read, category split, and owner helper. Do not duplicate a divergent App-only truth model. | Original Snapshot suite plus TXT-007/010/012/014/015 cross-surface cases |
| T3 | Use stored display zone as the initial label, explicit weekly clock, owner follow-up population, and shared errors. Denied/unverified renderers use improved text contracts. | Original unit/e2e/host gates plus S6 text fallbacks |
| T4 | Reuse the already-tested range engine and CAS store; retain all prepare/commit/expiry/receipt gates. Keep App empty edit no-op distinct from text empty replacement clear. | Original Answer suite plus TXT-003 coexistence/limits/null tests |
| T5 | Reuse the additive scheduled/closed builder and persisted receipt. No calendar/mail promise, no extra confirmation read. | Original Finalize suite plus TXT-007 and one-call efficiency trace |
| T6 | Reuse skipped-recipient diagnostics, owner/count helper, label fields and approved lifecycle policy. Preserve partial/unknown/multi-record evidence. | Original Group suite plus TXT-008/012/015/017, invalid-batch and replay cases |
| T7 | Add complete-catalog size, growth alarm, text-call/response budgets and token-safe summaries to all prior UI/performance/accessibility/host gates. | Original F6/host evidence plus TestMCPTextEfficiency and F7 gates |

Text-only success is not full-build completion. T0–T7, T1.5, all old and new gates, owner decisions, and real-host evidence remain required for the strongest allowed status, “Ready for human release review.”
