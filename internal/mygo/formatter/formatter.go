// Package formatter provides deterministic formatting for MyGO source files.
package formatter

import (
	"fmt"

	"github.com/mygo-lang/mygo/internal/mygo/abi"
)

// Format validates src and returns its canonical whole-file layout.
//
// The result is inspected through internal/mygo/abi's ABI-neutral helpers: a
// MyGO Result is an interface pair in the legacy ABI and a tagged struct in the
// canonical ABI, so neither variant type can be named here.
func Format(filename, src string) (string, error) {
	parsed := FormatSource(filename, src)
	if abi.HasOk[string, string](parsed) {
		return abi.OkValue[string, string](parsed, ""), nil
	}
	return "", fmt.Errorf("%s", abi.ErrValue[string, string](parsed, "unknown formatter failure"))
}
