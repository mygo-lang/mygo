package main
import (
  "fmt"
  parser "github.com/mygo-lang/mygo/internal/mygo/parser"
)
func main(){
src := `package main
func demo() -> Int
  42
end
`
f, err := parser.ParseFile(src)
fmt.Printf("file=%#v err=%v\n", f, err)
if f != nil { fmt.Printf("pkg=%q decls=%d\n", f.PackageName, len(f.Decls)) }
}
