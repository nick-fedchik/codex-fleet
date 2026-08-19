package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

var ErrWorkerNotFound = errors.New("worker not found")

type Worker struct {
	Name           string     `json:"name"`
	SSHHost        string     `json:"ssh_host"`
	SSHUser        string     `json:"ssh_user,omitempty"`
	Port           int        `json:"port,omitempty"`
	IdentityFile   string     `json:"identity_file,omitempty"`
	ExpectedModels []string   `json:"expected_models,omitempty"`
	LastState      string     `json:"last_state,omitempty"`
	LastCheckedAt  *time.Time `json:"last_checked_at,omitempty"`
	LastError      string     `json:"last_error,omitempty"`
}

type Registry struct {
	Workers []Worker `json:"workers"`
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(dir, "codex-fleet", "workers.json"), nil
}

func Load(path string) (Registry, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Registry{Workers: []Worker{}}, nil
	}
	if err != nil {
		return Registry{}, fmt.Errorf("read registry %s: %w", path, err)
	}
	var registry Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		return Registry{}, fmt.Errorf("parse registry %s: %w", path, err)
	}
	sort.Slice(registry.Workers, func(i, j int) bool {
		return registry.Workers[i].Name < registry.Workers[j].Name
	})
	return registry, nil
}

func Save(path string, registry Registry) error {
	if registry.Workers == nil {
		registry.Workers = []Worker{}
	}
	sort.Slice(registry.Workers, func(i, j int) bool {
		return registry.Workers[i].Name < registry.Workers[j].Name
	})
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return fmt.Errorf("encode registry: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create registry directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".workers-*.tmp")
	if err != nil {
		return fmt.Errorf("create registry temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set registry permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write registry: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close registry: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace registry: %w", err)
	}
	return nil
}

func (r *Registry) Find(name string) (*Worker, error) {
	for i := range r.Workers {
		if r.Workers[i].Name == name {
			return &r.Workers[i], nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrWorkerNotFound, name)
}

func (r *Registry) Upsert(worker Worker) {
	for i := range r.Workers {
		if r.Workers[i].Name == worker.Name {
			r.Workers[i] = worker
			return
		}
	}
	r.Workers = append(r.Workers, worker)
}

func (r *Registry) Remove(name string) error {
	for i := range r.Workers {
		if r.Workers[i].Name == name {
			r.Workers = append(r.Workers[:i], r.Workers[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("%w: %s", ErrWorkerNotFound, name)
}
