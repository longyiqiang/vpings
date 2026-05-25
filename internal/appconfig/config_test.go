package appconfig

import (
	"testing"
	"time"

	"github.com/longyiqiang/vpings/internal/probe"
)

func TestDefaultProbeCadence(t *testing.T) {
	cfg := Default()
	if cfg.ProbeInterval != 60*time.Second {
		t.Fatalf("ProbeInterval = %s, want 60s", cfg.ProbeInterval)
	}
	if cfg.DefaultSampleCount != 10 {
		t.Fatalf("DefaultSampleCount = %d, want 10", cfg.DefaultSampleCount)
	}
	if cfg.DefaultSampleInterval != time.Second {
		t.Fatalf("DefaultSampleInterval = %s, want 1s", cfg.DefaultSampleInterval)
	}
	for _, item := range cfg.Probes {
		if item.SampleCount != 10 {
			t.Fatalf("%s SampleCount = %d, want 10", item.ID, item.SampleCount)
		}
		if item.SampleInterval != time.Second {
			t.Fatalf("%s SampleInterval = %s, want 1s", item.ID, item.SampleInterval)
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
