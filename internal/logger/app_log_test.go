package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// var compliance check: *AppLog must satisfy LoggingInterface.
var _ LoggingInterface = &AppLog{}

func TestNewAppLogger_AutoModeDefaultsToJSONForNonTTYWriter(t *testing.T) {
	// A bytes.Buffer is never a terminal, so OutputModeAuto (the default)
	// must resolve to JSON here -- this is what makes JSON the default
	// in AWS/containers with zero environment configuration.
	var buf bytes.Buffer
	l := NewAppLogger(WithOutput(&buf))

	l.Info("hello")

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))
	assert.Equal(t, "info", decoded["level"])
	assert.Equal(t, "hello", decoded["msg"])
	assert.Contains(t, decoded, "time")
}

func TestJSONMode_LevelLabelsAndFields(t *testing.T) {
	var buf bytes.Buffer
	l := NewAppLogger(
		WithOutput(&buf),
		WithOutputMode(OutputModeJSON),
		WithSlogLevel(LevelTrace),
	)

	l.WithField("requestId", "abc-123").Tracef("starting %s", "work")

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))
	assert.Equal(t, "trace", decoded["level"])
	assert.Equal(t, "starting work", decoded["msg"])
	assert.Equal(t, "abc-123", decoded["requestId"])
}

func TestJSONMode_FatalLabel(t *testing.T) {
	defer func(orig func(int)) { osExit = orig }(osExit)
	var exitCode int
	exited := false
	osExit = func(code int) {
		exited = true
		exitCode = code
	}

	var buf bytes.Buffer
	l := NewAppLogger(WithOutput(&buf), WithOutputMode(OutputModeJSON))

	l.Fatal("boom")

	require.True(t, exited)
	assert.Equal(t, 1, exitCode)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))
	assert.Equal(t, "fatal", decoded["level"])
	assert.Equal(t, "boom", decoded["msg"])
}

func TestConsoleMode_BracketedColoredLevelAndFields(t *testing.T) {
	var buf bytes.Buffer
	l := NewAppLogger(WithOutput(&buf), WithOutputMode(OutputModeConsole))

	l.WithFields(map[string]any{"status": 200}).Info("some long message here")

	line := buf.String()
	cyan := resolveColor("cyan")
	assert.Contains(t, line, cyan+"[INFO]"+ansiReset)
	assert.Contains(t, line, "some long message here")
	assert.Contains(t, line, "status=200")
}

func TestConsoleMode_QuotesValuesNeedingIt(t *testing.T) {
	var buf bytes.Buffer
	l := NewAppLogger(WithOutput(&buf), WithOutputMode(OutputModeConsole))

	l.WithField("referer", "has space").Info("msg")

	assert.Contains(t, buf.String(), `referer="has space"`)
}

func TestConsoleMode_FieldColors(t *testing.T) {
	var buf bytes.Buffer
	l := NewAppLogger(
		WithOutput(&buf),
		WithOutputMode(OutputModeConsole),
		WithFieldColors(map[string]string{"statusCode": "yellow"}),
	)

	l.WithField("statusCode", 500).Info("req done")

	yellow := resolveColor("yellow")
	assert.Contains(t, buf.String(), "statusCode="+yellow+"500"+ansiReset)
}

func TestConsoleMode_Timestamp(t *testing.T) {
	var buf bytes.Buffer
	withTS := NewAppLogger(WithOutput(&buf), WithOutputMode(OutputModeConsole), WithTimestamp(true))
	withTS.Info("hi")
	assert.Regexp(t, `^\d{4}-\d{2}-\d{2}T`, buf.String())

	buf.Reset()
	noTS := NewAppLogger(WithOutput(&buf), WithOutputMode(OutputModeConsole))
	noTS.Info("hi")
	assert.True(t, strings.HasPrefix(buf.String(), "\x1b["))
}

func TestLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	l := NewAppLogger(WithOutput(&buf), WithOutputMode(OutputModeJSON), WithLogLevel("warn"))

	l.Info("should be filtered")
	assert.Empty(t, buf.String())

	l.Warn("should appear")
	assert.NotEmpty(t, buf.String())
}

func TestParseLogLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"trace":   LevelTrace,
		"DEBUG":   slog.LevelDebug,
		"info":    slog.LevelInfo,
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
		"fatal":   LevelFatal,
		"bogus":   slog.LevelInfo,
	}
	for input, want := range cases {
		assert.Equal(t, want, parseLogLevel(input), "input=%s", input)
	}
}

func TestDetectOutputMode(t *testing.T) {
	var buf bytes.Buffer
	assert.Equal(t, OutputModeJSON, detectOutputMode(&buf))
}
