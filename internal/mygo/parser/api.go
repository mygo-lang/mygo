package parser

import (
	"strconv"

	"github.com/mygo-lang/mygo/internal/mygo/ast"
)

type File = ast.File
type Decl = ast.Decl
type ImportDecl = ast.ImportDecl
type EnumDecl = ast.EnumDecl
type EnumVariant = ast.EnumVariant
type StructDecl = ast.StructDecl
type InterfaceDecl = ast.InterfaceDecl
type ImplDecl = ast.ImplDecl
type FuncDecl = ast.FuncDecl
type Constraint = ast.Constraint
type Param = ast.Param
type Field = ast.Field
type TypeExpr = ast.TypeExpr
type NamedType = ast.NamedType
type FuncType = ast.FuncType
type TupleType = ast.TupleType
type Expr = ast.Expr
type Stmt = ast.Stmt
type IdentExpr = ast.IdentExpr
type LiteralExpr = ast.LiteralExpr
type CallExpr = ast.CallExpr
type StructLitExpr = ast.StructLitExpr
type StructLitField = ast.StructLitField
type BinaryExpr = ast.BinaryExpr
type PrefixExpr = ast.PrefixExpr
type FieldExpr = ast.FieldExpr
type FuncLitExpr = ast.FuncLitExpr
type IfExpr = ast.IfExpr
type SwitchExpr = ast.SwitchExpr
type WhileExpr = ast.WhileExpr
type BlockExpr = ast.BlockExpr
type ExprStmt = ast.ExprStmt
type LetStmt = ast.LetStmt
type BindPattern = ast.BindPattern
type BindNamePattern = ast.BindNamePattern
type BindTuplePattern = ast.BindTuplePattern
type AssignStmt = ast.AssignStmt
type SwitchCase = ast.SwitchCase
type Pattern = ast.Pattern
type VariantPattern = ast.VariantPattern
type TuplePattern = ast.TuplePattern
type WildcardPattern = ast.WildcardPattern
type SliceLitExpr = ast.SliceLitExpr
type MapLitExpr = ast.MapLitExpr
type MapLitPair = ast.MapLitPair
type SetLitExpr = ast.SetLitExpr
type TupleLitExpr = ast.TupleLitExpr
type UnitLitExpr = ast.UnitLitExpr
type GoExpr = ast.GoExpr
type GoOperand = ast.GoOperand
type GoTypeOperand = ast.GoTypeOperand

func ParseFile(filename, src string) (*File, error)                { return parseFile(filename, src) }
func ParseFiles(srcs map[string]string) ([]*File, error) { return parseFiles(srcs) }
func MustParseInt(s string) int                          { n, _ := strconv.Atoi(s); return n }
