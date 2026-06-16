package logger

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type EventType string

const (
	EventSyncStarted   EventType = "sync_started"
	EventSyncComplete  EventType = "sync_complete"
	EventSyncFailed    EventType = "sync_failed"
	EventSyncCancelled EventType = "sync_cancelled"
)

type LogEntry struct {
	Timestamp        time.Time `json:"timestamp"`
	Event            EventType `json:"event"`
	NetworkLabel     string    `json:"network_label,omitempty"`
	FilesCopied      int       `json:"files_copied,omitempty"`
	FilesSkipped     int       `json:"files_skipped,omitempty"`
	BytesTransferred int64     `json:"bytes_transferred,omitempty"`
	DurationMs       int64     `json:"duration_ms,omitempty"`
	ErrorMessage     string    `json:"error,omitempty"`
	CancelReason     string    `json:"cancel_reason,omitempty"`
}

type Logger struct {
	path string
	mu   sync.Mutex
}

// Path returns the default log file path, mirroring config.Path() conventions.
func Path() string {
	base := os.Getenv("APPDATA")
	if base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "WifiSync", "sync.log")
}

func New(path string) *Logger {
	return &Logger{path: path}
}

// Log appends one JSON-lines entry to the log file.
func (l *Logger) Log(entry LogEntry) error {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(data, '\n'))
	return err
}

// Tail returns the last n entries from the log file.
// Returns an empty slice (not an error) if the file does not exist yet.
// Malformed lines are silently skipped.
func (l *Logger) Tail(n int) ([]LogEntry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := os.Open(l.path)
	if os.IsNotExist(err) {
		return []LogEntry{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []LogEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry LogEntry
		if json.Unmarshal(line, &entry) != nil {
			continue
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(entries) <= n {
		return entries, nil
	}
	return entries[len(entries)-n:], nil
}
