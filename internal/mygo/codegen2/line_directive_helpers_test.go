package codegen2

import "regexp"

var lineDirectivePattern = regexp.MustCompile(`\s*/\*line [^\n]*?\*/`)

func withoutLineDirectives(source string) string {
	return lineDirectivePattern.ReplaceAllString(source, "")
}
