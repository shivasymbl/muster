package mcp

import (
	"testing"
	"time"
)

func TestMergeMarksJoinsIncrement(t *testing.T) {
	start := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	got := mergeMarks([]time.Time{start.Add(30 * time.Minute), start, start.Add(15 * time.Minute)}, 15*time.Minute)
	if len(got) != 1 {
		t.Fatalf("ranges: %d", len(got))
	}
	if got[0].Start != start.Format(time.RFC3339) || got[0].End != start.Add(45*time.Minute).Format(time.RFC3339) {
		t.Fatalf("%+v", got[0])
	}
}
