package formatter

import (
	"strings"
	"testing"
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
	ok, isOk := formattedText(formatted)
	if !isOk {
		t.Fatalf("FormatSource failed: %v", formatted)
	}
	if !strings.Contains(ok, "case 0 if flag") || !strings.Contains(ok, "case 1 if flag") {
		t.Fatalf("guard header missing from formatted output:\n%s", ok)
	}
	again := FormatSource("guard.mygo", ok)
	okAgain, isOkAgain := formattedText(again)
	if !isOkAgain {
		t.Fatalf("second FormatSource failed: %v", again)
	}
	if okAgain != ok {
		t.Fatalf("formatter is not idempotent:\nfirst:\n%s\nsecond:\n%s", ok, okAgain)
	}
}
