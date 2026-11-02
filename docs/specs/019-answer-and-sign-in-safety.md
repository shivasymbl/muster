# Answer and sign-in safety

A revoked assistant token cannot mint a new one. Approving Claude sign-in only works from the consent page for that exact client, redirect, and challenge, and only for this site's MCP address. The session cookie is HttpOnly and Lax, and Secure when the site URL is https. `muster_set_availability` does not change an answer when both lists are omitted. Overlap stays inside the requested window, and an answer with no free time in that window blocks the result.

Not in this change: weekly answers stay on the stored reference week and are not copied onto a later week. Reminder and finalize mail still report `email_sent` false. OAuth registration limits, refresh failure windows, and a Go test job in CI stay open.

Done when a revoked token refresh is rejected, a consent post without the page nonce issues no code, an availability call with only an event id leaves the stored answer in place, and a mark at the window end does not produce a slot past the end.
