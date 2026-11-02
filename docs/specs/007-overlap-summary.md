# 7. Overlap summary

Status: Done.

`muster_summarize_overlap` returns the times every stored answer in the window can make. An answer with no free time in the window blocks the result. People who have not answered are left out. Two people with the same display name stay two people. A mark exactly at the window end is outside the window.

When blind availability is on and the caller is not the owner, the tool returns `OVERLAP_HIDDEN` and no names, counts, or times. A shorter list than the caller's own free times would reveal the other person on a two-person muster.

The owner, and anyone when blind availability is off, gets the times and the names.

Done when a hidden two-person muster gives the non-owner no name and no time, and gives the owner both names.
