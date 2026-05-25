package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/longyiqiang/vpings/internal/appconfig"
	"github.com/longyiqiang/vpings/internal/probe"
	"github.com/longyiqiang/vpings/internal/store"
	"github.com/longyiqiang/vpings/internal/ui"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 2 {
		printUsage(args[0])
		return nil
	}

	switch args[1] {
	case "app", "menu":
		return runApp(args[2:])
	case "run":
		return runProbe(args[2:])
	case "watch":
		return runWatch(args[2:])
	case "help", "-h", "--help":
		printUsage(args[0])
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[1])
	}
}

func runApp(args []string) error {
	fs := flag.NewFlagSet("app", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	configPath := appconfig.DefaultPath()
	storePath := defaultStorePath()
	fs.StringVar(&configPath, "config", configPath, "app config path")
	fs.StringVar(&storePath, "store", storePath, "JSONL record path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := appconfig.Load(configPath)
	if err != nil {
		return err
	}
	recorder, err := store.OpenJSONL(storePath)
	if err != nil {
		return err
	}
	defer recorder.Close()

	history, err := store.ReadRecent(storePath, 50000)
	if err != nil {
		return err
	}

	model := ui.NewAppModel(cfg, configPath, storePath, recorder, history)
	_, err = tea.NewProgram(model, tea.WithAltScreen()).Run()
	return err
}

func runProbe(args []string) error {
	cfg, err := parseConfig("run", args)
	if err != nil {
		return err
	}

	recorder, err := store.OpenJSONL(cfg.storePath)
	if err != nil {
		return err
	}
	defer recorder.Close()

	results := make([]probe.Result, 0, len(cfg.specs)*cfg.count)
	for i := 0; i < cfg.count; i++ {
		for _, spec := range cfg.specs {
			result := probe.Run(context.Background(), spec)
			results = append(results, result)
			if err := recorder.Append(result); err != nil {
				return err
			}
		}
		if i < cfg.count-1 {
			time.Sleep(cfg.interval)
		}
	}

	fmt.Println(ui.RenderResults(results))
	fmt.Printf("\nrecords: %s\n", cfg.storePath)
	return nil
}

func runWatch(args []string) error {
	cfg, err := parseConfig("watch", args)
	if err != nil {
		return err
	}

	recorder, err := store.OpenJSONL(cfg.storePath)
	if err != nil {
		return err
	}
	defer recorder.Close()

	model := ui.NewWatchModel(cfg.specs, cfg.interval, recorder)
	_, err = tea.NewProgram(model, tea.WithAltScreen()).Run()
	return err
}

type config struct {
	target    string
	specs     []probe.Spec
	count     int
	interval  time.Duration
	timeout   time.Duration
	storePath string
}

func parseConfig(command string, args []string) (config, error) {
	var cfg config
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var tcpPorts, udpPorts, quicPorts string
	fs.StringVar(&cfg.target, "target", "", "target host or IP")
	fs.StringVar(&cfg.target, "t", "", "target host or IP")
	fs.StringVar(&tcpPorts, "tcp", "", "comma-separated TCP ports")
	fs.StringVar(&udpPorts, "udp", "", "comma-separated UDP ports")
	fs.StringVar(&quicPorts, "quic", "", "comma-separated QUIC ports")
	fs.IntVar(&cfg.count, "count", 4, "probe rounds")
	fs.IntVar(&cfg.count, "c", 4, "probe rounds")
	fs.DurationVar(&cfg.interval, "interval", time.Second, "interval between rounds")
	fs.DurationVar(&cfg.timeout, "timeout", 2*time.Second, "per-probe timeout")
	fs.StringVar(&cfg.storePath, "store", defaultStorePath(), "JSONL record path")

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if cfg.target == "" {
		return cfg, errors.New("target is required")
	}
	if cfg.count < 1 {
		return cfg, errors.New("count must be at least 1")
	}
	if command == "watch" {
		cfg.count = 1
	}

	specs, err := buildSpecs(cfg.target, cfg.timeout, tcpPorts, udpPorts, quicPorts)
	if err != nil {
		return cfg, err
	}
	if len(specs) == 0 {
		return cfg, errors.New("at least one of --tcp, --udp, or --quic is required")
	}
	cfg.specs = specs
	return cfg, nil
}

func buildSpecs(target string, timeout time.Duration, tcpPorts, udpPorts, quicPorts string) ([]probe.Spec, error) {
	var specs []probe.Spec
	for _, item := range []struct {
		protocol probe.Protocol
		ports    string
	}{
		{probe.ProtocolTCP, tcpPorts},
		{probe.ProtocolUDP, udpPorts},
		{probe.ProtocolQUIC, quicPorts},
	} {
		parsed, err := parsePorts(item.ports)
		if err != nil {
			return nil, fmt.Errorf("%s ports: %w", item.protocol, err)
		}
		for _, port := range parsed {
			specs = append(specs, probe.Spec{
				Protocol: item.protocol,
				Host:     target,
				Port:     port,
				Timeout:  timeout,
			})
		}
	}
	return specs, nil
}

func parsePorts(value string) ([]int, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}

	parts := strings.Split(value, ",")
	ports := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		port, err := strconv.Atoi(part)
		if err != nil {
			return nil, err
		}
		if port < 1 || port > 65535 {
			return nil, fmt.Errorf("port %d out of range", port)
		}
		ports = append(ports, port)
	}
	return ports, nil
}

func defaultStorePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "records.jsonl"
	}
	return filepath.Join(home, ".vpings", "records.jsonl")
}

func printUsage(name string) {
	fmt.Fprintf(os.Stderr, `vpings probes TCP, UDP, and QUIC latency.

Usage:
  %[1]s app
  %[1]s run --target HOST --tcp 80,443 --udp 53 --quic 443
  %[1]s watch --target HOST --tcp 443 --quic 443

Commands:
  app      open the interactive menu
  run      run finite probes and print a table
  watch    open an interactive refreshing terminal view
`, name)
}
