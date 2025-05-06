package env

import (
	"os"
)

// Lookup returns the value of the environment variable with the given key.
//
//	If the variable is not set, it returns an empty string and false.
//	If the variable is set, it returns the value and true.
func Lookup(key string) (string, bool) {
	return os.LookupEnv(key)
}

// Get returns the value of the environment variable with the given key.
// If the variable is not set, it returns the fallback value if provided, or an empty string.
func Get(key string, fallback ...string) string {
	var fb string
	if len(fallback) > 0 {
		fb = fallback[0]
	}
	s, ok := Lookup(key)
	if !ok {
		return fb
	}
	return s
}

// GetString returns the value of the environment variable with the given key.
// If the variable is not set, it returns the fallback value if provided, or an empty string.
func GetString(key string, fallback ...string) string {
	return Get(key, fallback...)
}
