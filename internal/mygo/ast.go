package mygo

type File struct {
	PackageName string
	PackageLine int
	Decls       []Decl
}

type Decl interface{ declNode() }

type ImportDecl struct {
	Line  int
	Alias string
	Path  string
}

func (*ImportDecl) declNode() {}

type EnumDecl struct {
	Line       int
	Name       string
	TypeParams []string
	Variants   []EnumVariant
}

func (*EnumDecl) declNode() {}

type EnumVariant struct {
	Line   int
	Name   string
	Fields []Field
}

type StructDecl struct {
	Line       int
	Name       string
	TypeParams []string
	Fields     []Field
}

func (*StructDecl) declNode() {}

type InterfaceDecl struct {
	Line       int
	Name       string
	TypeParams []string
	Methods    []*FuncDecl
}

func (*InterfaceDecl) declNode() {}

type ImplDecl struct {
	Line     int
	Name     string
	TypeArgs []TypeExpr
	Methods  []*FuncDecl
}

func (*ImplDecl) declNode() {}

type FuncDecl struct {
	Line       int
	Name       string
	TypeParams []string
	Params     []Param
	Ret        TypeExpr
	Where      []Constraint
	Body       Expr
}

func (*FuncDecl) declNode() {}

type Constraint struct {
	Line int
	Name string
	Args []TypeExpr
}

type Param struct {
	Line int
	Name string
	Type TypeExpr
}

type Field struct {
	Line int
	Name string
	Type TypeExpr
}

type TypeExpr interface{ typeNode() }

type NamedType struct {
	Line int
	Name string
	Args []TypeExpr
}

func (*NamedType) typeNode() {}

type FuncType struct {
	Line   int
	Params []TypeExpr
	Ret    TypeExpr
}

func (*FuncType) typeNode() {}

type Expr interface{ exprNode() }

type Stmt interface{ stmtNode() }

type IdentExpr struct {
	Line int
	Name string
}

func (*IdentExpr) exprNode() {}

type LiteralExpr struct {
	Line  int
	Kind  string
	Value string
}

func (*LiteralExpr) exprNode() {}

type CallExpr struct {
	Line   int
	Callee Expr
	Args   []Expr
}

func (*CallExpr) exprNode() {}

type StructLitExpr struct {
	Line     int
	TypeName string
	TypeArgs []TypeExpr
	Fields   []StructLitField
}

func (*StructLitExpr) exprNode() {}

type StructLitField struct {
	Line  int
	Name  string
	Value Expr
}

type BinaryExpr struct {
	Line  int
	Op    string
	Left  Expr
	Right Expr
}

func (*BinaryExpr) exprNode() {}

type PrefixExpr struct {
	Line int
	Op   string
	Expr Expr
}

func (*PrefixExpr) exprNode() {}

type FieldExpr struct {
	Line  int
	Expr  Expr
	Field string
}

func (*FieldExpr) exprNode() {}

type FuncLitExpr struct {
	Line   int
	Params []Param
	Ret    TypeExpr
	Body   Expr
}

func (*FuncLitExpr) exprNode() {}

type IfExpr struct {
	Line int
	Cond Expr
	Then Expr
	Else Expr
}

func (*IfExpr) exprNode() {}

type SwitchExpr struct {
	Line   int
	Target Expr
	Cases  []SwitchCase
}

func (*SwitchExpr) exprNode() {}

type WhileExpr struct {
	Line int
	Cond Expr
	Body Expr
}

func (*WhileExpr) exprNode() {}

type BlockExpr struct {
	Line  int
	Stmts []Stmt
}

func (*BlockExpr) exprNode() {}

type ExprStmt struct {
	Line int
	Expr Expr
}

func (*ExprStmt) stmtNode() {}

type LetStmt struct {
	Line    int
	Mutable bool
	Name    string
	Type    TypeExpr
	Value   Expr
}

func (*LetStmt) stmtNode() {}
func (*LetStmt) declNode() {}

type AssignStmt struct {
	Line  int
	Name  string
	Value Expr
}

func (*AssignStmt) stmtNode() {}

type SwitchCase struct {
	Line    int
	Pattern Pattern
	Body    Expr
}

type Pattern interface{ patternNode() }

type VariantPattern struct {
	Line int
	Name string
	Args []string
}

func (*VariantPattern) patternNode() {}

type WildcardPattern struct{ Line int }

func (*WildcardPattern) patternNode() {}
