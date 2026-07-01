package mygo

type File struct {
	Module string
	Decls  []Decl
}

type Decl interface{ declNode() }

type ImportDecl struct {
	Alias string
	Path  string
}

func (*ImportDecl) declNode() {}

type ModuleDecl struct {
	Name string
}

func (*ModuleDecl) declNode() {}

type EnumDecl struct {
	Name       string
	TypeParams []string
	Variants   []EnumVariant
}

func (*EnumDecl) declNode() {}

type EnumVariant struct {
	Name   string
	Fields []Field
}

type StructDecl struct {
	Name       string
	TypeParams []string
	Fields     []Field
}

func (*StructDecl) declNode() {}

type InterfaceDecl struct {
	Name       string
	TypeParams []string
	Methods    []*FuncDecl
}

func (*InterfaceDecl) declNode() {}

type ImplDecl struct {
	Name     string
	TypeArgs []TypeExpr
	Methods  []*FuncDecl
}

func (*ImplDecl) declNode() {}

type FuncDecl struct {
	Name       string
	TypeParams []string
	Params     []Param
	Ret        TypeExpr
	Where      []Constraint
	Body       Expr
}

func (*FuncDecl) declNode() {}

type Constraint struct {
	Name string
	Args []TypeExpr
}

type Param struct {
	Name string
	Type TypeExpr
}

type Field struct {
	Name string
	Type TypeExpr
}

type TypeExpr interface{ typeNode() }

type NamedType struct {
	Name string
	Args []TypeExpr
}

func (*NamedType) typeNode() {}

type FuncType struct {
	Params []TypeExpr
	Ret    TypeExpr
}

func (*FuncType) typeNode() {}

type Expr interface{ exprNode() }

type Stmt interface{ stmtNode() }

type IdentExpr struct{ Name string }

func (*IdentExpr) exprNode() {}

type LiteralExpr struct {
	Kind  string
	Value string
}

func (*LiteralExpr) exprNode() {}

type CallExpr struct {
	Callee Expr
	Args   []Expr
}

func (*CallExpr) exprNode() {}

type BinaryExpr struct {
	Op    string
	Left  Expr
	Right Expr
}

func (*BinaryExpr) exprNode() {}

type PrefixExpr struct {
	Op   string
	Expr Expr
}

func (*PrefixExpr) exprNode() {}

type FieldExpr struct {
	Expr  Expr
	Field string
}

func (*FieldExpr) exprNode() {}

type FuncLitExpr struct {
	Params []Param
	Ret    TypeExpr
	Body   Expr
}

func (*FuncLitExpr) exprNode() {}

type SwitchExpr struct {
	Target Expr
	Cases  []SwitchCase
}

func (*SwitchExpr) exprNode() {}

type BlockExpr struct {
	Stmts []Stmt
}

func (*BlockExpr) exprNode() {}

type ExprStmt struct {
	Expr Expr
}

func (*ExprStmt) stmtNode() {}

type LetStmt struct {
	Mutable bool
	Name    string
	Type    TypeExpr
	Value   Expr
}

func (*LetStmt) stmtNode() {}

type AssignStmt struct {
	Name  string
	Value Expr
}

func (*AssignStmt) stmtNode() {}

type SwitchCase struct {
	Pattern Pattern
	Body    Expr
}

type Pattern interface{ patternNode() }

type VariantPattern struct {
	Name string
	Args []string
}

func (*VariantPattern) patternNode() {}

type WildcardPattern struct{}

func (*WildcardPattern) patternNode() {}
