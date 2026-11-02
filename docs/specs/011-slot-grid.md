# 11. Reject availability the muster cannot hold

Status: Done.

When a muster has stored dates, `muster_set_availability` rejects a time outside those windows and a time inside a window but off `slot_minutes`. A weekly muster uses the same rule on its June 2018 reference instants. A later calendar week is outside, and a window that runs past midnight still accepts the slots on the next reference day. A time in both lists is rejected. Duplicate times are stored once and counted as `duplicates_ignored`.

A group muster with no dates still accepts times. An empty answer still clears the previous one and leaves `my_status` as `cleared`.

Task: https://github.com/shivasymbl/muster/issues/13

Done when a dated muster refuses a time in 2030 and an off-grid minute, and a repeated timestamp is stored once.
