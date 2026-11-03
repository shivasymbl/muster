package mcp

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/shivasymbl/muster/server/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type snapEvents struct {
	decisionEvents
	attendees []models.Attendee
}

func (f *snapEvents) Attendees(primitive.ObjectID) []models.Attendee { return f.attendees }

type snapUsers map[string]*models.User

func (u snapUsers) GetByID(id string) *models.User {
	for _, v := range u {
		if v.Id.Hex() == id {
			return v
		}
	}
	return nil
}
func (u snapUsers) GetByEmail(e string) *models.User { return u[e] }

func dt(h, m int) primitive.DateTime {
	return primitive.NewDateTimeFromTime(time.Date(2030, 1, 7, h, m, 0, 0, time.UTC))
}

func TestMCPAppsSnapshot(t *testing.T) {
	owner := &models.User{Id: primitive.NewObjectID(), Email: "owner@x.com", FirstName: "Olive"}
	member := &models.User{Id: primitive.NewObjectID(), Email: "member@x.com", FirstName: "Mel"}
	other := &models.User{Id: primitive.NewObjectID(), Email: "other@x.com", FirstName: "Otto"}
	stranger := &models.User{Id: primitive.NewObjectID(), Email: "s@x.com"}
	users := snapUsers{owner.Email: owner, member.Email: member, other.Email: other}
	hours := float32(2)
	slot := 30
	newEvent := func(blind bool) *models.Event {
		return &models.Event{
			Id: primitive.NewObjectID(), OwnerId: owner.Id, Name: "Plan", Type: models.GROUP,
			Dates: []primitive.DateTime{dt(9, 0)}, Duration: &hours, TimeIncrement: &slot,
			BlindAvailabilityEnabled: &blind,
		}
	}
	resp := func(u *models.User, avail, need []primitive.DateTime) models.EventResponse {
		return models.EventResponse{UserId: u.Id.Hex(), Response: &models.Response{
			Name: u.FirstName, Email: u.Email, UserId: u.Id, Availability: avail, IfNeeded: need,
		}}
	}
	declined := true
	setup := func(blind bool, rows ...models.EventResponse) (Deps, *models.Event) {
		ev := newEvent(blind)
		fe := &snapEvents{attendees: []models.Attendee{{Email: member.Email}, {Email: other.Email, Declined: &declined}}}
		fe.event = ev
		fe.answers = rows
		return Deps{Events: fe, Users: users}, ev
	}
	open := func(d Deps, u *models.User, extra SnapshotRequest) (*Snapshot, *ToolError) {
		extra.Workflow = "pick-a-time"
		extra.EventID = "x"
		return BuildSnapshot(d, u, extra)
	}
	own := func(s *Snapshot) *OwnAnswer { return s.Sections.OwnAnswer.Data.(*OwnAnswer) }
	people := func(s *Snapshot) *SnapshotPeople { return s.Sections.People.Data.(*SnapshotPeople) }

	t.Run("owner sees email and ranking", func(t *testing.T) {
		d, _ := setup(false, resp(member, []primitive.DateTime{dt(9, 0), dt(9, 30)}, nil))
		s, bad := open(d, owner, SnapshotRequest{})
		if bad != nil {
			t.Fatal(bad)
		}
		if !s.Permissions.CanManageGroup || s.Permissions.CanEditOwnAnswer == false {
			t.Fatalf("%+v", s.Permissions)
		}
		c := s.Sections.Candidates.Data.(*SnapshotCandidates)
		if len(c.Items) == 0 || c.Items[0].Available != 1 || c.Items[0].Population != "stored_respondents" {
			t.Fatalf("%+v", c)
		}
		var email bool
		for _, p := range people(s).Items {
			if p.Email == member.Email {
				email = true
			}
		}
		if !email {
			t.Fatal("owner should see member email")
		}
		if own(s).MyStatus != "no_answer" || own(s).Exists {
			t.Fatalf("%+v", own(s))
		}
	})

	t.Run("member has no emails and sees totals", func(t *testing.T) {
		d, _ := setup(false, resp(member, []primitive.DateTime{dt(9, 0)}, nil))
		s, bad := open(d, member, SnapshotRequest{})
		if bad != nil {
			t.Fatal(bad)
		}
		raw, _ := json.Marshal(s)
		if strings.Contains(string(raw), "@x.com") {
			t.Fatalf("email leaked: %s", raw)
		}
		if p := people(s); p.Totals["responded"] != 1 || p.Totals["declined"] != 1 {
			t.Fatalf("%+v", p.Totals)
		}
		if own(s).MyStatus != "responded" || len(own(s).Available) != 1 {
			t.Fatalf("%+v", own(s))
		}
	})

	t.Run("blind non-owner gets no other totals", func(t *testing.T) {
		d, _ := setup(true, resp(member, []primitive.DateTime{dt(9, 0)}, nil), resp(other, []primitive.DateTime{dt(9, 0)}, nil))
		s, bad := open(d, member, SnapshotRequest{})
		if bad != nil {
			t.Fatal(bad)
		}
		if s.Sections.People.Status != SectionHidden || s.Sections.People.Data != nil {
			t.Fatalf("%+v", s.Sections.People)
		}
		if s.Sections.Candidates.Status != SectionHidden || s.Permissions.CanRank || s.Permissions.CanViewPeople {
			t.Fatalf("%+v", s)
		}
		raw, _ := json.Marshal(s)
		if strings.Contains(string(raw), "Otto") || strings.Contains(string(raw), "totals") {
			t.Fatalf("leak: %s", raw)
		}
		if !own(s).Exists {
			t.Fatal("own answer must still be present")
		}
		// Blind owner still sees everything.
		so, _ := open(d, owner, SnapshotRequest{})
		if so.Sections.People.Status == SectionHidden {
			t.Fatal("owner hidden")
		}
	})

	t.Run("stranger equals missing event", func(t *testing.T) {
		d, _ := setup(false)
		_, bad := open(d, stranger, SnapshotRequest{})
		d.Events.(*snapEvents).event = nil
		_, missing := open(d, stranger, SnapshotRequest{})
		if bad == nil || missing == nil || *bad != *missing || bad.Code != "EVENT_NOT_FOUND_OR_NO_ACCESS" {
			t.Fatalf("%v %v", bad, missing)
		}
	})

	t.Run("cleared and declined", func(t *testing.T) {
		d, _ := setup(false, resp(member, nil, nil))
		s, _ := open(d, member, SnapshotRequest{})
		if o := own(s); !o.Exists || o.MyStatus != "cleared" || o.Available == nil || len(o.Available) != 0 {
			t.Fatalf("%+v", o)
		}
		d, _ = setup(false, resp(other, []primitive.DateTime{dt(9, 0)}, nil))
		s, bad := open(d, other, SnapshotRequest{})
		if bad != nil {
			t.Fatal(bad)
		}
		// Declined with a stored answer shows both facts.
		if o := own(s); !o.Declined || !o.Exists || len(o.Available) != 1 {
			t.Fatalf("%+v", o)
		}
	})

	t.Run("duplicate own records are an error", func(t *testing.T) {
		d, _ := setup(false, resp(member, []primitive.DateTime{dt(9, 0)}, nil), resp(member, nil, nil))
		_, bad := open(d, member, SnapshotRequest{})
		if bad == nil || bad.Code != "DUPLICATE_OWN_RESPONSE" {
			t.Fatalf("%v", bad)
		}
	})

	t.Run("invalid duration", func(t *testing.T) {
		d, _ := setup(false)
		for _, n := range []int{0, -30, 45, 31} {
			n := n
			if _, bad := open(d, owner, SnapshotRequest{DurationMinutes: &n}); bad == nil || bad.Code != "BAD_ARGUMENTS" {
				t.Fatalf("%d: %v", n, bad)
			}
		}
		ok := 60
		if _, bad := open(d, owner, SnapshotRequest{DurationMinutes: &ok}); bad != nil {
			t.Fatal(bad)
		}
	})

	t.Run("whole own answer survives the window and does not mutate rows", func(t *testing.T) {
		far := primitive.NewDateTimeFromTime(time.Date(2030, 3, 1, 9, 0, 0, 0, time.UTC))
		row := resp(member, []primitive.DateTime{dt(9, 0), far}, []primitive.DateTime{dt(9, 30)})
		d, _ := setup(false, row)
		s, bad := open(d, member, SnapshotRequest{})
		if bad != nil {
			t.Fatal(bad)
		}
		o := own(s)
		if len(o.Available) != 2 || o.AvailableCounts.InWindow != 1 || o.AvailableCounts.OffWindow != 1 || o.AvailableCounts.OffGrid != 1 {
			t.Fatalf("%+v", o)
		}
		if len(row.Response.Availability) != 2 || o.TargetFingerprint == "" || s.StateFingerprint == "" {
			t.Fatal("bad state")
		}
	})

	t.Run("scheduled is separate from archived", func(t *testing.T) {
		d, ev := setup(false)
		ev.ScheduledEvent = &models.CalendarEvent{StartDate: dt(9, 0), EndDate: dt(10, 0)}
		s, _ := open(d, owner, SnapshotRequest{})
		if s.Event.Archived || s.Event.Scheduled == nil || s.Event.Scheduled.Start != "2030-01-07T09:00:00Z" {
			t.Fatalf("%+v", s.Event)
		}
		ev.ScheduledEvent = &models.CalendarEvent{StartDate: dt(10, 0), EndDate: dt(9, 0)}
		s, _ = open(d, owner, SnapshotRequest{})
		if s.Event.ScheduledStatus != "invalid" || s.Event.Scheduled != nil {
			t.Fatalf("%+v", s.Event)
		}
	})

	t.Run("reference week labels and no timezone", func(t *testing.T) {
		d, ev := setup(false, resp(member, []primitive.DateTime{primitive.NewDateTimeFromTime(time.Date(2018, 6, 18, 15, 0, 0, 0, time.UTC))}, nil))
		ev.Type = models.DOW
		ev.Dates = []primitive.DateTime{primitive.NewDateTimeFromTime(time.Date(2018, 6, 18, 15, 0, 0, 0, time.UTC))}
		s, bad := open(d, owner, SnapshotRequest{})
		if bad != nil {
			t.Fatal(bad)
		}
		g := s.Sections.Grid.Data.(*SnapshotGrid)
		if !g.ReferenceWeek || g.DateMode != "reference_week" || len(g.WeekdayLabels) != 7 {
			t.Fatalf("%+v", g)
		}
		c := s.Sections.Candidates.Data.(*SnapshotCandidates)
		if len(c.Items) == 0 || c.Items[0].Label != "Monday 15:00-15:30" {
			t.Fatalf("%+v", c)
		}
		if raw, _ := json.Marshal(s); strings.Contains(strings.ToLower(string(raw)), "timezone") {
			t.Fatal("invented timezone")
		}
	})

	t.Run("size caps and cursor", func(t *testing.T) {
		rows := []models.EventResponse{}
		for i := 0; i < 30; i++ {
			u := &models.User{Id: primitive.NewObjectID(), FirstName: "P" + string(rune('A'+i))}
			rows = append(rows, resp(u, []primitive.DateTime{dt(9, 0)}, nil))
		}
		d, ev := setup(false, rows...)
		ev.Type = models.SPECIFIC_DATES
		d.Events.(*snapEvents).attendees = nil
		s, bad := open(d, owner, SnapshotRequest{})
		if bad != nil {
			t.Fatal(bad)
		}
		p := people(s)
		if len(p.Items) != 25 || p.Complete || p.Total != 30 || s.Sections.People.NextCursor == nil || s.Sections.People.Status != SectionPartial {
			t.Fatalf("%d %+v", len(p.Items), s.Sections.People)
		}
		s2, bad := open(d, owner, SnapshotRequest{PeopleCursor: *s.Sections.People.NextCursor})
		if bad != nil || len(people(s2).Items) != 5 || s2.Sections.People.NextCursor != nil {
			t.Fatalf("%v", bad)
		}
		if _, bad := open(d, member, SnapshotRequest{PeopleCursor: *s.Sections.People.NextCursor}); bad == nil {
			t.Fatal("cross-scope cursor accepted")
		}
		if _, bad := open(d, owner, SnapshotRequest{PeopleCursor: "junk"}); bad == nil {
			t.Fatal("junk cursor accepted")
		}
	})

	t.Run("draft persists nothing", func(t *testing.T) {
		fe := &snapEvents{}
		s, bad := BuildSnapshot(Deps{Events: fe}, owner, SnapshotRequest{Workflow: "run-a-group"})
		if bad != nil || !s.Draft || s.Event != nil || fe.created != nil {
			t.Fatalf("%v %+v", bad, s)
		}
	})
}
