package parser2

// TEMPORARY probe: dumps the legacy LayoutEvent stream for representative
// fixtures so the CST-derived collector can be pinned against it.  Deleted
// before task 4.4 is marked complete.

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	. "github.com/mygo-lang/mygo/prelude"
)

func probeSpan(s ast2.SourceSpan) string {
	return fmt.Sprintf("L%dC%d-L%dC%d", s.Start.Line, s.Start.Column, s.End.Line, s.End.Column)
}

func TestProbeLayoutEvents(t *testing.T) {
	if os.Getenv("PROBE_CST") != "" {
		probeCstDump(t)
		return
	}
	fixtures := probeFixtures()
	for _, name := range probeOrder() {
		src := fixtures[name]
		res := ParseFileLossless(name+".mygo", src)
		var lf LosslessFile
		if ok, isOk := res.(Result__Ok[LosslessFile, string]); isOk {
			lf = ok.F0
		} else if e, isErr := res.(Result__Err[LosslessFile, string]); isErr {
			t.Logf("%s: parse error: %s", name, e.F0)
			continue
		}
		var b strings.Builder
		fmt.Fprintf(&b, "=== %s ===\n", name)
		for _, e := range lf.LayoutEvents {
			fmt.Fprintf(&b, "%-24s path=%v depth=%d span=%s anchor=%s body=%s blockbody=%s layout=%s indent=%v expand=%v hdrForm=%q sep=%s exit=%s HL=%d BL=%d EL=%d\n",
				e.Kind, e.Path, e.Depth, probeSpan(e.Span), probeSpan(e.Anchor), probeSpan(e.BodyAnchor), probeSpan(e.BlockBodyAnchor), e.BodyLayout, e.AffectsIndent, e.ExpandsAfter, e.HeaderForm, probeSpan(e.SeparatorAnchor), probeSpan(e.ExitAnchor), e.HeaderLine, e.BodyLine, e.ExitLine)
		}
		t.Log("\n" + b.String())
		var s strings.Builder
		fmt.Fprintf(&s, "--- %s nodepsans ---\n", name)
		for _, n := range lf.NodeSpans {
			fmt.Fprintf(&s, "%-18s path=%v span=%s\n", n.Kind, n.Path, probeSpan(n.Span))
		}
		t.Log("\n" + s.String())
	}
}

func probeOrder() []string {
	return []string{"if_else", "if_elsif", "if_arrow", "switch", "while", "funclit", "nested", "switchblk"}
}

func probeFixtures() map[string]string {
	return map[string]string{
		"if_else":  "package sample\nfunc f(n: Int) -> Int\n  let v = if n > 0 then\n    1\n  else\n    2\n  end\n  v\nend\n",
		"if_elsif": "package sample\nfunc f(n: Int) -> Int\n  let v = if n > 0 then\n    1\n  elsif n < 0 then\n    -1\n  else\n    2\n  end\n  v\nend\n",
		"if_arrow": "package sample\nfunc f(n: Int) -> Int\n  let v = if n > 0 => 1 else 2\n  v\nend\n",
		"switch":   "package sample\nfunc f(n: Int) -> Int\n  let v = switch n\n    case 0 => 1\n    case _ => 2\n  end\n  v\nend\n",
		"switchblk": "package sample\nfunc f(n: Int) -> Int\n  let v = switch n\n    case 0 then\n      1\n    end\n    case _ then\n      2\n    end\n  end\n  v\nend\n",
		"while":    "package sample\nfunc f(n: Int) -> Int\n  var i = n\n  while i > 0\n    i = i - 1\n  end\n  i\nend\n",
		"funclit":  "package sample\nfunc f()-> Int\n  let g = func(x: Int) -> Int\n    x + 1\n  end\n  g(1)\nend\n",
		"nested":   "package sample\nfunc f(n: Int) -> Int\n  let v = if n > 0 then\n    if n > 10 then\n      1\n    else\n      2\n    end\n  else\n    3\n  end\n  v\nend\n",
	}
}
func probeCstDump(t *testing.T) {
	fixtures := probeFixtures()
	for _, name := range probeOrder() {
		root := ParseSyntaxAt(name+".mygo", fixtures[name]).Root
		var b strings.Builder
		fmt.Fprintf(&b, "==== CST %s ====\n", name)
		probeDumpNode(*root, 0, &b)
		t.Log("\n" + b.String())
	}
}

func probeKindName(k interface{}) string {
	n := fmt.Sprintf("%T", k)
	if i := strings.LastIndex(n, "__"); i >= 0 {
		return n[i+2:]
	}
	return n
}

func probeDumpNode(node GreenNode, depth int, b *strings.Builder) {
	indent := strings.Repeat("  ", depth)
	fmt.Fprintf(b, "%s%s [%d:%d-%d:%d]\n", indent, probeKindName(node.Kind), node.Span.Start.Line, node.Span.Start.Column, node.Span.End.Line, node.Span.End.Column)
	for i := 0; i < len(node.Children); i++ {
		item := greenElementAt(node.Children, i)
		switch v := greenElementNode(item).(type) {
		case Option__Some[*GreenNode]:
			probeDumpNode(*v.F0, depth+1, b)
		case Option__None[*GreenNode]:
			raw := greenElementRaw(item)
			if raw != "" {
				fmt.Fprintf(b, "%s  leaf %q\n", indent, raw)
			} else {
				fmt.Fprintf(b, "%s  trivia\n", indent)
			}
		}
	}
}
