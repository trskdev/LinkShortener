package util

import "log/slog"

func getAttrByName(r slog.Record, key string) (slog.Value, bool) {
	var result slog.Value
	var found bool

	r.Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			result = a.Value
			found = true
			return false
		}
		return true
	})

	return result, found
}

func GetErrFromLog(r slog.Record) (error, bool) {
	val, found := getAttrByName(r, "error")
	if !found {
		return nil, false
	}

	err, ok := val.Any().(error)
	if !ok {
		return nil, false
	}

	return err, true
}
