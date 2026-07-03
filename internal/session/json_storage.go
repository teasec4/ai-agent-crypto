package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type JSONStorage struct {
	path string
}

func NewJSONStorage(path string) *JSONStorage {
	return &JSONStorage{path: path}
}

func (s *JSONStorage) Load() ([]PersistedState, error) {
	if s == nil || s.path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read session storage %q: %w", s.path, err)
	}
	if len(data) == 0 {
		return nil, nil
	}

	var file sessionFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("failed to parse session storage %q: %w", s.path, err)
	}
	return file.Sessions, nil
}

func (s *JSONStorage) Save(states []PersistedState) error {
	if s == nil || s.path == "" {
		return nil
	}

	file := sessionFile{Sessions: states}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session storage: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create session storage directory %q: %w", dir, err)
	}

	tmpFile, err := os.CreateTemp(dir, ".sessions-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary session storage file: %w", err)
	}
	tmpPath := tmpFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write temporary session storage file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary session storage file: %w", err)
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("failed to replace session storage file %q: %w", s.path, err)
	}
	cleanup = false
	return nil
}

type sessionFile struct {
	Sessions []PersistedState `json:"sessions"`
}
