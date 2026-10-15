package logger

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDefault(t *testing.T) {
	log := NewDefault()
	assert.NotNil(t, log)
}

func TestNew(t *testing.T) {
	var buf bytes.Buffer
	log := New(DEBUG, &buf)
	assert.NotNil(t, log)
}

func TestInfo(t *testing.T) {
	var buf bytes.Buffer
	log := New(INFO, &buf)
	log.Info("test info")
	assert.Contains(t, buf.String(), "test info")
	assert.Contains(t, buf.String(), "\"level\":\"info\"")
}

func TestInfoWithFields(t *testing.T) {
	var buf bytes.Buffer
	log := New(INFO, &buf)
	log.Info("test info with fields", map[string]interface{}{"key": "value"})
	output := buf.String()
	assert.Contains(t, output, "test info with fields")
	assert.Contains(t, output, "\"fields\"")
}

func TestWarn(t *testing.T) {
	var buf bytes.Buffer
	log := New(INFO, &buf)
	log.Warn("test warn")
	assert.Contains(t, buf.String(), "\"level\":\"warn\"")
}

func TestError(t *testing.T) {
	var buf bytes.Buffer
	log := New(INFO, &buf)
	log.Error("test error")
	assert.Contains(t, buf.String(), "\"level\":\"error\"")
}

func TestErrorWithFields(t *testing.T) {
	var buf bytes.Buffer
	log := New(INFO, &buf)
	log.Error("test error with fields", map[string]interface{}{"error": "something"})
	assert.Contains(t, buf.String(), "something")
}

func TestDebug_NotLogged(t *testing.T) {
	var buf bytes.Buffer
	log := New(INFO, &buf) // INFO level, DEBUG should not be logged
	log.Debug("debug message")
	assert.Empty(t, buf.String())
}

func TestDebug_Logged(t *testing.T) {
	var buf bytes.Buffer
	log := New(DEBUG, &buf) // DEBUG level
	log.Debug("debug message")
	assert.Contains(t, buf.String(), "debug message")
}

func TestLevelString(t *testing.T) {
	assert.Equal(t, "debug", DEBUG.String())
	assert.Equal(t, "info", INFO.String())
	assert.Equal(t, "warn", WARN.String())
	assert.Equal(t, "error", ERROR.String())
	assert.Equal(t, "unknown", Level(99).String())
}

func TestNew_NilOutput(t *testing.T) {
	log := New(INFO, nil)
	assert.NotNil(t, log)
	// Should not panic
	log.Info("test")
}

func TestLogLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	log := New(WARN, &buf) // Only WARN and above
	log.Info("should not appear")
	log.Warn("should appear")
	log.Error("should also appear")
	output := buf.String()
	assert.False(t, strings.Contains(output, "should not appear"))
	assert.True(t, strings.Contains(output, "should appear"))
	assert.True(t, strings.Contains(output, "should also appear"))
}
