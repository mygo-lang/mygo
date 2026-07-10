package mygo

import compilerpkg "github.com/mygo-lang/mygo/internal/mygo/compiler"

func CompileDir(dir string) ([]string, error) { return compilerpkg.CompileDir(dir) }
func Sync(root string) ([]string, error)    { return compilerpkg.Sync(root) }
