# Owner decisions still needed

These are recommendations, not recorded approvals. The requested full-build scope is approved; the choices below must be confirmed before their dependent behavior is implemented as the selected policy or final readiness is claimed.

Record decisions in `evals/owner_decisions.json` using its template and explicit human confirmation metadata. The coding agent must not turn the recommendations in this document into a fabricated approval.

## D1: Owner participation and follow-up

**Key:** `owner_as_invitee`. **Recommendation:** `participant_no_followup`.

Keep the owner as a scheduling participant and retain stored roles, roster membership, answers, and existing all-roster status totals. Exclude the owner from actionable follow-up recipients; add explicit follow-up population/count fields and label the owner's missing answer as owner participation, not a person to invite.

This intentionally changes the legacy reminder pending recipient list only if approved; existing attendee totals keep their population and receive additive owner/actionable counts. The App, remind, get_event owner counts, and attendee view must all expose the same distinction.

Alternative: `preserve_owner_pending`, retaining current owner-as-invitee/reminder behavior with explicit labels. A policy that removes owner membership entirely is not recommended because it changes scheduling participation and requires a separate data migration.

Dependent requirements: TXT-012, TXT-015, GROUP-006. Tests: A24/A27, FE11, policy and App follow-up assertions.

## D2: Both availability lists omitted

**Key:** `omitted_lists`. **Recommendation:** `success_noop`.

Return authenticated success with `changed:false`, without altering answer data or timestamps; keep explicit empty replace as a clear. Update instructions, schema, and skill wording to match exactly.

Alternative: `validation_error`, preserving BAD_ARGUMENTS and stating “no change occurs because the request is rejected.” Either selection is acceptable if consistent; the current package does not silently approve a change.

The final tip has now documented that rejection explicitly, so this alternative is already satisfied for the missing-both case. Choosing success no-op is an intentional ergonomics change, not a claim that the updated documentation is still wrong.

Dependent requirements: TXT-009 and null contract. Tests: A21, FE9, W1/W6.

## D3: Default event-list status

**Key:** `list_status_default`. **Recommendation:** `all`.

Keep `status:"all"` and full projection when omitted, preserving old callers. An explicit needs-my-answer request may choose open/compact without changing the global default.

Alternative: `open`, which must be recorded as an intentional default behavior change, with compatibility/version tests and release notes. It is not permitted merely because it produces a shorter answer.

Dependent requirements: TXT-011 and EFF-003. Tests: A23, FE13, cursor/filter and payload tests.

## D4: Text write concurrency

**Key:** `text_write_concurrency`. **Recommendation:** `ranges_cas_legacy_replace`.

New ranged replace/merge uses the shared F3 engine and target CAS; omission of a caller fingerprint still performs server load-plus-CAS, while a supplied fingerprint enforces the caller's base. Legacy array-only calls remain unconditional whole-answer replace by default as a documented compatibility exception; supplying an explicit precondition opts into guarded execution.

Alternative: `cas_all_text_writes`, moving legacy arrays to conditional target writes too; the owner must accept conflict responses where old callers would have overwritten changed values. Keeping all text writes unconditional is not recommended and requires a rejection/defer decision for the shared-safety portion of TXT-003 rather than claiming parity.

Neither option makes response/event/attendee documents jointly atomic, and neither licenses automatic retries after an unknown effect. Existing user/host authorization still applies to the single conversational call; App prepare/commit confirmation remains required on the App path.

Dependent requirements: TXT-003, F3, T1.5 shared primitives. Tests: A15, target-CAS integration, coexistence with legacy/website writers.

## D5: Archived and finalized mutability

**Key:** `write_lifecycle`. **Recommendation:** `preserve_current_editability`.

Preserve current authorized writes on archived/finalized records and say explicitly that archive/closed output is not an immutable lock. `closed` means a stored scheduled interval exists; archive:false clears only is_archived and preserves that interval.

Alternative: `lock_archived_and_finalized`, introducing defined EVENT_ARCHIVED/EVENT_CLOSED rejections for answer/invitation writes. Before implementing this alternative, explicitly approve the reopen rule and affected operations; archive:false must not silently erase scheduled history or accidentally unlock a finalized record.

The coding agent must not invent a destructive unfinalize operation. A lifecycle lock is not an access-control repair unless the approved product policy requires it.

Dependent requirements: TXT-007/TXT-017 and legacy/UI lifecycle labels. Tests: A19/A29 and allowed/refused matrix.

## D6: Weekly shorthand clock

**Key:** `weekly_shorthand_clock`. **Recommendation:** `reference_utc`.

Interpret `{weekday:"Monday",time:"15:00"}` as the reference-clock instant `2018-06-18T15:00:00Z`, return both forms, and state that clock in the receipt/schema. The optional display_timezone changes labels only, never the canonical instant or the weekly reference mapping.

For new creation, canonical weekdays map Monday June 18 through Sunday June 24. For answering an existing reference grid, resolve the unique matching stored weekday/time instead, preserving a legacy Sunday June 17; if two reference dates match, reject ambiguity with candidates and request an explicit raw instant rather than choosing silently.

Alternative: `named_local_clock` requires a supplied IANA zone, a clearly chosen reference-date conversion rule, and DST/gap/ambiguity behavior before implementation. Do not infer these from a numeric user offset or from the word “Monday.”

Dependent requirements: TXT-013/TXT-014. Tests: A25/A26, W8, date-mode and DST fixtures.

## Separate release-review approval

The original catalog measured 12,472 bytes; the final inspected tip measures 12,666 bytes, still 16 tools. The future full-build catalog is not yet measured. A reviewer must approve the exact baseline transition if growth exceeds the 10% alarm, without waiving the 32 KiB hard ceiling; this is evidence review, not deployment approval or a seventh product policy silently assumed by the agent.

All six policy records start pending. Unrelated protocol, read-only, documentation, and baseline tests may proceed, but a full handoff implementation cannot reach “Ready for human release review” while a dependent decision is unconfirmed.
