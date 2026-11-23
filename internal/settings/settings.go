// Package settings replaces the subset of spf13/pflag and spf13/viper used by
// this project, built on the standard library only.
//
// A Settings value owns a set of command-line flags and resolves every key
// from four sources, highest precedence first:
//
//  1. a flag explicitly passed on the command line
//  2. an environment variable (key upper-cased, "-" replaced by "_")
//  3. a JSON config file loaded with ReadConfigFile
//  4. the flag's default value
//
// Keys are normalised to lower case with "-" replaced by "_", so the flag
// "--app-env", the env var APP_ENV, and the config file key "app_env" all
// refer to the same setting.
package settings

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

type flagDef struct {
	name      string
	shorthand string
	usage     string
	defValue  string
	value     func() any
}

// Settings holds flag definitions, parsed command-line state, and config file data.
type Settings struct {
	fs        *flag.FlagSet
	flags     map[string]*flagDef // keyed by normalised key
	order     []string            // normalised keys in definition order, for usage output
	canonical map[string]string   // flag name or shorthand -> normalised key
	changed   map[string]bool     // normalised keys set on the command line
	file      map[string]any      // normalised top-level keys from the config file
}

// New creates an empty Settings. name is used in usage and error messages.
func New(name string) *Settings {
	s := &Settings{
		fs:        flag.NewFlagSet(name, flag.ContinueOnError),
		flags:     map[string]*flagDef{},
		canonical: map[string]string{},
		changed:   map[string]bool{},
		file:      map[string]any{},
	}
	s.fs.Usage = s.printUsage
	return s
}

// SetOutput sets where usage and parse error messages are written. Default is os.Stderr.
func (s *Settings) SetOutput(w io.Writer) { s.fs.SetOutput(w) }

// String defines a string flag. shorthand may be empty.
func (s *Settings) String(name, shorthand, value, usage string) {
	p := new(string)
	s.fs.StringVar(p, name, value, usage)
	if shorthand != "" {
		s.fs.StringVar(p, shorthand, value, usage)
	}
	s.register(name, shorthand, usage, fmt.Sprintf("%q", value), func() any { return *p })
}

// Int defines an int flag. shorthand may be empty.
func (s *Settings) Int(name, shorthand string, value int, usage string) {
	p := new(int)
	s.fs.IntVar(p, name, value, usage)
	if shorthand != "" {
		s.fs.IntVar(p, shorthand, value, usage)
	}
	s.register(name, shorthand, usage, fmt.Sprint(value), func() any { return *p })
}

// Int64 defines an int64 flag. shorthand may be empty.
func (s *Settings) Int64(name, shorthand string, value int64, usage string) {
	p := new(int64)
	s.fs.Int64Var(p, name, value, usage)
	if shorthand != "" {
		s.fs.Int64Var(p, shorthand, value, usage)
	}
	s.register(name, shorthand, usage, fmt.Sprint(value), func() any { return *p })
}

// Bool defines a bool flag. shorthand may be empty. On the command line it
// accepts "--name" or "--name=true|false"; "--name false" is not supported.
func (s *Settings) Bool(name, shorthand string, value bool, usage string) {
	p := new(bool)
	s.fs.BoolVar(p, name, value, usage)
	if shorthand != "" {
		s.fs.BoolVar(p, shorthand, value, usage)
	}
	s.register(name, shorthand, usage, fmt.Sprint(value), func() any { return *p })
}

func (s *Settings) register(name, shorthand, usage, defValue string, value func() any) {
	key := normaliseKey(name)
	if _, dup := s.flags[key]; dup {
		panic(fmt.Sprintf("settings: flag %q redefined", name))
	}
	s.flags[key] = &flagDef{name: name, shorthand: shorthand, usage: usage, defValue: defValue, value: value}
	s.order = append(s.order, key)
	s.canonical[name] = key
	if shorthand != "" {
		s.canonical[shorthand] = key
	}
}

// Parse parses command-line arguments (normally os.Args[1:]). Both "-name"
// and "--name" forms are accepted, as are "-name value" and "-name=value".
// It returns flag.ErrHelp when -h or --help is passed.
func (s *Settings) Parse(args []string) error {
	if err := s.fs.Parse(args); err != nil {
		return err
	}
	s.fs.Visit(func(f *flag.Flag) { s.changed[s.canonical[f.Name]] = true })
	return nil
}

// Changed reports whether the flag was explicitly passed on the command line.
func (s *Settings) Changed(name string) bool { return s.changed[normaliseKey(name)] }

// ReadConfigFile loads a JSON object from path. Its top-level keys become
// settings; nested values are kept as-is for Unmarshal.
func (s *Settings) ReadConfigFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	dec.UseNumber()
	var raw map[string]any
	if err := dec.Decode(&raw); err != nil {
		return fmt.Errorf("parse config file %s: %w", path, err)
	}
	for k, v := range raw {
		s.file[normaliseKey(k)] = v
	}
	return nil
}

// Get returns the resolved value for key, or nil and false when no source has it.
func (s *Settings) Get(key string) (any, bool) {
	key = normaliseKey(key)
	def, isFlag := s.flags[key]
	if isFlag && s.changed[key] {
		return def.value(), true
	}
	if v, ok := os.LookupEnv(strings.ToUpper(key)); ok && v != "" {
		return v, true
	}
	if v, ok := s.file[key]; ok {
		return v, true
	}
	if isFlag {
		return def.value(), true
	}
	return nil, false
}

// GetString returns the resolved value as a string, or "" when unset or not convertible.
func (s *Settings) GetString(key string) string { return getAs[string](s, key) }

// GetInt returns the resolved value as an int, or 0 when unset or not convertible.
func (s *Settings) GetInt(key string) int { return getAs[int](s, key) }

// GetInt64 returns the resolved value as an int64, or 0 when unset or not convertible.
func (s *Settings) GetInt64(key string) int64 { return getAs[int64](s, key) }

// GetBool returns the resolved value as a bool, or false when unset or not convertible.
func (s *Settings) GetBool(key string) bool { return getAs[bool](s, key) }

func getAs[T any](s *Settings, key string) T {
	var out T
	v, ok := s.Get(key)
	if !ok {
		return out
	}
	if err := decode(v, &out); err != nil {
		var zero T
		return zero
	}
	return out
}

// Unmarshal decodes every known setting (all flags plus all config file keys)
// into dst, which must be a pointer to a struct. Fields are matched by their
// `config:"name"` tag, or by field name when untagged; `config:"-"` skips a
// field. Matching is case-insensitive. Values are converted between strings,
// numbers, and bools as needed; a value that cannot be converted is an error.
func (s *Settings) Unmarshal(dst any) error {
	all := make(map[string]any, len(s.flags)+len(s.file))
	for k := range s.flags {
		all[k], _ = s.Get(k)
	}
	for k := range s.file {
		all[k], _ = s.Get(k)
	}
	return decode(all, dst)
}

func (s *Settings) printUsage() {
	w := s.fs.Output()
	fmt.Fprintf(w, "Usage of %s:\n", s.fs.Name())
	keys := append([]string(nil), s.order...)
	sort.Strings(keys)
	for _, k := range keys {
		d := s.flags[k]
		names := "--" + d.name
		if d.shorthand != "" {
			names = "-" + d.shorthand + ", " + names
		}
		fmt.Fprintf(w, "  %s\n    \t%s (default %s)\n", names, d.usage, d.defValue)
	}
}

func normaliseKey(k string) string {
	return strings.ToLower(strings.ReplaceAll(k, "-", "_"))
}
