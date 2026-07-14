package common

import (
	"errors"
	"fmt"
	"reflect"
)

type PositionedError struct {
	Message string
}

func (e PositionedError) Error() string { return e.Message }

func IsPositionedError(err error) bool {
	var perr PositionedError
	return errors.As(err, &perr)
}

func ErrorAtPos(file string, line, col int, format string, args ...any) error {
	prefix := ""
	switch {
	case line > 0 && col > 0:
		prefix = fmt.Sprintf("%s: line %d, col %d", file, line, col)
	case line > 0:
		prefix = fmt.Sprintf("%s: line %d", file, line)
	default:
		prefix = file
	}
	if prefix != "" {
		format = prefix + ": " + format
	}
	return PositionedError{Message: fmt.Sprintf(format, args...)}
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

func NodeSourceFile(v any) string {
	if v == nil {
		return ""
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return ""
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return ""
	}
	fileField := rv.FieldByName("SourceFile")
	if fileField.IsValid() && fileField.Kind() == reflect.String {
		return fileField.String()
	}
	return ""
}

// ErrorAtNode wraps an error message with source location extracted from an AST node.
// If the node has a SourceFile field, it uses that; otherwise it falls back to
// using the node's type name as the file prefix.
func ErrorAtNode(file string, node any, format string, args ...any) error {
	line, col := NodePos(node)
	return ErrorAtPos(file, line, col, format, args...)
}
