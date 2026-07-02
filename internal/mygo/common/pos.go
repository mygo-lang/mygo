package common

import (
	"fmt"
	"reflect"
)

func ErrorAtPos(line, col int, format string, args ...any) error {
	switch {
	case line > 0 && col > 0:
		format = fmt.Sprintf("line %d, col %d: %s", line, col, format)
	case line > 0:
		format = fmt.Sprintf("line %d: %s", line, format)
	}
	return fmt.Errorf(format, args...)
}

func NodePos(v any) (int, int) {
	if v == nil {
		return 0, 0
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return 0, 0
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return 0, 0
	}
	lineField := rv.FieldByName("Line")
	colField := rv.FieldByName("Column")
	line, col := 0, 0
	if lineField.IsValid() && lineField.Kind() == reflect.Int {
		line = int(lineField.Int())
	}
	if colField.IsValid() && colField.Kind() == reflect.Int {
		col = int(colField.Int())
	}
	return line, col
}
