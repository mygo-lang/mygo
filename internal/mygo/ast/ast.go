package ast

type File struct {
	PackageName   string
	PackageLine   int
	PackageColumn int
	Decls         []Decl
}

type Decl interface{ declNode() }

type ImportDecl struct {
	Line   int
	Column int
	Alias  string
	Path   string
}

func (*ImportDecl) declNode() {}

type EnumDecl struct {
	Line       int
	Column     int
	Name       string
	TypeParams []string
	Variants   []EnumVariant
}

func (*EnumDecl) declNode() {}

type EnumVariant struct {
	Line   int
	Column int
	Name   string
	Fields []Field
}

type StructDecl struct {
	Line       int
	Column     int
	Name       string
	TypeParams []string
	Fields     []Field
}

func (*StructDecl) declNode() {}

type InterfaceDecl struct {
	Line       int
	Column     int
	Name       string
	TypeParams []string
	Methods    []*FuncDecl
}

func (*InterfaceDecl) declNode() {}

type ImplDecl struct {
	Line     int
	Column   int
	Name     string
	TypeArgs []TypeExpr
	Methods  []*FuncDecl
}

func (*ImplDecl) declNode() {}

type FuncDecl struct {
	Line       int
	Column     int
	Name       string
	TypeParams []string
	Params     []Param
	Ret        TypeExpr
	Where      []Constraint
	Body       Expr
}

func (*FuncDecl) declNode() {}

type Constraint struct {
	Line   int
	Column int
	Name   string
	Args   []TypeExpr
}

type Param struct {
	Line   int
	Column int
	Name   string
	Type   TypeExpr
}

type Field struct {
	Line   int
	Column int
	Name   string
	Type   TypeExpr
}

type TypeExpr interface{ typeNode() }

type NamedType struct {
	Line   int
	Column int
	Name   string
	Args   []TypeExpr
}

func (*NamedType) typeNode() {}

type FuncType struct {
	Line   int
	Column int
	Params []TypeExpr
	Ret    TypeExpr
}

func (*FuncType) typeNode() {}

type Expr interface{ exprNode() }

type Stmt interface{ stmtNode() }

type IdentExpr struct {
	Line   int
	Column int
	Name   string
}

func (*IdentExpr) exprNode() {}

type LiteralExpr struct {
	Line   int
	Column int
	Kind   string
	Value  string
}

func (*LiteralExpr) exprNode() {}

type CallExpr struct {
	Line   int
	Column int
	Callee Expr
	Args   []Expr
}

func (*CallExpr) exprNode() {}

type StructLitExpr struct {
	Line     int
	Column   int
	TypeName string
	TypeArgs []TypeExpr
	Fields   []StructLitField
}

func (*StructLitExpr) exprNode() {}

type StructLitField struct {
	Line   int
	Column int
	Name   string
	Value  Expr
}

type BinaryExpr struct {
	Line   int
	Column int
	Op     string
	Left   Expr
	Right  Expr
}

func (*BinaryExpr) exprNode() {}

type PrefixExpr struct {
	Line   int
	Column int
	Op     string
	Expr   Expr
}

func (*PrefixExpr) exprNode() {}

type FieldExpr struct {
	Line   int
	Column int
	Expr   Expr
	Field  string
}

func (*FieldExpr) exprNode() {}

type FuncLitExpr struct {
	Line   int
	Column int
	Params []Param
	Ret    TypeExpr
	Body   Expr
}

func (*FuncLitExpr) exprNode() {}

type IfExpr struct {
	Line   int
	Column int
	Cond   Expr
	Then   Expr
	Else   Expr
}

func (*IfExpr) exprNode() {}

type SwitchExpr struct {
	Line   int
	Column int
	Target Expr
	Cases  []SwitchCase
}

func (*SwitchExpr) exprNode() {}

type WhileExpr struct {
	Line   int
	Column int
	Cond   Expr
	Body   Expr
}

func (*WhileExpr) exprNode() {}

type BlockExpr struct {
	Line   int
	Column int
	Stmts  []Stmt
}

func (*BlockExpr) exprNode() {}

type ExprStmt struct {
	Line   int
	Column int
	Expr   Expr
}

func (*ExprStmt) stmtNode() {}

type LetStmt struct {
	Line    int
	Column  int
	Mutable bool
	Name    string
	Type    TypeExpr
	Value   Expr
}

func (*LetStmt) stmtNode() {}
func (*LetStmt) declNode() {}

type AssignStmt struct {
	Line   int
	Column int
	Name   string
	Value  Expr
}

func (*AssignStmt) stmtNode() {}

type SwitchCase struct {
	Line    int
	Column  int
	Pattern Pattern
	Body    Expr
}

type Pattern interface{ patternNode() }

type VariantPattern struct {
	Line   int
	Column int
	Name   string
	Args   []string
}

func (*VariantPattern) patternNode() {}

type WildcardPattern struct {
	Line   int
	Column int
}

func (*WildcardPattern) patternNode() {}
