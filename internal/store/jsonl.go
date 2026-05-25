package store

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/longyiqiang/vpings/internal/probe"
)

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
		var result probe.Result
		if err := json.Unmarshal(scanner.Bytes(), &result); err != nil {
			continue
		}
		results = append(results, result)
		if limit > 0 && len(results) > limit {
			results = results[len(results)-limit:]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return results, nil
}
