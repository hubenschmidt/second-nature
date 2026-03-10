package main

import (
	"fmt"
	"sync"
	"time"
)

const logCapacity = 200

type LogEntry struct {
	Time    time.Time
	Level   string
	Message string
	Index   int
}

type appLogger struct {
	mu      sync.Mutex
	entries []LogEntry
	nextIdx int
}

var AppLog = &appLogger{}

func (l *appLogger) Log(level, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("[%s] %s\n", level, msg)

	l.mu.Lock()
	entry := LogEntry{
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

func (l *appLogger) Since(after int) []LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	var result []LogEntry
	for _, e := range l.entries {
		if e.Index > after {
			result = append(result, e)
		}
	}
	return result
}
