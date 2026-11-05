package parser

import (
	"os"

	"github.com/mygo-lang/mygo/internal/mygo/common"
)

func parseFile(filename, src string) (*File, error) {
	if err := rejectLegacyInlineGoRaw(filename, src); err != nil {
		return nil, err
	}
	p := newParser(filename, src)
	if err := p.parseWithYacc(); err != nil {
		return nil, err
	}
	setASTSourceFile(p.result, filename)
	return p.result, nil
}

// The legacy pipeline predates the explicit inline-Go raw result shape. It
// rejects the field before goyacc sees it so callers get a targeted message
// instead of a generic syntax error near the following type.
func rejectLegacyInlineGoRaw(filename, src string) error {
	for i := 0; i+3 < len(src); i++ {
		if src[i] != 'r' || src[i+1] != 'a' || src[i+2] != 'w' {
			continue
		}
		if i > 0 && (isIdentByte(src[i-1])) {
			continue
		}
		j := i + 3
		for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
			j++
		}
		if j >= len(src) || src[j] != ':' {
			continue
		}
		if j+1 < len(src) && isIdentByte(src[j+1]) {
			continue
		}
		line, column := 1, 1
		for k := 0; k < i; k++ {
			if src[k] == '\n' {
				line++
				column = 1
			} else {
				column++
			}
		}
		return common.ErrorAtPos(filename, line, column, "inline-Go raw declarations are not supported by --legacy")
	}
	return nil
}

func isIdentByte(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

func (p *parser) parseWithYacc() error {
	p.pos = 0
	p.skipNL = true
	p.err = nil
	p.result = nil
	p.packageName = ""
	p.packageLine = 0
	p.packageColumn = 0
	p.decls = nil
	p.currentName = ""
	p.currentNameLine = 0
	p.currentNameCol = 0
	p.currentType = nil
	p.currentTypeLine = 0
	p.currentTypeCol = 0
	p.currentTypeParams = nil
	p.currentParams = nil
	p.currentParamsStack = nil
	p.currentWhere = nil
	p.currentConstraintArgs = nil
	p.currentBlock = nil
	p.currentBlockStack = nil
	p.currentStmt = nil
	p.currentAssignTarget = nil
	p.currentExpr = nil
	p.currentLeftExpr = nil
	p.currentPipeLeftExpr = nil
	p.currentArgs = nil
	p.currentCallCalleeStack = nil
	p.currentArgsStack = nil
	p.currentSliceElemsStack = nil
	p.currentMapKey = nil
	p.currentMapValue = nil
	p.currentMapEntries = nil
	p.currentSetElems = nil
	p.currentEnumFields = nil
	p.currentCollectionHasPair = false
	p.currentIfCond = nil
	p.currentIfThen = nil
	p.currentIfElse = nil
	p.currentWhileCondStack = nil
	p.currentSwitchTarget = nil
	p.currentSwitchCases = nil
	p.currentPattern = nil
	p.currentPatternArgs = nil
	p.currentStructFields = nil
	p.currentStructTypeArgs = nil
	p.currentTypeArgStack = nil
	p.currentFuncTypeParamStack = nil
	p.funcTypeParamDepth = 0
	p.currentImplTypeParams = nil
	p.currentImplType = nil
	p.currentImplInterfaceArgs = nil
	p.currentImplLine = 0
	p.currentImplCol = 0
	p.currentSliceElems = nil
	p.currentConstraintBindName = ""
	p.savedTypeNameStack = nil
	p.savedStructTypeArgs = nil
	p.savedDeclName = ""
	p.expectTypeSuffix = false
	p.expectStructTypeArgs = false
	p.expectConstraintSuffix = false
	p.parsingImplTypeParams = false
	p.currentEnum = nil
	p.currentStruct = nil
	p.currentInterface = nil
	p.currentImpl = nil
	p.currentFunc = nil
	// Reset package-level yacc diagnostics for every parse. This keeps debug
	// output opt-in even when a previous parse enabled MYGO_PARSER_DEBUG.
	yyDebug = 0
	yyErrorVerbose = false
	if os.Getenv("MYGO_PARSER_DEBUG") != "" {
		yyDebug = 4
		yyErrorVerbose = true
	}
	if yyParse(p) != 0 {
		if p.err != nil {
			return p.err
		}
		return p.err
	}
	return nil
}
