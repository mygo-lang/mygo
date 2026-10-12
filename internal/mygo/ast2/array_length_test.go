package ast2

import "testing"

func TestArrayLengths(t *testing.T) {
	for raw, want := range map[string]int{"0": 0, "5": 5, "0x10": 16, "0o10": 8, "0b10": 2, "1_000": 1000, "-1": -1, "+1": -1, "1.5": -1, "1 + 1": -1, "N": -1, "9223372036854775808": -1} {
		if got := ParseArrayLength(raw); got != want {
			t.Fatalf("ParseArrayLength(%q) = %d, want %d", raw, got, want)
		}
	}
	t.Setenv("GOARCH", "386")
	if ParseArrayLength("2147483648") != -1 || ArrayMaxLength() != 2147483647 {
		t.Fatal("target int range not respected")
	}
}
