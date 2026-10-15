package logger

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"time"
)

// Level represents log level
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "debug"
	case INFO:
		return "info"
	case WARN:
		return "warn"
	case ERROR:
		return "error"
	default:
		return "unknown"
	}
}

// Logger is a structured JSON logger
type Logger struct {
	level  Level
	output io.Writer
}

// New creates a new logger
func New(level Level, output io.Writer) *Logger {
	if output == nil {
		output = os.Stdout
	}
	return &Logger{
		level:  level,
		output: output,
	}
}

// NewDefault creates a default logger (INFO level, stdout)
func NewDefault() *Logger {
	return New(INFO, os.Stdout)
}

// logEntry represents a structured log entry
type logEntry struct {
	Level   string      `json:"level"`
	Time    string      `json:"time"`
	Message string      `json:"message"`
	Fields  interface{} `json:"fields,omitempty"`
}

func (l *Logger) log(level Level, message string, fields map[string]interface{}) {
	if level < l.level {
		return
	}

	entry := logEntry{
		Level:   level.String(),
		Time:    time.Now().UTC().Format(time.RFC3339),
		Message: message,
	}
	if len(fields) > 0 {
		entry.Fields = fields
	}

	data, err := json.Marshal(entry)
	if err != nil {
		log.Printf("failed to marshal log entry: %v", err)
		return
	}

	l.output.Write(append(data, '\n'))
}

// Debug logs a debug message
func (l *Logger) Debug(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(DEBUG, message, f)
}

// Info logs an info message
func (l *Logger) Info(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(INFO, message, f)
}

// Warn logs a warning message
func (l *Logger) Warn(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(WARN, message, f)
}

// Error logs an error message
func (l *Logger) Error(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(ERROR, message, f)
}
