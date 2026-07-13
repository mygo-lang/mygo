package mygo

import parserpkg "github.com/mygo-lang/mygo/internal/mygo/parser"

func ParseFile(filename, src string) (*File, error)                { return parserpkg.ParseFile(filename, src) }
func ParseFiles(srcs map[string]string) ([]*File, error) { return parserpkg.ParseFiles(srcs) }
func MustParseInt(s string) int                          { return parserpkg.MustParseInt(s) }
