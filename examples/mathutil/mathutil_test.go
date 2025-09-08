package mathutil

import "testing"

func TestSum(t *testing.T) {
	if got := Sum(1, 2, 3, 4); got != 10 {
		t.Fatalf("Sum() = %d; want 10", got)
	}
}

func TestFactorial(t *testing.T) {
	cases := map[int]int{0: 1, 1: 1, 3: 6, 5: 120}
	for n, want := range cases {
		if got := Factorial(n); got != want {
			t.Fatalf("Factorial(%d) = %d; want %d", n, got, want)
		}
	}
}
