package applog

import (
	"fmt"
	"sync"
	"time"

	"second-nature/internal/model"
)

const logCapacity = 200

type appLogger struct {
	mu      sync.Mutex
	entries []model.LogEntry
	nextIdx int
}

var AppLog = &appLogger{}

func (l *appLogger) Log(level, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("[%s] %s\n", level, msg)

	l.mu.Lock()
	entry := model.LogEntry{
		Time:    time.Now(),
		Level:   level,
		Message: msg,
		Index:   l.nextIdx,
	}
	l.nextIdx++
	if len(l.entries) >= logCapacity {
		l.entries = l.entries[1:]
	}
	l.entries = append(l.entries, entry)
	l.mu.Unlock()
}

func (l *appLogger) Info(format string, args ...interface{}) {
	l.Log("info", format, args...)
}

func (l *appLogger) Warn(format string, args ...interface{}) {
	l.Log("warn", format, args...)
}

func (l *appLogger) Error(format string, args ...interface{}) {
	l.Log("error", format, args...)
}

func (l *appLogger) Since(after int) []model.LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	var result []model.LogEntry
	for _, e := range l.entries {
		if e.Index > after {
			result = append(result, e)
		}
	}
	return result
}
