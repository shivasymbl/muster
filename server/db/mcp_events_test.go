package db

import "testing"

func TestListProjectionKeepsScheduleAndTimezone(t *testing.T) {
	for _, key := range []string{"scheduledEvent", "displayTimezone", "isArchived"} {
		if _, ok := listEventFields[key]; !ok {
			t.Errorf("list projection dropped %s, so muster_list_my_events cannot match muster_get_event", key)
		}
	}
}
