package formatter

import (
	"strings"
	"testing"

	"github.com/mygo-lang/mygo/prelude"
)

func TestFormatterPreservesCaseGuards(t *testing.T) {
	source := `package sample

func f(v: Int, flag: Bool) -> Int
  switch v
    case 0 if flag then
      1
    end
    case 1 if flag => 2
    case _ => 0
  end
end
`
	formatted := FormatSource("guard.mygo", source)
	ok, isOk := formatted.(prelude.Result__Ok[string, string])
	if !isOk {
		t.Fatalf("FormatSource failed: %v", formatted)
	}
	if !strings.Contains(ok.F0, "case 0 if flag") || !strings.Contains(ok.F0, "case 1 if flag") {
		t.Fatalf("guard header missing from formatted output:\n%s", ok.F0)
	}
	again := FormatSource("guard.mygo", ok.F0)
	okAgain, isOkAgain := again.(prelude.Result__Ok[string, string])
	if !isOkAgain {
		t.Fatalf("second FormatSource failed: %v", again)
	}
	if okAgain.F0 != ok.F0 {
		t.Fatalf("formatter is not idempotent:\nfirst:\n%s\nsecond:\n%s", ok.F0, okAgain.F0)
	}
}
