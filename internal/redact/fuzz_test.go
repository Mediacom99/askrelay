package redact

import "testing"

// FuzzRedact checks that Redact never panics on arbitrary bytes (including
// invalid UTF-8 and text containing the marker glyph itself) and that its
// output is idempotent: redacting the result of a redaction must never find
// anything new. That invariant is what the marker-exclusion char classes
// ([^⟦"…], [^⟦\s]) exist to guarantee, so this fuzzes exactly the property the
// WP calls out — a crafted input tricking a rule into re-redacting, or
// mis-skipping, its own marker.
func FuzzRedact(f *testing.F) {
	seeds := []string{
		"",
		"⟦",
		"⟦redacted:aws-access-key⟧",
		"PASSWORD=⟦hunter2",
		`{"private_key":"⟦AAAA"}`,
		"-----BEGIN RSA PRIVATE KEY-----\n\xff\xfe binary junk -----END RSA PRIVATE KEY-----",
		"AKIAIOSFODNN7EXAMPLE",
		"AKIAIOSFODNN7EXAMPLEAKIAIOSFODNN7EXAMPLE",
		"ghp_0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ",
		"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NSJ9.abc",
		"eyJ.eyJ.",
		"xoxb-1-2-3",
		"TOKEN=\ufeff\u200bvalue", // BOM + zero-width space glued to the value
		"export SECRET=\"a b c\"",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		r := Redact(s) // must not panic on any input, including invalid UTF-8

		again := Redact(r.Text)
		if again.Redacted() {
			t.Fatalf("re-redacting Redact's own output found new matches:\n in:     %q\n once:   %q (counts=%v)\n twice:  %q (counts=%v)", s, r.Text, r.Counts, again.Text, again.Counts)
		}
	})
}
