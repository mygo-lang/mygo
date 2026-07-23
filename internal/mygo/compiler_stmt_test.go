package mygo

import (
	goparser "go/parser"
	"go/token"
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
	if strings.Contains(got, `github.com/mygo-lang/mygo/prelude`) {
		t.Fatalf("generated Go imported unused prelude\n--- got ---\n%s", got)
	}
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

func TestCompileDirAutoImportsPrelude(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "app")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	writeMygoFile(t, root, "go.mod", "module example.com/app\n\ngo 1.26\n\nreplace github.com/mygo-lang/mygo => "+filepath.ToSlash(repoRoot)+"\n")
	writeMygoFile(t, dir, "main.mygo", `package main
  func demo() -> Int
    let maybe: Option[Int] = Some(41)
    maybe.UnwrapOr(0) + 1
  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	for _, want := range []string{
		`. "github.com/mygo-lang/mygo/prelude"`,
		"var maybe_1 Option[int] = Some[int](41)",
		"MygoIN6OptionM8UnwrapOr(maybe_1, 0) + 1",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
	if _, err := goparser.ParseFile(token.NewFileSet(), "", got, goparser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n--- got ---\n%s", err, got)
	}
}

func TestCompileDirAutoImportsPreludeFromModuleCache(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "app")
	cacheRoot := filepath.Join(root, "modcache")
	mygoMod := filepath.Join(cacheRoot, "github.com", "mygo-lang", "mygo@v0.0.1")
	if err := os.MkdirAll(filepath.Join(mygoMod, "prelude"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeMygoFile(t, mygoMod, "go.mod", "module github.com/mygo-lang/mygo\n\ngo 1.26\n")
	writeMygoFile(t, filepath.Join(mygoMod, "prelude"), "prelude.mygo", `package prelude
enum Option[A]
  Some(A)
  None
end

impl[A] Option[A]
  func UnwrapOr(opt: Option[A], defaultVal: A) -> A
    defaultVal
  end
end
`)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOMODCACHE", cacheRoot)
	t.Setenv("GOPATH", "")
	writeMygoFile(t, root, "go.mod", "module example.com/app\n\ngo 1.26\n\nrequire github.com/mygo-lang/mygo v0.0.1\n")
	writeMygoFile(t, dir, "main.mygo", `package main
  func demo() -> Int
    let maybe: Option[Int] = Some(41)
    maybe.UnwrapOr(0) + 1
  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	for _, want := range []string{
		`. "github.com/mygo-lang/mygo/prelude"`,
		"MygoIN6OptionM8UnwrapOr(maybe_1, 0) + 1",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirReportsMissingPrelude(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "app")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeMygoFile(t, root, "go.mod", "module example.com/app\n\ngo 1.26\n")
	writeMygoFile(t, dir, "main.mygo", `package main
  func demo() -> Int
    42
  end
`)

	_, err := CompileDir(dir)
	if err == nil {
		t.Fatal("CompileDir() error = nil, want missing prelude error")
	}
	if !strings.Contains(err.Error(), "cannot locate prelude") {
		t.Fatalf("CompileDir() error = %v, want missing prelude error", err)
	}
}

func TestCompileDirSupportsStringSwitchOnStructField(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main

  struct Message
    Method: String
  end

  func demo(msg: Message) -> Int
    switch msg.Method
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

func TestCompileDirSupportsBlockIfWithoutElse(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  func demo(ok: Bool) -> ()
    if ok then
      ()
    end
  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	if _, err := goparser.ParseFile(token.NewFileSet(), "", got, goparser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n--- got ---\n%s", err, got)
	}
	if !strings.Contains(got, "if ok") {
		t.Fatalf("generated Go missing if statement\n--- got ---\n%s", got)
	}
}

func TestCompileDirSupportsSwitchOnLetBoundStructField(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main

  struct Message
    Method: String
  end

  func demo(msg: Message) -> Int
    let method = msg.Method
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
    Method: String
    Params: Option[Any]
  end

  func demo(msg: Message) -> Int
    let method = msg.Method
    switch method
      case "initialize" then
        switch msg.Params
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
		`if _, ok := msg.Params.(OptionSome[any]); ok {`,
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

func TestCompileDirSupportsCurrentModuleMyGoImport(t *testing.T) {
	root := t.TempDir()
	apiDir := filepath.Join(root, "lib", "api")
	appDir := filepath.Join(root, "examples", "app")
	if err := os.MkdirAll(apiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	writeMygoFile(t, root, "go.mod", "module example.com/project\n\ngo 1.26\n\nreplace github.com/mygo-lang/mygo => "+filepath.ToSlash(repoRoot)+"\n")
	writeMygoFile(t, apiDir, "api.mygo", `package api

  func PublicAdd(x: Int, y: Int) -> Int
    x + y
  end
`)
	writeMygoFile(t, appDir, "main.mygo", `package app
  import api "example.com/project/lib/api"

  func demo() -> Int
    api.PublicAdd(40, 2)
  end
`)

	outFiles, err := CompileDir(appDir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	if !strings.Contains(got, "api.PublicAdd(40, 2)") {
		t.Fatalf("generated Go missing current-module imported call\n--- got ---\n%s", got)
	}
}

func TestCompileDirSupportsReplacedModuleMyGoImport(t *testing.T) {
	root := t.TempDir()
	appRoot := filepath.Join(root, "app")
	depRoot := filepath.Join(root, "dep")
	depAPIDir := filepath.Join(depRoot, "lib", "api")
	appDir := filepath.Join(appRoot, "cmd", "app")
	for _, dir := range []string{depAPIDir, appDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	writeMygoFile(t, appRoot, "go.mod", "module example.com/app\n\ngo 1.26\n\nrequire example.com/dep v0.0.0\nreplace example.com/dep => ../dep\nreplace github.com/mygo-lang/mygo => "+filepath.ToSlash(repoRoot)+"\n")
	writeMygoFile(t, depRoot, "go.mod", "module example.com/dep\n\ngo 1.26\n")
	writeMygoFile(t, depAPIDir, "api.mygo", `package api

  func PublicAdd(x: Int, y: Int) -> Int
    x + y
  end
`)
	writeMygoFile(t, appDir, "main.mygo", `package app
  import api "example.com/dep/lib/api"

  func demo() -> Int
    api.PublicAdd(40, 2)
  end
`)

	outFiles, err := CompileDir(appDir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	if !strings.Contains(got, "api.PublicAdd(40, 2)") {
		t.Fatalf("generated Go missing replaced-module imported call\n--- got ---\n%s", got)
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

func TestCompileDirChecksMyGoImportFunctionTypes(t *testing.T) {
	root := t.TempDir()
	apiDir := filepath.Join(root, "api")
	appDir := filepath.Join(root, "app")
	if err := os.MkdirAll(apiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	writeMygoFile(t, root, "go.mod", "module example.com/app\n\ngo 1.26\n\nreplace github.com/mygo-lang/mygo => "+filepath.ToSlash(repoRoot)+"\n")

	writeMygoFile(t, apiDir, "api.mygo", `package api
  func PublicAdd(x: Int, y: Int) -> Int
    x + y
  end
`)
	writeMygoFile(t, appDir, "main.mygo", `package app
  import api "api"

  func demo() -> Int
    api.PublicAdd("wrong", 2)
  end
`)

	_, err = CompileDir(appDir)
	if err == nil {
		t.Fatal("CompileDir() error = nil, want imported MyGO function type mismatch")
	}
	if !strings.Contains(err.Error(), "PublicAdd") && !strings.Contains(err.Error(), "call type mismatch") {
		t.Fatalf("CompileDir() error = %v, want imported MyGO function type mismatch", err)
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

func TestCompileDirResolvesImportedMyGoStructAndEnumSymbols(t *testing.T) {
	root := t.TempDir()
	apiDir := filepath.Join(root, "api")
	appDir := filepath.Join(root, "app")
	for _, dir := range []string{apiDir, appDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	writeMygoFile(t, apiDir, "api.mygo", `package api
  struct Box
    Value: Int
  end

  enum FetchStatus
    Ok(Int)
  end

  func ReadBox(b: Box) -> Int
    b.Value
  end

  func ReadStatus(r: FetchStatus) -> Int
    switch r
      case Ok(v) then
        v
      end
    end
  end
`)
	writeMygoFile(t, appDir, "main.mygo", `package app
  import api "api"

  func demo() -> Int
    let b = api.Box{Value: 40}
    let r = api.FetchStatus.Ok(2)
    api.ReadBox(b) + api.ReadStatus(r)
  end
`)

	outFiles, err := CompileDir(appDir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	for _, want := range []string{
		`api.Box{Value: 40}`,
		`api.FetchStatusOkCtor(2)`,
		`api.ReadBox(b_1) + api.ReadStatus(r_2)`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirRejectsEnumConflictingWithPreludeType(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package app
  enum Result
    Done(Int)
  end
`)

	_, err := CompileDir(dir)
	if err == nil {
		t.Fatal("CompileDir() error = nil, want duplicate visible type failure")
	}
	if !strings.Contains(err.Error(), `symbol "Result" conflicts in scope`) {
		t.Fatalf("CompileDir() error = %v, want Result conflict", err)
	}
}

func TestCompileDirRejectsInterfaceConflictingWithPreludeTypeclass(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package app
  interface ToString[A]
    func ToString(value: A) -> String
  end
`)

	_, err := CompileDir(dir)
	if err == nil {
		t.Fatal("CompileDir() error = nil, want duplicate visible typeclass failure")
	}
	if !strings.Contains(err.Error(), `symbol "ToString" conflicts in scope`) {
		t.Fatalf("CompileDir() error = %v, want ToString conflict", err)
	}
}

func TestCompileDirAllowsImplForPreludeTypeclass(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package app
  impl Int64: ToString[Int64]
    func ToString(value: Int64) -> String
      "value"
    end
  end
`)

	_, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
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
		"a_",
		"b_",
		":= pair()",
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

func TestCompileDirSupportsLetRecBindingGroup(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  func demo() -> Int
    letrec
      even: func(Int) -> Bool = func(n: Int) -> Bool
        if n == 0 => true else odd(n - 1)
      end
      odd: func(Int) -> Bool = func(n: Int) -> Bool
        if n == 0 => false else even(n - 1)
      end
    end
    if even(4) => 1 else 0
  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	for _, want := range []string{
		"var even_1",
		"var odd_2",
		"even_1 = func",
		"odd_2 = func",
		"odd_2(n - 1)",
		"even_1(n - 1)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
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
    X: Int64
  end

  func make_point() -> Point
    Point { X: (42 as Int64) }
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
		"Point{X: int64(42)}",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirRejectsStructLiteralMissingField(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  struct Point
    X: Int64
    Y: Int64
  end

  func make_point() -> Point
    Point { X: (42 as Int64) }
  end
`)

	_, err := CompileDir(dir)
	if err == nil {
		t.Fatal("CompileDir() error = nil, want missing struct field failure")
	}
	if !strings.Contains(err.Error(), `struct "Point" literal missing field "Y"`) &&
		!strings.Contains(err.Error(), `struct Point literal missing field "Y"`) {
		t.Fatalf("CompileDir() error = %v, want missing struct field failure", err)
	}
}

func TestCompileDirAllowsOmittedEmbeddedStructLiteralField(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  struct Address
    Street: String
  end

  struct Contact
    Name: String
    embed Address
  end

  func make_contact() -> Contact
    Contact { Name: "Bob" }
  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	for _, want := range []string{
		"type Contact struct {",
		"Name string",
		"Address",
		`Contact{Name: "Bob"}`,
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
    Item: Option[Ref[Node]]
  end

  func use_ref(node: Ref[Node]) -> Int
    node.value
  end

  func describe(ok: Bool) -> Result[String, String]
    if ok => Ok("yes") else Err("no")
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
		"type Holder struct {",
		"Item Option[*Node]",
		"func use_ref(node *Node) int {",
		"func describe(ok bool) Result[string, string] {",
		"expr_1 = Ok[string, string](\"yes\")",
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
    Item: Option[Ref[Node]]
  end

  func maybe_node(ok: Bool, node: Ref[Node]) -> Option[Ref[Node]]
    if ok => Some(node) else None
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
		"expr_1 = Some[*Node](node)",
		"expr_1 = None[*Node]()",
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
    if ok => Some(Ref.new(node)) else None
  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	for _, want := range []string{
		"func maybe_node(ok bool, node Node) Option[*Node] {",
		"expr_1 = Some[*Node](&node)",
		"expr_1 = None[*Node]()",
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
    if ok => Ok(node) else Err("missing")
  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	for _, want := range []string{
		"func lookup(ok bool, node *Node) Result[*Node, string] {",
		"expr_1 = Ok[*Node, string](node)",
		"expr_1 = Err[*Node, string](\"missing\")",
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
    if ok => 1 else 2
  end

  func block(ok: Bool) -> Int
    if ok then
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
		"expr_1 = 1",
		"} else {",
		"expr_1 = 2",
		"func block(ok bool) int {",
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
  interface ShowString[A]
    func ToString(value: A) -> String
  end

  interface Render[A]
    func ToString(value: A) -> String
  end

  impl Int64: ShowString[Int64]
    func ToString(value: Int64) -> String
      "show"
    end
  end

  impl Int64: Render[Int64]
    func ToString(value: Int64) -> String
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
		"func MygoIT10ShowStringFN5Int64GN5Int64EM8ToString(value int64) string {",
		"func MygoIT6RenderFN5Int64GN5Int64EM8ToString(value int64) string {",
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

  interface LocalToString[A]
    func ToString(value: A) -> String
  end

  impl Int64: LocalToString[Int64]
    func ToString(value: Int64) -> String
      "typeclass"
    end
  end

  func demo() -> String
    let ToString = fmt.Sprint
    ToString(42)
  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	for _, want := range []string{
		"fmt.Sprint",
		"ToString_1(42)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
	if strings.Contains(got, "ToString_ToString(42)") {
		t.Fatalf("generated Go unexpectedly used typeclass dispatcher\n--- got ---\n%s", got)
	}
}

func TestCompileDirDeduplicatesTypeclassMethodParams(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
	  import fmt "go:fmt"

	  interface LocalToString[A]
	    func ToString(value: A) -> String
	  end

	  impl Int64Show: LocalToString[Int64]
	    func ToString(value: Int64) -> String
		  fmt.Sprint(value)
		end
	  end

	  func demo(value: Int64) -> String using LocalToString[Int64]
	    value.ToString()
	  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	if strings.Count(got, "ToStringFn ") != 1 {
		t.Fatalf("expected one typeclass function param, got generated Go:\n%s", got)
	}
}

func TestCompileDirSupportsMultipleUsingConstraints(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
	  import fmt "go:fmt"

	  interface LocalToString[A]
	    func ToString(value: A) -> String
	  end

	  interface Fancy[A]
	    func fancy(value: A) -> String
	  end

	  impl Int64Show: LocalToString[Int64]
	    func ToString(value: Int64) -> String
		  fmt.Sprint(value)
		end
	  end

	  impl StringFancy: Fancy[String]
	    func fancy(value: String) -> String
		  fmt.Sprint(value)
		end
	  end

	  func demo(value: Int64, label: String) -> String using LocalToString[Int64], Fancy[String]
	    value.ToString()
	    label.fancy()
	  end
`)

	outFiles, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, outFiles[0])
	for _, want := range []string{
		"ToStringFn func(int64) string",
		"fancyFn func(string) string",
		"ToStringFn(value)",
		"fancyFn(label)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirUsesNamedTypeclassImplementation(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main
  interface Equalish[A]
    func Equals(left: A, right: A) -> Bool
  end

  impl Int: Equalish[Int]
    func Equals(left: Int, right: Int) -> Bool
      false
    end
  end

  impl FastEq: Equalish[Int]
    func Equals(left: Int, right: Int) -> Bool
      true
    end
  end

  func same(left: Int, right: Int) -> Bool using FastEq: Equalish[Int]
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
	if !strings.Contains(got, "func MygoIT8EqualishFN6FastEqGN3IntEM6Equals") {
		t.Fatalf("expected named impl helper, got:\n%s", got)
	}
	if !strings.Contains(got, "same(1, 2, MygoIT8EqualishFN6FastEqGN3IntEM6Equals)") {
		t.Fatalf("expected call to pass named impl helper, got:\n%s", got)
	}
}

func TestCompileDirAllowsUsingImplementationName(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `package main

  interface LocalEnumerable[C[A], A]
    func Len(c: C[A]) -> Int
  end

  impl[T] SliceIEnumerable[T]: LocalEnumerable[Slice[T], T]
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
