package deep

// three is at the limit: an else-if stays at the level of its if.
func three(xs [][]int) int {
	n := 0
	for _, x := range xs {
		for _, y := range x {
			if y > 0 {
				n++
			} else if y < 0 {
				n--
			}
		}
	}
	return n
}

// four is one level too deep.
func four(xs [][]int) int {
	n := 0
	for _, x := range xs {
		for _, y := range x {
			if y > 0 {
				if y%2 == 0 {
					n++
				}
			}
		}
	}
	return n
}

// elseBlock: an else block is one level deeper than its if.
func elseBlock(xs []int) int {
	n := 0
	for _, x := range xs {
		if x > 0 {
			n++
		} else {
			switch x {
			case -1:
				if n > 0 {
					n--
				}
			}
		}
	}
	return n
}

// literal: a function literal starts again at 0, but is itself checked.
func literal(xs []int) func() int {
	for range xs {
		for range xs {
			_ = func() int {
				for range xs {
					for range xs {
						for range xs {
							for range xs {
								return 1
							}
						}
					}
				}
				return 0
			}
		}
	}
	return nil
}
