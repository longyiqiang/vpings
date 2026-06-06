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

	chart := RenderProbeDetailCharts(item, results, now, defaultDetailChartWindows(), detailChartRealtime)
	if !strings.Contains(chart, "█") {
		t.Fatalf("expected latency bars, got:\n%s", chart)
	}
	if !strings.Contains(chart, "·") {
		t.Fatalf("expected failed sample marker, got:\n%s", chart)
	}
	if !strings.Contains(chart, "realtime") {
		t.Fatalf("expected chart name caption, got:\n%s", chart)
	}
	if strings.Contains(chart, "•") || strings.Contains(chart, "median 15.0ms") || strings.Contains(chart, "loss 33%") || strings.Contains(chart, "latest") {
		t.Fatalf("expected chart footer to keep only chart names, got:\n%s", chart)
	}
}

func TestSummariesForProbeKeepsEachRoundAsPoint(t *testing.T) {
	start := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	item := appconfig.ProbeConfig{ID: "tcp-example-443", Protocol: probe.ProtocolTCP, Host: "example.com", Port: 443}
	results := []probe.Result{
		{StartedAt: start, RoundID: "round-1", ProbeID: item.ID, Protocol: item.Protocol, Host: item.Host, Port: item.Port, Status: probe.StatusOK, Duration: 10 * time.Millisecond, Attempts: 1},
		{StartedAt: start.Add(2 * time.Minute), RoundID: "round-2", ProbeID: item.ID, Protocol: item.Protocol, Host: item.Host, Port: item.Port, Status: probe.StatusOK, Duration: 30 * time.Millisecond, Attempts: 1},
	}

	summaries := summariesForProbe(item, results, 0, start.Add(-time.Minute))
	if len(summaries) != 2 {
		t.Fatalf("len(summaries) = %d, want 2", len(summaries))
	}
	if summaries[0].AverageMS != 10 {
		t.Fatalf("first AverageMS = %.1f, want 10", summaries[0].AverageMS)
	}
	if summaries[1].AverageMS != 30 {
		t.Fatalf("second AverageMS = %.1f, want 30", summaries[1].AverageMS)
	}
}

func TestSummariesForProbeDoesNotMergeSharedRoundAcrossProbes(t *testing.T) {
	start := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	item := appconfig.ProbeConfig{ID: "probe-a", Protocol: probe.ProtocolTCP, Host: "a.example", Port: 443}
	results := []probe.Result{
		{StartedAt: start, RoundID: "round-1", ProbeID: "probe-a", Protocol: probe.ProtocolTCP, Host: "a.example", Port: 443, Status: probe.StatusOK, Duration: 10 * time.Millisecond, Attempts: 1},
		{StartedAt: start, RoundID: "round-1", ProbeID: "probe-b", Protocol: probe.ProtocolTCP, Host: "b.example", Port: 443, Status: probe.StatusOK, Duration: 90 * time.Millisecond, Attempts: 1},
	}

	summaries := summariesForProbe(item, results, 0, start.Add(-time.Minute))
	if len(summaries) != 1 {
		t.Fatalf("len(summaries) = %d, want 1", len(summaries))
	}
	if summaries[0].AverageMS != 10 {
		t.Fatalf("AverageMS = %.1f, want only probe-a latency 10", summaries[0].AverageMS)
	}
}

func TestSummariesForProbeKeepsRoundSamples(t *testing.T) {
	start := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	item := appconfig.ProbeConfig{ID: "tcp-example-443", Protocol: probe.ProtocolTCP, Host: "example.com", Port: 443}
	results := []probe.Result{
		{StartedAt: start.Add(200 * time.Millisecond), RoundID: "round-1", ProbeID: item.ID, Protocol: item.Protocol, Host: item.Host, Port: item.Port, Status: probe.StatusFailed, Attempt: 3, Attempts: 3},
		{StartedAt: start, RoundID: "round-1", ProbeID: item.ID, Protocol: item.Protocol, Host: item.Host, Port: item.Port, Status: probe.StatusOK, Duration: 10 * time.Millisecond, Attempt: 1, Attempts: 3},
		{StartedAt: start.Add(100 * time.Millisecond), RoundID: "round-1", ProbeID: item.ID, Protocol: item.Protocol, Host: item.Host, Port: item.Port, Status: probe.StatusOK, Duration: 30 * time.Millisecond, Attempt: 2, Attempts: 3},
	}

	summaries := summariesForProbe(item, results, 0, start.Add(-time.Minute))
	if len(summaries) != 1 {
		t.Fatalf("len(summaries) = %d, want 1", len(summaries))
	}
	if len(summaries[0].Samples) != 3 {
		t.Fatalf("len(Samples) = %d, want 3", len(summaries[0].Samples))
	}
	if !summaries[0].Samples[0].OK || summaries[0].Samples[0].LatencyMS != 10 {
		t.Fatalf("first sample = %+v, want attempt 1 ok 10ms", summaries[0].Samples[0])
	}
	if summaries[0].Samples[2].OK {
		t.Fatalf("third sample = %+v, want failed attempt marker", summaries[0].Samples[2])
	}
}

func TestSummariesForProbeUsesAverageLatency(t *testing.T) {
	start := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	item := appconfig.ProbeConfig{ID: "tcp-example-443", Protocol: probe.ProtocolTCP, Host: "example.com", Port: 443}
	results := []probe.Result{
		{StartedAt: start, RoundID: "round-1", ProbeID: item.ID, Protocol: item.Protocol, Host: item.Host, Port: item.Port, Status: probe.StatusOK, Duration: 10 * time.Millisecond, Attempts: 3},
		{StartedAt: start.Add(100 * time.Millisecond), RoundID: "round-1", ProbeID: item.ID, Protocol: item.Protocol, Host: item.Host, Port: item.Port, Status: probe.StatusOK, Duration: 20 * time.Millisecond, Attempts: 3},
		{StartedAt: start.Add(200 * time.Millisecond), RoundID: "round-1", ProbeID: item.ID, Protocol: item.Protocol, Host: item.Host, Port: item.Port, Status: probe.StatusOK, Duration: 90 * time.Millisecond, Attempts: 3},
	}

	summaries := summariesForProbe(item, results, 0, start.Add(-time.Minute))
	if len(summaries) != 1 {
		t.Fatalf("len(summaries) = %d, want 1", len(summaries))
	}
	if summaries[0].AverageMS != 40 {
		t.Fatalf("AverageMS = %.1f, want 40", summaries[0].AverageMS)
	}
}

func TestTimeTickLabelsUseSecondsForShortSpans(t *testing.T) {
	start := time.Date(2026, 5, 25, 10, 28, 5, 0, time.UTC)
	summaries := []probeSummary{
		{StartedAt: start},
		{StartedAt: start.Add(25 * time.Second)},
		{StartedAt: start.Add(50 * time.Second)},
		{StartedAt: start.Add(75 * time.Second)},
	}

	labels := timeTickLabelsForCenters(summaries, []int{4, 18, 32, 46}, 56, 4)
	if !strings.Contains(labels, "10:28:05") {
		t.Fatalf("expected seconds in short-span labels, got %q", labels)
	}
	if !strings.Contains(labels, "10:28:30") || !strings.Contains(labels, "10:28:55") {
		t.Fatalf("expected repeated minutes to be disambiguated with seconds, got %q", labels)
	}
}

func TestRenderBarGridMarksEveryRoundCenter(t *testing.T) {
	grid := make([][]chartCell, 3)
	for row := range grid {
		grid[row] = make([]chartCell, 12)
		for col := range grid[row] {
			grid[row][col] = chartCell{text: " "}
		}
	}
	summaries := []probeSummary{
		{StartedAt: time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)},
		{StartedAt: time.Date(2026, 5, 25, 10, 0, 10, 0, time.UTC)},
		{StartedAt: time.Date(2026, 5, 25, 10, 0, 20, 0, time.UTC)},
		{StartedAt: time.Date(2026, 5, 25, 10, 0, 30, 0, time.UTC)},
	}

	chart := renderBarGrid("test", summaries, []int{1, 4, 7, 10}, grid, 1)
	if got := strings.Count(chart, "┬"); got != 4 {
		t.Fatalf("tick count = %d, want 4 in:\n%s", got, chart)
	}
}

func TestZoomDetailWindowBounds(t *testing.T) {
	if got := zoomDetailWindow(detailChartRealtime, 3*time.Hour, true); got != 90*time.Minute {
		t.Fatalf("zoom in realtime = %s, want 1h30m", got)
	}
	if got := zoomDetailWindow(detailChartRealtime, 10*time.Minute, true); got != 10*time.Minute {
		t.Fatalf("min realtime = %s, want 10m", got)
	}
	if got := zoomDetailWindow(detailChart7Days, 24*time.Hour, true); got != 24*time.Hour {
		t.Fatalf("min week = %s, want 24h", got)
	}
	if got := zoomDetailWindow(detailChart7Days, 4*24*time.Hour, false); got != 7*24*time.Hour {
		t.Fatalf("max week = %s, want 168h", got)
	}
}
