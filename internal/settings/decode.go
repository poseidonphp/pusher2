package settings

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
)

const tagName = "config"

// decode copies src (values as produced by flags, env vars, or encoding/json)
// into the value pointed to by dst, converting scalar types where needed.
func decode(src any, dst any) error {
	rv := reflect.ValueOf(dst)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("settings: decode target must be a non-nil pointer, got %T", dst)
	}
	return decodeValue(src, rv.Elem(), "")
}

func decodeValue(src any, rv reflect.Value, path string) error {
	if src == nil {
		return nil
	}

	switch rv.Kind() {
	case reflect.Pointer:
		if rv.IsNil() {
			rv.Set(reflect.New(rv.Type().Elem()))
		}
		return decodeValue(src, rv.Elem(), path)

	case reflect.Interface:
		sv := reflect.ValueOf(src)
		if !sv.Type().AssignableTo(rv.Type()) {
			return typeError(src, rv, path)
		}
		rv.Set(sv)
		return nil

	case reflect.String:
		s, ok := toString(src)
		if !ok {
			return typeError(src, rv, path)
		}
		rv.SetString(s)
		return nil

	case reflect.Bool:
		b, err := toBool(src)
		if err != nil {
			return fieldError(path, err)
		}
		rv.SetBool(b)
		return nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := toInt64(src)
		if err != nil {
			return fieldError(path, err)
		}
		if rv.OverflowInt(n) {
			return fieldError(path, fmt.Errorf("value %d overflows %s", n, rv.Type()))
		}
		rv.SetInt(n)
		return nil

	case reflect.Slice:
		items, ok := src.([]any)
		if !ok {
			return typeError(src, rv, path)
		}
		out := reflect.MakeSlice(rv.Type(), len(items), len(items))
		for i, item := range items {
			if err := decodeValue(item, out.Index(i), fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
		rv.Set(out)
		return nil

	case reflect.Struct:
		m, ok := src.(map[string]any)
		if !ok {
			return typeError(src, rv, path)
		}
		return decodeStruct(m, rv, path)
	}

	return fieldError(path, fmt.Errorf("unsupported target type %s", rv.Type()))
}

func decodeStruct(m map[string]any, rv reflect.Value, path string) error {
	t := rv.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		name := field.Name
		if tag, ok := field.Tag.Lookup(tagName); ok {
			if tag == "-" {
				continue
			}
			name = tag
		}
		v, found := lookupFold(m, name)
		if !found {
			continue
		}
		fieldPath := name
		if path != "" {
			fieldPath = path + "." + name
		}
		if err := decodeValue(v, rv.Field(i), fieldPath); err != nil {
			return err
		}
	}
	return nil
}

func lookupFold(m map[string]any, key string) (any, bool) {
	if v, ok := m[key]; ok {
		return v, true
	}
	for k, v := range m {
		if strings.EqualFold(k, key) {
			return v, true
		}
	}
	return nil, false
}

func toString(src any) (string, bool) {
	switch v := src.(type) {
	case string:
		return v, true
	case json.Number:
		return v.String(), true
	case bool:
		return strconv.FormatBool(v), true
	case int:
		return strconv.Itoa(v), true
	case int64:
		return strconv.FormatInt(v, 10), true
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), true
	}
	return "", false
}

func toBool(src any) (bool, error) {
	switch v := src.(type) {
	case bool:
		return v, nil
	case string:
		return strconv.ParseBool(strings.TrimSpace(v))
	case json.Number, int, int64, float64:
		n, err := toInt64(v)
		return n != 0, err
	}
	return false, fmt.Errorf("cannot convert %T to bool", src)
}

func toInt64(src any) (int64, error) {
	switch v := src.(type) {
	case int:
		return int64(v), nil
	case int64:
		return v, nil
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	case json.Number:
		return parseInt(v.String())
	case string:
		return parseInt(strings.TrimSpace(v))
	case float64:
		if v != math.Trunc(v) || v > math.MaxInt64 || v < math.MinInt64 {
			return 0, fmt.Errorf("cannot convert %v to an integer", v)
		}
		return int64(v), nil
	}
	return 0, fmt.Errorf("cannot convert %T to an integer", src)
}

// parseInt accepts plain integers and whole-number floats such as "10.0".
func parseInt(s string) (int64, error) {
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("cannot convert %q to an integer", s)
	}
	return toInt64(f)
}

func typeError(src any, rv reflect.Value, path string) error {
	return fieldError(path, fmt.Errorf("cannot convert %T to %s", src, rv.Type()))
}

func fieldError(path string, err error) error {
	if path == "" {
		return fmt.Errorf("settings: %w", err)
	}
	return fmt.Errorf("settings: %s: %w", path, err)
}
