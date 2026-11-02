# 18. Remind, remove, and a change feed

Status: In progress.

`muster_remind` lists group invitees who have not answered. No reminder mail is sent, because there is no reminder template, so `email_sent` is false.

`muster_remove_attendee` removes one invitee and that person's answer, matching the website. The owner cannot be removed.

`muster_list_changes` returns answers and invites recorded at or after `since`. Older edits that have no timestamp are left out. There is no webhook.

Task: https://github.com/shivasymbl/muster/issues/20

Done when the owner can see who has not answered, remove one invitee, and read changes after a timestamp.
