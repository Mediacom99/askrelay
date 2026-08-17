package relay

import (
	"strings"
	"testing"
	"unicode"
)

// TestSanitizeName guards the security-review finding: a self-asserted display
// name is untrusted and surfaces (unspotlighted) to other people via find_people,
// so it must be stripped of control chars, defanged of live URLs, and capped.
func TestSanitizeName(t *testing.T) {
	// Control chars (newline, tab, ANSI ESC) are dropped.
	if got := sanitizeName("a\nb\tc\x1b[31md"); strings.ContainsFunc(got, unicode.IsControl) {
		t.Errorf("control char survived: %q", got)
	}
	// Live URL schemes are defanged.
	if got := sanitizeName("Bob http://evil.example.com/x"); strings.Contains(got, "http://") {
		t.Errorf("live URL survived: %q", got)
	}
	// Length is capped.
	if got := sanitizeName(strings.Repeat("A", 500)); len([]rune(got)) > maxNameLen {
		t.Errorf("length not capped: %d runes", len([]rune(got)))
	}
	// Ordinary names (incl. non-ASCII) pass through unchanged.
	for _, ok := range []string{"Bob", "Édoardo B.", "李雷"} {
		if got := sanitizeName(ok); got != ok {
			t.Errorf("normal name mangled: %q -> %q", ok, got)
		}
	}
}
