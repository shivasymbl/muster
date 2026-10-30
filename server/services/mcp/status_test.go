package mcp

import "testing"

func TestMyStatusOrder(t *testing.T) {
	declined := true
	if got := MyStatus(ResponseFlags{Exists: true, NonEmpty: true}, true, &declined); got != "responded" {
		t.Fatal(got)
	}
	if got := MyStatus(ResponseFlags{Exists: true}, true, &declined); got != "cleared" {
		t.Fatal(got)
	}
	if got := MyStatus(ResponseFlags{}, true, &declined); got != "declined" {
		t.Fatal(got)
	}
	if got := MyStatus(ResponseFlags{}, true, nil); got != "invited_no_answer" {
		t.Fatal(got)
	}
	if got := MyStatus(ResponseFlags{}, false, nil); got != "no_answer" {
		t.Fatal(got)
	}
	roles := MyRoles(true, true)
	if len(roles) != 2 || roles[0] != "owner" || roles[1] != "invitee" {
		t.Fatal(roles)
	}
}
