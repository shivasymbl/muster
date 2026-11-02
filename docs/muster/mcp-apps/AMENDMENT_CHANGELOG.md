# Full-build amendment and traceability

Amendment date: 2 November 2026. Original research baseline is `59b1a48b21060946c6ee11092f6a7d27b113bef7`; final reviewed development tip is `cd089433aa25eea7231047774e6235da3348c44d`, after a late MCP receipt/documentation change and a favicon-only change. No application implementation or deployment was performed by this amendment.

This amendment retains T0–T7, F1–F6, A1–A12, S1–S6, all original security/byte/range/UI-performance gates, per-action flags, no-SSE decision, partial/unknown outcomes, and release restrictions. It adds T1.5, F7/TXT-001–017, A13–A29, EFF-001–006, explicit field/efficiency evidence, and six pending owner decisions.

## Field findings

“Satisfied” below describes current inspected code behavior, not a claim that the new feature has been implemented. Full source citations and live-versus-local distinctions are in RESEARCH's field-evidence section.

| Field item | Current disposition | Adopted solution / policy | Requirement and acceptance | Task and eval |
|---|---|---|---|---|
| FE-1 | Count receipt satisfied; echo absent | Nested authoritative stored-answer echo; keep integer count fields | TXT-001 / A13 | T1.5, TextParity/REQ_TXT_001, Field/FE1, efficiency availability |
| FE-2 | updated acknowledgement satisfied; event echo absent | Add authoritative event object, retain old keys | TXT-002 / A14 | T1.5, TXT002, FE2, rename/archive efficiency |
| FE-3 | Slot strings supported; ranges absent | Shared replace/merge range engine and conditional store | TXT-003 / A15, D4 | T1.5 shared primitives, T4 reuse; TXT003, FE3, actual CAS tests |
| FE-4 | Explicit-window validation satisfied; two read defaults absent | Reuse optionalWindow and echo chosen window | TXT-004 / A16 | T1.5, TXT004, FE4, W3 |
| FE-5 | Structured flat codes present; text loses structure/detail | Preserve flat fields, add canonical data, serialize text parity | TXT-005 / A17 | T1.5, TXT005, FE5 text-only repair |
| FE-6 | Schemas/descriptions/annotations exist; detail/effect proof partial | Parameter descriptions/formats/examples and truthful annotations | TXT-006 / A18 | T1.5, TXT006, FE6, EFF002/006 |
| FE-7 | Stored interval exists; get/list omit it | Add scheduled/closed to legacy outputs and finalize receipt | TXT-007 / A19 | T1.5, T2/T5 reuse; TXT007, FE7, finalize efficiency |
| FE-8 | Final tip now returns skipped strings; reasons/indexes absent | Preserve skipped type, add skipped_details, preserve invalid-batch rejection | TXT-008 / A20 | T1.5, T6 reuse; TXT008, FE8, ADR-TXT-LATEST |
| FE-9 | Final tip's rejection documentation agrees with runtime | Existing error option satisfied; success no-op remains proposed D2 change | TXT-009 / A21, D2 | T1.5, TXT009, FE9, W1/W6, ADR-TXT-LATEST |
| FE-10 | 20→30 promotion reproduced locally | Reject invalid duration in shared ranking, no rounding | TXT-010 / A22 and existing DATA-007 | T1.5, T2 reuse; TXT010, FE10, rank regression |
| FE-11 | Owner is a participant/invitee and can be pending | Retain participation; recommended exclude actionable owner follow-up, pending D1 | TXT-015 / A27, TXT-012 / A24 | T1.5, T2/T6; TXT015, FE11, cross-surface owner policy |
| FE-12 | Provider names found | Behavior-only mail wording; retain delivery caveat | TXT-016 / A28 | T1.5, TXT016, FE12, W1/W6 |
| FE-13 | Name/limit/cursor already present; requested filters/projection absent | Add status/needs-answer/compact; default all recommended | TXT-011 / A23, D3 | T1.5, TXT011, FE13, filter-before-page/size tests |

FE-5's broad interpretation “server has no error codes” is rejected because inspected wire responses already have them; its text-path usability requirement is adopted. FE-11 is a policy decision, not a claim that ownership checks are absent. FE-13 concerns event listing, not MCP tool discovery.

## Retained successes and missing field coverage

| Evidence | Decision and requirement | Task/eval |
|---|---|---|
| W1 instructions/three skills | Keep concise guidance; update only changed F7 semantics | T1.5, Field/W1, TXT006/009/016 |
| W2 grids/windows/owner removal | Preserve all original refusals and limits | T1.5, Field/W2, TXT003/005/017 |
| W3 ranking defaults | Already satisfied; share without regressing | T1.5, Field/W3, TXT004 |
| W4 full grids on get/list | Already satisfied; full mode preserved, compact explicit | T1.5, Field/W4, TXT007/011 |
| W5 display names/merged ranges | Already satisfied; preserve in receipts and category output | T1.5, Field/W5, TXT001/010 |
| W6 honest receipts | Already satisfied for existing fields; add evidence, never inferred success | T1.5/T4–T6, Field/W6, TXT001/002/007/008 |
| W7 timestamped changes | Already satisfied; not recast as an event log | T1.5, Field/W7 |
| W8 weekly raw reference rules | Already satisfied; shorthand additive, no narrowed raw interval | T1.5, Field/W8, TXT013/014 |
| Nonowner finalize/update/remind/remove | Present checks and local denial tests; live second account still unverified | TXT017/A29, T1.5 and T7, member/stranger matrices |
| Archived/finalized answer/add | Current source allows; choose explicit D5 policy and test | TXT017/A29, T1.5/T6, lifecycle matrix |
| archive:false | Allowed, preserves scheduled interval today; no silent reopen semantics | TXT007/017, A19/A29, D5 |
| Blind musters | Existing protection/tests partial across surfaces; complete every text-tool matrix | TXT005/012/017, A17/A24/A29, T1.5/T2/T7 |

## Package-review gaps resolved

| Gap | Resolution / rejected alternative | Requirements | Task/eval / decision |
|---|---|---|---|
| GAP1 text tools left behind | Add independent F7; reject App-only truthfulness | TXT001–017 | T1.5/F7; ADR-TXT-PARITY |
| GAP2 two write semantics | Share F3 engine/CAS for new ranges; explicit D4 legacy exception, not false parity | TXT003/F3 | T1.5 primitives→T4; ADR-TXT-RANGES |
| GAP3 strict EventRow prohibition | Permit additive fields with golden old-key contract; reject field remeaning | TXT007/012/014 | T1.5→T2/T5; ADR-TXT-EVENT |
| GAP4 duration conflict | Reject in shared path; no silent promotion | TXT010/DATA007 | FE10/A22; ADR-TXT-DURATION |
| GAP5 overlapping missing categories | Preserve missing, add disjoint conditional/unavailable names | TXT010 | TXT010/A22; ADR-TXT-CATEGORIES |
| GAP6 catalog cost/helper exposure | Short App-only labels, prepared-ID server checks, actual catalog budgets | EFF002/006, F1/F3 | T1.5/T7, TextEfficiency; ADR-TXT-CATALOG |
| GAP7 owner ambiguity | Explicit pending D1 and shared population helper | TXT012/015, GROUP006 | T1.5→T2/T6, A24/A27; ADR-TXT-OWNER |
| GAP8 sequencing | T1.5 after T1, parallel with T2/T3; no renderer dependency | F7/F3 | text-only selector plus full gates; ADR-TXT-SEQUENCE |

ADR headings/IDs in DECISIONS.md carry the full reasoning and rejected options; the table abbreviates their subjects. It does not claim owner-policy recommendations are approved.

## Additional candidate requirements and efficiency

| Candidate | Disposition | Requirement/task/eval |
|---|---|---|
| Owner-only response totals | Adopt, authorized complete counts with null when hidden | TXT012/A24; T1.5/T2; owner status one-call trace |
| Weekly weekday/time convenience | Adopt after D6 clock confirmation, preserve raw dates | TXT013/A25; T1.5/T3; W8/weekly matrix |
| Optional IANA display timezone | Adopt, presentation only with runtime tzdata validation | TXT014/A26; T1.5/T2/T3; zone/null/DST tests |
| Untested authorization/lifecycle/blind cases | Adopt complete matrix, record D5 policy | TXT017/A29; T1.5/T7 |
| Six requested task types | Adopt eight explicit cases separating unknown grid and rename/archive | EFF001; T1.5/T7; raw-call-count gate |
| Catalog size and regression alarm | Adopt 24 KiB soft/32 KiB hard, >10% review alarm, exact measured baseline | EFF002/006; final-tip 12,666 bytes, historical 12,472, future full-build null |
| Typical response/summary ~2,000 chars | Adopt for designated reference/compact cases; reject silent truncation of lossless full output | EFF003/004; true text and structured-summary counts |
| Large output ≤20,000 chars with paging | Adopt bounded complete pages/cursor continuity | EFF003; large and paginated fixture traces |
| Existing 60 calls/minute | Preserve and count actual attempts, no throttling bypass | EFF005; rate/trace assertions |
| Requested true idempotence annotations | Adopt only with no-op/dedup effect proof; reject claiming truth by metadata alone | TXT006/A18; repeated-call timestamps/mail/record tests |
| skipped invalid inputs | Adopt diagnostic detail; reject unapproved partial processing of invalid legacy batches | TXT008/A20; all-or-nothing invalid-batch test |
| Late skipped string field | Already satisfied as address reporting; reject changing its type and adopt separate details | TXT008/A20; ADR-TXT-LATEST |

## Changed files and preservation

- **README.md:** Full-build scope, T1.5 dependency order, new files/gates.
- **RESEARCH.md:** Supplied production evidence, per-FE/source disposition, retained works, local policy observations, actual catalog measurement.
- **REQUIREMENTS.md:** F7's 17 requirements/A13–A29, six F6 efficiency gates, additive/null clarification, owner-decision status.
- **UX_AND_CONTRACTS.md:** Text receipts/ranges/defaults/errors/categories/filter/weekly/timezone/null contracts; original App contracts retained.
- **IMPLEMENTATION_PLAN.md:** T1.5 implementation-ready steps, shared-primitives sequencing, additive EventRow permission and explicit T2–T7 integrations.
- **DECISIONS.md:** Eight appended gap decisions/rejected alternatives; original seven decisions retained.
- **SYSTEM_MAP.yaml:** Conversational tools, shared engine/event builder, owner-policy boundary and new consumers.
- **contracts/:** Five new additive text schemas, including the late-tip skip-string receipt compatibility contract; existing action schema gains optional display_timezone only, with App bounds and variants retained.
- **evals/:** T1.5/F7, direct-subtest enforcement, owner policy and field/efficiency evidence gates/templates, stage-aware catalog measurement; original IDs and gates retained.
- **CLAUDE_CODE_PROMPT.md:** Explicit full build and F7 instructions, shared-engine requirement, new checks and pending decisions.
- **SCENARIOS.md:** Existing S6 gains text-parity verification; all six scenarios retained.
- **VALIDATION.md:** Amendment validation and proof limitations, not a product completion claim.
- **AMENDMENT_CHANGELOG.md / OWNER_DECISIONS.md:** New traceability and unapproved decision records.

No production operations, connector publication, issue closure, or destructive migration is authorized. The strongest eventual status remains “Ready for human release review”; this amended planning package does not itself reach that product status.
