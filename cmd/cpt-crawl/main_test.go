package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/joshp123/cpt-crawl/internal/combustion"
)

func TestRequireSelectedProbesRequiresEveryRequestedSerial(t *testing.T) {
	err := requireSelectedProbes([]combustion.Probe{{Serial: "A"}}, "A,B")
	if err == nil || !strings.Contains(err.Error(), "B") {
		t.Fatalf("requireSelectedProbes() = %v, want missing B", err)
	}
}

func TestRequireSelectedProbesAllowsAllMatches(t *testing.T) {
	err := requireSelectedProbes([]combustion.Probe{{Serial: "B"}, {Serial: "A"}}, "A,B")
	if err != nil {
		t.Fatalf("requireSelectedProbes() = %v, want nil", err)
	}
}

func TestWatchRecordsEmitsLatestThenOnlyNewRows(t *testing.T) {
	seen := map[watchKey]bool{}
	last := map[watchKey]int{}
	snapshot := combustion.WindowSnapshot{
		GeneratedAt: "2026-06-01T12:00:00Z",
		Probes: []combustion.WindowProbe{{
			Serial:    "probe-a",
			SessionID: 123,
			Rows: []combustion.DataRow{
				{SampledAt: "2026-06-01T11:59:50Z", SequenceNumber: 10},
				{SampledAt: "2026-06-01T11:59:55Z", SequenceNumber: 11},
			},
			Latest: &combustion.DataRow{SampledAt: "2026-06-01T11:59:55Z", SequenceNumber: 11},
		}},
	}

	first := watchRecords(snapshot, seen, last)
	if len(first) != 1 || first[0].SequenceNumber != 11 {
		t.Fatalf("first watchRecords() = %#v, want latest sequence 11 only", first)
	}

	again := watchRecords(snapshot, seen, last)
	if len(again) != 0 {
		t.Fatalf("second watchRecords() = %#v, want no duplicate rows", again)
	}

	snapshot.Probes[0].Rows = append(snapshot.Probes[0].Rows, combustion.DataRow{SampledAt: "2026-06-01T12:00:00Z", SequenceNumber: 12})
	snapshot.Probes[0].Latest = &snapshot.Probes[0].Rows[2]
	next := watchRecords(snapshot, seen, last)
	if len(next) != 1 || next[0].SequenceNumber != 12 {
		t.Fatalf("third watchRecords() = %#v, want new sequence 12 only", next)
	}
}

func TestWatchLookbackCoversSlowIntervals(t *testing.T) {
	if got := watchLookbackMinutes(30 * time.Second); got != 2 {
		t.Fatalf("watchLookbackMinutes(30s) = %v, want 2", got)
	}
	if got := watchLookbackMinutes(5 * time.Minute); got != 15 {
		t.Fatalf("watchLookbackMinutes(5m) = %v, want 15", got)
	}
}

func TestDurationSecondsValueAcceptsWatchStyleBareSeconds(t *testing.T) {
	d := 30 * time.Second
	v := durationSecondsValue{target: &d}
	if err := v.Set("5"); err != nil {
		t.Fatal(err)
	}
	if d != 5*time.Second {
		t.Fatalf("duration = %v, want 5s", d)
	}
	if err := v.Set("250ms"); err != nil {
		t.Fatal(err)
	}
	if d != 250*time.Millisecond {
		t.Fatalf("duration = %v, want 250ms", d)
	}
}

func TestIsBrokenPipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	err = json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	if closeErr := w.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if !isBrokenPipe(err) {
		t.Fatalf("isBrokenPipe(%v) = false, want true", err)
	}
}
