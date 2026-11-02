# 12. Name the people and describe the tools

Status: In progress.

When the caller is allowed to see a person, an empty name is filled from that person's account. An email the privacy filter removed is not put back.

`my_status` is `no_answer`, `invited_no_answer`, `responded`, `cleared`, or `declined`. `manual` is day-level marks and stays in the payload. Overlap returns only times every respondent in the window can make, with display names.

Read tools are marked read-only. `muster_set_availability` is marked destructive because it replaces the answer. `muster_add_attendees` is marked open-world because it can email when Listmonk is on. `muster_decline` is marked destructive.

Task: https://github.com/shivasymbl/muster/issues/14

Done when a person with an account shows a display name, and `tools/list` carries those annotations.
