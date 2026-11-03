package mcp

import (
	"testing"
	"time"

	"github.com/shivasymbl/muster/server/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// statefulEvents is a route-level fake that persists the answer and does a
// real compare against the snapshot. It is NOT evidence for Mongo behavior;
// that is db.TestMCPAvailabilityCAS.
type statefulEvents struct {
	decisionEvents
	user   primitive.ObjectID
	writes int
	// beforeWrite runs after the caller's read and before the compare.
	beforeWrite func()
}

func (f *statefulEvents) ReplaceAvailabilityIf(_ *models.User, _ primitive.ObjectID, snap AnswerSnapshot, a, n []primitive.DateTime) (bool, error) {
	if f.beforeWrite != nil {
		f.beforeWrite()
		f.beforeWrite = nil
	}
	cur := callerSnapshot(Deps{Events: f}, &models.User{Id: f.user}, primitive.NilObjectID)
	if cur.Exists != snap.Exists || !sameInstants(cur.Availability, snap.Availability) || !sameInstants(cur.IfNeeded, snap.IfNeeded) {
		return false, nil
	}
	f.writes++
	f.set(a, n)
	return true, nil
}

func (f *statefulEvents) ReplaceAvailability(_ *models.User, _ primitive.ObjectID, a, n []primitive.DateTime) error {
	f.set(a, n)
	return nil
}

func (f *statefulEvents) set(a, n []primitive.DateTime) {
	f.answers = []models.EventResponse{{UserId: f.user.Hex(), Response: &models.Response{Availability: a, IfNeeded: n}}}
}

func at(h, m int) primitive.DateTime {
	return primitive.NewDateTimeFromTime(time.Date(2018, 6, 18, h, m, 0, 0, time.UTC))
}

func newWeekly() (*statefulEvents, Deps, *models.User) {
	user := &models.User{Id: primitive.NewObjectID()}
	hours := float32(24)
	slot := 30
	ev := &models.Event{Id: primitive.NewObjectID(), OwnerId: user.Id, Type: models.DOW,
		Dates: []primitive.DateTime{at(0, 0)}, Duration: &hours, TimeIncrement: &slot}
	f := &statefulEvents{decisionEvents: decisionEvents{event: ev}, user: user.Id}
	return f, Deps{Events: f}, user
}

func call(t *testing.T, d Deps, u *models.User, f *statefulEvents, args map[string]any) (map[string]any, *ToolError) {
	t.Helper()
	args["event_id"] = f.event.Id.Hex()
	out, bad, _ := Call(d, u, "muster_set_availability", args)
	if bad != nil {
		return nil, bad
	}
	return out.(map[string]any), nil
}

func rng(start, end, state string) map[string]any {
	return map[string]any{"start": start, "end": end, "state": state}
}

func TestMCPTextParity(t *testing.T) {
	t.Run("REQ_TXT_001", func(t *testing.T) {
		// Receipt echoes the stored answer read back, with counts kept as numbers.
		f, d, u := newWeekly()
		out, bad := call(t, d, u, f, map[string]any{"ranges": []any{rng("2018-06-18T15:00:00Z", "2018-06-18T16:00:00Z", "available")}})
		if bad != nil {
			t.Fatal(bad.Message)
		}
		if out["availability"] != 2 || out["changed"] != true || out["echo_complete"] != true {
			t.Fatalf("%#v", out)
		}
		views := out["answer"].(map[string]any)["availability"].([]SlotView)
		if len(views) != 2 || views[0].Instant != "2018-06-18T15:00:00Z" {
			t.Fatalf("%#v", views)
		}
		if out["state_fingerprint"] == "" || out["window"] == nil {
			t.Fatalf("%#v", out)
		}
	})
	t.Run("REQ_TXT_003", func(t *testing.T) {
		t.Run("replace_default_and_merge", func(t *testing.T) {
			f, d, u := newWeekly()
			f.set([]primitive.DateTime{at(9, 0)}, nil)
			out, bad := call(t, d, u, f, map[string]any{"ranges": []any{rng("2018-06-18T15:00:00Z", "2018-06-18T15:30:00Z", "available")}})
			if bad != nil || out["availability"] != 1 {
				t.Fatalf("replace %v %#v", bad, out)
			}
			out, bad = call(t, d, u, f, map[string]any{"mode": "merge", "ranges": []any{rng("2018-06-18T16:00:00Z", "2018-06-18T16:30:00Z", "if_needed")}})
			if bad != nil || out["availability"] != 1 || out["if_needed"] != 1 {
				t.Fatalf("merge %v %#v", bad, out)
			}
			out, bad = call(t, d, u, f, map[string]any{"mode": "merge", "ranges": []any{rng("2018-06-18T15:00:00Z", "2018-06-18T15:30:00Z", "unavailable")}})
			if bad != nil || out["availability"] != 0 || out["if_needed"] != 1 {
				t.Fatalf("unavailable %v %#v", bad, out)
			}
		})
		t.Run("exclusive_with_legacy_lists_even_null", func(t *testing.T) {
			f, d, u := newWeekly()
			for _, key := range []string{"availability", "if_needed", "availability_ranges"} {
				_, bad := call(t, d, u, f, map[string]any{"ranges": []any{}, key: nil})
				if bad == nil || bad.Code != "BAD_ARGUMENTS" {
					t.Fatalf("%s: %#v", key, bad)
				}
			}
			_, bad := call(t, d, u, f, map[string]any{"mode": "merge", "availability": []any{}})
			if bad == nil || bad.Code != "BAD_ARGUMENTS" {
				t.Fatalf("merge without ranges: %#v", bad)
			}
		})
		t.Run("validation", func(t *testing.T) {
			f, d, u := newWeekly()
			cases := map[string][]any{
				"empty span": {rng("2018-06-18T15:00:00Z", "2018-06-18T15:00:00Z", "available")},
				"bad state":  {rng("2018-06-18T15:00:00Z", "2018-06-18T16:00:00Z", "maybe")},
				"conflict":   {rng("2018-06-18T15:00:00Z", "2018-06-18T16:00:00Z", "available"), rng("2018-06-18T15:30:00Z", "2018-06-18T16:30:00Z", "if_needed")},
				"off grid":   {rng("2018-06-18T15:10:00Z", "2018-06-18T16:00:00Z", "available")},
				"outside":    {rng("2018-06-19T15:00:00Z", "2018-06-19T16:00:00Z", "available")},
			}
			for name, ranges := range cases {
				_, bad := call(t, d, u, f, map[string]any{"ranges": ranges})
				if bad == nil {
					t.Fatalf("%s accepted", name)
				}
			}
			tooMany := make([]any, 101)
			for i := range tooMany {
				tooMany[i] = rng("2018-06-18T15:00:00Z", "2018-06-18T15:30:00Z", "available")
			}
			if _, bad := call(t, d, u, f, map[string]any{"ranges": tooMany}); bad == nil {
				t.Fatal("101 ranges accepted")
			}
			if f.writes != 0 {
				t.Fatalf("invalid input wrote %d times", f.writes)
			}
		})
		t.Run("fingerprint", func(t *testing.T) {
			f, d, u := newWeekly()
			first, _ := call(t, d, u, f, map[string]any{"ranges": []any{rng("2018-06-18T15:00:00Z", "2018-06-18T15:30:00Z", "available")}})
			fp := first["state_fingerprint"].(string)
			next := []any{rng("2018-06-18T16:00:00Z", "2018-06-18T16:30:00Z", "available")}
			if _, bad := call(t, d, u, f, map[string]any{"ranges": next, "mode": "merge", "expected_state_fingerprint": "stale"}); bad == nil || bad.Code != "CONFLICT" {
				t.Fatalf("%#v", bad)
			}
			if _, bad := call(t, d, u, f, map[string]any{"ranges": next, "mode": "merge", "expected_state_fingerprint": fp}); bad != nil {
				t.Fatal(bad.Message)
			}
		})
		t.Run("lost_compare_is_conflict_not_overwrite", func(t *testing.T) {
			f, d, u := newWeekly()
			f.set([]primitive.DateTime{at(9, 0)}, nil)
			f.beforeWrite = func() { f.set([]primitive.DateTime{at(10, 0)}, nil) } // concurrent writer
			_, bad := call(t, d, u, f, map[string]any{"mode": "merge", "ranges": []any{rng("2018-06-18T15:00:00Z", "2018-06-18T15:30:00Z", "available")}})
			if bad == nil || bad.Code != "CONFLICT" {
				t.Fatalf("%#v", bad)
			}
			if got := f.answers[0].Response.Availability; len(got) != 1 || got[0] != at(10, 0) {
				t.Fatalf("concurrent write was overwritten: %v", got)
			}
		})
		t.Run("repeat_is_noop", func(t *testing.T) {
			f, d, u := newWeekly()
			args := func() map[string]any {
				return map[string]any{"ranges": []any{rng("2018-06-18T15:00:00Z", "2018-06-18T16:00:00Z", "available")}}
			}
			call(t, d, u, f, args())
			out, bad := call(t, d, u, f, args())
			if bad != nil || out["changed"] != false || f.writes != 1 {
				t.Fatalf("%v %#v writes=%d", bad, out, f.writes)
			}
		})
		t.Run("legacy_lists_stay_unconditional", func(t *testing.T) {
			f, d, u := newWeekly()
			f.set([]primitive.DateTime{at(9, 0)}, nil)
			f.casMiss = true // a conditional store would refuse; the legacy path must not use it
			legacy := &legacyProbe{statefulEvents: f}
			d.Events = legacy
			_, bad := call(t, d, u, f, map[string]any{"availability": []any{"2018-06-18T15:00:00Z"}, "if_needed": []any{}})
			if bad != nil || !legacy.unconditional {
				t.Fatalf("%v unconditional=%v", bad, legacy.unconditional)
			}
		})
	})
	t.Run("REQ_TXT_009", func(t *testing.T) {
		// Owner decision: omitting both lists stays a validation error, not a no-op.
		f, d, u := newWeekly()
		for _, args := range []map[string]any{{}, {"availability": nil, "if_needed": nil}} {
			_, bad := call(t, d, u, f, args)
			if bad == nil || bad.Code != "BAD_ARGUMENTS" {
				t.Fatalf("%#v", bad)
			}
		}
		out, bad := call(t, d, u, f, map[string]any{"availability": []any{}})
		if bad != nil || out["changed"] != false {
			t.Fatalf("explicit empty replace %v %#v", bad, out)
		}
	})
	t.Run("REQ_TXT_013", func(t *testing.T) {
		f, d, u := newWeekly()
		out, bad := call(t, d, u, f, map[string]any{"availability": []any{map[string]any{"weekday": "Monday", "time": "15:00"}}, "if_needed": []any{}})
		if bad != nil {
			t.Fatal(bad.Message)
		}
		views := out["answer"].(map[string]any)["availability"].([]SlotView)
		if len(views) != 1 || views[0] != (SlotView{Instant: "2018-06-18T15:00:00Z", Weekday: "Monday", Time: "15:00"}) {
			t.Fatalf("%#v", views)
		}
	})
}

type legacyProbe struct {
	*statefulEvents
	unconditional bool
}

func (p *legacyProbe) ReplaceAvailability(_ *models.User, _ primitive.ObjectID, a, n []primitive.DateTime) error {
	p.unconditional = true
	p.set(a, n)
	return nil
}

func TestMCPFieldFindings(t *testing.T) {
	// FE1: the set receipt gains an authoritative stored-answer echo; old count fields stay numbers.
	t.Run("FE1", func(t *testing.T) {
		f, d, u := newWeekly()
		out, bad := call(t, d, u, f, map[string]any{"availability": []any{"2018-06-18T15:00:00Z"}, "if_needed": []any{}})
		if bad != nil {
			t.Fatal(bad.Message)
		}
		if _, ok := out["availability"].(int); !ok {
			t.Fatalf("count type changed: %#v", out["availability"])
		}
		if out["replaced"] != true || out["duplicates_ignored"] != 0 || out["answer"] == nil || out["echo_complete"] != true {
			t.Fatalf("%#v", out)
		}
	})
	// FE1 readback honesty: a store that does not persist must not be reported complete.
	t.Run("FE1_readback_mismatch_is_incomplete", func(t *testing.T) {
		f, d, u := newWeekly()
		d.Events = &dropsWrites{statefulEvents: f}
		out, bad := call(t, d, u, f, map[string]any{"availability": []any{"2018-06-18T15:00:00Z"}, "if_needed": []any{}})
		if bad != nil || out["echo_complete"] != false {
			t.Fatalf("%v %#v", bad, out)
		}
	})
	// FE3: ranges and mode are accepted; covered in depth by REQ_TXT_003.
	t.Run("FE3", func(t *testing.T) {
		f, d, u := newWeekly()
		out, bad := call(t, d, u, f, map[string]any{"ranges": []any{rng("2018-06-18T15:00:00Z", "2018-06-18T16:00:00Z", "if_needed")}})
		if bad != nil || out["if_needed"] != 2 {
			t.Fatalf("%v %#v", bad, out)
		}
	})
	// FE9 (owner decision: omit-both stays validation_error).
	t.Run("FE9", func(t *testing.T) {
		f, d, u := newWeekly()
		_, bad := call(t, d, u, f, map[string]any{})
		if bad == nil || bad.Code != "BAD_ARGUMENTS" || f.writes != 0 {
			t.Fatalf("%#v", bad)
		}
	})
	// W-series preservation for what this change touches: legacy replace writes and keeps its receipt keys.
	t.Run("W_legacy_receipt_keys", func(t *testing.T) {
		f, d, u := newWeekly()
		out, _ := call(t, d, u, f, map[string]any{"availability": []any{"2018-06-18T15:00:00Z", "2018-06-18T15:00:00Z"}, "if_needed": []any{}})
		for _, k := range []string{"event_id", "availability", "if_needed", "duplicates_ignored", "replaced"} {
			if _, ok := out[k]; !ok {
				t.Fatalf("missing %s", k)
			}
		}
		if out["duplicates_ignored"] != 1 {
			t.Fatalf("%#v", out)
		}
	})
}

type dropsWrites struct{ *statefulEvents }

func (p *dropsWrites) ReplaceAvailability(*models.User, primitive.ObjectID, []primitive.DateTime, []primitive.DateTime) error {
	return nil
}
