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
	MedianMS  float64
	MaxMS     float64
	LossRate  float64
}

type summaryBucket struct {
	summary   probeSummary
	latencies []float64
}

func RenderRealtimeProbeChart(item appconfig.ProbeConfig, results []probe.Result, roundInterval time.Duration) string {
	summaries := summariesForProbe(item, results, 0, time.Time{})
	if len(summaries) == 0 {
		return mutedStyle.Render("No realtime samples yet. Press r to run this probe round.")
	}
	now := time.Now()
	summaries = alignRecentSummaries(summaries, roundInterval, 60, now)
	return renderSummaryChart("realtime "+item.Name, summaries, roundInterval, 56, 9)
}

func RenderProbeDetailCharts(item appconfig.ProbeConfig, results []probe.Result, now time.Time, roundInterval time.Duration) string {
	sections := []struct {
		title  string
		window time.Duration
	}{
		{title: "realtime", window: 60 * roundInterval},
		{title: "past 24 hours", window: 24 * time.Hour},
		{title: "past 2 days", window: 48 * time.Hour},
		{title: "past week", window: 7 * 24 * time.Hour},
	}

	var b strings.Builder
	for i, section := range sections {
		if i > 0 {
			b.WriteString("\n\n")
		}
		since := now.Add(-section.window)
		summaries := summariesForProbe(item, results, 0, since)
		if len(summaries) == 0 {
			b.WriteString(headerStyle.Render(section.title))
			b.WriteString("\n")
			b.WriteString(mutedStyle.Render("No samples in this window."))
			continue
		}
		summaries = alignWindowSummaries(summaries, roundInterval, since, now)
		b.WriteString(renderSummaryChart(section.title, summaries, roundInterval, 56, 8))
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
	return fmt.Sprintf("%-22s %-8s %-24s median %.1fms range %.1f-%.1fms loss %.0f%%",
		truncate(item.Name, 22),
		item.Protocol,
		truncate(item.Host, 24),
		last.MedianMS,
		last.MinMS,
		last.MaxMS,
		last.LossRate*100,
	)
}

func renderSummaryChart(title string, summaries []probeSummary, roundInterval time.Duration, width, height int) string {
	if len(summaries) == 0 {
		return mutedStyle.Render("No samples.")
	}

	mins := make([]float64, len(summaries))
	maxes := make([]float64, len(summaries))
	medianByLoss := make([][]float64, 5)
	for i := range medianByLoss {
		medianByLoss[i] = nanSeries(len(summaries))
	}

	hasLatency := false
	for i, summary := range summaries {
		if summary.Received == 0 {
			mins[i] = math.NaN()
			maxes[i] = math.NaN()
			continue
		}
		hasLatency = true
		mins[i] = summary.MinMS
		maxes[i] = summary.MaxMS
		medianByLoss[lossBucket(summary.LossRate)][i] = summary.MedianMS
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

	data := [][]float64{mins, maxes}
	data = append(data, medianByLoss...)
	caption := fmt.Sprintf("%s | x=time/%s y=latency(ms) | range=min/max | color=loss green->red", title, formatChartInterval(roundInterval))
	graph := asciigraph.PlotMany(data,
		asciigraph.Caption(caption),
		asciigraph.Height(height),
		asciigraph.Width(width),
		asciigraph.Precision(1),
		asciigraph.SeriesColors(asciigraph.Gray, asciigraph.DarkGray, asciigraph.Green, asciigraph.Yellow, asciigraph.Orange, asciigraph.Magenta, asciigraph.Red),
		asciigraph.XAxisRange(0, float64(len(summaries)-1)),
		asciigraph.XAxisTickCount(4),
		asciigraph.XAxisValueFormatter(func(value float64) string {
			if len(summaries) == 0 {
				return ""
			}
			index := int(math.Round(value))
			if index < 0 {
				index = 0
			}
			if index >= len(summaries) {
				index = len(summaries) - 1
			}
			return summaries[index].StartedAt.Format("15:04")
		}),
	)

	last := summaries[len(summaries)-1]
	return graph + "\n" + mutedStyle.Render(fmt.Sprintf(
		"latest %s median %.1fms range %.1f-%.1fms loss %.0f%% attempts %d | x unit %s",
		last.StartedAt.Format("15:04:05"),
		last.MedianMS,
		last.MinMS,
		last.MaxMS,
		last.LossRate*100,
		last.Attempts,
		formatChartInterval(roundInterval),
	))
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

func alignRecentSummaries(summaries []probeSummary, interval time.Duration, slots int, now time.Time) []probeSummary {
	if len(summaries) == 0 {
		return nil
	}
	interval = normalizeChartInterval(interval)
	if slots < 1 {
		slots = len(summaries)
	}
	end := summaries[len(summaries)-1].StartedAt
	if now.After(end) && now.Sub(end) < interval {
		end = now
	}
	end = end.Truncate(interval)
	start := end.Add(-time.Duration(slots-1) * interval)
	return alignWindowSummaries(summaries, interval, start, end)
}

func alignWindowSummaries(summaries []probeSummary, interval time.Duration, start, end time.Time) []probeSummary {
	if len(summaries) == 0 {
		return nil
	}
	interval = normalizeChartInterval(interval)
	if end.Before(start) {
		end = start
	}
	start = start.Truncate(interval)
	end = end.Truncate(interval)

	bySlot := make(map[int64]probeSummary, len(summaries))
	for _, summary := range summaries {
		slotTime := summary.StartedAt.Truncate(interval)
		key := slotTime.UnixNano()
		existing, ok := bySlot[key]
		if !ok || summary.StartedAt.After(existing.StartedAt) {
			summary.StartedAt = slotTime
			bySlot[key] = summary
		}
	}

	slotCount := int(end.Sub(start)/interval) + 1
	if slotCount < 1 {
		slotCount = 1
	}
	aligned := make([]probeSummary, 0, slotCount)
	for i := 0; i < slotCount; i++ {
		slotTime := start.Add(time.Duration(i) * interval)
		if summary, ok := bySlot[slotTime.UnixNano()]; ok {
			aligned = append(aligned, summary)
			continue
		}
		aligned = append(aligned, probeSummary{StartedAt: slotTime})
	}
	return aligned
}

func normalizeChartInterval(interval time.Duration) time.Duration {
	if interval <= 0 {
		return appconfig.DefaultProbeInterval
	}
	return interval
}

func formatChartInterval(interval time.Duration) string {
	return normalizeChartInterval(interval).String()
}

func summarizeResults(results []probe.Result) []probeSummary {
	buckets := map[string]*summaryBucket{}
	order := make([]string, 0)
	for _, result := range results {
		key := result.RoundID
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
			bucket.latencies = append(bucket.latencies, float64(result.Duration.Microseconds())/1000)
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
			bucket.summary.MedianMS = median(bucket.latencies)
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

func nanSeries(length int) []float64 {
	values := make([]float64, length)
	for i := range values {
		values[i] = math.NaN()
	}
	return values
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return math.NaN()
	}
	mid := len(values) / 2
	if len(values)%2 == 1 {
		return values[mid]
	}
	return (values[mid-1] + values[mid]) / 2
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

func summaryProbeKey(summary probeSummary) string {
	if summary.ProbeID != "" {
		return summary.ProbeID
	}
	return fmt.Sprintf("%s/%s/%d", summary.Protocol, summary.Host, summary.Port)
}
