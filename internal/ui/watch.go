package ui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/longyiqiang/vpings/internal/appconfig"
	"github.com/longyiqiang/vpings/internal/probe"
)

type Recorder interface {
	Append(probe.Result) error
}

type WatchModel struct {
	specs    []probe.Spec
	interval time.Duration
	recorder Recorder
	results  []probe.Result
	err      error
}

type tickMsg time.Time

type resultsMsg []probe.Result

type errMsg struct {
	err error
}

func NewWatchModel(specs []probe.Spec, interval time.Duration, recorder Recorder) WatchModel {
	return WatchModel{
		specs:    specs,
		interval: interval,
		recorder: recorder,
		results:  make([]probe.Result, 0, len(specs)*12),
	}
}

func (m WatchModel) Init() tea.Cmd {
	return runRound(m.specs)
}

func (m WatchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	case resultsMsg:
		for _, result := range msg {
			if err := m.recorder.Append(result); err != nil {
				m.err = err
				return m, nil
			}
			m.results = append(m.results, result)
		}
		if len(m.results) > 120 {
			m.results = m.results[len(m.results)-120:]
		}
		return m, waitTick(nextRoundDelay(msg, m.interval, time.Now()))
	case tickMsg:
		return m, runRound(m.specs)
	case errMsg:
		m.err = msg.err
		return m, nil
	}
	return m, nil
}

func (m WatchModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("vpings watch"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("q/esc quit"))
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(failStyle.Render(m.err.Error()))
		b.WriteString("\n")
	}

	start := 0
	if len(m.results) > 16 {
		start = len(m.results) - 16
	}
	b.WriteString(RenderResults(m.results[start:]))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render(fmt.Sprintf("interval %s | records are appended after every probe", m.interval)))
	return b.String()
}

func waitTick(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func nextRoundDelay(results []probe.Result, interval time.Duration, now time.Time) time.Duration {
	if interval <= 0 {
		return 0
	}
	if len(results) == 0 {
		return interval
	}
	start := results[0].StartedAt
	for _, result := range results[1:] {
		if result.StartedAt.IsZero() {
			continue
		}
		if start.IsZero() || result.StartedAt.Before(start) {
			start = result.StartedAt
		}
	}
	if start.IsZero() || now.Before(start) {
		return interval
	}
	elapsed := now.Sub(start)
	if elapsed >= interval {
		return 0
	}
	return interval - elapsed
}

func runRound(specs []probe.Spec) tea.Cmd {
	return func() tea.Msg {
		return resultsMsg(runSpecRound(specs, probe.Run))
	}
}

func runSpecRound(specs []probe.Spec, runner probeRunner) []probe.Result {
	results := make([]probe.Result, len(specs))
	roundID := newRoundID(time.Now())
	var wg sync.WaitGroup
	wg.Add(len(specs))
	for i, spec := range specs {
		i, spec := i, spec
		go func() {
			defer wg.Done()
			result := runner(context.Background(), spec)
			result.RoundID = roundID
			if result.ProbeID == "" {
				result.ProbeID = appconfig.NewProbeID(spec.Protocol, spec.Host, spec.Port)
			}
			results[i] = result
		}()
	}
	wg.Wait()
	return results
}
