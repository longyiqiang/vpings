package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/guptarohit/asciigraph"

	"github.com/longyiqiang/vpings/internal/probe"
)

type latencySeries struct {
	key      string
	values   []float64
	lastSeen time.Time
}

func RenderLatencyCharts(results []probe.Result) string {
	series := collectLatencySeries(results, 4, 24)
	if len(series) == 0 {
		return mutedStyle.Render("No latency data available for charts yet.")
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("Latency charts"))
	for _, item := range series {
		b.WriteString("\n\n")
		b.WriteString(asciigraph.Plot(item.values,
			asciigraph.Caption(item.key+" ms"),
			asciigraph.Height(7),
			asciigraph.Width(48),
			asciigraph.Precision(1),
		))
	}
	return b.String()
}

func collectLatencySeries(results []probe.Result, maxGroups, maxPoints int) []latencySeries {
	byKey := map[string]*latencySeries{}
	order := make([]string, 0)

	for _, result := range results {
		if result.Duration <= 0 {
			continue
		}
		key := fmt.Sprintf("%s %s:%d", result.Protocol, result.Host, result.Port)
		item, ok := byKey[key]
		if !ok {
			item = &latencySeries{key: key}
			byKey[key] = item
			order = append(order, key)
		}
		item.values = append(item.values, float64(result.Duration.Microseconds())/1000)
		item.lastSeen = result.StartedAt
		if len(item.values) > maxPoints {
			item.values = item.values[len(item.values)-maxPoints:]
		}
	}

	if len(order) == 0 {
		return nil
	}

	// Keep the most recently updated probe groups first without importing a sorting dependency.
	for i := 0; i < len(order); i++ {
		for j := i + 1; j < len(order); j++ {
			if byKey[order[j]].lastSeen.After(byKey[order[i]].lastSeen) {
				order[i], order[j] = order[j], order[i]
			}
		}
	}

	series := make([]latencySeries, 0, maxGroups)
	for _, key := range order {
		item := byKey[key]
		if len(item.values) == 0 {
			continue
		}
		series = append(series, *item)
		if len(series) == maxGroups {
			break
		}
	}
	return series
}
