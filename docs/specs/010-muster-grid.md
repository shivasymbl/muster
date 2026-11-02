# 10. Show the muster grid

Status: In progress.

`muster_get_event` and `muster_list_my_events` return the stored grid: `dates` as UTC instants, `duration_hours`, `slot_minutes`, `days_only`, `start_on_monday`, and `is_group`. A weekly muster also returns `weekdays`. Muster does not store a timezone, so none is invented.

Displayed names are trimmed. The tool descriptions say that a weekly window matches the stored UTC weekday and UTC clock, and that `start` and `end` stay absolute RFC3339 times at most 7 days apart.

A too-wide window is still rejected before any muster lookup. The dates are on the event, not in that error.

Task: https://github.com/shivasymbl/muster/issues/12

Done when a dated muster shows the grid and `is_group`, and a weekly muster explains the weekday mapping in the tool description.
