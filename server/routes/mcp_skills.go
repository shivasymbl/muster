package routes

import "github.com/gin-gonic/gin"

const mcpInstructions = "Muster is a group availability poll, not a calendar. Before a multi-step task, read one of muster://skills/pick-a-time, muster://skills/answer, or muster://skills/run-a-group. Weekly dates are the stored reference week 2018-06-17 through 2018-06-24. There is no timezone. {weekday: Monday, time: 15:00} means 2018-06-18T15:00:00Z. muster_list_my_events defaults to status needs_answer, the musters that still need an answer; pass status all for every muster, or open, archived, needs_my_answer, projection compact to narrow it. muster_set_availability replaces the whole answer: an empty list clears that list, and omitting both lists is rejected so the stored answer stays unchanged. Pass if_match as the state_fingerprint string, or as {availability, if_needed}, to refuse when the stored answer differs. expected_state_fingerprint does the same check. Omit both to replace anyway. muster_summarize_overlap is only the times every stored answer in the window can make. People who have not answered are left out. An answer with no free time in the window blocks it. Use muster_find_best_times for a ranked partial fit. A duration that is not a positive multiple of slot_minutes is rejected. muster_finalize stores the chosen time and closes the muster. It does not create a calendar event and it sends no mail. muster_remind sends no mail. A hidden muster shows a non-owner nothing about other people."

func skillResources() []gin.H {
	out := make([]gin.H, 0, len(skillBodies))
	for _, skill := range skillBodies {
		out = append(out, gin.H{
			"uri":         skill.uri,
			"name":        skill.name,
			"description": skill.description,
			"mimeType":    "text/markdown",
		})
	}
	return out
}

func skillText(uri string) (string, bool) {
	for _, skill := range skillBodies {
		if skill.uri == uri {
			return skill.text, true
		}
	}
	return "", false
}

type mcpSkill struct {
	uri, name, description, text string
}

var skillBodies = []mcpSkill{
	{
		uri:         "muster://skills/pick-a-time",
		name:        "Pick a time",
		description: "Read a muster, rank the fits, and close it only after the user chooses.",
		text: `# Pick a time

Use this when someone asks which time works.

1. Call muster_get_event. Use its dates, duration_hours, and slot_minutes. A weekly muster stores dates on 2018-06-17 through 2018-06-24. Do not move those dates onto a later week. Muster has no timezone.
2. Call muster_list_attendees when you need to know who was invited. Email is visible to the owner only. For the owner, muster_get_event also returns response_totals (null for anyone else). Omit start and end on muster_get_responses and muster_summarize_overlap to use the muster dates, at most 7 days.
3. Call muster_find_best_times to rank slot starts. It includes people who are if-needed or missing: missing is everyone not fully available, split into if_needed_names and unavailable_names. The window defaults to the muster dates and is at most 7 days. duration_minutes must be a positive multiple of slot_minutes. A shorter or off-grid length is rejected, not rounded.
4. Call muster_summarize_overlap only for times every stored answer in the window can make. People who have not answered are left out, so an overlap is not "everyone invited." An answer with no free time in the window blocks the result. Say that plainly.
5. A hidden muster returns OVERLAP_HIDDEN to a non-owner. Stop. Ask the owner.
6. Call muster_finalize only after the user picks a start and end that sit on the grid. It stores that time and closes the muster. It does not create a calendar event and it sends no mail. Say both of those.

Do not invent a time the grid rejected. Read the event again if you are unsure the close stuck.`,
	},
	{
		uri:         "muster://skills/answer",
		name:        "Answer a muster",
		description: "Replace the caller's availability without wiping an answer by accident.",
		text: `# Answer a muster

Use this when the signed-in person is marking when they can make it.

1. Call muster_get_event before writing. Copy slot starts from its grid. A weekly muster only accepts the stored reference-week instants.
2. Call muster_set_availability with the full replacement. Legacy lists do not merge. Pass availability, if_needed, or both. Or pass ranges [{start, end, state}] with mode replace or merge; range writes return CONFLICT if the stored answer changed. An empty list clears that list. Omitting one list clears it. Omitting both, or passing null, is rejected and the stored answer stays unchanged. On a weekly muster, {weekday, time} uses the stored weekday. Monday 15:00 is 2018-06-18T15:00:00Z when that Monday is the only one stored. Pass if_match as the state_fingerprint string, or as {availability, if_needed}, to refuse when the stored answer changed. expected_state_fingerprint is the same check. Omit both to replace anyway.
3. Each time is an RFC3339 slot start. A time outside the muster dates, off slot_minutes, or in both lists is rejected. Duplicates are stored once.
4. An empty stored answer is my_status cleared, not no_answer.
5. Call muster_decline only for a group muster the caller was invited to. Decline does not delete an availability answer.
6. The receipt echoes the stored answer and a state_fingerprint, so no second read is needed. echo_complete false means the readback did not confirm the write. On a hidden muster a non-owner can read only their own answer.`,
	},
	{
		uri:         "muster://skills/run-a-group",
		name:        "Run a group muster",
		description: "Create a group muster, invite people, and see what changed. Mail is usually off.",
		text: `# Run a group muster

Use this when the signed-in person is the owner and needs an invite list.

1. Call muster_create_event with type group. dates are required unless you truly want an undated group. Weekly dates must be on 2018-06-17 through 2018-06-24. {weekday: Monday, time: 15:00} means 2018-06-18T15:00:00Z. emails are allowed only for a group. The owner is not invited again.
2. email_sent is true only when mail delivery is configured. Production usually returns false. Do not tell the user the invites were emailed unless email_sent is true. Give them page_url.
3. Call muster_add_attendees to add more addresses. It does not remove anyone. Read added for who was newly invited and skipped for the owner, duplicates, and anyone already invited; skipped_details gives each skipped input's reason and index. An empty added list means nobody new was invited.
4. Call muster_list_attendees for invited, responded, declined, and cleared. The owner's muster_get_event has response_totals. Then call muster_remind to list people who have not answered; the owner can appear there. Remind sends no mail, so email_sent is false.
5. Call muster_remove_attendee to drop one guest and that guest's answer. You cannot remove the owner.
6. Call muster_list_changes with a since timestamp to see answers and invites recorded at or after that time. Older edits with no timestamp are omitted. There is no webhook.
7. Call muster_update_event to rename, replace dates, change duration, archive, or set blind. It cannot change the owner or the type.
8. When a time is chosen, follow muster://skills/pick-a-time. Finalize still sends no mail and creates no calendar event.`,
	},
}
