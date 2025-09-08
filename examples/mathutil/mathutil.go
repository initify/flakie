package mathutil

// Sum returns the sum of the numbers.
func Sum(nums ...int) int {
	s := 0
	for _, n := range nums {
		s += n
	}
	return s
}

// Factorial returns n! for n >= 0; panics for negative n.
func Factorial(n int) int {
	if n < 0 {
		panic("negative")
	}
	if n == 0 {
		return 1
	}
	f := 1
	for i := 2; i <= n; i++ {
		f *= i
	}
	return f
}
