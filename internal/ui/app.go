package ui

import (
	"context"
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
	cfg              appconfig.Config
	configPath       string
	storePath        string
	recorder         Recorder
	results          []probe.Result
	logs             []string
	active           appView
	probeIndex       int
	resultProbeIndex int
	resultDetail     bool
	setting          int
	startedAt        time.Time
	lastRun          time.Time
	form             *probeForm
	defaultsForm     *probeDefaultsForm
	err              error
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
	if len(m.cfg.EnabledProbes()) == 0 {
		return nil
	}
	return runConfiguredRound(m.cfg.EnabledProbes())
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.form != nil {
		return m.updateForm(msg)
	}
	if m.defaultsForm != nil {
		return m.updateDefaultsForm(msg)
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
			m.addLog(fmt.Sprintf("completed %d sample attempt(s)", len(msg)))
		}
		if len(m.results) > 20000 {
			m.results = m.results[len(m.results)-20000:]
		}
		if len(m.cfg.EnabledProbes()) == 0 {
			return m, nil
		}
		return m, waitTick(m.cfg.ProbeInterval)
	case tickMsg:
		if len(m.cfg.EnabledProbes()) == 0 {
			return m, nil
		}
		return m, runConfiguredRound(m.cfg.EnabledProbes())
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
		if len(m.cfg.EnabledProbes()) == 0 {
			m.addLog("no enabled probes to run")
			return m, nil
		}
		m.addLog("manual probe round started")
		return m, runConfiguredRound(m.cfg.EnabledProbes())
	}

	switch m.active {
	case viewResults:
		return m.updateResultKeys(msg)
	case viewProbes:
		return m.updateProbeKeys(msg)
	case viewSettings:
		return m.updateSettingKeys(msg)
	}
	return m, nil
}

func (m AppModel) updateResultKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	enabled := m.cfg.EnabledProbes()
	if len(enabled) == 0 {
		m.resultDetail = false
		m.resultProbeIndex = 0
		return m, nil
	}
	if m.resultProbeIndex >= len(enabled) {
		m.resultProbeIndex = len(enabled) - 1
	}
	switch msg.String() {
	case "esc":
		m.resultDetail = false
	case "up", "k":
		if m.resultProbeIndex > 0 {
			m.resultProbeIndex--
		}
	case "down", "j":
		if m.resultProbeIndex < len(enabled)-1 {
			m.resultProbeIndex++
		}
	case "enter":
		m.resultDetail = true
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
			ID:             appconfig.NewProbeID(probe.ProtocolICMP, "dns.alidns.com", 0),
			Name:           "New ICMP Probe",
			Protocol:       probe.ProtocolICMP,
			Host:           "dns.alidns.com",
			Port:           0,
			Timeout:        m.cfg.DefaultTimeout,
			SampleCount:    m.cfg.DefaultSampleCount,
			SampleInterval: m.cfg.DefaultSampleInterval,
			Enabled:        true,
		}, -1)
	case "g":
		m.defaultsForm = newProbeDefaultsForm(m.cfg)
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

func (m AppModel) updateDefaultsForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch key.String() {
	case "esc":
		m.defaultsForm = nil
		m.addLog("probe defaults edit cancelled")
		return m, nil
	case "enter":
		defaults, err := m.defaultsForm.value()
		if err != nil {
			m.defaultsForm.err = err.Error()
			return m, nil
		}
		m.cfg.ProbeInterval = defaults.probeInterval
		m.cfg.DefaultTimeout = defaults.defaultTimeout
		m.cfg.DefaultSampleCount = defaults.sampleCount
		m.cfg.DefaultSampleInterval = defaults.sampleInterval
		if defaults.applyExisting {
			for i := range m.cfg.Probes {
				m.cfg.Probes[i].Timeout = defaults.defaultTimeout
				m.cfg.Probes[i].SampleCount = defaults.sampleCount
				m.cfg.Probes[i].SampleInterval = defaults.sampleInterval
			}
		}
		m.defaultsForm = nil
		m.saveConfig("probe defaults saved")
		return m, nil
	default:
		m.defaultsForm.update(key)
	}
	return m, nil
}

func (m AppModel) View() string {
	if m.form != nil {
		return m.renderFrame(m.form.view())
	}
	if m.defaultsForm != nil {
		return m.renderFrame(m.defaultsForm.view())
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
		if appView(i) == m.active && m.form == nil && m.defaultsForm == nil {
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
	enabled := m.cfg.EnabledProbes()
	if len(enabled) == 0 {
		return mutedStyle.Render("No enabled probes. Open 3 Probes and enable or create one.")
	}
	if m.resultProbeIndex >= len(enabled) {
		m.resultProbeIndex = len(enabled) - 1
	}
	selected := enabled[m.resultProbeIndex]
	if m.resultDetail {
		return m.viewProbeDetail(selected)
	}

	var b strings.Builder
	b.WriteString(headerStyle.Render("Realtime latency"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("up/down select probe | enter detail | r run now"))
	b.WriteString("\n\n")
	b.WriteString(RenderRealtimeProbeChart(selected, m.results))
	b.WriteString("\n\n")
	b.WriteString(headerStyle.Render("Probes"))
	b.WriteString("\n")
	for i, item := range enabled {
		prefix := "  "
		if i == m.resultProbeIndex {
			prefix = "> "
			b.WriteString(okStyle.Render(prefix + formatProbeSelector(item, m.results)))
		} else {
			b.WriteString(prefix + formatProbeSelector(item, m.results))
		}
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m AppModel) viewProbeDetail(item appconfig.ProbeConfig) string {
	var b strings.Builder
	b.WriteString(headerStyle.Render(item.Name))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("esc back | r run now"))
	b.WriteString("\n\n")
	b.WriteString(RenderProbeDetailCharts(item, m.results, time.Now()))
	return b.String()
}

func (m AppModel) viewStatus() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Program status"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Uptime:       %s\n", time.Since(m.startedAt).Truncate(time.Second)))
	b.WriteString(fmt.Sprintf("Interval:     %s\n", m.cfg.ProbeInterval))
	b.WriteString(fmt.Sprintf("Enabled:      %d/%d probes\n", len(m.cfg.EnabledProbes()), len(m.cfg.Probes)))
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
	b.WriteString(mutedStyle.Render("g defaults | n new | e/enter edit | d delete | space enable/disable"))
	b.WriteString("\n\n")
	b.WriteString(headerStyle.Render("Defaults"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Round interval: %s | Timeout: %s | Samples: %d | Sample gap: %s\n\n",
		m.cfg.ProbeInterval,
		m.cfg.DefaultTimeout,
		m.cfg.DefaultSampleCount,
		m.cfg.DefaultSampleInterval,
	))
	if len(m.cfg.Probes) == 0 {
		b.WriteString(mutedStyle.Render("No probes configured. Press n to create one."))
		return b.String()
	}
	b.WriteString(headerStyle.Render(fmt.Sprintf("%-3s %-22s %-8s %-24s %-6s %-8s %-12s %s", "", "NAME", "PROTO", "HOST", "PORT", "TIMEOUT", "SAMPLES", "STATE")))
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
		samples := fmt.Sprintf("%dx/%s", item.SampleCount, item.SampleInterval)
		line := fmt.Sprintf("%-3s %-22s %-8s %-24s %-6s %-8s %-12s %s",
			cursor,
			truncate(item.Name, 22),
			item.Protocol,
			truncate(item.Host, 24),
			formatPort(item),
			item.Timeout,
			samples,
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

func runConfiguredRound(probes []appconfig.ProbeConfig) tea.Cmd {
	return func() tea.Msg {
		results := make([]probe.Result, 0)
		for _, item := range probes {
			roundID := fmt.Sprintf("%s-%d", item.ID, time.Now().UnixNano())
			for attempt := 1; attempt <= item.SampleCount; attempt++ {
				result := probe.Run(context.Background(), item.Spec())
				result.RoundID = roundID
				result.ProbeID = item.ID
				result.ProbeName = item.Name
				result.Attempt = attempt
				result.Attempts = item.SampleCount
				results = append(results, result)
				if attempt < item.SampleCount && item.SampleInterval > 0 {
					time.Sleep(item.SampleInterval)
				}
			}
		}
		return resultsMsg(results)
	}
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

func formatPort(item appconfig.ProbeConfig) string {
	if item.Protocol == probe.ProtocolICMP {
		return "-"
	}
	return strconv.Itoa(item.Port)
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

type probeDefaultsForm struct {
	cursor int
	err    string
	fields []formField
}

type probeDefaults struct {
	probeInterval  time.Duration
	defaultTimeout time.Duration
	sampleCount    int
	sampleInterval time.Duration
	applyExisting  bool
}

func newProbeDefaultsForm(cfg appconfig.Config) *probeDefaultsForm {
	return &probeDefaultsForm{
		fields: []formField{
			{label: "Round interval", value: cfg.ProbeInterval.String()},
			{label: "Timeout", value: cfg.DefaultTimeout.String()},
			{label: "Samples", value: strconv.Itoa(cfg.DefaultSampleCount)},
			{label: "Sample gap", value: cfg.DefaultSampleInterval.String()},
			{label: "Apply existing", value: "false"},
		},
	}
}

func (f *probeDefaultsForm) update(key tea.KeyMsg) {
	f.err = ""
	switch key.String() {
	case "up", "shift+tab":
		if f.cursor > 0 {
			f.cursor--
		}
	case "down", "tab":
		if f.cursor < len(f.fields)-1 {
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

func (f probeDefaultsForm) value() (probeDefaults, error) {
	probeInterval, err := parseDurationWithSecondsDefault(strings.TrimSpace(f.fields[0].value))
	if err != nil || probeInterval <= 0 {
		return probeDefaults{}, fmt.Errorf("round interval must be a duration like 60s or 60")
	}
	defaultTimeout, err := parseDurationWithSecondsDefault(strings.TrimSpace(f.fields[1].value))
	if err != nil || defaultTimeout <= 0 {
		return probeDefaults{}, fmt.Errorf("timeout must be a duration like 3s or 3")
	}
	sampleCount, err := strconv.Atoi(strings.TrimSpace(f.fields[2].value))
	if err != nil || sampleCount < 1 || sampleCount > 100 {
		return probeDefaults{}, fmt.Errorf("samples must be 1-100")
	}
	sampleInterval, err := parseDurationWithSecondsDefault(strings.TrimSpace(f.fields[3].value))
	if err != nil || sampleInterval < 0 {
		return probeDefaults{}, fmt.Errorf("sample gap must be a duration like 1s or 1")
	}
	applyExisting, err := strconv.ParseBool(strings.TrimSpace(f.fields[4].value))
	if err != nil {
		return probeDefaults{}, fmt.Errorf("apply existing must be true or false")
	}
	return probeDefaults{
		probeInterval:  probeInterval,
		defaultTimeout: defaultTimeout,
		sampleCount:    sampleCount,
		sampleInterval: sampleInterval,
		applyExisting:  applyExisting,
	}, nil
}

func (f probeDefaultsForm) view() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Probe defaults"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("up/down move | type edit | ctrl+u clear | enter save | esc cancel"))
	b.WriteString("\n\n")
	for i, field := range f.fields {
		line := fmt.Sprintf("%-16s %s", field.label+":", field.value)
		if i == f.cursor {
			b.WriteString(okStyle.Render("> " + line))
		} else {
			b.WriteString("  " + line)
		}
		b.WriteByte('\n')
	}
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("Apply existing=false only changes defaults for new probes. true also updates current probes."))
	if f.err != "" {
		b.WriteString("\n\n")
		b.WriteString(failStyle.Render(f.err))
	}
	return strings.TrimRight(b.String(), "\n")
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
			{label: "Samples", value: strconv.Itoa(item.SampleCount)},
			{label: "Sample gap", value: item.SampleInterval.String()},
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
	if err != nil {
		return appconfig.ProbeConfig{}, f.index, fmt.Errorf("port must be a number")
	}
	protocol := probe.Protocol(strings.ToLower(strings.TrimSpace(f.fields[1].value)))
	switch protocol {
	case probe.ProtocolTCP, probe.ProtocolUDP, probe.ProtocolQUIC, probe.ProtocolICMP:
	default:
		return appconfig.ProbeConfig{}, f.index, fmt.Errorf("protocol must be icmp, tcp, udp, or quic")
	}
	if protocol == probe.ProtocolICMP {
		port = 0
	} else if port < 1 || port > 65535 {
		return appconfig.ProbeConfig{}, f.index, fmt.Errorf("port must be 1-65535")
	}
	timeout, err := parseDurationWithSecondsDefault(strings.TrimSpace(f.fields[4].value))
	if err != nil || timeout <= 0 {
		return appconfig.ProbeConfig{}, f.index, fmt.Errorf("timeout must be a duration like 3s or 3")
	}
	sampleCount, err := strconv.Atoi(strings.TrimSpace(f.fields[5].value))
	if err != nil || sampleCount < 1 || sampleCount > 100 {
		return appconfig.ProbeConfig{}, f.index, fmt.Errorf("samples must be 1-100")
	}
	sampleInterval, err := parseDurationWithSecondsDefault(strings.TrimSpace(f.fields[6].value))
	if err != nil || sampleInterval < 0 {
		return appconfig.ProbeConfig{}, f.index, fmt.Errorf("sample gap must be a duration like 1s or 1")
	}
	enabled, err := strconv.ParseBool(strings.TrimSpace(f.fields[7].value))
	if err != nil {
		return appconfig.ProbeConfig{}, f.index, fmt.Errorf("enabled must be true or false")
	}
	id := strings.TrimSpace(f.fields[8].value)
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
		ID:             id,
		Name:           name,
		Protocol:       protocol,
		Host:           host,
		Port:           port,
		Timeout:        timeout,
		SampleCount:    sampleCount,
		SampleInterval: sampleInterval,
		Enabled:        enabled,
	}, f.index, nil
}

func parseDurationWithSecondsDefault(value string) (time.Duration, error) {
	if value == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		return time.Duration(seconds) * time.Second, nil
	}
	return time.ParseDuration(value)
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
