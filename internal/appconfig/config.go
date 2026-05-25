package appconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/longyiqiang/vpings/internal/probe"
)

type Config struct {
	ProbeInterval  time.Duration `json:"probe_interval"`
	DefaultTimeout time.Duration `json:"default_timeout"`
	AutoStart      bool          `json:"auto_start"`
	Probes         []ProbeConfig `json:"probes"`
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
		ProbeInterval:  2 * time.Second,
		DefaultTimeout: 3 * time.Second,
		AutoStart:      false,
		Probes: []ProbeConfig{
			{ID: "tcp-alidns-443", Name: "AliDNS TCP 443", Protocol: probe.ProtocolTCP, Host: "dns.alidns.com", Port: 443, Timeout: 3 * time.Second, SampleCount: 5, SampleInterval: 200 * time.Millisecond, Enabled: true},
			{ID: "udp-alidns-53", Name: "AliDNS UDP 53", Protocol: probe.ProtocolUDP, Host: "dns.alidns.com", Port: 53, Timeout: 3 * time.Second, SampleCount: 5, SampleInterval: 200 * time.Millisecond, Enabled: true},
			{ID: "quic-alidns-853", Name: "AliDNS QUIC 853", Protocol: probe.ProtocolQUIC, Host: "dns.alidns.com", Port: 853, Timeout: 3 * time.Second, SampleCount: 5, SampleInterval: 200 * time.Millisecond, Enabled: true},
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
		c.ProbeInterval = 2 * time.Second
	}
	if c.DefaultTimeout <= 0 {
		c.DefaultTimeout = 3 * time.Second
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
			c.Probes[i].SampleCount = 5
		}
		if c.Probes[i].SampleInterval <= 0 {
			c.Probes[i].SampleInterval = 200 * time.Millisecond
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
