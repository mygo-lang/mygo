package mygo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompileDirSupportsCollectionLiterals(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  func demo() -> Int
    let numbers: Slice[Int] = [1, 2, 3]
    let m: Map[String, String] = {"a": "1", "b": "2"}
    let s: Set[String] = {"x", "y"}
    let empty_s: Slice[Int] = []
  42
  end

  func main() -> Unit
    demo()
  end
`)

	out, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, out)
	for _, want := range []string{
		"[]int{1, 2, 3}",
		`map[string]string{"a": "1", "b": "2"}`,
		`map[string]struct{}{"x":{}, "y":{}}`,
		"[]int{}",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirSupportsLetVarAndDiscard(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  import fmt "go:fmt"

  func add(x: Int, y: Int) -> Int
    x + y
  end

  func demo() -> Int
    let msg: String = "abc"
    let _ = fmt.Println(msg)
    var n: Int = add(40, 2)
    n = n + 1
    n
  end

  func main() -> Unit
    demo()
  end
`)

	out, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	if out != filepath.Join(dir, "zz_mygo.gen.go") {
		t.Fatalf("CompileDir() output path = %q, want %q", out, filepath.Join(dir, "zz_mygo.gen.go"))
	}
	got := readFile(t, out)
	for _, want := range []string{
		"func demo() int {",
		"var msg_1 string = \"abc\"",
		"fmt.Println(msg_1)",
		"var n_2 int = add(40, 2)",
		"n_2 = (n_2 + 1)",
		"return n_2",
		"func main() {",
		"demo()",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirSupportsWhileLoops(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  import fmt "go:fmt"

  func demo() -> Int
    var n: Int = 0
    while n < 3
      let _ = fmt.Println(n)
      n = n + 1
    end
    n
  end
`)

	out, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, out)
	for _, want := range []string{
		"for (n_1 < 3) {",
		"fmt.Println(n_1)",
		"n_1 = (n_1 + 1)",
		"return n_1",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirSupportsTupleReturnValues(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main

  func pair() -> (Int, String)
    (1, "a")
  end

  func demo() -> String
    let p = pair()
    p.F1
  end
`)

	out, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, out)
	for _, want := range []string{
		"func pair() (int, string) {",
		"return 1, \"a\"",
		"func demo() string {",
		":= pair()",
		".F1",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirSupportsTupleParameters(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main

  func swap(pair: (Int, String)) -> String
    pair.F1
  end

  func demo() -> String
    let p = (1, "a")
    swap(p)
  end
`)

	out, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, out)
	for _, want := range []string{
		"func swap(pair struct {",
		"F0 int",
		"F1 string",
		"func demo() string {",
		"p_",
		"swap(p_",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirSupportsTupleDestructuringLet(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main

  func pair() -> (Int, String)
    (1, "a")
  end

  func demo() -> String
    let (a, b) = pair()
    b
  end
`)

	out, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, out)
	for _, want := range []string{
		"__tuple_",
		"a_",
		"b_",
		"return b_",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirSupportsNestedTupleDestructuringLet(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main

  func nested() -> (Int, (String, Bool))
    (1, ("a", true))
  end

  func demo() -> String
    let (a, (b, c)) = nested()
    b
  end
`)

	out, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, out)
	for _, want := range []string{
		"func nested() (int, struct {",
		"F0 string",
		"F1 bool",
		"__tuple_",
		"b_",
		"c_",
		"return b_",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirSupportsIgnoredTupleBindings(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main

  func pair() -> (Int, String)
    (1, "a")
  end

  func nested() -> (Int, (String, Bool))
    (1, ("a", true))
  end

  func demo() -> String
    let (_, b) = pair()
    let (x, (_, y)) = nested()
    b
  end
`)

	out, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, out)
	for _, want := range []string{
		"b_",
		"y_",
		"return b_",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
	if strings.Contains(got, "let _") {
		t.Fatalf("generated Go should not preserve source syntax\n--- got ---\n%s", got)
	}
}

func TestCompileDirKeepsTupleValueOnSingleLetBinding(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main

  func pair() -> (Int, String)
    (1, "a")
  end

  func demo() -> Int
    let c = pair()
    c.F0
  end
`)

	out, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, out)
	for _, want := range []string{
		"pair()",
		".F0",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirAllowsLetShadowingAndInference(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  func demo() -> Int
    let x = 1
    let x = 2
    x
  end
`)

	out, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, out)
	if !strings.Contains(got, "return x_2") {
		t.Fatalf("generated Go missing shadowed return\n--- got ---\n%s", got)
	}
	if !strings.Contains(got, "x_1 := 1") || !strings.Contains(got, "x_2 := 2") {
		t.Fatalf("generated Go missing shadowed bindings\n--- got ---\n%s", got)
	}
}

func TestCompileDirRejectsAssignmentToLet(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  func bad() -> Int
    let x: Int = 1
    x = 2
    x
  end
`)

	_, err := CompileDir(dir)
	if err == nil {
		t.Fatal("CompileDir() error = nil, want immutable binding failure")
	}
	if !strings.Contains(err.Error(), "immutable binding") {
		t.Fatalf("CompileDir() error = %v, want immutable binding failure", err)
	}
}

func TestCompileDirSupportsStructLiterals(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  struct ABC
    aaa: Int64
  end

  struct Box[A]
    value: A
  end

  func demo() -> Int64
    let item = ABC {
      aaa: 123
    }
    let boxed = Box {
      value: item.aaa
    }
    boxed.value
  end
`)

	out, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, out)
	for _, want := range []string{
		"type ABC struct {",
		"Aaa int64",
		"type Box[A any] struct {",
		"Box[int64]{Value: item_1.Aaa}",
		"return boxed_2.Value",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirSupportsRefAndResultTypes(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  enum Result[A, E]
    Ok(A)
    Err(E)
  end

  struct Node
    value: Int
  end

  struct Holder
    item: Option[Ref[Node]]
  end

  func use_ref(node: Ref[Node]) -> Int
    node.value
  end

  func describe(ok: Bool) -> Result[String, String]
    if ok then Ok("yes") else Err("no")
  end
`)

	out, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, out)
	for _, want := range []string{
		"type Result[A any, E any] interface{ isResult() }",
		"type Holder struct {",
		"Item Option[*Node]",
		"func use_ref(node *Node) int {",
		"func describe(ok bool) Result[string, string] {",
		"return Ok[string, string](\"yes\")",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirSupportsOptionOfRefTypes(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  enum Option[A]
    Some(A)
    None()
  end

  struct Node
    value: Int
  end

  struct Holder
    item: Option[Ref[Node]]
  end

  func maybe_node(ok: Bool, node: Ref[Node]) -> Option[Ref[Node]]
    if ok then Some(node) else None()
  end
`)

	out, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, out)
	for _, want := range []string{
		"type Holder struct {",
		"Item Option[*Node]",
		"func maybe_node(ok bool, node *Node) Option[*Node] {",
		"return Some[*Node](node)",
		"return None[*Node]()",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirSupportsRefNew(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  enum Option[A]
    Some(A)
    None()
  end

  struct Node
    value: Int
  end

  func maybe_node(ok: Bool, node: Node) -> Option[Ref[Node]]
    if ok then Some(Ref.new(node)) else None()
  end
`)

	out, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, out)
	for _, want := range []string{
		"func maybe_node(ok bool, node Node) Option[*Node] {",
		"return Some[*Node](&node)",
		"return None[*Node]()",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirSupportsDynamicTypeclassDispatch(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  import fmt "go:fmt"

  interface Show[A]
    func show(value: A) -> String
  end

  impl Int64: Show[Int64]
    func show(value: Int64) -> String
      fmt.Sprint(value)
    end
  end

  func demo() -> String
    show(42)
  end
`)

	out, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, out)
	for _, want := range []string{
		"var Show_showDispatchRegistry = map[string]func(any) string{}",
		"func Show_show(value any) string {",
		"Show_showDispatchRegistry[\"int64\"] = func(value any) string {",
		"return show_int64(valueTyped)",
		"return Show_show(42)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirWrapsGoErrorReturnsIntoResult(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  import os "go:os"

  enum Result[A, E]
    Ok(A)
    Err(E)
  end

  func demo() -> Result[any, String]
    os.Open("/tmp/does-not-matter")
  end
`)

	out, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, out)
	for _, want := range []string{
		"func demo() Result[any, string] {",
		"value, err := os.Open(\"/tmp/does-not-matter\")",
		"return Err[any, string](err.Error())",
		"return Ok[any, string](value)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirRejectsGoSelectorArgMismatch(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  import os "go:os"

  func demo() -> Bool
    os.Open()
  end
`)

	_, err := CompileDir(dir)
	if err == nil {
		t.Fatal("CompileDir() error = nil, want argument mismatch")
	}
	if !strings.Contains(err.Error(), "expected 1 args") {
		t.Fatalf("CompileDir() error = %v, want argument mismatch", err)
	}
}

func TestCompileDirSupportsGoValueAndPointerMethods(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  import bytes "go:bytes"
  import time "go:time"

  func demo() -> Int
    let buf = bytes.NewBufferString("hi")
    let year = time.Date(2024, 1, 2, 3, 4, 5, 6, time.UTC).Year()
    buf.String()
    year
  end
`)

	out, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, out)
	for _, want := range []string{
		"buf_1 := bytes.NewBufferString(\"hi\")",
		"year_2 := time.Date(2024, 1, 2, 3, 4, 5, 6, time.UTC).Year()",
		"buf_1.String()",
		"return year_2",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirPreservesRefInGoBoundaryResults(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  import os "go:os"

  enum Result[A, E]
    Ok(A)
    Err(E)
  end

  func open_file() -> Result[Ref[Any], String]
    os.Open("/tmp/does-not-matter")
  end
`)

	out, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, out)
	for _, want := range []string{
		"func open_file() Result[*any, string] {",
		"value, err := os.Open(\"/tmp/does-not-matter\")",
		"return Err[*any, string](err.Error())",
		"return Ok[*any, string](value)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirSupportsResultOfRefTypes(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  enum Result[A, E]
    Ok(A)
    Err(E)
  end

  struct Node
    value: Int
  end

  func lookup(ok: Bool, node: Ref[Node]) -> Result[Ref[Node], String]
    if ok then Ok(node) else Err("missing")
  end
`)

	out, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, out)
	for _, want := range []string{
		"func lookup(ok bool, node *Node) Result[*Node, string] {",
		"return Ok[*Node, string](node)",
		"return Err[*Node, string](\"missing\")",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirSupportsIfExpressionForms(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  func compact(ok: Bool) -> Int
    if ok then 1 else 2
  end

  func block(ok: Bool) -> Int
    if ok
      1
    else
      2
    end
  end
`)

	out, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, out)
	for _, want := range []string{
		"func compact(ok bool) int {",
		"if ok {",
		"return 1",
		"} else {",
		"return 2",
		"func block(ok bool) int {",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirSupportsMultiParamTypeclassDispatch(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  interface Eq[A]
    func equals(left: A, right: A) -> Bool
  end

  impl Int64: Eq[Int64]
    func equals(left: Int64, right: Int64) -> Bool
      left == right
    end
  end

  func demo() -> Bool
    equals(1, 2)
  end
`)

	out, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, out)
	for _, want := range []string{
		"var Eq_equalsDispatchRegistry = map[string]func(any, any) bool{}",
		"Eq_equalsDispatchRegistry[\"int64|int64\"] = func(left any, right any) bool {",
		"key := typeKeyFromType(reflect.TypeOf(left).String()) + \"|\" + typeKeyFromType(reflect.TypeOf(right).String())",
		"return Eq_equals(1, 2)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirSupportsArithmeticAndLogicOperators(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  func demo(a: Int64, b: Int64, ok: Bool) -> Bool
    ok && (a + b > 10) || (a - b <= 2)
  end
`)

	out, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, out)
	for _, want := range []string{
		"((ok && ((a + b) > 10)) || ((a - b) <= 2))",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirRejectsRelationWithoutEq(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  struct Node
    value: Int64
  end

  func demo(a: Node, b: Node) -> Bool
    a == b
  end
`)

	if _, err := CompileDir(dir); err == nil {
		t.Fatalf("CompileDir() error = nil, want relation operator to require Eq")
	}
}

func TestCompileDirSeparatesSameNamedMethodsByInterface(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  interface Show[A]
    func show(value: A) -> String
  end

  interface Render[A]
    func show(value: A) -> String
  end

  impl Int64: Show[Int64]
    func show(value: Int64) -> String
      "show"
    end
  end

  impl Int64: Render[Int64]
    func show(value: Int64) -> String
      "render"
    end
  end
`)

	out, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, out)
	for _, want := range []string{
		"var Render_showDispatchRegistry = map[string]func(any) string{}",
		"var Show_showDispatchRegistry = map[string]func(any) string{}",
		"func Render_show(value any) string {",
		"func Show_show(value any) string {",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirLetsLocalBindingShadowTypeclassName(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  import fmt "go:fmt"

  interface Show[A]
    func show(value: A) -> String
  end

  impl Int64: Show[Int64]
    func show(value: Int64) -> String
      "typeclass"
    end
  end

  func demo() -> String
    let show = fmt.Sprint
    show(42)
  end
`)

	out, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, out)
	for _, want := range []string{
		"show_1 := fmt.Sprint",
		"return show_1(42)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
	if strings.Contains(got, "Show_show(42)") {
		t.Fatalf("generated Go unexpectedly used typeclass dispatcher\n--- got ---\n%s", got)
	}
}

func TestCompileDirDeduplicatesTypeclassMethodParams(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  interface FancyShow[A]
    func show(value: A) -> String
  end

  impl Int64FancyShow: FancyShow[Int64]
    func show(value: Int64) -> String
	  "fancy"
	end
  end

  func demo(value: Int64) -> String using Show[Int64], FancyShow[Int64]
    value.show()
  end
`)

	out, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, out)
	if strings.Count(got, "showFn ") != 1 {
		t.Fatalf("expected one typeclass function param, got generated Go:\n%s", got)
	}
}

func writeMygoFile(t *testing.T, dir, name, src string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	return string(data)
}
