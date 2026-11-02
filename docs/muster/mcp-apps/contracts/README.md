# Proposed App input contracts

These three Draft 2020-12 schemas specify proposed App tools, not currently implemented endpoints or a full snapshot schema. They accompany `UX_AND_CONTRACTS.md`; no schema validation result grants authorization or proves the server implements the contract.

| Schema | Proposed tool |
|---|---|
| `opener-input.schema.json` | `muster_open_view` |
| `action-prepare.schema.json` | `muster_ui_prepare_action` |
| `action-commit.schema.json` | `muster_ui_commit_action` |

## Closed inputs and nulls

Every original App input/payload object is closed to unknown properties. Prepare selects exactly one of eight operation branches; all existing-event operations require `event_id` and `expected_state_fingerprint`, while create forbids both and binds the authenticated actor and canonical draft to the new action record instead.

Only `run-a-group` may omit the opener event ID to inspect a creation draft without persistence; explicit null is invalid for every workflow. Window and cursor may be null; missing/null duration uses the documented grid-derived default, and an empty cursor is treated as absent.

Action payloads never accept null, arbitrary patches, tool names, URLs, owners, or replacement full-answer lists. Empty `changes` means no-op, not clear; `clear_own_answer` and `decline_own_invitation` require an empty object payload, and commit accepts only `action_id`.

## Organizer mapping and intentional bounds

The inspected organizer accepts `archive`, not `is_archived`, as an update input; `is_archived` is a read DTO field and `isArchived` is a storage field. The proposed narrow `update_group` branch therefore exposes `name`, `dates`, `duration_hours`, and `archive`, plus F7's additive `display_timezone`; existing support for `blind` is deliberately not exposed by this subset.

Create fixes type to `group` and retains the existing create fields and slot options, with a stricter maximum of 20 raw email entries before canonicalization. Existing create accepts at most 31 dates; the shared existing update parser permits up to 2,000, additionally constrained here by the separate canonical-arguments and raw-body byte limits below.

`duration_hours` is the length of each day/window, not meeting length. GROUP dates may be omitted or empty; the legacy reference-week requirement applies to DOW, not automatically to GROUP, so these schemas do not invent a weekly recurrence or reinterpret reference dates as the current week.

## Required server checks

- **Authorization:** Authenticate every call, check current actor/event ownership or membership and action allowlists, protect hidden fields, reject removal of the owner, and bind receipt access to the actor. Fingerprints and action IDs are comparison/lookup values, never bearer credentials.
- **Time/grid semantics:** Parse RFC3339 with offsets, reject reversed/zero spans, enforce opener windows at most seven elapsed days, and validate slot alignment, grid membership, duration and expanded-mark bounds. `format` may be annotation-only in a validator, so Go parsing remains mandatory.
- **Complete-answer safety:** Validate the complete authorized own-answer base and fingerprint, reject duplicate/ambiguous source records, preserve off-window and noneditable manual/calendar layers, and reject conflicting overlapping ranges rather than rely on order-dependent implicit overwrites.
- **Input and canonical review:** Enforce canonical tool arguments at most 12,288 UTF-8 bytes and independently reject a raw full JSON-RPC body above 16,384 bytes; do not conflate the tool arguments with the envelope or bypass the raw-body limit through canonicalization. Canonicalize dates and email addresses with current domain logic, expose deduplication and skipped recipients, and show every default/side effect before confirmation; empty update dates is an explicit replacement, not an omitted field.
- **Execution safety:** Persist immutable actor/target/payload/state binding and expiry, reserve stable create identity, claim one execution attempt, use actual target-field CAS and inspect `MatchedCount`, and reconcile pending/partial/unknown receipts. Do not promise atomic multi-record completion or exactly-once mail; never automatically resend uncertain mail.

Schema validation cannot compare two timestamps, verify a cursor/fingerprint against live state, enforce raw request bytes, ensure database uniqueness, or establish human confirmation. Implement and test these semantic checks in the server; keep unsafe write operations disabled until their actual acceptance and fault-injection suites pass.

## Validation scope

JSON parsing and local-reference/object-closure/operation-branch selfchecks are possible without installing dependencies. A Draft 2020-12 validator such as Ajv 2020 with explicit format support should also compile these schemas during implementation; if unavailable, report that limitation rather than claiming full schema-engine validation.

## Proposed text-contract additions

Five additional Draft 2020-12 schemas cover the text-tool amendment below, including the final-tip skip-string compatibility correction. The existing App action schema only gains optional presentation-timezone fields; its branches, required fields, limits and existing semantics remain intact. These additions are proposed contracts, not evidence that current endpoints implement them.

| Schema | Scope |
|---|---|
| `text-availability.input.schema.json` | Extended availability input preserving unrelated legacy top-level keys: nullable lists, RFC3339 or weekday/time items, mutually exclusive ranges, replace/merge, optional fingerprint. New range/weekly objects remain closed. |
| `text-event-additions.schema.json` | Output only: required scheduled/closed/display-timezone/totals additions, optional schema version, existing event fields allowed. |
| `text-tool-error.schema.json` | Output only: retained flat error fields plus canonical repair data; legacy/future extra output fields allowed. |
| `text-list-events.input.schema.json` | Event-list input retaining name/limit/cursor and historical unknown-key tolerance, plus status, needs-my-answer and explicit projection. |
| `text-invite-receipt.schema.json` | Successful output retains shipped skipped address strings and adds separate reason/index/basis objects; invalid batches remain atomic errors. |

The text availability schema accepts omitted or both-null lists structurally so it does not preempt the pending no-op/error decision. Ranges forbid either legacy list key entirely, including null; merge requires ranges, and lists have at most 2,000 entries each while ranges have at most 100 entries before expansion. New weekly creation generates Monday 2018-06-18 through Sunday 2018-06-24; answering resolves the unique matching stored weekday/time, including legacy June 17, and rejects ambiguous duplicates. DOW/reference-group eligibility and the proposed reference-UTC clock require server validation and owner approval respectively.

Defaults are metadata recommendations, not approved owner decisions or an instruction to enable validator mutation. Authorization, archived/finalized mutation policy, pending-owner counts, no-op behavior, range assignment/expansion, valid IANA identifiers, freshness and byte limits still require the approved prose contracts and server checks; full/compact and status defaults must not be silently changed by schema tooling.

The error schema validates the structured payload, not the outer JSON-RPC/tool-result envelope or the JSON string inside `content[].text`. Implementation must retain `isError:true`, emit equivalent parseable repair information in text, preserve flat compatibility codes, sanitize bounded offending scalar values and disclose valid bounds only to authorized callers; schema acceptance cannot prove those properties.
