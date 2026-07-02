package compiler

import root "github.com/mygo-lang/mygo/internal/mygo"

func CompileDir(dir string) (string, error) { return root.CompileDir(dir) }
func Sync(rootDir string) ([]string, error) { return root.Sync(rootDir) }
