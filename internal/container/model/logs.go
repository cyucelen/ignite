package model

import (
	"time"
)

type LogStream string

const (
	LogStreamStdout LogStream = "stdout"
	LogStreamStderr LogStream = "stderr"
)

type LogEntry struct {
	ContainerID string    `json:"container_id"`
	Stream      LogStream `json:"stream"`
	Timestamp   time.Time `json:"timestamp"`
	Message     string    `json:"message"`
	LineNumber  int       `json:"line_number"`
}

func NewLogEntry(containerID string, stream LogStream, message string, lineNumber int) *LogEntry {
	return &LogEntry{
		ContainerID: containerID,
		Stream:      stream,
		Timestamp:   time.Now(),
		Message:     message,
		LineNumber:  lineNumber,
	}
}

func (l *LogEntry) Validate() error {
	if l.ContainerID == "" {
		return ErrInvalidContainerID
	}
	if l.Stream != LogStreamStdout && l.Stream != LogStreamStderr {
		return ErrInvalidLogStream
	}
	if l.LineNumber < 1 {
		return ErrInvalidLogEntry
	}
	return nil
}

func (l *LogEntry) IsError() bool {
	return l.Stream == LogStreamStderr
}

type LogFilter struct {
	Since      *time.Time
	Until      *time.Time
	Tail       int
	Follow     bool
	Timestamps bool
}

func NewLogFilter() *LogFilter {
	return &LogFilter{
		Tail:       100,
		Follow:     false,
		Timestamps: true,
	}
}

func (f *LogFilter) Validate() error {
	if f.Since != nil && f.Until != nil && f.Since.After(*f.Until) {
		return ErrInvalidTimeRange
	}
	if f.Tail < 0 {
		return ErrInvalidLogFilter
	}
	return nil
}
