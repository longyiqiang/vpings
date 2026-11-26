package ui

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/longyiqiang/vpings/internal/appconfig"
	"github.com/longyiqiang/vpings/internal/probe"
)

// ---------------------------------------------------------------------------
// Per-round summaries (used by the probe selector list).
// ---------------------------------------------------------------------------

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
}

type summaryBucket struct {
	summary   probeSummary
	latencies []float64
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
			bucket.latencies = append(bucket.latencies, durationMS(result.Duration))
		} else {
			bucket.summary.Lost++
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

// ---------------------------------------------------------------------------
// Detail chart windows and zoom.
// ---------------------------------------------------------------------------

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
	{title: "realtime", defaultWindow: 30 * time.Minute, minWindow: 5 * time.Minute},
	{title: "past 24 hours", defaultWindow: 24 * time.Hour, minWindow: 30 * time.Minute},
	{title: "past 48 hours", defaultWindow: 48 * time.Hour, minWindow: time.Hour},
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

// ---------------------------------------------------------------------------
// Palette. Hues follow loss rate: green -> yellow -> orange -> magenta -> red.
// "solid" paints the median area, "smoke" paints the median..p95 spread.
// ---------------------------------------------------------------------------

const colorDefault = -1

const (
	chartAxisColor  = 240
	chartLabelColor = 245
)

type lossTone struct {
	solid int
	smoke int
}

var lossTones = [5]lossTone{
	{solid: 42, smoke: 29},   // < 1%
	{solid: 220, smoke: 136}, // < 5%
	{solid: 214, smoke: 130}, // < 15%
	{solid: 207, smoke: 96},  // < 50%
	{solid: 196, smoke: 124}, // >= 50%
}

var lossToneLabels = [5]string{"<1%", "<5%", "<15%", "<50%", "≥50%"}

// lossBucket maps a loss rate to a tone. A single lost packet among hundreds
// is noise, so anything under 1% still counts as healthy.
func lossBucket(rate float64) int {
	switch {
	case rate < 0.01:
		return 0
	case rate < 0.05:
		return 1
	case rate < 0.15:
		return 2
	case rate < 0.5:
		return 3
	default:
		return 4
	}
}

func toneForLoss(rate float64) lossTone {
	return lossTones[lossBucket(rate)]
}

// ---------------------------------------------------------------------------
// Time buckets: every column group of the plot is one fixed slice of time.
// ---------------------------------------------------------------------------

type latencyBucket struct {
	total     int
	lost      int
	latencies []float64
	median    float64
	p95       float64
	max       float64
}

func (b latencyBucket) received() int {
	return b.total - b.lost
}

func (b latencyBucket) lossRate() float64 {
	if b.total == 0 {
		return 0
	}
	return float64(b.lost) / float64(b.total)
}

type latencySeries struct {
	start     time.Time
	end       time.Time
	bucketDur time.Duration
	colsPer   int
	buckets   []latencyBucket
	latencies []float64 // sorted successful latencies inside the window
	total     int
	lost      int
}

func (s latencySeries) received() int {
	return s.total - s.lost
}

func (s latencySeries) lossRate() float64 {
	if s.total == 0 {
		return 0
	}
	return float64(s.lost) / float64(s.total)
}

func (s latencySeries) plotWidth() int {
	return len(s.buckets) * s.colsPer
}

// bucketLayout splits window into at most width buckets, never finer than one
// probe interval. When there are fewer buckets than columns each bucket is
// stretched over several columns.
func bucketLayout(window, interval time.Duration, width int) (count int, dur time.Duration, colsPer int) {
	if width < 1 {
		width = 1
	}
	if interval <= 0 {
		interval = time.Second
	}
	if window < interval {
		window = interval
	}
	count = width
	if byInterval := int(window / interval); byInterval < count {
		count = byInterval
	}
	if count < 1 {
		count = 1
	}
	dur = window / time.Duration(count)
	if dur < time.Millisecond {
		dur = time.Millisecond
	}
	colsPer = width / count
	if colsPer < 1 {
		colsPer = 1
	}
	return count, dur, colsPer
}

func buildLatencySeries(item appconfig.ProbeConfig, results []probe.Result, now time.Time, window, interval time.Duration, width int) latencySeries {
	count, dur, colsPer := bucketLayout(window, interval, width)
	// Align bucket edges to absolute time so columns do not shift between redraws.
	step := int64(dur)
	end := time.Unix(0, (now.UnixNano()/step+1)*step).In(now.Location())
	start := end.Add(-dur * time.Duration(count))

	series := latencySeries{
		start:     start,
		end:       end,
		bucketDur: dur,
		colsPer:   colsPer,
		buckets:   make([]latencyBucket, count),
	}
	key := configProbeKey(item)
	for _, result := range results {
		if resultProbeKey(result) != key {
			continue
		}
		if result.StartedAt.Before(start) || !result.StartedAt.Before(end) {
			continue
		}
		index := int(result.StartedAt.Sub(start) / dur)
		if index < 0 || index >= count {
			continue
		}
		bucket := &series.buckets[index]
		bucket.total++
		series.total++
		if result.Status != probe.StatusOK {
			bucket.lost++
			series.lost++
			continue
		}
		latency := durationMS(result.Duration)
		bucket.latencies = append(bucket.latencies, latency)
		series.latencies = append(series.latencies, latency)
	}
	for i := range series.buckets {
		bucket := &series.buckets[i]
		if len(bucket.latencies) == 0 {
			continue
		}
		sort.Float64s(bucket.latencies)
		bucket.median = percentile(bucket.latencies, 0.5)
		bucket.p95 = percentile(bucket.latencies, 0.95)
		bucket.max = bucket.latencies[len(bucket.latencies)-1]
	}
	sort.Float64s(series.latencies)
	return series
}

// ---------------------------------------------------------------------------
// Vertical scale.
// ---------------------------------------------------------------------------

type latencyScale struct {
	rows    int
	rowStep float64
	max     float64
}

func newLatencyScale(series latencySeries, rows int) latencyScale {
	if rows < 1 {
		rows = 1
	}
	// Clip at p99 so one spike does not flatten the chart, but never clip a median.
	target := percentile(series.latencies, 0.99)
	for _, bucket := range series.buckets {
		if bucket.received() > 0 && bucket.median > target {
			target = bucket.median
		}
	}
	if math.IsNaN(target) || target <= 0 {
		target = 1
	}
	step := niceCeil(target / float64(rows))
	return latencyScale{rows: rows, rowStep: step, max: step * float64(rows)}
}

// level converts a latency into eighth-of-a-row units, at least 1 so that any
// successful sample stays visible.
func (s latencyScale) level(value float64) int {
	full := s.rows * 8
	if s.max <= 0 {
		return 1
	}
	level := int(math.Round(value / s.max * float64(full)))
	if level < 1 {
		return 1
	}
	if level > full {
		return full
	}
	return level
}

// labelEvery returns how many rows separate two y-axis labels.
func (s latencyScale) labelEvery() int {
	for _, k := range []int{1, 2, 5, 10} {
		if s.rows/k <= 5 {
			return k
		}
	}
	return 20
}

func niceCeil(value float64) float64 {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 1
	}
	exp := math.Pow(10, math.Floor(math.Log10(value)))
	fraction := value / exp
	for _, nice := range []float64{1, 1.5, 2, 2.5, 3, 4, 5, 6, 8, 10} {
		if fraction <= nice+1e-9 {
			return nice * exp
		}
	}
	return 10 * exp
}

// ---------------------------------------------------------------------------
// Rendering.
// ---------------------------------------------------------------------------

const chartLabelWidth = 5

var lowerBlocks = [9]string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

type chartCell struct {
	glyph string
	fg    int
	bg    int
}

var blankCell = chartCell{glyph: " ", fg: colorDefault, bg: colorDefault}

type latencyChart struct {
	title    string
	window   time.Duration
	interval time.Duration
	width    int // total width including the y-axis gutter
	height   int // plot rows; <= 0 renders only the header line
	now      time.Time
	selected bool
}

// RenderRealtimeProbeChart draws the realtime chart shown on the results page.
func RenderRealtimeProbeChart(item appconfig.ProbeConfig, results []probe.Result, now time.Time, window, interval time.Duration, width, height int) string {
	return renderLatencyChart(item, results, latencyChart{
		title:    "realtime · " + item.Name,
		window:   window,
		interval: interval,
		width:    width,
		height:   height,
		now:      now,
	})
}

// detailChartLayout sizes the four detail charts. The selected chart gets
// selectedHeight rows; the others get otherHeight rows (0 = header only).
type detailChartLayout struct {
	width          int
	selectedHeight int
	otherHeight    int
	interval       time.Duration
}

func RenderProbeDetailCharts(item appconfig.ProbeConfig, results []probe.Result, now time.Time, windows [detailChartCount]time.Duration, selected int, layout detailChartLayout) string {
	var b strings.Builder
	b.WriteString(renderChartLegend())
	for i, spec := range detailChartSpecs {
		b.WriteString("\n\n")
		window := windows[i]
		if window <= 0 {
			window = spec.defaultWindow
		}
		height := layout.otherHeight
		if i == selected {
			height = layout.selectedHeight
		}
		b.WriteString(renderLatencyChart(item, results, latencyChart{
			title:    spec.title,
			window:   window,
			interval: layout.interval,
			width:    layout.width,
			height:   height,
			now:      now,
			selected: i == selected,
		}))
	}
	return b.String()
}

func renderLatencyChart(item appconfig.ProbeConfig, results []probe.Result, chart latencyChart) string {
	plotWidth := chart.width - chartLabelWidth - 2
	if plotWidth < 10 {
		plotWidth = 10
	}
	series := buildLatencySeries(item, results, chart.now, chart.window, chart.interval, plotWidth)

	var b strings.Builder
	b.WriteString(renderChartHeader(chart, series))
	if chart.height <= 0 {
		return b.String()
	}
	b.WriteByte('\n')
	if series.total == 0 {
		b.WriteString(mutedStyle.Render(strings.Repeat(" ", chartLabelWidth+2) + "No samples in this window. Press r to run a probe round."))
		return b.String()
	}

	scale := newLatencyScale(series, chart.height)
	grid := plotGrid(series, scale)
	every := scale.labelEvery()
	for row := range grid {
		rowsFromBottom := scale.rows - row
		label := ""
		tick := "│"
		if rowsFromBottom%every == 0 && series.received() > 0 {
			label = formatAxisValue(scale.rowStep*float64(rowsFromBottom), scale.rowStep)
			tick = "┤"
		}
		b.WriteString(colorize(fmt.Sprintf("%*s ", chartLabelWidth, label), chartLabelColor))
		b.WriteString(colorize(tick, chartAxisColor))
		writeCells(&b, grid[row])
		b.WriteByte('\n')
	}

	b.WriteString(colorize(fmt.Sprintf("%*s ", chartLabelWidth, "loss"), chartLabelColor))
	b.WriteString(colorize("┤", chartAxisColor))
	writeCells(&b, lossRow(series))
	b.WriteByte('\n')

	ticks := timeTicks(series)
	b.WriteString(strings.Repeat(" ", chartLabelWidth+1))
	b.WriteString(colorize("└"+axisLine(series.plotWidth(), ticks), chartAxisColor))
	b.WriteByte('\n')
	b.WriteString(strings.Repeat(" ", chartLabelWidth+2))
	b.WriteString(colorize(strings.TrimRight(tickLabelLine(series.plotWidth(), ticks), " "), chartLabelColor))
	return b.String()
}

// renderChartHeader builds the title and stats line. When the terminal is
// narrow the least important parts are dropped first so the line never wraps.
func renderChartHeader(chart latencyChart, series latencySeries) string {
	type part struct {
		text string
		drop int // parts with drop < level are omitted; -1 = always kept
	}
	var parts []part
	if chart.selected {
		parts = append(parts, part{okStyle.Render("▸ " + chart.title), -1})
	} else {
		parts = append(parts, part{headerStyle.Render("  " + chart.title), -1})
	}
	parts = append(parts,
		part{mutedStyle.Render("  " + formatWindow(chart.window)), -1},
		part{mutedStyle.Render(" · " + formatWindow(series.bucketDur) + "/col"), 3},
	)
	if series.total == 0 {
		parts = append(parts, part{mutedStyle.Render("  · no samples"), -1})
	} else {
		parts = append(parts, part{mutedStyle.Render("  │"), -1})
		if series.received() > 0 {
			parts = append(parts,
				part{" med " + formatLatency(percentile(series.latencies, 0.5)), -1},
				part{"  p95 " + formatLatency(percentile(series.latencies, 0.95)), 2},
				part{"  max " + formatLatency(series.latencies[len(series.latencies)-1]), 1},
			)
		} else {
			parts = append(parts, part{" no reply", -1})
		}
		loss := series.lossRate()
		lossStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(strconv.Itoa(toneForLoss(loss).solid)))
		parts = append(parts,
			part{"  " + lossStyle.Render("loss "+formatPercent(loss)), -1},
			part{mutedStyle.Render(fmt.Sprintf("  n=%d", series.total)), 0},
		)
	}

	var line string
	for level := 0; level <= 4; level++ {
		var b strings.Builder
		for _, p := range parts {
			if p.drop >= 0 && p.drop < level {
				continue
			}
			b.WriteString(p.text)
		}
		line = b.String()
		if chart.width <= 0 || lipgloss.Width(line) <= chart.width {
			break
		}
	}
	return line
}

func renderChartLegend() string {
	green := lossTones[0]
	var b strings.Builder
	b.WriteString(colorize("█", green.solid))
	b.WriteString(mutedStyle.Render(" median  "))
	b.WriteString(colorize("█", green.smoke))
	b.WriteString(mutedStyle.Render(" median→p95  "))
	b.WriteString(colorize("░", lossTones[4].solid))
	b.WriteString(mutedStyle.Render(" no reply   color = loss "))
	for i, tone := range lossTones {
		b.WriteString(colorize("█", tone.solid))
		b.WriteString(mutedStyle.Render(lossToneLabels[i]))
		if i < len(lossTones)-1 {
			b.WriteByte(' ')
		}
	}
	return b.String()
}

// plotGrid returns rows top-to-bottom.
func plotGrid(series latencySeries, scale latencyScale) [][]chartCell {
	width := series.plotWidth()
	grid := make([][]chartCell, scale.rows)
	for row := range grid {
		grid[row] = make([]chartCell, width)
		for col := range grid[row] {
			grid[row][col] = blankCell
		}
	}
	cols := series.colsPer
	if cols >= 3 {
		cols-- // leave a gap between wide bars
	}
	for index, bucket := range series.buckets {
		column := bucketColumn(bucket, scale)
		first := index * series.colsPer
		for c := 0; c < cols; c++ {
			for row := range grid {
				grid[row][first+c] = column[row]
			}
		}
	}
	return grid
}

// bucketColumn renders one time bucket SmokePing-style: a solid area up to the
// median with eighth-block precision, and a dimmer "smoke" up to the p95.
func bucketColumn(bucket latencyBucket, scale latencyScale) []chartCell {
	cells := make([]chartCell, scale.rows)
	for i := range cells {
		cells[i] = blankCell
	}
	if bucket.total == 0 {
		return cells
	}
	tone := toneForLoss(bucket.lossRate())
	if bucket.received() == 0 {
		for i := range cells {
			cells[i] = chartCell{glyph: "░", fg: tone.solid, bg: colorDefault}
		}
		return cells
	}
	median := scale.level(bucket.median)
	top := scale.level(bucket.p95)
	if top < median {
		top = median
	}
	for row := range cells {
		base := (scale.rows - 1 - row) * 8
		switch {
		case median >= base+8:
			cells[row] = chartCell{glyph: "█", fg: tone.solid, bg: colorDefault}
		case median > base:
			cell := chartCell{glyph: lowerBlocks[median-base], fg: tone.solid, bg: colorDefault}
			// Fill the rest of the cell with smoke when the spread covers at
			// least half of the remaining space.
			if top >= base+8 || (top-median)*2 >= base+8-median {
				cell.bg = tone.smoke
			}
			cells[row] = cell
		case top > base:
			glyph := "█"
			if top < base+8 {
				glyph = lowerBlocks[top-base]
			}
			cells[row] = chartCell{glyph: glyph, fg: tone.smoke, bg: colorDefault}
		}
	}
	return cells
}

func lossRow(series latencySeries) []chartCell {
	row := make([]chartCell, series.plotWidth())
	for i := range row {
		row[i] = blankCell
	}
	cols := series.colsPer
	if cols >= 3 {
		cols--
	}
	for index, bucket := range series.buckets {
		if bucket.lost == 0 {
			continue
		}
		rate := bucket.lossRate()
		level := int(math.Ceil(rate * 8))
		if level < 1 {
			level = 1
		}
		if level > 8 {
			level = 8
		}
		color := toneForLoss(rate).solid
		if lossBucket(rate) == 0 {
			color = chartAxisColor // sub-1% loss stays visible but quiet
		}
		cell := chartCell{glyph: lowerBlocks[level], fg: color, bg: colorDefault}
		first := index * series.colsPer
		for c := 0; c < cols; c++ {
			row[first+c] = cell
		}
	}
	return row
}

// ---------------------------------------------------------------------------
// Time axis.
// ---------------------------------------------------------------------------

type timeTick struct {
	col   int
	label string
}

var tickSteps = []time.Duration{
	10 * time.Second, 15 * time.Second, 30 * time.Second,
	time.Minute, 2 * time.Minute, 5 * time.Minute, 10 * time.Minute, 15 * time.Minute, 30 * time.Minute,
	time.Hour, 2 * time.Hour, 3 * time.Hour, 6 * time.Hour, 12 * time.Hour,
	24 * time.Hour, 48 * time.Hour,
}

func timeTicks(series latencySeries) []timeTick {
	if len(series.buckets) == 0 || series.bucketDur <= 0 {
		return nil
	}
	window := series.end.Sub(series.start)
	step := tickSteps[len(tickSteps)-1]
	for _, candidate := range tickSteps {
		labelWidth := 5
		if candidate < time.Minute {
			labelWidth = 8
		}
		spacing := float64(candidate) / float64(series.bucketDur) * float64(series.colsPer)
		if spacing >= float64(labelWidth+3) {
			step = candidate
			break
		}
	}

	ticks := make([]timeTick, 0)
	first := alignLocal(series.start, step)
	if first.Before(series.start) {
		first = first.Add(step)
	}
	for t := first; t.Before(series.end); t = t.Add(step) {
		col := int(t.Sub(series.start)/series.bucketDur) * series.colsPer
		if col < 0 || col >= series.plotWidth() {
			continue
		}
		ticks = append(ticks, timeTick{col: col, label: tickLabel(t, step, window)})
	}
	return ticks
}

func alignLocal(t time.Time, step time.Duration) time.Time {
	seconds := int64(step / time.Second)
	if seconds <= 0 {
		return t
	}
	_, offset := t.Zone()
	local := t.Unix() + int64(offset)
	aligned := local - ((local%seconds)+seconds)%seconds
	return time.Unix(aligned-int64(offset), 0).In(t.Location())
}

func tickLabel(t time.Time, step, window time.Duration) string {
	switch {
	case step < time.Minute:
		return t.Format("15:04:05")
	case step >= 24*time.Hour:
		return t.Format("01-02")
	case window >= 12*time.Hour && t.Hour() == 0 && t.Minute() == 0:
		return t.Format("01-02")
	default:
		return t.Format("15:04")
	}
}

func axisLine(width int, ticks []timeTick) string {
	axis := []rune(strings.Repeat("─", width))
	for _, tick := range ticks {
		if tick.col >= 0 && tick.col < width {
			axis[tick.col] = '┬'
		}
	}
	return string(axis)
}

func tickLabelLine(width int, ticks []timeTick) string {
	line := []rune(strings.Repeat(" ", width))
	occupiedUntil := -1
	for _, tick := range ticks {
		label := []rune(tick.label)
		start := tick.col - len(label)/2
		if start < 0 {
			start = 0
		}
		if start+len(label) > width {
			start = width - len(label)
		}
		if start < 0 || start <= occupiedUntil+1 { // keep at least two spaces between labels
			continue
		}
		copy(line[start:], label)
		occupiedUntil = start + len(label)
	}
	return string(line)
}

// ---------------------------------------------------------------------------
// Helpers.
// ---------------------------------------------------------------------------

func writeCells(b *strings.Builder, row []chartCell) {
	fg, bg := colorDefault, colorDefault
	for _, cell := range row {
		if cell.fg != fg || cell.bg != bg {
			b.WriteString(sgr(cell.fg, cell.bg))
			fg, bg = cell.fg, cell.bg
		}
		b.WriteString(cell.glyph)
	}
	if fg != colorDefault || bg != colorDefault {
		b.WriteString("\x1b[0m")
	}
}

func sgr(fg, bg int) string {
	code := "\x1b[0m"
	if fg != colorDefault {
		code += fmt.Sprintf("\x1b[38;5;%dm", fg)
	}
	if bg != colorDefault {
		code += fmt.Sprintf("\x1b[48;5;%dm", bg)
	}
	return code
}

func colorize(text string, fg int) string {
	return sgr(fg, colorDefault) + text + "\x1b[0m"
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return math.NaN()
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	position := p * float64(len(sorted)-1)
	lower := int(math.Floor(position))
	upper := int(math.Ceil(position))
	if lower == upper {
		return sorted[lower]
	}
	weight := position - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
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

func durationMS(value time.Duration) float64 {
	return float64(value) / float64(time.Millisecond)
}

// formatAxisValue prints value with just enough decimals to show step exactly
// (0.25 -> 2 decimals, 0.5 -> 1, 5 -> 0).
func formatAxisValue(value, step float64) string {
	decimals := 0
	for decimals < 3 {
		scaled := step * math.Pow(10, float64(decimals))
		if math.Abs(scaled-math.Round(scaled)) < 1e-6 {
			break
		}
		decimals++
	}
	return strconv.FormatFloat(value, 'f', decimals, 64)
}

func formatLatency(ms float64) string {
	switch {
	case math.IsNaN(ms):
		return "-"
	case ms >= 100:
		return fmt.Sprintf("%.0fms", ms)
	case ms >= 10:
		return fmt.Sprintf("%.1fms", ms)
	default:
		return fmt.Sprintf("%.2fms", ms)
	}
}

func formatPercent(rate float64) string {
	switch {
	case rate <= 0:
		return "0%"
	case rate < 0.01:
		return fmt.Sprintf("%.2f%%", rate*100)
	case rate < 0.1:
		return fmt.Sprintf("%.1f%%", rate*100)
	default:
		return fmt.Sprintf("%.0f%%", rate*100)
	}
}

func formatWindow(value time.Duration) string {
	switch {
	case value >= 24*time.Hour && value%(24*time.Hour) == 0:
		return fmt.Sprintf("%dd", value/(24*time.Hour))
	case value >= time.Hour && value%time.Hour == 0:
		return fmt.Sprintf("%dh", value/time.Hour)
	case value >= time.Hour && value%time.Minute == 0:
		return fmt.Sprintf("%dh%dm", value/time.Hour, (value%time.Hour)/time.Minute)
	case value >= time.Minute && value%time.Minute == 0:
		return fmt.Sprintf("%dm", value/time.Minute)
	case value >= time.Second:
		return value.Round(time.Second).String()
	default:
		return value.String()
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
