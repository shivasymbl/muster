package mcp

// ResponseFlags is the caller's answer on one muster, without the timestamps.
type ResponseFlags struct {
	Exists   bool
	NonEmpty bool
}

// MyRoles lists owner and invitee. Both can be set. Invitee includes a declined attendee.
func MyRoles(owner, attendee bool) []string {
	roles := make([]string, 0, 2)
	if owner {
		roles = append(roles, "owner")
	}
	if attendee {
		roles = append(roles, "invitee")
	}
	return roles
}

// MyStatus uses the five-step order. A nil declined pointer is not declined.
// isAttendee is required so a missing declined field can still be invited_no_answer.
func MyStatus(flags ResponseFlags, isAttendee bool, declined *bool) string {
	if flags.Exists && flags.NonEmpty {
		return "responded"
	}
	if flags.Exists {
		return "cleared"
	}
	if isAttendee && declined != nil && *declined {
		return "declined"
	}
	if isAttendee {
		return "invited_no_answer"
	}
	return "no_answer"
}
