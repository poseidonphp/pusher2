package logger

import (
	"context"
	"io"
	"log/slog"
	"strconv"
	"sync"
	"time"
)

// consoleHandler is a slog.Handler that renders records as colorized,
// human-readable lines:
//
//	[INFO] some long message here key=value key2="value with spaces"
//
// It exists because slog's stdlib handlers only produce plain text or JSON
// in fixed shapes; coloring the level label (and optionally specific field
// values) needs a small custom Handle implementation.
type consoleHandler struct {
	mu          *sync.Mutex
	out         io.Writer
	level       slog.Leveler
	timestamps  bool
	fieldColors map[string]string // field name -> resolved SGR escape code
	groupPrefix string
	baseFields  []string // pre-rendered "key=value" strings from WithAttrs
}

func newConsoleHandler(out io.Writer, level slog.Leveler, timestamps bool, fieldColors map[string]string) *consoleHandler {
	return &consoleHandler{
		mu:          &sync.Mutex{},
		out:         out,
		level:       level,
		timestamps:  timestamps,
		fieldColors: fieldColors,
	}
}

func (h *consoleHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *consoleHandler) Handle(_ context.Context, r slog.Record) error {
	var b []byte
	if h.timestamps {
		b = append(b, r.Time.Format(time.RFC3339)...)
		b = append(b, ' ')
	}

	label := levelLabel(r.Level)
	b = append(b, colorize(defaultLevelColors[r.Level], "["+label+"]")...)
	b = append(b, ' ')
	b = append(b, r.Message...)

	for _, f := range h.baseFields {
		b = append(b, ' ')
		b = append(b, f...)
	}
	r.Attrs(func(a slog.Attr) bool {
		for _, f := range renderAttr(h.groupPrefix, a, h.fieldColors) {
			b = append(b, ' ')
			b = append(b, f...)
		}
		return true
	})
	b = append(b, '\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.out.Write(b)
	return err
}

func (h *consoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	nh := h.clone()
	for _, a := range attrs {
		nh.baseFields = append(nh.baseFields, renderAttr(h.groupPrefix, a, h.fieldColors)...)
	}
	return nh
}

func (h *consoleHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	nh := h.clone()
	if nh.groupPrefix == "" {
		nh.groupPrefix = name
	} else {
		nh.groupPrefix = nh.groupPrefix + "." + name
	}
	return nh
}

func (h *consoleHandler) clone() *consoleHandler {
	nh := *h
	nh.baseFields = append([]string(nil), h.baseFields...)
	return &nh
}

// renderAttr formats a into one or more "key=value" strings, flattening
// groups (including nested ones) with dot-joined keys. A fieldColors match
// is made on the attr's own key, not its dotted group path.
func renderAttr(prefix string, a slog.Attr, fieldColors map[string]string) []string {
	a.Value = a.Value.Resolve()
	if a.Value.Kind() == slog.KindGroup {
		groupAttrs := a.Value.Group()
		if len(groupAttrs) == 0 {
			return nil
		}
		newPrefix := a.Key
		if prefix != "" {
			newPrefix = prefix + "." + a.Key
		}
		out := make([]string, 0, len(groupAttrs))
		for _, ga := range groupAttrs {
			out = append(out, renderAttr(newPrefix, ga, fieldColors)...)
		}
		return out
	}

	key := a.Key
	if prefix != "" {
		key = prefix + "." + key
	}
	val := formatAttrValue(a.Value)
	if color, ok := fieldColors[a.Key]; ok {
		val = colorize(color, val)
	}
	return []string{key + "=" + val}
}

func formatAttrValue(v slog.Value) string {
	s := v.String()
	if v.Kind() == slog.KindString && needsQuote(s) {
		return strconv.Quote(s)
	}
	return s
}

// needsQuote reports whether s needs quoting to stay a single token in the
// console line (empty, contains a space, '=', '"', or a control character).
func needsQuote(s string) bool {
	if s == "" {
		return true
	}
	for _, r := range s {
		if r == ' ' || r == '=' || r == '"' || r < 0x20 {
			return true
		}
	}
	return false
}
