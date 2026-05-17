package memory

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type JSONStore struct {
	path string
	mu   sync.Mutex
}

func NewJSONStore(path string) Store {
	if path == "" {
		path = DefaultMemoryPath
	}
	return &JSONStore{path: path}
}

func (js *JSONStore) Append(event MemoryEvent) error {
	js.mu.Lock()
	defer js.mu.Unlock()

	event = normalizeEvent(event)

	if err := os.MkdirAll(filepath.Dir(js.path), 0o700); err != nil {
		return fmt.Errorf("create memory dir: %w", err)
	}

	file, err := os.OpenFile(js.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open memory store: %w", err)
	}
	defer file.Close()

	if err := json.NewEncoder(file).Encode(event); err != nil {
		return fmt.Errorf("append memory event: %w", err)
	}

	return nil
}

func (js *JSONStore) Recent(sessionID string, limit int) ([]MemoryEvent, error) {
	events, err := js.readAll()
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = DefaultContextLimit
	}

	recent := make([]MemoryEvent, 0, limit)
	for _, event := range events {
		if sessionID != "" && event.SessionID != sessionID {
			continue
		}
		recent = append(recent, event)
		if len(recent) > limit {
			recent = recent[1:]
		}
	}

	return recent, nil
}

func (js *JSONStore) ByTag(tag string, limit int) ([]MemoryEvent, error) {
	events, err := js.readAll()
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = DefaultTaggedLimit
	}

	matches := make([]MemoryEvent, 0, limit)
	for _, event := range events {
		if !hasTag(event.Tags, tag) {
			continue
		}
		matches = append(matches, event)
		if len(matches) > limit {
			matches = matches[1:]
		}
	}

	return matches, nil
}

func (js *JSONStore) readAll() ([]MemoryEvent, error) {
	js.mu.Lock()
	defer js.mu.Unlock()

	file, err := os.Open(js.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open memory store: %w", err)
	}
	defer file.Close()

	var events []MemoryEvent
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for line := 1; scanner.Scan(); line++ {
		if scanner.Text() == "" {
			continue
		}
		var event MemoryEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, fmt.Errorf("decode memory event line %d: %w", line, err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read memory store: %w", err)
	}

	return events, nil
}

func normalizeEvent(event MemoryEvent) MemoryEvent {
	if event.ID == "" {
		event.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if event.SessionID == "" {
		event.SessionID = DefaultSessionID
	}
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	}
	return event
}

func hasTag(tags []string, want string) bool {
	for _, tag := range tags {
		if tag == want {
			return true
		}
	}
	return false
}
