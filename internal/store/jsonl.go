package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/longyiqiang/vpings/internal/probe"
)

type JSONL struct {
	mu   sync.Mutex
	file *os.File
}

func OpenJSONL(path string) (*JSONL, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &JSONL{file: file}, nil
}

func (j *JSONL) Append(result probe.Result) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	data, err := json.Marshal(result)
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
