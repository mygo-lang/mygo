package parser2

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	ps "github.com/mygo-lang/mygo/lib/text/parsec"
	. "github.com/mygo-lang/mygo/prelude"
)

func TestParseFileAtIncludesSourceLocation(t *testing.T) {
	got := ParseFileAt("broken.mygo", "package sample\n\nfunc")
	err, ok := got.(Result__Err[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFileAt() = %T, want parse error", got)
	}
	if !strings.Contains(err.F0, "broken.mygo:3:5: parse error: expected identifier") {
		t.Fatalf("ParseFileAt() error = %q, want source name, line, column, and expectation", err.F0)
	}
}

func TestParseFileLosslessRetainsTokensAndTrivia(t *testing.T) {
	source := "package sample\n# keep   this\nfunc f() -> String\n  \"hello   world\"\nend\n"
	got := ParseFileLossless("sample.mygo", source)
	result, ok := got.(Result__Ok[LosslessFile, string])
	if !ok {
		t.Fatalf("ParseFileLossless() failed: %v", got)
	}
	if len(result.F0.Tokens) == 0 || len(result.F0.Trivia) == 0 {
		t.Fatalf("lossless result omitted tokens or trivia: %#v", result.F0)
	}
	commentFound := false
	for _, item := range result.F0.Trivia {
		if item.Kind == "comment" && item.Raw == "# keep   this" {
			commentFound = true
		}
	}
	if !commentFound {
		t.Fatalf("comment spelling was not retained: %#v", result.F0.Trivia)
	}
	if result.F0.Tokens[len(result.F0.Tokens)-1].Raw != "end" {
		t.Fatalf("last token = %q, want end", result.F0.Tokens[len(result.F0.Tokens)-1].Raw)
	}
	if len(result.F0.NodeSpans) < len(result.F0.Tokens) {
		t.Fatalf("node span count = %d, want at least %d", len(result.F0.NodeSpans), len(result.F0.Tokens))
	}
	if result.F0.NodeSpans[0].Span.Start.Line != result.F0.Tokens[0].Span.Start.Line {
		t.Fatalf("node span start = %#v, token start = %#v", result.F0.NodeSpans[0].Span.Start, result.F0.Tokens[0].Span.Start)
	}
	if len(result.F0.LayoutEvents) == 0 || !strings.HasPrefix(result.F0.LayoutEvents[0].Kind, "enter:") {
		t.Fatalf("layout event stream = %#v", result.F0.LayoutEvents)
	}
}

func TestExpressionsCarrySourceSpans(t *testing.T) {
	got := ParseFileAt("span.mygo", "package sample\nfunc f() -> Int\n  1\nend\n")
	parsed, ok := got.(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFileAt() failed: %v", got)
	}
	fn := parsed.F0.Decls[0].(ast2.Decl__FuncDecl)
	body := fn.F4.Kind.(ast2.ExprKind__BlockExpr)
	value := body.F0[0].(ast2.Stmt__ExprStmt).F0
	if value.Span.Start.SourceName != "span.mygo" || value.Span.Start.Line == 0 {
		t.Fatalf("expression span = %#v", value.Span)
	}
}

func TestParametersCarrySourceSpans(t *testing.T) {
	got := ParseFileAt("param-span.mygo", "package sample\nfunc f(value: Option[Int]) -> Int\n  1\nend\n")
	parsed, ok := got.(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFileAt() failed: %v", got)
	}
	fn := parsed.F0.Decls[0].(ast2.Decl__FuncDecl)
	if len(fn.F2) != 1 || fn.F2[0].Span.Start.Line == 0 || fn.F2[0].Span.End.Column <= fn.F2[0].Span.Start.Column {
		t.Fatalf("parameter span = %#v", fn.F2[0].Span)
	}
}

func TestSwitchCaseParserSpan(t *testing.T) {
	reply := ps.ParseInput(switchCase(), "case 1 => 10\nend")
	if !reply.Ok {
		t.Fatalf("switch case parser failed: state=%#v error=%#v", reply.State, reply.Error)
	}
	if reply.Value.Span.Start.Line == 0 || reply.Value.Span.End.Line < reply.Value.Span.Start.Line {
		t.Fatalf("switch case parser span = %#v", reply.Value.Span)
	}
}

func TestLosslessNodeSpansCarryDeclarationAndStatementPaths(t *testing.T) {
	got := ParseFileLossless("paths.mygo", "package sample\nfunc f() -> Int\n  1\nend\n")
	parsed, ok := got.(Result__Ok[LosslessFile, string])
	if !ok {
		t.Fatalf("ParseFileLossless() failed: %v", got)
	}
	found := false
	statementFound := false
	for _, item := range parsed.F0.NodeSpans {
		if (item.Kind == "expr" || item.Kind == "literal") && len(item.Path) == 2 && item.Path[0] == 0 && item.Path[1] == 0 {
			found = true
		}
		if item.Kind == "stmt:expr" && len(item.Path) == 2 && item.Path[0] == 0 && item.Path[1] == 0 {
			statementFound = true
		}
	}
	if !found {
		t.Fatalf("node spans do not contain declaration/statement path: %#v", parsed.F0.NodeSpans)
	}
	if !statementFound {
		t.Fatalf("node spans do not contain statement span: %#v", parsed.F0.NodeSpans)
	}
}

func TestLayoutEventsCarryBranchTokenAnchors(t *testing.T) {
	source := "package sample\nfunc f(x: Bool) -> Int\n  if x then\n    1\n  else\n    2\n  end\nend\n"
	got := ParseFileLossless("anchors.mygo", source)
	parsed, ok := got.(Result__Ok[LosslessFile, string])
	if !ok {
		t.Fatalf("ParseFileLossless() failed: %v", got)
	}
	elseFound := false
	for _, event := range parsed.F0.LayoutEvents {
		if event.Kind == "enter:if-else" {
			// The branch exit points at the enclosing if's shared `end`,
			// which lies past the else-branch body span.
			elseFound = event.Anchor.Start.Line > 0 && event.HeaderLine == event.Anchor.Start.Line && event.BodyLine == event.Span.Start.Line && event.ExitLine > event.Span.End.Line
		}
	}
	if !elseFound {
		t.Fatalf("layout events do not contain an else anchor: %#v", parsed.F0.LayoutEvents)
	}
}

func TestLayoutEventsCarryInlineIfBoundaryAnchors(t *testing.T) {
	source := "package sample\nfunc f(value: Bool) -> Int\n  let rendered = if value then 1 else 2 end\n  rendered\nend\n"
	got := ParseFileLossless("inline-anchors.mygo", source)
	parsed, ok := got.(Result__Ok[LosslessFile, string])
	if !ok {
		t.Fatalf("ParseFileLossless() failed: %v", got)
	}
	var ifEvent, thenEvent, elseEvent *LayoutEvent
	for index := range parsed.F0.LayoutEvents {
		event := &parsed.F0.LayoutEvents[index]
		switch event.Kind {
		case "enter:if":
			ifEvent = event
		case "enter:if-then":
			thenEvent = event
		case "enter:if-else":
			elseEvent = event
		}
	}
	if ifEvent == nil || thenEvent == nil || elseEvent == nil {
		t.Fatalf("missing inline branch events: %#v", parsed.F0.LayoutEvents)
	}
	if ifEvent.ExitAnchor.Start.Line != 3 || ifEvent.ExitAnchor.Start.Column <= elseEvent.Anchor.End.Column {
		t.Fatalf("inline if exit anchor = %#v, else anchor = %#v", ifEvent.ExitAnchor, elseEvent.Anchor)
	}
	if len(thenEvent.Path) != len(ifEvent.Path)+1 || len(elseEvent.Path) != len(ifEvent.Path)+1 {
		t.Fatalf("inline branch paths do not descend from if: if=%v then=%v else=%v", ifEvent.Path, thenEvent.Path, elseEvent.Path)
	}
}

func TestLayoutEventsCarryCaseBodyBoundaryAnchors(t *testing.T) {
	source := "package sample\nfunc f(x: Int) -> Int\n  switch x\n    case VeryLongPattern => veryLongCaseBody\n  end\nend\n"
	got := ParseFileLossless("case-anchors.mygo", source)
	parsed, ok := got.(Result__Ok[LosslessFile, string])
	if !ok {
		t.Fatalf("ParseFileLossless() failed: %v", got)
	}
	for _, event := range parsed.F0.LayoutEvents {
		if event.Kind == "enter:case" && event.SeparatorAnchor.Start.Line == 4 && event.SeparatorAnchor.End.Column <= event.BodyAnchor.Start.Column {
			return
		}
	}
	t.Fatalf("case event lacks separator anchor: %#v", parsed.F0.LayoutEvents)
}

func TestLosslessCarriesTopLevelDelimitedSeparators(t *testing.T) {
	got := ParseFileLossless("delimited.mygo", "package sample\nfunc f() -> Int\n  let values = [1, call(2, 3), 4]\n  0\nend\n")
	parsed, ok := got.(Result__Ok[LosslessFile, string])
	if !ok {
		t.Fatalf("ParseFileLossless() failed: %v", got)
	}
	for _, item := range parsed.F0.Delimited {
		if item.Kind == "delimited:slice" && len(item.Items) == 3 && len(item.Separators) == 2 {
			return
		}
	}
	t.Fatalf("missing top-level delimited anchors: %#v", parsed.F0.Delimited)
}

func TestLosslessProtectedLinesIncludeCommentsAndLiterals(t *testing.T) {
	got := ParseFileLossless("protected.mygo", "package sample\n# keep   comment\nfunc f() -> String\n  \"hello   world\"\nend\n")
	parsed, ok := got.(Result__Ok[LosslessFile, string])
	if !ok {
		t.Fatalf("ParseFileLossless() failed: %v", got)
	}
	lines := LosslessProtectedLines(parsed.F0)
	foundComment, foundLiteral := false, false
	for _, line := range lines {
		foundComment = foundComment || line == 2
		foundLiteral = foundLiteral || line == 4
	}
	if !foundComment || !foundLiteral {
		t.Fatalf("protected lines = %#v", lines)
	}
}

func TestLosslessProtectedLinesCoverTripleQuotedLiteralSpan(t *testing.T) {
	got := ParseFileLossless("triple-protected.mygo", "package sample\nfunc f() -> String\n  \"\"\"first   line\n  second   line\n  \"\"\"\nend\n")
	parsed, ok := got.(Result__Ok[LosslessFile, string])
	if !ok {
		t.Fatalf("ParseFileLossless() failed: %v", got)
	}
	lines := LosslessProtectedLines(parsed.F0)
	foundStart, foundContent, foundEnd := false, false, false
	for _, line := range lines {
		foundStart = foundStart || line == 3
		foundContent = foundContent || line == 4
		foundEnd = foundEnd || line == 5
	}
	if !foundStart || !foundContent || !foundEnd {
		t.Fatalf("triple-quoted literal lines were not protected: %#v", lines)
	}
}

func TestLosslessProtectedLinesCoverRawLiteralSpan(t *testing.T) {
	got := ParseFileLossless("raw-protected.mygo", "package sample\nfunc f() -> String\n  `first   line\n  second   line`\nend\n")
	parsed, ok := got.(Result__Ok[LosslessFile, string])
	if !ok {
		t.Fatalf("ParseFileLossless() failed: %v", got)
	}
	lines := LosslessProtectedLines(parsed.F0)
	foundStart, foundEnd := false, false
	for _, line := range lines {
		foundStart = foundStart || line == 3
		foundEnd = foundEnd || line == 4
	}
	if !foundStart || !foundEnd {
		t.Fatalf("raw literal lines were not protected: %#v", lines)
	}
}


func TestSpannedTypeAndPatternParsersCaptureStateRanges(t *testing.T) {
	exprReply := ps.ParseInput(spannedExpr(), "value.Apply(1)")
	if !exprReply.Ok || exprReply.Value.Span.Start.Column != 1 || exprReply.Value.Span.End.Column <= exprReply.Value.Span.Start.Column {
		t.Fatalf("expression span = %#v", exprReply.Value.Span)
	}
	typeReply := ps.ParseInput(spannedTypeExpr(), "Box[Int]")
	if !typeReply.Ok || typeReply.Value.Span.Start.Column != 1 || typeReply.Value.Span.End.Column <= typeReply.Value.Span.Start.Column {
		t.Fatalf("type span = %#v", typeReply.Value.Span)
	}
	patternReply := ps.ParseInput(spannedPattern(), "Some(value)")
	if !patternReply.Ok || patternReply.Value.Span.Start.Column != 1 || patternReply.Value.Span.End.Column <= patternReply.Value.Span.Start.Column {
		t.Fatalf("pattern span = %#v", patternReply.Value.Span)
	}
	nestedTypeReply := ps.ParseInput(spannedTypeExpr(), "Result[Map[String, List[Int]]]")
	if !nestedTypeReply.Ok || nestedTypeReply.Value.Span.End.Column <= nestedTypeReply.Value.Span.Start.Column {
		t.Fatalf("nested type span = %#v", nestedTypeReply.Value.Span)
	}
	nestedPatternReply := ps.ParseInput(spannedPattern(), "Some((left, Right(value)))")
	if !nestedPatternReply.Ok || nestedPatternReply.Value.Span.End.Column <= nestedPatternReply.Value.Span.Start.Column {
		t.Fatalf("nested pattern span = %#v", nestedPatternReply.Value.Span)
	}
	literalReply := ps.ParseInput(spannedLiteral(), "\"literal\"")
	if !literalReply.Ok || literalReply.Value.Span.Start.Column != 1 || literalReply.Value.Span.End.Column <= literalReply.Value.Span.Start.Column {
		t.Fatalf("literal span = %#v", literalReply.Value.Span)
	}
}

func TestLosslessFlattensNestedTypeAndPatternPaths(t *testing.T) {
	source := "package sample\nfunc f(value: Result[Map[String, List[Int]]]) -> Int\n  switch value\n    case Some((left, Right(1))) => 1\n  end\nend\n"
	got := ParseFileLossless("nested-spans.mygo", source)
	parsed, ok := got.(Result__Ok[LosslessFile, string])
	if !ok {
		t.Fatalf("ParseFileLossless() failed: %v", got)
	}
	nestedType := false
	nestedPattern := false
	patternLiteral := false
	for _, item := range parsed.F0.NodeSpans {
		if item.Kind == "type" && len(item.Path) >= 5 {
			nestedType = true
		}
		if item.Kind == "pattern" && len(item.Path) >= 2 {
			nestedPattern = true
		}
		if item.Kind == "pattern-literal" {
			patternLiteral = true
		}
	}
	if !nestedType || !nestedPattern || !patternLiteral {
		t.Fatalf("missing flattened nested spans: %#v", parsed.F0.NodeSpans)
	}
}

func TestLosslessFlattensTypeDeclarationAndFunctionReturnSpans(t *testing.T) {
	source := "package sample\ntype Alias = Result[Map[String, Int]]\nfunc f() -> Result[List[Int]]\n  1\nend\n"
	got := ParseFileLossless("type-roots.mygo", source)
	parsed, ok := got.(Result__Ok[LosslessFile, string])
	if !ok {
		t.Fatalf("ParseFileLossless() failed: %v", got)
	}
	aliasRoot := false
	returnRoot := false
	for _, item := range parsed.F0.NodeSpans {
		if item.Kind != "type" {
			continue
		}
		if len(item.Path) == 2 && item.Path[0] == 0 && item.Path[1] == 0 && item.Span.Start.Line == 2 {
			aliasRoot = true
		}
		if len(item.Path) == 2 && item.Path[0] == 1 && item.Path[1] == -1 && item.Span.Start.Line == 3 {
			returnRoot = true
		}
	}
	if !aliasRoot || !returnRoot {
		t.Fatalf("missing declaration type roots: %#v", parsed.F0.NodeSpans)
	}
}

func TestLayoutEventsExposeParserOwnedIndentEffect(t *testing.T) {
	got := ParseFileLossless("indent-events.mygo", "package sample\nstruct Point\n  value: Int\nend\nfunc f() -> Int\n  1\nend\n")
	parsed, ok := got.(Result__Ok[LosslessFile, string])
	if !ok {
		t.Fatalf("ParseFileLossless() failed: %v", got)
	}
	foundStruct, foundFunc, foundStructExit, foundFuncExit := false, false, false, false
	for _, item := range parsed.F0.LayoutEvents {
		if item.Kind == "enter:decl:struct" {
			foundStruct = item.AffectsIndent
		}
		if item.Kind == "enter:decl:func" {
			foundFunc = item.AffectsIndent
		}
		if item.Kind == "exit:decl:struct" {
			foundStructExit = item.AffectsIndent
		}
		if item.Kind == "exit:decl:func" {
			foundFuncExit = item.AffectsIndent
		}
	}
	if !foundStruct || !foundFunc || !foundStructExit || !foundFuncExit {
		t.Fatalf("parser omitted indentation semantics: %#v", parsed.F0.LayoutEvents)
	}
}

func TestParseFileParsesSelf(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine parser test path")
	}
	sourcePath := filepath.Join(filepath.Dir(thisFile), "parser.mygo")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read %s: %v", sourcePath, err)
	}
	reply := ps.ParseInput(fileParser(), string(source))
	if !reply.Ok {
		t.Fatalf("ParseInput(%s) failed at %#v: %#v", sourcePath, reply.State.Position, reply.Error)
	}
	got := ParseFile(string(source))
	parsed, ok := got.(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFile(%s) failed: %v", sourcePath, got)
	}
	if len(parsed.F0.Decls) == 0 {
		t.Fatalf("ParseFile(%s) returned no declarations", sourcePath)
	}
}

func TestParseTypeAliasAndDefinedType(t *testing.T) {
	parsed := ParseFile(`package sample

type UserID = Int
type AccountID Int
`)
	file, ok := parsed.(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFile failed: %v", parsed)
	}
	if _, ok := file.F0.Decls[0].(ast2.Decl__TypeAliasDecl); !ok {
		t.Fatalf("decl[0] = %T, want DeclTypeAliasDecl", file.F0.Decls[0])
	}
	if _, ok := file.F0.Decls[1].(ast2.Decl__TypeDecl); !ok {
		t.Fatalf("decl[1] = %T, want DeclTypeDecl", file.F0.Decls[1])
	}
}

func TestParseFileParsesPrelude(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine parser test path")
	}
	sourcePath := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "prelude", "prelude.mygo")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read %s: %v", sourcePath, err)
	}
	got := ParseFileAt(sourcePath, string(source))
	parsed, ok := got.(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFileAt(%s) failed: %v", sourcePath, got)
	}
	if len(parsed.F0.Decls) == 0 {
		t.Fatalf("ParseFileAt(%s) returned no declarations", sourcePath)
	}
	if parsed.F0.SourceName != sourcePath || parsed.F0.Line != 1 || parsed.F0.Column != 1 {
		t.Fatalf("file position = %q:%d:%d, want %q:1:1", parsed.F0.SourceName, parsed.F0.Line, parsed.F0.Column, sourcePath)
	}
	if len(parsed.F0.DeclPositions) != len(parsed.F0.Decls) || parsed.F0.DeclPositions[0].SourceName != sourcePath || parsed.F0.DeclPositions[0].Line != 3 {
		t.Fatalf("first declaration position = %#v, want %q:3:*", parsed.F0.DeclPositions[0], sourcePath)
	}
}

func TestParseFileParsesPreludeConversion(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine parser test path")
	}
	sourcePath := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "prelude", "conversion.mygo")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read %s: %v", sourcePath, err)
	}
	parsed := ParseFileAt(sourcePath, string(source))
	if _, ok := parsed.(Result__Ok[ast2.File, string]); !ok {
		t.Fatalf("ParseFileAt(%s) failed: %v", sourcePath, parsed)
	}
}

func TestParseFileParsesPreludeMapImpl(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine parser test path")
	}
	sourcePath := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "prelude", "map.mygo")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read %s: %v", sourcePath, err)
	}
	parsed := ParseFileAt(sourcePath, string(source))
	if _, ok := parsed.(Result__Ok[ast2.File, string]); !ok {
		t.Fatalf("ParseFileAt(%s) failed: %v", sourcePath, parsed)
	}
}

func TestParseFileParsesPreludeStringIndexImpl(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine parser test path")
	}
	sourcePath := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "prelude", "stringindexrune.mygo")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read %s: %v", sourcePath, err)
	}
	parsed := ParseFileAt(sourcePath, string(source))
	if _, ok := parsed.(Result__Ok[ast2.File, string]); !ok {
		t.Fatalf("ParseFileAt(%s) failed: %v", sourcePath, parsed)
	}
}

func TestParseFunctionLiteral(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func make(values: Slice[ast2.Decl])
func(_: String) -> ps.Parser[ast2.File]
    ps.PBind(kw("x"), func(_: String) -> ps.Parser[ast2.File]
      ps.PPure(ast2.File { PackageName: "sample", Decls: values })
    end)
  end
end
`)
	body := fn.F4.Kind.(ast2.ExprKind__BlockExpr)
	got := body.F0[0].(ast2.Stmt__ExprStmt).F0
	if _, ok := got.Kind.(ast2.ExprKind__FuncLitExpr); !ok {
		t.Fatalf("body = %T, want ExprFuncLitExpr", got)
	}
}

func TestParseLetRecBindingGroup(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func parity(n: Int) -> Bool
  letrec
    even: func(Int) -> Bool = func(value: Int) -> Bool
      if value == 0 => true else odd(value - 1)
    end
    odd: func(Int) -> Bool = func(value: Int) -> Bool
      if value == 0 => false else even(value - 1)
    end
  end
  even(n)
end
`)
	body := fn.F4.Kind.(ast2.ExprKind__BlockExpr)
	if len(body.F0) != 2 {
		t.Fatalf("body statement count = %d, want 2", len(body.F0))
	}
	rec, ok := body.F0[0].(ast2.Stmt__LetRecStmt)
	if !ok {
		t.Fatalf("first statement = %T, want StmtLetRecStmt", body.F0[0])
	}
	if len(rec.F0) != 2 || rec.F0[0].Name != "even" || rec.F0[1].Name != "odd" {
		t.Fatalf("letrec bindings = %#v, want even/odd", rec.F0)
	}
}

func TestParseNestedTupleLetPattern(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func unpack()
  let (first, (_, last)) = (1, (2, 3))
end
`)
	body := fn.F4.Kind.(ast2.ExprKind__BlockExpr)
	stmt, ok := body.F0[0].(ast2.Stmt__TupleLetStmt)
	if !ok {
		t.Fatalf("first statement = %T, want StmtTupleLetStmt", body.F0[0])
	}
	pattern, ok := stmt.F0.(ast2.Pattern__TuplePattern)
	if !ok || len(pattern.F0) != 2 {
		t.Fatalf("tuple pattern = %#v, want two items", stmt.F0)
	}
	if bind, ok := pattern.F0[0].(ast2.Pattern__BindPattern); !ok || bind.F0 != "first" {
		t.Fatalf("first pattern = %#v, want BindPattern(first)", pattern.F0[0])
	}
	nested, ok := pattern.F0[1].(ast2.Pattern__TuplePattern)
	if !ok || len(nested.F0) != 2 {
		t.Fatalf("nested pattern = %#v, want (_, last)", pattern.F0[1])
	}
	if _, ok := nested.F0[0].(ast2.Pattern__WildcardPattern); !ok {
		t.Fatalf("nested first pattern = %T, want WildcardPattern", nested.F0[0])
	}
	if bind, ok := nested.F0[1].(ast2.Pattern__BindPattern); !ok || bind.F0 != "last" {
		t.Fatalf("nested second pattern = %#v, want BindPattern(last)", nested.F0[1])
	}
}

func TestParseVerbatimTripleQuotedAndRawStrings(t *testing.T) {
	fn := parseSingleFunc(t, "package sample\n\nfunc strings()\n  let triple = \"\"\"first\\\\n\nsecond\"\"\"\n  let raw = `first\\\\nsecond`\nend\n")
	body := fn.F4.Kind.(ast2.ExprKind__BlockExpr)
	for index, want := range []string{"first\\\\n\nsecond", "first\\\\nsecond"} {
		stmt, ok := body.F0[index].(ast2.Stmt__LetStmt)
		if !ok {
			t.Fatalf("statement[%d] = %T, want StmtLetStmt", index, body.F0[index])
		}
		literal, ok := stmt.F0.Value.Kind.(ast2.ExprKind__StringExpr)
		if !ok || literal.F0 != want {
			t.Fatalf("statement[%d] literal = %#v, want %q", index, stmt.F0.Value.Kind, want)
		}
	}
}

func TestParseSliceLiteralWithTrailingCommaAndTypeAs(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func values()
  [1, 2,] as Slice[Int]
end
`)
	body := fn.F4.Kind.(ast2.ExprKind__BlockExpr)
	cast, ok := body.F0[0].(ast2.Stmt__ExprStmt).F0.Kind.(ast2.ExprKind__TypeAsExpr)
	if !ok {
		t.Fatalf("body = %T, want ExprTypeAsExpr", body.F0[0].(ast2.Stmt__ExprStmt).F0)
	}
	slice, ok := (cast.F0).Kind.(ast2.ExprKind__SliceLitExpr)
	if !ok || len(slice.F0) != 2 {
		t.Fatalf("cast value = %T, want two-item ExprSliceLitExpr", cast.F0)
	}
}

func TestParseNumericLiteralSyntax(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func values()
  [18_446_744u64, 3.14f32, 0xff, 0XFFi8, 0o777u, 0B1010u64]
end
`)
	body := fn.F4.Kind.(ast2.ExprKind__BlockExpr)
	lit := body.F0[0].(ast2.Stmt__ExprStmt).F0.Kind.(ast2.ExprKind__SliceLitExpr)
	want := []string{"18446744u64", "3.14f32", "0xff", "0XFFi8", "0o777u", "0B1010u64"}
	if len(lit.F0) != len(want) {
		t.Fatalf("literal count = %d, want %d", len(lit.F0), len(want))
	}
	for i, expected := range want {
		got, ok := lit.F0[i].Kind.(ast2.ExprKind__NumberExpr)
		if !ok || got.F0 != expected {
			t.Fatalf("literal[%d] = %#v, want number %q", i, lit.F0[i].Kind, expected)
		}
	}
}

func TestRejectInvalidNumericLiteralSyntax(t *testing.T) {
	for _, literal := range []string{"1.2.3", "1oops", "0x", "0b102", "1_"} {
		t.Run(literal, func(t *testing.T) {
			got := ParseFile("package sample\n\nfunc bad()\n  " + literal + "\nend\n")
			if _, ok := got.(Result__Err[ast2.File, string]); !ok {
				t.Fatalf("ParseFile(%q) = %v, want parse error", literal, got)
			}
		})
	}
}

func TestParseEscapedRuneLiterals(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func newline() -> Rune
  '\n'
end
`)
	body := fn.F4.Kind.(ast2.ExprKind__BlockExpr)
	runeExpr, ok := body.F0[0].(ast2.Stmt__ExprStmt).F0.Kind.(ast2.ExprKind__RuneExpr)
	if !ok {
		t.Fatalf("body = %T, want ExprRuneExpr", body.F0[0].(ast2.Stmt__ExprStmt).F0)
	}
	if runeExpr.F0 != "\n" {
		t.Fatalf("rune value = %q, want newline", runeExpr.F0)
	}
}

func TestParseRuneLiteralSwitchPatterns(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func escape(r: Rune) -> String
  switch r
    case '"' => "quote"
    case '\n' => "newline"
    case '\0' => "null"
    case _ => "other"
  end
end
`)
	body := fn.F4.Kind.(ast2.ExprKind__BlockExpr)
	switchExpr := body.F0[0].(ast2.Stmt__ExprStmt).F0.Kind.(ast2.ExprKind__SwitchExpr)
	for index, want := range []string{"\"", "\n", "\x00"} {
		pattern, ok := switchExpr.F1[index].Pattern.(ast2.Pattern__LiteralPattern)
		if !ok || pattern.F0 != "rune" || pattern.F1 != want {
			t.Fatalf("case %d pattern = %#v, want rune %q", index, switchExpr.F1[index].Pattern, want)
		}
	}
}

func TestParseSwitchVariantAndWildcardPatterns(t *testing.T) {
	got := ParseFile(`package sample

enum Maybe[A]
  Some(A)
  None
end

func unwrap(value: Maybe[Int]) -> Int
  switch value
    case Some(item) => item
    case None => 0
    case _ => -1
  end
end
`)
	parsed, ok := got.(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFile failed: %v", got)
	}
	fn, ok := parsed.F0.Decls[1].(ast2.Decl__FuncDecl)
	if !ok {
		t.Fatalf("decl[1] = %T, want DeclFuncDecl", parsed.F0.Decls[1])
	}
	body := fn.F4.Kind.(ast2.ExprKind__BlockExpr)
	sw, ok := body.F0[0].(ast2.Stmt__ExprStmt).F0.Kind.(ast2.ExprKind__SwitchExpr)
	if !ok {
		t.Fatalf("body = %T, want ExprSwitchExpr", body.F0[0].(ast2.Stmt__ExprStmt).F0)
	}
	if len(sw.F1) != 3 {
		t.Fatalf("case count = %d, want 3", len(sw.F1))
	}
	variant, ok := sw.F1[0].Pattern.(ast2.Pattern__VariantPattern)
	item, itemOK := variant.F1[0].(ast2.Pattern__BindPattern)
	if !ok || variant.F0 != "Some" || len(variant.F1) != 1 || !itemOK || item.F0 != "item" {
		t.Fatalf("first pattern = %#v, want Some(item)", sw.F1[0].Pattern)
	}
	if _, ok := sw.F1[2].Pattern.(ast2.Pattern__WildcardPattern); !ok {
		t.Fatalf("third pattern = %T, want PatternWildcardPattern", sw.F1[2].Pattern)
	}
}

func TestParseNamedStructEnumVariantsAndPatterns(t *testing.T) {
	parsed := ParseFile(`package sample

enum Shape
  Circle { radius: Float64 }
  Rectangle(Float64, Float64)
end

func area(shape: Shape) -> Float64
  switch shape
    case Circle { radius: r } => r
    case _ => 0.0
  end
end
`)
	file, ok := parsed.(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFile failed: %v", parsed)
	}
	shape := file.F0.Decls[0].(ast2.Decl__EnumDecl)
	circle := shape.F2[0]
	if !circle.Named || len(circle.Names) != 1 || circle.Names[0] != "radius" {
		t.Fatalf("Circle variant = %#v, want named radius field", circle)
	}
	fn := file.F0.Decls[1].(ast2.Decl__FuncDecl)
	body := fn.F4.Kind.(ast2.ExprKind__BlockExpr)
	sw := body.F0[0].(ast2.Stmt__ExprStmt).F0.Kind.(ast2.ExprKind__SwitchExpr)
	pat, ok := sw.F1[0].Pattern.(ast2.Pattern__StructVariantPattern)
	if !ok || pat.F0 != "Circle" || len(pat.F1) != 1 || pat.F1[0].Field != "radius" || pat.F1[0].Bind != "r" {
		t.Fatalf("pattern = %#v, want Circle { radius: r }", sw.F1[0].Pattern)
	}
}

func TestParseSwitchExpressionTarget(t *testing.T) {
	got := ParseFile(`package sample

enum Maybe[A]
  Some(A)
  None
end

func select(value: Maybe[Int]) -> Int
  switch value.map(func(item: Int) -> Int item + 1 end)
    case Some(item) => item
    case None => 0
  end
end
`)
	parsed, ok := got.(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFile failed: %v", got)
	}
	fn := parsed.F0.Decls[1].(ast2.Decl__FuncDecl)
	body := fn.F4.Kind.(ast2.ExprKind__BlockExpr)
	sw, ok := body.F0[0].(ast2.Stmt__ExprStmt).F0.Kind.(ast2.ExprKind__SwitchExpr)
	if !ok {
		t.Fatalf("body = %T, want ExprSwitchExpr", body.F0[0].(ast2.Stmt__ExprStmt).F0)
	}
	call, ok := sw.F0.Kind.(ast2.ExprKind__CallExpr)
	if !ok {
		t.Fatalf("switch target = %T, want ExprCallExpr", sw.F0.Kind)
	}
	field, ok := call.F0.Kind.(ast2.ExprKind__FieldExpr)
	if !ok || field.F1 != "map" {
		t.Fatalf("switch call callee = %#v, want value.map", call.F0.Kind)
	}
}

func TestParseSwitchCaseBlock(t *testing.T) {
	got := ParseFile(`package sample

enum Maybe[A]
  Some(A)
  None
end

func unwrap(value: Maybe[Int]) -> Int
  switch value
    case Some(item) then
      item
    end
    case None => 0
  end
end
`)
	parsed, ok := got.(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFile failed: %v", got)
	}
	fn := parsed.F0.Decls[1].(ast2.Decl__FuncDecl)
	body := fn.F4.Kind.(ast2.ExprKind__BlockExpr)
	sw := body.F0[0].(ast2.Stmt__ExprStmt).F0.Kind.(ast2.ExprKind__SwitchExpr)
	if len(sw.F1) != 2 {
		t.Fatalf("case count = %d, want 2", len(sw.F1))
	}
	if _, ok := sw.F1[0].Body.Kind.(ast2.ExprKind__IdentExpr); !ok {
		t.Fatalf("block case body = %T, want ast2.ExprIdentExpr", sw.F1[0].Body)
	}
}

func TestParseFileBasicDeclarations(t *testing.T) {
	src := `package sample

import fmt "go:fmt"

struct Point
  x: Int
  y: Int
end

enum Maybe[A]
  Some(A)
  None
end

func add(a: Int, b: Int) -> Int
  a + b
end
`

	got := ParseFile(src)
	ok, yes := got.(Result__Ok[ast2.File, string])
	if !yes {
		t.Fatalf("ParseFile failed: %v", got)
	}
	file := ok.F0
	if file.PackageName != "sample" {
		t.Fatalf("package = %q, want sample", file.PackageName)
	}
	if len(file.Decls) != 4 {
		t.Fatalf("decl count = %d, want 4", len(file.Decls))
	}
}

func TestParseExpressionPrecedence(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func calc(a: Int, b: Int, c: Int, d: Int) -> Int
  a + b * c - d
end
`)

	body := fn.F4.Kind.(ast2.ExprKind__BlockExpr)
	if len(body.F0) != 1 {
		t.Fatalf("body expr count = %d, want 1", len(body.F0))
	}
	first := body.F0[0].(ast2.Stmt__ExprStmt)
	root := first.F0.Kind.(ast2.ExprKind__BinaryExpr)
	if root.F0 != "-" {
		t.Fatalf("root op = %q, want -", root.F0)
	}
	left := (root.F1).Kind.(ast2.ExprKind__BinaryExpr)
	if left.F0 != "+" {
		t.Fatalf("left op = %q, want +", left.F0)
	}
	rightMul := (left.F2).Kind.(ast2.ExprKind__BinaryExpr)
	if rightMul.F0 != "*" {
		t.Fatalf("nested op = %q, want *", rightMul.F0)
	}
}

func TestParseLogicalAndUnaryExpressions(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func ok(a: Bool, b: Bool, c: Bool, n: Int) -> Bool
  !a || b && -n > 0
end
`)

	body := fn.F4.Kind.(ast2.ExprKind__BlockExpr)
	first := body.F0[0].(ast2.Stmt__ExprStmt)
	root := first.F0.Kind.(ast2.ExprKind__BinaryExpr)
	if root.F0 != "||" {
		t.Fatalf("root op = %q, want ||", root.F0)
	}
	left := (root.F1).Kind.(ast2.ExprKind__UnaryExpr)
	if left.F0 != "!" {
		t.Fatalf("left unary op = %q, want !", left.F0)
	}
	right := (root.F2).Kind.(ast2.ExprKind__BinaryExpr)
	if right.F0 != "&&" {
		t.Fatalf("right op = %q, want &&", right.F0)
	}
	cmp := (right.F2).Kind.(ast2.ExprKind__BinaryExpr)
	if cmp.F0 != ">" {
		t.Fatalf("comparison op = %q, want >", cmp.F0)
	}
	neg := (cmp.F1).Kind.(ast2.ExprKind__UnaryExpr)
	if neg.F0 != "-" {
		t.Fatalf("comparison left unary op = %q, want -", neg.F0)
	}
}

func TestParseBlockIfWithElsif(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func choose(a: Int) -> Int
  if a > 10 then
    1
  elsif a > 5 then
    2
  else
    3
  end
end
`)

	body := fn.F4.Kind.(ast2.ExprKind__BlockExpr)
	first := body.F0[0].(ast2.Stmt__ExprStmt)
	root := first.F0.Kind.(ast2.ExprKind__IfExpr)
	thenExpr := (root.F1).Kind.(ast2.ExprKind__NumberExpr)
	if thenExpr.F0 != "1" {
		t.Fatalf("then value = %q, want 1", thenExpr.F0)
	}
	nested := (root.F2).Kind.(ast2.ExprKind__IfExpr)
	nestedThen := (nested.F1).Kind.(ast2.ExprKind__NumberExpr)
	if nestedThen.F0 != "2" {
		t.Fatalf("elsif value = %q, want 2", nestedThen.F0)
	}
	elseExpr := (nested.F2).Kind.(ast2.ExprKind__NumberExpr)
	if elseExpr.F0 != "3" {
		t.Fatalf("else value = %q, want 3", elseExpr.F0)
	}
}

func TestParseVarDeclaration(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo() -> Int
  var x: Int = 42
  x
end
`)

	body := fn.F4.Kind.(ast2.ExprKind__BlockExpr)
	if len(body.F0) != 2 {
		t.Fatalf("body expr count = %d, want 2", len(body.F0))
	}
	varStmt, ok := body.F0[0].(ast2.Stmt__VarStmt)
	if !ok {
		t.Fatalf("first stmt = %T, want StmtVarStmt", body.F0[0])
	}
	if varStmt.F0.Name != "x" {
		t.Fatalf("var name = %q, want x", varStmt.F0.Name)
	}
}

func TestParseWhileLoop(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo(n: Int) -> Int
  while n > 0
    n
  end
  n
end
`)

	body := fn.F4.Kind.(ast2.ExprKind__BlockExpr)
	if len(body.F0) != 2 {
		t.Fatalf("body expr count = %d, want 2", len(body.F0))
	}
	whileStmt, ok := body.F0[0].(ast2.Stmt__WhileStmt)
	if !ok {
		t.Fatalf("first stmt = %T, want StmtWhileStmt", body.F0[0])
	}
	cond := whileStmt.F0.Kind.(ast2.ExprKind__BinaryExpr)
	if cond.F0 != ">" {
		t.Fatalf("while cond op = %q, want >", cond.F0)
	}
}

func TestParseReturnStatement(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo(n: Int) -> Int
  return n
end
`)

	body := fn.F4.Kind.(ast2.ExprKind__BlockExpr)
	retStmt, ok := body.F0[0].(ast2.Stmt__ReturnWithStmt)
	if !ok {
		t.Fatalf("stmt = %T, want StmtReturnWithStmt", body.F0[0])
	}
	identExpr := retStmt.F0.Kind.(ast2.ExprKind__IdentExpr)
	if identExpr.F0 != "n" {
		t.Fatalf("return ident = %q, want n", identExpr.F0)
	}
}

func TestParseBareReturnStatement(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo(n: Int)
  return
end
`)

	body := fn.F4.Kind.(ast2.ExprKind__BlockExpr)
	_, ok := body.F0[0].(ast2.Stmt__ReturnStmt)
	if !ok {
		t.Fatalf("stmt = %T, want StmtReturnStmt", body.F0[0])
	}
}

func TestParseInlineGoExpression(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo(n: Int) -> Int
  go[String]{code: "return strconv.Itoa(n)"}
end
`)

	body := fn.F4.Kind.(ast2.ExprKind__BlockExpr)
	first := body.F0[0].(ast2.Stmt__ExprStmt)
	_, ok := first.F0.Kind.(ast2.ExprKind__InlineGoExpr)
	if !ok {
		t.Fatalf("expr = %T, want ExprInlineGoExpr", first.F0)
	}
}

func TestParseInlineGoOperands(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo(n: Int) -> String
  go[String]{code: "{T}({v})" in v = n type T = String}
end
`)
	body := fn.F4.Kind.(ast2.ExprKind__BlockExpr)
	expr := body.F0[0].(ast2.Stmt__ExprStmt).F0.Kind.(ast2.ExprKind__InlineGoExpr)
	if len(expr.F2) != 1 || expr.F2[0].Name != "v" {
		t.Fatalf("value operands = %#v, want one v operand", expr.F2)
	}
	if len(expr.F3) != 1 || expr.F3[0].Name != "T" {
		t.Fatalf("type operands = %#v, want one T operand", expr.F3)
	}
}

func parseSingleFunc(t *testing.T, src string) ast2.Decl__FuncDecl {
	t.Helper()

	got := ParseFile(src)
	ok, yes := got.(Result__Ok[ast2.File, string])
	if !yes {
		t.Fatalf("ParseFile failed: %v", got)
	}
	if len(ok.F0.Decls) != 1 {
		t.Fatalf("decl count = %d, want 1", len(ok.F0.Decls))
	}
	fn, yes := ok.F0.Decls[0].(ast2.Decl__FuncDecl)
	if !yes {
		t.Fatalf("decl type = %T, want DeclFuncDecl", ok.F0.Decls[0])
	}
	return fn
}

func TestParseAssignSimpleVar(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo() -> Int
  var x: Int = 42
  x = 1
  x
end
`)

	body := fn.F4.Kind.(ast2.ExprKind__BlockExpr)
	if len(body.F0) != 3 {
		t.Fatalf("body expr count = %d, want 3", len(body.F0))
	}
	assign, ok := body.F0[1].(ast2.Stmt__AssignStmt)
	if !ok {
		t.Fatalf("second stmt = %T, want StmtAssignStmt", body.F0[1])
	}
	lhs := assign.F0
	rhs := assign.F1
	if lhs.Kind.(ast2.ExprKind__IdentExpr).F0 != "x" {
		t.Fatalf("assign lhs = %q, want x", lhs.Kind.(ast2.ExprKind__IdentExpr).F0)
	}
	if rhs.Kind.(ast2.ExprKind__NumberExpr).F0 != "1" {
		t.Fatalf("assign rhs = %q, want 1", rhs.Kind.(ast2.ExprKind__NumberExpr).F0)
	}
}

func TestParseAssignFieldSimple(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo()
  var p: Point = Point { x: 1, y: 2 }
  p.x = 99
end
`)

	body := fn.F4.Kind.(ast2.ExprKind__BlockExpr)
	assign, ok := body.F0[1].(ast2.Stmt__AssignStmt)
	if !ok {
		t.Fatalf("second stmt = %T, want StmtAssignStmt", body.F0[1])
	}
	lhs := assign.F0
	field, ok := lhs.Kind.(ast2.ExprKind__FieldExpr)
	if !ok {
		t.Fatalf("assign lhs = %T, want ExprFieldExpr", lhs)
	}
	if field.F1 != "x" {
		t.Fatalf("assign field name = %q, want x", field.F1)
	}
	obj := field.F0
	if obj.Kind.(ast2.ExprKind__IdentExpr).F0 != "p" {
		t.Fatalf("assign field obj = %q, want p", obj.Kind.(ast2.ExprKind__IdentExpr).F0)
	}
	rhs := assign.F1
	if rhs.Kind.(ast2.ExprKind__NumberExpr).F0 != "99" {
		t.Fatalf("assign rhs = %q, want 99", rhs.Kind.(ast2.ExprKind__NumberExpr).F0)
	}
}

func TestParseAssignFieldChain(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo()
  cfg.settings.theme = "dark"
end
`)

	body := fn.F4.Kind.(ast2.ExprKind__BlockExpr)
	assign, ok := body.F0[0].(ast2.Stmt__AssignStmt)
	if !ok {
		t.Fatalf("first stmt = %T, want StmtAssignStmt", body.F0[0])
	}
	// lhs = cfg.settings.theme
	lhs := assign.F0
	themeField, ok := lhs.Kind.(ast2.ExprKind__FieldExpr)
	if !ok {
		t.Fatalf("assign lhs = %T, want ExprFieldExpr", lhs)
	}
	if themeField.F1 != "theme" {
		t.Fatalf("outer field = %q, want theme", themeField.F1)
	}
	// cfg.settings
	inner := themeField.F0
	settingsField, ok := inner.Kind.(ast2.ExprKind__FieldExpr)
	if !ok {
		t.Fatalf("inner = %T, want ExprFieldExpr", inner)
	}
	if settingsField.F1 != "settings" {
		t.Fatalf("inner field = %q, want settings", settingsField.F1)
	}
	cfg := settingsField.F0
	if cfg.Kind.(ast2.ExprKind__IdentExpr).F0 != "cfg" {
		t.Fatalf("base ident = %q, want cfg", cfg.Kind.(ast2.ExprKind__IdentExpr).F0)
	}
	rhs := assign.F1
	if rhs.Kind.(ast2.ExprKind__StringExpr).F0 != "dark" {
		t.Fatalf("assign rhs = %q, want dark", rhs.Kind.(ast2.ExprKind__StringExpr).F0)
	}
}

func TestParseAssignInBlock(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo() -> Int
  var x: Int = 1
  var y: Int = 2
  x = y
  x + y
end
`)

	body := fn.F4.Kind.(ast2.ExprKind__BlockExpr)
	if len(body.F0) != 4 {
		t.Fatalf("body expr count = %d, want 4", len(body.F0))
	}
	_, ok := body.F0[2].(ast2.Stmt__AssignStmt)
	if !ok {
		t.Fatalf("third stmt = %T, want StmtAssignStmt", body.F0[2])
	}
}

func TestExpressionsCarrySourcePositions(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo() -> Int
  1 + 2
end
`)

	body := fn.F4
	if body.Pos.Line != 4 || body.Pos.Column != 3 {
		t.Fatalf("body position = %d:%d, want 4:3", body.Pos.Line, body.Pos.Column)
	}
	root := body.Kind.(ast2.ExprKind__BlockExpr).F0[0].(ast2.Stmt__ExprStmt).F0
	if root.Pos.Line != 4 || root.Pos.Column != 3 {
		t.Fatalf("root position = %d:%d, want 4:3", root.Pos.Line, root.Pos.Column)
	}
	binary := root.Kind.(ast2.ExprKind__BinaryExpr)
	if binary.F1.Pos.Line != 4 || binary.F1.Pos.Column != 3 {
		t.Fatalf("left operand position = %d:%d, want 4:3", binary.F1.Pos.Line, binary.F1.Pos.Column)
	}
	if binary.F2.Pos.Line != 4 || binary.F2.Pos.Column != 7 {
		t.Fatalf("right operand position = %d:%d, want 4:7", binary.F2.Pos.Line, binary.F2.Pos.Column)
	}
}
