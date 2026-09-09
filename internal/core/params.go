package core

import "strconv"

// AsString, AsInt, and AsBool coerce a loosely-typed Params value to a concrete type.
func AsString(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	case interface{ String() string }:
		return t.String(), true
	default:
		return "", false
	}
}

func AsInt(v any) (int, bool) {
	switch t := v.(type) {
	case int:
		return t, true
	case int64:
		return int(t), true
	case float64:
		return int(t), true
	case string:
		n, err := strconv.Atoi(t)
		return n, err == nil
	default:
		return 0, false
	}
}

func AsBool(v any) (bool, bool) {
	switch t := v.(type) {
	case bool:
		return t, true
	case string:
		b, err := strconv.ParseBool(t)
		return b, err == nil
	default:
		return false, false
	}
}
