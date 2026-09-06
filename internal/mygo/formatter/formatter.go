// Package formatter provides deterministic formatting for MyGO source files.
package formatter

import (
	"fmt"

	"github.com/mygo-lang/mygo/prelude"
)

// Format validates src and returns its canonical whole-file layout.
func Format(filename, src string) (string, error) {
	parsed := FormatSource(filename, src)
	if result, ok := parsed.(prelude.Result__Ok[string, string]); ok {
		return result.F0, nil
	}
	if err, ok := parsed.(prelude.Result__Err[string, string]); ok {
		return "", fmt.Errorf("%s", err.F0)
	}
	return "", fmt.Errorf("%s: formatter returned an unknown result", filename)
}
