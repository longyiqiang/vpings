package appconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/longyiqiang/vpings/internal/probe"
)

const (
	DefaultProbeInterval  = 60 * time.Second
	DefaultProbeTimeout   = 3 * time.Second
	DefaultSampleCount    = 10
	DefaultSampleInterval = time.Second
)

type Config struct {
	ProbeInterval         time.Duration `json:"probe_interval"`
	DefaultTimeout        time.Duration `json:"default_timeout"`
	DefaultSampleCount    int           `json:"default_sample_count"`
	DefaultSampleInterval time.Duration `json:"default_sample_interval"`
	AutoStart             bool          `json:"auto_start"`
	Probes                []ProbeConfig `json:"probes"`
}

type ProbeConfig struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Protocol       probe.Protocol `json:"protocol"`
	Host           string         `json:"host"`
	Port           int            `json:"port"`
	Timeout        time.Duration  `json:"timeout"`
	SampleCount    int            `json:"sample_count"`
	SampleInterval time.Duration  `json:"sample_interval"`
	Enabled        bool           `json:"enabled"`
}

func Default() Config {
	return Config{
		ProbeInterval:         DefaultProbeInterval,
		DefaultTimeout:        DefaultProbeTimeout,
		DefaultSampleCount:    DefaultSampleCount,
		DefaultSampleInterval: DefaultSampleInterval,
		AutoStart:             false,
		Probes: []ProbeConfig{
			{ID: "icmp-alidns", Name: "AliDNS ICMP", Protocol: probe.ProtocolICMP, Host: "dns.alidns.com", Port: 0, Timeout: DefaultProbeTimeout, SampleCount: DefaultSampleCount, SampleInterval: DefaultSampleInterval, Enabled: true},
			{ID: "tcp-alidns-443", Name: "AliDNS TCP 443", Protocol: probe.ProtocolTCP, Host: "dns.alidns.com", Port: 443, Timeout: DefaultProbeTimeout, SampleCount: DefaultSampleCount, SampleInterval: DefaultSampleInterval, Enabled: true},
			{ID: "udp-alidns-53", Name: "AliDNS UDP 53", Protocol: probe.ProtocolUDP, Host: "dns.alidns.com", Port: 53, Timeout: DefaultProbeTimeout, SampleCount: DefaultSampleCount, SampleInterval: DefaultSampleInterval, Enabled: true},
			{ID: "quic-alidns-853", Name: "AliDNS QUIC 853", Protocol: probe.ProtocolQUIC, Host: "dns.alidns.com", Port: 853, Timeout: DefaultProbeTimeout, SampleCount: DefaultSampleCount, SampleInterval: DefaultSampleInterval, Enabled: true},
		},
	}
}

func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.json"
	}
	return filepath.Join(home, ".vpings", "config.json")
}

func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, Save(path, cfg)
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	cfg.normalize()
	cfg.migrateCloudflareDefaults()
	cfg.migrateFastAliDNSDefaults()
	return cfg, nil
}

func Save(path string, cfg Config) error {
	cfg.normalize()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func (c *Config) EnabledSpecs() []probe.Spec {
	specs := make([]probe.Spec, 0, len(c.Probes))
	for _, item := range c.EnabledProbes() {
		specs = append(specs, item.Spec())
	}
	return specs
}

func (c *Config) EnabledProbes() []ProbeConfig {
	probes := make([]ProbeConfig, 0, len(c.Probes))
	for _, item := range c.Probes {
		if item.Enabled {
			probes = append(probes, item)
		}
	}
	return probes
}

func (p ProbeConfig) Spec() probe.Spec {
	return probe.Spec{
		Protocol: p.Protocol,
		ID:       p.ID,
		Name:     p.Name,
		Host:     p.Host,
		Port:     p.Port,
		Timeout:  p.Timeout,
	}
}

func NewProbeID(protocol probe.Protocol, host string, port int) string {
	return fmt.Sprintf("%s-%s-%d-%d", protocol, host, port, time.Now().Unix())
}

func (c *Config) normalize() {
	if c.ProbeInterval <= 0 {
		c.ProbeInterval = DefaultProbeInterval
	}
	if c.DefaultTimeout <= 0 {
		c.DefaultTimeout = DefaultProbeTimeout
	}
	if c.DefaultSampleCount <= 0 {
		c.DefaultSampleCount = DefaultSampleCount
	}
	if c.DefaultSampleInterval <= 0 {
		c.DefaultSampleInterval = DefaultSampleInterval
	}
	for i := range c.Probes {
		if c.Probes[i].ID == "" {
			c.Probes[i].ID = NewProbeID(c.Probes[i].Protocol, c.Probes[i].Host, c.Probes[i].Port)
		}
		if c.Probes[i].Name == "" {
			c.Probes[i].Name = fmt.Sprintf("%s %s:%d", c.Probes[i].Protocol, c.Probes[i].Host, c.Probes[i].Port)
		}
		if c.Probes[i].Timeout <= 0 {
			c.Probes[i].Timeout = c.DefaultTimeout
		}
		if c.Probes[i].SampleCount <= 0 {
			c.Probes[i].SampleCount = c.DefaultSampleCount
		}
		if c.Probes[i].SampleInterval <= 0 {
			c.Probes[i].SampleInterval = c.DefaultSampleInterval
		}
	}
}

func (c *Config) migrateCloudflareDefaults() {
	if len(c.Probes) != 3 {
		return
	}
	for _, item := range c.Probes {
		if item.Host != "cloudflare.com" {
			return
		}
	}
	c.Probes = Default().Probes
}

func (c *Config) migrateFastAliDNSDefaults() {
	if len(c.Probes) != 3 {
		return
	}
	defaultIDs := map[string]bool{
		"tcp-alidns-443":  true,
		"udp-alidns-53":   true,
		"quic-alidns-853": true,
	}
	for _, item := range c.Probes {
		if item.Host != "dns.alidns.com" || !defaultIDs[item.ID] {
			return
		}
	}
	if c.ProbeInterval == 2*time.Second {
		c.ProbeInterval = DefaultProbeInterval
	}
	if c.DefaultSampleCount == 5 {
		c.DefaultSampleCount = DefaultSampleCount
	}
	if c.DefaultSampleInterval == 200*time.Millisecond {
		c.DefaultSampleInterval = DefaultSampleInterval
	}
	for i := range c.Probes {
		if c.Probes[i].SampleCount == 5 {
			c.Probes[i].SampleCount = DefaultSampleCount
		}
		if c.Probes[i].SampleInterval == 200*time.Millisecond {
			c.Probes[i].SampleInterval = DefaultSampleInterval
		}
	}
	if len(c.Probes) == 3 {
		defaultICMP := Default().Probes[0]
		c.Probes = append([]ProbeConfig{defaultICMP}, c.Probes...)
	}
}
