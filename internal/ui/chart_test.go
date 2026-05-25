package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/longyiqiang/vpings/internal/appconfig"
	"github.com/longyiqiang/vpings/internal/probe"
)

func TestRenderProbeDetailCharts(t *testing.T) {
	now := time.Now()
	item := appconfig.ProbeConfig{
		ID:       "tcp-example-443",
		Name:     "Example TCP",
		Protocol: probe.ProtocolTCP,
		Host:     "example.com",
		Port:     443,
	}
	results := []probe.Result{
		{StartedAt: now, RoundID: "round-1", ProbeID: item.ID, ProbeName: item.Name, Protocol: item.Protocol, Host: item.Host, Port: item.Port, Status: probe.StatusOK, Duration: 10 * time.Millisecond, Attempt: 1, Attempts: 3},
		{StartedAt: now.Add(100 * time.Millisecond), RoundID: "round-1", ProbeID: item.ID, ProbeName: item.Name, Protocol: item.Protocol, Host: item.Host, Port: item.Port, Status: probe.StatusOK, Duration: 20 * time.Millisecond, Attempt: 2, Attempts: 3},
		{StartedAt: now.Add(200 * time.Millisecond), RoundID: "round-1", ProbeID: item.ID, ProbeName: item.Name, Protocol: item.Protocol, Host: item.Host, Port: item.Port, Status: probe.StatusFailed, Duration: time.Second, Attempt: 3, Attempts: 3},
	}

	chart := RenderProbeDetailCharts(item, results, now, time.Minute)
	if !strings.Contains(chart, "median 15.0ms") {
		t.Fatalf("expected chart caption, got:\n%s", chart)
	}
	if !strings.Contains(chart, "loss 33%") {
		t.Fatalf("expected loss summary, got:\n%s", chart)
	}
}

func TestAlignWindowSummariesUsesRoundIntervalSlots(t *testing.T) {
	start := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	summaries := []probeSummary{
		{StartedAt: start, Received: 1, Attempts: 1, MedianMS: 10, MinMS: 10, MaxMS: 10},
		{StartedAt: start.Add(2 * time.Minute), Received: 1, Attempts: 1, MedianMS: 30, MinMS: 30, MaxMS: 30},
	}

	aligned := alignWindowSummaries(summaries, time.Minute, start, start.Add(2*time.Minute))
	if len(aligned) != 3 {
		t.Fatalf("len(aligned) = %d, want 3", len(aligned))
	}
	if aligned[1].Received != 0 {
		t.Fatalf("middle slot Received = %d, want 0 for missing round", aligned[1].Received)
	}
	if aligned[2].MedianMS != 30 {
		t.Fatalf("last slot MedianMS = %.1f, want 30", aligned[2].MedianMS)
	}
}
