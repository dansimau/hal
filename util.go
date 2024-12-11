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
