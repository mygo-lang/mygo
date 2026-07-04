package parser

import (
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/ast"
)

func TestParseFileSupportsPackageAndFuncDecl(t *testing.T) {
	src := `package main
func demo() -> Int
  42
end
`
	file, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	if file.PackageName != "main" {
		t.Fatalf("PackageName = %q, want %q", file.PackageName, "main")
	}
	if len(file.Decls) != 1 {
		t.Fatalf("len(Decls) = %d, want %d", len(file.Decls), 1)
	}
	fn, ok := file.Decls[0].(*FuncDecl)
	if !ok {
		t.Fatalf("Decls[0] type = %T, want *FuncDecl", file.Decls[0])
	}
	if fn.Name != "demo" {
		t.Fatalf("FuncDecl.Name = %q, want %q", fn.Name, "demo")
	}
}

func TestParseFileSupportsCollectionLiterals(t *testing.T) {
	src := `package main
func demo() -> Int
  let numbers: Slice[Int] = [1, 2, 3]
  let m: Map[String, String] = {"a": "1", "b": "2"}
  let s: Set[String] = {"x", "y"}
  let nested: Map[String, Slice[Int]] = {"nums": [4, 5]}
  let empty_slice: Slice[Int] = []
  let empty_map: Map[String, Int] = {}
  let empty_set: Set[String] = {}
  42
end
`
	file, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	fn, ok := file.Decls[0].(*FuncDecl)
	if !ok {
		t.Fatalf("Decls[0] type = %T, want *FuncDecl", file.Decls[0])
	}
	if fn.Body == nil {
		t.Fatal("FuncDecl.Body = nil")
	}
	block, ok := fn.Body.(*BlockExpr)
	if !ok {
		t.Fatalf("FuncDecl.Body type = %T, want *BlockExpr", fn.Body)
	}
	if got := len(block.Stmts); got != 8 {
		t.Fatalf("len(BlockExpr.Stmts) = %d, want %d", got, 8)
	}
}

func TestParseFileSupportsNestedCollectionLiterals(t *testing.T) {
	src := `package main
func demo() -> Int
  let matrix: Slice[Slice[Int]] = [[1, 2], [3, 4]]
  let buckets: Map[String, Slice[Int]] = {"a": [1], "b": [2, 3]}
  0
end
`
	file, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	fn, ok := file.Decls[0].(*FuncDecl)
	if !ok {
		t.Fatalf("Decls[0] type = %T, want *FuncDecl", file.Decls[0])
	}
	block, ok := fn.Body.(*BlockExpr)
	if !ok {
		t.Fatalf("FuncDecl.Body type = %T, want *BlockExpr", fn.Body)
	}
	if got := len(block.Stmts); got != 3 {
		t.Fatalf("len(BlockExpr.Stmts) = %d, want %d", got, 3)
	}
}

func TestParseFileSupportsChainPostfix(t *testing.T) {
	src := `package main
func demo() -> Int
  box.make(1).value
end
`
	file, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	fn, ok := file.Decls[0].(*FuncDecl)
	if !ok {
		t.Fatalf("Decls[0] type = %T, want *FuncDecl", file.Decls[0])
	}
	block, ok := fn.Body.(*BlockExpr)
	if ok {
		if len(block.Stmts) != 1 {
			t.Fatalf("len(BlockExpr.Stmts) = %d, want %d", len(block.Stmts), 1)
		}
		return
	}
	if _, ok := fn.Body.(*FieldExpr); !ok {
		t.Fatalf("FuncDecl.Body type = %T, want *FieldExpr or *BlockExpr", fn.Body)
	}
}

func TestParseFileSupportsIfWhileAndSwitch(t *testing.T) {
	src := `package main
func demo(n: Int) -> Int
  if n < 1 then 10 else 20
  while n < 3
    n = n + 1
  end
  switch n
  case zero => 100
  case _ => 200
  end
end
`
	file, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	fn, ok := file.Decls[0].(*FuncDecl)
	if !ok {
		t.Fatalf("Decls[0] type = %T, want *FuncDecl", file.Decls[0])
	}
	block, ok := fn.Body.(*BlockExpr)
	if !ok {
		t.Fatalf("FuncDecl.Body type = %T, want *BlockExpr", fn.Body)
	}
	if got := len(block.Stmts); got != 3 {
		t.Fatalf("len(BlockExpr.Stmts) = %d, want %d", got, 3)
	}
	ifExpr, ok := block.Stmts[0].(*ExprStmt)
	if !ok {
		t.Fatalf("Stmt[0] type = %T, want *ExprStmt", block.Stmts[0])
	}
	if _, ok := ifExpr.Expr.(*IfExpr); !ok {
		t.Fatalf("Stmt[0].Expr type = %T, want *IfExpr", ifExpr.Expr)
	}
	if whileStmt, ok := block.Stmts[1].(*ExprStmt); !ok {
		t.Fatalf("Stmt[1] type = %T, want *ExprStmt", block.Stmts[1])
	} else if _, ok := whileStmt.Expr.(*WhileExpr); !ok {
		t.Fatalf("Stmt[1].Expr type = %T, want *WhileExpr", whileStmt.Expr)
	}
	switchStmt, ok := block.Stmts[2].(*ExprStmt)
	if !ok {
		t.Fatalf("Stmt[2] type = %T, want *ExprStmt", block.Stmts[2])
	}
	sw, ok := switchStmt.Expr.(*SwitchExpr)
	if !ok {
		t.Fatalf("Stmt[2].Expr type = %T, want *SwitchExpr", switchStmt.Expr)
	}
	if got := len(sw.Cases); got != 2 {
		t.Fatalf("len(SwitchExpr.Cases) = %d, want %d", got, 2)
	}
}

func TestParseFilePreservesPipePrecedence(t *testing.T) {
	src := `package main
func demo() -> Int
  1 + 2 |> add(3)
end
`
	file, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	fn, ok := file.Decls[0].(*FuncDecl)
	if !ok {
		t.Fatalf("Decls[0] type = %T, want *FuncDecl", file.Decls[0])
	}
	bin, ok := fn.Body.(*BinaryExpr)
	if !ok {
		t.Fatalf("FuncDecl.Body type = %T, want *BinaryExpr", fn.Body)
	}
	if bin.Op != "|>" {
		t.Fatalf("BinaryExpr.Op = %q, want %q", bin.Op, "|>")
	}
	left, ok := bin.Left.(*BinaryExpr)
	if !ok {
		t.Fatalf("BinaryExpr.Left type = %T, want *BinaryExpr", bin.Left)
	}
	if left.Op != "+" {
		t.Fatalf("BinaryExpr.Left.Op = %q, want %q", left.Op, "+")
	}
	if _, ok := bin.Right.(*ast.CallExpr); !ok {
		t.Fatalf("BinaryExpr.Right type = %T, want *CallExpr", bin.Right)
	}
}

func TestParseFileSupportsStructInterfaceAndImplDecls(t *testing.T) {
	src := `package main
struct Box[T]
  value: T
end

interface Show[T]
  func show(value: T) -> String
end

impl Box[T]: Show[T]
  func show(value: Box[T]) -> String
    "ok"
  end
end
`
	file, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	if got := len(file.Decls); got != 3 {
		t.Fatalf("len(Decls) = %d, want %d", got, 3)
	}
	if _, ok := file.Decls[0].(*StructDecl); !ok {
		t.Fatalf("Decls[0] type = %T, want *StructDecl", file.Decls[0])
	}
	iface, ok := file.Decls[1].(*InterfaceDecl)
	if !ok {
		t.Fatalf("Decls[1] type = %T, want *InterfaceDecl", file.Decls[1])
	}
	if got := len(iface.Methods); got != 1 {
		t.Fatalf("len(InterfaceDecl.Methods) = %d, want %d", got, 1)
	}
	if _, ok := file.Decls[2].(*ImplDecl); !ok {
		t.Fatalf("Decls[2] type = %T, want *ImplDecl", file.Decls[2])
	}
}

func TestParseFileSupportsLetVarAndAssign(t *testing.T) {
	src := `package main
func demo() -> Int
  let x: Int = 1
  var y: Int = 2
  y = y + 1
  x + y
end
`
	file, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	fn, ok := file.Decls[0].(*FuncDecl)
	if !ok {
		t.Fatalf("Decls[0] type = %T, want *FuncDecl", file.Decls[0])
	}
	block, ok := fn.Body.(*BlockExpr)
	if !ok {
		t.Fatalf("FuncDecl.Body type = %T, want *BlockExpr", fn.Body)
	}
	if got := len(block.Stmts); got != 4 {
		t.Fatalf("len(BlockExpr.Stmts) = %d, want %d", got, 4)
	}
	if letStmt, ok := block.Stmts[0].(*LetStmt); !ok {
		t.Fatalf("Stmt[0] type = %T, want *LetStmt", block.Stmts[0])
	} else if letStmt.Mutable {
		t.Fatal("let stmt marked mutable")
	}
	if varStmt, ok := block.Stmts[1].(*LetStmt); !ok {
		t.Fatalf("Stmt[1] type = %T, want *LetStmt", block.Stmts[1])
	} else if !varStmt.Mutable {
		t.Fatal("var stmt not marked mutable")
	}
	if _, ok := block.Stmts[2].(*AssignStmt); !ok {
		t.Fatalf("Stmt[2] type = %T, want *AssignStmt", block.Stmts[2])
	}
}

func TestParseFileSupportsSingleLineIfAndFuncLit(t *testing.T) {
	src := `package main
func demo() -> Int
  if true then 1 else 2
  func(x: Int) -> Int x + 1 end
end
`
	file, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	fn, ok := file.Decls[0].(*FuncDecl)
	if !ok {
		t.Fatalf("Decls[0] type = %T, want *FuncDecl", file.Decls[0])
	}
	block, ok := fn.Body.(*BlockExpr)
	if !ok {
		t.Fatalf("FuncDecl.Body type = %T, want *BlockExpr", fn.Body)
	}
	if got := len(block.Stmts); got != 2 {
		t.Fatalf("len(BlockExpr.Stmts) = %d, want %d", got, 2)
	}
	if _, ok := block.Stmts[0].(*ExprStmt); !ok {
		t.Fatalf("Stmt[0] type = %T, want *ExprStmt", block.Stmts[0])
	}
	if _, ok := block.Stmts[1].(*ExprStmt); !ok {
		t.Fatalf("Stmt[1] type = %T, want *ExprStmt", block.Stmts[1])
	}
}

func TestParseFileSupportsUsingClauses(t *testing.T) {
	src := `package main
func eq[A](left: A, right: A) -> Bool using Eq[A], Show[A]
  true
end
`
	file, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	fn, ok := file.Decls[0].(*FuncDecl)
	if !ok {
		t.Fatalf("Decls[0] type = %T, want *FuncDecl", file.Decls[0])
	}
	if got := len(fn.TypeParams); got != 1 || fn.TypeParams[0] != "A" {
		t.Fatalf("FuncDecl.TypeParams = %#v, want [A]", fn.TypeParams)
	}
	if got := len(fn.Using); got != 2 {
		t.Fatalf("len(FuncDecl.Using) = %d, want %d", got, 2)
	}
	if fn.Using[0].Name != "Eq" {
		t.Fatalf("FuncDecl.Using[0].Name = %q, want %q", fn.Using[0].Name, "Eq")
	}
	if got := len(fn.Using[0].Args); got != 1 {
		t.Fatalf("len(FuncDecl.Using[0].Args) = %d, want %d", got, 1)
	}
	if fn.Using[1].Name != "Show" {
		t.Fatalf("FuncDecl.Using[1].Name = %q, want %q", fn.Using[1].Name, "Show")
	}
	if got := len(fn.Using[1].Args); got != 1 {
		t.Fatalf("len(FuncDecl.Using[1].Args) = %d, want %d", got, 1)
	}
}

func TestParseFileSupportsInterfaceUsingClauses(t *testing.T) {
	src := `package main
interface Show[A]
  func show(value: A) -> String using Eq[A]
end
`
	file, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	iface, ok := file.Decls[0].(*InterfaceDecl)
	if !ok {
		t.Fatalf("Decls[0] type = %T, want *InterfaceDecl", file.Decls[0])
	}
	if got := len(iface.Methods); got != 1 {
		t.Fatalf("len(InterfaceDecl.Methods) = %d, want %d", got, 1)
	}
	method := iface.Methods[0]
	if got := len(method.Using); got != 1 {
		t.Fatalf("len(FuncDecl.Using) = %d, want %d", got, 1)
	}
	if method.Using[0].Name != "Eq" {
		t.Fatalf("FuncDecl.Using[0].Name = %q, want %q", method.Using[0].Name, "Eq")
	}
}

func TestParseFileSupportsEnumDecls(t *testing.T) {
	src := `package main
enum Option[T]
  Some
  None
end
`
	file, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	if got := len(file.Decls); got != 1 {
		t.Fatalf("len(Decls) = %d, want %d", got, 1)
	}
	enum, ok := file.Decls[0].(*EnumDecl)
	if !ok {
		t.Fatalf("Decls[0] type = %T, want *EnumDecl", file.Decls[0])
	}
	if enum.Name != "Option" {
		t.Fatalf("EnumDecl.Name = %q, want %q", enum.Name, "Option")
	}
	if got := len(enum.TypeParams); got != 1 || enum.TypeParams[0] != "T" {
		t.Fatalf("EnumDecl.TypeParams = %#v, want [T]", enum.TypeParams)
	}
	if got := len(enum.Variants); got != 2 {
		t.Fatalf("len(EnumDecl.Variants) = %d, want %d", got, 2)
	}
	if enum.Variants[0].Name != "Some" || enum.Variants[1].Name != "None" {
		t.Fatalf("EnumDecl.Variants = %#v, want [Some None]", enum.Variants)
	}
}

func TestParseFileSupportsSwitchVariantPatterns(t *testing.T) {
	src := `package main
func demo(v: Option) -> Int
  switch v
  case Some(x) => 1
  case None => 0
  case _ => 2
  end
end
`
	file, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	fn, ok := file.Decls[0].(*FuncDecl)
	if !ok {
		t.Fatalf("Decls[0] type = %T, want *FuncDecl", file.Decls[0])
	}
	sw, ok := fn.Body.(*SwitchExpr)
	if !ok {
		t.Fatalf("FuncDecl.Body type = %T, want *SwitchExpr", fn.Body)
	}
	if got := len(sw.Cases); got != 3 {
		t.Fatalf("len(SwitchExpr.Cases) = %d, want %d", got, 3)
	}
	if pat, ok := sw.Cases[0].Pattern.(*VariantPattern); !ok {
		t.Fatalf("Cases[0].Pattern type = %T, want *VariantPattern", sw.Cases[0].Pattern)
	} else if pat.Name != "Some" || len(pat.Args) != 1 || pat.Args[0] != "x" {
		t.Fatalf("Cases[0].Pattern = %#v, want Some(x)", pat)
	}
	if pat, ok := sw.Cases[1].Pattern.(*VariantPattern); !ok {
		t.Fatalf("Cases[1].Pattern type = %T, want *VariantPattern", sw.Cases[1].Pattern)
	} else if pat.Name != "None" || len(pat.Args) != 0 {
		t.Fatalf("Cases[1].Pattern = %#v, want None", pat)
	}
	if _, ok := sw.Cases[2].Pattern.(*WildcardPattern); !ok {
		t.Fatalf("Cases[2].Pattern type = %T, want *WildcardPattern", sw.Cases[2].Pattern)
	}
}

func TestParseFileSupportsIfArrowForm(t *testing.T) {
	src := `package main
func demo(n: Int) -> Int
  if n > 0 => n else 0
  if true => 1 else 2
end
`
	file, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	fn, ok := file.Decls[0].(*FuncDecl)
	if !ok {
		t.Fatalf("Decls[0] type = %T, want *FuncDecl", file.Decls[0])
	}
	block, ok := fn.Body.(*BlockExpr)
	if !ok {
		t.Fatalf("FuncDecl.Body type = %T, want *BlockExpr", fn.Body)
	}
	if got := len(block.Stmts); got != 2 {
		t.Fatalf("len(BlockExpr.Stmts) = %d, want %d", got, 2)
	}
	// First if: if n > 0 => n else 0
	ifExpr, ok := block.Stmts[0].(*ExprStmt)
	if !ok {
		t.Fatalf("Stmt[0] type = %T, want *ExprStmt", block.Stmts[0])
	}
	if _, ok := ifExpr.Expr.(*IfExpr); !ok {
		t.Fatalf("Stmt[0].Expr type = %T, want *IfExpr", ifExpr.Expr)
	}
	// Second if: if true => 1 else 2
	ifExpr2, ok := block.Stmts[1].(*ExprStmt)
	if !ok {
		t.Fatalf("Stmt[1] type = %T, want *ExprStmt", block.Stmts[1])
	}
	if _, ok := ifExpr2.Expr.(*IfExpr); !ok {
		t.Fatalf("Stmt[1].Expr type = %T, want *IfExpr", ifExpr2.Expr)
	}
}

func TestParseFileSupportsSwitchCaseThenEndBlock(t *testing.T) {
	src := `package main
func demo(v: Option) -> Int
  switch v
  case Some(x) then
    x
  end
  case None then
    0
  end
  case _ then
    2
  end
  end
end
`
	file, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	fn, ok := file.Decls[0].(*FuncDecl)
	if !ok {
		t.Fatalf("Decls[0] type = %T, want *FuncDecl", file.Decls[0])
	}
	sw, ok := fn.Body.(*SwitchExpr)
	if !ok {
		t.Fatalf("FuncDecl.Body type = %T, want *SwitchExpr", fn.Body)
	}
	if got := len(sw.Cases); got != 3 {
		t.Fatalf("len(SwitchExpr.Cases) = %d, want %d", got, 3)
	}
	if pat, ok := sw.Cases[0].Pattern.(*VariantPattern); !ok {
		t.Fatalf("Cases[0].Pattern type = %T, want *VariantPattern", sw.Cases[0].Pattern)
	} else if pat.Name != "Some" || len(pat.Args) != 1 || pat.Args[0] != "x" {
		t.Fatalf("Cases[0].Pattern = %#v, want Some(x)", pat)
	}
	if pat, ok := sw.Cases[1].Pattern.(*VariantPattern); !ok {
		t.Fatalf("Cases[1].Pattern type = %T, want *VariantPattern", sw.Cases[1].Pattern)
	} else if pat.Name != "None" || len(pat.Args) != 0 {
		t.Fatalf("Cases[1].Pattern = %#v, want None", pat)
	}
	if _, ok := sw.Cases[2].Pattern.(*WildcardPattern); !ok {
		t.Fatalf("Cases[2].Pattern type = %T, want *WildcardPattern", sw.Cases[2].Pattern)
	}
}

func TestParseFileSupportsMixedSwitchCaseForms(t *testing.T) {
	src := `package main
func demo(v: Option) -> Int
  switch v
  case Some(x) => x
  case None then
    fmt.Println("none")
    0
  end
  case _ => 2
  end
end
`
	file, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	fn, ok := file.Decls[0].(*FuncDecl)
	if !ok {
		t.Fatalf("Decls[0] type = %T, want *FuncDecl", file.Decls[0])
	}
	sw, ok := fn.Body.(*SwitchExpr)
	if !ok {
		t.Fatalf("FuncDecl.Body type = %T, want *SwitchExpr", fn.Body)
	}
	if got := len(sw.Cases); got != 3 {
		t.Fatalf("len(SwitchExpr.Cases) = %d, want %d", got, 3)
	}
}
