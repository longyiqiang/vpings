package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/longyiqiang/vpings/internal/appconfig"
	"github.com/longyiqiang/vpings/internal/probe"
)

type appView int

const (
	viewResults appView = iota
	viewStatus
	viewProbes
	viewSettings
)

var menuItems = []string{
	"1 Results",
	"2 Status & logs",
	"3 Probes",
	"4 Program",
}

type AppModel struct {
	cfg        appconfig.Config
	configPath string
	storePath  string
	recorder   Recorder
	results    []probe.Result
	logs       []string
	active     appView
	probeIndex int
	setting    int
	startedAt  time.Time
	lastRun    time.Time
	form       *probeForm
	err        error
}

func NewAppModel(cfg appconfig.Config, configPath, storePath string, recorder Recorder, history []probe.Result) AppModel {
	return AppModel{
		cfg:        cfg,
		configPath: configPath,
		storePath:  storePath,
		recorder:   recorder,
		results:    history,
		logs:       []string{fmt.Sprintf("app started, loaded %d historical records", len(history))},
		active:     viewResults,
		startedAt:  time.Now(),
	}
}

func (m AppModel) Init() tea.Cmd {
	if len(m.cfg.EnabledSpecs()) == 0 {
		return nil
	}
	return runRound(m.cfg.EnabledSpecs())
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.form != nil {
		return m.updateForm(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.updateKey(msg)
	case resultsMsg:
		for _, result := range msg {
			if err := m.recorder.Append(result); err != nil {
				m.err = err
				m.addLog("record write failed: " + err.Error())
				return m, nil
			}
			m.results = append(m.results, result)
		}
		if len(msg) > 0 {
			m.lastRun = time.Now()
			m.addLog(fmt.Sprintf("completed %d probe(s)", len(msg)))
		}
		if len(m.results) > 200 {
			m.results = m.results[len(m.results)-200:]
		}
		if len(m.cfg.EnabledSpecs()) == 0 {
			return m, nil
		}
		return m, waitTick(m.cfg.ProbeInterval)
	case tickMsg:
		if len(m.cfg.EnabledSpecs()) == 0 {
			return m, nil
		}
		return m, runRound(m.cfg.EnabledSpecs())
	case errMsg:
		m.err = msg.err
		m.addLog("runtime error: " + msg.err.Error())
		return m, nil
	}
	return m, nil
}

func (m AppModel) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "1":
		m.active = viewResults
	case "2":
		m.active = viewStatus
	case "3":
		m.active = viewProbes
	case "4":
		m.active = viewSettings
	case "tab", "right":
		m.active = appView((int(m.active) + 1) % len(menuItems))
	case "shift+tab", "left":
		m.active = appView((int(m.active) + len(menuItems) - 1) % len(menuItems))
	case "r":
		if len(m.cfg.EnabledSpecs()) == 0 {
			m.addLog("no enabled probes to run")
			return m, nil
		}
		m.addLog("manual probe round started")
		return m, runRound(m.cfg.EnabledSpecs())
	}

	switch m.active {
	case viewProbes:
		return m.updateProbeKeys(msg)
	case viewSettings:
		return m.updateSettingKeys(msg)
	}
	return m, nil
}

func (m AppModel) updateProbeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.probeIndex > 0 {
			m.probeIndex--
		}
	case "down", "j":
		if m.probeIndex < len(m.cfg.Probes)-1 {
			m.probeIndex++
		}
	case "n":
		m.form = newProbeForm(appconfig.ProbeConfig{
			ID:       appconfig.NewProbeID(probe.ProtocolTCP, "example.com", 443),
			Name:     "New TCP Probe",
			Protocol: probe.ProtocolTCP,
			Host:     "example.com",
			Port:     443,
			Timeout:  m.cfg.DefaultTimeout,
			Enabled:  true,
		}, -1)
	case "e", "enter":
		if len(m.cfg.Probes) > 0 {
			m.form = newProbeForm(m.cfg.Probes[m.probeIndex], m.probeIndex)
		}
	case " ":
		if len(m.cfg.Probes) > 0 {
			m.cfg.Probes[m.probeIndex].Enabled = !m.cfg.Probes[m.probeIndex].Enabled
			m.saveConfig("probe enabled state updated")
		}
	case "d", "backspace":
		if len(m.cfg.Probes) > 0 {
			removed := m.cfg.Probes[m.probeIndex].Name
			m.cfg.Probes = append(m.cfg.Probes[:m.probeIndex], m.cfg.Probes[m.probeIndex+1:]...)
			if m.probeIndex >= len(m.cfg.Probes) && m.probeIndex > 0 {
				m.probeIndex--
			}
			m.saveConfig("deleted probe " + removed)
		}
	}
	return m, nil
}

func (m AppModel) updateSettingKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.setting > 0 {
			m.setting--
		}
	case "down", "j":
		if m.setting < 2 {
			m.setting++
		}
	case "a", "enter", " ":
		if m.setting == 0 {
			m.cfg.AutoStart = !m.cfg.AutoStart
			m.saveConfig("auto-start setting updated")
		}
	case "h":
		m.addLog("help: 1-4 switch pages, r run now, n/e/d/space manage probes, a toggle auto-start, q quit")
	case "u":
		m.addLog("update check: install/update flow is not wired yet; use release binaries for now")
	}
	return m, nil
}

func (m AppModel) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch key.String() {
	case "esc":
		m.form = nil
		m.addLog("probe edit cancelled")
		return m, nil
	case "enter":
		item, index, err := m.form.value()
		if err != nil {
			m.form.err = err.Error()
			return m, nil
		}
		if index >= 0 {
			m.cfg.Probes[index] = item
			m.addLog("updated probe " + item.Name)
		} else {
			m.cfg.Probes = append(m.cfg.Probes, item)
			m.probeIndex = len(m.cfg.Probes) - 1
			m.addLog("created probe " + item.Name)
		}
		m.form = nil
		m.saveConfig("probe configuration saved")
		return m, nil
	default:
		m.form.update(key)
	}
	return m, nil
}

func (m AppModel) View() string {
	if m.form != nil {
		return m.renderFrame(m.form.view())
	}

	var body string
	switch m.active {
	case viewResults:
		body = m.viewResults()
	case viewStatus:
		body = m.viewStatus()
	case viewProbes:
		body = m.viewProbes()
	case viewSettings:
		body = m.viewSettings()
	default:
		body = ""
	}
	return m.renderFrame(body)
}

func (m AppModel) renderFrame(body string) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("vpings"))
	b.WriteString("\n")
	for i, item := range menuItems {
		if appView(i) == m.active && m.form == nil {
			b.WriteString(okStyle.Render("[" + item + "]"))
		} else {
			b.WriteString(mutedStyle.Render(" " + item + " "))
		}
		if i < len(menuItems)-1 {
			b.WriteString("  ")
		}
	}
	b.WriteString("\n\n")
	b.WriteString(body)
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("1-4 switch | tab/arrow switch | r run now | q quit"))
	return b.String()
}

func (m AppModel) viewResults() string {
	if len(m.results) == 0 {
		return mutedStyle.Render("No probe results yet. Press r to run enabled probes.")
	}
	start := 0
	if len(m.results) > 18 {
		start = len(m.results) - 18
	}
	return RenderLatencyCharts(m.results) + "\n\n" + RenderResults(m.results[start:])
}

func (m AppModel) viewStatus() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Program status"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Uptime:       %s\n", time.Since(m.startedAt).Truncate(time.Second)))
	b.WriteString(fmt.Sprintf("Interval:     %s\n", m.cfg.ProbeInterval))
	b.WriteString(fmt.Sprintf("Enabled:      %d/%d probes\n", len(m.cfg.EnabledSpecs()), len(m.cfg.Probes)))
	b.WriteString(fmt.Sprintf("Auto-start:   %t\n", m.cfg.AutoStart))
	if m.lastRun.IsZero() {
		b.WriteString("Last run:     never\n")
	} else {
		b.WriteString(fmt.Sprintf("Last run:     %s\n", m.lastRun.Format(time.RFC3339)))
	}
	b.WriteString(fmt.Sprintf("Config:       %s\n", m.configPath))
	b.WriteString(fmt.Sprintf("Records:      %s\n", m.storePath))
	if m.err != nil {
		b.WriteString(failStyle.Render("Last error:   " + m.err.Error()))
		b.WriteByte('\n')
	}
	b.WriteString("\n")
	b.WriteString(headerStyle.Render("Logs"))
	b.WriteString("\n")
	for _, line := range tailStrings(m.logs, 12) {
		b.WriteString(mutedStyle.Render(line))
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m AppModel) viewProbes() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Probe settings"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("n new | e/enter edit | d delete | space enable/disable"))
	b.WriteString("\n\n")
	if len(m.cfg.Probes) == 0 {
		b.WriteString(mutedStyle.Render("No probes configured. Press n to create one."))
		return b.String()
	}
	b.WriteString(headerStyle.Render(fmt.Sprintf("%-3s %-22s %-8s %-28s %-6s %-9s %s", "", "NAME", "PROTO", "HOST", "PORT", "TIMEOUT", "STATE")))
	b.WriteByte('\n')
	for i, item := range m.cfg.Probes {
		cursor := " "
		if i == m.probeIndex {
			cursor = ">"
		}
		state := failStyle.Render("off")
		if item.Enabled {
			state = okStyle.Render("on")
		}
		line := fmt.Sprintf("%-3s %-22s %-8s %-28s %-6d %-9s %s",
			cursor,
			truncate(item.Name, 22),
			item.Protocol,
			truncate(item.Host, 28),
			item.Port,
			item.Timeout,
			state,
		)
		if i == m.probeIndex {
			b.WriteString(okStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m AppModel) viewSettings() string {
	rows := []string{
		fmt.Sprintf("Auto-start: %t", m.cfg.AutoStart),
		"Help: press h to write usage help into logs",
		"Update: press u to write update guidance into logs",
	}
	var b strings.Builder
	b.WriteString(headerStyle.Render("Program settings"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("up/down select | a/space/enter toggle auto-start | h help | u update"))
	b.WriteString("\n\n")
	for i, row := range rows {
		prefix := "  "
		if i == m.setting {
			prefix = "> "
			b.WriteString(okStyle.Render(prefix + row))
		} else {
			b.WriteString(prefix + row)
		}
		b.WriteByte('\n')
	}
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("Auto-start is currently stored in config. OS service registration will be added in the installer layer."))
	return strings.TrimRight(b.String(), "\n")
}

func (m *AppModel) addLog(message string) {
	m.logs = append(m.logs, fmt.Sprintf("%s %s", time.Now().Format("15:04:05"), message))
	if len(m.logs) > 200 {
		m.logs = m.logs[len(m.logs)-200:]
	}
}

func (m *AppModel) saveConfig(message string) {
	if err := appconfig.Save(m.configPath, m.cfg); err != nil {
		m.err = err
		m.addLog("config save failed: " + err.Error())
		return
	}
	m.addLog(message)
}

func tailStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return values[len(values)-limit:]
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	if max <= 1 {
		return value[:max]
	}
	return value[:max-1] + "."
}

type probeForm struct {
	index  int
	cursor int
	err    string
	fields []formField
}

type formField struct {
	label string
	value string
}

func newProbeForm(item appconfig.ProbeConfig, index int) *probeForm {
	return &probeForm{
		index: index,
		fields: []formField{
			{label: "Name", value: item.Name},
			{label: "Protocol", value: string(item.Protocol)},
			{label: "Host", value: item.Host},
			{label: "Port", value: strconv.Itoa(item.Port)},
			{label: "Timeout", value: item.Timeout.String()},
			{label: "Enabled", value: strconv.FormatBool(item.Enabled)},
			{label: "ID", value: item.ID},
		},
	}
}

func (f *probeForm) update(key tea.KeyMsg) {
	f.err = ""
	switch key.String() {
	case "up", "shift+tab":
		if f.cursor > 0 {
			f.cursor--
		}
	case "down", "tab":
		if f.cursor < len(f.fields)-2 {
			f.cursor++
		}
	case "backspace":
		value := f.fields[f.cursor].value
		if len(value) > 0 {
			f.fields[f.cursor].value = value[:len(value)-1]
		}
	case "ctrl+u":
		f.fields[f.cursor].value = ""
	default:
		if len(key.Runes) > 0 {
			f.fields[f.cursor].value += string(key.Runes)
		}
	}
}

func (f probeForm) value() (appconfig.ProbeConfig, int, error) {
	port, err := strconv.Atoi(strings.TrimSpace(f.fields[3].value))
	if err != nil || port < 1 || port > 65535 {
		return appconfig.ProbeConfig{}, f.index, fmt.Errorf("port must be 1-65535")
	}
	timeout, err := time.ParseDuration(strings.TrimSpace(f.fields[4].value))
	if err != nil || timeout <= 0 {
		return appconfig.ProbeConfig{}, f.index, fmt.Errorf("timeout must be a duration like 3s or 500ms")
	}
	enabled, err := strconv.ParseBool(strings.TrimSpace(f.fields[5].value))
	if err != nil {
		return appconfig.ProbeConfig{}, f.index, fmt.Errorf("enabled must be true or false")
	}
	protocol := probe.Protocol(strings.ToLower(strings.TrimSpace(f.fields[1].value)))
	switch protocol {
	case probe.ProtocolTCP, probe.ProtocolUDP, probe.ProtocolQUIC:
	default:
		return appconfig.ProbeConfig{}, f.index, fmt.Errorf("protocol must be tcp, udp, or quic")
	}
	id := strings.TrimSpace(f.fields[6].value)
	host := strings.TrimSpace(f.fields[2].value)
	if host == "" {
		return appconfig.ProbeConfig{}, f.index, fmt.Errorf("host is required")
	}
	if id == "" {
		id = appconfig.NewProbeID(protocol, host, port)
	}
	name := strings.TrimSpace(f.fields[0].value)
	if name == "" {
		name = fmt.Sprintf("%s %s:%d", protocol, host, port)
	}
	return appconfig.ProbeConfig{
		ID:       id,
		Name:     name,
		Protocol: protocol,
		Host:     host,
		Port:     port,
		Timeout:  timeout,
		Enabled:  enabled,
	}, f.index, nil
}

func (f probeForm) view() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Probe editor"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("up/down move | type edit | ctrl+u clear | enter save | esc cancel"))
	b.WriteString("\n\n")
	for i, field := range f.fields {
		if field.label == "ID" {
			continue
		}
		line := fmt.Sprintf("%-10s %s", field.label+":", field.value)
		if i == f.cursor {
			b.WriteString(okStyle.Render("> " + line))
		} else {
			b.WriteString("  " + line)
		}
		b.WriteByte('\n')
	}
	if f.err != "" {
		b.WriteString("\n")
		b.WriteString(failStyle.Render(f.err))
	}
	return strings.TrimRight(b.String(), "\n")
}
