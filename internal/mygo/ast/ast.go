package ast

type File struct {
	PackageName   string
	PackageLine   int
	PackageColumn int
	Decls         []Decl
}

type Decl interface{ declNode() }

type ImportDecl struct {
	Line       int
	Column     int
	SourceFile string
	Alias      string
	Path       string
}

func (*ImportDecl) declNode() {}

type EnumDecl struct {
	Line       int
	Column     int
	SourceFile string
	Name       string
	TypeParams []string
	Variants   []EnumVariant
}

func (*EnumDecl) declNode() {}

type EnumVariant struct {
	Line       int
	Column     int
	SourceFile string
	Name       string
	Fields     []Field
}

type StructDecl struct {
	Line       int
	Column     int
	SourceFile string
	Name       string
	TypeParams []string
	Fields     []Field
}

func (*StructDecl) declNode() {}

type InterfaceDecl struct {
	Line       int
	Column     int
	SourceFile string
	Name       string
	TypeParams []string
	Methods    []*FuncDecl
}

func (*InterfaceDecl) declNode() {}

type ImplDecl struct {
	Line          int
	Column        int
	SourceFile    string
	Name          string     // interface name (e.g. "Enumerable", for old-style "impl Show[Int]")
	TypeArgs      []TypeExpr // interface type args (e.g. [Int], for old-style)
	InterfaceName string     // interface name (e.g. "Enumerable", for new-style "impl List[T]: Enumerable[T]")
	InterfaceArgs []TypeExpr // interface type args (e.g. [T], for new-style)
	Type          TypeExpr   // the type being implemented on (e.g. List[T], for new-style)
	TypeParams    []string   // impl-level type params (e.g. [T], for new-style)
	Methods       []*FuncDecl
}

func (*ImplDecl) declNode() {}

type FuncDecl struct {
	Line       int
	Column     int
	SourceFile string
	Name       string
	TypeParams []string
	Params     []Param
	Ret        TypeExpr
	Using      []Constraint
	Body       Expr
}

func (*FuncDecl) declNode() {}

type Constraint struct {
	Line       int
	Column     int
	SourceFile string
	Name       string
	Args       []TypeExpr
	BindName   string // optional named binding, e.g. "intShow" in "using intShow: Show[Int]"
}

type Param struct {
	Line       int
	Column     int
	SourceFile string
	Name       string
	Type       TypeExpr
}

type Field struct {
	Line       int
	Column     int
	SourceFile string
	Name       string
	Type       TypeExpr
	Tag        string
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

type TupleType struct {
	Line   int
	Column int
	Elems  []TypeExpr
}

func (*TupleType) typeNode() {}

type Expr interface{ exprNode() }

type Stmt interface{ stmtNode() }

type IdentExpr struct {
	Line       int
	Column     int
	SourceFile string
	Name       string
}

func (*IdentExpr) exprNode() {}

type LiteralExpr struct {
	Line       int
	Column     int
	SourceFile string
	Kind       string
	Value      string
}

func (*LiteralExpr) exprNode() {}

type CallExpr struct {
	Line       int
	Column     int
	SourceFile string
	Callee     Expr
	TypeArgs   []TypeExpr // 泛型类型参数，如 Func[T](args)
	Args       []Expr
}

func (*CallExpr) exprNode() {}

type StructLitExpr struct {
	Line       int
	Column     int
	SourceFile string
	TypeName   string
	TypeArgs   []TypeExpr
	Fields     []StructLitField
}

func (*StructLitExpr) exprNode() {}

type StructLitField struct {
	Line       int
	Column     int
	SourceFile string
	Name       string
	Value      Expr
}

type BinaryExpr struct {
	Line       int
	Column     int
	SourceFile string
	Op         string
	Left       Expr
	Right      Expr
}

func (*BinaryExpr) exprNode() {}

type PrefixExpr struct {
	Line       int
	Column     int
	SourceFile string
	Op         string
	Expr       Expr
}

func (*PrefixExpr) exprNode() {}

type CastExpr struct {
	Line       int
	Column     int
	SourceFile string
	Expr       Expr
	Type       TypeExpr
}

func (*CastExpr) exprNode() {}

type FieldExpr struct {
	Line       int
	Column     int
	SourceFile string
	Expr       Expr
	Field      string
}

func (*FieldExpr) exprNode() {}

type FuncLitExpr struct {
	Line       int
	Column     int
	SourceFile string
	Params     []Param
	Ret        TypeExpr
	Body       Expr
}

func (*FuncLitExpr) exprNode() {}

type IfExpr struct {
	Line       int
	Column     int
	SourceFile string
	Cond       Expr
	Then       Expr
	Else       Expr
}

func (*IfExpr) exprNode() {}

type SwitchExpr struct {
	Line       int
	Column     int
	SourceFile string
	Target     Expr
	Cases      []SwitchCase
}

func (*SwitchExpr) exprNode() {}

type WhileExpr struct {
	Line       int
	Column     int
	SourceFile string
	Cond       Expr
	Body       Expr
}

func (*WhileExpr) exprNode() {}

type SliceLitExpr struct {
	Line       int
	Column     int
	SourceFile string
	Elem       TypeExpr
	Elems      []Expr
}

func (*SliceLitExpr) exprNode() {}

type MapLitExpr struct {
	Line       int
	Column     int
	SourceFile string
	Key        TypeExpr
	Val        TypeExpr
	Pairs      []MapLitPair
}

func (*MapLitExpr) exprNode() {}

type MapLitPair struct {
	Line       int
	Column     int
	SourceFile string
	Key        Expr
	Value      Expr
}

type SetLitExpr struct {
	Line       int
	Column     int
	SourceFile string
	Elem       TypeExpr
	Elems      []Expr
}

func (*SetLitExpr) exprNode() {}

type TupleLitExpr struct {
	Line       int
	Column     int
	SourceFile string
	Elems      []Expr
}

func (*TupleLitExpr) exprNode() {}

type UnitLitExpr struct {
	Line       int
	Column     int
	SourceFile string
}

func (*UnitLitExpr) exprNode() {}

type GoExpr struct {
	Line         int
	Column       int
	SourceFile   string
	Result       TypeExpr
	Code         string
	Operands     []GoOperand
	TypeOperands []GoTypeOperand
}

func (*GoExpr) exprNode() {}

type GoOperand struct {
	Line       int
	Column     int
	SourceFile string
	Name       string
	Value      Expr
}

type GoTypeOperand struct {
	Line       int
	Column     int
	SourceFile string
	Name       string
	Type       TypeExpr
}

type BlockExpr struct {
	Line       int
	Column     int
	SourceFile string
	Stmts      []Stmt
}

func (*BlockExpr) exprNode() {}

type ExprStmt struct {
	Line       int
	Column     int
	SourceFile string
	Expr       Expr
}

func (*ExprStmt) stmtNode() {}

type LetStmt struct {
	Line       int
	Column     int
	SourceFile string
	Mutable    bool
	Name       string
	Names      []string
	Bind       BindPattern
	Type       TypeExpr
	Value      Expr
}

func (*LetStmt) stmtNode() {}

type LetRecStmt struct {
	Line       int
	Column     int
	SourceFile string
	Bindings   []LetRecBinding
}

type LetRecBinding struct {
	Line       int
	Column     int
	SourceFile string
	Name       string
	Type       TypeExpr
	Value      Expr
}

func (*LetRecStmt) stmtNode() {}

type ReturnStmt struct {
	Line       int
	Column     int
	SourceFile string
	Value      Expr
}

func (*ReturnStmt) stmtNode() {}
func (*LetStmt) declNode()    {}

type AssignStmt struct {
	Line       int
	Column     int
	SourceFile string
	Name       string
	Value      Expr
}

func (*AssignStmt) stmtNode() {}

type SwitchCase struct {
	Line       int
	Column     int
	SourceFile string
	Pattern    Pattern
	Body       Expr
}

type Pattern interface{ patternNode() }

type BindPattern interface{ bindPatternNode() }

type BindNamePattern struct {
	Line       int
	Column     int
	SourceFile string
	Name       string
}

func (*BindNamePattern) bindPatternNode() {}

type BindTuplePattern struct {
	Line       int
	Column     int
	SourceFile string
	Elems      []BindPattern
}

func (*BindTuplePattern) bindPatternNode() {}

type VariantPattern struct {
	Line       int
	Column     int
	SourceFile string
	Name       string
	Args       []string
}

func (*VariantPattern) patternNode() {}

type LiteralPattern struct {
	Line       int
	Column     int
	SourceFile string
	Kind       string
	Value      string
}

func (*LiteralPattern) patternNode() {}

type TuplePattern struct {
	Line       int
	Column     int
	SourceFile string
	Elems      []Pattern
}

func (*TuplePattern) patternNode() {}

type WildcardPattern struct {
	Line       int
	Column     int
	SourceFile string
}

func (*WildcardPattern) patternNode() {}
