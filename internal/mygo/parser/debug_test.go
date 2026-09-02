package parser

import "testing"

func TestParserDebugOutputIsOptInPerParse(t *testing.T) {
	t.Setenv("MYGO_PARSER_DEBUG", "")
	yyDebug = 4
	yyErrorVerbose = true

	if _, err := ParseFile("debug-reset.mygo", "package p\n"); err != nil {
		t.Fatal(err)
	}
	if yyDebug != 0 || yyErrorVerbose {
		t.Fatalf("parser diagnostics remained enabled: yyDebug=%d, yyErrorVerbose=%v", yyDebug, yyErrorVerbose)
	}
}
