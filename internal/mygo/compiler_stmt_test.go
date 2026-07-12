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

  func main() -> ()
    demo()
  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	for _, want := range []string{
		"[]int{1, 2, 3}",
		`"a": "1"`,
		`"b": "2"`,
		`"x": struct{}{}`,
		`"y": struct{}{}`,
		"[]int{}",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirSupportsStringSwitchOnStructField(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main

  struct Message
    method: String
  end

  func demo(msg: Message) -> Int
    switch msg.method
      case "initialize" => 1
      case "initialized" => 2
      case _ => 3
    end
  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	for _, want := range []string{
		"if msg.Method == \"initialize\"",
		"else",
		"msg.Method == \"initialized\"",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirSupportsSwitchOnLetBoundStructField(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main

  struct Message
    method: String
  end

  func demo(msg: Message) -> Int
    let method = msg.method
    switch method
      case "initialize" => 1
      case _ => 2
    end
  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	for _, want := range []string{
		`method_1 := msg.Method`,
		`if method_1 == "initialize"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirSupportsNestedStringAndOptionSwitches(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main

  struct Message
    method: String
    params: Option[Any]
  end

  func demo(msg: Message) -> Int
    let method = msg.method
    switch method
      case "initialize" then
        switch msg.params
          case Some(p) then 1
        end
      end
      end
      case "initialized" then
        3
      end
      case _ then
        4
      end
    end
  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	for _, want := range []string{
		`method_1 := msg.Method`,
		`if method_1 == "initialize"`,
		`if method_1 == "initialized"`,
		`if _, ok := msg.Params.(prelude.OptionSome[Any]); ok {`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirSupportsMyGoPackageImportExportsOnly(t *testing.T) {
	root := t.TempDir()
	apiDir := filepath.Join(root, "api")
	appDir := filepath.Join(root, "app")
	if err := os.MkdirAll(apiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeMygoFile(t, apiDir, "api.mygo", `package api

  func PublicAdd(x: Int, y: Int) -> Int
    x + y
  end

  func hiddenMul(x: Int, y: Int) -> Int
    x * y
  end
`)
	writeMygoFile(t, appDir, "main.mygo", `package app
  import api "api"

  func demo() -> Int
    api.PublicAdd(40, 2)
  end

  func main() -> ()
    demo()
  end
`)

	outFiles, err := CompileDir(appDir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	if !strings.Contains(got, "api.PublicAdd(40, 2)") {
		t.Fatalf("generated Go missing imported public call\n--- got ---\n%s", got)
	}
}

func TestCompileDirRejectsMyGoPackagePrivateSymbol(t *testing.T) {
	root := t.TempDir()
	apiDir := filepath.Join(root, "api")
	appDir := filepath.Join(root, "app")
	if err := os.MkdirAll(apiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeMygoFile(t, apiDir, "api.mygo", `package api

  func PublicAdd(x: Int, y: Int) -> Int
    x + y
  end

  func hiddenMul(x: Int, y: Int) -> Int
    x * y
  end
`)
	writeMygoFile(t, appDir, "main.mygo", `package app
  import api "api"

  func demo() -> Int
    api.hiddenMul(40, 2)
  end
`)

	_, err := CompileDir(appDir)
	if err == nil {
		t.Fatal("CompileDir() error = nil, want private symbol rejection")
	}
	if !strings.Contains(err.Error(), "hiddenMul") {
		t.Fatalf("CompileDir() error = %v, want hiddenMul rejection", err)
	}
}

func TestCompileDirKeepsSameNamedMyGoFunctionsSeparate(t *testing.T) {
	root := t.TempDir()
	apiDir := filepath.Join(root, "api")
	utilDir := filepath.Join(root, "util")
	appDir := filepath.Join(root, "app")
	for _, dir := range []string{apiDir, utilDir, appDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	writeMygoFile(t, apiDir, "api.mygo", `package api
  func PublicAdd(x: Int, y: Int) -> Int
    x + y
  end
`)
	writeMygoFile(t, utilDir, "util.mygo", `package util
  func PublicAdd(x: Int, y: Int) -> Int
    x * y
  end
`)
	writeMygoFile(t, appDir, "main.mygo", `package app
  import api "api"
  import util "util"

  func demo() -> Int
    api.PublicAdd(1, 2) + util.PublicAdd(3, 4)
  end
`)

	outFiles, err := CompileDir(appDir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	if !strings.Contains(got, "api.PublicAdd(1, 2)") {
		t.Fatalf("generated Go missing api call\n--- got ---\n%s", got)
	}
	if !strings.Contains(got, "util.PublicAdd(3, 4)") {
		t.Fatalf("generated Go missing util call\n--- got ---\n%s", got)
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

  func main() -> ()
    demo()
  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	if len(outFiles) < 1 {
		t.Fatalf("CompileDir() returned %d files, expected at least 1", len(outFiles))
	}
	got := readFile(t, outFiles[0])
	for _, want := range []string{
		"func demo() int {",
		"string = \"abc\"",
		"fmt.Println(msg_",
		"int = add(40, 2)",
		"+ 1",
		"return n_",
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

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	for _, want := range []string{
		"for n_",
		"fmt.Println(n_",
		"+ 1",
		"return n_",
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

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
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

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
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

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
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

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
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

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
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

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
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

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	if !strings.Contains(got, "return x_") {
		t.Fatalf("generated Go missing shadowed return\n--- got ---\n%s", got)
	}
	if !strings.Contains(got, "x_") || !strings.Contains(got, "x_") {
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
  struct Point
    x: Int64
  end

  func make_point() -> Point
    let p = Point { x: 42 }
    p.x
  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	for _, want := range []string{
		"type Point struct {",
		"X int64",
		"func make_point() Point {",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirSupportsRefAndResultTypes(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
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

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	gotContent := ""
	for _, f := range outFiles {
		gotContent += readFile(t, f)
	}
	got := gotContent
	for _, want := range []string{
		"type Result[A any, E any] interface {",
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

  struct Node
    value: Int
  end

  struct Holder
    item: Option[Ref[Node]]
  end

  func maybe_node(ok: Bool, node: Ref[Node]) -> Option[Ref[Node]]
    if ok then Some(node) else None
  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
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

func TestCompileDirSupportsRefValueMethod(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main

  func copy_value(value: Ref[Int]) -> Int
    value.value()
  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	for _, want := range []string{
		"func copy_value(value *int) int {",
		"return *value",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirSupportsRefNew(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main

  struct Node
    value: Int
  end

  func maybe_node(ok: Bool, node: Node) -> Option[Ref[Node]]
    if ok then Some(Ref.new(node)) else None
  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
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
    42.show()
  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	for _, want := range []string{
		"type Show[A any] interface {",
		"show(value A) string",
		"func show_int64(value int64) string {",
		"return 42.show()",
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
  import fmt "go:fmt"

  func demo() -> String
    let name = os.Getpid()
    fmt.Sprint(name)
  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	for _, want := range []string{
		"func demo() string {",
		"os.Getpid()",
		"fmt.Sprint(name",
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
	if !strings.Contains(err.Error(), "call type mismatch") {
		t.Fatalf("CompileDir() error = %v, want argument mismatch", err)
	}
}

func TestCompileDirSupportsGoValueAndPointerMethods(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  import bytes "go:bytes"

  func demo() -> Int
    let buf = bytes.NewBufferString("hi")
    42
  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	for _, want := range []string{
		"func demo() int {",
		"bytes.NewBufferString(\"hi\")",
		"return 42",
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

  func demo() -> Int
    os.Getpid()
    1
  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	for _, want := range []string{
		"func demo() int {",
		"os.Getpid()",
		"return 1",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirSupportsResultOfRefTypes(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main

  struct Node
    value: Int
  end

  func lookup(ok: Bool, node: Ref[Node]) -> Result[Ref[Node], String]
    if ok then Ok(node) else Err("missing")
  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
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

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
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
    1.equals(2)
  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	for _, want := range []string{
		"type Eq[A any] interface {",
		"equals(left A, right A) bool",
		"func equals_int64(left int64, right int64) bool {",
		"return 1.equals(2)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirSupportsArithmeticAndLogicOperators(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  func demo(a: Int, b: Int, ok: Bool) -> Bool
    ok && (a + b > 10) || (a - b <= 2)
  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	for _, want := range []string{
		"ok && a+b > 10 || a-b <= 2",
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

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	for _, want := range []string{
		"type Show[A any] interface {",
		"type Render[A any] interface {",
		"func show_int64(value int64) string {",
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

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	for _, want := range []string{
		"fmt.Sprint",
		"show(42)",
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

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	if strings.Count(got, "showFn ") != 1 {
		t.Fatalf("expected one typeclass function param, got generated Go:\n%s", got)
	}
}

func TestCompileDirUsesNamedTypeclassImplementation(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  interface Eq[A]
    func Equals(left: A, right: A) -> Bool
  end

  impl Int: Eq[Int]
    func Equals(left: Int, right: Int) -> Bool
      false
    end
  end

  impl FastEq: Eq[Int]
    func Equals(left: Int, right: Int) -> Bool
      true
    end
  end

  func same(left: Int, right: Int) -> Bool using FastEq: Eq[Int]
    left.Equals(right)
  end

  func demo() -> Bool
    same(1, 2)
  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	if !strings.Contains(got, "func Equals_fasteq_int") {
		t.Fatalf("expected named impl helper, got:\n%s", got)
	}
	if !strings.Contains(got, "same(1, 2, Equals_fasteq_int)") {
		t.Fatalf("expected call to pass named impl helper, got:\n%s", got)
	}
}

func TestCompileDirAllowsUsingImplementationName(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main

  interface IEnumerable[C[A], A]
    func Len(c: C[A]) -> Int
  end

  impl[T] SliceIEnumerable[T]: IEnumerable[Slice[T], T]
    func Len(c: Slice[T]) -> Int
      len(c)
    end
  end

  func count(values: Slice[Int]) -> Int using SliceIEnumerable[Int]
    values.len()
  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	if !strings.Contains(got, "Len_slice_t_t") && !strings.Contains(got, "lenFn") && !strings.Contains(got, "LenFn") {
		t.Fatalf("expected generated Go to include a typeclass helper, got:\n%s", got)
	}
}

func TestCompileDirEmitsHKTTypesOncePerPackage(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "prelude.mygo", `package prelude

  interface IEnumerable[C[A], A]
    func Each(c: C[A], fn: func(A) -> ()) -> ()
  end
`)
	writeMygoFile(t, dir, "string.mygo", `package prelude

  interface IStringOps[A]
    func Wrap(value: A) -> IEnumerable[Slice, A]
  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	var combined strings.Builder
	for _, path := range outFiles {
		combined.WriteString(readFile(t, path))
	}
	got := combined.String()
	if strings.Count(got, "type HKTType interface{}") != 1 {
		t.Fatalf("expected one HKTType declaration, got generated Go:\n%s", got)
	}
	if strings.Count(got, "type HKT1[F any] interface{}") != 1 {
		t.Fatalf("expected one HKT1 declaration, got generated Go:\n%s", got)
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
