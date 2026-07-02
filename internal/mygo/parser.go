package mygo

import parserpkg "github.com/mygo-lang/mygo/internal/mygo/parser"

func ParseFile(src string) (*File, error)                { return parserpkg.ParseFile(src) }
func ParseFiles(srcs map[string]string) ([]*File, error) { return parserpkg.ParseFiles(srcs) }
func MustParseInt(s string) int                          { return parserpkg.MustParseInt(s) }
