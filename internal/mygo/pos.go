package mygo

import (
	"fmt"
	"reflect"
)

func errorAtLine(line int, format string, args ...any) error {
	if line > 0 {
		format = fmt.Sprintf("line %d: %s", line, format)
	}
	return fmt.Errorf(format, args...)
}

func nodeLine(v any) int {
	if v == nil {
		return 0
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return 0
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return 0
	}
	f := rv.FieldByName("Line")
	if !f.IsValid() || f.Kind() != reflect.Int {
		return 0
	}
	return int(f.Int())
}

