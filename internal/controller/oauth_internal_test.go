package controller

import "testing"

// TestSafeReturnTo locks the open-redirect guard, including the backslash vector
// (a browser normalizes "/\evil.com" to the scheme-relative "//evil.com").
func TestSafeReturnTo(t *testing.T) {
	cases := map[string]string{
		"":             "/",
		"/profile":     "/profile",
		"/a/b?x=1#f":   "/a/b?x=1#f",
		"//evil.com":   "/",
		"/\\evil.com":  "/",
		"/\\/evil.com": "/",
		"https://evil": "/",
		"relative":     "/",
		"/path\r\nx":   "/",
		"/path\tx":     "/",
	}
	for in, want := range cases {
		if got := safeReturnTo(in); got != want {
			t.Errorf("safeReturnTo(%q) = %q, want %q", in, got, want)
		}
	}
}
