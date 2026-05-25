package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/longyiqiang/vpings/internal/probe"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("250"))
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	failStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	mutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
)

func RenderResults(results []probe.Result) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("vpings results"))
	b.WriteString("\n\n")
	b.WriteString(headerStyle.Render(fmt.Sprintf("%-8s %-28s %-8s %-10s %-12s %s", "PROTO", "TARGET", "PORT", "STATUS", "LATENCY", "DETAIL")))
	b.WriteByte('\n')
	for _, result := range results {
		status := renderStatus(result.Status)
		detail := result.Description
		if detail == "" {
			detail = result.Error
		}
		b.WriteString(fmt.Sprintf("%-8s %-28s %-8s %-10s %-12s %s\n",
			result.Protocol,
			result.Host,
			resultDisplayPort(result),
			status,
			formatDuration(result.Duration),
			mutedStyle.Render(detail),
		))
	}
	return strings.TrimRight(b.String(), "\n")
}

func resultDisplayPort(result probe.Result) string {
	if result.Protocol == probe.ProtocolICMP {
		return "-"
	}
	return fmt.Sprintf("%d", result.Port)
}

func renderStatus(status probe.Status) string {
	switch status {
	case probe.StatusOK:
		return okStyle.Render(string(status))
	case probe.StatusSentNoReply:
		return warnStyle.Render(string(status))
	default:
		return failStyle.Render(string(status))
	}
}

func formatDuration(value time.Duration) string {
	if value <= 0 {
		return "-"
	}
	if value < time.Second {
		return fmt.Sprintf("%.1fms", float64(value.Microseconds())/1000)
	}
	return value.Truncate(time.Millisecond).String()
}
