%{
package parser

import (
	"bufio"
	gotoken "go/token"
	"io"
	"strings"
	"unicode"

	"modernc.org/golex/lex"
)

const (
	classUnicodeLetter = iota + 0x80
	classUnicodeDigit
	classOther
)

type golexer struct {
	*lex.Lexer
	multilineContent string
}

func rune2Class(r rune) int {
	if r >= 0 && r < 0x80 {
		return int(r)
	}
	if unicode.IsLetter(r) {
		return classUnicodeLetter
	}
	if unicode.IsDigit(r) {
		return classUnicodeDigit
	}
	return classOther
}

func newGolexer(src io.Reader, fName string) (*golexer, error) {
	file := gotoken.NewFileSet().AddFile(fName, -1, 1<<31-1)
	lx, err := lex.New(file, bufio.NewReader(src), lex.RuneClass(rune2Class))
	if err != nil {
		return nil, err
	}
	return &golexer{Lexer: lx}, nil
}

func (l *golexer) char(r int) lex.Char { return lex.NewChar(l.First.Pos(), rune(r)) }

func (l *golexer) scanMultilineStringFromLookahead() lex.Char {
	var buf strings.Builder
	read := func() int {
		ch := l.Lookahead()
		l.Next()
		return int(ch.Rune)
	}
	for {
		r := read()
		if r == lex.RuneEOF {
			break
		}
		if r == '"' {
			r2 := read()
			if r2 == lex.RuneEOF {
				break
			}
			r3 := read()
			if r3 == lex.RuneEOF {
				break
			}
			if r2 == '"' && r3 == '"' {
				break
			}
			buf.WriteRune(rune('"'))
			buf.WriteRune(rune(r2))
			buf.WriteRune(rune(r3))
			continue
		}
		buf.WriteRune(rune(r))
	}
	l.multilineContent = buf.String()
	return l.char(STRING)
}

func (l *golexer) scan() lex.Char {
	c := l.Enter()
%}

%yyc c
%yyn c = l.Next()
%yym l.Mark()

ident      ({letter}|{unicodeLetter})({letter}|{digit}|{unicodeLetter}|{unicodeDigit}|_)*
digit      [0-9]|{unicodeDigit}
digits     {digit}(_?{digit})*
intsuffix  i8|i16|i32|i64|u|u8|u16|u32|u64
floatsuffix f32|f64
number     {digits}(\.{digits})?({intsuffix}|{floatsuffix})?
hexdigit   [0-9a-fA-F]
hexdigits  {hexdigit}(_?{hexdigit})*
octdigits  [0-7](_?[0-7])*
bindigits  [01](_?[01])*
hexnumber  0[xX]{hexdigits}({intsuffix})?
octnumber  0[oO]{octdigits}({intsuffix})?
binnumber  0[bB]{bindigits}({intsuffix})?
letter     [_a-zA-Z]|{unicodeLetter}
unicodeDigit  \x81
unicodeLetter \x80

%%
	c = l.Rule0()

[ \t\r]+
\n                return l.char(NEWLINE)
\#([^\n])*
\"\"\" { return l.scanMultilineStringFromLookahead() }
`[^\`]*`  return l.char(STRING)
\"([^\"\\]|\\.)*\"  return l.char(STRING)
'([^'\\]|\\.)'  return l.char(RUNE)
package            return l.char(PACKAGE)
import             return l.char(IMPORT)
enum               return l.char(ENUM)
struct             return l.char(STRUCT)
interface          return l.char(INTERFACE)
impl               return l.char(IMPL)
func               return l.char(FUNC)
if                 return l.char(IF)
then               return l.char(THEN)
elsif              return l.char(ELSIF)
else               return l.char(ELSE)
switch             return l.char(SWITCH)
case               return l.char(CASE)
end                return l.char(END)
using              return l.char(USING)
let                return l.char(LET)
var                return l.char(VAR)
embed              return l.char(EMBED)
while              return l.char(WHILE)
return             return l.char(RETURN)
go                 return l.char(GO)
in                 return l.char(IN)
type               return l.char(TYPE)
as                 return l.char(AS)
{ident}            return l.char(IDENT)
{hexnumber}        return l.char(NUMBER)
{octnumber}        return l.char(NUMBER)
{binnumber}        return l.char(NUMBER)
{number}           return l.char(NUMBER)

=>                 return l.char(ARROW)
->                 return l.char(ARROW)
\<\=               return l.char(LTE)
\>\=               return l.char(GTE)
\<\|               return l.char(PIPEBACK)
\|>                return l.char(PIPEFWD)
==                 return l.char(EQEQ)
!=                 return l.char(NEQ)
&&                 return l.char(ANDAND)
\|\|               return l.char(OROR)
[:]                return l.char(COLON)
[,]                return l.char(COMMA)
[.]                return l.char(DOT)
[(]                return l.char(LPAREN)
[)]                return l.char(RPAREN)
[\[]               return l.char(LBRACK)
[\]]               return l.char(RBRACK)
[{}]               return l.char(0)
[-+*/!<>_=]        return l.char(int(c))

%%
	if c, ok := l.Abort(); ok {
		return l.char(c)
	}
	goto yyAction
}
