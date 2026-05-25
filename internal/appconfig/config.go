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
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Protocol probe.Protocol `json:"protocol"`
	Host     string         `json:"host"`
	Port     int            `json:"port"`
	Timeout  time.Duration  `json:"timeout"`
	Enabled  bool           `json:"enabled"`
}

func Default() Config {
	return Config{
		ProbeInterval:  2 * time.Second,
		DefaultTimeout: 3 * time.Second,
		AutoStart:      false,
		Probes: []ProbeConfig{
			{ID: "tcp-cloudflare-443", Name: "Cloudflare TCP 443", Protocol: probe.ProtocolTCP, Host: "cloudflare.com", Port: 443, Timeout: 3 * time.Second, Enabled: true},
			{ID: "udp-cloudflare-53", Name: "Cloudflare UDP 53", Protocol: probe.ProtocolUDP, Host: "cloudflare.com", Port: 53, Timeout: 3 * time.Second, Enabled: true},
			{ID: "quic-cloudflare-443", Name: "Cloudflare QUIC 443", Protocol: probe.ProtocolQUIC, Host: "cloudflare.com", Port: 443, Timeout: 3 * time.Second, Enabled: true},
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
	for _, item := range c.Probes {
		if !item.Enabled {
			continue
		}
		specs = append(specs, item.Spec())
	}
	return specs
}

func (p ProbeConfig) Spec() probe.Spec {
	return probe.Spec{
		Protocol: p.Protocol,
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
	}
}
