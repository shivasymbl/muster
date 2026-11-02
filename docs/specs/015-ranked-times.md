# 15. Rank times by who can make them

Status: Done.

`muster_find_best_times` ranks slot starts by how many respondents can make a meeting of `duration_minutes`. Each slot includes an if-needed count and who is missing. The window defaults to the muster's dates, capped at 7 days. `muster_summarize_overlap` still returns only unanimous times. A hidden muster still returns nothing to a non-owner.

Task: https://github.com/shivasymbl/muster/issues/17

Done when a partial fit returns a ranked slot instead of an empty unanimous result.
