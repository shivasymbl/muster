# MCP writes

The website stays public by link. These tools use the same signed-in bearer as the read tools. They do not call the website handlers, because those handlers accept a guest and a session cookie.

## Tools

`muster_set_availability` replaces the caller's `availability` and `if_needed`. Pass at least one list. An empty list clears that list. Omitting both, or passing null, is rejected and the stored answer stays unchanged. Omitting one list clears that list. It does not merge with the previous answer. Calendar settings already stored on the answer are left in place. A new answer clears a previous decline. Sign-up forms are refused. When the muster has stored dates, a time outside those windows or off the slot length is rejected. A weekly muster stores those dates on the reference week 2018-06-17 through 2018-06-24, so a later week is rejected. A window that crosses midnight still accepts the later slots. A time in both lists is rejected. Duplicate times are stored once.

`muster_create_event` creates a muster for the signed-in user. `muster_list_attendees` lists invite status. `muster_find_best_times` ranks partial fits. `muster_finalize` stores a chosen time and closes the muster. `muster_update_event` renames, replaces dates, or archives. `muster_remind` lists non-responders and does not send mail. `muster_remove_attendee` removes one invitee and that person's answer. `muster_list_changes` returns answers and invites recorded after a timestamp.

`muster_get_event` returns that grid: `dates`, `duration_hours`, `slot_minutes`, `days_only`, `start_on_monday`, and `is_group`. A weekly muster also returns `weekdays`. There is no stored timezone. An empty display name is filled from the account when the caller is allowed to see that person. Read tools are annotated read-only. This replace tool is annotated destructive.

`muster_decline` sets declined on a group muster. The caller must be on the invite list. The owner is not an invitee unless their email is on that list.

`muster_add_attendees` is owner only. It adds addresses and does not remove anyone. An address is stored as the bare mailbox, lowercased, so `Bob <bob@x.com>` and `BOB@x.com` are the same person. The result lists new addresses in `added` and the owner, duplicates, and anyone already invited in `skipped`. An empty `added` list means nobody new was invited. Invite mail is sent only when Listmonk is configured. The tool result says `email_sent: false` when it is not. Production does not have Listmonk on, so people are added and no mail goes out.

`muster_summarize_overlap` returns times that every stored answer in the window can make. An answer with no free time in the window blocks the result. People who have not answered are left out. A mark exactly at the window end is outside the window. People are kept apart by user id, so two respondents with the same display name do not collapse into one. When blind availability is on and the caller is not the owner, the tool returns `OVERLAP_HIDDEN` and no names, counts, or times. A two-person muster would reveal the other person if it returned a shorter list than the caller's own free times.

A stranger gets the same not-found error as a missing muster. A member who is not the owner gets `NOT_OWNER` on an invite. A member who is not invited gets `NOT_AN_INVITEE` on decline.

Edit and duplicate stay on the website. OAuth discovery stays off until a named client cannot paste a bearer.

## Upstream

Timeful (https://github.com/schej-it/timeful.app) has no GitHub releases. The three newest commits are a per-date daylight-saving fix, a mobile keyboard fix, and `America/Vancouver`. The Vancouver zone is in the timezone list. The other two commits are large frontend changes on a fork that has already diverged (rebrand, billing removal, MCP). They are not in this release.
