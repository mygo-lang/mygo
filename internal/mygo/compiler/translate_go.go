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
var goTupleErrorRE = regexp.MustCompile(`func\(\)\s*\([^)]+,\s*error\s*\)`)

func (g *generator) translateGoExpr(n *GoExpr, ctx *exprCtx, expected string) (jen.Code, string, error) {
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

	code := jen.Id(substituted)
	if isUnitType(n.Result) {
		return code, "", nil
	}
	resultType := g.goType(n.Result, ctx.typeParams)
	if wrapped, wrappedType, ok := wrapGoFFIExpr(code, resultType, expected, substituted); ok {
		return wrapped, wrappedType, nil
	}
	return code, resultType, nil
}

func wrapGoFFIExpr(code jen.Code, typ, expected, rawCode string) (jen.Code, string, bool) {
	typ = strings.TrimSpace(typ)
	expected = strings.TrimSpace(expected)
	if typ == expected && strings.Contains(rawCode, expected) {
		return nil, "", false
	}
	if inner, ok := refToExpectedOptionRefInner(typ, expected); ok {
		tmp := "__mygo_go_ref"
		return jen.Func().Params().Id(expected).Block(
			jen.Id(tmp).Op(":=").Add(code),
			jen.If(jen.Id(tmp).Op("==").Nil()).Block(
				jen.Return(jen.Id("None["+inner+"]").Call()),
			),
			jen.Return(jen.Id("Some["+inner+"]").Call(jen.Id(tmp))),
		).Call(), expected, true
	}
	optPrefix := "Option["
	if strings.HasPrefix(expected, optPrefix) && strings.HasSuffix(expected, "]") {
		inner := strings.TrimSuffix(strings.TrimPrefix(expected, optPrefix), "]")
		tmp := "__mygo_value"
		return jen.Func().Params().Id(expected).Block(
			jen.Id(tmp).Op(":=").Add(code),
			jen.Return(jen.Id("Some["+inner+"]").Call(jen.Id(tmp))),
		).Call(), expected, true
	}
	resPrefix := "Result["
	if strings.HasPrefix(expected, resPrefix) && strings.HasSuffix(expected, "]") {
		inner := strings.TrimSuffix(strings.TrimPrefix(expected, resPrefix), "]")
		parts := strings.SplitN(inner, ",", 2)
		if len(parts) == 2 {
			okType := strings.TrimSpace(parts[0])
			errType := strings.TrimSpace(parts[1])
			tmp := "__mygo_result"
			// Detect Go (T, error) tuple return pattern — destructure for Ok/Err wrapping
			if goTupleErrorRE.MatchString(rawCode) {
				valTmp := tmp + "_val"
				errTmp := tmp + "_err"
				return jen.Func().Params().Id(expected).Block(
					jen.List(jen.Id(valTmp), jen.Id(errTmp)).Op(":=").Add(code),
					jen.If(jen.Id(errTmp).Op("!=").Nil()).Block(
						jen.Return(jen.Id("Err["+okType+", "+errType+"]").Call(jen.Id(errTmp))),
					),
					jen.Return(jen.Id("Ok["+okType+", "+errType+"]").Call(jen.Id(valTmp))),
				).Call(), expected, true
			}
			return jen.Func().Params().Id(expected).Block(
				jen.Id(tmp).Op(":=").Add(code),
				jen.Return(jen.Id("Ok["+okType+", "+errType+"]").Call(jen.Id(tmp))),
			).Call(), expected, true
		}
	}
	_ = typ
	return nil, "", false
}

func refToExpectedOptionRefInner(typ, expected string) (string, bool) {
	typ = strings.TrimSpace(typ)
	expected = strings.TrimSpace(expected)
	if !strings.HasPrefix(typ, "*") {
		return "", false
	}
	const optPrefix = "Option["
	if !strings.HasPrefix(expected, optPrefix) || !strings.HasSuffix(expected, "]") {
		return "", false
	}
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(expected, optPrefix), "]"))
	if strings.TrimSpace(inner) != typ {
		return "", false
	}
	return inner, true
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
