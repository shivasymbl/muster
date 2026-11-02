# 5. Assistant reads

Status: Done.

`POST https://muster.asymbl.app/mcp` is a second door. It accepts a bearer token created in Settings. The token starts with `muster_`. The plaintext is shown once. The server stores a hash. Settings can list and revoke tokens. A person can hold five active tokens. Each lasts 30 days.

The read tools are `muster_whoami`, `muster_list_my_events`, `muster_get_event`, and `muster_get_responses`. The user id comes from the token. An extra user id in the arguments is ignored.

The list is the caller's own musters, paged, with an optional name filter on that list only. A response window is at most 7 days. The assistant sees the same answers the website would show that person. A sign-up form is refused with a link to the page.

A call with no token returns 401. `GET /mcp` returns 401 with a sign-in header when the caller is not signed in. A valid token on `GET /mcp` returns 405 because there is no event stream. The website routes do not accept the bearer token.

`initialize` includes instructions for three workflows, also readable at `muster://skills/pick-a-time`, `muster://skills/answer`, and `muster://skills/run-a-group`. The server does not advertise resource-list changes.

Done when a token from Settings can list the caller's musters and a missing token cannot.
