package logger

import (
	"log/slog"
	"strings"
)

// Custom levels extending slog's four built-in ones (Debug=-4, Info=0,
// Warn=4, Error=8) so this package can keep the Trace/Fatal levels the
// previous logrus-based logger supported.
const (
	LevelTrace slog.Level = -8
	LevelFatal slog.Level = 12
)

// levelLabel returns the display name for a level. slog.Level.String()
// doesn't know about LevelTrace/LevelFatal, so those two are handled
// explicitly; any other (unexpected) level falls back to slog's own
// formatting (e.g. "INFO+2").
func levelLabel(level slog.Level) string {
	switch level {
	case LevelTrace:
		return "TRACE"
	case slog.LevelDebug:
		return "DEBUG"
	case slog.LevelInfo:
		return "INFO"
	case slog.LevelWarn:
		return "WARN"
	case slog.LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return level.String()
	}
}

// defaultLevelColors is the out-of-the-box color for each level's
// bracketed label in console mode, e.g. "[INFO]" rendered in cyan. Levels
// not listed here (shouldn't happen in practice) render uncolored.
var defaultLevelColors = map[slog.Level]string{
	LevelTrace:      resolveColor("gray"),
	slog.LevelDebug: resolveColor("magenta"),
	slog.LevelInfo:  resolveColor("cyan"),
	slog.LevelWarn:  resolveColor("yellow"),
	slog.LevelError: resolveColor("red"),
	LevelFatal:      resolveColor("brightRed"),
}

// parseLogLevel converts a string such as "trace", "debug", "info", "warn"
// (or "warning"), "error", or "fatal" (case-insensitive) into a slog.Level.
// Unrecognized values default to info.
func parseLogLevel(levelStr string) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(levelStr)) {
	case "TRACE":
		return LevelTrace
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	case "FATAL":
		return LevelFatal
	default:
		return slog.LevelInfo
	}
}
