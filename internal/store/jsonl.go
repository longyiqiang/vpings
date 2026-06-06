package store

import (
	"bufio"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/longyiqiang/vpings/internal/probe"
)

type probeRecord struct {
	StartedAt    time.Time      `json:"started_at"`
	RoundID      string         `json:"round_id,omitempty"`
	ProbeID      string         `json:"probe_id,omitempty"`
	ProbeName    string         `json:"probe_name,omitempty"`
	Attempt      int            `json:"attempt,omitempty"`
	AttemptCount int            `json:"attempt_count,omitempty"`
	Protocol     probe.Protocol `json:"protocol"`
	Host         string         `json:"host"`
	Port         int            `json:"port"`
	Status       probe.Status   `json:"status"`
	DurationMS   float64        `json:"duration_ms"`
	Error        string         `json:"error,omitempty"`
	Description  string         `json:"description,omitempty"`

	LegacyDuration time.Duration `json:"duration,omitempty"`
	LegacyAttempts int           `json:"attempts,omitempty"`
}

type JSONL struct {
	mu   sync.Mutex
	file *os.File
	path string
}

func OpenJSONL(path string) (*JSONL, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &JSONL{file: file, path: path}, nil
}

func (j *JSONL) Append(result probe.Result) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	data, err := json.Marshal(recordFromResult(result))
	if err != nil {
		return err
	}
	if _, err := j.file.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

func (j *JSONL) Close() error {
	return j.file.Close()
}

func (j *JSONL) Path() string {
	return j.path
}

func ReadRecent(path string, limit int) ([]probe.Result, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var results []probe.Result
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record probeRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			continue
		}
		results = append(results, record.result())
		if limit > 0 && len(results) > limit {
			results = results[len(results)-limit:]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func recordFromResult(result probe.Result) probeRecord {
	return probeRecord{
		StartedAt:    result.StartedAt,
		RoundID:      result.RoundID,
		ProbeID:      result.ProbeID,
		ProbeName:    result.ProbeName,
		Attempt:      result.Attempt,
		AttemptCount: result.Attempts,
		Protocol:     result.Protocol,
		Host:         result.Host,
		Port:         result.Port,
		Status:       result.Status,
		DurationMS:   durationToMilliseconds(result.Duration),
		Error:        result.Error,
		Description:  result.Description,
	}
}

func (r probeRecord) result() probe.Result {
	duration := millisecondsToDuration(r.DurationMS)
	if duration == 0 && r.LegacyDuration > 0 {
		duration = r.LegacyDuration
	}
	attemptCount := r.AttemptCount
	if attemptCount == 0 {
		attemptCount = r.LegacyAttempts
	}
	return probe.Result{
		StartedAt:   r.StartedAt,
		RoundID:     r.RoundID,
		ProbeID:     r.ProbeID,
		ProbeName:   r.ProbeName,
		Attempt:     r.Attempt,
		Attempts:    attemptCount,
		Protocol:    r.Protocol,
		Host:        r.Host,
		Port:        r.Port,
		Status:      r.Status,
		Duration:    duration,
		Error:       r.Error,
		Description: r.Description,
	}
}

func durationToMilliseconds(value time.Duration) float64 {
	return float64(value) / float64(time.Millisecond)
}

func millisecondsToDuration(value float64) time.Duration {
	if value <= 0 {
		return 0
	}
	return time.Duration(math.Round(value * float64(time.Millisecond)))
}
