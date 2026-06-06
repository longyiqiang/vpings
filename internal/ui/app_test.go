package ui

import (
	"context"
	"testing"
	"time"

	"github.com/longyiqiang/vpings/internal/appconfig"
	"github.com/longyiqiang/vpings/internal/probe"
)

func TestProbeDefaultsFormValue(t *testing.T) {
	form := newProbeDefaultsForm(appconfig.Config{
		ProbeInterval:         appconfig.DefaultProbeInterval,
		DefaultTimeout:        appconfig.DefaultProbeTimeout,
		DefaultSampleCount:    appconfig.DefaultSampleCount,
		DefaultSampleInterval: appconfig.DefaultSampleInterval,
	})
	form.fields[0].value = "60"
	form.fields[1].value = "3"
	form.fields[2].value = "10"
	form.fields[3].value = "0"
	form.fields[4].value = "true"

	value, err := form.value()
	if err != nil {
		t.Fatal(err)
	}
	if value.probeInterval != 60*time.Second {
		t.Fatalf("probeInterval = %s, want 60s", value.probeInterval)
	}
	if value.defaultTimeout != 3*time.Second {
		t.Fatalf("defaultTimeout = %s, want 3s", value.defaultTimeout)
	}
	if value.sampleCount != 10 {
		t.Fatalf("sampleCount = %d, want 10", value.sampleCount)
	}
	if value.sampleInterval != 0 {
		t.Fatalf("sampleInterval = %s, want 0s", value.sampleInterval)
	}
	if !value.applyExisting {
		t.Fatal("applyExisting = false, want true")
	}
}

func TestNextRoundDelayUsesRoundStart(t *testing.T) {
	start := time.Date(2026, 5, 26, 10, 0, 0, 0, time.UTC)
	results := []probe.Result{
		{StartedAt: start.Add(9 * time.Second)},
		{StartedAt: start},
		{StartedAt: start.Add(5 * time.Second)},
	}

	if got := nextRoundDelay(results, 10*time.Second, start.Add(9*time.Second)); got != time.Second {
		t.Fatalf("delay before interval elapsed = %s, want 1s", got)
	}
	if got := nextRoundDelay(results, 10*time.Second, start.Add(11*time.Second)); got != 0 {
		t.Fatalf("delay after interval elapsed = %s, want 0", got)
	}
}

func TestRunConfiguredProbeRoundRunsProbesConcurrently(t *testing.T) {
	probes := []appconfig.ProbeConfig{
		{ID: "a", Name: "A", Protocol: probe.ProtocolICMP, Host: "a.example", SampleCount: 1, Timeout: time.Second},
		{ID: "b", Name: "B", Protocol: probe.ProtocolICMP, Host: "b.example", SampleCount: 1, Timeout: time.Second},
	}
	started := make(chan string, len(probes))
	release := make(chan struct{})
	done := make(chan []probe.Result, 1)
	runner := func(_ context.Context, spec probe.Spec) probe.Result {
		started <- spec.ID
		<-release
		return probe.Result{StartedAt: time.Now(), Protocol: spec.Protocol, Host: spec.Host, Status: probe.StatusOK}
	}

	go func() {
		done <- runConfiguredProbeRound(probes, runner)
	}()

	seen := map[string]bool{}
	for len(seen) < len(probes) {
		select {
		case id := <-started:
			seen[id] = true
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("probes did not start concurrently, saw %v", seen)
		}
	}
	close(release)

	select {
	case results := <-done:
		if len(results) != 2 {
			t.Fatalf("len(results) = %d, want 2", len(results))
		}
		if results[0].ProbeID != "a" || results[1].ProbeID != "b" {
			t.Fatalf("results order = %s, %s; want a, b", results[0].ProbeID, results[1].ProbeID)
		}
		if results[0].RoundID == "" || results[0].RoundID != results[1].RoundID {
			t.Fatalf("RoundID = %q, %q; want one shared round id", results[0].RoundID, results[1].RoundID)
		}
	case <-time.After(time.Second):
		t.Fatal("probe round did not finish")
	}
}
