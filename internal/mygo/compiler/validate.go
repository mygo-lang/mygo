package compiler

import (
	"fmt"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
	"github.com/mygo-lang/mygo/internal/mygo/pkg"
	"github.com/mygo-lang/mygo/internal/mygo/typeinference"
)

// validator walks all declarations in a package and checks for name/type reference
// errors. It uses the typedInfo from type inference to resolve expression types.

// builtinTypeNames lists all MyGO primitive type names that can appear in
// expression position (e.g. String.FromRunes, Int.From).
var builtinTypeNames = []string{
	"Int", "Int8", "UInt8", "Int16", "UInt16", "Int32", "UInt32",
	"Int64", "UInt", "UInt64", "Float32", "Float64",
	"Byte", "Rune", "String", "Bool", "Unit",
}
type validator struct {
	pkg       *pkg.Package
	typedInfo *typeinference.TypedInfo

	// scope stack
	locals   map[string]struct{} // local variables in current scope
	globals  map[string]struct{} // global declarations
	bindings map[string]struct{} // pattern binding names

	// letBindings tracks names declared with "let" (immutable) to reject
	// assignment attempts. "var" declared names are mutable.
	letBindings map[string]struct{}
}

func newValidator(p *pkg.Package, info *typeinference.TypedInfo) *validator {
	globals := make(map[string]struct{})
	for name := range p.Funcs {
		globals[name] = struct{}{}
	}
	for name := range p.Enums {
		globals[name] = struct{}{}
	}
	for name := range p.Structs {
		globals[name] = struct{}{}
	}
	for name := range p.Interfaces {
		globals[name] = struct{}{}
	}
	// Register import aliases as globals so import "go:fmt" aliases like
	// "fmt" are visible during name resolution.
	for _, decl := range p.Decls {
		imp, ok := decl.(*ImportDecl)
		if !ok || imp.Alias == "." {
			continue
		}
		globals[imp.Alias] = struct{}{}
	}

	// Register prelude enum variant constructors that are handled as
	// built-in identifiers in type inference (see inferIdent).
	for _, name := range []string{"Some", "None", "Ok", "Err", "Ref", "Zero", "TypeKeyFromType"} {
		globals[name] = struct{}{}
	}
	// Register built-in literal identifiers and functions.
	// Register built-in type names that may appear in expression position.
	for _, name := range builtinTypeNames {
		globals[name] = struct{}{}
	}

	for _, name := range []string{"true", "false", "break", "continue", "len", "append", "copy", "delete"} {
		globals[name] = struct{}{}
	}

	return &validator{
		pkg:         p,
		typedInfo:   info,
		globals:     globals,
		locals:      make(map[string]struct{}),
		bindings:    make(map[string]struct{}),
		letBindings: make(map[string]struct{}),
	}
}

// Validate checks all declarations in a package for name and type reference errors.
// It must be called after type inference and before code generation.
func Validate(p *pkg.Package, typedInfo *typeinference.TypedInfo) error {
	if p == nil {
		return common.ErrorAtPos("", 0, 0, "cannot validate nil package")
	}
	v := newValidator(p, typedInfo)

	for _, decl := range p.Decls {
		if err := v.validateDecl(decl); err != nil {
			return err
		}
	}
	return nil
}

func (v *validator) validateDecl(decl Decl) error {
	switch d := decl.(type) {
	case *ImportDecl:
		return v.validateImport(d)
	case *EnumDecl:
		return v.validateEnum(d)
	case *StructDecl:
		return v.validateStruct(d)
	case *InterfaceDecl:
		return v.validateInterface(d)
	case *ImplDecl:
		return v.validateImpl(d)
	case *FuncDecl:
		return v.validateFunc(d)
	case *LetStmt:
		return v.validateLet(d)
	}
	return nil
}

// ---- Imports ----
func (v *validator) validateImport(d *ImportDecl) error {
	if d.Path == "" {
		return common.ErrorAtNode(d.SourceFile, d, "import path is empty")
	}
	return nil
}

// ---- Enums ----
func (v *validator) validateEnum(d *EnumDecl) error {
	if err := v.checkDuplicateTypeParam(d.Name, d.TypeParams); err != nil {
		return err
	}
	return nil
}

// ---- Structs ----
func (v *validator) validateStruct(d *StructDecl) error {
	if err := v.checkDuplicateTypeParam(d.Name, d.TypeParams); err != nil {
		return err
	}
	seenFields := make(map[string]struct{})
	for _, f := range d.Fields {
		if _, ok := seenFields[f.Name]; ok {
			return common.ErrorAtNode(d.Name, f, "duplicate field %q in struct %s", f.Name, d.Name)
		}
		seenFields[f.Name] = struct{}{}
		if err := v.validateField(f); err != nil {
			return err
		}
	}
	return nil
}

// ---- Interfaces ----
func (v *validator) validateInterface(d *InterfaceDecl) error {
	return v.checkDuplicateTypeParam(d.Name, d.TypeParams)
}

// ---- Impl blocks ----
func (v *validator) validateImpl(d *ImplDecl) error {
	if d.Type != nil {
		if err := v.validateTypeExpr(d.Type); err != nil {
			return err
		}
	}
	for _, arg := range d.TypeArgs {
		if err := v.validateTypeExpr(arg); err != nil {
			return err
		}
	}
	for _, arg := range d.InterfaceArgs {
		if err := v.validateTypeExpr(arg); err != nil {
			return err
		}
	}
	for _, m := range d.Methods {
		if err := v.validateFunc(m); err != nil {
			return err
		}
	}
	return nil
}

// ---- Functions ----
func (v *validator) validateFunc(d *FuncDecl) error {
	if err := v.checkDuplicateTypeParam(d.Name, d.TypeParams); err != nil {
		return err
	}
	// Push function scope
	prevLocals := v.locals
	prevBindings := v.bindings
	v.locals = make(map[string]struct{})
	v.bindings = make(map[string]struct{})

	for _, p := range d.Params {
		if err := v.validateParam(p); err != nil {
			return err
		}
		v.locals[p.Name] = struct{}{}
	}
	if d.Ret != nil {
		if err := v.validateTypeExpr(d.Ret); err != nil {
			return err
		}
	}
	for _, c := range d.Using {
		for _, a := range c.Args {
			if err := v.validateTypeExpr(a); err != nil {
				return err
			}
		}
	}
	if d.Body != nil {
		if err := v.validateExpr(d.Body); err != nil {
			return err
		}
	}

	// Restore scope
	v.locals = prevLocals
	v.bindings = prevBindings
	return nil
}

// ---- Let statements (also used as top-level decls) ----
func (v *validator) validateLet(d *LetStmt) error {
	// First validate the value expression (binding name is not yet visible).
	if d.Value != nil {
		if err := v.validateExpr(d.Value); err != nil {
			return err
		}
	}

	// Validate type annotation if present.
	if d.Type != nil {
		if err := v.validateTypeExpr(d.Type); err != nil {
			return err
		}
	}

	// Now register binding name(s) so subsequent uses can reference them.
	if d.Bind != nil {
		if err := v.collectBindPatternNames(d.Bind, d.Mutable); err != nil {
			return err
		}
	} else if d.Name != "_" {
		v.locals[d.Name] = struct{}{}
		if !d.Mutable {
			v.letBindings[d.Name] = struct{}{}
		}
	}

	if d.Mutable {
		// var: must have initial value
		if d.Value == nil {
			return common.ErrorAtNode(d.SourceFile, d, "var %q requires initial value", d.Name)
		}
	}
	return nil
}

// ---- Type expression validation ----
func (v *validator) validateTypeExpr(t TypeExpr) error {
	if t == nil {
		return nil
	}
	switch tt := t.(type) {
	case *NamedType:
		if tt.Name == "" {
			return common.ErrorAtNode("", t, "empty type name")
		}
		for _, arg := range tt.Args {
			if err := v.validateTypeExpr(arg); err != nil {
				return err
			}
		}
	case *FuncType:
		for _, p := range tt.Params {
			if err := v.validateTypeExpr(p); err != nil {
				return err
			}
		}
		if tt.Ret != nil {
			if err := v.validateTypeExpr(tt.Ret); err != nil {
				return err
			}
		}
	case *TupleType:
		for _, e := range tt.Elems {
			if err := v.validateTypeExpr(e); err != nil {
				return err
			}
		}
	}
	return nil
}

// ---- Expression validation ----
func (v *validator) validateExpr(e Expr) error {
	if e == nil {
		return nil
	}
	switch x := e.(type) {
	case *IdentExpr:
		return v.validateIdent(x)
	case *LiteralExpr:
		return nil
	case *CallExpr:
		return v.validateCall(x)
	case *BinaryExpr:
		return v.validateBinary(x)
	case *PrefixExpr:
		return v.validatePrefix(x)
	case *FieldExpr:
		return v.validateFieldExpr(x)
	case *StructLitExpr:
		return v.validateStructLit(x)
	case *IfExpr:
		return v.validateIf(x)
	case *SwitchExpr:
		return v.validateSwitch(x)
	case *WhileExpr:
		return v.validateWhile(x)
	case *FuncLitExpr:
		return v.validateFuncLit(x)
	case *SliceLitExpr:
		return v.validateSliceLit(x)
	case *MapLitExpr:
		return v.validateMapLit(x)
	case *SetLitExpr:
		return v.validateSetLit(x)
	case *TupleLitExpr:
		return v.validateTupleLit(x)
	case *UnitLitExpr:
		return nil
	case *GoExpr:
		return v.validateGoExpr(x)
	case *BlockExpr:
		return v.validateBlock(x)
	case *CastExpr:
		return v.validateCast(x)
	}
	return nil
}

func (v *validator) validateIdent(x *IdentExpr) error {
	if x.Name == "_" {
		return nil
	}
	// Check locals, then globals
	_, isLocal := v.locals[x.Name]
	_, isBinding := v.bindings[x.Name]
	_, isGlobal := v.globals[x.Name]
	if !isLocal && !isGlobal && !isBinding {
		return common.ErrorAtNode(x.SourceFile, x, "undefined variable %q", x.Name)
	}
	return nil
}

func (v *validator) validateFieldExpr(x *FieldExpr) error {
	if err := v.validateExpr(x.Expr); err != nil {
		return err
	}
	return nil
}

func (v *validator) validateCall(x *CallExpr) error {
	if x.Callee != nil {
		if err := v.validateExpr(x.Callee); err != nil {
			return err
		}
	}
	for _, t := range x.TypeArgs {
		if err := v.validateTypeExpr(t); err != nil {
			return err
		}
	}
	for _, a := range x.Args {
		if err := v.validateExpr(a); err != nil {
			return err
		}
	}
	return nil
}

func (v *validator) validateBinary(x *BinaryExpr) error {
	if err := v.validateExpr(x.Left); err != nil {
		return err
	}
	return v.validateExpr(x.Right)
}

func (v *validator) validatePrefix(x *PrefixExpr) error {
	return v.validateExpr(x.Expr)
}

func (v *validator) validateStructLit(x *StructLitExpr) error {
	// If we have a struct type, validate field names
	st := v.pkg.Structs[x.TypeName]
	if st != nil && len(x.Fields) > 0 {
		fieldSet := make(map[string]struct{}, len(st.Fields))
		for _, sf := range st.Fields {
			fieldSet[sf.Name] = struct{}{}
		}
		for _, f := range x.Fields {
			if _, ok := fieldSet[f.Name]; !ok {
				return common.ErrorAtNode(x.SourceFile, f, "struct %s has no field %q", x.TypeName, f.Name)
			}
			if err := v.validateExpr(f.Value); err != nil {
				return err
			}
		}
	}
	for _, t := range x.TypeArgs {
		if err := v.validateTypeExpr(t); err != nil {
			return err
		}
	}
	return nil
}

func (v *validator) validateIf(x *IfExpr) error {
	if err := v.validateExpr(x.Cond); err != nil {
		return err
	}
	if err := v.validateExpr(x.Then); err != nil {
		return err
	}
	if x.Else != nil {
		if err := v.validateExpr(x.Else); err != nil {
			return err
		}
	}
	return nil
}

func (v *validator) validateSwitch(x *SwitchExpr) error {
	if x.Target != nil {
		if err := v.validateExpr(x.Target); err != nil {
			return err
		}
	}
	// Determine the enum type of the target from typedInfo for variant checking
	var enumDecl *EnumDecl
	if x.Target != nil && v.typedInfo != nil {
		if mt, ok := v.typedInfo.ExprTypes[x.Target]; ok {
			enumDecl = v.findEnumForType(mt)
		}
	}
	for _, c := range x.Cases {
		if err := v.validatePattern(c.Pattern, enumDecl); err != nil {
			return err
		}
		// Register pattern binding names (e.g. "Some(node)" registers "node")
		// in the current scope before validating the case body.
		v.collectPatternBindings(c.Pattern)

		if c.Body != nil {
			if err := v.validateExpr(c.Body); err != nil {
				return err
			}
		}
	}
	return nil
}

func (v *validator) findEnumForType(mt typeinference.MonoType) *EnumDecl {
	tc, ok := mt.(typeinference.TCon)
	if !ok {
		return nil
	}
	enum, ok := v.pkg.Enums[tc.Name]
	if ok {
		return enum
	}
	// Also check imported enums (merged into pkg.Enums via mergeImportedDecls)
	return nil
}

func (v *validator) validatePattern(p Pattern, enumDecl *EnumDecl) error {
	if p == nil {
		return nil
	}
	switch pt := p.(type) {
	case *VariantPattern:
		if enumDecl != nil {
			found := false
			for _, variant := range enumDecl.Variants {
				if variant.Name == pt.Name {
					found = true
					break
				}
			}
			if !found {
				return common.ErrorAtNode(pt.SourceFile, pt, "enum %s has no variant %q", enumDecl.Name, pt.Name)
			}
		}
	case *LiteralPattern:
		// valid
	case *WildcardPattern:
		// valid
	case *TuplePattern:
		for _, e := range pt.Elems {
			if err := v.validatePattern(e, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

func (v *validator) validateWhile(x *WhileExpr) error {
	if err := v.validateExpr(x.Cond); err != nil {
		return err
	}
	return v.validateExpr(x.Body)
}

func (v *validator) validateFuncLit(x *FuncLitExpr) error {
	// Push a new scope for the function literal.
	// Inherit all locals from enclosing scope (closures capture outer variables).
	prevLocals := v.locals
	locals := make(map[string]struct{}, len(prevLocals))
	for k, vv := range prevLocals {
		locals[k] = vv
	}
	v.locals = locals

	for _, p := range x.Params {
		if err := v.validateParam(p); err != nil {
			return err
		}
		v.locals[p.Name] = struct{}{}
	}
	if x.Ret != nil {
		if err := v.validateTypeExpr(x.Ret); err != nil {
			return err
		}
	}
	if x.Body != nil {
		if err := v.validateExpr(x.Body); err != nil {
			return err
		}
	}

	v.locals = prevLocals
	return nil
}

func (v *validator) validateSliceLit(x *SliceLitExpr) error {
	if x.Elem != nil {
		if err := v.validateTypeExpr(x.Elem); err != nil {
			return err
		}
	}
	for _, e := range x.Elems {
		if err := v.validateExpr(e); err != nil {
			return err
		}
	}
	return nil
}

func (v *validator) validateMapLit(x *MapLitExpr) error {
	if x.Key != nil {
		if err := v.validateTypeExpr(x.Key); err != nil {
			return err
		}
	}
	if x.Val != nil {
		if err := v.validateTypeExpr(x.Val); err != nil {
			return err
		}
	}
	for _, p := range x.Pairs {
		if err := v.validateExpr(p.Key); err != nil {
			return err
		}
		if err := v.validateExpr(p.Value); err != nil {
			return err
		}
	}
	return nil
}

func (v *validator) validateSetLit(x *SetLitExpr) error {
	if x.Elem != nil {
		if err := v.validateTypeExpr(x.Elem); err != nil {
			return err
		}
	}
	for _, e := range x.Elems {
		if err := v.validateExpr(e); err != nil {
			return err
		}
	}
	return nil
}

func (v *validator) validateTupleLit(x *TupleLitExpr) error {
	for _, e := range x.Elems {
		if err := v.validateExpr(e); err != nil {
			return err
		}
	}
	return nil
}

func (v *validator) validateGoExpr(x *GoExpr) error {
	if x.Result != nil {
		if err := v.validateTypeExpr(x.Result); err != nil {
			return err
		}
	}
	for _, op := range x.Operands {
		if op.Value != nil {
			if err := v.validateExpr(op.Value); err != nil {
				return err
			}
		}
	}
	for _, op := range x.TypeOperands {
		if op.Type != nil {
			if err := v.validateTypeExpr(op.Type); err != nil {
				return err
			}
		}
	}
	return nil
}

func (v *validator) validateBlock(x *BlockExpr) error {
	for _, s := range x.Stmts {
		if err := v.validateStmt(s); err != nil {
			return err
		}
	}
	return nil
}

func (v *validator) validateCast(x *CastExpr) error {
	if err := v.validateExpr(x.Expr); err != nil {
		return err
	}
	if x.Type != nil {
		if err := v.validateTypeExpr(x.Type); err != nil {
			return err
		}
	}
	return nil
}

// ---- Statement validation ----
func (v *validator) validateStmt(s Stmt) error {
	if s == nil {
		return nil
	}
	switch st := s.(type) {
	case *ExprStmt:
		return v.validateExpr(st.Expr)
	case *LetStmt:
		return v.validateLet(st)
	case *ReturnStmt:
		return v.validateReturn(st)
	case *AssignStmt:
		return v.validateAssign(st)
	}
	return nil
}

func (v *validator) validateReturn(s *ReturnStmt) error {
	if s.Value == nil {
		return nil
	}
	return v.validateExpr(s.Value)
}

func (v *validator) validateAssign(s *AssignStmt) error {
	// Check that the target is not a "let" (immutable) binding.
	if s.Name != "" {
		if _, isLet := v.letBindings[s.Name]; isLet {
			return common.ErrorAtNode(s.SourceFile, s,
				"immutable binding %q cannot be assigned", s.Name)
		}
	}

	if s.Value == nil {
		return common.ErrorAtNode(s.SourceFile, s, "assignment requires a value")
	}
	if err := v.validateExpr(s.Value); err != nil {
		return err
	}
	return nil
}

// ---- Helpers ----
func (v *validator) validateParam(p Param) error {
	if p.Type != nil {
		return v.validateTypeExpr(p.Type)
	}
	return nil
}

func (v *validator) validateField(f Field) error {
	if f.Type != nil {
		return v.validateTypeExpr(f.Type)
	}
	return nil
}

func (v *validator) checkDuplicateTypeParam(declName string, params []string) error {
	seen := make(map[string]struct{})
	for _, p := range params {
		if _, ok := seen[p]; ok {
			return common.ErrorAtPos(declName, 0, 0, "duplicate type parameter %q in %s", p, declName)
		}
		seen[p] = struct{}{}
	}
	return nil
}

// collectBindPatternNames registers all names from a destructuring bind pattern.
func (v *validator) collectBindPatternNames(p BindPattern, mutable bool) error {
	switch pat := p.(type) {
	case *BindNamePattern:
		if pat.Name == "_" {
			return nil
		}
		v.locals[pat.Name] = struct{}{}
		if !mutable {
			v.letBindings[pat.Name] = struct{}{}
		}
		return nil
	case *BindTuplePattern:
		for _, elem := range pat.Elems {
			if err := v.collectBindPatternNames(elem, mutable); err != nil {
				return err
			}
		}
		return nil
	default:
		return nil
	}
}

// collectPatternBindings registers all names bound in a pattern
// (e.g. variant bindings like "Some(node)") into the current locals scope.
func (v *validator) collectPatternBindings(p Pattern) {
	if p == nil {
		return
	}
	switch pt := p.(type) {
	case *VariantPattern:
		// Variant patterns like Some(node) — register the inner bindings.
		for _, arg := range pt.Args {
			if arg == "_" {
				continue
			}
			if arg != "" {
				v.locals[arg] = struct{}{}
			}
		}
	case *TuplePattern:
		for _, elem := range pt.Elems {
			v.collectPatternBindings(elem)
		}
	case *WildcardPattern:
		// Nothing to register.
	case *LiteralPattern:
		// Nothing to register.
	}
}

// ensure common.ErrorAtNode exists — it's used above
func init() {
	_ = fmt.Sprintf // ensure fmt is used (for possible future use)
}
