package compiler

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	jen "github.com/dave/jennifer/jen"
	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
)

var goPlaceholderRE = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)\}`)

func (g *generator) translateGoExpr(n *GoExpr, ctx *exprCtx, expected string) (jen.Code, string, error) {
	_ = expected
	operands := map[string]string{}

	// Resolve value operands (in name = expr)
	for _, op := range n.Operands {
		if _, exists := operands[op.Name]; exists {
			return nil, "", common.ErrorAtPos(op.Line, op.Column, "duplicate go operand %q", op.Name)
		}
		code, _, err := g.translateExpr(op.Value, ctx, "")
		if err != nil {
			return nil, "", common.ErrorAtPos(op.Line, op.Column, "go operand %q: %w", op.Name, err)
		}
		rendered, err := renderGoOperand(code)
		if err != nil {
			return nil, "", common.ErrorAtPos(op.Line, op.Column, "go operand %q: render failed: %w", op.Name, err)
		}
		operands[op.Name] = rendered
	}

	// Resolve type operands (type T = SomeType)
	for _, tp := range n.TypeOperands {
		if _, exists := operands[tp.Name]; exists {
			return nil, "", common.ErrorAtPos(tp.Line, tp.Column, "duplicate go operand %q", tp.Name)
		}
		operands[tp.Name] = g.goType(tp.Type, ctx.typeParams)
	}

	missing := ""
	substituted := goPlaceholderRE.ReplaceAllStringFunc(n.Code, func(match string) string {
		name := match[1 : len(match)-1]
		code, ok := operands[name]
		if !ok {
			missing = name
			return match
		}
		return code
	})
	if missing != "" {
		return nil, "", common.ErrorAtPos(n.Line, n.Column, "go code references unknown operand %q", missing)
	}

	if isUnitType(n.Result) {
		return jen.Id(substituted), "", nil
	}
	return jen.Id(substituted), g.goType(n.Result, ctx.typeParams), nil
}

func renderGoOperand(code jen.Code) (string, error) {
	f := jen.NewFile("mygo_raw")
	f.Add(jen.Var().Id("_").Op("=").Add(code))
	var buf bytes.Buffer
	if err := f.Render(&buf); err != nil {
		return "", err
	}
	src := buf.String()
	const marker = "var _ = "
	idx := strings.Index(src, marker)
	if idx < 0 {
		return "", fmt.Errorf("rendered operand missing marker")
	}
	expr := strings.TrimSpace(src[idx+len(marker):])
	return expr, nil
}
