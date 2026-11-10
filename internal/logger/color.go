package logger

import "strconv"

const ansiReset = "\x1b[0m"

// namedANSIColors is a minimal name -> SGR code table covering what this
// package needs today (level labels, and optionally specific field values
// via WithFieldColors). It is intentionally small: if a richer palette is
// ever needed (e.g. the 256-color resolver from maruel/ansi256),
// resolveColor is the only place that needs to change.
var namedANSIColors = map[string]string{
	"black":   "30",
	"red":     "31",
	"green":   "32",
	"yellow":  "33",
	"blue":    "34",
	"magenta": "35",
	"cyan":    "36",
	"white":   "37",
	"gray":    "90",
	"grey":    "90",

	"brightBlack":   "90",
	"brightRed":     "91",
	"brightGreen":   "92",
	"brightYellow":  "93",
	"brightBlue":    "94",
	"brightMagenta": "95",
	"brightCyan":    "96",
	"brightWhite":   "97",
}

// resolveColor turns a color name, or a raw "0"-"255" string (256-color
// palette index), into a full SGR escape sequence. An unknown name or an
// out-of-range number resolves to "" (no color), so a bad config value
// degrades to plain text instead of garbage output.
func resolveColor(name string) string {
	if name == "" {
		return ""
	}
	if code, ok := namedANSIColors[name]; ok {
		return "\x1b[" + code + "m"
	}
	if n, err := strconv.Atoi(name); err == nil && n >= 0 && n <= 255 {
		return "\x1b[38;5;" + name + "m"
	}
	return ""
}

// colorize wraps s in color, resetting afterward. A blank color leaves s
// untouched.
func colorize(color, s string) string {
	if color == "" {
		return s
	}
	return color + s + ansiReset
}
