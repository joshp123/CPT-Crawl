package combustion

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestExpectedAndMissingRows(t *testing.T) {
	ranges := [][]int{{0, 2}, {5, 6}}
	rows := []DataRow{{SequenceNumber: 0}, {SequenceNumber: 2}, {SequenceNumber: 5}, {SequenceNumber: 6}}

	if got := ExpectedCount(ranges); got != 5 {
		t.Fatalf("ExpectedCount() = %d, want 5", got)
	}

	missing := Missing(ranges, rows)
	if len(missing) != 1 || missing[0] != 1 {
		t.Fatalf("Missing() = %#v, want [1]", missing)
	}
}

func TestLatestRangeUsesSamplePeriod(t *testing.T) {
	got := latestRange([][]int{{0, 999}}, 5000, 5)
	want := [2]int{928, 999}
	if got != want {
		t.Fatalf("latestRange() = %#v, want %#v", got, want)
	}
}

func TestLatestRangesSpanOlderRanges(t *testing.T) {
	got := latestRanges([][]int{{0, 99}, {200, 204}}, 5000, 1)
	want := [][]int{{81, 99}, {200, 204}}
	if fmtRanges(got) != fmtRanges(want) {
		t.Fatalf("latestRanges() = %#v, want %#v", got, want)
	}
}

func TestLatestRangesEmpty(t *testing.T) {
	if got := latestRanges(nil, 5000, 1); got != nil {
		t.Fatalf("latestRanges() = %#v, want nil", got)
	}
}

func TestSequenceInRanges(t *testing.T) {
	if !sequenceInRanges([][]int{{2, 4}}, 3) {
		t.Fatal("sequenceInRanges() = false, want true")
	}
	if sequenceInRanges([][]int{{2, 4}}, 5) {
		t.Fatal("sequenceInRanges() = true, want false")
	}
}

func fmtRanges(ranges [][]int) string {
	return fmt.Sprintf("%v", ranges)
}

func TestDecodeSessionPreservesUnknownSourceFields(t *testing.T) {
	raw := json.RawMessage(`{"device_session_id":"42","started_at":"2026-06-01T00:00:00Z","unexpected_label":"brisket"}`)
	session, err := decodeSession(raw)
	if err != nil {
		t.Fatal(err)
	}
	if session.Source["unexpected_label"] != "brisket" {
		t.Fatalf("unexpected_label = %#v, want brisket", session.Source["unexpected_label"])
	}
}

func TestURLWithoutQueryRemovesPrivateParameters(t *testing.T) {
	got := urlWithoutQuery("https://data-api.combustion.inc/v1/session_data?uid=user&device_serial_number=serial&device_session_id=123")
	want := "https://data-api.combustion.inc/v1/session_data"
	if got != want {
		t.Fatalf("urlWithoutQuery() = %q, want %q", got, want)
	}
}
