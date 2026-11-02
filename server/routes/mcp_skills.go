package routes

import "github.com/gin-gonic/gin"

const mcpInstructions = "Muster is a group availability poll, not a calendar. Before a multi-step task, read one of muster://skills/pick-a-time, muster://skills/answer, or muster://skills/run-a-group. Weekly dates are the stored reference week 2018-06-17 through 2018-06-24. There is no timezone. muster_set_availability replaces the whole answer: an empty list clears that list, and omitting both lists changes nothing. muster_summarize_overlap is only the times every stored answer in the window can make. People who have not answered are left out. An answer with no free time in the window blocks it. Use muster_find_best_times for a ranked partial fit. muster_finalize stores the chosen time and closes the muster. It does not create a calendar event and it sends no mail. muster_remind sends no mail. A hidden muster shows a non-owner nothing about other people."

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
2. Call muster_list_attendees when you need to know who was invited. Email is visible to the owner only.
3. Call muster_find_best_times to rank slot starts. It includes people who are if-needed or missing. The window defaults to the muster dates and is at most 7 days.
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
2. Call muster_set_availability with the full replacement. It does not merge. Pass availability, if_needed, or both. An empty list clears that list. Omitting one list clears it. Omitting both, or passing null, leaves the stored answer unchanged.
3. Each time is an RFC3339 slot start. A time outside the muster dates, off slot_minutes, or in both lists is rejected. Duplicates are stored once.
4. An empty stored answer is my_status cleared, not no_answer.
5. Call muster_decline only for a group muster the caller was invited to. Decline does not delete an availability answer.
6. Call muster_get_responses for the caller's own visible answer when you need to confirm what was saved. On a hidden muster that is the only answer a non-owner can see.`,
	},
	{
		uri:         "muster://skills/run-a-group",
		name:        "Run a group muster",
		description: "Create a group muster, invite people, and see what changed. Mail is usually off.",
		text: `# Run a group muster

Use this when the signed-in person is the owner and needs an invite list.

1. Call muster_create_event with type group. dates are required unless you truly want an undated group. Weekly dates must be on 2018-06-17 through 2018-06-24. emails are allowed only for a group. The owner is not invited again.
2. email_sent is true only when Listmonk is configured. Production usually returns false. Do not tell the user the invites were emailed unless email_sent is true. Give them page_url.
3. Call muster_add_attendees to add more addresses. It does not remove anyone. Already-invited addresses are skipped.
4. Call muster_list_attendees for invited, responded, declined, and cleared. Then call muster_remind to list people who have not answered. Remind sends no mail, so email_sent is false.
5. Call muster_remove_attendee to drop one guest and that guest's answer. You cannot remove the owner.
6. Call muster_list_changes with a since timestamp to see answers and invites recorded at or after that time. Older edits with no timestamp are omitted. There is no webhook.
7. Call muster_update_event to rename, replace dates, change duration, archive, or set blind. It cannot change the owner or the type.
8. When a time is chosen, follow muster://skills/pick-a-time. Finalize still sends no mail and creates no calendar event.`,
	},
}
