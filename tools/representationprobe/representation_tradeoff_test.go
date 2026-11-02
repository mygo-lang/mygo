package representationprobe

import (
	"testing"
	"unsafe"
)

// These Go-only models isolate the representation cost of a wide tagged value
// from code generation and compiler work. Payloads deliberately have distinct
// concrete types to include the type assertion used by compact matching.
type tradePayload0 struct {
	F0   uintptr
	Name string
	Refs []uintptr
}
type tradePayload1 struct {
	F0 string
	F1 uint64
}

type tradeWide struct {
	Tag string
	P00 *tradePayload0
	P01 *tradePayload1
	P02 *tradePayload0
	P03 *tradePayload1
	P04 *tradePayload0
	P05 *tradePayload1
	P06 *tradePayload0
	P07 *tradePayload1
	P08 *tradePayload0
	P09 *tradePayload1
	P10 *tradePayload0
	P11 *tradePayload1
	P12 *tradePayload0
	P13 *tradePayload1
	P14 *tradePayload0
	P15 *tradePayload1
	P16 *tradePayload0
	P17 *tradePayload1
	P18 *tradePayload0
	P19 *tradePayload1
	P20 *tradePayload0
	P21 *tradePayload1
	P22 *tradePayload0
	P23 *tradePayload1
}

type tradeCompact struct {
	Tag  string
	Data any
}

// The current ordinal layout keeps the same generic payload slot so this
// comparison isolates discriminator width and comparison cost.
type tradeCompactOrdinal struct {
	Tag  uint8
	Data any
}

var tradeWideSink []tradeWide
var tradeCompactSink []tradeCompact
var tradeCompactOrdinalSink []tradeCompactOrdinal
var tradeTagScanSink uintptr

func tradeMakeWide(n int) []tradeWide {
	values := make([]tradeWide, n)
	for i := range values {
		values[i] = tradeWide{Tag: "variant0", P00: &tradePayload0{F0: uintptr(i)}}
	}
	return values
}

func tradeMakeCompact(n int) []tradeCompact {
	values := make([]tradeCompact, n)
	for i := range values {
		values[i] = tradeCompact{Tag: "variant0", Data: tradePayload0{F0: uintptr(i)}}
	}
	return values
}

func tradeMakeCompactOrdinal(n int) []tradeCompactOrdinal {
	values := make([]tradeCompactOrdinal, n)
	for i := range values {
		values[i] = tradeCompactOrdinal{Tag: 1, Data: tradePayload0{F0: uintptr(i)}}
	}
	return values
}

func TestRepresentationTradeoff(t *testing.T) {
	if got := unsafe.Sizeof(tradeCompact{}); got >= unsafe.Sizeof(tradeWide{}) {
		t.Fatalf("compact layout is not smaller: compact=%d wide=%d", got, unsafe.Sizeof(tradeWide{}))
	}
	if got, old := unsafe.Sizeof(tradeCompactOrdinal{}), unsafe.Sizeof(tradeCompact{}); got >= old {
		t.Fatalf("ordinal tag did not reduce compact wrapper size: ordinal=%d string=%d", got, old)
	}
	wide := tradeMakeWide(2)
	compact := tradeMakeCompact(2)
	wideCopy := append([]tradeWide(nil), wide...)
	compactCopy := append([]tradeCompact(nil), compact...)
	if wideCopy[1].Tag != compactCopy[1].Tag || wideCopy[1].P00.F0 != compactCopy[1].Data.(tradePayload0).F0 {
		t.Fatal("wide and compact copies changed the selected variant payload")
	}
	if _, ok := compactCopy[0].Data.(tradePayload1); ok {
		t.Fatal("compact payload assertion accepted the wrong variant type")
	}
	ordinal := tradeMakeCompactOrdinal(2)
	ordinalCopy := append([]tradeCompactOrdinal(nil), ordinal...)
	if ordinalCopy[1].Tag != 1 || ordinalCopy[1].Data.(tradePayload0).F0 != compactCopy[1].Data.(tradePayload0).F0 {
		t.Fatal("ordinal-tag copy changed the selected variant payload")
	}
}

func BenchmarkRepresentationWide(b *testing.B) {
	for i := 0; i < b.N; i++ {
		values := tradeMakeWide(4096)
		tradeWideSink = append([]tradeWide(nil), values...)
		var sum uintptr
		for _, value := range tradeWideSink {
			if value.Tag == "variant0" {
				sum += value.P00.F0
			}
		}
		if sum == 0 {
			b.Fatal("unexpected zero sum")
		}
	}
}

func BenchmarkRepresentationCompact(b *testing.B) {
	for i := 0; i < b.N; i++ {
		values := tradeMakeCompact(4096)
		tradeCompactSink = append([]tradeCompact(nil), values...)
		var sum uintptr
		for _, value := range tradeCompactSink {
			if value.Tag == "variant0" {
				sum += value.Data.(tradePayload0).F0
			}
		}
		if sum == 0 {
			b.Fatal("unexpected zero sum")
		}
	}
}

func BenchmarkRepresentationOrdinal(b *testing.B) {
	for i := 0; i < b.N; i++ {
		values := tradeMakeCompactOrdinal(4096)
		tradeCompactOrdinalSink = append([]tradeCompactOrdinal(nil), values...)
		var sum uintptr
		for _, value := range tradeCompactOrdinalSink {
			if value.Tag == 1 {
				sum += value.Data.(tradePayload0).F0
			}
		}
		if sum == 0 {
			b.Fatal("unexpected zero sum")
		}
	}
}

func BenchmarkRepresentationCompactTagScan(b *testing.B) {
	values := tradeMakeCompact(4096)
	b.ResetTimer()
	var sum uintptr
	for i := 0; i < b.N; i++ {
		for _, value := range values {
			if value.Tag == "variant0" {
				sum++
			}
		}
	}
	tradeTagScanSink = sum
}

func BenchmarkRepresentationOrdinalTagScan(b *testing.B) {
	values := tradeMakeCompactOrdinal(4096)
	b.ResetTimer()
	var sum uintptr
	for i := 0; i < b.N; i++ {
		for _, value := range values {
			if value.Tag == 1 {
				sum++
			}
		}
	}
	tradeTagScanSink = sum
}
