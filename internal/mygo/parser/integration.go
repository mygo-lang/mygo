package parser

func parseFile(src string) (*File, error) {
	p := newParser(src)
	p.pos = 0
	p.skipNL = true
	return p.parseFileRD()
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
	p.currentWhere = nil
	p.currentBlock = nil
	p.currentStmt = nil
	p.currentExpr = nil
	p.currentLeftExpr = nil
	p.currentArgs = nil
	p.currentMapKey = nil
	p.currentMapValue = nil
	p.currentMapEntries = nil
	p.currentSetElems = nil
	p.currentCollectionHasPair = false
	p.currentIfCond = nil
	p.currentIfThen = nil
	p.currentIfElse = nil
	p.currentWhileCond = nil
	p.currentWhileBody = nil
	p.currentSwitchTarget = nil
	p.currentSwitchCases = nil
	p.currentPattern = nil
	p.currentStructFields = nil
	p.currentStructTypeArgs = nil
	p.currentSliceElems = nil
	p.expectTypeSuffix = false
	p.expectStructTypeArgs = false
	p.expectConstraintSuffix = false
	p.currentEnum = nil
	p.currentStruct = nil
	p.currentInterface = nil
	p.currentFunc = nil
	p.needFallback = false
	if yyParse(p) != 0 {
		if p.err != nil {
			return p.err
		}
		return p.err
	}
	if p.needFallback || p.result == nil {
		p.pos = 0
		p.skipNL = true
		p.result, p.err = p.parseFileRD()
		if p.err != nil {
			return p.err
		}
	}
	return nil
}
