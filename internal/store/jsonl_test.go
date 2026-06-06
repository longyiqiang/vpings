package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/longyiqiang/vpings/internal/probe"
)

func TestJSONLAppendUsesRecordFieldNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records.jsonl")
	store, err := OpenJSONL(path)
	if err != nil {
		t.Fatal(err)
	}
	result := probe.Result{
		StartedAt: time.Date(2026, 5, 26, 11, 30, 0, 0, time.UTC),
		RoundID:   "round-1",
		ProbeID:   "probe-1",
		ProbeName: "Probe 1",
		Attempt:   2,
		Attempts:  10,
		Protocol:  probe.ProtocolTCP,
		Host:      "example.com",
		Port:      443,
		Status:    probe.StatusOK,
		Duration:  1250 * time.Microsecond,
	}
	if err := store.Append(result); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	if _, ok := record["duration_ms"]; !ok {
		t.Fatalf("duration_ms missing in %s", data)
	}
	if _, ok := record["attempt_count"]; !ok {
		t.Fatalf("attempt_count missing in %s", data)
	}
	if _, ok := record["duration"]; ok {
		t.Fatalf("legacy duration field should not be written: %s", data)
	}
	if _, ok := record["attempts"]; ok {
		t.Fatalf("legacy attempts field should not be written: %s", data)
	}
}

func TestReadRecentSupportsLegacyRecordFieldNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records.jsonl")
	line := `{"started_at":"2026-05-26T11:30:00Z","round_id":"round-1","probe_id":"probe-1","attempt":2,"attempts":10,"protocol":"tcp","host":"example.com","port":443,"status":"ok","duration":1250000}` + "\n"
	if err := os.WriteFile(path, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := ReadRecent(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Attempts != 10 {
		t.Fatalf("Attempts = %d, want 10", results[0].Attempts)
	}
	if results[0].Duration != 1250*time.Microsecond {
		t.Fatalf("Duration = %s, want 1.25ms", results[0].Duration)
	}
}
