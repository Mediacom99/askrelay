package redact

import (
	"strings"
	"testing"
)

func mark(k Kind) string { return "⟦redacted:" + string(k) + "⟧" }

// Each pattern's true positive: the secret becomes its marker, counted once.
func TestRedactTruePositives(t *testing.T) {
	cases := []struct {
		name string
		kind Kind
		in   string
	}{
		{"aws", KindAWSKey, "key AKIAIOSFODNN7EXAMPLE here"},
		{"github-classic", KindGitHubToken, "tok ghp_0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ end"},
		{"github-fine", KindGitHubToken, "tok github_pat_ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMN end"},
		{"slack", KindSlackToken, "hook xoxb-1234567890-abcdefghij end"},
		{"jwt", KindJWT, "bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NSJ9.abc-_123XYZ end"},
		{"gcp", KindGCPSAKey, `{"type":"service_account","private_key":"abc123keymaterial"}`},
		{"env-password", KindEnvSecret, "PASSWORD=hunter2"},
		{"env-export-token", KindEnvSecret, "export API_TOKEN=abc123def"},
		{"private-key", KindPrivateKey, "-----BEGIN RSA PRIVATE KEY-----\nMIIEabc123\n-----END RSA PRIVATE KEY-----"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := Redact(c.in)
			if !strings.Contains(r.Text, mark(c.kind)) {
				t.Errorf("Redact(%q).Text = %q, want it to contain %q", c.in, r.Text, mark(c.kind))
			}
			if r.Counts[c.kind] != 1 {
				t.Errorf("Counts[%s] = %d, want 1 (counts=%v)", c.kind, r.Counts[c.kind], r.Counts)
			}
			// The raw secret must be gone (spot-check the env cases keep the name).
			if c.kind == KindEnvSecret && !strings.Contains(r.Text, "=") {
				t.Errorf("env redaction dropped the NAME= prefix: %q", r.Text)
			}
		})
	}
}

// The false-positive corpus must pass through byte-for-byte unchanged.
func TestRedactFalsePositives(t *testing.T) {
	fp := strings.Join([]string{
		"019fc890-5c7b-79ee-a0e4-c29a2a6f3e80",        // UUIDv7
		"e83c5163316f89bfbde7d9ab23ca2e25604af290",    // 40-char git SHA
		"SGVsbG8gd29ybGQgdGhpcyBpcyBhIHRlc3Q=",        // plain base64 blob
		"The staging deploy is green, all tests pass", // ordinary prose
		"a normal_variable = 42",                      // assignment, not a secret name
	}, "\n")
	r := Redact(fp)
	if r.Redacted() {
		t.Errorf("false-positive corpus was redacted: %v\ntext=%q", r.Counts, r.Text)
	}
	if r.Text != fp {
		t.Errorf("false-positive corpus mutated:\n got %q\nwant %q", r.Text, fp)
	}
}

// A secret inside a NAME=value is labelled by its specific shape (github-token),
// not the broad env rule, and is not double-redacted.
func TestRedactSpecificLabelWins(t *testing.T) {
	r := Redact("GITHUB_TOKEN=ghp_0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	if r.Counts[KindGitHubToken] != 1 {
		t.Errorf("github-token count = %d, want 1", r.Counts[KindGitHubToken])
	}
	if r.Counts[KindEnvSecret] != 0 {
		t.Errorf("env-secret count = %d, want 0 (must not double-redact the marker)", r.Counts[KindEnvSecret])
	}
	if strings.Contains(r.Text, mark(KindEnvSecret)) {
		t.Errorf("marker was re-redacted as env-secret: %q", r.Text)
	}
}

// Documented T-12 ceiling: any NAME containing KEY is treated as a secret, so a
// harmless PUBLIC_KEY= over-redacts. Over-redaction is the safe direction; this
// test pins the accepted behaviour.
func TestRedactEnvKeyOverMatch(t *testing.T) {
	r := Redact("PUBLIC_KEY=https://example.com/pub")
	if r.Counts[KindEnvSecret] != 1 {
		t.Errorf("PUBLIC_KEY= expected to over-match as env-secret (T-12 ceiling); counts=%v", r.Counts)
	}
}

func TestSummary(t *testing.T) {
	if s := Redact("clean text").Summary(); s != "" {
		t.Errorf("Summary of clean text = %q, want empty", s)
	}
	// Two AWS keys + one JWT → stable sorted summary.
	r := Redact("AKIAIOSFODNN7EXAMPLE AKIAIOSFODNN7EXAMPLE eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ4In0.sig123")
	got := r.Summary()
	want := "aws-access-key×2, jwt×1"
	if got != want {
		t.Errorf("Summary = %q, want %q (counts=%v)", got, want, r.Counts)
	}
}
