package parser

import (
	"fmt"

	"github.com/mygo-lang/mygo/internal/mygo/ast"
)

type parser struct {
	toks                      []token
	pos                       int
	skipNL                    bool
	err                       error
	result                    *ast.File
	packageName               string
	packageLine               int
	packageColumn             int
	decls                     []Decl
	currentName               string
	currentNameLine           int
	currentNameCol            int
	currentType               TypeExpr
	currentTypeLine           int
	currentTypeCol            int
	currentTypeParams         []string
	currentParams             []ast.Param
	currentWhere              []ast.Constraint
	currentConstraintArgs     []TypeExpr
	currentBlock              []ast.Stmt
	currentBlockStack         [][]ast.Stmt
	currentStmt               ast.Stmt
	currentExpr               ast.Expr
	currentLeftExpr           ast.Expr
	currentPipeLeftExpr       ast.Expr
	currentArgs               []ast.Expr
	currentCallCalleeStack    []ast.Expr
	currentArgsStack          [][]ast.Expr
	currentSliceElemsStack    [][]ast.Expr
	currentMapKey             ast.Expr
	currentMapValue           ast.Expr
	currentMapEntries         []ast.MapLitPair
	currentSetElems           []ast.Expr
	currentEnumFields         []ast.Field
	currentCollectionHasPair  bool
	currentIfCond             ast.Expr
	currentIfThen             ast.Expr
	currentIfElse             ast.Expr
	currentWhileCond          ast.Expr
	currentWhileBody          ast.Expr
	currentSwitchTarget       ast.Expr
	currentSwitchCases        []ast.SwitchCase
	currentPattern            ast.Pattern
	currentPatternArgs        []string
	currentStructFields       []ast.StructLitField
	currentStructTypeArgs     []ast.TypeExpr
	currentTypeArgStack       [][]ast.TypeExpr
	currentFuncTypeParamStack [][]ast.TypeExpr
	funcTypeParamDepth        int
	currentImplTypeParams     []string
	currentImplType           TypeExpr
	currentImplInterfaceArgs  []ast.TypeExpr
	currentImplLine           int
	currentImplCol            int
	currentSliceElems         []ast.Expr
	currentConstraintBindName string
	savedDeclName             string
	savedTypeNameStack        []typeNameEntry
	savedStructTypeArgs       []ast.TypeExpr
	expectTypeSuffix          bool
	expectStructTypeArgs      bool
	expectConstraintSuffix    bool
	parsingImplTypeParams     bool
	currentEnum               *ast.EnumDecl
	currentStruct             *ast.StructDecl
	currentInterface          *ast.InterfaceDecl
	currentImpl               *ast.ImplDecl
	currentFunc               *ast.FuncDecl
}

type typeNameEntry struct {
	name string
	line int
	col  int
	args []TypeExpr
}

func parseFiles(srcs map[string]string) ([]*ast.File, error) {
	files := make([]*ast.File, 0, len(srcs))
	for path, src := range srcs {
		f, err := parseFile(src)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		files = append(files, f)
	}
	return files, nil
}

func newParser(src string) *parser {
	l := newLexer(src)
	var toks []token
	for {
		tok := l.nextToken()
		toks = append(toks, tok)
		if tok.kind == tokEOF {
			break
		}
	}
	return &parser{toks: toks, skipNL: true}
}

func (p *parser) peek() token {
	if p.skipNL {
		p.skipNewlines()
	}
	return p.peekRaw()
}

func (p *parser) peekRaw() token {
	if p.pos >= len(p.toks) {
		return token{kind: tokEOF, line: p.lineAtEOF()}
	}
	return p.toks[p.pos]
}

func (p *parser) lineAtEOF() int {
	if len(p.toks) == 0 {
		return 1
	}
	return p.toks[len(p.toks)-1].line
}

func (p *parser) nextRaw() token {
	tok := p.peekRaw()
	if p.pos < len(p.toks) {
		p.pos++
	}
	return tok
}

func (p *parser) skipNewlines() {
	for p.peekRaw().kind == tokNewline {
		p.pos++
	}
}
