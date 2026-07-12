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
  let nonempty_set: Set[String] = {"x"}
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
	if decl, ok := block.Stmts[5].(*LetStmt); !ok {
		t.Fatalf("Stmt[5] type = %T, want *LetStmt", block.Stmts[5])
	} else if _, ok := decl.Value.(*MapLitExpr); !ok {
		t.Fatalf("empty_map value type = %T, want *MapLitExpr", decl.Value)
	}
	if decl, ok := block.Stmts[6].(*LetStmt); !ok {
		t.Fatalf("Stmt[6] type = %T, want *LetStmt", block.Stmts[6])
	} else if _, ok := decl.Value.(*SetLitExpr); !ok {
		t.Fatalf("nonempty_set value type = %T, want *SetLitExpr", decl.Value)
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

func TestParseFileSupportsInherentImplDecl(t *testing.T) {
	src := `package main
struct Rectangle
  width: Float64
  height: Float64
end

impl Rectangle
  func area(self: Rectangle) -> Float64
    self.width * self.height
  end
end
`
	file, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	if got := len(file.Decls); got != 2 {
		t.Fatalf("len(Decls) = %d, want %d", got, 2)
	}
	impl, ok := file.Decls[1].(*ImplDecl)
	if !ok {
		t.Fatalf("Decls[1] type = %T, want *ImplDecl", file.Decls[1])
	}
	if impl.InterfaceName != "" || impl.Name != "" {
		t.Fatalf("inherent impl interface fields = Name:%q InterfaceName:%q, want empty", impl.Name, impl.InterfaceName)
	}
	if got := len(impl.Methods); got != 1 {
		t.Fatalf("len(ImplDecl.Methods) = %d, want 1", got)
	}
}

func TestParseFileSupportsStructFieldTags(t *testing.T) {
	src := `package main
struct User
  id: Int "json:\"id\""
  name: String "json:\"name,omitempty\" yaml:\"name\""
end
`
	file, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	st, ok := file.Decls[0].(*StructDecl)
	if !ok {
		t.Fatalf("Decls[0] type = %T, want *StructDecl", file.Decls[0])
	}
	if got := len(st.Fields); got != 2 {
		t.Fatalf("len(Fields) = %d, want %d", got, 2)
	}
	if st.Fields[0].Tag != "json:\"id\"" {
		t.Fatalf("Fields[0].Tag = %q, want %q", st.Fields[0].Tag, "json:\"id\"")
	}
	if st.Fields[1].Tag != "json:\"name,omitempty\" yaml:\"name\"" {
		t.Fatalf("Fields[1].Tag = %q, want %q", st.Fields[1].Tag, "json:\"name,omitempty\" yaml:\"name\"")
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

func TestParseFileSupportsNamedUsingImplementation(t *testing.T) {
	src := `package main
func eq(left: Int, right: Int) -> Bool using FastEq: Eq[Int]
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
	if got := len(fn.Using); got != 1 {
		t.Fatalf("len(FuncDecl.Using) = %d, want 1", got)
	}
	if fn.Using[0].Name != "Eq" {
		t.Fatalf("FuncDecl.Using[0].Name = %q, want Eq", fn.Using[0].Name)
	}
	if fn.Using[0].BindName != "FastEq" {
		t.Fatalf("FuncDecl.Using[0].BindName = %q, want FastEq", fn.Using[0].BindName)
	}
	if got := len(fn.Using[0].Args); got != 1 {
		t.Fatalf("len(FuncDecl.Using[0].Args) = %d, want 1", got)
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

func TestParseFileSupportsLiteralSwitchPatterns(t *testing.T) {
	src := `package main
func demo(n: Int) -> Int
  switch n
  case 0 => 1
  case 1 then
    2
  end
  case _ => 3
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
	lit, ok := sw.Cases[0].Pattern.(*ast.LiteralPattern)
	if !ok {
		t.Fatalf("Cases[0].Pattern type = %T, want *LiteralPattern", sw.Cases[0].Pattern)
	}
	if lit.Kind != "number" || lit.Value != "0" {
		t.Fatalf("Cases[0].Pattern = %#v, want number 0", lit)
	}
	lit, ok = sw.Cases[1].Pattern.(*ast.LiteralPattern)
	if !ok {
		t.Fatalf("Cases[1].Pattern type = %T, want *LiteralPattern", sw.Cases[1].Pattern)
	}
	if lit.Kind != "number" || lit.Value != "1" {
		t.Fatalf("Cases[1].Pattern = %#v, want number 1", lit)
	}
}

func TestParseFileKeepsNestedSwitchCasesSeparate(t *testing.T) {
	src := `package main
func demo(method: String, params: Option[Any]) -> Int
  switch method
  case "initialize" then
    switch params
    case Some(p) => 1
    case _ => 2
    end
  end
  case "initialized" => 3
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
	outer, ok := fn.Body.(*SwitchExpr)
	if !ok {
		t.Fatalf("FuncDecl.Body type = %T, want *SwitchExpr", fn.Body)
	}
	if got := len(outer.Cases); got != 2 {
		t.Fatalf("len(outer.Cases) = %d, want 2", got)
	}
	first, ok := outer.Cases[0].Pattern.(*ast.LiteralPattern)
	if !ok || first.Kind != "string" || first.Value != "initialize" {
		t.Fatalf("outer.Cases[0].Pattern = %#v, want string initialize", outer.Cases[0].Pattern)
	}
	body, ok := outer.Cases[0].Body.(*BlockExpr)
	if !ok || len(body.Stmts) != 1 {
		t.Fatalf("outer.Cases[0].Body = %T with %d stmts, want one-stmt block", outer.Cases[0].Body, len(body.Stmts))
	}
	stmt, ok := body.Stmts[0].(*ExprStmt)
	if !ok {
		t.Fatalf("nested stmt type = %T, want *ExprStmt", body.Stmts[0])
	}
	inner, ok := stmt.Expr.(*SwitchExpr)
	if !ok {
		t.Fatalf("nested expr type = %T, want *SwitchExpr", stmt.Expr)
	}
	if got := len(inner.Cases); got != 2 {
		t.Fatalf("len(inner.Cases) = %d, want 2", got)
	}
	if _, ok := inner.Cases[0].Pattern.(*VariantPattern); !ok {
		t.Fatalf("inner.Cases[0].Pattern type = %T, want *VariantPattern", inner.Cases[0].Pattern)
	}
}

func TestParseFileDocumentStoreBodyShape(t *testing.T) {
	src := `package main
struct Document
  uri: String
end
struct DocumentStore
  docs: Map[String, Ref[Document]]
end
func newDocumentStore() -> DocumentStore
  DocumentStore{docs: {}}
end
`
	file, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	fn, ok := file.Decls[2].(*FuncDecl)
	if !ok {
		t.Fatalf("Decls[2] type = %T, want *FuncDecl", file.Decls[2])
	}
	if _, ok := fn.Body.(*StructLitExpr); !ok {
		t.Fatalf("FuncDecl.Body type = %T, want *StructLitExpr", fn.Body)
	}
}

func TestParseFileSupportsInlineGoExpr(t *testing.T) {
	src := `package main
func demo(n: Int) -> Int
  go[Int] {
    code: "{x} + 1"
    in x = n
  }
end
`
	file, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	fn := file.Decls[0].(*FuncDecl)
	goExpr, ok := fn.Body.(*GoExpr)
	if !ok {
		t.Fatalf("FuncDecl.Body type = %T, want *GoExpr", fn.Body)
	}
	if goExpr.Code != "{x} + 1" {
		t.Fatalf("GoExpr.Code = %q, want %q", goExpr.Code, "{x} + 1")
	}
	if got := len(goExpr.Operands); got != 1 {
		t.Fatalf("len(GoExpr.Operands) = %d, want 1", got)
	}
	if goExpr.Operands[0].Name != "x" {
		t.Fatalf("GoExpr.Operands[0].Name = %q, want x", goExpr.Operands[0].Name)
	}
	if _, ok := goExpr.Result.(*NamedType); !ok {
		t.Fatalf("GoExpr.Result type = %T, want *NamedType", goExpr.Result)
	}
}

func TestParseFileRejectsInlineGoMissingResultType(t *testing.T) {
	src := `package main
func demo(n: Int) -> Int
  go {
    code: "{x}"
    in x = n
  }
end
`
	if _, err := ParseFile(src); err == nil {
		t.Fatal("ParseFile() error = nil, want error")
	}
}

func TestParseFileRejectsMalformedInlineGoOperand(t *testing.T) {
	src := `package main
func demo(n: Int) -> Int
  go[Int] {
    code: "{x}"
    in = n
  }
end
`
	if _, err := ParseFile(src); err == nil {
		t.Fatal("ParseFile() error = nil, want error")
	}
}

func TestParseFileSupportsInlineGoTypeOperand(t *testing.T) {
	src := `package main
func demo(n: Int) -> Int
  go[Int] {
    code: "var x {T} = {v}"
    in v = n
    type T = Int
  }
end
`
	file, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	fn := file.Decls[0].(*FuncDecl)
	goExpr, ok := fn.Body.(*GoExpr)
	if !ok {
		t.Fatalf("FuncDecl.Body type = %T, want *GoExpr", fn.Body)
	}
	if goExpr.Code != "var x {T} = {v}" {
		t.Fatalf("GoExpr.Code = %q, want %q", goExpr.Code, "var x {T} = {v}")
	}
	if got := len(goExpr.Operands); got != 1 {
		t.Fatalf("len(GoExpr.Operands) = %d, want 1", got)
	}
	if got := len(goExpr.TypeOperands); got != 1 {
		t.Fatalf("len(GoExpr.TypeOperands) = %d, want 1", got)
	}
	if goExpr.TypeOperands[0].Name != "T" {
		t.Fatalf("GoExpr.TypeOperands[0].Name = %q, want T", goExpr.TypeOperands[0].Name)
	}
	if _, ok := goExpr.TypeOperands[0].Type.(*NamedType); !ok {
		t.Fatalf("GoExpr.TypeOperands[0].Type type = %T, want *NamedType", goExpr.TypeOperands[0].Type)
	}
}

func TestParseFileSupportsInlineGoMixedOperands(t *testing.T) {
	src := `package main
func demo(n: Int, s: String) -> Bool
  go[Bool] {
    code: "{T}({v})"
    in v = n
    type T = String
  }
end
`
	file, err := ParseFile(src)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	fn := file.Decls[0].(*FuncDecl)
	goExpr, ok := fn.Body.(*GoExpr)
	if !ok {
		t.Fatalf("FuncDecl.Body type = %T, want *GoExpr", fn.Body)
	}
	if len(goExpr.Operands) != 1 || len(goExpr.TypeOperands) != 1 {
		t.Fatalf("expected 1 value operand and 1 type operand, got %d/%d",
			len(goExpr.Operands), len(goExpr.TypeOperands))
	}
}

func TestParseFileSupportsNewNumericTypes(t *testing.T) {
	src := `package main
func demo() -> Int8
  let a: Int8 = 42
  let b: UInt8 = 200
  let c: Int16 = 1000
  let d: UInt16 = 60000
  let e: Int32 = 100000
  let f: UInt32 = 3000000000
  let g: Float32 = 3.14
  127
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
	if len(block.Stmts) < 7 {
		t.Fatalf("expected at least 7 stmts in block, got %d", len(block.Stmts))
	}
	// Check type annotations
	for i, name := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		letStmt, ok := block.Stmts[i].(*LetStmt)
		if !ok {
			t.Fatalf("Stmts[%d] type = %T, want *LetStmt", i, block.Stmts[i])
		}
		if letStmt.Name != name {
			t.Fatalf("LetStmt[%d].Name = %q, want %q", i, letStmt.Name, name)
		}
	}
}

func TestParseFileSupportsTupleLiteralAndType(t *testing.T) {
	file, err := ParseFile(`package main

func pair() -> (Int, String)
  (1, "a")
end
`)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	fn, ok := file.Decls[0].(*FuncDecl)
	if !ok {
		t.Fatalf("Decls[0] type = %T, want *FuncDecl", file.Decls[0])
	}
	if _, ok := fn.Ret.(*TupleType); !ok {
		t.Fatalf("FuncDecl.Ret type = %T, want *TupleType", fn.Ret)
	}
	block, ok := fn.Body.(*TupleLitExpr)
	if !ok {
		t.Fatalf("FuncDecl.Body type = %T, want *TupleLitExpr", fn.Body)
	}
	if got := len(block.Elems); got != 2 {
		t.Fatalf("len(tuple elems) = %d, want 2", got)
	}
	if lit, ok := block.Elems[0].(*LiteralExpr); !ok || lit.Value != "1" {
		t.Fatalf("tuple elem[0] = %#v, want 1", block.Elems[0])
	}
	if lit, ok := block.Elems[1].(*LiteralExpr); !ok || lit.Value != "a" {
		t.Fatalf("tuple elem[1] = %#v, want \"a\"", block.Elems[1])
	}
}

func TestParseFileSupportsUnitLiteral(t *testing.T) {
	file, err := ParseFile(`package main

func demo() -> ()
  ()
end
`)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	fn, ok := file.Decls[0].(*FuncDecl)
	if !ok {
		t.Fatalf("Decls[0] type = %T, want *FuncDecl", file.Decls[0])
	}
	if _, ok := fn.Body.(*UnitLitExpr); !ok {
		t.Fatalf("FuncDecl.Body type = %T, want *UnitLitExpr", fn.Body)
	}
}

func TestParseFileSupportsHexOctalBinaryLiterals(t *testing.T) {
	src := `package main
func demo() -> Int
  let h: Int = 0xff
  let o: Int = 0o777
  let b: Int = 0b1010
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
	if len(block.Stmts) < 3 {
		t.Fatalf("expected at least 3 stmts in block, got %d", len(block.Stmts))
	}
	for i, expected := range []string{"0xff", "0o777", "0b1010"} {
		letStmt, ok := block.Stmts[i].(*LetStmt)
		if !ok {
			t.Fatalf("Stmts[%d] type = %T, want *LetStmt", i, block.Stmts[i])
		}
		lit, ok := letStmt.Value.(*LiteralExpr)
		if !ok {
			t.Fatalf("LetStmt[%d].Value type = %T, want *LiteralExpr", i, letStmt.Value)
		}
		if lit.Kind != "number" {
			t.Fatalf("LiteralExpr[%d].Kind = %q, want %q", i, lit.Kind, "number")
		}
		if lit.Value != expected {
			t.Fatalf("LiteralExpr[%d].Value = %q, want %q", i, lit.Value, expected)
		}
	}
}
