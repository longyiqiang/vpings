package ui

import (
	"testing"
	"time"

	"github.com/longyiqiang/vpings/internal/appconfig"
)

func TestProbeDefaultsFormValue(t *testing.T) {
	form := newProbeDefaultsForm(appconfig.Config{
		ProbeInterval:         appconfig.DefaultProbeInterval,
		DefaultTimeout:        appconfig.DefaultProbeTimeout,
		DefaultSampleCount:    appconfig.DefaultSampleCount,
		DefaultSampleInterval: appconfig.DefaultSampleInterval,
	})
	form.fields[0].value = "60"
	form.fields[1].value = "3"
	form.fields[2].value = "10"
	form.fields[3].value = "0"
	form.fields[4].value = "true"

	value, err := form.value()
	if err != nil {
		t.Fatal(err)
	}
	if value.probeInterval != 60*time.Second {
		t.Fatalf("probeInterval = %s, want 60s", value.probeInterval)
	}
	if value.defaultTimeout != 3*time.Second {
		t.Fatalf("defaultTimeout = %s, want 3s", value.defaultTimeout)
	}
	if value.sampleCount != 10 {
		t.Fatalf("sampleCount = %d, want 10", value.sampleCount)
	}
	if value.sampleInterval != 0 {
		t.Fatalf("sampleInterval = %s, want 0s", value.sampleInterval)
	}
	if !value.applyExisting {
		t.Fatal("applyExisting = false, want true")
	}
}
