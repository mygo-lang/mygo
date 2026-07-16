package main

import (
	"fmt"
	"os"
	"strings"

	mygo_parser "github.com/mygo-lang/mygo/internal/mygo/parser"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: parsetest <file.mygo>\n")
		os.Exit(1)
	}
	src, _ := os.ReadFile(os.Args[1])
	
	dumpTokens(string(src), "mySepBy")
	
	f, err := mygo_parser.ParseFile(os.Args[1], string(src))
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		os.Exit(1)
	}
	for _, decl := range f.Decls {
		fd, ok := decl.(*mygo_parser.FuncDecl)
		if !ok {
			continue
		}
		fmt.Printf("=== Function: %s ===\n", fd.Name)
		fmt.Printf("  Body type: %T\n", fd.Body)
		if call, ok := fd.Body.(*mygo_parser.CallExpr); ok {
			fmt.Printf("  Body is CallExpr: callee=%s, nargs=%d\n", exprName(call.Callee), len(call.Args))
		}
		if fd.Name == "mySepBy" {
			printCallInfo(fd.Body, 0)
		}
	}
}

func dumpTokens(src string, funcName string) {
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		if strings.Contains(line, "func "+funcName) {
			fmt.Printf("// Source lines for %s (line %d):\n", funcName, i+1)
			fmt.Printf("//   %s\n", strings.TrimSpace(line))
			for j := i + 1; j < len(lines); j++ {
				trimmed := strings.TrimSpace(lines[j])
				if trimmed == "end" {
					break
				}
				fmt.Printf("//   %s\n", lines[j])
			}
		}
	}
}

func printCallInfo(e mygo_parser.Expr, depth int) {
	if e == nil {
		return
	}
	indent := ""
	for i := 0; i < depth; i++ {
		indent += "  "
	}
	switch n := e.(type) {
	case *mygo_parser.CallExpr:
		fmt.Printf("%sCALL callee=%s [%T] args=%d (line %d, col %d)\n", indent, exprName(n.Callee), n.Callee, len(n.Args), n.Line, n.Column)
		for i, arg := range n.Args {
			fmt.Printf("%s  arg[%d]: ", indent, i)
			printExprSummary(arg)
			fmt.Println()
			printCallInfo(arg, depth+1)
		}
	case *mygo_parser.FuncLitExpr:
		fmt.Printf("%sFUNC LITERAL params=%v\n", indent, n.Params)
		printCallInfo(n.Body, depth+1)
	case *mygo_parser.IdentExpr:
		fmt.Printf("%sIDENT %s\n", indent, n.Name)
	case *mygo_parser.SliceLitExpr:
		fmt.Printf("%sSLICE[]\n", indent)
	default:
		fmt.Printf("%sEXPR %T => %+v\n", indent, e, e)
	}
}

func exprName(e mygo_parser.Expr) string {
	if id, ok := e.(*mygo_parser.IdentExpr); ok {
		return "Ident(" + id.Name + ")"
	}
	return fmt.Sprintf("%T", e)
}

func printExprSummary(e mygo_parser.Expr) {
	if e == nil {
		fmt.Print("nil")
		return
	}
	switch n := e.(type) {
	case *mygo_parser.IdentExpr:
		fmt.Printf("Ident(%s)", n.Name)
	case *mygo_parser.CallExpr:
		fmt.Printf("Call(callee=%s)", exprName(n.Callee))
	case *mygo_parser.SliceLitExpr:
		fmt.Printf("SliceLit(n=%d)", len(n.Elems))
	case *mygo_parser.StructLitExpr:
		fmt.Printf("StructLit(%s)", n.TypeName)
	case *mygo_parser.FuncLitExpr:
		fmt.Printf("FuncLit")
	default:
		fmt.Printf("%T", e)
	}
}
