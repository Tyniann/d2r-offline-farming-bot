//go:build windows

package process

import "testing"

func TestCanonicalWindowsVersion(t *testing.T) {
	for raw, want := range map[string]string{
		"3.2.92777":      "3.2.92777",
		"3.2.92777.0":    "3.2.92777",
		"3, 2, 92777, 0": "3.2.92777",
		"3.2.0.92777":    "3.2.0.92777",
		"not-a-version":  "",
	} {
		if got := canonicalWindowsVersion(raw); got != want {
			t.Errorf("canonicalWindowsVersion(%q) = %q, want %q", raw, got, want)
		}
	}
}
