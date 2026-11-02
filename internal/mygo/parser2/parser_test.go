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

type resultOkView[A any] struct {
	F0 A
}

type resultErrView[E any] struct {
	F0 E
}

type optionSomeView[A any] struct {
	F0 A
}

func resultOk[A, E any](result Result[A, E]) (resultOkView[A], bool) {
	if !MygoIN6ResultM4IsOk[A, E](result) {
		return resultOkView[A]{}, false
	}
	return resultOkView[A]{F0: MygoIN6ResultM6Unwrap[A, E](result)}, true
}

func resultErr[A, E any](result Result[A, E]) (resultErrView[E], bool) {
	if !MygoIN6ResultM5IsErr[A, E](result) {
		return resultErrView[E]{}, false
	}
	return resultErrView[E]{F0: MygoIN6ResultM9UnwrapErr[A, E](result)}, true
}

func optionSome[A any](option Option[A]) (optionSomeView[A], bool) {
	if !MygoIN6OptionM6IsSome[A](option) {
		return optionSomeView[A]{}, false
	}
	return optionSomeView[A]{F0: MygoIN6OptionM6Unwrap[A](option)}, true
}

func optionIsNone[A any](option Option[A]) bool {
	return MygoIN6OptionM6IsNone[A](option)
}

func requireFunctionDecl(t *testing.T, file ast2.File, index int) ast2.Decl {
	t.Helper()
	if index < 0 || index >= len(file.Decls) || !ast2.DeclIsFunc(file.Decls[index]) {
		t.Fatalf("decls[%d] is not a function declaration: %#v", index, file.Decls)
	}
	return file.Decls[index]
}

func requireBlockExpr(t *testing.T, expr ast2.Expr) ast2.Expr {
	t.Helper()
	if !ast2.ExprIsBlock(expr) {
		t.Fatalf("expression is not a block: %#v", expr)
	}
	return expr
}

func requireExprStatement(t *testing.T, statements []ast2.Stmt, index int) ast2.Expr {
	t.Helper()
	if index < 0 || index >= len(statements) || !ast2.StmtIsExpr(statements[index]) {
		t.Fatalf("statement %d is not an expression statement: %#v", index, statements)
	}
	return ast2.StmtExpr(statements[index])
}

func requireSwitchExpr(t *testing.T, expr ast2.Expr) ast2.SwitchExprView {
	t.Helper()
	if !ast2.ExprIsSwitch(expr) {
		t.Fatalf("expression is not a switch: %#v", expr)
	}
	return ast2.ExprSwitch(expr)
}

func TestParseFileAtIncludesSourceLocation(t *testing.T) {
	got := ParseFileAt("broken.mygo", "package sample\n\nfunc")
	err, ok := resultErr[ast2.File, string](got)
	if !ok {
		t.Fatalf("ParseFileAt() = %T, want parse error", got)
	}
	if !strings.Contains(err.F0, "broken.mygo:3:5: error [syntax]: parse error: expected identifier") {
		t.Fatalf("ParseFileAt() error = %q, want source name, line, column, and expectation", err.F0)
	}
}

// TestParseKeepsNestedBareConstructorAsBindPattern documents the parse that
// motivates the bare-variant resolution in typeinference2: `case Ok(None)`
// parses to `VariantPattern("Ok", [BindPattern("None")])`.  The ambiguity is
// context-free on purpose; only the switch target enum disambiguates it.
func TestParseKeepsNestedBareConstructorAsBindPattern(t *testing.T) {
	parsed := ParseFile(`package sample

func unwrapOr(t: Result[Option[Int], String]) -> Int
  switch t
    case Ok(Some(x)) => x
    case Ok(None) => 0
  end
end
`)
	file, ok := resultOk[ast2.File, string](parsed)
	if !ok {
		t.Fatalf("ParseFile failed: %v", parsed)
	}
	decl := requireFunctionDecl(t, file.F0, 0)
	body := requireBlockExpr(t, ast2.DeclFuncBody(decl))
	sw := requireSwitchExpr(t, requireExprStatement(t, ast2.ExprBlockStatements(body), 0))
	if len(sw.Cases) != 2 {
		t.Fatalf("cases = %d, want 2", len(sw.Cases))
	}
	if !ast2.PatternIsVariant(sw.Cases[1].Pattern) {
		t.Fatalf("second case pattern is not a variant: %#v", sw.Cases[1].Pattern)
	}
	noneCase := ast2.PatternVariant(sw.Cases[1].Pattern)
	if noneCase.Name != "Ok" || len(noneCase.Items) != 1 {
		t.Fatalf("second case = %q with %d args, want Ok with 1 arg", noneCase.Name, len(noneCase.Items))
	}
	if !ast2.PatternIsBind(noneCase.Items[0]) {
		t.Fatalf("inner pattern is not a bind: %#v", noneCase.Items[0])
	}
	if ast2.PatternBindName(noneCase.Items[0]) != "None" {
		t.Fatalf("inner bind = %q, want None", ast2.PatternBindName(noneCase.Items[0]))
	}
}

func TestParseCaseGuardsIntoSwitchCases(t *testing.T) {
	parsed := ParseFile(`package sample

enum Maybe
  Some(Int)
  None
end

func f(v: Maybe, flag: Bool) -> Int
  switch v
    case Some(x) if x > 100 then
      x
    end
    case None if flag => -1
    case _ => 0
  end
end
`)
	file, ok := resultOk[ast2.File, string](parsed)
	if !ok {
		t.Fatalf("ParseFile failed: %v", parsed)
	}
	decl := requireFunctionDecl(t, file.F0, 1)
	body := requireBlockExpr(t, ast2.DeclFuncBody(decl))
	sw := requireSwitchExpr(t, requireExprStatement(t, ast2.ExprBlockStatements(body), 0))
	if len(sw.Cases) != 3 {
		t.Fatalf("cases = %d, want 3", len(sw.Cases))
	}

	thenGuard, ok := optionSome[ast2.Expr](sw.Cases[0].Guard)
	if !ok {
		t.Fatalf("first case guard is not Some: %#v", sw.Cases[0].Guard)
	}
	if !ast2.ExprIsBinary(thenGuard.F0) || ast2.ExprBinary(thenGuard.F0).Operator != ">" {
		t.Fatalf("first guard is not a > binary: %#v", thenGuard.F0)
	}

	arrowGuard, ok := optionSome[ast2.Expr](sw.Cases[1].Guard)
	if !ok {
		t.Fatalf("second case guard is not Some: %#v", sw.Cases[1].Guard)
	}
	if !ast2.ExprIsIdent(arrowGuard.F0) {
		t.Fatalf("second guard is not an identifier: %#v", arrowGuard.F0)
	}
	if ast2.ExprIdentName(arrowGuard.F0) != "flag" {
		t.Fatalf("second guard is %q, want flag identifier", ast2.ExprIdentName(arrowGuard.F0))
	}

	if !optionIsNone[ast2.Expr](sw.Cases[2].Guard) {
		t.Fatalf("wildcard guard is not None: %#v", sw.Cases[2].Guard)
	}
}

func TestParseCaseGuardAllowsParenthesizedIfExpression(t *testing.T) {
	parsed := ParseFile(`package sample

enum Maybe
  Some(Int)
  None
end

func f(v: Maybe) -> Int
  switch v
    case Some(x) if (if x > 0 then true else false end) => x
    case _ => 0
  end
end
`)
	file, ok := resultOk[ast2.File, string](parsed)
	if !ok {
		t.Fatalf("ParseFile failed: %v", parsed)
	}
	decl := requireFunctionDecl(t, file.F0, 1)
	body := requireBlockExpr(t, ast2.DeclFuncBody(decl))
	sw := requireSwitchExpr(t, requireExprStatement(t, ast2.ExprBlockStatements(body), 0))
	guard, ok := optionSome[ast2.Expr](sw.Cases[0].Guard)
	if !ok {
		t.Fatalf("guard is not Some: %#v", sw.Cases[0].Guard)
	}
	if !ast2.ExprIsIf(guard.F0) {
		t.Fatalf("guard is not IfExpr: %#v", guard.F0)
	}
}

func TestParseCaseGuardRejectsUnparenthesizedIfExpression(t *testing.T) {
	parsed := ParseFile(`package sample

enum Maybe
  Some(Int)
  None
end

func f(v: Maybe, a: Bool, flag: Bool) -> Int
  switch v
    case Some(x) if a && if flag then true else false end => x
    case _ => 0
  end
end
`)
	err, ok := resultErr[ast2.File, string](parsed)
	if !ok {
		t.Fatalf("ParseFile = %T, want parse error", parsed)
	}
	if !strings.Contains(err.F0, "case guard if-expression must be parenthesized") {
		t.Fatalf("error = %q, want parenthesization diagnostic", err.F0)
	}
}

func TestParseCaseGuardRequiresExpression(t *testing.T) {
	parsed := ParseFile(`package sample

enum Maybe
  Some(Int)
  None
end

func f(v: Maybe) -> Int
  switch v
    case Some(x) if => x
    case _ => 0
  end
end
`)
	err, ok := resultErr[ast2.File, string](parsed)
	if !ok {
		t.Fatalf("ParseFile = %T, want parse error", parsed)
	}
	if !strings.Contains(err.F0, "case guard requires a boolean expression") {
		t.Fatalf("error = %q, want missing-guard diagnostic", err.F0)
	}
}

func TestParseIfExpressionAfterCaseTerminatorIsBody(t *testing.T) {
	parsed := ParseFile(`package sample

func f(v: Int, flag: Bool) -> Int
  switch v
    case 0 => if flag then 1 else 0 end
    case _ => 0
  end
end
`)
	file, ok := resultOk[ast2.File, string](parsed)
	if !ok {
		t.Fatalf("ParseFile failed: %v", parsed)
	}
	decl := requireFunctionDecl(t, file.F0, 0)
	body := requireBlockExpr(t, ast2.DeclFuncBody(decl))
	sw := requireSwitchExpr(t, requireExprStatement(t, ast2.ExprBlockStatements(body), 0))
	if !optionIsNone[ast2.Expr](sw.Cases[0].Guard) {
		t.Fatalf("first guard is not None: %#v", sw.Cases[0].Guard)
	}
	if !ast2.ExprIsIf(sw.Cases[0].Body) {
		t.Fatalf("first body is not IfExpr: %#v", sw.Cases[0].Body)
	}
}

func TestParseCaseGuardLosslessRoundTrip(t *testing.T) {
	source := `package sample

enum Maybe
  Some(Int)
  None
end

func f(v: Maybe, flag: Bool) -> Int
  switch v
    case Some(x) if x > 0 => x
    case _ if flag => 0 # wildcard guard comment
    case _ => -1
  end
end
`
	tree := ParseSyntaxAt("guard.mygo", source)
	if len(tree.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %v, want none", tree.Diagnostics)
	}
	if got := cstNodeText(*tree.Root); got != source {
		t.Fatalf("round trip mismatch:\n got %q\nwant %q", got, source)
	}
	if !cstNodeHasKindForTest(*tree.Root, CstNodeKindIsCaseGuard) {
		t.Fatal("CST lacks a CaseGuard region")
	}
}

func cstNodeHasKindForTest(node CstNode, match func(CstNodeKind) bool) bool {
	if match(node.Kind) {
		return true
	}
	for _, item := range node.Children {
		if CstElementHasNode(item) && cstNodeHasKindForTest(*CstElementNodeOrEmpty(item), match) {
			return true
		}
	}
	return false
}

func TestParseFileLosslessRetainsTokensAndTrivia(t *testing.T) {
	source := "package sample\n# keep   this\nfunc f() -> String\n  \"hello   world\"\nend\n"
	got := ParseFileLossless("sample.mygo", source)
	result, ok := resultOk[LosslessFile, string](got)
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

func TestParseFileLosslessCarriesInlineGoLayoutEvent(t *testing.T) {
	source := "package sample\nfunc f() -> Int\n  go[Int] {\n    code: \"1\"\n  }\nend\n"
	got := ParseFileLossless("inline-go.mygo", source)
	parsed, ok := resultOk[LosslessFile, string](got)
	if !ok {
		t.Fatalf("ParseFileLossless() failed: %v", got)
	}
	for _, event := range parsed.F0.LayoutEvents {
		if event.Kind == "enter:inline-go" {
			if !event.AffectsIndent || event.Anchor.Start.Line != 3 || event.ExitAnchor.Start.Line != 5 {
				t.Fatalf("inline Go layout event = %#v", event)
			}
			return
		}
	}
	t.Fatalf("missing inline Go layout event: %#v", parsed.F0.LayoutEvents)
}

func TestParseFileLosslessKeepsRangeBranchesBlockStructured(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "prelude", "range.mygo")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := ParseFileLossless(path, string(source))
	parsed, ok := resultOk[LosslessFile, string](got)
	if !ok {
		t.Fatalf("ParseFileLossless() failed: %v", got)
	}
	for _, event := range parsed.F0.LayoutEvents {
		if event.Kind == "enter:if-then" || event.Kind == "enter:if-else" {
			if event.ExpandsAfter {
				t.Fatalf("block branch was marked expandable: %#v", event)
			}
			if !event.AffectsIndent {
				t.Fatalf("block branch did not affect indentation: %#v", event)
			}
		}
	}
}

func TestExpressionsCarrySourceSpans(t *testing.T) {
	got := ParseFileAt("span.mygo", "package sample\nfunc f() -> Int\n  1\nend\n")
	parsed, ok := resultOk[ast2.File, string](got)
	if !ok {
		t.Fatalf("ParseFileAt() failed: %v", got)
	}
	fn := requireFunctionDecl(t, parsed.F0, 0)
	body := requireBlockExpr(t, ast2.DeclFuncBody(fn))
	value := requireExprStatement(t, ast2.ExprBlockStatements(body), 0)
	if value.Span.Start.SourceName != "span.mygo" || value.Span.Start.Line == 0 {
		t.Fatalf("expression span = %#v", value.Span)
	}
}

func TestParametersCarrySourceSpans(t *testing.T) {
	got := ParseFileAt("param-span.mygo", "package sample\nfunc f(value: Option[Int]) -> Int\n  1\nend\n")
	parsed, ok := resultOk[ast2.File, string](got)
	if !ok {
		t.Fatalf("ParseFileAt() failed: %v", got)
	}
	parameters := ast2.DeclFuncArguments(requireFunctionDecl(t, parsed.F0, 0))
	if len(parameters) != 1 || parameters[0].Span.Start.Line == 0 || parameters[0].Span.End.Column <= parameters[0].Span.Start.Column {
		t.Fatalf("parameter span = %#v", parameters)
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

func TestNestedBlocksInThenCase(t *testing.T) {
	source := "package sample\nfunc f(value: Int) -> Int\n  switch value\n    case 1 then\n      switch value\n        case 1 => 10\n      end\n    end\n  end\nend\n"
	got := ParseFileAt("nested-blocks.mygo", source)
	if _, ok := resultOk[ast2.File, string](got); !ok {
		t.Fatalf("nested switch in case then failed: %v", got)
	}
}

func TestNestedIfAndWhileInThenCase(t *testing.T) {
	source := "package sample\nfunc f(value: Int) -> Int\n  switch value\n    case 1 then\n      if value == 1 then\n        while value == 1\n          1\n        end\n      end\n    end\n  end\nend\n"
	got := ParseFileAt("nested-if-while.mygo", source)
	if _, ok := resultOk[ast2.File, string](got); !ok {
		t.Fatalf("nested if/while in case then failed: %v", got)
	}
}

func TestNestedBlockMissingEndReportsError(t *testing.T) {
	source := "package sample\nfunc f(value: Int) -> Int\n  switch value\n    case 1 then\n      switch value\n        case 1 => 10\n    end\nend\n"
	got := ParseFileAt("missing-nested-end.mygo", source)
	if _, ok := resultErr[ast2.File, string](got); !ok {
		t.Fatalf("missing nested end unexpectedly parsed: %v", got)
	}
}

func TestMixedArrowAndThenNestedSwitchCases(t *testing.T) {
	source := "package sample\nfunc f(value: Int) -> Int\n  let result = switch value\n    case 1 => 1\n    case 2 then\n      switch value\n        case 2 => 2\n      end\n    end\n    case 3 => 3\n  end\n  result\nend\n"
	got := ParseFileAt("mixed-case-nested-switch.mygo", source)
	if _, ok := resultOk[ast2.File, string](got); !ok {
		t.Fatalf("mixed arrow/then nested switch failed: %v", got)
	}
}

func TestNestedTypePatternSwitchShape(t *testing.T) {
	source := "package sample\nfunc f(value: Int) -> String\n  switch value\n    case TFunc(methodArgs, methodRet) then\n      let first = methodArgs\n      let recvName = switch first\n        case TCon(name) => name\n        case TApp(ctor, _) then\n          switch ctor.value()\n            case TCon(name) => name\n            case _ => \"\"\n          end\n        end\n        case TQualifiedName(_, _) => \"qualified\"\n        case _ => \"\"\n      end\n      recvName\n    end\n  end\nend\n"
	got := ParseFileAt("nested-type-pattern.mygo", source)
	if _, ok := resultOk[ast2.File, string](got); !ok {
		t.Fatalf("nested type-pattern switch failed: %v", got)
	}
}

func TestIfThenElseExpressionInsideThenCase(t *testing.T) {
	source := "package sample\nfunc f(value: Int) -> Int\n  switch value\n    case 1 then\n      let result = if value == 1 then 0 else 1 end\n      result\n    end\n  end\nend\n"
	got := ParseFileAt("if-expression-in-case.mygo", source)
	if _, ok := resultOk[ast2.File, string](got); !ok {
		t.Fatalf("if then/else expression in case failed: %v", got)
	}
}

func TestIfMethodLenThenElseInsideThenCase(t *testing.T) {
	source := "package sample\nfunc f(methodArgs: Slice[Int]) -> Int\n  switch methodArgs\n    case 1 then\n      let result = if methodArgs.Len() == 0 then 0 else 1 end\n      result\n    end\n  end\nend\n"
	got := ParseFileAt("if-method-len-in-case.mygo", source)
	if _, ok := resultOk[ast2.File, string](got); !ok {
		t.Fatalf("if method len expression in case failed: %v", got)
	}
}

func TestIfElseAfterTFuncCase(t *testing.T) {
	source := "package sample\nfunc f(value: Int) -> Int\n  switch value\n    case TFunc(methodArgs, methodRet) then\n      if methodArgs.Len() == 0 then\n        0\n      else\n        1\n      end\n    end\n  end\nend\n"
	got := ParseFileAt("if-after-tfunc.mygo", source)
	if _, ok := resultOk[ast2.File, string](got); !ok {
		t.Fatalf("if else after TFunc case failed: %v", got)
	}
}

func TestStripReceiverPrefixShape(t *testing.T) {
	source := "package sample\nfunc f(methodArgs: Slice[Int]) -> Int\n  switch methodArgs\n    case TFunc(methodArgs, methodRet) then\n      if methodArgs.Len() == 0 then\n        None\n      else\n        let first = methodArgs.Get(0).UnwrapOr(ast2.MonoType.TUnit)\n        let recvName = switch first\n          case TCon(name) => name\n          case TApp(ctor, _) then\n            switch ctor.value()\n              case TCon(name) => name\n              case _ => \"\"\n            end\n          end\n          case TQualifiedName(_, _) => \"qualified\"\n          case _ => \"\"\n        end\n        0\n      end\n    end\n  end\nend\n"
	got := ParseFileAt("strip-prefix.mygo", source)
	if _, ok := resultOk[ast2.File, string](got); !ok {
		t.Fatalf("strip receiver prefix failed: %v", got)
	}
}

func TestThenCaseFollowedByArrowCase(t *testing.T) {
	source := "package sample\nfunc f(value: Int) -> Int\n  switch value\n    case TFunc(methodArgs, methodRet) then\n      if methodArgs.Len() == 0 then\n        None\n      else\n        let first = methodArgs.Get(0).UnwrapOr(ast2.MonoType.TUnit)\n        let recvName = switch first\n          case TCon(name) => name\n          case TApp(ctor, _) then\n            switch ctor.value()\n              case TCon(name) => name\n              case _ => \"\"\n            end\n          end\n          case TQualifiedName(_, _) => \"qualified\"\n          case _ => \"\"\n        end\n        1\n      end\n    end\n    case _ => 0\n  end\nend\n"
	got := ParseFileAt("then-followed-arrow.mygo", source)
	if _, ok := resultOk[ast2.File, string](got); !ok {
		t.Fatalf("then case followed by arrow case failed: %v", got)
	}
}

func TestStripReceiverExactSignatureIncrement(t *testing.T) {
	source := "package sample\nfunc stripReceiverArg(fieldType: ast2.MonoType, typeName: String, args: Slice[ast2.MonoType], subst: Subst) -> Option[Result[(ast2.MonoType, Subst), String]]\n  switch fieldType\n    case TFunc(methodArgs, methodRet) then\n      if methodArgs.Len() == 0 then\n        None\n      else\n        let first = methodArgs.Get(0).UnwrapOr(ast2.MonoType.TUnit)\n        let recvName = switch first\n          case TCon(name) => name\n          case TApp(ctor, _) then\n            switch ctor.value()\n              case TCon(name) => name\n              case _ => \"\"\n            end\n          end\n          case TQualifiedName(_, _) => typeName + \"\\0\"\n          case _ => \"\"\n        end\n        if recvName == typeName + \"\\0\" then\n          None\n        elsif recvName != typeName then\n          None\n        else\n          let receiverType = tCon(typeName, args)\n          let us = unify(first, receiverType, subst)\n          switch us\n            case Err(msg) => Some(Err(\"method receiver mismatch: \" + msg))\n            case Ok(s2) then\n              let droppedArgs = common2.SliceDrop(methodArgs, 1)\n              let newRet = applySubst(s2, methodRet.value())\n              let strippedBody = applySubstMonoList(s2, droppedArgs)\n              Some(Ok((ast2.MonoType.TFunc(strippedBody, Ref.new(newRet)), s2)))\n            end\n          end\n        end\n      end\n    end\n    case _ => None\n  end\nend\n"
	source = strings.ReplaceAll(source, "\\0", "\\u0000")
	got := ParseFileAt("strip-exact-signature.mygo", source)
	if _, ok := resultOk[ast2.File, string](got); !ok {
		t.Fatalf("exact signature increment failed: %v", got)
	}
}

func TestStripReceiverReturnAndUnifyShape(t *testing.T) {
	source := "package sample\nfunc f(first: ast2.MonoType, receiverType: ast2.MonoType, methodArgs: Slice[ast2.MonoType], methodRet: Ref[ast2.MonoType], subst: Subst) -> Option[Result[(ast2.MonoType, Subst), String]]\n  let us = unify(first, receiverType, subst)\n  switch us\n    case Err(msg) => Some(Err(\"method receiver mismatch: \" + msg))\n    case Ok(s2) then\n      let droppedArgs = common2.SliceDrop(methodArgs, 1)\n      let newRet = applySubst(subst, methodRet.value())\n      let strippedBody = applySubstMonoList(s2, droppedArgs)\n      Some(Ok((ast2.MonoType.TFunc(strippedBody, Ref.new(newRet)), s2)))\n    end\n  end\nend\n"
	got := ParseFileAt("strip-return-unify.mygo", source)
	if _, ok := resultOk[ast2.File, string](got); !ok {
		t.Fatalf("return/unify shape failed: %v", got)
	}
}

func TestLosslessNodeSpansCarryDeclarationAndStatementPaths(t *testing.T) {
	got := ParseFileLossless("paths.mygo", "package sample\nfunc f() -> Int\n  1\nend\n")
	parsed, ok := resultOk[LosslessFile, string](got)
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
	parsed, ok := resultOk[LosslessFile, string](got)
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
	parsed, ok := resultOk[LosslessFile, string](got)
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
	parsed, ok := resultOk[LosslessFile, string](got)
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
	parsed, ok := resultOk[LosslessFile, string](got)
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

// Comments stay line-protected (their interior spacing must survive verbatim),
// but a single-line `"…"` literal no longer locks its whole source line: that
// would freeze a row the formatter is free to reflow (e.g. a wrapping struct
// literal).  The renderer protects the literal's interior during line
// normalization instead, so the line stays reflowable.  Multi-line raw spans
// are covered by the dedicated span tests below.
func TestLosslessProtectedLinesIncludeCommentsNotInlineLiterals(t *testing.T) {
	got := ParseFileLossless("protected.mygo", "package sample\n# keep   comment\nfunc f() -> String\n  \"hello   world\"\nend\n")
	parsed, ok := resultOk[LosslessFile, string](got)
	if !ok {
		t.Fatalf("ParseFileLossless() failed: %v", got)
	}
	lines := LosslessProtectedLines(parsed.F0)
	foundComment, foundLiteral := false, false
	for _, line := range lines {
		foundComment = foundComment || line == 2
		foundLiteral = foundLiteral || line == 4
	}
	if !foundComment {
		t.Fatalf("comment line was not protected: %#v", lines)
	}
	if foundLiteral {
		t.Fatalf("single-line literal must not lock its whole line: %#v", lines)
	}
}

func TestLosslessProtectedLinesCoverTripleQuotedLiteralSpan(t *testing.T) {
	got := ParseFileLossless("triple-protected.mygo", "package sample\nfunc f() -> String\n  \"\"\"first   line\n  second   line\n  \"\"\"\nend\n")
	parsed, ok := resultOk[LosslessFile, string](got)
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
	parsed, ok := resultOk[LosslessFile, string](got)
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
	parsed, ok := resultOk[LosslessFile, string](got)
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
	parsed, ok := resultOk[LosslessFile, string](got)
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
	parsed, ok := resultOk[LosslessFile, string](got)
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
	parsed, ok := resultOk[ast2.File, string](got)
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
	file, ok := resultOk[ast2.File, string](parsed)
	if !ok {
		t.Fatalf("ParseFile failed: %v", parsed)
	}
	if !ast2.DeclIsTypeAlias(file.F0.Decls[0]) {
		t.Fatalf("decl[0] = %#v, want type alias declaration", file.F0.Decls[0])
	}
	if !ast2.DeclIsType(file.F0.Decls[1]) {
		t.Fatalf("decl[1] = %#v, want nominal type declaration", file.F0.Decls[1])
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
	parsed, ok := resultOk[ast2.File, string](got)
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
	if _, ok := resultOk[ast2.File, string](parsed); !ok {
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
	if _, ok := resultOk[ast2.File, string](parsed); !ok {
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
	if _, ok := resultOk[ast2.File, string](parsed); !ok {
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
	body := requireBlockExpr(t, ast2.DeclFuncBody(fn))
	got := requireExprStatement(t, ast2.ExprBlockStatements(body), 0)
	if !ast2.ExprIsFuncLit(got) {
		t.Fatalf("body is not a function literal: %#v", got)
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
	body := requireBlockExpr(t, ast2.DeclFuncBody(fn))
	statements := ast2.ExprBlockStatements(body)
	if len(statements) != 2 {
		t.Fatalf("body statement count = %d, want 2", len(statements))
	}
	if !ast2.StmtIsLetRec(statements[0]) {
		t.Fatalf("first statement is not letrec: %#v", statements[0])
	}
	bindings := ast2.StmtLetRecBindings(statements[0])
	if len(bindings) != 2 || bindings[0].Name != "even" || bindings[1].Name != "odd" {
		t.Fatalf("letrec bindings = %#v, want even/odd", bindings)
	}
}

func TestParseNestedTupleLetPattern(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func unpack()
  let (first, (_, last)) = (1, (2, 3))
end
`)
	body := requireBlockExpr(t, ast2.DeclFuncBody(fn))
	stmt := ast2.ExprBlockStatements(body)[0]
	if !ast2.StmtIsTupleLet(stmt) {
		t.Fatalf("first statement is not tuple let: %#v", stmt)
	}
	pattern := ast2.StmtTupleLetPattern(stmt)
	if !ast2.PatternIsTuple(pattern) || len(ast2.PatternTupleItems(pattern)) != 2 {
		t.Fatalf("tuple pattern = %#v, want two items", pattern)
	}
	items := ast2.PatternTupleItems(pattern)
	if !ast2.PatternIsBind(items[0]) || ast2.PatternBindName(items[0]) != "first" {
		t.Fatalf("first pattern = %#v, want BindPattern(first)", items[0])
	}
	if !ast2.PatternIsTuple(items[1]) || len(ast2.PatternTupleItems(items[1])) != 2 {
		t.Fatalf("nested pattern = %#v, want (_, last)", items[1])
	}
	nested := ast2.PatternTupleItems(items[1])
	if !ast2.PatternIsWildcard(nested[0]) {
		t.Fatalf("nested first pattern is not wildcard: %#v", nested[0])
	}
	if !ast2.PatternIsBind(nested[1]) || ast2.PatternBindName(nested[1]) != "last" {
		t.Fatalf("nested second pattern = %#v, want BindPattern(last)", nested[1])
	}
}

func TestParseVerbatimTripleQuotedAndRawStrings(t *testing.T) {
	fn := parseSingleFunc(t, "package sample\n\nfunc strings()\n  let triple = \"\"\"first\\\\n\nsecond\"\"\"\n  let raw = `first\\\\nsecond`\nend\n")
	body := requireBlockExpr(t, ast2.DeclFuncBody(fn))
	statements := ast2.ExprBlockStatements(body)
	for index, want := range []string{"first\\\\n\nsecond", "first\\\\nsecond"} {
		if !ast2.StmtIsLet(statements[index]) {
			t.Fatalf("statement[%d] is not let: %#v", index, statements[index])
		}
		literal := ast2.StmtLetBind(statements[index]).Value
		if !ast2.ExprIsString(literal) || ast2.ExprStringLiteral(literal) != want {
			t.Fatalf("statement[%d] literal = %#v, want %q", index, literal, want)
		}
	}
}

func TestParseUnicodeEscapesInOrdinaryStrings(t *testing.T) {
	fn := parseSingleFunc(t, "package sample\n\nfunc strings()\n  let ascii = \"\\u0041\"\n  let nonASCII = \"\\u4E2D\"\n  let nul = \"\\0\"\n  let raw = `\\u4E2D`\n  let multiline = \"\"\"\\u4E2D\"\"\"\nend\n")
	body := requireBlockExpr(t, ast2.DeclFuncBody(fn))
	statements := ast2.ExprBlockStatements(body)
	for index, want := range []string{"A", "中", "\x00", "\\u4E2D", "\\u4E2D"} {
		if !ast2.StmtIsLet(statements[index]) {
			t.Fatalf("statement[%d] is not let: %#v", index, statements[index])
		}
		literal := ast2.StmtLetBind(statements[index]).Value
		if !ast2.ExprIsString(literal) || ast2.ExprStringLiteral(literal) != want {
			t.Fatalf("statement[%d] literal = %#v, want %q", index, literal, want)
		}
	}
}

func TestRejectInvalidUnicodeEscapes(t *testing.T) {
	for name, literal := range map[string]string{
		"incomplete":      "\\u12",
		"non-hexadecimal": "\\u12G4",
		"surrogate":       "\\uD800",
	} {
		t.Run(name, func(t *testing.T) {
			source := "package sample\nfunc f() -> String\n  \"" + literal + "\"\nend\n"
			got := ParseFileAt("invalid-unicode-"+name+".mygo", source)
			if _, ok := resultErr[ast2.File, string](got); !ok {
				t.Fatalf("ParseFileAt() = %T, want parse error: %v", got, got)
			}
		})
	}
}

func TestParseSliceLiteralWithTrailingCommaAndTypeAs(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func values()
  [1, 2,] as Slice[Int]
end
`)
	body := requireBlockExpr(t, ast2.DeclFuncBody(fn))
	castExpr := requireExprStatement(t, ast2.ExprBlockStatements(body), 0)
	if !ast2.ExprIsTypeAs(castExpr) {
		t.Fatalf("body is not a type-as expression: %#v", castExpr)
	}
	cast := ast2.ExprTypeAs(castExpr)
	if !ast2.ExprIsSliceLit(cast.Value) || len(ast2.ExprSliceItems(cast.Value)) != 2 {
		t.Fatalf("cast value = %#v, want two-item slice literal", cast.Value)
	}
}

func TestParseNumericLiteralSyntax(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func values()
  [18_446_744u64, 3.14f32, 0xff, 0XFFi8, 0o777u, 0B1010u64]
end
`)
	body := requireBlockExpr(t, ast2.DeclFuncBody(fn))
	literalExpr := requireExprStatement(t, ast2.ExprBlockStatements(body), 0)
	if !ast2.ExprIsSliceLit(literalExpr) {
		t.Fatalf("body is not a slice literal: %#v", literalExpr)
	}
	items := ast2.ExprSliceItems(literalExpr)
	want := []string{"18446744u64", "3.14f32", "0xff", "0XFFi8", "0o777u", "0B1010u64"}
	if len(items) != len(want) {
		t.Fatalf("literal count = %d, want %d", len(items), len(want))
	}
	for i, expected := range want {
		if !ast2.ExprIsNumber(items[i]) || ast2.ExprNumberLiteral(items[i]) != expected {
			t.Fatalf("literal[%d] = %#v, want number %q", i, items[i], expected)
		}
	}
}

func TestRejectInvalidNumericLiteralSyntax(t *testing.T) {
	for _, literal := range []string{"1.2.3", "1oops", "0x", "0b102", "1_"} {
		t.Run(literal, func(t *testing.T) {
			got := ParseFile("package sample\n\nfunc bad()\n  " + literal + "\nend\n")
			if _, ok := resultErr[ast2.File, string](got); !ok {
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
	runeExpr := requireExprStatement(t, ast2.ExprBlockStatements(requireBlockExpr(t, ast2.DeclFuncBody(fn))), 0)
	if !ast2.ExprIsRune(runeExpr) {
		t.Fatalf("body is not a rune expression: %#v", runeExpr)
	}
	if ast2.ExprRuneLiteral(runeExpr) != "\n" {
		t.Fatalf("rune value = %q, want newline", ast2.ExprRuneLiteral(runeExpr))
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
	body := requireBlockExpr(t, ast2.DeclFuncBody(fn))
	switchExpr := requireSwitchExpr(t, requireExprStatement(t, ast2.ExprBlockStatements(body), 0))
	for index, want := range []string{"\"", "\n", "\x00"} {
		pattern := switchExpr.Cases[index].Pattern
		if !ast2.PatternIsLiteral(pattern) || ast2.PatternLiteralKind(pattern) != "rune" || ast2.PatternLiteralValue(pattern) != want {
			t.Fatalf("case %d pattern = %#v, want rune %q", index, pattern, want)
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
	parsed, ok := resultOk[ast2.File, string](got)
	if !ok {
		t.Fatalf("ParseFile failed: %v", got)
	}
	fn := requireFunctionDecl(t, parsed.F0, 1)
	body := requireBlockExpr(t, ast2.DeclFuncBody(fn))
	sw := requireSwitchExpr(t, requireExprStatement(t, ast2.ExprBlockStatements(body), 0))
	if len(sw.Cases) != 3 {
		t.Fatalf("case count = %d, want 3", len(sw.Cases))
	}
	if !ast2.PatternIsVariant(sw.Cases[0].Pattern) {
		t.Fatalf("first pattern is not a variant: %#v", sw.Cases[0].Pattern)
	}
	variant := ast2.PatternVariant(sw.Cases[0].Pattern)
	if variant.Name != "Some" || len(variant.Items) != 1 || !ast2.PatternIsBind(variant.Items[0]) || ast2.PatternBindName(variant.Items[0]) != "item" {
		t.Fatalf("first pattern = %#v, want Some(item)", sw.Cases[0].Pattern)
	}
	if !ast2.PatternIsWildcard(sw.Cases[2].Pattern) {
		t.Fatalf("third pattern is not wildcard: %#v", sw.Cases[2].Pattern)
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
	file, ok := resultOk[ast2.File, string](parsed)
	if !ok {
		t.Fatalf("ParseFile failed: %v", parsed)
	}
	if !ast2.DeclIsEnum(file.F0.Decls[0]) {
		t.Fatalf("decl[0] is not an enum: %#v", file.F0.Decls[0])
	}
	circle := ast2.DeclEnumVariants(file.F0.Decls[0])[0]
	if !circle.Named || len(circle.Names) != 1 || circle.Names[0] != "radius" {
		t.Fatalf("Circle variant = %#v, want named radius field", circle)
	}
	fn := requireFunctionDecl(t, file.F0, 1)
	body := requireBlockExpr(t, ast2.DeclFuncBody(fn))
	sw := requireSwitchExpr(t, requireExprStatement(t, ast2.ExprBlockStatements(body), 0))
	if !ast2.PatternIsStructVariant(sw.Cases[0].Pattern) {
		t.Fatalf("pattern is not a struct variant: %#v", sw.Cases[0].Pattern)
	}
	pat := ast2.PatternStructVariant(sw.Cases[0].Pattern)
	if pat.Name != "Circle" || len(pat.Fields) != 1 || pat.Fields[0].Field != "radius" || pat.Fields[0].Bind != "r" {
		t.Fatalf("pattern = %#v, want Circle { radius: r }", sw.Cases[0].Pattern)
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
	parsed, ok := resultOk[ast2.File, string](got)
	if !ok {
		t.Fatalf("ParseFile failed: %v", got)
	}
	fn := requireFunctionDecl(t, parsed.F0, 1)
	body := requireBlockExpr(t, ast2.DeclFuncBody(fn))
	sw := requireSwitchExpr(t, requireExprStatement(t, ast2.ExprBlockStatements(body), 0))
	if !ast2.ExprIsCall(sw.Target) {
		t.Fatalf("switch target is not a call: %#v", sw.Target)
	}
	call := ast2.ExprCall(sw.Target)
	if !ast2.ExprIsField(call.Callee) || ast2.ExprField(call.Callee).Name != "map" {
		t.Fatalf("switch call callee = %#v, want value.map", call.Callee)
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
	parsed, ok := resultOk[ast2.File, string](got)
	if !ok {
		t.Fatalf("ParseFile failed: %v", got)
	}
	fn := requireFunctionDecl(t, parsed.F0, 1)
	body := requireBlockExpr(t, ast2.DeclFuncBody(fn))
	sw := requireSwitchExpr(t, requireExprStatement(t, ast2.ExprBlockStatements(body), 0))
	if len(sw.Cases) != 2 {
		t.Fatalf("case count = %d, want 2", len(sw.Cases))
	}
	if !ast2.ExprIsIdent(sw.Cases[0].Body) {
		t.Fatalf("block case body is not an identifier: %#v", sw.Cases[0].Body)
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
	ok, yes := resultOk[ast2.File, string](got)
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

	body := requireBlockExpr(t, ast2.DeclFuncBody(fn))
	statements := ast2.ExprBlockStatements(body)
	if len(statements) != 1 {
		t.Fatalf("body expr count = %d, want 1", len(statements))
	}
	rootExpr := requireExprStatement(t, statements, 0)
	if !ast2.ExprIsBinary(rootExpr) {
		t.Fatalf("root expression is not binary: %#v", rootExpr)
	}
	root := ast2.ExprBinary(rootExpr)
	if root.Operator != "-" {
		t.Fatalf("root op = %q, want -", root.Operator)
	}
	if !ast2.ExprIsBinary(root.Left) {
		t.Fatalf("left expression is not binary: %#v", root.Left)
	}
	left := ast2.ExprBinary(root.Left)
	if left.Operator != "+" {
		t.Fatalf("left op = %q, want +", left.Operator)
	}
	if !ast2.ExprIsBinary(left.Right) {
		t.Fatalf("nested expression is not binary: %#v", left.Right)
	}
	if ast2.ExprBinary(left.Right).Operator != "*" {
		t.Fatalf("nested op = %q, want *", ast2.ExprBinary(left.Right).Operator)
	}
}

func TestParseLogicalAndUnaryExpressions(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func ok(a: Bool, b: Bool, c: Bool, n: Int) -> Bool
  !a || b && -n > 0
end
`)

	body := requireBlockExpr(t, ast2.DeclFuncBody(fn))
	rootExpr := requireExprStatement(t, ast2.ExprBlockStatements(body), 0)
	if !ast2.ExprIsBinary(rootExpr) {
		t.Fatalf("root expression is not binary: %#v", rootExpr)
	}
	root := ast2.ExprBinary(rootExpr)
	if root.Operator != "||" {
		t.Fatalf("root op = %q, want ||", root.Operator)
	}
	if !ast2.ExprIsUnary(root.Left) {
		t.Fatalf("left expression is not unary: %#v", root.Left)
	}
	if ast2.ExprUnary(root.Left).Operator != "!" {
		t.Fatalf("left unary op = %q, want !", ast2.ExprUnary(root.Left).Operator)
	}
	if !ast2.ExprIsBinary(root.Right) {
		t.Fatalf("right expression is not binary: %#v", root.Right)
	}
	right := ast2.ExprBinary(root.Right)
	if right.Operator != "&&" {
		t.Fatalf("right op = %q, want &&", right.Operator)
	}
	if !ast2.ExprIsBinary(right.Right) {
		t.Fatalf("comparison expression is not binary: %#v", right.Right)
	}
	cmp := ast2.ExprBinary(right.Right)
	if cmp.Operator != ">" {
		t.Fatalf("comparison op = %q, want >", cmp.Operator)
	}
	if !ast2.ExprIsUnary(cmp.Left) {
		t.Fatalf("comparison left is not unary: %#v", cmp.Left)
	}
	if ast2.ExprUnary(cmp.Left).Operator != "-" {
		t.Fatalf("comparison left unary op = %q, want -", ast2.ExprUnary(cmp.Left).Operator)
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

	body := requireBlockExpr(t, ast2.DeclFuncBody(fn))
	rootExpr := requireExprStatement(t, ast2.ExprBlockStatements(body), 0)
	if !ast2.ExprIsIf(rootExpr) {
		t.Fatalf("root expression is not if: %#v", rootExpr)
	}
	root := ast2.ExprIf(rootExpr)
	if !ast2.ExprIsNumber(root.Then) || ast2.ExprNumberLiteral(root.Then) != "1" {
		t.Fatalf("then value = %#v, want 1", root.Then)
	}
	if !ast2.ExprIsIf(root.Else) {
		t.Fatalf("elsif expression is not if: %#v", root.Else)
	}
	nested := ast2.ExprIf(root.Else)
	if !ast2.ExprIsNumber(nested.Then) || ast2.ExprNumberLiteral(nested.Then) != "2" {
		t.Fatalf("elsif value = %#v, want 2", nested.Then)
	}
	if !ast2.ExprIsNumber(nested.Else) || ast2.ExprNumberLiteral(nested.Else) != "3" {
		t.Fatalf("else value = %#v, want 3", nested.Else)
	}
}

func TestParseVarDeclaration(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo() -> Int
  var x: Int = 42
  x
end
`)

	body := requireBlockExpr(t, ast2.DeclFuncBody(fn))
	statements := ast2.ExprBlockStatements(body)
	if len(statements) != 2 {
		t.Fatalf("body expr count = %d, want 2", len(statements))
	}
	if !ast2.StmtIsVar(statements[0]) {
		t.Fatalf("first stmt is not var: %#v", statements[0])
	}
	if ast2.StmtVarBind(statements[0]).Name != "x" {
		t.Fatalf("var name = %q, want x", ast2.StmtVarBind(statements[0]).Name)
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

	body := requireBlockExpr(t, ast2.DeclFuncBody(fn))
	statements := ast2.ExprBlockStatements(body)
	if len(statements) != 2 {
		t.Fatalf("body expr count = %d, want 2", len(statements))
	}
	if !ast2.StmtIsWhile(statements[0]) {
		t.Fatalf("first stmt is not while: %#v", statements[0])
	}
	condition := ast2.StmtWhileCondition(statements[0])
	if !ast2.ExprIsBinary(condition) || ast2.ExprBinary(condition).Operator != ">" {
		t.Fatalf("while condition is not > binary: %#v", condition)
	}
}

func TestParseReturnStatement(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo(n: Int) -> Int
  return n
end
`)

	body := requireBlockExpr(t, ast2.DeclFuncBody(fn))
	statement := ast2.ExprBlockStatements(body)[0]
	if !ast2.StmtIsReturnWith(statement) {
		t.Fatalf("stmt is not return-with: %#v", statement)
	}
	value := ast2.StmtReturnValue(statement)
	if !ast2.ExprIsIdent(value) || ast2.ExprIdentName(value) != "n" {
		t.Fatalf("return value is %#v, want n", value)
	}
}

func TestParseBareReturnStatement(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo(n: Int)
  return
end
`)

	body := requireBlockExpr(t, ast2.DeclFuncBody(fn))
	if !ast2.StmtIsReturn(ast2.ExprBlockStatements(body)[0]) {
		t.Fatalf("stmt is not bare return: %#v", ast2.ExprBlockStatements(body)[0])
	}
}

func TestParseInlineGoExpression(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo(n: Int) -> Int
  go[String]{code: "return strconv.Itoa(n)"}
end
`)

	body := requireBlockExpr(t, ast2.DeclFuncBody(fn))
	first := requireExprStatement(t, ast2.ExprBlockStatements(body), 0)
	if !ast2.ExprIsInlineGo(first) {
		t.Fatalf("expr is not inline Go: %#v", first)
	}
}

func TestParseInlineGoOperands(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo(n: Int) -> String
  go[String]{code: "{T}({v})" in v = n type T = String}
end
`)
	body := requireBlockExpr(t, ast2.DeclFuncBody(fn))
	expr := requireExprStatement(t, ast2.ExprBlockStatements(body), 0)
	operands := ast2.ExprInlineGoOperands(expr)
	if len(operands) != 1 || operands[0].Name != "v" {
		t.Fatalf("value operands = %#v, want one v operand", operands)
	}
	typeOperands := ast2.ExprInlineGoTypeOperands(expr)
	if len(typeOperands) != 1 || typeOperands[0].Name != "T" {
		t.Fatalf("type operands = %#v, want one T operand", typeOperands)
	}
}

func parseSingleFunc(t *testing.T, src string) ast2.Decl {
	t.Helper()

	got := ParseFile(src)
	ok, yes := resultOk[ast2.File, string](got)
	if !yes {
		t.Fatalf("ParseFile failed: %v", got)
	}
	if len(ok.F0.Decls) != 1 {
		t.Fatalf("decl count = %d, want 1", len(ok.F0.Decls))
	}
	if !ast2.DeclIsFunc(ok.F0.Decls[0]) {
		t.Fatalf("decl type is not function: %#v", ok.F0.Decls[0])
	}
	return ok.F0.Decls[0]
}

func TestParseAssignSimpleVar(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo() -> Int
  var x: Int = 42
  x = 1
  x
end
`)

	body := requireBlockExpr(t, ast2.DeclFuncBody(fn))
	statements := ast2.ExprBlockStatements(body)
	if len(statements) != 3 {
		t.Fatalf("body expr count = %d, want 3", len(statements))
	}
	if !ast2.StmtIsAssign(statements[1]) {
		t.Fatalf("second stmt is not assignment: %#v", statements[1])
	}
	lhs := ast2.StmtAssignTarget(statements[1])
	rhs := ast2.StmtAssignValue(statements[1])
	if !ast2.ExprIsIdent(lhs) || ast2.ExprIdentName(lhs) != "x" {
		t.Fatalf("assign lhs = %#v, want x", lhs)
	}
	if !ast2.ExprIsNumber(rhs) || ast2.ExprNumberLiteral(rhs) != "1" {
		t.Fatalf("assign rhs = %#v, want 1", rhs)
	}
}

func TestParseAssignFieldSimple(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo()
  var p: Point = Point { x: 1, y: 2 }
  p.x = 99
end
`)

	body := requireBlockExpr(t, ast2.DeclFuncBody(fn))
	statements := ast2.ExprBlockStatements(body)
	if len(statements) < 2 || !ast2.StmtIsAssign(statements[1]) {
		t.Fatalf("second stmt is not assignment: %#v", statements)
	}
	lhs := ast2.StmtAssignTarget(statements[1])
	if !ast2.ExprIsField(lhs) {
		t.Fatalf("assign lhs is not field: %#v", lhs)
	}
	field := ast2.ExprField(lhs)
	if field.Name != "x" {
		t.Fatalf("assign field name = %q, want x", field.Name)
	}
	if !ast2.ExprIsIdent(field.Receiver) || ast2.ExprIdentName(field.Receiver) != "p" {
		t.Fatalf("assign field obj = %#v, want p", field.Receiver)
	}
	rhs := ast2.StmtAssignValue(statements[1])
	if !ast2.ExprIsNumber(rhs) || ast2.ExprNumberLiteral(rhs) != "99" {
		t.Fatalf("assign rhs = %#v, want 99", rhs)
	}
}

func TestParseAssignFieldChain(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo()
  cfg.settings.theme = "dark"
end
`)

	body := requireBlockExpr(t, ast2.DeclFuncBody(fn))
	statements := ast2.ExprBlockStatements(body)
	if !ast2.StmtIsAssign(statements[0]) {
		t.Fatalf("first stmt is not assignment: %#v", statements[0])
	}
	// lhs = cfg.settings.theme
	lhs := ast2.StmtAssignTarget(statements[0])
	if !ast2.ExprIsField(lhs) {
		t.Fatalf("assign lhs is not field: %#v", lhs)
	}
	themeField := ast2.ExprField(lhs)
	if themeField.Name != "theme" {
		t.Fatalf("outer field = %q, want theme", themeField.Name)
	}
	// cfg.settings
	if !ast2.ExprIsField(themeField.Receiver) {
		t.Fatalf("inner expression is not field: %#v", themeField.Receiver)
	}
	settingsField := ast2.ExprField(themeField.Receiver)
	if settingsField.Name != "settings" {
		t.Fatalf("inner field = %q, want settings", settingsField.Name)
	}
	if !ast2.ExprIsIdent(settingsField.Receiver) || ast2.ExprIdentName(settingsField.Receiver) != "cfg" {
		t.Fatalf("base ident = %#v, want cfg", settingsField.Receiver)
	}
	rhs := ast2.StmtAssignValue(statements[0])
	if !ast2.ExprIsString(rhs) || ast2.ExprStringLiteral(rhs) != "dark" {
		t.Fatalf("assign rhs = %#v, want dark", rhs)
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

	body := requireBlockExpr(t, ast2.DeclFuncBody(fn))
	statements := ast2.ExprBlockStatements(body)
	if len(statements) != 4 {
		t.Fatalf("body expr count = %d, want 4", len(statements))
	}
	if !ast2.StmtIsAssign(statements[2]) {
		t.Fatalf("third stmt is not assignment: %#v", statements[2])
	}
}

func TestExpressionsCarrySourcePositions(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo() -> Int
  1 + 2
end
`)

	body := ast2.DeclFuncBody(fn)
	if body.Pos.Line != 4 || body.Pos.Column != 3 {
		t.Fatalf("body position = %d:%d, want 4:3", body.Pos.Line, body.Pos.Column)
	}
	root := requireExprStatement(t, ast2.ExprBlockStatements(requireBlockExpr(t, body)), 0)
	if root.Pos.Line != 4 || root.Pos.Column != 3 {
		t.Fatalf("root position = %d:%d, want 4:3", root.Pos.Line, root.Pos.Column)
	}
	if !ast2.ExprIsBinary(root) {
		t.Fatalf("root expression is not binary: %#v", root)
	}
	binary := ast2.ExprBinary(root)
	if binary.Left.Pos.Line != 4 || binary.Left.Pos.Column != 3 {
		t.Fatalf("left operand position = %d:%d, want 4:3", binary.Left.Pos.Line, binary.Left.Pos.Column)
	}
	if binary.Right.Pos.Line != 4 || binary.Right.Pos.Column != 7 {
		t.Fatalf("right operand position = %d:%d, want 4:7", binary.Right.Pos.Line, binary.Right.Pos.Column)
	}
}

func TestLayoutEventsKeepSwitchBlockStructure(t *testing.T) {
	source := "package sample\nfunc g(x: Int) -> Int\n  switch x\n    case 1 => 10\n    case _ then\n      0\n    end\n  end\nend\n"
	got := ParseFileLossless("case-then.mygo", source)
	parsed, ok := resultOk[LosslessFile, string](got)
	if !ok {
		t.Fatalf("ParseFileLossless() failed: %v", got)
	}
	var switchEvent *LayoutEvent
	var thenCaseEvent *LayoutEvent
	for index := range parsed.F0.LayoutEvents {
		event := &parsed.F0.LayoutEvents[index]
		if event.Kind == "enter:switch" {
			switchEvent = event
		}
		if event.Kind == "enter:case" && event.BodyLine > event.HeaderLine {
			thenCaseEvent = event
		}
	}
	if switchEvent == nil {
		t.Fatalf("missing switch event: %#v", parsed.F0.LayoutEvents)
	}
	if !switchEvent.AffectsIndent || switchEvent.ExitLine != 8 {
		t.Fatalf("switch event lost its block structure: %#v", *switchEvent)
	}
	if thenCaseEvent == nil || !thenCaseEvent.AffectsIndent || thenCaseEvent.ExitLine != 7 {
		t.Fatalf("then-style case body did not anchor its own end: %#v", parsed.F0.LayoutEvents)
	}
}

func TestLayoutEventsKeepWideIfBranchExpansion(t *testing.T) {
	source := "package sample\nfunc f() -> Int\n  if veryLongConditionNameThatKeepsGoingAndGoingAndGoing => veryLongBranchExpressionNameThatKeepsGoingAndGoingAndGoing else veryLongBranchExpressionNameThatKeepsGoingAndGoingAndGoing\nend\n"
	got := ParseFileLossless("wide-if.mygo", source)
	parsed, ok := resultOk[LosslessFile, string](got)
	if !ok {
		t.Fatalf("ParseFileLossless() failed: %v", got)
	}
	var thenEvent, elseEvent *LayoutEvent
	for index := range parsed.F0.LayoutEvents {
		event := &parsed.F0.LayoutEvents[index]
		if event.Kind == "enter:if-then" {
			thenEvent = event
		}
		if event.Kind == "enter:if-else" {
			elseEvent = event
		}
	}
	if thenEvent == nil || elseEvent == nil {
		t.Fatalf("wide single-line conditional lost branch events: %#v", parsed.F0.LayoutEvents)
	}
	if thenEvent.Anchor.Start.Line != 3 || thenEvent.Anchor.End.Column > thenEvent.Span.Start.Column || !thenEvent.ExpandsAfter {
		t.Fatalf("then branch anchor/expansion = %#v", *thenEvent)
	}
	if elseEvent.Anchor.Start.Line != 3 || elseEvent.Anchor.End.Column > elseEvent.Span.Start.Column || !elseEvent.ExpandsAfter {
		t.Fatalf("else branch anchor/expansion = %#v", *elseEvent)
	}
}
