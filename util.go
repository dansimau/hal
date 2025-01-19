package hal

import (
	"reflect"
	"runtime"
	"strings"
)

func getShortFunctionName(i interface{}) string {
	fullName := runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
	// Split by dots and get the last element
	parts := strings.Split(fullName, ".")

	return parts[len(parts)-1]
}

func getStringOrStringSlice(value any) []string {
	if value == nil {
		return []string{}
	}

	ss, ok := value.([]string)
	if ok {
		return ss
	}

	s, ok := value.(string)
	if ok {
		return []string{s}
	}

	slice, ok := value.([]any)
	if ok {
		strings := []string{}

		for _, v := range slice {
			s, ok := v.(string)
			if ok {
				strings = append(strings, s)
			}
		}

		return strings
	}

	return []string{}
}
