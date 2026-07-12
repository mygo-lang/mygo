package parser

import "os"

func parseFile(src string) (*File, error) {
	p := newParser(src)
	if err := p.parseWithYacc(); err != nil {
		return nil, err
	}
	return p.result, nil
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
	p.currentWhileCond = nil
	p.currentWhileBody = nil
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
