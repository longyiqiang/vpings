package probe

import (
	"context"
	"fmt"
	"time"
)

type Protocol string

const (
	ProtocolTCP  Protocol = "tcp"
	ProtocolUDP  Protocol = "udp"
	ProtocolQUIC Protocol = "quic"
	ProtocolICMP Protocol = "icmp"
)

type Status string

const (
	StatusOK          Status = "ok"
	StatusFailed      Status = "failed"
	StatusSentNoReply Status = "sent_no_reply"
)

type Spec struct {
	Protocol Protocol      `json:"protocol"`
	ID       string        `json:"id,omitempty"`
	Name     string        `json:"name,omitempty"`
	Host     string        `json:"host"`
	Port     int           `json:"port"`
	Timeout  time.Duration `json:"timeout"`
}

type Result struct {
	StartedAt   time.Time     `json:"started_at"`
	RoundID     string        `json:"round_id,omitempty"`
	ProbeID     string        `json:"probe_id,omitempty"`
	ProbeName   string        `json:"probe_name,omitempty"`
	Attempt     int           `json:"attempt,omitempty"`
	Attempts    int           `json:"attempts,omitempty"`
	Protocol    Protocol      `json:"protocol"`
	Host        string        `json:"host"`
	Port        int           `json:"port"`
	Status      Status        `json:"status"`
	Duration    time.Duration `json:"duration"`
	Error       string        `json:"error,omitempty"`
	Description string        `json:"description,omitempty"`
}

func Run(parent context.Context, spec Spec) Result {
	ctx, cancel := context.WithTimeout(parent, spec.Timeout)
	defer cancel()

	start := time.Now()
	result := Result{
		StartedAt: start,
		ProbeID:   spec.ID,
		ProbeName: spec.Name,
		Protocol:  spec.Protocol,
		Host:      spec.Host,
		Port:      spec.Port,
		Status:    StatusFailed,
	}

	var err error
	switch spec.Protocol {
	case ProtocolTCP:
		err = probeTCP(ctx, spec)
	case ProtocolUDP:
		err = probeUDP(ctx, spec, &result)
	case ProtocolQUIC:
		err = probeQUIC(ctx, spec)
	case ProtocolICMP:
		err = probeICMP(ctx, spec)
	default:
		err = fmt.Errorf("unsupported protocol %q", spec.Protocol)
	}

	result.Duration = time.Since(start)
	if err != nil {
		result.Error = err.Error()
		if result.Status == StatusFailed {
			result.Description = result.Error
		}
		return result
	}
	if result.Status == StatusFailed {
		result.Status = StatusOK
	}
	return result
}
