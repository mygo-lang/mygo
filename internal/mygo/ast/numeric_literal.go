package ast

import "strings"

type NumericLiteralInfo struct {
	Value   string
	Suffix  string
	Type    string
	IsFloat bool
}

func ParseNumericLiteral(raw string) NumericLiteralInfo {
	info := NumericLiteralInfo{Value: raw}
	lower := strings.ToLower(raw)
	for _, suffix := range []string{"i16", "i32", "i64", "i8", "u16", "u32", "u64", "u8", "f32", "f64", "u"} {
		if strings.HasSuffix(lower, suffix) {
			info.Value = raw[:len(raw)-len(suffix)]
			info.Suffix = suffix
			break
		}
	}
	info.IsFloat = strings.Contains(info.Value, ".")
	switch info.Suffix {
	case "i8":
		info.Type = "Int8"
	case "i16":
		info.Type = "Int16"
	case "i32":
		info.Type = "Int32"
	case "i64":
		info.Type = "Int64"
	case "u":
		info.Type = "UInt"
	case "u8":
		info.Type = "UInt8"
	case "u16":
		info.Type = "UInt16"
	case "u32":
		info.Type = "UInt32"
	case "u64":
		info.Type = "UInt64"
	case "f32":
		info.Type = "Float32"
	case "f64":
		info.Type = "Float64"
	default:
		if info.IsFloat {
			info.Type = "Float64"
		} else {
			info.Type = "Int"
		}
	}
	return info
}
