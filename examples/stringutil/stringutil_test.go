package stringutil

import "testing"

func TestReverse(t *testing.T) {
	tests := []struct {
		in, out string
	}{
		{"abc", "cba"},
		{"", ""},
		{"åß∂", "∂ßå"},
	}
	for _, tt := range tests {
		if got := Reverse(tt.in); got != tt.out {
			t.Fatalf("Reverse(%q) = %q; want %q", tt.in, got, tt.out)
		}
	}
}

func TestIsPalindrome(t *testing.T) {
	if !IsPalindrome("Level") {
		t.Fatal("expected palindrome")
	}
	if IsPalindrome("hello") {
		t.Fatal("did not expect palindrome")
	}
}
