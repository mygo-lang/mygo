package prelude

type Range struct {
	Start int
	End   int
	Step  int
}

func MygoIN5RangeM3New(start int, stop int) Range {
	return Range{Start: start, End: stop, Step: 1}
}
func MygoIN5RangeM11NewWithStep(start int, stop int, step int) Range {
	return Range{Start: start, End: stop, Step: step}
}
func MygoIT11IEnumerableFN16RangeIEnumerableGN5RangeN3IntEM4Each(c Range, fn func(int)) {
	i := c.Start
	if c.Step > 0 {
		for i < c.End {
			fn(i)
			i = i + c.Step
		}
	} else {
		if c.Step < 0 {
			for i > c.End {
				fn(i)
				i = i + c.Step
			}
		} else {
		}
	}
	return
}
func MygoIT11IEnumerableFN16RangeIEnumerableGN5RangeN3IntEM3Len(c Range) int {
	count := 0
	i := c.Start
	if c.Step > 0 {
		for i < c.End {
			count = count + 1
			i = i + c.Step
		}
	} else {
		if c.Step < 0 {
			for i > c.End {
				count = count + 1
				i = i + c.Step
			}
		} else {
		}
	}
	return count
}
func MygoIT11IEnumerableFN16RangeIEnumerableGN5RangeN3IntEM3Map[B any](c Range, fn func(int) B) []B {
	count := 0
	i := c.Start
	if c.Step > 0 {
		for i < c.End {
			count = count + 1
			i = i + c.Step
		}
	} else {
		if c.Step < 0 {
			for i > c.End {
				count = count + 1
				i = i + c.Step
			}
		} else {
		}
	}
	result := []B{}
	j := 0
	i = c.Start
	if c.Step > 0 {
		for j < count {
			result = append(result, fn(i))
			i = i + c.Step
			j = j + 1
		}
	} else {
		if c.Step < 0 {
			for j < count {
				result = append(result, fn(i))
				i = i + c.Step
				j = j + 1
			}
		} else {
		}
	}
	return result
}
func MygoIT11IEnumerableFN16RangeIEnumerableGN5RangeN3IntEM6Filter(c Range, fn func(int) bool) []int {
	count := 0
	i := c.Start
	if c.Step > 0 {
		for i < c.End {
			if fn(i) {
				count = count + 1
			} else {
			}
			i = i + c.Step
		}
	} else {
		if c.Step < 0 {
			for i > c.End {
				if fn(i) {
					count = count + 1
				} else {
				}
				i = i + c.Step
			}
		} else {
		}
	}
	result := []int{}
	j := 0
	i = c.Start
	if c.Step > 0 {
		for i < c.End {
			if fn(i) {
				result = append(result, i)
				j = j + 1
			} else {
			}
			i = i + c.Step
		}
	} else {
		if c.Step < 0 {
			for i > c.End {
				if fn(i) {
					result = append(result, i)
					j = j + 1
				} else {
				}
				i = i + c.Step
			}
		} else {
		}
	}
	return result
}
func MygoIT11IEnumerableFN16RangeIEnumerableGN5RangeN3IntEM4Fold[B any](c Range, initial B, fn func(B, int) B) B {
	acc := initial
	i := c.Start
	if c.Step > 0 {
		for i < c.End {
			acc = fn(acc, i)
			i = i + c.Step
		}
	} else {
		if c.Step < 0 {
			for i > c.End {
				acc = fn(acc, i)
				i = i + c.Step
			}
		} else {
		}
	}
	return acc
}
func MygoIT11IEnumerableFN16RangeIEnumerableGN5RangeN3IntEM4Find(c Range, fn func(int) bool) Option[*int] {
	i := c.Start
	found := None[*int]()
	done := false
	if c.Step > 0 {
		for i < c.End && !done {
			if fn(i) {
				current := i
				found = Some[*int](&current)
				done = true
			} else {
			}
			i = i + c.Step
		}
	} else {
		if c.Step < 0 {
			for i > c.End && !done {
				if fn(i) {
					current_1 := i
					found = Some[*int](&current_1)
					done = true
				} else {
				}
				i = i + c.Step
			}
		} else {
		}
	}
	return found
}
func MygoIT11IEnumerableFN16RangeIEnumerableGN5RangeN3IntEM8Contains(c Range, item int, EqualsFn func(int, int) bool) bool {
	i := c.Start
	found := false
	if c.Step > 0 {
		for i < c.End && !found {
			if EqualsFn(item, i) {
				found = true
			} else {
			}
			i = i + c.Step
		}
	} else {
		if c.Step < 0 {
			for i > c.End && !found {
				if EqualsFn(item, i) {
					found = true
				} else {
				}
				i = i + c.Step
			}
		} else {
		}
	}
	return found
}
