package prelude

import "strings"

func MygoIT11IEnumerableFN17StringIEnumerableGN6StringN4RuneEM4Each(c string, fn func(rune)) {
	func() {
		for _, ru := range c {
			fn(ru)
		}
	}()
}
func MygoIT11IEnumerableFN17StringIEnumerableGN6StringN4RuneEM3Len(c string) int {
	return len(c)
}
func MygoIT11IEnumerableFN17StringIEnumerableGN6StringN4RuneEM3Map[B any](c string, fn func(rune) B) []B {
	slc := []rune(c)
	return MygoIT11IEnumerableFN16SliceIEnumerableGN1TEGN5SliceGN1TEN1TEM3Map(slc, fn)
}
func MygoIT11IEnumerableFN17StringIEnumerableGN6StringN4RuneEM6Filter(c string, fn func(rune) bool) []rune {
	slc := []rune(c)
	return MygoIT11IEnumerableFN16SliceIEnumerableGN1TEGN5SliceGN1TEN1TEM6Filter(slc, fn)
}
func MygoIT11IEnumerableFN17StringIEnumerableGN6StringN4RuneEM4Fold[B any](c string, initial B, fn func(B, rune) B) B {
	slc := []rune(c)
	return MygoIT11IEnumerableFN16SliceIEnumerableGN1TEGN5SliceGN1TEN1TEM4Fold(slc, initial, fn)
}
func MygoIT11IEnumerableFN17StringIEnumerableGN6StringN4RuneEM4Find(c string, fn func(rune) bool) Option[*rune] {
	slc := []rune(c)
	return MygoIT11IEnumerableFN16SliceIEnumerableGN1TEGN5SliceGN1TEN1TEM4Find(slc, fn)
}
func MygoIT11IEnumerableFN17StringIEnumerableGN6StringN4RuneEM8Contains(c string, item rune) bool {
	return strings.ContainsRune(c, item)
}
func MygoIN6StringM9FromRunes(rs []rune) string {
	return string(rs)
}
func MygoIN6StringM11MatchString(s string, prefix string) bool {
	if MygoIT11IEnumerableFN17StringIEnumerableGN6StringN4RuneEM3Len(s) >= MygoIT11IEnumerableFN17StringIEnumerableGN6StringN4RuneEM3Len(prefix) {
		return s[:len(prefix)] == prefix
	} else {
		return false
	}
}
func MygoIN6StringM9HasPrefix(s string, prefix string) bool {
	return strings.HasPrefix(s, prefix)
}
func MygoIN6StringM9HasSuffix(s string, suffix string) bool {
	return strings.HasSuffix(s, suffix)
}
func MygoIN6StringM4Trim(s string, cutset string) string {
	return strings.Trim(s, cutset)
}
func MygoIN6StringM9TrimSpace(s string) string {
	return strings.TrimSpace(s)
}
func MygoIN6StringM10TrimPrefix(s string, prefix string) string {
	return strings.TrimPrefix(s, prefix)
}
func MygoIN6StringM10TrimSuffix(s string, suffix string) string {
	return strings.TrimSuffix(s, suffix)
}
func MygoIN6StringM5Split(s string, sep string) []string {
	return strings.Split(s, sep)
}
func MygoIN6StringM6SplitN(s string, sep string, n int) []string {
	return strings.SplitN(s, sep, n)
}
func MygoIN6StringM4Join(sep string, elems []string) string {
	return strings.Join(elems, sep)
}
func MygoIN6StringM7Replace(s string, old string, new string, n int) string {
	return strings.Replace(s, old, new, n)
}
func MygoIN6StringM10ReplaceAll(s string, old string, new string) string {
	return strings.ReplaceAll(s, old, new)
}
func MygoIN6StringM7ToUpper(s string) string {
	return strings.ToUpper(s)
}
func MygoIN6StringM7ToLower(s string) string {
	return strings.ToLower(s)
}
func MygoIN6StringM6Repeat(s string, count int) string {
	return strings.Repeat(s, count)
}
func MygoIN6StringM5Index(s string, substr string) int {
	return strings.Index(s, substr)
}
func MygoIN6StringM9LastIndex(s string, substr string) int {
	return strings.LastIndex(s, substr)
}
func MygoIN6StringM6Fields(s string) []string {
	return strings.Fields(s)
}
