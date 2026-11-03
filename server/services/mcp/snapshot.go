package mcp

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shivasymbl/muster/server/models"
	"github.com/shivasymbl/muster/server/utils"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	SnapshotSchemaVersion = 1
	MaxSnapshotPeople     = 25
	MaxSnapshotCandidates = 20
	maxWindowSpan         = 7 * 24 * time.Hour
)

// Section statuses. Data is nil unless the status is complete or partial.
const (
	SectionComplete     = "complete"
	SectionPartial      = "partial"
	SectionHidden       = "hidden"
	SectionNotRequested = "not_requested"
	SectionUnavailable  = "unavailable"
)

// Section is one part of a snapshot. Reason explains hidden, unavailable, and partial states.
type Section struct {
	Status     string  `json:"status"`
	Reason     string  `json:"reason,omitempty"`
	Data       any     `json:"data"`
	NextCursor *string `json:"next_cursor,omitempty"`
}

func sectionOf(status, reason string, data any) Section {
	return Section{Status: status, Reason: reason, Data: data}
}

// SnapshotRequest is the parsed opener input. Window is nil when the caller let the server choose.
type SnapshotRequest struct {
	Workflow        string
	EventID         string
	WindowStart     *time.Time
	WindowEnd       *time.Time
	DurationMinutes *int
	PeopleCursor    string
}

type SnapshotEvent struct {
	ID                string         `json:"id"`
	ShortID           string         `json:"short_id,omitempty"`
	Name              string         `json:"name"`
	Kind              string         `json:"kind"`
	Archived          bool           `json:"archived"`
	Scheduled         *ScheduledSpan `json:"scheduled"`
	ScheduledStatus   string         `json:"scheduled_status"` // none, set, invalid
	BlindAvailability bool           `json:"blind_availability"`
	IsSignUpForm      bool           `json:"is_sign_up_form"`
}

// ScheduledSpan is the stored final interval. It is independent of archived.
type ScheduledSpan struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type SnapshotPermissions struct {
	CanRank          bool `json:"can_rank"`
	CanViewPeople    bool `json:"can_view_people"`
	CanEditOwnAnswer bool `json:"can_edit_own_answer"`
	CanManageGroup   bool `json:"can_manage_group"`
	CanFinalize      bool `json:"can_finalize"`
}

// SnapshotGrid. DateMode is actual, reference_week, undated, or ambiguous.
// Muster stores no timezone, so none is reported.
type SnapshotGrid struct {
	SlotMinutes   int      `json:"slot_minutes"`
	DurationHours float64  `json:"duration_hours,omitempty"`
	DaysOnly      bool     `json:"days_only"`
	StartOnMonday bool     `json:"start_on_monday"`
	ReferenceWeek bool     `json:"reference_week"`
	DateMode      string   `json:"date_mode"`
	Dates         []string `json:"dates"`
	WeekdayLabels []string `json:"weekday_labels,omitempty"`
	WindowStart   string   `json:"window_start,omitempty"`
	WindowEnd     string   `json:"window_end,omitempty"`
}

type SnapshotCandidate struct {
	Start                string   `json:"start"`
	End                  string   `json:"end"`
	Label                string   `json:"label,omitempty"`
	DurationMinutes      int      `json:"duration_minutes"`
	Available            int      `json:"available"`
	IfNeeded             int      `json:"if_needed"`
	NotFullyAvailable    int      `json:"not_fully_available"`
	NotFullyAvailableWho []string `json:"not_fully_available_labels"`
	Population           string   `json:"population"`
}

type SnapshotCandidates struct {
	DurationMinutes int                 `json:"duration_minutes"`
	Complete        bool                `json:"complete"`
	Items           []SnapshotCandidate `json:"items"`
}

type SnapshotPerson struct {
	Key       string      `json:"key"`
	Name      string      `json:"name"`
	Email     string      `json:"email,omitempty"`
	Status    string      `json:"status"`
	IsMe      bool        `json:"is_me"`
	Available []TimeRange `json:"available"`
	IfNeeded  []TimeRange `json:"if_needed"`
}

type SnapshotPeople struct {
	Complete bool             `json:"complete"`
	Total    int              `json:"total"`
	Totals   map[string]int   `json:"totals"`
	Items    []SnapshotPerson `json:"items"`
}

type OwnCounts struct {
	InWindow  int `json:"in_window"`
	OffWindow int `json:"off_window"`
	OffGrid   int `json:"off_grid"`
}

// OwnAnswer carries the caller's whole stored answer. Available and IfNeeded are never
// cut to the window. Window counts say how much of it the visible window shows.
type OwnAnswer struct {
	Exists            bool      `json:"exists"`
	MyStatus          string    `json:"my_status"`
	IsAttendee        bool      `json:"is_attendee"`
	Declined          bool      `json:"declined"`
	Available         []string  `json:"available"`
	IfNeeded          []string  `json:"if_needed"`
	EditableLayers    []string  `json:"editable_layers"`
	PreservedLayers   []string  `json:"preserved_noneditable_layers"`
	ManualDays        int       `json:"manual_days"`
	AvailableCounts   OwnCounts `json:"available_counts"`
	IfNeededCounts    OwnCounts `json:"if_needed_counts"`
	Editable          bool      `json:"editable"`
	Complete          bool      `json:"complete"`
	TargetFingerprint string    `json:"target_fingerprint"`
}

type SnapshotSections struct {
	Grid       Section `json:"grid"`
	Candidates Section `json:"candidates"`
	People     Section `json:"people"`
	OwnAnswer  Section `json:"own_answer"`
}

// Snapshot is the _meta.muster payload of muster_open_view.
type Snapshot struct {
	SchemaVersion    int                 `json:"schema_version"`
	SnapshotID       string              `json:"snapshot_id"`
	StateFingerprint string              `json:"state_fingerprint"`
	ActionBases      map[string]string   `json:"action_bases"`
	ReadAt           string              `json:"read_at"`
	Workflow         string              `json:"workflow"`
	Draft            bool                `json:"draft"`
	Event            *SnapshotEvent      `json:"event"`
	Permissions      SnapshotPermissions `json:"permissions"`
	Sections         SnapshotSections    `json:"sections"`
}

// Summary is the model-safe one-line text. It never names other people.
func (s *Snapshot) Summary() string {
	if s.Draft {
		return "New group draft. Nothing is saved until the group is created."
	}
	var b strings.Builder
	b.WriteString(s.Event.Name)
	if c, ok := s.Sections.Candidates.Data.(*SnapshotCandidates); ok && len(c.Items) > 0 {
		top := c.Items[0]
		fmt.Fprintf(&b, ": %d candidate times; best has %d available and %d if needed.", len(c.Items), top.Available, top.IfNeeded)
	} else {
		b.WriteString(": no candidate times to show.")
	}
	if o, ok := s.Sections.OwnAnswer.Data.(*OwnAnswer); ok {
		b.WriteString(" Your status: " + o.MyStatus + ".")
	}
	return b.String()
}

// BuildSnapshot reads one authorized snapshot. With no event_id it returns an unsaved creation draft.
// It runs separate reads, so it is not a transactional snapshot.
func BuildSnapshot(deps Deps, user *models.User, req SnapshotRequest) (*Snapshot, *ToolError) {
	snap := &Snapshot{
		SchemaVersion: SnapshotSchemaVersion,
		SnapshotID:    newSnapshotID(),
		ActionBases:   map[string]string{},
		ReadAt:        deps.now().UTC().Format(time.RFC3339),
		Workflow:      req.Workflow,
	}
	if req.EventID == "" {
		snap.Draft = true
		snap.Sections = SnapshotSections{
			Grid: sectionOf(SectionComplete, "", &SnapshotGrid{
				SlotMinutes: 15, DateMode: "undated", Dates: []string{},
			}),
			Candidates: sectionOf(SectionNotRequested, "", nil),
			People:     sectionOf(SectionNotRequested, "", nil),
			OwnAnswer:  sectionOf(SectionNotRequested, "", nil),
		}
		snap.StateFingerprint = fingerprint("draft", req.Workflow)
		return snap, nil
	}

	event := deps.Events.GetByEitherID(req.EventID)
	if user == nil || !member(deps, user, event) {
		return nil, notFound()
	}
	owner := event.OwnerId == user.Id
	blind := utils.Coalesce(event.BlindAvailabilityEnabled)
	signUp := utils.Coalesce(event.IsSignUpForm)
	viewAll := owner || !blind
	inc := increment(event)
	slotMinutes := int(inc / time.Minute)

	// Validate the duration before any read work.
	minutes := slotMinutes
	if req.DurationMinutes != nil {
		minutes = *req.DurationMinutes
	}
	if minutes < 1 || time.Duration(minutes)*time.Minute%inc != 0 {
		return nil, toolErr("BAD_ARGUMENTS", "duration_minutes must be a positive multiple of slot_minutes.", "Use a multiple of "+strconv.Itoa(slotMinutes)+".")
	}

	start, end, haveWindow := time.Time{}, time.Time{}, false
	if req.WindowStart != nil && req.WindowEnd != nil {
		start, end, haveWindow = req.WindowStart.UTC(), req.WindowEnd.UTC(), true
		if !start.Before(end) || end.Sub(start) > maxWindowSpan {
			return nil, toolErr("BAD_ARGUMENTS", "window must be at most 7 days with start before end.", "Narrow the window.")
		}
	} else {
		start, end, haveWindow = eventWindow(event)
	}

	attendees := deps.Events.Attendees(event.Id)
	responses := deps.Events.Responses(event.Id)

	// Own answer first, copied before anything else touches the response rows.
	own, bad := readOwnAnswer(deps, user, event, attendees, responses, start, end, haveWindow, signUp)
	if bad != nil {
		return nil, bad
	}

	snap.Event = snapshotEvent(event, blind, signUp)
	snap.Permissions = SnapshotPermissions{
		CanRank:          !signUp && viewAll,
		CanViewPeople:    viewAll,
		CanEditOwnAnswer: own.Status == SectionComplete && !signUp && own.Data.(*OwnAnswer).Editable,
		CanManageGroup:   owner,
		CanFinalize:      owner && !signUp && !snap.Event.Archived,
	}
	grid := snapshotGrid(event, start, end, haveWindow)

	cands := sectionOf(SectionNotRequested, "", nil)
	switch {
	case signUp:
		cands = sectionOf(SectionUnavailable, "sign_up_form", nil)
	case !viewAll:
		cands = sectionOf(SectionHidden, "hidden_availability", nil)
	case !haveWindow:
		cands = sectionOf(SectionUnavailable, "undated", nil)
	default:
		cands = buildCandidates(deps, event, responses, start, end, minutes)
	}

	people := sectionOf(SectionNotRequested, "", nil)
	switch {
	case signUp:
		people = sectionOf(SectionUnavailable, "sign_up_form", nil)
	case !viewAll:
		people = sectionOf(SectionHidden, "hidden_availability", nil)
	default:
		var bad *ToolError
		people, bad = buildPeople(deps, user, event, owner, attendees, responses, start, end, haveWindow, req.PeopleCursor)
		if bad != nil {
			return nil, bad
		}
	}

	snap.Sections = SnapshotSections{
		Grid:       sectionOf(SectionComplete, "", grid),
		Candidates: cands,
		People:     people,
		OwnAnswer:  own,
	}
	snap.StateFingerprint = fingerprint("event", snap.Event, snap.Permissions, grid, cands.Data, people.Data, own.Data)
	return snap, nil
}

func newSnapshotID() string {
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		return "read-unavailable"
	}
	return "read-" + hex.EncodeToString(raw)
}

// fingerprint is a canonical comparison string over typed values. It is not an auth token.
func fingerprint(parts ...any) string {
	h := sha256.New()
	for _, p := range parts {
		raw, err := json.Marshal(p)
		if err != nil {
			raw = []byte("?")
		}
		h.Write([]byte(strconv.Itoa(len(raw))))
		h.Write([]byte{':'})
		h.Write(raw)
	}
	return "fp1:" + hex.EncodeToString(h.Sum(nil)[:16])
}

func opaqueKey(eventID primitive.ObjectID, kind, id string) string {
	sum := sha256.Sum256([]byte(eventID.Hex() + "|" + kind + "|" + id))
	return "p_" + hex.EncodeToString(sum[:8])
}

func snapshotEvent(event *models.Event, blind, signUp bool) *SnapshotEvent {
	out := &SnapshotEvent{
		ID:                event.Id.Hex(),
		Name:              truncateRunes(strings.TrimSpace(event.Name), 200),
		Kind:              string(event.Type),
		Archived:          utils.Coalesce(event.IsArchived),
		ScheduledStatus:   "none",
		BlindAvailability: blind,
		IsSignUpForm:      signUp,
	}
	if event.ShortId != nil {
		out.ShortID = *event.ShortId
	}
	if s := event.ScheduledEvent; s != nil {
		st, en := s.StartDate.Time().UTC(), s.EndDate.Time().UTC()
		if s.StartDate == 0 && s.EndDate == 0 {
			// Empty object: no recorded interval.
		} else if s.StartDate == 0 || s.EndDate == 0 || !st.Before(en) {
			out.ScheduledStatus = "invalid"
		} else {
			out.ScheduledStatus = "set"
			out.Scheduled = &ScheduledSpan{Start: st.Format(time.RFC3339), End: en.Format(time.RFC3339)}
		}
	}
	return out
}

var weekLabels = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

func snapshotGrid(event *models.Event, start, end time.Time, haveWindow bool) *SnapshotGrid {
	base := eventGrid(event)
	g := &SnapshotGrid{
		SlotMinutes: base.SlotMinutes, DurationHours: base.DurationHours,
		DaysOnly: base.DaysOnly, StartOnMonday: base.StartOnMonday,
		Dates: base.Dates,
	}
	ref, actual := 0, 0
	for _, d := range event.Dates {
		if onReferenceWeek(d.Time()) {
			ref++
		} else {
			actual++
		}
	}
	switch {
	case len(event.Dates) == 0:
		g.DateMode = "undated"
	case ref > 0 && actual > 0:
		g.DateMode = "ambiguous"
	case ref > 0:
		g.DateMode = "reference_week"
		g.ReferenceWeek = true
		g.WeekdayLabels = weekLabels
	default:
		g.DateMode = "actual"
	}
	if haveWindow {
		g.WindowStart, g.WindowEnd = start.UTC().Format(time.RFC3339), end.UTC().Format(time.RFC3339)
	}
	return g
}

// weeklyLabel names a reference-week instant without claiming a timezone.
func weeklyLabel(start, end time.Time) string {
	s, e := start.UTC(), end.UTC()
	return fmt.Sprintf("%s %s-%s", s.Weekday().String(), s.Format("15:04"), e.Format("15:04"))
}

func buildCandidates(deps Deps, event *models.Event, responses []models.EventResponse, start, end time.Time, minutes int) Section {
	// Copy rows so name filling cannot touch the store's data.
	rows := make([]models.EventResponse, 0, len(responses))
	for _, row := range responses {
		if row.Response == nil {
			continue
		}
		cp := *row.Response
		if strings.TrimSpace(personName(&cp)) == "" {
			cp.Name = displayName(deps, &cp)
		}
		row.Response = &cp
		rows = append(rows, row)
	}
	slots := rankSlots(event, rows, start, end, time.Duration(minutes)*time.Minute)
	weekly := onReferenceWeek(start) && (event.Type == models.DOW || event.Type == models.GROUP)
	out := &SnapshotCandidates{DurationMinutes: minutes, Complete: len(slots) < MaxSnapshotCandidates, Items: []SnapshotCandidate{}}
	if len(slots) > MaxSnapshotCandidates {
		slots = slots[:MaxSnapshotCandidates]
	}
	for _, s := range slots {
		st, _ := time.Parse(time.RFC3339, s.Start)
		en, _ := time.Parse(time.RFC3339, s.End)
		c := SnapshotCandidate{
			Start: s.Start, End: s.End, DurationMinutes: minutes,
			Available: s.Available, IfNeeded: s.IfNeeded,
			NotFullyAvailable: len(s.Missing), NotFullyAvailableWho: s.Missing,
			Population: "stored_respondents",
		}
		if weekly {
			c.Label = weeklyLabel(st, en)
		}
		out.Items = append(out.Items, c)
	}
	if !out.Complete {
		return sectionOf(SectionPartial, "candidate_cap", out)
	}
	return sectionOf(SectionComplete, "", out)
}

func buildPeople(deps Deps, user *models.User, event *models.Event, owner bool, attendees []models.Attendee, responses []models.EventResponse, start, end time.Time, haveWindow bool, cursor string) (Section, *ToolError) {
	byUser := map[string]*models.Response{}
	for _, row := range responses {
		if row.Response != nil {
			if _, dup := byUser[row.UserId]; !dup {
				byUser[row.UserId] = row.Response
			}
		}
	}
	statusOf := func(r *models.Response) string {
		if len(r.Availability)+len(r.IfNeeded) > 0 {
			return "responded"
		}
		return "cleared"
	}
	rangesIn := func(marks []primitive.DateTime) []TimeRange {
		if !haveWindow {
			return []TimeRange{}
		}
		ts := []time.Time{}
		for _, m := range marks {
			if t := m.Time().UTC(); !t.Before(start) && t.Before(end) {
				ts = append(ts, t)
			}
		}
		return mergeMarks(ts, increment(event))
	}
	people := []SnapshotPerson{}
	used := map[string]bool{}
	add := func(p SnapshotPerson, r *models.Response, email string) {
		if owner && email != "" {
			p.Email = email
		}
		p.Available, p.IfNeeded = []TimeRange{}, []TimeRange{}
		if r != nil && p.Status != "declined" {
			p.Available, p.IfNeeded = rangesIn(r.Availability), rangesIn(r.IfNeeded)
		}
		people = append(people, p)
	}
	for _, a := range attendees {
		p := SnapshotPerson{Key: opaqueKey(event.Id, "a", strings.ToLower(strings.TrimSpace(a.Email))), Status: "invited", IsMe: a.Email == user.Email}
		if a.Declined != nil && *a.Declined {
			p.Status = "declined"
		}
		var resp *models.Response
		if deps.Users != nil {
			if account := deps.Users.GetByEmail(a.Email); account != nil {
				p.Name = strings.TrimSpace(account.FirstName + " " + account.LastName)
				if r := byUser[account.Id.Hex()]; r != nil {
					used[account.Id.Hex()] = true
					if p.Status != "declined" {
						p.Status = statusOf(r)
						resp = r
					}
				}
			}
		}
		add(p, resp, a.Email)
	}
	if event.Type != models.GROUP {
		for id, r := range byUser {
			if used[id] {
				continue
			}
			p := SnapshotPerson{Key: opaqueKey(event.Id, "r", id), Name: displayName(deps, r), Status: statusOf(r), IsMe: id == user.Id.Hex()}
			add(p, r, personEmail(r))
		}
	}
	sort.Slice(people, func(i, j int) bool {
		a, b := strings.ToLower(people[i].Name), strings.ToLower(people[j].Name)
		if a != b {
			return a < b
		}
		return people[i].Key < people[j].Key
	})
	totals := map[string]int{}
	for _, p := range people {
		totals[p.Status]++
	}
	scope := peopleScope(user, event, start, end, haveWindow)
	offset, ok := decodePeopleCursor(cursor, scope)
	if !ok || offset > len(people) {
		return Section{}, toolErr("BAD_ARGUMENTS", "people_cursor is not valid for this view.", "Omit people_cursor to start from the first page.")
	}
	page := people[offset:]
	data := &SnapshotPeople{Total: len(people), Totals: totals, Complete: true}
	sec := Section{Status: SectionComplete}
	if len(page) > MaxSnapshotPeople {
		page = page[:MaxSnapshotPeople]
	}
	if offset+len(page) < len(people) {
		next := encodePeopleCursor(offset+len(page), scope)
		sec.NextCursor = &next
	}
	if offset > 0 || sec.NextCursor != nil {
		data.Complete = false
		sec.Status, sec.Reason = SectionPartial, "page"
	}
	data.Items = page
	sec.Data = data
	return sec, nil
}

func peopleScope(user *models.User, event *models.Event, start, end time.Time, haveWindow bool) string {
	sum := sha256.Sum256([]byte(user.Id.Hex() + "|" + event.Id.Hex() + "|" + strconv.FormatBool(haveWindow) + start.UTC().Format(time.RFC3339) + end.UTC().Format(time.RFC3339)))
	return hex.EncodeToString(sum[:6])
}

func encodePeopleCursor(offset int, scope string) string {
	return "pc1." + strconv.Itoa(offset) + "." + scope
}

func decodePeopleCursor(cursor, scope string) (int, bool) {
	if cursor == "" {
		return 0, true
	}
	parts := strings.Split(cursor, ".")
	if len(parts) != 3 || parts[0] != "pc1" || parts[2] != scope {
		return 0, false
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// readOwnAnswer finds the caller's own row by authenticated identity. Zero rows is absent,
// one is unique, and more than one is an error. It copies before anything can filter or edit.
func readOwnAnswer(deps Deps, user *models.User, event *models.Event, attendees []models.Attendee, responses []models.EventResponse, start, end time.Time, haveWindow, signUp bool) (Section, *ToolError) {
	if signUp {
		return sectionOf(SectionUnavailable, "sign_up_form", nil), nil
	}
	var rows []models.EventResponse
	for _, row := range responses {
		if row.UserId == user.Id.Hex() {
			rows = append(rows, row)
		}
	}
	if len(rows) > 1 {
		return Section{}, toolErr("DUPLICATE_OWN_RESPONSE", "More than one stored answer exists for you on this muster.", "Open the muster on the website.")
	}
	isAttendee, declined := false, false
	for _, a := range attendees {
		if a.Email == user.Email {
			isAttendee = true
			declined = declined || (a.Declined != nil && *a.Declined)
		}
	}
	var declinedPtr *bool
	if isAttendee {
		declinedPtr = &declined
	}
	out := &OwnAnswer{
		IsAttendee: isAttendee, Declined: declined,
		Available: []string{}, IfNeeded: []string{},
		EditableLayers: []string{"available", "if_needed"}, PreservedLayers: []string{},
		Complete: true, Editable: true,
	}
	flags := ResponseFlags{}
	if len(rows) == 1 {
		r := rows[0].Response
		if r == nil {
			return Section{}, toolErr("OWN_RESPONSE_UNREADABLE", "Your stored answer could not be read.", "Open the muster on the website.")
		}
		cp := *r // own copy, taken before any filter
		avail := append([]primitive.DateTime(nil), cp.Availability...)
		need := append([]primitive.DateTime(nil), cp.IfNeeded...)
		out.Exists = true
		flags = ResponseFlags{Exists: true, NonEmpty: len(avail)+len(need) > 0}
		out.Available, out.AvailableCounts = ownMarks(event, avail, start, end, haveWindow)
		out.IfNeeded, out.IfNeededCounts = ownMarks(event, need, start, end, haveWindow)
		if cp.ManualAvailability != nil && len(*cp.ManualAvailability) > 0 {
			out.ManualDays = len(*cp.ManualAvailability)
			out.PreservedLayers = append(out.PreservedLayers, "manual_availability")
		}
		if len(avail) > maxMarks || len(need) > maxMarks {
			out.Complete, out.Editable = false, false
		}
		if out.AvailableCounts.OffGrid+out.IfNeededCounts.OffGrid > 0 {
			out.PreservedLayers = append(out.PreservedLayers, "off_grid_marks")
		}
		if out.ManualDays > 0 || out.AvailableCounts.OffGrid+out.IfNeededCounts.OffGrid > 0 {
			// Preserved layers stay in the stored answer; the form only edits the two grid layers.
			out.EditableLayers = []string{"available", "if_needed"}
		}
	}
	out.MyStatus = MyStatus(flags, isAttendee, declinedPtr)
	out.TargetFingerprint = fingerprint("own", event.Id.Hex(), out.Exists, out.Available, out.IfNeeded, out.Declined, out.ManualDays)
	if !out.Complete {
		// Too large for the safe path: report why, and send no editable form.
		out.Available, out.IfNeeded = []string{}, []string{}
		return sectionOf(SectionPartial, "own_answer_too_large", out), nil
	}
	return sectionOf(SectionComplete, "", out), nil
}

func ownMarks(event *models.Event, marks []primitive.DateTime, start, end time.Time, haveWindow bool) ([]string, OwnCounts) {
	ts := make([]time.Time, 0, len(marks))
	for _, m := range marks {
		ts = append(ts, m.Time().UTC())
	}
	sort.Slice(ts, func(i, j int) bool { return ts[i].Before(ts[j]) })
	out := make([]string, 0, len(ts))
	var c OwnCounts
	for _, t := range ts {
		out = append(out, t.Format(time.RFC3339))
		if haveWindow && !t.Before(start) && t.Before(end) {
			c.InWindow++
		} else {
			c.OffWindow++
		}
		if slotProblem(event, t) != "" {
			c.OffGrid++
		}
	}
	return out, c
}
