package prelude

func MygoIT10IIndexableFN15StringRuneIndexGN6StringN3IntN4RuneEM3Get(s string, index int) Option[rune] {
	rs := []rune(s)
	if index < 0 || index >= MygoIT11IEnumerableFN16SliceIEnumerableGN1TEGN5SliceGN1TEN1TEM3Len(rs) {
		return None[rune]()
	} else {
		return Some[rune](rs[index])
	}
}
func MygoIT10IIndexableFN15StringRuneIndexGN6StringN3IntN4RuneEM5Slice(s string, startPos int, endPos int) Option[string] {
	rs := []rune(s)
	if startPos < 0 || endPos < startPos || startPos >= MygoIT11IEnumerableFN16SliceIEnumerableGN1TEGN5SliceGN1TEN1TEM3Len(rs) || endPos > MygoIT11IEnumerableFN16SliceIEnumerableGN1TEGN5SliceGN1TEN1TEM3Len(rs) {
		return None[string]()
	} else {
		return Some[string](string(rs[startPos:endPos]))
	}
}
