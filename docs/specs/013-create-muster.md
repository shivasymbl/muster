# 13. Create a muster from the assistant

Status: Done.

`muster_create_event` creates a muster for the signed-in user and returns `event_id`, `short_id`, and `page_url`. The type is `specific_dates`, `dow`, or `group`. A weekly muster must use the reference week 2018-06-17 through 2018-06-24. Invitees are allowed only on a group muster. Invite mail is sent only when Listmonk is on.

Task: https://github.com/shivasymbl/muster/issues/15

Done when the owner gets a link back and a weekly date outside that week is refused.
