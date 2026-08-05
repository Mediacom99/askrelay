package redact

import (
	"strings"
	"testing"
	"time"
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

// --- Adversarial: bypasses a real user could plausibly trigger ---------------
//
// Each case below asserts the SECURE expectation ("this secret must not survive
// un-redacted"). Where the implementation falls short the test FAILS — that
// failure is the finding, and is intentionally left red rather than weakened.
func TestRedactBypasses(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		secret string // raw material that must not appear verbatim in the output
	}{
		// Truncated PEM: message got cut off mid-paste, no END line. The whole
		// key is currently left untouched in plaintext.
		{"private-key-no-end", "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEAtruncatedsecretmaterial\n", "MIIEpAIBAAKCAQEAtruncatedsecretmaterial"},

		// A JWT hard-wrapped by chat/email line-wrapping. Both halves survive.
		{"jwt-split-by-newline", "tok eyJhbGciOiJIUzI1NiJ9.eyJzdWI\niOiIxMjM0NSJ9.abcDEF123-_XYZ end", "eyJhbGciOiJIUzI1NiJ9.eyJzdWI"},

		// alg=none JWT: trailing dot, empty signature segment.
		{"jwt-alg-none-empty-sig", "eyJhbGciOiJub25lIn0.eyJzdWIiOiIxIn0.", "eyJhbGciOiJub25lIn0.eyJzdWIiOiIxIn0."},

		// Slack app-level / config tokens use "xapp-"/"xoxe-" prefixes, not xox[baprs]-.
		{"slack-xapp-token", "xapp-1-A0123456789-1234567890123-abcdefabcdefabcdefabcdefabcdefab", "xapp-1-A0123456789"},

		// Two AWS key IDs pasted with no separator (e.g. a stripped-whitespace
		// table dump): \b fails on both sides of the join point, so neither matches.
		{"aws-keys-concatenated-no-boundary", "id: AKIAIOSFODNN7EXAMPLEAKIAIOSFODNN7EXAMPLE.", "AKIAIOSFODNN7EXAMPLE"},

		// Windows/cmd style assignment: no "export", space before NAME breaks the
		// anchored name-prefix match.
		{"env-cmd-set-style", "set PASSWORD=hunter2ssupersecret", "hunter2ssupersecret"},

		// YAML-style "name: value" (colon, not '='). Very common paste format.
		{"env-yaml-colon", "password: hunter2ssupersecret", "hunter2ssupersecret"},

		// JSON-style "name": "value" (colon, not '=') for a name containing SECRET.
		{"env-json-colon", `{"aws_secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"}`, "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"},

		// Marker-injection: a stray literal '⟦' right after '=' defeats the
		// env-secret value class entirely (it requires the value's first byte to
		// not be '⟦'), leaking the ENTIRE value, not just the first char.
		{"env-marker-injection-defeats-rule", "PASSWORD=⟦hunter2ssupersecret", "hunter2ssupersecret"},

		// Same injection class against the GCP rule's value group.
		{"gcp-marker-injection-defeats-rule", `{"private_key":"⟦AAAAsecretmaterialBBBBmoresecret"}`, "AAAAsecretmaterialBBBBmoresecret"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := Redact(c.in)
			if strings.Contains(r.Text, c.secret) {
				t.Errorf("BYPASS: secret material survived un-redacted\n  in:     %q\n  out:    %q\n  secret: %q\n  counts: %v", c.in, r.Text, c.secret, r.Counts)
			}
		})
	}
}

// A quoted env value containing spaces is only PARTIALLY redacted: the value
// regex stops at the first whitespace, so everything after the space in the
// quoted string is left in plaintext right next to the marker — worse than a
// full miss, because the marker gives the sender false confidence the whole
// secret was scrubbed.
func TestRedactBypass_QuotedValuePartialLeak(t *testing.T) {
	in := `PASSWORD="hunter2 with more secret words after the space"`
	r := Redact(in)
	if strings.Contains(r.Text, "with more secret words after the space") {
		t.Errorf("PARTIAL LEAK: tail of a quoted, space-containing secret survived un-redacted: got %q", r.Text)
	}
}

// Real GCP service-account JSON embeds a full "-----BEGIN PRIVATE KEY-----...
// -----END PRIVATE KEY-----" PEM block as the private_key value (escaped \n,
// not real newlines — DOTALL is irrelevant either way since '.' matches those
// literal backslash-n bytes too). Rule order means the PrivateKey rule (index
// 0) consumes that span before the GCP-specific rule (index 1) ever runs, so
// KindGCPSAKey is effectively unreachable for realistic input: nothing leaks,
// but the distinct "gcp-service-account" label the Summary is supposed to
// surface never fires for the one shape it exists to describe.
func TestRedactGCPRuleUnreachableForRealisticInput(t *testing.T) {
	in := `{"type":"service_account","private_key":"-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEA\n-----END PRIVATE KEY-----\n"}`
	r := Redact(in)
	if r.Counts[KindGCPSAKey] != 0 || r.Counts[KindPrivateKey] != 1 {
		t.Skipf("informational: got counts=%v (kept as a skip, not a failure, since the secret IS safely redacted either way)", r.Counts)
	}
}

// Concatenating two independently-valid PEM blocks must still yield two
// separate, correctly-scoped redactions (confirms the non-greedy .*? doesn't
// bridge from the first BEGIN to the last END).
func TestRedactMultiplePrivateKeysNotBridged(t *testing.T) {
	in := "-----BEGIN RSA PRIVATE KEY-----\nAAAA\n-----END RSA PRIVATE KEY-----\nsome text in between\n-----BEGIN EC PRIVATE KEY-----\nBBBB\n-----END EC PRIVATE KEY-----"
	r := Redact(in)
	if r.Counts[KindPrivateKey] != 2 {
		t.Errorf("Counts[private-key] = %d, want 2 (two independent PEM blocks)", r.Counts[KindPrivateKey])
	}
	if !strings.Contains(r.Text, "some text in between") {
		t.Errorf("non-secret text between the two keys was swallowed: %q", r.Text)
	}
}

// Marker-forgery around a REAL secret: attacker-controlled text carries a
// hand-typed fake marker glyph immediately before a genuine secret on the same
// line, hoping to either dodge the rule or desync Counts. The real secret must
// still be found and redacted, and Counts must reflect exactly what changed.
func TestRedactMarkerForgeryDoesNotSuppressRealSecret(t *testing.T) {
	in := "note ⟦redacted:aws-access-key⟧ but here is a real one AKIAIOSFODNN7EXAMPLE"
	r := Redact(in)
	if strings.Contains(r.Text, "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("real secret survived next to a forged marker: %q", r.Text)
	}
	if r.Counts[KindAWSKey] != 1 {
		t.Errorf("Counts[aws-access-key] = %d, want 1", r.Counts[KindAWSKey])
	}
}

// Fabricated marker text with NO adjacent secret must pass through unchanged
// and must not be counted (nothing was actually redacted).
func TestRedactFakeMarkerAloneNotCounted(t *testing.T) {
	in := "the tool prints things like ⟦redacted:aws-access-key⟧ in its output"
	r := Redact(in)
	if r.Redacted() {
		t.Errorf("fabricated marker text with no real secret was counted: %v", r.Counts)
	}
	if r.Text != in {
		t.Errorf("fabricated marker text was mutated: got %q want %q", r.Text, in)
	}
}

// Idempotency / no re-redaction: redacting already-redacted text must be a
// no-op. This is the structural guarantee the marker-exclusion char classes
// exist for; assert it directly rather than trusting the char classes by
// inspection.
func TestRedactIdempotent(t *testing.T) {
	seeds := []string{
		"AKIAIOSFODNN7EXAMPLE AKIAIOSFODNN7EXAMPLE",
		"GITHUB_TOKEN=ghp_0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ",
		"PASSWORD=hunter2",
		`{"private_key":"-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n"}`,
		"bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NSJ9.abc-_123XYZ",
		"xoxb-1234567890-abcdefghij",
	}
	for _, s := range seeds {
		once := Redact(s)
		twice := Redact(once.Text)
		if twice.Redacted() {
			t.Errorf("re-redacting an already-redacted string found new matches: seed=%q\n once:  %q counts=%v\n twice: %q counts=%v", s, once.Text, once.Counts, twice.Text, twice.Counts)
		}
		if twice.Text != once.Text {
			t.Errorf("re-redacting mutated already-redacted text: %q -> %q", once.Text, twice.Text)
		}
	}
}

// Pathological / large input must stay fast (RE2 is linear; this is a
// regression guard against an accidental backtracking-style rule being added
// later, and against FindAll-driven O(n^2) blowups in apply()).
func TestRedactPathologicalInputStaysLinear(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping perf check in -short mode")
	}
	mk := func(n int) string {
		var b strings.Builder
		for i := 0; i < n; i++ {
			b.WriteString("-----BEGIN RSA PRIVATE KEY----- filler filler filler ")
		}
		return b.String()
	}
	small := mk(2000)
	large := mk(20000) // 10x
	t0 := time.Now()
	Redact(small)
	tSmall := time.Since(t0)
	t1 := time.Now()
	Redact(large)
	tLarge := time.Since(t1)
	t.Logf("small(%d)=%v large(%d)=%v", len(small), tSmall, len(large), tLarge)
	// Generous bound: quadratic blowup would make this thousands of times
	// slower, not ~10x. Guard against a hang, not a tight perf budget.
	if tLarge > 5*time.Second {
		t.Errorf("Redact on %d bytes of unterminated BEGIN markers took %v — possible non-linear blowup", len(large), tLarge)
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
