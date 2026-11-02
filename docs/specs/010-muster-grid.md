# 10. Show the muster grid

Status: In progress.

`muster_get_event` and `muster_list_my_events` return the stored grid: `dates` as UTC instants, `duration_hours`, `slot_minutes`, `days_only`, `start_on_monday`, and `is_group`. A weekly muster also returns `weekdays`. Muster does not store a timezone, so none is invented.

Displayed names are trimmed. A weekly muster stores its times on the reference week 2018-06-17 through 2018-06-24. The tool description says to query those `dates`, not a later calendar week. `start` and `end` stay absolute RFC3339 times at most 7 days apart.

A too-wide window is still rejected before any muster lookup. The dates are on the event, not in that error.

Task: https://github.com/shivasymbl/muster/issues/12

Done when a dated muster shows the grid and `is_group`, and a weekly muster explains the weekday mapping in the tool description.
