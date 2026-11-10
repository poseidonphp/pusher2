package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// OutputMode controls whether log lines are written as JSON or as
// human-readable, colorized text meant for a terminal.
type OutputMode uint8

const (
	// OutputModeAuto picks OutputModeConsole when the configured output is
	// an interactive terminal, and OutputModeJSON otherwise (pipes, files,
	// redirected output, and non-TTY environments such as AWS ECS/Lambda).
	// This is the default, so no environment variable is needed to get JSON
	// in those environments.
	OutputModeAuto OutputMode = iota
	OutputModeConsole
	OutputModeJSON
)

// AppLog is a LoggingInterface implementation backed by log/slog.
type AppLog struct {
	logger *slog.Logger
	level  *slog.LevelVar
}

// osExit is a seam for Fatal/Fatalf so tests can observe the exit call
// without killing the test process.
var osExit = os.Exit

type appLoggerConfig struct {
	level         slog.Level
	outputMode    OutputMode
	out           io.Writer
	showTimestamp bool
	fieldColors   map[string]string
}

// Option configures a logger built with NewAppLogger.
type Option func(*appLoggerConfig)

// WithLogLevel sets the minimum level using a string such as "trace",
// "debug", "info", "warn", "error", or "fatal" (case-insensitive).
// Unrecognized values fall back to info.
func WithLogLevel(levelStr string) Option {
	return func(cfg *appLoggerConfig) {
		cfg.level = parseLogLevel(levelStr)
	}
}

// WithSlogLevel sets the minimum level directly, including the package's
// LevelTrace / LevelFatal.
func WithSlogLevel(level slog.Level) Option {
	return func(cfg *appLoggerConfig) {
		cfg.level = level
	}
}

// WithOutputMode forces JSON or console output instead of auto-detecting
// from the output writer.
func WithOutputMode(mode OutputMode) Option {
	return func(cfg *appLoggerConfig) {
		cfg.outputMode = mode
	}
}

// WithOutput sets the destination for log lines. Defaults to os.Stdout.
func WithOutput(out io.Writer) Option {
	return func(cfg *appLoggerConfig) {
		cfg.out = out
	}
}

// WithTimestamp controls whether console mode prefixes each line with a
// timestamp. JSON mode always includes one. Defaults to false.
func WithTimestamp(show bool) Option {
	return func(cfg *appLoggerConfig) {
		cfg.showTimestamp = show
	}
}

// WithFieldColors sets colors for specific attribute names in console
// mode, e.g. WithFieldColors(map[string]string{"requestId": "yellow"})
// colors just that field's value. Accepted color values are the names in
// color.go, or a "0"-"255" 256-color index. Unknown names/indexes render
// that field uncolored. Has no effect in JSON mode.
func WithFieldColors(colors map[string]string) Option {
	return func(cfg *appLoggerConfig) {
		cfg.fieldColors = colors
	}
}

// NewAppLogger builds a LoggingInterface backed by log/slog.
func NewAppLogger(options ...Option) *AppLog {
	cfg := &appLoggerConfig{
		level:      slog.LevelInfo,
		outputMode: OutputModeAuto,
		out:        os.Stdout,
	}
	for _, opt := range options {
		opt(cfg)
	}

	mode := cfg.outputMode
	if mode == OutputModeAuto {
		mode = detectOutputMode(cfg.out)
	}

	levelVar := &slog.LevelVar{}
	levelVar.Set(cfg.level)

	var handler slog.Handler
	if mode == OutputModeJSON {
		handler = slog.NewJSONHandler(cfg.out, &slog.HandlerOptions{
			Level:       levelVar,
			ReplaceAttr: jsonReplaceAttr,
		})
	} else {
		resolvedFieldColors := make(map[string]string, len(cfg.fieldColors))
		for field, color := range cfg.fieldColors {
			resolvedFieldColors[field] = resolveColor(color)
		}
		handler = newConsoleHandler(cfg.out, levelVar, cfg.showTimestamp, resolvedFieldColors)
	}

	return &AppLog{
		logger: slog.New(handler),
		level:  levelVar,
	}
}

// jsonReplaceAttr lowercases the level string (matching the previous
// logrus JSON output, e.g. "level":"error") and gives the custom
// Trace/Fatal levels their proper names instead of slog's default
// "DEBUG-4"/"ERROR+4".
func jsonReplaceAttr(_ []string, a slog.Attr) slog.Attr {
	if a.Key == slog.LevelKey {
		if level, ok := a.Value.Any().(slog.Level); ok {
			return slog.String(slog.LevelKey, strings.ToLower(levelLabel(level)))
		}
	}
	return a
}

// detectOutputMode returns OutputModeConsole only when w is the process's
// own *os.File attached to an interactive terminal; everything else
// (buffers, pipes, redirected files, non-TTY environments) gets JSON.
func detectOutputMode(w io.Writer) OutputMode {
	if f, ok := w.(*os.File); ok && isTerminal(f) {
		return OutputModeConsole
	}
	return OutputModeJSON
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func (a *AppLog) log(level slog.Level, args ...any) {
	a.logger.Log(context.Background(), level, fmt.Sprint(args...))
}

func (a *AppLog) logf(level slog.Level, format string, args ...any) {
	a.logger.Log(context.Background(), level, fmt.Sprintf(format, args...))
}

func (a *AppLog) Trace(args ...any) { a.log(LevelTrace, args...) }

func (a *AppLog) Tracef(format string, args ...any) { a.logf(LevelTrace, format, args...) }

func (a *AppLog) Debug(args ...any) { a.log(slog.LevelDebug, args...) }

func (a *AppLog) Debugf(format string, args ...any) { a.logf(slog.LevelDebug, format, args...) }

func (a *AppLog) Info(args ...any) { a.log(slog.LevelInfo, args...) }

func (a *AppLog) Infof(format string, args ...any) { a.logf(slog.LevelInfo, format, args...) }

func (a *AppLog) Warn(args ...any) { a.log(slog.LevelWarn, args...) }

func (a *AppLog) Warnf(format string, args ...any) { a.logf(slog.LevelWarn, format, args...) }

func (a *AppLog) Error(args ...any) { a.log(slog.LevelError, args...) }

func (a *AppLog) Errorf(format string, args ...any) { a.logf(slog.LevelError, format, args...) }

// Fatal logs at LevelFatal and then terminates the process, matching the
// previous logrus Fatal behavior.
func (a *AppLog) Fatal(args ...any) {
	a.log(LevelFatal, args...)
	osExit(1)
}

// Fatalf logs at LevelFatal and then terminates the process, matching the
// previous logrus Fatalf behavior.
func (a *AppLog) Fatalf(format string, args ...any) {
	a.logf(LevelFatal, format, args...)
	osExit(1)
}

// SetLevel changes the minimum level after construction. Since loggers
// derived via WithField/WithFields share this logger's *slog.LevelVar, the
// change applies to all of them too. Use this when some messages must be
// logged before the desired level is known: build with NewAppLogger (level
// defaults to Info), then call SetLevel once flags/env vars are parsed.
func (a *AppLog) SetLevel(level slog.Level) {
	a.level.Set(level)
}

// SetLogLevel is SetLevel taking a string level name (see WithLogLevel).
// Unrecognized values fall back to info.
func (a *AppLog) SetLogLevel(levelStr string) {
	a.level.Set(parseLogLevel(levelStr))
}

func (a *AppLog) WithField(key string, value any) LoggingInterface {
	return &AppLog{logger: a.logger.With(key, value), level: a.level}
}

func (a *AppLog) WithFields(fields map[string]any) LoggingInterface {
	args := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}
	return &AppLog{logger: a.logger.With(args...), level: a.level}
}
