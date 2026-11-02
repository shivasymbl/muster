# 11. Reject availability the muster cannot hold

Status: In progress.

When a muster has stored dates, `muster_set_availability` rejects a time outside those windows and a time inside a window but off `slot_minutes`. A time in both lists is rejected. Duplicate times are stored once and counted as `duplicates_ignored`.

A group muster with no dates still accepts times. An empty answer still clears the previous one and leaves `my_status` as `cleared`.

Task: https://github.com/shivasymbl/muster/issues/13

Done when a dated muster refuses a time in 2030 and an off-grid minute, and a repeated timestamp is stored once.
