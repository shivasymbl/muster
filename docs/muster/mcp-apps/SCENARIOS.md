# Reviewer scenarios

These are end-to-end reviewer scenarios, not proof that the feature works. Keep them separate from the implementing agent if independent holdout evaluation is wanted; because this package includes them, they are not inherently secret or blind.

## Reviewer scenarios

### S1: A hidden guest is present

**Persona:** A nonowner in a blind event.

**Setup:** Use separate owner, invited member, removed member, and stranger identities; include another attendee with an email and distinctive availability.

**Journey:** Open each workflow, inspect all result fields, switch sections, request next pages, refresh after permission changes, and attempt direct helper calls.

**Expected:** No hidden identity, email, count, ranking deduction, or metadata leak; unrelated and nonexistent IDs follow the same nondisclosing failure. After denied refresh, prior private content is cleared.

**Adversarial variant:** Use duplicate display names and a name containing HTML/script markup.

**Evidence:** Sanitized wire captures plus real-host screenshots. Automated entry: `TestMCPAppsHoldoutHiddenGuest`.

### S2: Two cards disagree

**Persona:** An owner choosing a time from an older conversation.

**Setup:** Open two cards for one event; alter event duration/grid or scheduled/archive state through the existing website/legacy tool between preparations.

**Journey:** Remount the older card after the newer one, select a candidate, prepare, then change the target again before commit.

**Expected:** Server conditional target state prevents stale event overwrite; UI explains conflict and requires a fresh review. Mount order and browser timestamps do not grant validity.

**Limit:** This proves the event CAS, not atomicity across every collection.

**Evidence:** Stored before/after target state and both card receipts. Automated entry: `TestMCPAppsHoldoutStaleTwoCards`.

### S3: An answer extends beyond the visible week

**Persona:** A member editing one small part of a large answer.

**Setup:** Existing normal/if-needed marks span multiple windows; include preserved manual/calendar fields, an off-grid legacy mark, and an independently changed answer during review.

**Journey:** Open one week, change two ranges, review full preservation summary, cancel once, then commit; repeat with a conflicting target edit and with an oversized snapshot.

**Expected:** Cancel writes nothing; successful edit changes only intended editable marks; the other window and noneditable fields survive; conflicting answer is not overwritten; oversized/incomplete bases remain noneditable.

**Evidence:** Exact stored before/after arrays and receipt. Automated entry: `TestMCPAppsHoldoutWholeAnswerPreservation`.

### S4: Creation succeeds but the response disappears

**Persona:** An owner creating a group with recipients.

**Setup:** Fault-inject after the event insert, before receipt completion, and between attendee operations; disable real mail.

**Journey:** Commit, lose the response, reopen the App, and ask for the same action receipt. Attempt duplicate commit.

**Expected:** The action keeps its stable event identity and is not executed again; known partial effects and unresolved effects are reported honestly. No exactly-once-completion or delivery claim is made; uncertain mail is never retried automatically.

**Evidence:** Action record, event IDs, per-step result, and UI unknown/partial state. Automated entry: `TestMCPAppsHoldoutTimeoutCreate`.

### S5: Weekly dates on a narrow mobile screen

**Persona:** A user viewing a recurring/reference-week Muster from another timezone.

**Setup:** Separate real dated event, `dow` reference-week event, and undated/differently represented group fixtures; include DST boundary times and long names.

**Journey:** Open in actual iOS and Android, rotate, enlarge text, change theme/display timezone, decline expanded mode, and use keyboard/screen-reader alternative where supported.

**Expected:** Reference-week values are not silently mapped to this week; undated groups do not become guessed dates; repeated/gap instants are explicit; safe areas preserve controls; a list alternative works without a nested scroll trap.

**Evidence:** Client/OS versions, sanitized screenshots, exact canonical/displayed time pairs, accessibility notes. Automated semantic entry: `TestMCPAppsHoldoutWeeklyMobile`; this test does not replace native-host evidence.

### S6: No renderer or incompatible view

**Persona:** A Claude Code user, a user who denies display consent, and a user reopening an old resource.

**Setup:** Supported and legacy core versions, missing Apps capability, invalid version header, old bundle/schema, malformed result, oversized hydration response, and absent asset.

**Journey:** Call the opener, follow the text flow, request an unknown URI, and attempt an incompatible action.

**Expected:** Useful text remains; invalid headers/resources return defined errors; no wrong-version snapshot is silently interpreted; no accidental mutation occurs; old hash URIs never return different bytes.

**Evidence:** Protocol traces, Code text output, fallback screenshots, manifest hash check. Automated entry: `TestMCPAppsHoldoutFallbackVersion`.

Amendment: repeat the known-grid range-write, owner-status, finalize-readback, and rename/archive tasks on the text-only path, and inspect canonical error data plus honest scheduled/closed/skipped fields. The F7 call/size gates run independently; denying App rendering cannot quietly revert to the former counts-only/error-message-only experience.

## Failure and Recovery

Every negative path must preserve the distinction between rejected-with-no-effect and partial/unknown execution. Recovery is a fresh authorized read or receipt lookup, not an automatic repeat of a potentially committed mutation.

Verify that disconnecting, reloading, or changing themes while an operation is pending cannot submit again. Keep drafts where safe, clear sensitive data on access denial, and record unresolved outcomes without claiming rollback.

## Contract Integration Scenarios

These are additional assertions within the existing scenarios and F7 suites, not replacements for them. Run the same persisted fixture and authenticated actor through legacy text, amended text, and App-facing builders so additive compatibility is observable.

| Existing scenario | Cross-contract assertion | Required eval |
|---|---|---|
| S1 hidden guest | New counts/error valid bounds/metadata do not reveal what legacy access denies | TXT005/012/017, Snapshot, S1 |
| S2 two cards | Text ranged CAS and App prepared CAS use the same target predicates; a new App-only counter cannot falsely protect against website/legacy writers | TXT003, Answer, Finalize, S2 |
| S3 off-window answer | The legacy array default still means approved whole replacement; text merge and App edit preserve the same untouched marks; empty merge/edit differ intentionally from explicit replace clear | TXT003/009, Answer, S3 |
| S4 uncertain creation | Added skipped/event echoes do not turn a partial/unknown receipt into an unconditional success | TXT002/007/008, Group, S4 |
| S5 weekly/mobile | New display_timezone never changes old raw UTC values; shorthand resolves a unique stored reference occurrence, including legacy Sunday, or rejects ambiguity | TXT013/014, S5 |
| S6 nonrenderer | Old event keys/values for the same state still match, new fields survive text rendering, and helpers cannot execute without the required prepared state | TXT005/006/007, EFF006, S6 |
