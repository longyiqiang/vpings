package appconfig

import (
	"testing"
	"time"

	"github.com/longyiqiang/vpings/internal/probe"
)

func TestDefaultProbeCadence(t *testing.T) {
	cfg := Default()
	if cfg.ProbeInterval != DefaultProbeInterval {
		t.Fatalf("ProbeInterval = %s, want %s", cfg.ProbeInterval, DefaultProbeInterval)
	}
	if cfg.DefaultSampleCount != DefaultSampleCount {
		t.Fatalf("DefaultSampleCount = %d, want %d", cfg.DefaultSampleCount, DefaultSampleCount)
	}
	if cfg.DefaultSampleInterval != DefaultSampleInterval {
		t.Fatalf("DefaultSampleInterval = %s, want %s", cfg.DefaultSampleInterval, DefaultSampleInterval)
	}
	for _, item := range cfg.Probes {
		if item.SampleCount != DefaultSampleCount {
			t.Fatalf("%s SampleCount = %d, want %d", item.ID, item.SampleCount, DefaultSampleCount)
		}
		if item.SampleInterval != DefaultSampleInterval {
			t.Fatalf("%s SampleInterval = %s, want %s", item.ID, item.SampleInterval, DefaultSampleInterval)
		}
	}
}

func TestDefaultIncludesICMPProbe(t *testing.T) {
	cfg := Default()
	for _, item := range cfg.Probes {
		if item.Protocol == probe.ProtocolICMP && item.Host == "dns.alidns.com" {
			return
		}
	}
	t.Fatal("default config does not include AliDNS ICMP probe")
}

func TestMigrateFastAliDNSDefaultsRemovesSampleDelay(t *testing.T) {
	cfg := Config{
		ProbeInterval:         DefaultProbeInterval,
		DefaultTimeout:        DefaultProbeTimeout,
		DefaultSampleCount:    DefaultSampleCount,
		DefaultSampleInterval: time.Second,
		Probes: []ProbeConfig{
			{ID: "icmp-alidns", Host: "dns.alidns.com", SampleInterval: time.Second},
			{ID: "tcp-alidns-443", Host: "dns.alidns.com", SampleInterval: time.Second},
			{ID: "udp-alidns-53", Host: "dns.alidns.com", SampleInterval: time.Second},
			{ID: "quic-alidns-853", Host: "dns.alidns.com", SampleInterval: time.Second},
		},
	}

	cfg.migrateFastAliDNSDefaults()
	if cfg.DefaultSampleInterval != DefaultSampleInterval {
		t.Fatalf("DefaultSampleInterval = %s, want %s", cfg.DefaultSampleInterval, DefaultSampleInterval)
	}
	for _, item := range cfg.Probes {
		if item.SampleInterval != DefaultSampleInterval {
			t.Fatalf("%s SampleInterval = %s, want %s", item.ID, item.SampleInterval, DefaultSampleInterval)
		}
	}
}
