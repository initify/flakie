package stringutil

import "strings"

// Reverse returns s reversed rune-wise.
func Reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

// IsPalindrome reports whether s reads the same forward and backward, case-insensitive.
func IsPalindrome(s string) bool {
	s = strings.ToLower(s)
	return s == Reverse(s)
}
