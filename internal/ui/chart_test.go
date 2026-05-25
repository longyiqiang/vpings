package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/longyiqiang/vpings/internal/probe"
)

func TestRenderLatencyCharts(t *testing.T) {
	now := time.Now()
	results := []probe.Result{
		{StartedAt: now, Protocol: probe.ProtocolTCP, Host: "example.com", Port: 443, Duration: 10 * time.Millisecond},
		{StartedAt: now.Add(time.Second), Protocol: probe.ProtocolTCP, Host: "example.com", Port: 443, Duration: 20 * time.Millisecond},
	}

	chart := RenderLatencyCharts(results)
	if !strings.Contains(chart, "tcp example.com:443 ms") {
		t.Fatalf("expected chart caption, got:\n%s", chart)
	}
}
