package prelude

import "fmt"
import "strconv"

func MygoIT4FromFN3IntGN3IntN5Int64EM4From(value int) int64 {
	return int64(value)
}
func MygoIT4FromFN5Int64GN5Int64N3IntEM4From(value int64) int {
	return int(value)
}
func MygoIT4FromFN5Int64GN5Int64N7Float64EM4From(value int64) float64 {
	return float64(value)
}
func MygoIT4FromFN7Float64GN7Float64N5Int64EM4From(value float64) int64 {
	return int64(value)
}
func MygoIT4FromFN7Float32GN7Float32N7Float64EM4From(value float32) float64 {
	return float64(value)
}
func MygoIT4FromFN7Float64GN7Float64N7Float32EM4From(value float64) float32 {
	return float32(value)
}
func MygoIT4FromFN3IntGN3IntN7Float64EM4From(value int) float64 {
	return float64(value)
}
func MygoIT4FromFN7Float64GN7Float64N3IntEM4From(value float64) int {
	return int(value)
}
func MygoIT4FromFN3IntGN3IntN6StringEM4From(value int) string {
	return fmt.Sprint(value)
}
func MygoIT4FromFN6StringGN6StringN6ResultGN3IntN6StringEEM4From(value string) Result[int, string] {
	var __mygo_expr_2 Result[int, error]
	__mygo_expr_0, __mygo_expr_1 := strconv.Atoi(value)
	if __mygo_expr_1 != nil {
		__mygo_expr_2 = Err[int, error](__mygo_expr_1)
	} else {
		__mygo_expr_2 = Ok[int, error](__mygo_expr_0)
	}
	var __mygo_expr_3 Result[int, string]
	if __mygo_match___mygo_expr_5, ok := __mygo_expr_2.(ResultOk[int, error]); ok {
		__mygo_expr_3 = Ok[int, string](__mygo_match___mygo_expr_5.F0)
	} else {
		if __mygo_match___mygo_expr_4, ok := __mygo_expr_2.(ResultErr[int, error]); ok {
			__mygo_expr_3 = Err[int, string](__mygo_match___mygo_expr_4.F0.Error())
		} else {
		}
	}
	return __mygo_expr_3
}
func MygoIT4FromFN6StringGN6StringN6ResultGN5Int64N6StringEEM4From(value string) Result[int64, string] {
	var __mygo_expr_2 Result[int64, error]
	__mygo_expr_0, __mygo_expr_1 := strconv.ParseInt(value, 10, 64)
	if __mygo_expr_1 != nil {
		__mygo_expr_2 = Err[int64, error](__mygo_expr_1)
	} else {
		__mygo_expr_2 = Ok[int64, error](__mygo_expr_0)
	}
	var __mygo_expr_3 Result[int64, string]
	if __mygo_match___mygo_expr_5, ok := __mygo_expr_2.(ResultOk[int64, error]); ok {
		__mygo_expr_3 = Ok[int64, string](__mygo_match___mygo_expr_5.F0)
	} else {
		if __mygo_match___mygo_expr_4, ok := __mygo_expr_2.(ResultErr[int64, error]); ok {
			__mygo_expr_3 = Err[int64, string](__mygo_match___mygo_expr_4.F0.Error())
		} else {
		}
	}
	return __mygo_expr_3
}
func MygoIT4FromFN6StringGN6StringN6ResultGN7Float64N6StringEEM4From(value string) Result[float64, string] {
	var __mygo_expr_2 Result[float64, error]
	__mygo_expr_0, __mygo_expr_1 := strconv.ParseFloat(value, 64)
	if __mygo_expr_1 != nil {
		__mygo_expr_2 = Err[float64, error](__mygo_expr_1)
	} else {
		__mygo_expr_2 = Ok[float64, error](__mygo_expr_0)
	}
	var __mygo_expr_3 Result[float64, string]
	if __mygo_match___mygo_expr_5, ok := __mygo_expr_2.(ResultOk[float64, error]); ok {
		__mygo_expr_3 = Ok[float64, string](__mygo_match___mygo_expr_5.F0)
	} else {
		if __mygo_match___mygo_expr_4, ok := __mygo_expr_2.(ResultErr[float64, error]); ok {
			__mygo_expr_3 = Err[float64, string](__mygo_match___mygo_expr_4.F0.Error())
		} else {
		}
	}
	return __mygo_expr_3
}
