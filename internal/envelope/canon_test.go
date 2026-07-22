package envelope

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

// TestJCSGoldenRFC8785 cross-checks the canonicalizer against the worked
// example from RFC 8785 §3.2.3 (numbers, unicode, literals, nested arrays):
// ECMAScript number formatting, JCS string escaping, and recursive member
// sorting in one vector.
//
// The input is assembled from a Go value so no \uXXXX escape text is hand-typed
// in source; the number spellings are preserved verbatim via json.Number. The
// string value is the RFC's parsed characters (€, U+000F, newline, quotes,
// backslashes, solidus); Go's json.Marshal of that value equals its JCS form
// here (no HTML-escapable or U+2028/9 characters), so it doubles as the
// expected string bytes.
func TestJCSGoldenRFC8785(t *testing.T) {
	strVal := "€$" + string(rune(0x0f)) + "\nA'B\"\\\\\"/"

	inputBytes, err := json.Marshal(map[string]any{
		"numbers": []any{
			json.Number("333333333.33333329"),
			json.Number("1E30"),
			json.Number("4.50"),
			json.Number("2e-3"),
			json.Number("0.000000000000000000000000001"),
		},
		"string":   strVal,
		"literals": []any{nil, true, false},
	})
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}

	strCanon, err := json.Marshal(strVal)
	if err != nil {
		t.Fatalf("marshal strVal: %v", err)
	}
	want := `{"literals":[null,true,false],"numbers":[333333333.3333333,1e+30,4.5,0.002,1e-27],"string":` +
		string(strCanon) + `}`

	got, err := jcs(inputBytes)
	if err != nil {
		t.Fatalf("jcs: %v", err)
	}
	if string(got) != want {
		t.Errorf("canonical mismatch\n got: %s\nwant: %s", got, want)
	}
}

// TestJCSGoldenSimple is an integer/string-only vector: it isolates member
// sorting and integer formatting from the float machinery.
func TestJCSGoldenSimple(t *testing.T) {
	input := `{"from_account":"836284937","to_account":"268544152","amount":100,"currency":"USD"}`
	want := `{"amount":100,"currency":"USD","from_account":"836284937","to_account":"268544152"}`
	got, err := jcs([]byte(input))
	if err != nil {
		t.Fatalf("jcs: %v", err)
	}
	if string(got) != want {
		t.Errorf("canonical mismatch\n got: %s\nwant: %s", got, want)
	}
}

// TestJCSFieldOrderStable confirms that member order in the input does not
// affect the canonical output.
func TestJCSFieldOrderStable(t *testing.T) {
	a := `{"b":1,"a":2,"c":{"z":3,"y":4}}`
	b := `{"c":{"y":4,"z":3},"a":2,"b":1}`
	ca, err := jcs([]byte(a))
	if err != nil {
		t.Fatalf("jcs(a): %v", err)
	}
	cb, err := jcs([]byte(b))
	if err != nil {
		t.Fatalf("jcs(b): %v", err)
	}
	if string(ca) != string(cb) {
		t.Errorf("field order changed canonical form:\n a: %s\n b: %s", ca, cb)
	}
	if want := `{"a":2,"b":1,"c":{"y":4,"z":3}}`; string(ca) != want {
		t.Errorf("unexpected canonical form: got %s want %s", ca, want)
	}
}

// TestJCSUnicodeStable confirms an escaped input and its literal spelling
// canonicalize identically, and that non-ASCII is emitted as literal UTF-8
// (not \u-escaped as encoding/json's ASCII-safe modes would).
func TestJCSUnicodeStable(t *testing.T) {
	// Escaped-input bytes built at runtime ("\\u" is a literal backslash+u, i.e.
	// the six-byte escape text, not a Go rune escape).
	escaped := []byte("{\"k\":\"caf\\u00e9 \\u20ac\"}")
	literal := `{"k":"café €"}`
	want := `{"k":"café €"}`

	ce, err := jcs(escaped)
	if err != nil {
		t.Fatalf("jcs(escaped): %v", err)
	}
	cl, err := jcs([]byte(literal))
	if err != nil {
		t.Fatalf("jcs(literal): %v", err)
	}
	if string(ce) != string(cl) {
		t.Errorf("escaped and literal disagree:\n escaped: %s\n literal: %s", ce, cl)
	}
	if string(ce) != want {
		t.Errorf("non-ASCII not emitted literally: got %s want %s", ce, want)
	}
}

// TestJCSNoHTMLEscape confirms JCS leaves '<', '>', and '&' literal, unlike
// encoding/json's default HTML-escaping output.
func TestJCSNoHTMLEscape(t *testing.T) {
	in := `{"k":"a<b>c&d"}`
	got, err := jcs([]byte(in))
	if err != nil {
		t.Fatalf("jcs: %v", err)
	}
	if string(got) != in {
		t.Errorf("HTML characters not left literal\n got: %s\nwant: %s", got, in)
	}
}

// TestJCSControlEscapes checks the C0 control-character escaping rules: the
// short letter escapes are used where defined, solidus is not escaped, and the
// remaining controls use lowercase \u00XX (verified via a marshal round-trip so
// no \uXXXX escape text is hand-typed).
func TestJCSControlEscapes(t *testing.T) {
	// Letter escapes and solidus (no \uXXXX): safe as raw-string literals.
	in := `{"k":" \b\t\n\f\r\"\\\/"}`
	want := `{"k":" \b\t\n\f\r\"\\/"}`
	got, err := jcs([]byte(in))
	if err != nil {
		t.Fatalf("jcs: %v", err)
	}
	if string(got) != want {
		t.Errorf("control escape mismatch\n got: %s\nwant: %s", got, want)
	}

	// Low controls without a letter escape must render as lowercase \u00XX.
	// encoding/json emits exactly that for these, so jcs must be a no-op on it.
	lows := "\x00\x01\x1e\x1f"
	marshaled, err := json.Marshal(map[string]string{"k": lows})
	if err != nil {
		t.Fatalf("marshal lows: %v", err)
	}
	got2, err := jcs(marshaled)
	if err != nil {
		t.Fatalf("jcs(lows): %v", err)
	}
	if string(got2) != string(marshaled) {
		t.Errorf("low-control escaping diverged\n got: %s\nwant: %s", got2, marshaled)
	}
}

// TestJCSIdempotent confirms canonicalizing already-canonical bytes is a no-op.
func TestJCSIdempotent(t *testing.T) {
	inputs := []string{
		`{"b":1,"a":[1,2,{"y":true,"x":null}]}`,
		`[1e30,4.5,0.002,1e-27,-0,100,-5]`,
		`"just a string with € and \n"`,
		`{"numbers":[333333333.33333329,1E30,4.50,2e-3,0.000000000000000000000000001]}`,
	}
	for _, in := range inputs {
		once, err := jcs([]byte(in))
		if err != nil {
			t.Fatalf("jcs(%s): %v", in, err)
		}
		twice, err := jcs(once)
		if err != nil {
			t.Fatalf("jcs(jcs(%s)): %v", in, err)
		}
		if string(once) != string(twice) {
			t.Errorf("not idempotent for %s:\n once:  %s\n twice: %s", in, once, twice)
		}
	}
}

// TestFormatFloatES spot-checks the ECMAScript number formatter against the
// RFC 8785 §3.2.3 values plus integer and threshold cases.
func TestFormatFloatES(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{math.Copysign(0, -1), "0"}, // negative zero collapses to "0"
		{1, "1"},
		{-5, "-5"},
		{100, "100"},
		{836284937, "836284937"},
		{333333333.33333329, "333333333.3333333"},
		{1e30, "1e+30"},
		{4.50, "4.5"},
		{2e-3, "0.002"},
		{1e-27, "1e-27"},
		{1e21, "1e+21"},                 // first exponent that switches to exponential
		{1e20, "100000000000000000000"}, // last that stays fixed
		{1e-6, "0.000001"},
		{1e-7, "1e-7"},
		{-1.5e-27, "-1.5e-27"},
	}
	for _, c := range cases {
		got, err := formatFloatES(c.in)
		if err != nil {
			t.Fatalf("formatFloatES(%v): %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("formatFloatES(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestCanonicalOmitsSig confirms the signature never appears in canonical bytes
// and that a change in Sig leaves the canonical form untouched.
func TestCanonicalOmitsSig(t *testing.T) {
	e := sampleEnvelope()
	base, err := Canonical(e)
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	if strings.Contains(string(base), `"sig"`) {
		t.Errorf("canonical form contains sig field: %s", base)
	}
	e.Sig = []byte("some signature bytes")
	withSig, err := Canonical(e)
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	if string(base) != string(withSig) {
		t.Errorf("Sig changed canonical form:\n without: %s\n with:    %s", base, withSig)
	}
	if !json.Valid(base) {
		t.Errorf("canonical form is not valid JSON: %s", base)
	}
}
