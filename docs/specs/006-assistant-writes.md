# 6. Assistant writes

Status: Done.

Three write tools use the same token as the reads. They do not call the website handlers, because those handlers allow a guest.

`muster_set_availability` replaces the caller's times. It does not merge with the previous answer. Omitting a list clears it.

`muster_decline` sets declined on a group muster. The caller must be on the invite list.

`muster_add_attendees` is owner only. It adds addresses and does not remove anyone. An address is stored as the bare mailbox, lowercased. `added` lists who was newly invited. `skipped` lists the owner, duplicates, and anyone already on the list. Invite mail goes out only when Listmonk is configured. Production does not have Listmonk on, so the row is added and no mail is sent.

A stranger gets the same not-found error as a missing muster.

Done when the owner can add a person, an invitee can decline, and a non-owner cannot invite.
