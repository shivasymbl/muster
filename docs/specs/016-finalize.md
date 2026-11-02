# 16. Finalize a time

Status: Done.

`muster_finalize` is owner only. The start and end must sit on the grid. The chosen time is stored and the muster is marked closed. No finalize mail is sent, so `email_sent` is false. It does not create a calendar event.

Task: https://github.com/shivasymbl/muster/issues/18

Done when the owner can close a time on the grid and a guest cannot.
