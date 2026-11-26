package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/longyiqiang/vpings/internal/appconfig"
	"github.com/longyiqiang/vpings/internal/probe"
)

var testProbe = appconfig.ProbeConfig{
	ID:       "tcp-example-443",
	Name:     "Example TCP",
	Protocol: probe.ProtocolTCP,
	Host:     "example.com",
	Port:     443,
}

func testResult(at time.Time, status probe.Status, latency time.Duration) probe.Result {
	return probe.Result{
		StartedAt: at,
		RoundID:   "round-" + at.Format("150405"),
		ProbeID:   testProbe.ID,
		ProbeName: testProbe.Name,
		Protocol:  testProbe.Protocol,
		Host:      testProbe.Host,
		Port:      testProbe.Port,
		Status:    status,
		Duration:  latency,
	}
}

func TestRenderProbeDetailChartsFitsWidth(t *testing.T) {
	now := time.Date(2026, 5, 25, 10, 30, 0, 0, time.Local)
	var results []probe.Result
	for i := 0; i < 180; i++ {
		at := now.Add(-time.Duration(i) * 10 * time.Second)
		results = append(results, testResult(at, probe.StatusOK, time.Duration(10+i%7)*time.Millisecond))
		if i%9 == 0 {
			results = append(results, testResult(at.Add(time.Second), probe.StatusFailed, time.Second))
		}
	}

	layout := detailChartLayout{width: 90, selectedHeight: 8, otherHeight: 3, interval: 10 * time.Second}
	chart := RenderProbeDetailCharts(testProbe, results, now, defaultDetailChartWindows(), detailChartRealtime, layout)
	for _, line := range strings.Split(chart, "\n") {
		if w := lipgloss.Width(line); w > layout.width {
			t.Fatalf("line width %d exceeds %d: %q", w, layout.width, line)
		}
	}
	for _, want := range []string{"▸ realtime", "past 24 hours", "past week", "█", "loss", "┬"} {
		if !strings.Contains(chart, want) {
			t.Fatalf("expected %q in chart:\n%s", want, chart)
		}
	}
}

func TestRenderProbeDetailChartsCollapsesPreviews(t *testing.T) {
	now := time.Date(2026, 5, 25, 10, 30, 0, 0, time.Local)
	results := []probe.Result{testResult(now.Add(-time.Minute), probe.StatusOK, 12*time.Millisecond)}
	layout := detailChartLayout{width: 80, selectedHeight: 5, otherHeight: 0, interval: 10 * time.Second}

	chart := RenderProbeDetailCharts(testProbe, results, now, defaultDetailChartWindows(), detailChart24Hours, layout)
	// legend + 4 blank separators + 3 header-only previews + selected chart (header + 5 rows + loss + axis + labels)
	if got, want := len(strings.Split(chart, "\n")), 1+4+3+9; got != want {
		t.Fatalf("line count = %d, want %d:\n%s", got, want, chart)
	}
}

func TestBuildLatencySeriesBucketsByTime(t *testing.T) {
	now := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	results := []probe.Result{
		testResult(now.Add(-45*time.Second), probe.StatusOK, 10*time.Millisecond),
		testResult(now.Add(-44*time.Second), probe.StatusOK, 30*time.Millisecond),
		testResult(now.Add(-43*time.Second), probe.StatusFailed, time.Second),
		testResult(now.Add(-5*time.Second), probe.StatusFailed, time.Second),
		// another probe must be ignored
		{StartedAt: now.Add(-5 * time.Second), ProbeID: "other", Status: probe.StatusOK, Duration: time.Second},
	}

	series := buildLatencySeries(testProbe, results, now, time.Minute, 10*time.Second, 60)
	if len(series.buckets) != 6 || series.colsPer != 10 || series.bucketDur != 10*time.Second {
		t.Fatalf("layout = %d buckets x %d cols, %s each; want 6 x 10, 10s", len(series.buckets), series.colsPer, series.bucketDur)
	}
	if series.total != 4 || series.lost != 2 {
		t.Fatalf("total/lost = %d/%d, want 4/2", series.total, series.lost)
	}

	var withData []latencyBucket
	for _, bucket := range series.buckets {
		if bucket.total > 0 {
			withData = append(withData, bucket)
		}
	}
	if len(withData) != 2 {
		t.Fatalf("buckets with data = %d, want 2", len(withData))
	}
	if withData[0].median != 20 || withData[0].lossRate() < 0.33 || withData[0].lossRate() > 0.34 {
		t.Fatalf("first bucket median=%.1f loss=%.2f, want 20 and 0.33", withData[0].median, withData[0].lossRate())
	}
	if withData[1].received() != 0 {
		t.Fatalf("second bucket received = %d, want 0", withData[1].received())
	}
}

func TestBucketLayoutCapsAtWidth(t *testing.T) {
	count, dur, cols := bucketLayout(24*time.Hour, 10*time.Second, 100)
	if count != 100 || dur != 864*time.Second || cols != 1 {
		t.Fatalf("24h layout = %d, %s, %d", count, dur, cols)
	}
	count, dur, cols = bucketLayout(5*time.Minute, 10*time.Second, 100)
	if count != 30 || dur != 10*time.Second || cols != 3 {
		t.Fatalf("5m layout = %d, %s, %d", count, dur, cols)
	}
}

func TestBucketColumnDrawsMedianAndSmoke(t *testing.T) {
	scale := latencyScale{rows: 4, rowStep: 10, max: 40}
	bucket := latencyBucket{total: 10, median: 15, p95: 35, latencies: []float64{15}}
	column := bucketColumn(bucket, scale)

	// rows are top-to-bottom: 30-40, 20-30, 10-20, 0-10
	if column[3].glyph != "█" || column[3].fg != lossTones[0].solid {
		t.Fatalf("bottom cell = %+v, want solid block", column[3])
	}
	if column[2].glyph != "▄" || column[2].bg != lossTones[0].smoke {
		t.Fatalf("median cell = %+v, want half block over smoke", column[2])
	}
	if column[1].glyph != "█" || column[1].fg != lossTones[0].smoke {
		t.Fatalf("spread cell = %+v, want smoke block", column[1])
	}
	if column[0].glyph != "▄" || column[0].fg != lossTones[0].smoke {
		t.Fatalf("top cell = %+v, want partial smoke", column[0])
	}
}

func TestBucketColumnMarksOutage(t *testing.T) {
	scale := latencyScale{rows: 3, rowStep: 10, max: 30}
	column := bucketColumn(latencyBucket{total: 5, lost: 5}, scale)
	for _, cell := range column {
		if cell.glyph != "░" || cell.fg != lossTones[4].solid {
			t.Fatalf("outage cell = %+v, want red shade", cell)
		}
	}
}

func TestLatencyScaleClipsSpikes(t *testing.T) {
	latencies := make([]float64, 0, 200)
	for i := 0; i < 199; i++ {
		latencies = append(latencies, 20)
	}
	latencies = append(latencies, 900)
	series := latencySeries{latencies: latencies, buckets: []latencyBucket{{total: 200, median: 20, latencies: latencies}}}

	scale := newLatencyScale(series, 8)
	if scale.max >= 900 || scale.max < 20 {
		t.Fatalf("scale max = %.1f, want spike clipped but median visible", scale.max)
	}
}

func TestNiceCeil(t *testing.T) {
	for _, tc := range []struct{ in, want float64 }{{3.7, 4}, {0.42, 0.5}, {12, 15}, {81, 100}, {1, 1}} {
		if got := niceCeil(tc.in); got != tc.want {
			t.Fatalf("niceCeil(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestTimeTicksAlignToClock(t *testing.T) {
	now := time.Date(2026, 5, 25, 10, 47, 13, 0, time.Local)
	series := buildLatencySeries(testProbe, nil, now, 30*time.Minute, 10*time.Second, 90)
	ticks := timeTicks(series)
	if len(ticks) < 2 {
		t.Fatalf("ticks = %+v, want several", ticks)
	}
	for _, tick := range ticks {
		if !strings.HasSuffix(tick.label, "0") && !strings.HasSuffix(tick.label, "5") {
			t.Fatalf("tick label %q is not on a round clock value", tick.label)
		}
	}
	line := tickLabelLine(series.plotWidth(), ticks)
	if !strings.Contains(line, "10:30") {
		t.Fatalf("expected 10:30 tick in %q", line)
	}
}

func TestTickLabelShowsDateAtMidnightForLongWindows(t *testing.T) {
	midnight := time.Date(2026, 5, 26, 0, 0, 0, 0, time.Local)
	if got := tickLabel(midnight, 6*time.Hour, 48*time.Hour); got != "05-26" {
		t.Fatalf("midnight label = %q, want 05-26", got)
	}
	if got := tickLabel(midnight.Add(6*time.Hour), 6*time.Hour, 48*time.Hour); got != "06:00" {
		t.Fatalf("label = %q, want 06:00", got)
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

func TestZoomDetailWindowBounds(t *testing.T) {
	if got := zoomDetailWindow(detailChartRealtime, 30*time.Minute, true); got != 15*time.Minute {
		t.Fatalf("zoom in realtime = %s, want 15m", got)
	}
	if got := zoomDetailWindow(detailChartRealtime, 5*time.Minute, true); got != 5*time.Minute {
		t.Fatalf("min realtime = %s, want 5m", got)
	}
	if got := zoomDetailWindow(detailChart7Days, 24*time.Hour, true); got != 24*time.Hour {
		t.Fatalf("min week = %s, want 24h", got)
	}
	if got := zoomDetailWindow(detailChart7Days, 4*24*time.Hour, false); got != 7*24*time.Hour {
		t.Fatalf("max week = %s, want 168h", got)
	}
}

func TestFormatAxisValueKeepsStepPrecision(t *testing.T) {
	for _, tc := range []struct {
		value, step float64
		want        string
	}{{0.75, 0.25, "0.75"}, {1.5, 0.5, "1.5"}, {40, 10, "40"}, {0.3, 0.15, "0.30"}} {
		if got := formatAxisValue(tc.value, tc.step); got != tc.want {
			t.Fatalf("formatAxisValue(%v, %v) = %q, want %q", tc.value, tc.step, got, tc.want)
		}
	}
}
