# Signed-in MCP for Muster

*   **Author:** shivasymbl
*   **Status:** Draft
*   **Date:** 2026-10-29

--------------------------------------------------------------------------------

## Overview

Muster is one app at https://muster.asymbl.app. People open a muster from a link and can answer without an account. Signed-in people see the musters they own, were invited to, or have already answered.

This design adds a second way in for an assistant. The first version is for MCP clients that can send a static `Authorization: Bearer` header. That includes Claude Desktop and Cursor when the user pastes a header. Clients that can connect only through OAuth discovery are out of this version. The assistant acts only as a signed-in user. It can list that person's musters, open one of them, and read answers under the same privacy rules as the website. The public page does not change. The website session cookie is not the assistant's credential.

The first version is read-only. Writing availability, declining, and adding attendees wait until the reads are in use. A "best times" summary waits until the API can compute overlap without naming a person whose answer is hidden.

## The Problem

An assistant cannot use Muster today. The HTTP API is built for the browser. Identity is a cookie named `session`. The server signs it and does not set `MaxAge`. `12 * time.Hour` in `server/main.go` is the CORS preflight cache, not the cookie lifetime. The server cannot revoke the cookie. An MCP client cannot hold that cookie.

Several routes that an assistant would want are open to anyone who has the link, or to a guest name with no account. Create, read-by-id, and read responses work that way on purpose, so a candidate can answer a recruiter's link. If the assistant called those same handlers, nothing in the handler would require a signed-in user. A by-id read would also let the assistant open any muster whose id it guessed. The website allows that. The assistant must not.

The signed-in home list (`GET /api/user/events`, `getEvents` in `server/routes/user.go`) loads every owned, invited, and answered muster, with no page size. `server/db/init.go` creates only the one-time-password expiry index, and it ignores index errors. `server/scripts/20250201_optimize_event_indexes/main.go` drops `responses_userId_1` and `attendees_email_1`, then creates compounds on embedded `events.responses.userId` and `events.attendees.email` plus `attendees.declined`. `getEvents` does not query those embedded fields. The live query reads the `eventResponses` and `attendees` collections. A response payload is arrays of 15-minute timestamps. A two-week panel with many people is large enough to fill an assistant's context. Privacy rules for hidden availability live inside `getResponses`. A second copy in the assistant will drift and can reveal a hidden person's times.

## Goals

*   A signed-in user can connect an assistant and later disconnect it. Revoking a token rejects the next call at once. Turning `MCP_ENABLED` off takes effect on process restart.
*   `tools/list`, `tools/call`, and `ping` carry a token that names one user. No valid token on those methods means rejection. `initialize` and `notifications/initialized` do not use a token. The user id never comes from the tool arguments.
*   That token is useless on the website. The website cookie is useless on the assistant.
*   The assistant can list the caller's musters, with a page size and an optional name filter on that list only.
*   The assistant can open one muster by id only when the caller owns it, has an attendee row (including declined), or has a response row. A missing muster and a muster that is not theirs return the same not-found result.
*   The assistant can read answers for a date window of at most 7 days, as merged time ranges, with the same visibility as the website. The responses JSON is at most 4096 bytes.
*   The public link, guest answer, and guest create flows on the website stay as they are.
*   Turning the assistant route off, and revoking tokens, returns the system to today's behavior. Musters and answers already stored stay.

## Non-Goals

*   Requiring a login to open or answer a muster in the browser.
*   Searching every muster in the database by name.
*   Connecting calendars, reading contacts, or admin analytics through the assistant.
*   Editing, archiving, deleting, or duplicating a muster through the assistant.
*   Drawing the availability grid inside the chat client. The page link is the grid.
*   Guest or anonymous assistant calls.
*   A second database, a second server process, or a second copy of the privacy rules.
*   In the first version: setting or clearing availability, declining an invite, adding attendees, or summarizing who is free.
*   Changing Microsoft sign-in. Production still has an empty Microsoft client id. That setup is a different change and does not belong in this one.

## Open Questions

None for the first version. A later "who is free" summary is a non-goal here, not an open choice inside this design.

## Critical User Journeys (CUJs)

### CUJ 1: Connect an assistant

*   **Given**: A person is signed in on the website. `MCP_ENABLED` is `true`.
*   **When**: They create an assistant token in Settings and paste it into an MCP client that can send `Authorization: Bearer`.
*   **Then**: The client can call `muster_whoami` and receive that person's `user_id`, `email`, and `name`. The plaintext token is not shown again. The website session still works. The token is rejected on `GET /api/user/events`.

### CUJ 2: List and open only their musters

*   **Given**: The person owns one muster, is an invitee on a second, and has answered a third. A fourth muster exists that they have never touched.
*   **When**: The assistant calls `muster_list_my_events`, then `muster_get_event` on each id, including the fourth.
*   **Then**: The list contains the first three, each with `my_roles`, `my_status`, and `page_url`. The fourth returns the same not-found error as an id that does not exist. The name filter only narrows this list.

### CUJ 3: Read answers without leaking a hidden person

*   **Given**: A muster has blind availability on. The caller is an invitee, not the owner. Other people have answered.
*   **When**: The assistant calls `muster_get_responses` with a window of 7 days or less.
*   **Then**: The payload contains only the caller's own merged ranges. Other people's times are absent. The JSON is at most 4096 bytes. A window longer than 7 days is rejected before the event is loaded.

### CUJ 4: Disconnect

*   **Given**: A token exists and has been used.
*   **When**: The person revokes it in Settings, or `MCP_ENABLED` is unset and the process restarts.
*   **Then**: The next tool call with that token is rejected. Existing musters and answers are unchanged. With the flag unset, `/mcp` is not mounted.

## Technical Plan (Architecture)

The assistant is a stateless MCP endpoint on the existing Go process. Stateless means the server does not keep an MCP session. It may store tokens so they can expire and be revoked.

```text
MCP client
  POST /mcp
  Authorization: Bearer muster_...
        |
        v
  MCP_ENABLED gate
        |
        v
  mcp auth middleware
  hash token, load mcpTokens row, reject if missing, expired, or revoked
  set user id from that row only
        |
        v
  JSON-RPC: initialize | tools/list | tools/call | ping
        |
        v
  tool function(userId, args)
        |
        +-- membership check (by-id tools)
        +-- db.ListEventsForUser / events.FilterResponses then mcp.MergeRanges
        |
        v
  responses JSON at most 4096 bytes; list bounded by 50 rows
```

The user id is never read from the tool arguments, the JSON-RPC body, or the website cookie. If a request has both a session cookie and a bearer token, the tool uses the token. A cookie alone is not accepted on `/mcp`.

### Token

A signed-in user creates a personal access token from Settings. The server stores only a SHA-256 hash. The plaintext is returned once. It is 32 random bytes, encoded as base64url, with the prefix `muster_`.

Each row in `mcpTokens` has: `_id` ObjectID, `userId` ObjectID, `tokenHash` 32 bytes, `label` string, `createdAt` time, `expiresAt` time, `revokedAt` time pointer. There is no scope column. The `muster_` prefix is the scope. The hash is SHA-256 of the full plaintext, including `muster_`. Compare hashes with `subtle.ConstantTimeCompare`. Expiry is `time.Now().UTC().Add(30 * 24 * time.Hour)`. There is no refresh.

A user may have at most 5 rows with `revokedAt` nil and `expiresAt` after now. A sixth create returns HTTP 409 `{error: "revoke a token first"}`. It does not revoke an older token. Two creates at the same instant may briefly leave 6 active rows. That is accepted.

Session routes, all behind the existing `AuthRequired` on `/api/user`:

*   `POST /api/user/mcp-tokens` body `{label}`. Trim the label. Empty after trim is HTTP 400 `{error: "label is required"}`. More than 40 Unicode code points is HTTP 400 `{error: "label is too long"}`. Success is HTTP 201 `{token, id, label, createdAt, expiresAt}`. `token` is the only response that contains the plaintext.
*   `GET /api/user/mcp-tokens` returns HTTP 200, an array of `{id, label, createdAt, expiresAt, revokedAt}` for that user, newest first. No plaintext and no hash.
*   `DELETE /api/user/mcp-tokens/:tokenId` sets `revokedAt` to now. Success and already-revoked are HTTP 204. A bad id, or a token owned by someone else, is HTTP 404.

Disconnect sets `revokedAt` and keeps the row, so a later lookup of that hash still fails. A token whose user was deleted fails the same way: after the hash matches, `db.GetUserById` must return a user, or the call is `NOT_SIGNED_IN`.

This is not an OAuth 2.1 authorization server. Clients that can only connect through OAuth discovery cannot use the first version. The target clients are named in the Overview.

Website routes keep using the session cookie. `AuthRequired` does not grow a bearer branch. A test sends a live `muster_` token to `GET /api/user/events` and expects 401.

### Protocol

No MCP library. `POST /mcp` is a Gin handler that parses JSON-RPC itself. The methods are `initialize`, `notifications/initialized`, `tools/list`, `tools/call`, and `ping`. The response is one JSON body. The server does not issue `Mcp-Session-Id` and ignores one if sent. `GET /mcp` returns HTTP 405 with an empty body, whether or not the flag is on.

`MCP_ENABLED` must be the string `true` or tool calls return HTTP 404 `{"code":"MCP_DISABLED","message":"MCP is off."}`. The route is still registered, before `router.NoRoute`, because `NoRoute` would otherwise return HTTP 200 and `index.html` for `POST /mcp`. Unset means tools are off. Restart picks up the flag. There is no second process.

Read the body first, at most 16384 bytes. One byte more is HTTP 413 `{"code":"TOO_LARGE","message":"Request body is over 16384 bytes.","retryable":false}` and the handler stops. Invalid JSON is HTTP 400 `{"code":"BAD_ARGUMENTS","message":"Body is not JSON.","retryable":false}`.

Then branch on `method`:

*   `notifications/initialized`: HTTP 202, empty body. No token. It does not count toward a rate limit.
*   `initialize`: HTTP 200. No token. A bearer header is ignored. Result: `protocolVersion` `2025-03-26`, `capabilities.tools.listChanged` false, `serverInfo` `{name: "muster", version: "1"}`. No other protocol version is negotiated.
*   `tools/list`, `tools/call`, `ping`: require a token. Failure is HTTP 401, JSON-RPC error, `error.data` equal to `{code: "NOT_SIGNED_IN", message: "Sign in by creating a token in Muster Settings.", what_to_do_next: "Create a token in Muster Settings and set Authorization: Bearer.", retryable: false}`. Unknown, expired, revoked, and deleted-user tokens use that same body.
*   Any other method: HTTP 200, JSON-RPC error code `-32601`.

Failed auth, before a user id exists, is limited to 30 requests per minute per client address, fixed window, in this process. That limiter runs before the hash lookup. It is not the per-user limit.

The client address is `c.ClientIP()` only after `router.SetTrustedProxies([]string{"127.0.0.1", "::1"})` in `main`. Gin otherwise trusts every proxy, and `middleware/ratelimit.go` already calls `ClientIP()` with that default, so a caller can spoof `X-Forwarded-For`. After the change, a loopback peer (Caddy on this machine) uses the forwarded client, and any other peer uses the TCP address and ignores `X-Forwarded-For`. The public 404 limiter picks up the same rule. That is intended.

A successful `tools/call` is HTTP 200. `result.content` is one `{type: "text", text: "<JSON string>"}`. `result.structuredContent` is that same JSON object. `result.isError` is false. A tool error (bad arguments, not found, window, sign-up form) is also HTTP 200, with `isError` true, `structuredContent` set to the error object, and `content[0].text` set to `message`. Auth failure stays HTTP 401, not `isError`. Both rate limits stay HTTP 429, JSON-RPC error, `error.data` set to `RATE_LIMITED` with `retryable` true. They are not `isError` results. The per-user message is `Too many tool calls.` The failed-auth message is `Too many sign-in attempts.` `what_to_do_next` for both is `Wait and retry.` Log outcome `rate_limited` for both.

`ping` result is `{}`. `tools/list` returns the four tools. Each input schema is JSON Schema. `muster_whoami` has no properties. `muster_list_my_events` has optional `name` (string), `limit` (integer), `cursor` (string). `muster_get_event` requires `event_id` (string). `muster_get_responses` requires `event_id`, `start`, and `end` (strings). An extra `user_id` argument is ignored. The user comes from the token.

Panic recovery returns HTTP 200, `isError` true, code `INTERNAL`, message `Try again.`, `retryable` true, and no panic text.

CORS on the website router stays limited to `Content-Type`. Do not add `Authorization` to that allow-list.

### Tools

| Tool | Arguments | Result |
| --- | --- | --- |
| `muster_whoami` | none | `user_id`, `email`, `name` |
| `muster_list_my_events` | `name` optional, `limit` optional, `cursor` optional | page of rows plus `next_cursor` |
| `muster_get_event` | `event_id` | one row, or not-found |
| `muster_get_responses` | `event_id`, `start`, `end` | people the caller is allowed to see, as merged ranges |

`limit` omitted means 20. A number above 50 is treated as 50. A number below 1, or a non-integer, is `BAD_ARGUMENTS`. `name` omitted means no name filter. `name` longer than 80 code points is `BAD_ARGUMENTS`. The filter is a case-insensitive substring. Escape regex metacharacters with `regexp.QuoteMeta`. Apply it in the events query below, not as a second fetch of every muster. An empty page means nothing on the caller's list matched.

`cursor` omitted means the first page. `cursor` must be a 24-character hex ObjectID. Anything else is `BAD_ARGUMENTS`. The query adds `_id` less than that id. Sort is `_id` descending. Fetch `limit + 1` rows. If the extra row exists, `next_cursor` is the last returned id and the extra row is dropped. Otherwise `next_cursor` is `""`.

Each row contains only: `event_id` (hex `_id`), `short_id` if present, `name` truncated to 200 code points, `type`, `is_archived`, `blind_availability`, `my_roles`, `my_status`, `page_url`. No description, no attendee emails, no remindee emails, no `creatorPosthogId`, no sign-up responses, no raw `models.Event`. `Remindee.TaskIds` is already `json:"-"` on the website. The leak to avoid is the rest of the event document, including remindee emails and `creatorPosthogId`.

`page_url` is `utils.GetBaseUrl()` plus `/e/` plus `event.GetId()`. `GetId` already returns the short id when set, otherwise the hex id. Do not read `BASE_URL` with `os.Getenv` alone. The website route is `/e/:eventId`.

`my_roles` is a list of `owner` and `invitee`. Both can be set. `invitee` is set when any attendee row matches the caller's email, including declined.

`my_status` is the first match in this order:

1.  A response row exists and `availability` or `ifNeeded` has length greater than 0: `responded`.
2.  A response row exists and both arrays are empty: `cleared`.
3.  An attendee row exists and `declined` is true: `declined`.
4.  An attendee row exists: `invited_no_answer`. A missing `declined` field counts as not declined.
5.  Otherwise: `no_answer`. This is the owner who was never an attendee and never answered. The brief's four values do not cover that person, so this fifth value stays.

Today `HasResponded` is set only when `event.Type == group`. The assistant must not copy that field. `my_status` is computed from the attendee row and the response row for every event type.

Sign-up forms (`isSignUpForm`) may appear in the list. `muster_get_responses` on one returns `SIGN_UP_FORM` and `page_url`, and does not return `signUpResponses`.

Deleted events (`isDeleted` true) are excluded, as in `getEvents`. Archived events are included and marked `is_archived`.

### Membership

A by-id tool loads the event with `db.GetEventByEitherId`. It then allows the call only if any of these is true:

*   `event.ownerId` is the caller.
*   an `attendees` row matches `user.Email` with exact string equality, the same comparison `getEvents` uses. `declined` true still matches. There is no case folding on this check.
*   an `eventResponses` row matches the caller's user id hex and this event.

Otherwise the tool returns `EVENT_NOT_FOUND_OR_NO_ACCESS` as an HTTP 200 tool error (`isError` true). The same body is used when `GetEventByEitherId` returns nil, including a malformed id. The message is `No muster found.` `retryable` is false. `what_to_do_next` is `Call muster_list_my_events and use an id from that list.` A declined invitee is on that list and can open the muster.

The check does not say whether the id was missing or forbidden.

### Responses and privacy

Check order for `muster_get_responses`, and stop at the first failure:

1.  Argument types and RFC3339 parse. `end` before or equal to `start` is `BAD_ARGUMENTS`.
2.  `end.Sub(start) > 7 * 24 * time.Hour` is `WINDOW_TOO_WIDE`. This runs before the event is loaded.
3.  Membership. Failure is `EVENT_NOT_FOUND_OR_NO_ACCESS`. Do this before telling the caller the muster is a sign-up form.
4.  `isSignUpForm` true is `SIGN_UP_FORM` with `page_url` in `what_to_do_next`. Do not return `signUpResponses`. A sign-up form is an event whose answers are named blocks, not availability timestamps.
5.  Load responses and filter.
6.  Merge, then apply the 4096 byte cap.

`FilterResponses(event, viewer, responses, attendees, start, end)` lives in `server/services/events`. It does not call Mongo. The caller passes the response slice and the attendee slice. `viewer` is `{UserID, GuestName, Email string}`. `Email` is already trimmed and lowercased. The website fills `UserID` from the session, or `GuestName` when there is no session, and loads the user with `db.GetUserById` when it needs `Email`. If `event.Attendees` is nil, the caller loads attendees with `db.GetAttendees` before the call. The tool passes the token user id, an empty guest name, and that user's email.

Return type is `map[string]*models.Response`, the same shape `getResponses` returns today. The function:

*   Builds the map with the same keys as `getResponsesMap` (`EventResponse.UserId`, a user hex id or a guest name).
*   Keeps a timestamp when `!t.Before(start) && !t.After(end)`. That matches today's inclusive compare.
*   Calls `stripSensitiveUserFields`, which clears `CalendarAccounts`, `CalendarOptions`, and `PrimaryAccountKey`.
*   Clears emails unless the caller is the owner and `CollectEmails` is true. Group events keep `response.User.Email` for a signed-in owner, and for an invitee whose lowercased trimmed email matches an attendee, even when `CollectEmails` is false. That match is `KeepGroupEmail(eventType, viewerEmail, isOwner, attendees)`, a pure function. It does not call `db.GetUserById` or `db.GetAttendees`. Membership and the list still compare `user.Email` with exact equality, including `declined: false` on today's website list. Those two comparisons stay different because that is what the code does now. This change does not fold them together.
*   Move `stripSensitiveUserFields` and `getResponsesMap` into this package as well. `getEvent` calls all three, not only the email helper: `getResponsesMap` near line 500, `stripSensitiveUserFields` near lines 571 and 581, and the email helper near lines 574 and 584. The second loop walks `event.SignUpResponses`. It calls the moved helpers directly. It does not call `FilterResponses`. `getResponses` calls `getResponsesMap` near line 683 and `stripSensitiveUserFields` near line 727, then `FilterResponses`. Do not change `createEvent`, `editEvent`, or `updateEventResponse`.
*   When blind availability is off, or the caller is the owner, return every response in the window, including guests.
*   When blind availability is on and the caller is not the owner, return only the caller's own response.
*   When `UserID` and `GuestName` are both empty, return an empty map.

The website handler JSON-encodes that map and does not merge. The tool calls `MergeRanges` in `server/services/mcp` after the filter. Merge is not used by the website.

Merge sorts timestamps and joins a mark to the previous range when it equals the previous mark plus the event `timeIncrement` minutes, or 15 when `timeIncrement` is nil. The range `end` is the last mark plus that increment. `manualAvailability` values whose timestamps fall in the window are flattened into one list and merged the same way. Day keys outside the window are dropped.

Tool JSON:

```json
{
  "event_id": "hex",
  "start": "RFC3339",
  "end": "RFC3339",
  "truncated": false,
  "hint": "",
  "people": [
    {
      "key": "user hex or guest name",
      "name": "display name or empty",
      "email": "present only when FilterResponses kept it",
      "available": [{"start": "RFC3339", "end": "RFC3339"}],
      "if_needed": [],
      "manual": []
    }
  ]
}
```

Sort people by `key` ascending, then move the caller's key to index 0 if present. Serialize. If the JSON is over 4096 bytes, drop from the tail until it fits or only the caller remains. If the caller row alone is still over 4096 bytes, keep that person and set `available`, `if_needed`, and `manual` to empty arrays. Set `truncated` true and `hint` to `Narrow start and end. The window must stay within 7 days.` The 7-day window, the merge, and the 4096 byte cap do different jobs. The window rejects a bad query. The merge shrinks timestamps. The cap is the hard stop.

### Errors

Tool failures are JSON objects: `code`, `message`, `what_to_do_next`, `retryable`.

| Code | retryable | When |
| --- | --- | --- |
| `NOT_SIGNED_IN` | false | missing, unknown, expired, or revoked token |
| `EVENT_NOT_FOUND_OR_NO_ACCESS` | false | missing id or caller is not a member |
| `WINDOW_TOO_WIDE` | false | span over 7 times 24 hours |
| `SIGN_UP_FORM` | false | responses requested for a sign-up form |
| `BAD_ARGUMENTS` | false | bad limit, name, cursor, or timestamps, including `end` before or equal to `start` |
| `RATE_LIMITED` | true | HTTP 429, either limiter. Not an `isError` tool result |
| `TOO_LARGE` | false | request body over 16384 bytes |
| `INTERNAL` | true | recovered panic |
| `MCP_DISABLED` | false | `MCP_ENABLED` is not `true`. This one is HTTP 404, not a tool result |

`NOT_SIGNED_IN` message is `Sign in by creating a token in Muster Settings.` It must not distinguish unknown, expired, and revoked.

### Load, limits, and logs

Indexes are created only by `server/scripts/20261029_mcp_indexes/main.go`, not by `db.Init`. `db.Init` runs on every boot and from tests and older scripts. Building indexes there would run on production at the next deploy, and a second create under another name panics the process. Today's `Init` ignores the OTP index error. Leave that behavior. The new script uses these names and keys:

*   `eventResponses_userId_eventId` on `eventResponses`: `userId` 1, `eventId` 1
*   `attendees_email_declined` on `attendees`: `email` 1, `declined` 1
*   `events_ownerId` on `events`: `ownerId` 1
*   `mcpTokens_tokenHash` on `mcpTokens`: `tokenHash` 1, unique
*   `mcpTokens_userId` on `mcpTokens`: `userId` 1

Creating an index that already has that name is success. Do not drop the 2025-02-01 indexes. Queries are correct without the new indexes. They scan. Production must run the script before `MCP_ENABLED=true`. This design does not run it.

`ListEventsForUser(user, name, limit, cursor) (rows []ListedEvent, nextCursor string)` does not call `getEvents`. Steps:

1.  Load response ids: `eventResponses` where `userId` is the hex id, project `eventId` only.
2.  Load attendee ids: `attendees` where `email` equals `user.Email` exactly, including `declined` true. This is wider than `getEvents`, which requires `declined: false`.
3.  Find `events` with `_id` in that union or `ownerId` equal to the user, `isDeleted` not true, optional `_id` less than the cursor, optional name regex, sort `_id` -1, limit `limit+1`. Project `_id`, `shortId`, `name`, `type`, `isArchived`, `blindAvailabilityEnabled`, `isSignUpForm`, `ownerId`.
4.  For those page ids only, `ResponseFlags(userIdHex, ids)` returns whether a response row exists and whether `availability` or `ifNeeded` is non-empty, using `$size`. It does not return the timestamp arrays.

Rate limit for a known user is a fixed window of 60 calls per 60 seconds per user id, in memory, in this process. It counts `tools/list`, `tools/call`, and `ping`. It does not count `initialize` or `notifications/initialized`. Check body size, then parse, then auth, then this limit. On each check, delete map entries whose window ended more than 60 seconds ago. Muster production is one server. A second instance would not share the counter. That is accepted. `LimitEventLookupMisses` stays on the public event routes and is not used here.

One log line through `logger.StdOut`: `mcp request_id=%s tool=%s user_id=%s event_id=%s outcome=%s latency_ms=%d`. `request_id` is 16 hex chars from `crypto/rand`. `event_id` is `-` when the call has none. `user_id` is `-` when auth failed. `outcome` is one of `ok`, `not_signed_in`, `not_found`, `bad_arguments`, `window`, `sign_up`, `rate_limited`, `too_large`, `internal`, `disabled`. No arguments, no body, no email, no availability, no token.

### Website surface

Settings gains a section that lists token labels, created time, and expiry, and can create or revoke. The create response shows the plaintext once. The frontend calls the new session routes. No other page changes.

## Alternatives Considered

### Alternative A: Reuse the session cookie

*   **Pros**: No new credential.
*   **Cons**: An MCP client cannot store or revoke the cookie. The cookie is valid for the whole website. The cookie store cannot be revoked server-side.
*   **Reason for Rejection**: The assistant and the website must not share a credential.

### Alternative B: Call `getEvents`, `getEvent`, and `getResponses` from the tool

*   **Pros**: Less new code.
*   **Cons**: Those handlers read the session, allow anonymous and guest callers, and `getEvent` returns any id. `getEvents` returns full event documents, including remindee emails and `creatorPosthogId`.
*   **Reason for Rejection**: "Signed-in only" would depend on every handler. Tools call functions that take `userId`.

### Alternative C: Full OAuth 2.1 authorization server

*   **Pros**: Clients that require OAuth discovery can connect without a pasted token.
*   **Cons**: Muster has no authorization server. Dynamic client registration, consent, and refresh are a second product. The first users of this feature can paste a bearer token.
*   **Reason for Rejection for this version**: The pasted token ships the read tools. OAuth discovery is a later version if a client cannot paste a token. This version fails for those clients on purpose.

### Alternative D: A Node or Python MCP process

*   **Pros**: The TypeScript MCP SDK is the usual example.
*   **Cons**: A second process must share Mongo, auth, and privacy with the Go server. The brief rejected a second process.
*   **Reason for Rejection**: Mount `/mcp` on the existing Gin server.

### Alternative E: `muster_summarize_event` in this version

*   **Pros**: Assistants ask "when can everyone meet" first.
*   **Cons**: With two people, a count of who is free names the other person when blind availability is on. The website has no shared overlap function.
*   **Reason for Rejection**: Cut until that function exists and cannot identify a hidden person.

### Alternative F: `github.com/modelcontextprotocol/go-sdk`

*   **Pros**: The SDK tracks protocol changes.
*   **Cons**: This server has four methods and one JSON body. The auth split (`initialize` open, tools authenticated) would have to be threaded through the SDK's HTTP handler, and the module is a new dependency. [concepts_brief.md](/code/muster/concepts_brief.md) already rejected a second process.
*   **Reason for Rejection**: The Protocol section specifies the JSON-RPC messages. The handler is Gin code. No new module.

## Non-Functional Requirements

### Security and privacy

*   Tokens are stored as SHA-256 hashes. Logs and list APIs never return the hash or the plaintext.
*   The token prefix `muster_` is checked before the hash lookup. There is no scope column.
*   Membership failure and a missing event are indistinguishable.
*   Blind availability uses the same function as the website. The tool has no guest override.
*   Adding attendees is not in this version, so this version cannot send invite mail.

### Performance

*   List page size defaults to 20 and cannot exceed 50.
*   Response JSON is capped at 4096 bytes.
*   Response windows cannot exceed 7 days.
*   Indexes above are part of the same change as the list tool. Do not run them against production from a design session. An operator runs the dated script.

### Rollout and rollback

*   Default is off. `MCP_ENABLED` unset means `POST /mcp` returns HTTP 404 `MCP_DISABLED`, not the website HTML.
*   Rollback is unsetting `MCP_ENABLED` and restarting. Revoke tokens from Settings if the route stays on.
*   Stored musters and answers are not migrated and are not deleted.
*   Index creation is additive. Rollback does not drop indexes.

--------------------------------------------------------------------------------

### Additional Resources

*   [concepts_brief.md](/code/muster/concepts_brief.md)
*   `server/routes/user.go` `getEvents`
*   `server/routes/events.go` `getResponses`, `shouldKeepGroupResponseUserEmails`
*   `server/db/init.go`
*   `server/scripts/20250201_optimize_event_indexes/main.go`
*   `server/middleware/auth.go`, `server/middleware/ratelimit.go`
*   `server/main.go` route mounting and CORS

### Future Improvements

*   **OAuth discovery**: Add it when a target client cannot paste a bearer token.
*   **Shared rate-limit store**: Add it when more than one server process is running.
*   **Writes**: Availability replace, decline, and owner add-attendees, each as its own operation, after the reads are in use.
*   **Summary**: One API function for overlap that refuses to identify a hidden person, then a tool that only calls it.

--------------------------------------------------------------------------------

## Detailed Implementation Plan

No production code is part of this design pass. The steps below are the implementation that a later change must follow. Do not refactor `createEvent`, `editEvent`, or `updateEventResponse` in that change.

### Phase 1: Token storage, route off by default

Leaves the server buildable. `POST /mcp` returns HTTP 404 until the flag is on. Website behavior is unchanged. Token tests use a fake collection interface, not a live Mongo, because `db.Init` connects to a real server and `db_test.go` already does that.

#### 1. `server/models/mcp_token.go` (Create)

*   **Type / Scope**: Go struct.
*   **Proposed Changes**: `McpToken` with the fields in the Technical Plan. BSON names match the field list. No JSON tag that would emit `tokenHash` from a handler by accident. List responses use a separate `McpTokenPublic` struct with `id`, `label`, `createdAt`, `expiresAt`, `revokedAt` only.

#### 2. `server/db/init.go` (Modify)

*   **Type / Scope**: Mongo init.
*   **Proposed Changes**: Add `McpTokensCollection` on `mcpTokens`. Do not create indexes here. Do not change the OTP index error handling.

#### 3. `server/scripts/20261029_mcp_indexes/main.go` (Create)

*   **Type / Scope**: One-off script, same pattern as `server/scripts/20250201_optimize_event_indexes/main.go`. Not imported by the server.
*   **Proposed Changes**: Create only the five named indexes in the Load section. If the name already exists, continue. An operator runs this against production. This design does not run it. The script may call `db.Init` for the connection. That must not also create these indexes.

#### 4. `server/db/mcp_tokens.go` (Create)

*   **Type / Scope**: Mongo accessor. The only package that writes `mcpTokens`.
*   **Proposed Changes**: `CreateMcpToken`, `ListMcpTokens`, `RevokeMcpToken`, `GetActiveMcpTokenByHash` with the HTTP and hash rules in the Token section. A sixth active token returns an error the route maps to HTTP 409. Lookup of expired or revoked returns nil. Hash the full plaintext including `muster_`.

#### 5. `server/db/mcp_tokens_test.go` (Create)

*   **Type / Scope**: Go test.
*   **Proposed Changes**: Against the fake collection: a sixth active create fails and does not revoke the oldest. Revoked and expired hashes return nil. The stored hash is not the plaintext. Compare uses the full `muster_` string.

#### 6. `server/routes/user.go` (Modify)

*   **Type / Scope**: Session routes already behind `AuthRequired`.
*   **Proposed Changes**: `InitUser` takes an `mcp.TokenStore`. `POST /user/mcp-tokens` body `{label}`. `GET /user/mcp-tokens`. `DELETE /user/mcp-tokens/:tokenId`. User id comes from `utils.GetAuthUser`. A delete for a token owned by someone else returns 404. Status codes are the ones in the Token section. Swag comments on each handler.

#### 7. `server/.env.template` and `DEPLOYMENT.md` (Modify)

*   **Type / Scope**: Config docs.
*   **Proposed Changes**: Document `MCP_ENABLED`. Unset means `POST /mcp` returns 404. The index script is an operator step. No token and no server secret go in these files. Tokens are random and are not signed.

### Phase 2: Membership, list, privacy function

Still no `/mcp` mount. Website `getResponses` calls the extracted function. A characterization test locks today's JSON for the blind and email cases.

#### 8. `server/services/events/privacy.go` (Create)

*   **Type / Scope**: Privacy function.
*   **Proposed Changes**: Implement `FilterResponses` and `KeepGroupEmail` as in Responses and privacy. Move `stripSensitiveUserFields` and `getResponsesMap` here. No function in this file calls Mongo. Do not merge ranges here.

#### 9. `server/services/events/privacy_test.go` (Create)

*   **Type / Scope**: Unit test.
*   **Proposed Changes**: Pass attendees and a lowercased email in the viewer. Cases: blind off returns others; blind on owner returns others; blind on non-owner returns only self; guest name returns only that guest; empty viewer returns an empty map; group invitee keeps `User.Email` when `CollectEmails` is false; non-group strips that email. No Mongo.

#### 10. `server/routes/events.go` (Modify)

*   **Type / Scope**: HTTP handler.
*   **Proposed Changes**: `getResponses` loads the event, responses, and attendees, builds `viewer`, and calls `FilterResponses`. It still returns the map. `getEvent` calls the moved helpers at the five sites listed in Responses and privacy: `getResponsesMap` near line 500, `stripSensitiveUserFields` near lines 571 and 581, and `KeepGroupEmail` near lines 574 and 584. Lines 581 and 584 are the sign-up-response loop. It does not call `FilterResponses`. Neither handler grows an auth requirement.

#### 11. `server/db/events.go` (Modify)

*   **Type / Scope**: Query.
*   **Proposed Changes**: `ListEventsForUser` and `ResponseFlags` as specified in Load, limits, and logs. Include declined attendees. Do not return timestamp arrays from `ResponseFlags`.

#### 12. `server/services/mcp/status.go` (Create)

*   **Type / Scope**: Status mapping.
*   **Proposed Changes**: `MyRoles(owner bool, attendee bool) []string` and `MyStatus(flags ResponseFlags, declined *bool) string` using the five-step order in the Tools section. Do not read `Event.HasResponded`. `declined == nil` is not declined.

### Phase 3: MCP route

#### 13. `server/go.mod` (no change)

*   **Type / Scope**: Module.
*   **Proposed Changes**: Do not add a dependency. JSON-RPC is written in `server/routes/mcp.go`.

#### 14. `server/services/mcp/server.go` (Create)

*   **Type / Scope**: Tool functions.
*   **Proposed Changes**: Four functions. Each receives `mcp.Deps` and the `*models.User` from the token lookup. Ignore a `user_id` argument. `muster_get_responses` loads attendees through `Deps.Events`, calls `FilterResponses`, then `MergeRanges`, then the 4096 byte rule. Check order is the numbered list in Responses and privacy.

#### 15. `server/services/mcp/dto.go` (Create)

*   **Type / Scope**: Response types.
*   **Proposed Changes**: The row struct and the error struct from the Technical Plan. No embed of `models.Event`, `models.Response`, or `models.User`.

#### 16. `server/middleware/mcp_auth.go` (Create)

*   **Type / Scope**: Gin middleware.
*   **Proposed Changes**: Used only after the method is known to be `tools/list`, `tools/call`, or `ping`. If the failed-auth window for `c.ClientIP()` is already spent, return HTTP 429 before hashing. Otherwise require `Bearer` and the `muster_` prefix. Hash the full token. `Deps.Tokens.GetActive`, then `Deps.Users.GetByID`. On any failure, count it against that window and return HTTP 401 with the `NOT_SIGNED_IN` body. Set the user on the request context. Do not read the session. If both a cookie and a token are present, the user is the token user.

#### 17. `server/middleware/mcp_ratelimit.go` (Create)

*   **Type / Scope**: In-process limiter.
*   **Proposed Changes**: Fixed window, 60 calls per 60 seconds per user id, for the three authenticated methods, checked after auth succeeds. Failed-auth window is 30 per 60 seconds per `c.ClientIP()` after trusted proxies are set. Both return HTTP 429 with the JSON-RPC body in the Protocol section. Body size is checked before either limiter.

#### 18. `server/routes/mcp.go` (Create)

*   **Type / Scope**: HTTP mount.
*   **Proposed Changes**: `InitMcp(router, enabled bool, deps mcp.Deps)` registers `POST /mcp` and `GET /mcp`. `deps` is defined in `server/services/mcp/deps.go`: `Tokens TokenStore`, `Events EventStore`, `Users UserStore`, `Now func() time.Time`. `TokenStore` has `Create`, `List`, `Revoke`, and `GetActive`. `EventStore` has `GetByEitherID`, `Responses`, `Attendees`, `ListForUser`, and `ResponseFlags`. `UserStore` has `GetByID`. `GET` is HTTP 405. `POST` follows the Protocol section. Log one line as specified. Recover panics to `INTERNAL`.

#### 19. `server/main.go` (Modify)

*   **Type / Scope**: Process entry.
*   **Proposed Changes**: Call `router.SetTrustedProxies([]string{"127.0.0.1", "::1"})` before any route. Call `routes.InitMcp(router, os.Getenv("MCP_ENABLED") == "true", db.NewMcpDeps())` before `router.NoRoute`. `db.NewMcpDeps` returns the Mongo implementations of the three stores. `routes.InitUser` gains a `mcp.TokenStore` argument and the three token handlers use it. Do not change CORS `AllowHeaders`.

#### 20. `server/routes/mcp_test.go` (Create)

*   **Type / Scope**: `httptest` against a Gin engine from `InitMcp`. Stores and event lookups are interfaces with fakes. No Mongo.
*   **Proposed Changes**: Required cases:
    *   `enabled` false: `POST /mcp` returns HTTP 404 `MCP_DISABLED`, not HTML.
    *   `GET /mcp` returns HTTP 405.
    *   `initialize` with a bad bearer token still returns HTTP 200 and the protocol version `2025-03-26`.
    *   `notifications/initialized` returns HTTP 202.
    *   `tools/call` without a token returns HTTP 401 `NOT_SIGNED_IN` and `retryable` false. Revoked, expired, and unknown tokens return that same body.
    *   A revoked token and an expired token return the same error as an unknown token.
    *   A valid token on `GET /api/user/events` returns 401.
    *   A session cookie for user A and a token for user B on `tools/call` acts as user B.
    *   `muster_get_event` for a missing id and for another user's id returns the same `EVENT_NOT_FOUND_OR_NO_ACCESS` body.
    *   A member can get the event.
    *   List `limit` 1000 is treated as 50.
    *   Name filter does not return a muster the caller does not belong to, even if the name matches.
    *   Responses window of 8 days returns `WINDOW_TOO_WIDE` and does not query responses.
    *   Blind non-owner payload has no other person's ranges.
    *   A payload that would exceed 4096 bytes sets `truncated` true and stays at or under 4096 bytes.
    *   The log line for that call does not contain the token, an email, or a timestamp from availability.
    *   The 61st authenticated call in a window returns HTTP 429 `RATE_LIMITED` with `retryable` true.
    *   The 31st failed auth from one client address returns HTTP 429 before a hash lookup. A spoofed `X-Forwarded-For` from a non-loopback peer does not change the bucket.

#### 21. `frontend/src/components/settings/McpTokens.vue` (Create)

*   **Type / Scope**: Vue 2 component.
*   **Proposed Changes**: List, create, show-once, revoke. Uses the existing API client in `frontend/src/utils/fetch_utils.js`. No other settings behavior changes.

#### 22. `frontend/src/views/Settings.vue` (Modify)

*   **Type / Scope**: Page.
*   **Proposed Changes**: Render `McpTokens` inside the existing settings layout.

#### 23. Frontend test (do not create)

*   **Type / Scope**: Vitest.
*   **Proposed Changes**: Do not add a Vue component test. `vitest.config.mjs` uses the node environment and the repo has no Vue test renderer. Do not add that dependency for this feature.

#### 24. `server/docs` via `swag init` (Modify)

*   **Type / Scope**: Generated swagger.
*   **Proposed Changes**: Regenerated from the new session-route comments. `/mcp` is not documented as a REST route in swagger.

### Verification Plan

*   **Compilation/Build**: `go test ./...` in `server/`. `npm run test:unit` in `frontend/`.
*   **Unit Tests**: The new Go tests. Run `go test ./routes -count=1` and `go test ./services/events -count=1` while iterating. There is no new frontend unit test.
*   **Static Analysis & Linting**: `go test` covers compile. Do not add a new linter.
*   **Test Coverage Requirement**: Every case listed under `server/routes/mcp_test.go` must exist and pass. No percentage gate.
*   **E2E/Integration**: Not required for this version. Manual check, after the operator enables the flag in a non-production environment: create a token, call `muster_whoami`, revoke, call again, expect `NOT_SIGNED_IN`.
*   **Production**: Do not set `MCP_ENABLED` on the production server as part of implementation. Do not run the index script against production until the operator says so.

--------------------------------------------------------------------------------

*Drafted with EGM*

