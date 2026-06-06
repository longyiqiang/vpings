package ui

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/guptarohit/asciigraph"

	"github.com/longyiqiang/vpings/internal/appconfig"
	"github.com/longyiqiang/vpings/internal/probe"
)

type probeSummary struct {
	ProbeID   string
	ProbeName string
	Protocol  probe.Protocol
	Host      string
	Port      int
	StartedAt time.Time
	EndedAt   time.Time
	Attempts  int
	Received  int
	Lost      int
	MinMS     float64
	AverageMS float64
	MaxMS     float64
	JitterMS  float64
	LossRate  float64
	Samples   []roundSample
}

type summaryBucket struct {
	summary   probeSummary
	latencies []float64
	samples   []roundSample
}

type roundSample struct {
	LatencyMS float64
	OK        bool
	Attempt   int
	StartedAt time.Time
}

type timeTick struct {
	col   int
	index int
}

const barRoundGap = 3

const (
	detailChartRealtime = iota
	detailChart24Hours
	detailChart48Hours
	detailChart7Days
	detailChartCount
)

type detailChartSpec struct {
	title         string
	defaultWindow time.Duration
	minWindow     time.Duration
}

var detailChartSpecs = [detailChartCount]detailChartSpec{
	{title: "realtime", defaultWindow: 3 * time.Hour, minWindow: 10 * time.Minute},
	{title: "past 24 hours", defaultWindow: 24 * time.Hour, minWindow: 10 * time.Minute},
	{title: "past 48 hours", defaultWindow: 48 * time.Hour, minWindow: 30 * time.Minute},
	{title: "past week", defaultWindow: 7 * 24 * time.Hour, minWindow: 24 * time.Hour},
}

func defaultDetailChartWindows() [detailChartCount]time.Duration {
	var windows [detailChartCount]time.Duration
	for i := range windows {
		windows[i] = detailChartSpecs[i].defaultWindow
	}
	return windows
}

func detailChartDefaultWindow(index int) time.Duration {
	if index < 0 || index >= detailChartCount {
		return detailChartSpecs[detailChartRealtime].defaultWindow
	}
	return detailChartSpecs[index].defaultWindow
}

func zoomDetailWindow(index int, current time.Duration, zoomIn bool) time.Duration {
	if index < 0 || index >= detailChartCount {
		index = detailChartRealtime
	}
	spec := detailChartSpecs[index]
	if current <= 0 {
		current = spec.defaultWindow
	}
	if zoomIn {
		next := current / 2
		if next < spec.minWindow {
			return spec.minWindow
		}
		return next
	}
	next := current * 2
	if next > spec.defaultWindow {
		return spec.defaultWindow
	}
	return next
}

func RenderRealtimeProbeChart(item appconfig.ProbeConfig, results []probe.Result, window time.Duration) string {
	summaries := summariesForProbe(item, results, 0, time.Now().Add(-window))
	if len(summaries) == 0 {
		return mutedStyle.Render("No realtime samples yet. Press r to run this probe round.")
	}
	return renderBarChart("realtime "+item.Name, summaries, 72, 9)
}

func RenderProbeDetailCharts(item appconfig.ProbeConfig, results []probe.Result, now time.Time, windows [detailChartCount]time.Duration, selected int) string {
	var b strings.Builder
	for i, spec := range detailChartSpecs {
		if i > 0 {
			b.WriteString("\n\n")
		}
		window := windows[i]
		if window <= 0 {
			window = spec.defaultWindow
		}
		since := now.Add(-window)
		summaries := summariesForProbe(item, results, 0, since)
		title := fmt.Sprintf("%s window=%s min=%s", spec.title, window, spec.minWindow)
		if i == selected {
			title = "> " + title
		} else {
			title = "  " + title
		}
		if len(summaries) == 0 {
			b.WriteString(headerStyle.Render(title))
			b.WriteString("\n")
			b.WriteString(mutedStyle.Render("No samples in this window."))
			continue
		}
		if i == selected {
			b.WriteString(okStyle.Render(title))
		} else {
			b.WriteString(mutedStyle.Render(title))
		}
		b.WriteString("\n")
		b.WriteString(renderBarChart(spec.title, summaries, 72, 8))
	}
	return b.String()
}

func formatProbeSelector(item appconfig.ProbeConfig, results []probe.Result) string {
	summaries := summariesForProbe(item, results, 1, time.Time{})
	if len(summaries) == 0 {
		return fmt.Sprintf("%-22s %-8s %-24s no samples", truncate(item.Name, 22), item.Protocol, truncate(item.Host, 24))
	}
	last := summaries[len(summaries)-1]
	if last.Received == 0 {
		return fmt.Sprintf("%-22s %-8s %-24s no reply loss %.0f%%",
			truncate(item.Name, 22),
			item.Protocol,
			truncate(item.Host, 24),
			last.LossRate*100,
		)
	}
	return fmt.Sprintf("%-22s %-8s %-24s avg %.1fms range %.1f-%.1fms loss %.0f%%",
		truncate(item.Name, 22),
		item.Protocol,
		truncate(item.Host, 24),
		last.AverageMS,
		last.MinMS,
		last.MaxMS,
		last.LossRate*100,
	)
}

type chartCell struct {
	text  string
	color asciigraph.AnsiColor
}

func renderBarChart(title string, summaries []probeSummary, width, height int) string {
	if len(summaries) == 0 {
		return mutedStyle.Render("No samples.")
	}
	if width < 12 {
		width = 12
	}
	if height < 3 {
		height = 3
	}

	visible := visibleSummariesForBars(summaries, width)

	hasLatency := false
	maxLatency := 0.0
	for _, summary := range visible {
		for _, sample := range summary.Samples {
			if !sample.OK {
				continue
			}
			hasLatency = true
			if sample.LatencyMS > maxLatency {
				maxLatency = sample.LatencyMS
			}
		}
	}
	if !hasLatency {
		last := summaries[len(summaries)-1]
		return headerStyle.Render(title) + "\n" + mutedStyle.Render(fmt.Sprintf(
			"No successful samples yet. latest %s loss %.0f%% attempts %d",
			last.StartedAt.Format("15:04:05"),
			last.LossRate*100,
			last.Attempts,
		))
	}

	grid := make([][]chartCell, height)
	for row := range grid {
		grid[row] = make([]chartCell, width)
		for col := range grid[row] {
			grid[row][col] = chartCell{text: " ", color: asciigraph.Default}
		}
	}

	col := 0
	groupCenters := make([]int, 0, len(visible))
	for i, summary := range visible {
		samples := samplesForBarWidth(summary.Samples, width-col)
		if len(samples) == 0 {
			continue
		}
		startCol := col
		color := lossColor(summary.LossRate)
		for _, sample := range samples {
			if col >= width {
				break
			}
			if sample.OK {
				barHeight := latencyBarHeight(sample.LatencyMS, maxLatency, height)
				for row := height - barHeight; row < height; row++ {
					grid[row][col] = chartCell{text: "█", color: color}
				}
			} else {
				grid[height-1][col] = chartCell{text: "·", color: color}
			}
			col++
		}
		if col > startCol {
			groupCenters = append(groupCenters, startCol+(col-startCol-1)/2)
		}
		if i < len(visible)-1 {
			col += barRoundGap
			if col > width {
				col = width
			}
		}
	}

	return renderBarGrid(title, visible, groupCenters, grid, maxLatency)
}

func summariesForProbe(item appconfig.ProbeConfig, results []probe.Result, limit int, since time.Time) []probeSummary {
	all := summarizeResults(results)
	key := configProbeKey(item)
	filtered := make([]probeSummary, 0, len(all))
	for _, summary := range all {
		if summaryProbeKey(summary) != key {
			continue
		}
		if !since.IsZero() && summary.StartedAt.Before(since) {
			continue
		}
		filtered = append(filtered, summary)
	}
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}
	return filtered
}

func summarizeResults(results []probe.Result) []probeSummary {
	buckets := map[string]*summaryBucket{}
	order := make([]string, 0)
	for _, result := range results {
		key := resultSummaryKey(result)
		if key == "" {
			key = fmt.Sprintf("%s-%d", resultProbeKey(result), result.StartedAt.UnixNano())
		}
		bucket, ok := buckets[key]
		if !ok {
			bucket = &summaryBucket{
				summary: probeSummary{
					ProbeID:   result.ProbeID,
					ProbeName: result.ProbeName,
					Protocol:  result.Protocol,
					Host:      result.Host,
					Port:      result.Port,
					StartedAt: result.StartedAt,
					EndedAt:   result.StartedAt,
					Attempts:  result.Attempts,
				},
			}
			if bucket.summary.Attempts == 0 {
				bucket.summary.Attempts = 1
			}
			buckets[key] = bucket
			order = append(order, key)
		}
		if result.StartedAt.Before(bucket.summary.StartedAt) {
			bucket.summary.StartedAt = result.StartedAt
		}
		if result.StartedAt.After(bucket.summary.EndedAt) {
			bucket.summary.EndedAt = result.StartedAt
		}
		if result.Attempts > bucket.summary.Attempts {
			bucket.summary.Attempts = result.Attempts
		}
		if result.Status == probe.StatusOK {
			bucket.summary.Received++
			latencyMS := float64(result.Duration.Microseconds()) / 1000
			bucket.latencies = append(bucket.latencies, latencyMS)
			bucket.samples = append(bucket.samples, roundSample{
				LatencyMS: latencyMS,
				OK:        true,
				Attempt:   result.Attempt,
				StartedAt: result.StartedAt,
			})
		} else {
			bucket.summary.Lost++
			bucket.samples = append(bucket.samples, roundSample{
				OK:        false,
				Attempt:   result.Attempt,
				StartedAt: result.StartedAt,
			})
		}
	}

	summaries := make([]probeSummary, 0, len(order))
	for _, key := range order {
		bucket := buckets[key]
		if bucket.summary.Attempts < bucket.summary.Received+bucket.summary.Lost {
			bucket.summary.Attempts = bucket.summary.Received + bucket.summary.Lost
		}
		if len(bucket.latencies) > 0 {
			sort.Float64s(bucket.latencies)
			bucket.summary.MinMS = bucket.latencies[0]
			bucket.summary.MaxMS = bucket.latencies[len(bucket.latencies)-1]
			bucket.summary.AverageMS = average(bucket.latencies)
			bucket.summary.JitterMS = bucket.summary.MaxMS - bucket.summary.MinMS
		}
		sort.SliceStable(bucket.samples, func(i, j int) bool {
			left := bucket.samples[i]
			right := bucket.samples[j]
			if left.Attempt > 0 && right.Attempt > 0 && left.Attempt != right.Attempt {
				return left.Attempt < right.Attempt
			}
			return left.StartedAt.Before(right.StartedAt)
		})
		bucket.summary.Samples = append([]roundSample(nil), bucket.samples...)
		if bucket.summary.Attempts > 0 {
			bucket.summary.LossRate = float64(bucket.summary.Attempts-bucket.summary.Received) / float64(bucket.summary.Attempts)
		}
		summaries = append(summaries, bucket.summary)
	}
	sort.SliceStable(summaries, func(i, j int) bool {
		return summaries[i].StartedAt.Before(summaries[j].StartedAt)
	})
	return summaries
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return math.NaN()
	}
	var total float64
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func lossBucket(rate float64) int {
	switch {
	case rate <= 0:
		return 0
	case rate <= 0.25:
		return 1
	case rate <= 0.5:
		return 2
	case rate <= 0.75:
		return 3
	default:
		return 4
	}
}

func lossColor(rate float64) asciigraph.AnsiColor {
	switch lossBucket(rate) {
	case 0:
		return asciigraph.Green
	case 1:
		return asciigraph.Yellow
	case 2:
		return asciigraph.Orange
	case 3:
		return asciigraph.Magenta
	default:
		return asciigraph.Red
	}
}

func visibleSummariesForBars(summaries []probeSummary, width int) []probeSummary {
	if len(summaries) == 0 {
		return nil
	}
	used := 0
	start := len(summaries)
	for start > 0 {
		nextWidth := summaryBarWidth(summaries[start-1])
		if used > 0 {
			nextWidth += barRoundGap
		}
		if used > 0 && used+nextWidth > width {
			break
		}
		used += nextWidth
		start--
		if used >= width {
			break
		}
	}
	if start == len(summaries) {
		start = len(summaries) - 1
	}
	return summaries[start:]
}

func summaryBarWidth(summary probeSummary) int {
	if len(summary.Samples) > 0 {
		return len(summary.Samples)
	}
	if summary.Attempts > 0 {
		return summary.Attempts
	}
	return 1
}

func samplesForBarWidth(samples []roundSample, available int) []roundSample {
	if available <= 0 {
		return nil
	}
	if len(samples) <= available {
		return samples
	}
	return samples[len(samples)-available:]
}

func latencyBarHeight(value, maxValue float64, height int) int {
	if maxValue <= 0 {
		return 1
	}
	barHeight := int(math.Ceil(value / maxValue * float64(height)))
	if barHeight < 1 {
		return 1
	}
	if barHeight > height {
		return height
	}
	return barHeight
}

func renderBarGrid(title string, summaries []probeSummary, groupCenters []int, grid [][]chartCell, maxValue float64) string {
	height := len(grid)
	width := len(grid[0])
	maxLabel := fmt.Sprintf("%.1f", maxValue)
	labelWidth := len(maxLabel)

	var b strings.Builder
	for row := 0; row < height; row++ {
		value := maxValue
		if height > 1 {
			value = maxValue - (float64(row) * maxValue / float64(height-1))
		}
		b.WriteString(fmt.Sprintf("%*.1f ┤", labelWidth, value))
		writeChartCells(&b, grid[row])
		if row < height-1 {
			b.WriteByte('\n')
		}
	}
	b.WriteByte('\n')
	b.WriteString(strings.Repeat(" ", labelWidth+1))
	b.WriteString("└")
	axis := make([]rune, width)
	for i := range axis {
		axis[i] = '─'
	}
	for _, col := range groupCenters {
		if col >= 0 && col < width {
			axis[col] = '┬'
		}
	}
	b.WriteString(string(axis))
	b.WriteByte('\n')
	b.WriteString(strings.Repeat(" ", labelWidth+2))
	b.WriteString(timeTickLabelsForCenters(summaries, groupCenters, width, 4))
	b.WriteByte('\n')
	b.WriteString(strings.Repeat(" ", labelWidth+2))
	if len(title) < width {
		b.WriteString(strings.Repeat(" ", (width-len(title))/2))
	}
	b.WriteString(title)
	return b.String()
}

func writeChartCells(b *strings.Builder, row []chartCell) {
	active := asciigraph.Default
	for _, cell := range row {
		if cell.color != active {
			if active != asciigraph.Default {
				b.WriteString(asciigraph.Default.String())
			}
			active = cell.color
			if active != asciigraph.Default {
				b.WriteString(active.String())
			}
		}
		b.WriteString(cell.text)
	}
	if active != asciigraph.Default {
		b.WriteString(asciigraph.Default.String())
	}
}

func timeTicksForCenters(centers []int, count int) []timeTick {
	if len(centers) == 0 {
		return nil
	}
	if count < 2 {
		count = 2
	}
	ticks := make([]timeTick, 0, count)
	seen := map[int]struct{}{}
	for i := 0; i < count; i++ {
		index := int(math.Round(float64(i) * float64(len(centers)-1) / float64(count-1)))
		col := centers[index]
		if _, ok := seen[col]; ok {
			continue
		}
		seen[col] = struct{}{}
		ticks = append(ticks, timeTick{col: col, index: index})
	}
	return ticks
}

func timeTickLabelsForCenters(summaries []probeSummary, centers []int, width, count int) string {
	line := []rune(strings.Repeat(" ", width))
	if len(summaries) == 0 || len(centers) == 0 {
		return string(line)
	}
	layout := timeLabelLayout(summaries)
	occupiedUntil := -1
	for _, tick := range timeTicksForCenters(centers, count) {
		if tick.index >= len(summaries) {
			tick.index = len(summaries) - 1
		}
		label := summaries[tick.index].StartedAt.Format(layout)
		start := tick.col - len(label)/2
		if start < 0 {
			start = 0
		}
		if start+len(label) > width {
			start = width - len(label)
		}
		if start <= occupiedUntil {
			continue
		}
		for i, r := range label {
			pos := start + i
			if pos >= 0 && pos < len(line) {
				line[pos] = r
			}
		}
		occupiedUntil = start + len(label)
	}
	return string(line)
}

func timeLabelLayout(summaries []probeSummary) string {
	if len(summaries) < 2 {
		return "15:04:05"
	}
	start := summaries[0].StartedAt
	end := summaries[len(summaries)-1].StartedAt
	span := end.Sub(start)
	if span < 0 {
		span = -span
	}
	switch {
	case span <= time.Hour:
		return "15:04:05"
	case span <= 48*time.Hour:
		return "15:04"
	case start.Year() == end.Year():
		return "01-02 15"
	default:
		return "2006-01-02"
	}
}

func configProbeKey(item appconfig.ProbeConfig) string {
	if item.ID != "" {
		return item.ID
	}
	return fmt.Sprintf("%s/%s/%d", item.Protocol, item.Host, item.Port)
}

func resultProbeKey(result probe.Result) string {
	if result.ProbeID != "" {
		return result.ProbeID
	}
	return fmt.Sprintf("%s/%s/%d", result.Protocol, result.Host, result.Port)
}

func resultSummaryKey(result probe.Result) string {
	if result.RoundID == "" {
		return ""
	}
	return resultProbeKey(result) + "|" + result.RoundID
}

func summaryProbeKey(summary probeSummary) string {
	if summary.ProbeID != "" {
		return summary.ProbeID
	}
	return fmt.Sprintf("%s/%s/%d", summary.Protocol, summary.Host, summary.Port)
}
