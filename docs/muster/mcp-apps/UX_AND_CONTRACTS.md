# User experience, wire contracts, and action sequence

Protocol field names below follow stable MCP Apps and the Claude quickstart; `muster_*` tools, `schema_version`, and `_meta.muster` are proposed application contracts, not MCP-standard fields ([stable specification](https://github.com/modelcontextprotocol/ext-apps/blob/main/specification/2026-01-26/apps.mdx), [Claude quickstart](https://claude.com/docs/connectors/building/mcp-apps/quickstart)).

## Interaction model

Claude handles the conversation and ambiguous intent. Muster's App handles inspection, comparison, constrained input, and review of an exact action; it should not contain another chat box or the entire website.

### Pick a time

Inline: event name, explicit timezone/reference-week label, requested duration, best candidate with available/conditional counts, and “Compare times.” Show “No full fit” rather than an empty success state when only partial fits exist.

Expanded: ranked candidate list beside an accessible heatmap/table; list-first on narrow screens. Each candidate shows start/end, duration, definitely available, available if needed, and people not fully available, with the precise respondent population explained. Do not add overlapping categories into a false total.

Selection is local. In the write phase, “Review this time” loads/revalidates state and shows the exact effect; “Finalize time” commits only that prepared plan. Successful receipt says “Time recorded; Muster archived. No calendar event created. No email sent.”

Do not use drag interactions as the sole selection method. Each slot and candidate needs an accessible text equivalent, and users must be able to complete the task through keyboard controls.

### Answer availability

Inline: event title, own response existence/content and invitation status separately, view timezone, and “Review availability.” A declined invitation with a stored answer should show both facts instead of making one disappear.

Expanded: current visible window with available/if-needed/unavailable choices; local draft count; explicit preserved-outside-window notice; and a review footer. Use click/tap toggles and keyboard controls first; drag-fill can be a later enhancement, not a release dependency.

Saving sends a bounded change-set, not a lossy copy of a seven-day response. The server reconstructs the complete result from the stored answer, and the review states changed ranges, preserved ranges, and what clearing means. “Clear my answer” is separate from saving an empty draft and does not claim to delete unrelated manual/calendar data.

When the full stored answer is unavailable, oversized for the safe snapshot path, or duplicated ambiguously, show the reason and “Open in Muster.” Never turn that condition into an editable empty form.

### Run a group

Inline: group name, authorized response counts/status, stored finalized time if any, and “Manage group.” Nonowners see a participant view without owner controls or hidden addresses.

Expanded: participant statuses, owner-only email detail, pending-follow-up list, and group settings. Creation uses constrained fields and a reviewed canonical recipient list; it may offer reference-week dates as an explicit new-UI default, but must also support an undated group and preserve an existing actual-date group. Freeform planning stays in chat.

Add and remove are separate actions. Removal review must say that the person's stored answer is also removed and must forbid owner removal. A follow-up list is not a sent reminder; `email_sent:true` is only the existing server flag, with delivery unverified.

### Shared states

| State | User experience | Action rule |
|---|---|---|
| Loading | Stable skeleton with event/workflow context where known | No write |
| Empty | Explain zero records only after a complete authorized read | No invented zero for hidden data |
| Hidden | “This information is not available to your account” | No derived counts |
| Incomplete | Label the affected section and offer bounded next page/refresh | No edit from incomplete base |
| Draft | Unsaved changes and discard action | No persistence |
| Prepared | Exact server-generated effect and expiry | Payload changes invalidate plan |
| Pending | “Checking the outcome” and stable action ID | Duplicate button disabled |
| Completed | Authoritative receipt plus refreshed state | No optimistic claims |
| Rejected/conflict | Explain stale/permission/validation reason | Fresh read and new preparation |
| Partial/unknown | State which effects are established and which are unresolved | No automatic mutation retry |
| Auth expired | Clear sensitive snapshot, preserve nonsensitive local intent only | Reconnect, then reload |

## Layout, accessibility, and visualization

The inline card should fit content and avoid a vertical scrolling region inside the conversation; expose at most two bottom actions and expand for dense interaction, matching Claude's design guidance ([design guide](https://claude.com/docs/connectors/building/mcp-apps/design-guidelines)).

- Use host colors, typography, theme updates, and safe-area insets. Never hard-code a dark-only dashboard.
- Use color plus text/pattern/icon labels for availability. Provide an accessible ranked-list/table alternative to the heatmap.
- Keep participant identity rows stable through opaque keys, not display-name matching.
- Label actual instants with the selected IANA timezone, but never reinterpret stored reference-week dates as today's calendar week. `dow` enforces the reference week; `group` can be reference-week, actual-date, or undated, so derive and validate an explicit `date_mode` rather than treating every group as weekly.
- Treat DST gaps/repeated times explicitly: use an instant plus offset in review and preserve the server's canonical UTC value.
- Render participant names/event text as text nodes, not injected HTML.
- Handle host refusal of fullscreen with a paginated inline flow or a user-clicked website fallback.
- Do not ship a charting framework for a slot grid. Typed DOM/CSS grid and a small state reducer are sufficient for the first release; isolate rendering from state so tests can exercise it.

These are design decisions. The building agent may propose a different dependency-light renderer only with measured bundle, lifecycle, and accessibility evidence, not by importing the SPA wholesale.

## Tool catalog

| Tool | Visibility | Purpose |
|---|---|---|
| Existing conversational tools | Model-only in the Apps visibility metadata | Existing text workflows, unchanged permissions/semantics |
| `muster_open_view` | Model and App | Read-only initial snapshot or bounded refresh |
| `muster_ui_prepare_action` | App only, write phase | Validate a closed typed operation and persist its expiring immutable preview |
| `muster_ui_commit_action` | App only, write phase | Attempt the already-prepared operation once |
| `muster_ui_get_action` | App only, write phase | Read its authorized receipt/reconciliation status |

Host enforcement of `visibility` limits the App's tool access, but the server must still authenticate and authorize every endpoint; an app-only marker is not an origin credential ([stable visibility rules](https://github.com/modelcontextprotocol/ext-apps/blob/main/specification/2026-01-26/apps.mdx)).

Gate the three action tools behind `MCP_APPS_WRITES_ENABLED=false` by default, plus an enabled-action allowlist. Never disable existing text tools as collateral damage to the read-only UI rollout.

Annotations MUST reflect persistence: prepare is `readOnlyHint:false`, `destructiveHint:false`, `idempotentHint:false` unless separately proven deduplicated; commit is `readOnlyHint:false`, conservative `destructiveHint:true`, and `idempotentHint:true` only because the same immutable action ID cannot re-execute; receipt reads are read-only. These hints are not authorization or proof that the prepared action is safe.

`muster_open_view` parameters:

```json
{
  "workflow": "pick-a-time",
  "event_id": "canonical-or-short-id",
  "window": {"start": "2030-11-04T09:00:00Z", "end": "2030-11-11T09:00:00Z"},
  "duration_minutes": 30,
  "people_cursor": null
}
```

Supported workflows are `pick-a-time`, `answer`, and `run-a-group`. `event_id` is required except for `run-a-group` creation mode; omission there returns a nonsensitive creation draft definition, not a new persisted event. A window is at most seven days for displayed availability; complete own-answer state is separately obtained through the new bounded store read.

### Tool definition

This example is illustrative; compute the real resource hash at build time and validate the full input schema in Go.

```json
{
  "name": "muster_open_view",
  "title": "Open Muster",
  "description": "Inspect a Muster event using pick-a-time, answer, or run-a-group. Read-only; opening or selecting a view changes no scheduling data.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "workflow": {"type": "string", "enum": ["pick-a-time", "answer", "run-a-group"]},
      "event_id": {"type": "string"},
      "duration_minutes": {"type": ["integer", "null"], "minimum": 1},
      "window": {
        "anyOf": [
          {"type": "null"},
          {
            "type": "object",
            "properties": {
              "start": {"type": "string", "format": "date-time"},
              "end": {"type": "string", "format": "date-time"}
            },
            "required": ["start", "end"],
            "additionalProperties": false
          }
        ]
      },
      "people_cursor": {"type": ["string", "null"]}
    },
    "required": ["workflow"],
    "additionalProperties": false
  },
  "annotations": {"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true},
  "_meta": {
    "ui": {
      "resourceUri": "ui://muster/view/BUILD_HASH.html",
      "visibility": ["model", "app"]
    }
  }
}
```

The authoritative input contract is `contracts/opener-input.schema.json`, including the conditional event requirement and bounds. Generate or test the Go tool definition against that schema; do not accept undocumented arguments.

### Resource response

```json
{
  "contents": [{
    "uri": "ui://muster/view/BUILD_HASH.html",
    "mimeType": "text/html;profile=mcp-app",
    "text": "<!doctype html><html>...bundled application...</html>",
    "_meta": {
      "ui": {
        "prefersBorder": true,
        "csp": {
          "connectDomains": [],
          "resourceDomains": [],
          "frameDomains": [],
          "baseUriDomains": []
        }
      }
    }
  }]
}
```

Bundle code/styles into HTML, omit `ui.domain` initially, and use the host bridge for data. A stable domain is optional and Claude-specific domain values have exact derivation rules; introducing one adds cross-instance/storage considerations without benefiting this server-mediated design ([Claude troubleshooting](https://claude.com/docs/connectors/building/mcp-apps/troubleshooting), [instance supersession](https://claude.com/docs/connectors/building/mcp-apps/instance-supersession.md)).

### Result envelope and view schema

```json
{
  "content": [{"type":"text","text":"Three candidate times are available. The best has four available respondents and one conditional respondent. This is a read-only view."}],
  "structuredContent": {
    "schema_version": 1,
    "workflow": "pick-a-time",
    "event_id": "event-123",
    "summary": "Three candidates; best has four available and one conditional.",
    "read_only": true
  },
  "_meta": {
    "muster": {
      "schema_version": 1,
      "snapshot_id": "read-unique-id",
      "state_fingerprint": "server-computed-state-digest",
      "action_bases": {},
      "read_at": "2030-11-04T10:00:00Z",
      "workflow": "pick-a-time",
      "event": {
        "id": "event-123",
        "name": "Planning",
        "kind": "event",
        "archived": false,
        "scheduled": null
      },
      "permissions": {
        "can_rank": true,
        "can_view_people": true,
        "can_edit_own_answer": false,
        "can_manage_group": false,
        "can_finalize": false
      },
      "sections": {
        "grid": {"status": "complete", "data": {"slot_minutes":15,"reference_week":false,"days_only":false,"dates":[]}},
        "candidates": {"status":"complete","data":[]},
        "people": {"status":"not_requested","data":null,"next_cursor":null},
        "own_answer": {"status":"not_requested","data":null}
      }
    }
  }
}
```

Example values are illustrative, not a fixture to ship. Each section status is one of `complete`, `partial`, `hidden`, `not_requested`, `unavailable`; reject unknown schema versions instead of inferring their shape. Snapshot IDs are diagnostic identifiers, and fingerprints are conditional-state comparison values, not authorization tokens.

`action_bases` maps only currently permitted and enabled operation names to their server-computed expected-state fingerprint. The overall `state_fingerprint` is diagnostic; an action must use its matching `action_bases[operation]`, not a hash computed by the browser or a different operation's base.

| Operation | Fingerprint/preview input | Atomic target boundary |
|---|---|---|
| Edit/clear own answer | Actor/event, relevant grid and permission state, exact own-record identity and raw editable fields | Own response document; related policy/grid rechecks remain cross-document |
| Decline own invitation | Actor/event and own attendee identity/declined state | Own attendee document |
| Finalize/update group | Actor/event owner, nondeleted state, affected grid/settings/archive/scheduled fields | Event document |
| Add/remove attendees | Actor/event owner plus canonical invite targets and associated response identities where affected | Per-target documents with explicit multi-record partial outcome |
| Create group | Canonical actor-bound draft captured by prepare; no prior-event fingerprint | Reserved stable event ID plus tracked related steps |

Build the fingerprint from a server-owned typed canonical representation that distinguishes absent, null, and empty fields. Store the original BSON preconditions in the prepared record; the digest does not replace those predicates, and a client cannot alter authorization by supplying a matching-looking hash.

The grid's real schema MUST include canonical date starts, duration, increment, days-only, week-start, and `date_mode: "actual" | "reference_week" | "undated" | "ambiguous"`. The aggregate must validate the stored representation; ambiguous/mixed dates are read-only with explanation, not silently remapped. Candidate rows MUST include start/end, available count, if-needed count, not-fully-available count/authorized labels, and `population: "stored_respondents"`.

Own-answer data MUST include `exists`, explicit normal/if-needed lists, decline separately, editable layer names, completeness, preserved off-window/off-grid counts where authorized, and a target fingerprint. On a successful unambiguous read, normalize legacy missing/null stored lists into explicit `[]` in the DTO while preserving the original raw representation in conditional-write preconditions; a failed/unavailable read must never be normalized into an empty editable snapshot.

Parser order: validate `_meta.muster` and its supported schema version first. If it is absent, display only the validated model-safe summary and a refresh/text/website fallback; do not treat summary fields as a complete editable view. Optional designated text-JSON parsing must validate an object and version, never blindly parse `content[0]`, arrays, scalar values, prose, or errors as snapshots.

### Null Handling Contract

| Field | Missing/null | Empty or zero | Invalid/unknown |
|---|---|---|---|
| `event_id` | Omission only for run-a-group creation; explicit null invalid | Empty string invalid | Invalid/inaccessible ID uses nondisclosing not-found |
| `window` | Server chooses a bounded valid display window, or explicit undated state | Empty object invalid | Over-seven-day or reversed bounds rejected |
| `duration_minutes` | Use documented event/grid-derived default | Zero invalid | Nonmultiple/out-of-range rejected, not rounded |
| section `data` | Required for hidden/unavailable/not-requested | `[]` means truly empty only if complete | Reject mismatched status/data |
| `scheduled` | No recorded final interval | Empty object invalid | Reject invalid start/end |
| own `exists` | Unavailable, not implicitly false | False means no record; true with empty lists means cleared | Duplicate source records disable edit |
| counts | Hidden/unknown, not zero | Zero is a real complete count | Negative rejected; do not clamp into plausibility |
| `next_cursor` | End of complete page sequence | Empty string treated as absent | Invalid/stale cursor rejected without leaking scope |
| action `changes` | Invalid for edit; never clear implicitly | Empty array is no-op | Invalid slot/range rejected |
| `email_sent` | Delivery/reporting unknown | False is a server-reported no-send flag | True still does not prove delivery |

## Browser-to-server sequence

| Step | Actor and operation | Required behavior |
|---|---|---|
| Discovery | Host → `initialize`, `tools/list` | Version/extension handling; no shared per-client globals |
| Invocation | Claude → `muster_open_view` | Server authenticates, authorizes, returns summary plus snapshot |
| Resource | Host → `resources/read` | Exact hashed resource, no account data in HTML |
| Bridge | App SDK → `ui/initialize` then initialized notification | Register callbacks before `connect()` |
| Hydration | Host → tool-input/tool-result notifications | Validate schema; partial input never drives writes |
| View change | App → host `callServerTool(muster_open_view)` | Bounded read; render returned result directly; no new model turn required |
| Draft | Browser local reducer | No RPC per slot or hover |
| Review | App → `muster_ui_prepare_action` | Server returns fixed effect preview and expiring action ID |
| Confirmation | User activates exact commit control | Host may separately request tool permission |
| Commit | App → `muster_ui_commit_action` | Reauthorize, single claim, guarded target operation |
| Receipt | App → returned receipt / `muster_ui_get_action` | Completed, rejected, partial, pending, or outcome_unknown |
| Follow-up | Optional `updateModelContext` / user-requested message | Compact committed result only, not full private grids |

SDK methods such as `callServerTool`, `updateModelContext`, `sendMessage`, `openLink`, and `requestDisplayMode` must be used according to host capability support; optional methods are not guaranteed on every host ([SDK App API](https://modelcontextprotocol.github.io/ext-apps/api/classes/app.App.html)). If using `sendMessage`, pass a content-block array according to the pinned SDK types rather than relying on inconsistent prose examples ([pinned SDK types](https://raw.githubusercontent.com/modelcontextprotocol/ext-apps/v1.7.5/src/spec.types.ts)).

Read feature support through `getHostCapabilities()` and available display modes through host context; declare the App's own supported modes and handle a denied mode request. Do not assume that a method's presence in the SDK means the host supports it.

Register result, input, cancellation, host-context, teardown, and error handling before `app.connect()`. The result callback receives the tool result directly; do not add another `.result` level. On teardown, cancel/discard in-flight reads and remove listeners; never initiate a write.

## Bounded action protocol

Allowed operation enum: `edit_own_answer`, `clear_own_answer`, `decline_own_invitation`, `finalize_time`, `create_group`, `update_group`, `add_attendees`, `remove_attendee`. Each dispatches to a fixed typed validator and store adapter; no tool name, script, arbitrary JSON patch, URL, or generic executor is accepted.

Prepare example:

```json
{
  "operation": "edit_own_answer",
  "event_id": "event-123",
  "expected_state_fingerprint": "fingerprint-from-complete-own-snapshot",
  "payload": {
    "changes": [
      {"start":"2030-11-04T15:00:00Z","end":"2030-11-04T15:30:00Z","state":"if_needed"}
    ]
  }
}
```

The server first compares the supplied base fingerprint with the freshly loaded authoritative editable state; a mismatch rejects preparation instead of silently rebasing the draft. It then expands half-open `[start,end)` grid-aligned ranges; conflicting overlapping assignments are rejected, while identical assignments may be deduplicated. Each touched mark is removed from both editable lists and added only to the chosen available/if-needed list, or neither for unavailable.

Untouched existing off-grid marks are preserved and MUST NOT be passed through a whole-answer legacy grid validator that would reject them; validate new/touched marks and final per-list bounds instead. The final prepared normal and if-needed lists must each remain within the existing 2,000-mark limit, with explicit noneditable fallback for an oversized legacy base.

Limit a preparation to 100 ranges and canonical UTF-8 tool-arguments bytes ≤12,288 for envelope headroom; the server independently enforces the existing entire JSON-RPC request-body limit of ≤16,384 UTF-8 bytes. The iframe cannot know every host-added envelope byte, so the route remains authoritative; do not use JavaScript string length as a byte count or split oversized plans into destructive replacements. Test exact 16,384/16,385 boundaries, non-ASCII labels, and 100/101 ranges.

Prepare returns an opaque action ID, expiry (proposed five minutes), target identity, canonical review summary, affected counts, side-effect disclosures, and confirmation text. It stores actor and expected raw fields server-side; it does not expose raw BSON preconditions to the browser.

Commit example:

```json
{"action_id":"opaque-prepared-action-id"}
```

The action ID is the idempotency identity. It cannot be reused with a new payload because commit accepts no payload; editing requires a new preparation. Store actor/event/action binding, expire unclaimed previews, and preserve claimed receipts for a documented reconciliation window; do not TTL-delete an executing action and then allow it to be attempted again.

State progression:

```text
prepared -> rejected/expired
prepared -> pending -> completed
                    -> rejected (known no target effect)
                    -> partial
                    -> outcome_unknown
```

Atomically claim `prepared → pending`; an already claimed action returns its existing state and never triggers another attempt. A separate action record plus target update is not an atomic transaction: if the process dies between them, return uncertainty and reconcile from target evidence rather than promising exactly-once completion.

For single-document updates, combine target preconditions and mutation in the same Mongo operation and check `MatchedCount`. For new own answers, require audited uniqueness; for multi-record group actions, persist stable IDs and per-step outcomes, never retry uncertain mail, and expose partial results truthfully.

Cross-document changes remain an explicit limitation on the existing storage model. Do not assert atomic membership/grid freshness from a response CAS; if a stronger invariant becomes a requirement, stop that action and propose coordinated writer changes separately.

## Performance and caching

Cache only immutable HTML by content hash; ship current and previous supported resource artifacts where feasible. If an old URI is unavailable, return an explicit version-unavailable response and text path; never silently serve different bytes under an old hash.

Keep sensitive snapshots in instance memory, not localStorage or cross-conversation BroadcastChannel. Coalesce duplicate reads per instance, keep draft state separate from server state, discard late responses using a request sequence, and expire/revalidate permissions on refresh.

Do local sort/filter/timezone formatting without RPC. Read the first page with the initial view; do not call `get_event`, responses, ranks, and people serially from the iframe. The aggregate can share backend reads, but it must not claim a transactional snapshot if it used independent queries.

Measured performance budgets are in F6. Before requesting more data, respect explicit completeness/cursors and the current rate limit; read retries may use bounded jitter/backoff, while mutation retries are governed by action receipts, never generic retry middleware.

## Conversational-tool amendment

This section extends the existing text tools; it does not replace the App contract above. The same authorization, half-open range engine, raw-byte/range bounds, target-CAS helpers, and truthful result builders must be shared across surfaces; App prepare/commit confirmation remains distinct from an authorized conversational tool call.

### Additive legacy event and receipt fields

`muster_get_event`, full `muster_list_my_events`, create, update's nested `event`, and appropriate post-write receipts use one event-view builder. Existing fields retain names, types, and meanings; new fields are additive, with `schema_version` optional only as a description of the additive extension, not a reason to remove old output.

```json
{
  "scheduled": {"start":"2026-11-10T15:00:00Z","end":"2026-11-10T15:30:00Z"},
  "closed": true,
  "display_timezone": "Asia/Kolkata",
  "response_totals": {
    "responded": 12,
    "invited": 3,
    "declined": 1,
    "cleared": 2,
    "pending_count": 4,
    "response_population": "authorized_roster_statuses",
    "invitation_population": "stored_attendees_including_owner",
    "pending_population": "nonowner_nondeclined_no_or_cleared_answer",
    "owner_participates": true
  }
}
```

The numbers illustrate shape, not a fixture or a claim about production. `pending_count` follows the approved D1 policy and may overlap invited/cleared rather than adding a fifth disjoint status; do not sum it into a participant total. For nonowners or hidden counts, the entire `response_totals` is null.

`scheduled:null` plus `is_archived:true` is manual archive without a recorded time; `scheduled` nonnull yields `closed:true`. Under the recommended D5 policy, archive:false does not erase `scheduled`, so `closed` is finalization state, not an undocumented immutable-write lock.

Availability receipts keep the existing integer fields `availability` and `if_needed`; do not replace either with an array. Add a separate object:

```json
{
  "event_id": "canonical-id",
  "availability": 8,
  "if_needed": 0,
  "duplicates_ignored": 0,
  "replaced": false,
  "changed": true,
  "stored_answer": {
    "start": "2026-11-10T15:00:00Z",
    "end": "2026-11-10T17:00:00Z",
    "availability_ranges": [{"start":"2026-11-10T15:00:00Z","end":"2026-11-10T17:00:00Z"}],
    "if_needed_ranges": [],
    "my_status": "responded",
    "echo_complete": true
  }
}
```

The window is the affected bounded interval, or explicit bounded windows for a multiday operation; each normal reference task must fit a complete compact echo. For exceptional large echoes, use a continuation/completeness contract and never claim full readback from a partial response. Read persisted state internally or obtain the updated document atomically; internal database reads do not add MCP calls.

Update preserves `event_id` and `updated:true`, adding `event:{...}`; finalize preserves `closed:true` and adds the stored `scheduled` interval. A failed post-write verification is partial/unknown, not a normal success echo of submitted parameters or a license to retry blindly.

### Ranged text writes

```json
{
  "event_id": "canonical-id",
  "mode": "merge",
  "ranges": [
    {"start":"2026-11-10T15:00:00Z","end":"2026-11-10T17:00:00Z","state":"available"}
  ]
}
```

`mode` defaults to replace. With ranges, replace starts from empty editable lists and constructs the full intended replacement; merge starts from the complete stored lists and preserves untouched marks. Both preserve manual/calendar layers, use `[start,end)`, reject conflicting overlaps, and remove touched marks from both editable lists before assigning the chosen state.

Keep the legacy tools' handling of unrelated unknown top-level arguments unchanged; the additive schemas describe the recognized extension rather than authorizing a global strictness change. New range/weekly objects are closed and validated, and unknown keys are never interpreted as user identity, authorization, or executable instructions.

Do not combine `ranges` with either legacy list key, even if null; reject ambiguous representations. `mode:"merge"` requires ranges; array-only mode remains the whole-answer replace interface. Explicit `ranges:[]` with replace means clear, while empty merge is no-op; the App's existing empty `changes:[]` remains no-op because that App operation is a merge/edit, not a replacement.

This is an explicit null-contract extension, not a redefinition of the existing App contract. Empty array-only availability remains clear; omitted/null both legacy arrays follow D2; `ranges:null`, unknown mode, and mixed representations are invalid.

New ranged modes share F3's conditional persistence even without a caller fingerprint: load, construct against that base, and atomically match target raw fields. `expected_state_fingerprint` is optional for one-call convenience but, when supplied, must match before preparation; a CAS miss returns CONFLICT and never silently rebases/retries. The legacy array default is separately governed by D4.

Do not split a large replace into several calls. The canonical UTF-8 tool arguments stay within 12,288 bytes, the raw JSON-RPC body within 16,384 bytes, each change set within 100 ranges, and the final lists within existing bounds; use UTF-8 bytes, not string length.

### Default read windows and compact listings

Use the same event-derived default helper for responses, unanimous overlap, and ranking. Both endpoints absent/null together request the default; one absent endpoint is invalid; return the actual `start`/`end` used, at most seven days. Undated or unusable event windows return BAD_ARGUMENTS with a safe request for explicit dates, never a guessed current week.

List input adds `status:"open"|"archived"|"all"`, `needs_my_answer`, and `projection:"full"|"compact"`, preserving name/limit/cursor. Recommended omitted defaults are all/full; `open` means not archived, while finalized state is separately exposed.

Recommended needs-my-answer predicate is an accessible, nonarchived, nonfinalized event where the caller is a relevant participant, has no response or a cleared response, and has not declined; D1 governs owner participation. Apply this predicate before paging and explicitly echo it; `status:"archived",needs_my_answer:true` therefore returns no actionable rows rather than silently overriding the request.

Compact rows include identity/share link, name, type/date mode, my_status, archive/closed state, and display timezone, with a bounded summary of the grid if needed. Full projection retains the full current grid; bind cursors to actor/filter/projection scope and do not discard remaining matches by filtering only after pagination.

Heavy diagnostic reads may add an explicitly requested compact projection/page size, keeping old full-field meanings. Record which projection the efficiency fixture uses; no hidden suppression of complete data to meet a small-character gate.

### Error parity and repair

Domain failures remain `result.isError:true`; protocol/auth failures retain their existing JSON-RPC/HTTP envelopes. For domain failures, `structuredContent` carries the compatibility object below, and the text block serializes the same object so a text-only client can branch on `data.code`:

```json
{
  "code": "WINDOW_TOO_WIDE",
  "message": "The requested window exceeds seven days.",
  "what_to_do_next": "Use the stated window or omit both endpoints for the event default.",
  "retryable": false,
  "data": {
    "code": "WINDOW_TOO_LONG",
    "message": "The requested window exceeds seven days.",
    "offending": {"path":"end","index":null,"value":"2026-11-18T17:00:00Z"},
    "valid": {"max_window_days":7,"slot_minutes":30},
    "what_to_do_next": "Use the stated window or omit both endpoints for the event default."
  }
}
```

The root legacy code is unchanged; `data.code` is the new canonical alias. Similarly map EVENT_NOT_FOUND_OR_NO_ACCESS to NOT_FOUND without revealing whether the event exists. A zero-based failing index/path is required for a multi-slot/range input, and valid start/end/increment bounds are included only after access is established.

Code/message/repair text must never contain credentials or unbounded raw objects; truncate/omit offending values safely with an explicit indication. Do not convert business validation into JSON-RPC invalid-method errors or rely on the host showing `_meta`.

Existing tool error `retryable` must not be interpreted as proof that an uncertain write did nothing. Preserve the App's partial/unknown semantics and include outcome/retry guidance; a one-retry repair acceptance applies only to known validation failures before effects.

### Skipped recipients and annotations

The final reviewed tip already returns `skipped:string[]`; preserve that type and meaning alongside `added` and the mail flag. Add `skipped_details:[{email,reason,input_index,basis}]` for per-occurrence reasons, using `basis` to distinguish existing_attendee, earlier_input, owner_identity, and invalid_input. A new address repeated within the batch can legitimately appear in both legacy added and skipped arrays; explain the skipped occurrence rather than claiming the arrays are disjoint.

On an invalid legacy batch, preserve no-side-effects rejection and place invalid per-input details in the error; do not silently send the valid subset. If a storage-level concurrent skip cannot be attributed reliably, report uncertainty rather than constructing a complete receipt solely from the pre-write check.

All 16 tools retain accurate annotations and gain precise parameter descriptions, formats and examples. Before advertising true idempotence on repeated set/update/add calls, implement and test no additional domain changes, timestamps, invitation records, or mail attempts for identical repeated intent; annotations alone are not a retry guarantee.

The three App helpers must say “App only” and stay brief. A host that ignores visibility may show them to its model, but the server still rejects a commit without a valid actor-bound, unexpired prepared action; do not advertise an arbitrary execution escape hatch.

### Weekly shorthand and presentation timezone

Under recommended D6, input `{"weekday":"Monday","time":"15:00"}` canonicalizes to `2018-06-18T15:00:00Z`; generated Monday–Sunday map to June 18–24, 2018. Existing raw timestamps accepted on June 17 remain accepted, and both shorthand and canonical forms are returned.

That generation rule applies to new creation. On an existing grid, set-availability must resolve the unique matching stored reference weekday/time, including legacy Sunday June 17; if both Sundays match, return a bounded AMBIGUOUS_WEEKDAY error with authorized candidates and require an explicit raw instant. Do not force a valid legacy Sunday onto June 24 or narrow the old accepted date interval.

Create accepts shorthand in `dates` only for dow or an explicitly reference-week group; set accepts it in slot lists only when the stored grid is eligible. Actual-date, undated, or ambiguous groups are not inferred weekly. Validate the converted instant with the same grid rules as a raw slot and reject conflicting representations.

`display_timezone` is an optional event label, validated with the server's IANA timezone database. On create, absent/null means unspecified; on update, absent preserves and null clears; on reads, unspecified is null. Use it as the App's initial presentation zone, with an explicit user override, but do not change stored UTC values or convert a weekly reference clock silently.

Update initialization/skills precisely: the event may now carry a named display timezone; numeric user offsets are not inferred as named zones. The chosen clock rule and any unsupported local-time recurrence semantics must be visible in examples.

### Amended null/empty contract

| Field | Omitted/null | Empty/false | Effect |
|---|---|---|---|
| Both legacy slot lists | D2 no-op or documented error; never clear | Explicit empty replace clears editable lists | Manual/calendar fields preserved |
| `ranges` | Omitted means legacy interface; null invalid | Empty replace clears; empty merge is no-op | App empty changes remains no-op |
| `mode` | Omitted replace; null invalid | Unknown/empty string invalid | No implicit merge |
| `expected_state_fingerprint` | Omitted allowed for internal-CAS range path; null invalid | Empty invalid | Supplied mismatch rejects |
| start/end | Both omitted/null default | Empty string or one-sided invalid | Actual window echoed |
| `scheduled` | Null means no interval | Empty object invalid | closed derives independently of archive |
| `skipped` / `skipped_details` | Preserve skipped string array; new details must not replace it | [] means no reported skips/details after complete processing | Invalid-batch detail is in error; per-occurrence indexes explain duplicates |
| `display_timezone` | Create unspecified; update null clears/omission preserves | Empty/unknown IANA name invalid | Labels only |
| `response_totals` | Null means hidden/unavailable | Numeric zero only for known complete authorized counts | Explicit populations required |
| list status/projection | Approved default all/full recommended | Empty/null invalid for new fields | Legacy name/limit/cursor null behavior retained |
