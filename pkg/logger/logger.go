package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type LogLevel string

const (
	LevelInfo  LogLevel = "INFO"
	LevelWarn  LogLevel = "WARN"
	LevelError LogLevel = "ERROR"
)

type LogEntry struct {
	ID        int64    `json:"id"`
	Timestamp int64    `json:"timestamp_ms"`
	TimeStr   string   `json:"time_str"`
	Level     LogLevel `json:"level"`
	Tag       string   `json:"tag"`
	Message   string   `json:"message"`
}

type RingLogger struct {
	mu         sync.RWMutex
	maxEntries int
	entries    []LogEntry
	counter    int64
	listeners  []func(entry LogEntry)
}

var (
	defaultLogger *RingLogger
	once          sync.Once
)

func GetLogger() *RingLogger {
	once.Do(func() {
		defaultLogger = NewRingLogger(2000)
	})
	return defaultLogger
}

func NewRingLogger(maxEntries int) *RingLogger {
	if maxEntries <= 0 {
		maxEntries = 2000
	}
	return &RingLogger{
		maxEntries: maxEntries,
		entries:    make([]LogEntry, 0, maxEntries),
	}
}

func (l *RingLogger) Log(level LogLevel, tag, message string) {
	now := time.Now()
	entry := LogEntry{
		Timestamp: now.UnixMilli(),
		TimeStr:   now.Format("15:04:05.000"),
		Level:     level,
		Tag:       tag,
		Message:   message,
	}

	l.mu.Lock()
	l.counter++
	entry.ID = l.counter

	if len(l.entries) >= l.maxEntries {
		l.entries = append(l.entries[1:], entry)
	} else {
		l.entries = append(l.entries, entry)
	}
	listeners := append([]func(entry LogEntry){}, l.listeners...)
	l.mu.Unlock()

	for _, cb := range listeners {
		cb(entry)
	}

	// Also print to console for terminal debugging
	fmt.Printf("[%s] [%s] [%s] %s\n", entry.TimeStr, level, tag, message)
}

func (l *RingLogger) Info(tag, message string) {
	l.Log(LevelInfo, tag, message)
}

func (l *RingLogger) Warn(tag, message string) {
	l.Log(LevelWarn, tag, message)
}

func (l *RingLogger) Error(tag, message string) {
	l.Log(LevelError, tag, message)
}

func (l *RingLogger) RegisterListener(cb func(entry LogEntry)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.listeners = append(l.listeners, cb)
}

func (l *RingLogger) GetEntries(filterLevel string) []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if filterLevel == "" || strings.EqualFold(filterLevel, "ALL") {
		result := make([]LogEntry, len(l.entries))
		copy(result, l.entries)
		return result
	}

	var filtered []LogEntry
	for _, e := range l.entries {
		if strings.EqualFold(filterLevel, "PROBLEMS") {
			if e.Level == LevelWarn || e.Level == LevelError {
				filtered = append(filtered, e)
			}
		} else if strings.EqualFold(string(e.Level), filterLevel) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func (l *RingLogger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = make([]LogEntry, 0, l.maxEntries)
}

func (l *RingLogger) ExportAsText() string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var sb strings.Builder
	for _, e := range l.entries {
		dateStr := time.UnixMilli(e.Timestamp).Format("2006-01-02 15:04:05.000")
		sb.WriteString(fmt.Sprintf("%s %-5s [%s] %s\n", dateStr, e.Level, e.Tag, e.Message))
	}
	return sb.String()
}

func (l *RingLogger) SaveToFile(dir string) (string, error) {
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			dir = "."
		} else {
			dir = filepath.Join(home, "Downloads")
		}
	}
	_ = os.MkdirAll(dir, 0755)

	stamp := time.Now().Format("20060102-150405")
	filePath := filepath.Join(dir, fmt.Sprintf("foxyvpn-logs-%s.txt", stamp))
	content := l.ExportAsText()
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return "", err
	}
	return filePath, nil
}
