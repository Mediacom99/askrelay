package envelope

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
)

// U+2028 LINE SEPARATOR / U+2029 PARAGRAPH SEPARATOR, built from rune values so
// the exact source bytes are unambiguous. encoding/json escapes these on output;
// RFC 8785 (JCS) requires them emitted as literal UTF-8.
var (
	lineSep = string(rune(0x2028))
	paraSep = string(rune(0x2029))
)

// jsonEscapeOf returns exactly how encoding/json spells s on the wire, without
// the surrounding quotes — so tests never hardcode an escape spelling.
func jsonEscapeOf(t *testing.T, s string) []byte {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal %q: %v", s, err)
	}
	return bytes.Trim(b, `"`)
}

// --- Whole-artifact caps (T-16): a sender must not be able to smuggle bulk
// past MaxBodyBytes by inflating non-body fields, the part count, or the whole
// envelope. Before T-16 the cap bounded only the sum of Part.Text bytes, so a
// 4 MB envelope with tiny body text signed and verified; these tests now prove
// each such attempt is rejected — by ErrTooLarge (whole-wire, MaxWireBytes) or
// ErrTooManyParts (part count, MaxParts) — in both the Sign and Verify path. ---

func TestExfilByStructure_Rejected(t *testing.T) {
	pub, priv := keypair(t)
	const bulk = 1 << 20 // 1 MiB of smuggled bytes
	giant := strings.Repeat("A", bulk)

	cases := []struct {
		name    string
		mutate  func(*Envelope)
		wantErr error
	}{
		{"giant Part.Type", func(e *Envelope) {
			e.Body.Parts = []Part{{Type: giant, Text: "hi"}}
		}, ErrTooLarge},
		{"giant To (a person field)", func(e *Envelope) { e.To = giant }, ErrTooLarge},
		{"giant From.Person", func(e *Envelope) { e.From.Person = giant }, ErrTooLarge},
		{"giant From.Device", func(e *Envelope) { e.From.Device = giant }, ErrTooLarge},
		{"giant From.Agent", func(e *Envelope) { e.From.Agent = giant }, ErrTooLarge},
		{"giant ID", func(e *Envelope) { e.ID = giant }, ErrTooLarge},
		{"giant Thread", func(e *Envelope) { e.Thread = giant }, ErrTooLarge},
		{"many empty-text parts", func(e *Envelope) {
			parts := make([]Part, 200_000)
			for i := range parts {
				parts[i] = Part{Type: "text", Text: ""}
			}
			e.Body.Parts = parts
		}, ErrTooManyParts},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := sampleEnvelope()
			c.mutate(&e)

			// The body-text cap alone does not catch these (that was the pre-T-16
			// gap): bodySize stays tiny. The whole-artifact caps do.
			if got := bodySize(e.Body); got > MaxBodyBytes {
				t.Fatalf("precondition: bodySize %d over body cap; not an exfil-by-structure case", got)
			}
			if err := Sign(&e, priv); !errors.Is(err, c.wantErr) {
				t.Errorf("Sign = %v, want %v", err, c.wantErr)
			}
			// Verify rejects on the same cap regardless of signature state.
			if err := Verify(e, pub); !errors.Is(err, c.wantErr) {
				t.Errorf("Verify = %v, want %v", err, c.wantErr)
			}
		})
	}
}

// TestWholeEnvelopeSizeBounded is the headline case: an envelope whose canonical
// form far exceeds MaxBodyBytes (4 MiB Part.Type, tiny body text) is now
// rejected by the whole-wire cap on both Sign and Verify.
func TestWholeEnvelopeSizeBounded(t *testing.T) {
	pub, priv := keypair(t)
	e := sampleEnvelope()
	// One part, tiny text, but a multi-megabyte Type string.
	e.Body.Parts = []Part{{Type: strings.Repeat("Z", 4<<20), Text: "ok"}}

	if bodySize(e.Body) > MaxBodyBytes {
		t.Fatalf("bodySize %d unexpectedly over body cap", bodySize(e.Body))
	}
	// Pre-T-16 this signed and verified; the whole-wire cap now rejects it.
	if err := Sign(&e, priv); !errors.Is(err, ErrTooLarge) {
		t.Errorf("Sign 4 MiB envelope = %v, want ErrTooLarge", err)
	}
	if err := Verify(e, pub); !errors.Is(err, ErrTooLarge) {
		t.Errorf("Verify 4 MiB envelope = %v, want ErrTooLarge", err)
	}
}

// --- Soft spot 2: verification re-canonicalizes the decoded struct, never the
// raw received bytes. These construct wire variants that would fail if someone
// verified raw bytes, and confirm decode+re-canonicalize is byte-stable. ---

// signedWire returns a signed sample envelope, its wire JSON, and the pubkey.
func signedWire(t *testing.T) (Envelope, []byte, ed25519.PublicKey) {
	t.Helper()
	pub, priv := keypair(t)
	e := sampleEnvelope()
	if err := Sign(&e, priv); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	return e, raw, pub
}

func TestVerifyReCanonicalizesNotRawBytes(t *testing.T) {
	signed, raw, pub := signedWire(t)
	wantCanon, err := Canonical(signed)
	if err != nil {
		t.Fatalf("Canonical(signed): %v", err)
	}

	// reorderTopLevel unmarshals into a map and re-marshals: encoding/json emits
	// map keys in sorted order, a DIFFERENT byte order than the struct produced.
	reorderTopLevel := func(b []byte) []byte {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatalf("reorder unmarshal: %v", err)
		}
		out, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("reorder marshal: %v", err)
		}
		return out
	}
	addWhitespace := func(b []byte) []byte {
		var buf bytes.Buffer
		if err := json.Indent(&buf, b, "", "  "); err != nil {
			t.Fatalf("indent: %v", err)
		}
		return buf.Bytes()
	}
	addUnknownField := func(b []byte) []byte {
		// Insert unknown top-level fields right after the opening brace (§12
		// forward-compat: unknown fields are dropped on decode).
		injected := `{"z_future_field":{"nested":[1,2,3]},"another_unknown":true,`
		return append([]byte(injected), b[1:]...)
	}
	tsToPlusZero := func(b []byte) []byte {
		// Same instant, "+00:00" spelling instead of "Z" — Go normalizes both to
		// "Z" on re-marshal, so it re-canonicalizes identically.
		return bytes.Replace(b, []byte("2026-07-16T09:30:00Z"), []byte("2026-07-16T09:30:00+00:00"), 1)
	}

	variants := []struct {
		name  string
		xform func([]byte) []byte
	}{
		{"reordered top-level keys", reorderTopLevel},
		{"insignificant whitespace", addWhitespace},
		{"unknown extra fields", addUnknownField},
		{"sent_at +00:00 vs Z", tsToPlusZero},
	}
	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			wire := v.xform(raw)
			if bytes.Equal(wire, raw) {
				t.Fatalf("precondition: transform did not change the wire bytes")
			}
			got, err := Decode(wire)
			if err != nil {
				t.Fatalf("Decode variant: %v", err)
			}
			// Byte-stable re-canonicalization: identical to the originally signed form.
			gotCanon, err := Canonical(got)
			if err != nil {
				t.Fatalf("Canonical(got): %v", err)
			}
			if !bytes.Equal(gotCanon, wantCanon) {
				t.Errorf("canonical form not byte-stable across %s:\n want: %s\n got:  %s", v.name, wantCanon, gotCanon)
			}
			// And the signature still verifies despite the raw bytes differing.
			if err := Verify(got, pub); err != nil {
				t.Errorf("Verify after %s = %v, want nil (verifying raw bytes would have failed here)", v.name, err)
			}
		})
	}
}

// TestVerifyUnicodeLiteralVsEscapedWire signs a body containing U+2028/U+2029
// (which encoding/json escapes on the wire) and confirms a wire variant using
// the literal 3-byte UTF-8 sequences decodes and verifies identically — the
// exact case that breaks if a verifier compares raw bytes.
func TestVerifyUnicodeLiteralVsEscapedWire(t *testing.T) {
	pub, priv := keypair(t)
	e := sampleEnvelope()
	e.Body.Parts = []Part{{Type: "text", Text: "line" + lineSep + "para" + paraSep + "end"}}
	if err := Sign(&e, priv); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	escLine := jsonEscapeOf(t, lineSep)
	escPara := jsonEscapeOf(t, paraSep)
	// encoding/json must have escaped the separators on the wire.
	if !bytes.Contains(raw, escLine) || !bytes.Contains(raw, escPara) {
		t.Fatalf("precondition: expected encoding/json to escape U+2028/9 on the wire; wire=%s", raw)
	}
	// Rewrite the wire to use the literal UTF-8 bytes instead of the escapes.
	literal := bytes.ReplaceAll(raw, escLine, []byte(lineSep))
	literal = bytes.ReplaceAll(literal, escPara, []byte(paraSep))
	if bytes.Equal(literal, raw) {
		t.Fatalf("precondition: literal rewrite changed nothing")
	}
	got, err := Decode(literal)
	if err != nil {
		t.Fatalf("Decode literal-unicode wire: %v", err)
	}
	if err := Verify(got, pub); err != nil {
		t.Errorf("Verify literal-unicode wire = %v, want nil", err)
	}
}

// TestVerifyRejectsSemanticTimestampRewrite confirms a same-instant but
// different-offset timestamp ("+01:00") does NOT verify: Go re-marshals it with
// the numeric offset, so the canonical bytes differ from what was signed. This
// is correct (a raw-byte verifier would also reject it) and bounds the previous
// test's "+00:00" equivalence claim.
func TestVerifyRejectsSemanticTimestampRewrite(t *testing.T) {
	_, raw, pub := signedWire(t)
	wire := bytes.Replace(raw, []byte("2026-07-16T09:30:00Z"), []byte("2026-07-16T10:30:00+01:00"), 1)
	if bytes.Equal(wire, raw) {
		t.Fatalf("precondition: timestamp rewrite changed nothing")
	}
	got, err := Decode(wire)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if err := Verify(got, pub); !errors.Is(err, ErrBadSignature) {
		t.Errorf("Verify semantic timestamp rewrite = %v, want ErrBadSignature", err)
	}
}

// --- Error precedence: check() runs version → part-count → body-text →
// whole-wire, all before any signature work; then public-key / signature
// length; then the Ed25519 check. Both byte caps report ErrTooLarge; the
// part-count cap reports ErrTooManyParts. ---

func TestErrorPrecedence(t *testing.T) {
	pub, priv := keypair(t)
	oversizeText := strings.Repeat("a", MaxBodyBytes+1)
	tooManyParts := func() []Part {
		p := make([]Part, MaxParts+1)
		for i := range p {
			p[i] = Part{Type: "text", Text: "x"}
		}
		return p
	}

	cases := []struct {
		name    string
		mutate  func(*Envelope)
		verWith ed25519.PublicKey
		want    error
	}{
		{
			name: "version beats everything",
			mutate: func(e *Envelope) {
				e.V = 2                       // bad version
				e.Body.Parts = tooManyParts() // also too many parts
				e.Sig[0] ^= 0xff              // also tampered sig
			},
			verWith: pub,
			want:    ErrBadVersion,
		},
		{
			name: "part-count beats size and signature",
			mutate: func(e *Envelope) {
				p := tooManyParts()
				p[0].Text = oversizeText // also over the body-text cap
				e.Body.Parts = p
				e.Sig[0] ^= 0xff // also tampered sig
			},
			verWith: pub,
			want:    ErrTooManyParts,
		},
		{
			name: "body-text beats signature",
			mutate: func(e *Envelope) {
				e.Body.Parts = []Part{{Type: "text", Text: oversizeText}}
				e.Sig[0] ^= 0xff
			},
			verWith: pub,
			want:    ErrTooLarge,
		},
		{
			name: "whole-wire beats signature",
			mutate: func(e *Envelope) {
				// Within the part-count and body-text caps, but metadata pushes the
				// canonical form over MaxWireBytes.
				e.From.Agent = strings.Repeat("a", MaxWireBytes)
				e.Sig[0] ^= 0xff
			},
			verWith: pub,
			want:    ErrTooLarge,
		},
		{
			name:    "nil sig within caps",
			mutate:  func(e *Envelope) { e.Sig = nil },
			verWith: pub,
			want:    ErrBadSignature,
		},
		{
			name:    "short public key within caps",
			mutate:  func(_ *Envelope) {},
			verWith: ed25519.PublicKey{1, 2, 3},
			want:    ErrBadSignature,
		},
		{
			name:    "caps beat short public key",
			mutate:  func(e *Envelope) { e.Body.Parts = []Part{{Type: "text", Text: oversizeText}} },
			verWith: ed25519.PublicKey{1, 2, 3},
			want:    ErrTooLarge,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := sampleEnvelope()
			if err := Sign(&e, priv); err != nil {
				t.Fatalf("Sign: %v", err)
			}
			c.mutate(&e)
			if err := Verify(e, c.verWith); !errors.Is(err, c.want) {
				t.Errorf("Verify = %v, want %v", err, c.want)
			}
		})
	}
}

// --- Nested-part tamper matrix on a multi-part body (the existing matrix uses a
// single part). ---

func TestMultiPartTamper(t *testing.T) {
	pub, priv := keypair(t)
	base := func() Envelope {
		e := sampleEnvelope()
		e.Body.Parts = []Part{
			{Type: "text", Text: "first"},
			{Type: "text", Text: "second"},
			{Type: "text", Text: "third"},
		}
		return e
	}
	cases := []struct {
		name   string
		mutate func(*Envelope)
	}{
		{"mutate middle text", func(e *Envelope) { e.Body.Parts[1].Text = "SECOND" }},
		{"mutate middle type", func(e *Envelope) { e.Body.Parts[1].Type = "code" }},
		{"reorder parts", func(e *Envelope) {
			e.Body.Parts[0], e.Body.Parts[2] = e.Body.Parts[2], e.Body.Parts[0]
		}},
		{"drop last part", func(e *Envelope) { e.Body.Parts = e.Body.Parts[:2] }},
		{"insert part in middle", func(e *Envelope) {
			p := append([]Part{e.Body.Parts[0], {Type: "text", Text: "injected"}}, e.Body.Parts[1:]...)
			e.Body.Parts = p
		}},
		{"duplicate a part", func(e *Envelope) {
			e.Body.Parts = append(e.Body.Parts, e.Body.Parts[0])
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := base()
			if err := Sign(&e, priv); err != nil {
				t.Fatalf("Sign: %v", err)
			}
			c.mutate(&e)
			if err := Verify(e, pub); !errors.Is(err, ErrBadSignature) {
				t.Errorf("Verify after %q = %v, want ErrBadSignature", c.name, err)
			}
		})
	}
}

// --- Key robustness: wrong/short/nil/zero/oversized keys never panic and are a
// signature verdict; a garbage-but-correct-length signing key yields a signature
// that does not verify against the real public key (no silent accept). ---

func TestVerifyMalformedKeysNeverPanic(t *testing.T) {
	_, priv := keypair(t)
	e := sampleEnvelope()
	if err := Sign(&e, priv); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	zero := make(ed25519.PublicKey, ed25519.PublicKeySize)
	allFF := bytes.Repeat([]byte{0xff}, ed25519.PublicKeySize)
	oversized := bytes.Repeat([]byte{0x01}, ed25519.PublicKeySize+1)

	keys := map[string]ed25519.PublicKey{
		"nil":                  nil,
		"empty":                {},
		"short":                {1, 2, 3},
		"all-zero (valid len)": zero,
		"all-0xff (valid len)": ed25519.PublicKey(allFF),
		"oversized":            ed25519.PublicKey(oversized),
	}
	for name, k := range keys {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Verify panicked on %s key: %v", name, r)
				}
			}()
			if err := Verify(e, k); !errors.Is(err, ErrBadSignature) {
				t.Errorf("Verify with %s key = %v, want ErrBadSignature", name, err)
			}
		})
	}
}

func TestSignWithGarbageKeyDoesNotSilentlyAccept(t *testing.T) {
	realPub, _ := keypair(t)
	e := sampleEnvelope()
	// All-zero private key of the correct length: Sign succeeds (Go does not
	// validate key material) but the signature must not verify under the real key.
	garbage := make(ed25519.PrivateKey, ed25519.PrivateKeySize)
	if err := Sign(&e, garbage); err != nil {
		t.Fatalf("Sign with correct-length garbage key errored unexpectedly: %v", err)
	}
	if err := Verify(e, realPub); !errors.Is(err, ErrBadSignature) {
		t.Errorf("garbage-key signature verified under real key = %v, want ErrBadSignature", err)
	}
}

// TestReSignIsDeterministic confirms re-signing an already-signed envelope
// recomputes over the sig-less canonical form (Ed25519 is deterministic → same
// bytes) and still verifies.
func TestReSignIsDeterministic(t *testing.T) {
	pub, priv := keypair(t)
	e := sampleEnvelope()
	if err := Sign(&e, priv); err != nil {
		t.Fatalf("Sign #1: %v", err)
	}
	first := append([]byte(nil), e.Sig...)
	if err := Sign(&e, priv); err != nil {
		t.Fatalf("Sign #2: %v", err)
	}
	if !bytes.Equal(first, e.Sig) {
		t.Errorf("re-sign changed the signature (sig field leaked into canonical form?)")
	}
	if err := Verify(e, pub); err != nil {
		t.Errorf("Verify after re-sign: %v", err)
	}
}

// TestSignVerifyEmptyBody characterizes the zero-body edge: no parts / empty
// message have bodySize 0 and sign+verify cleanly.
func TestSignVerifyEmptyBody(t *testing.T) {
	pub, priv := keypair(t)
	for _, body := range []Message{
		{Role: "agent", Parts: nil},
		{Role: "agent", Parts: []Part{}},
		{},
	} {
		e := sampleEnvelope()
		e.Body = body
		if err := Sign(&e, priv); err != nil {
			t.Fatalf("Sign empty body %+v: %v", body, err)
		}
		if err := Verify(e, pub); err != nil {
			t.Errorf("Verify empty body %+v: %v", body, err)
		}
	}
}

// --- Decode hardening: duplicate fields, deeply nested input. ---

func TestDecodeDuplicateFields(t *testing.T) {
	// encoding/json takes the last value for duplicate keys; confirm Decode is
	// consistent (an attacker cannot smuggle a shadow value that survives).
	raw := `{"v":1,"id":"a","thread":"b","to":"marco","to":"eve",` +
		`"state":"submitted","sent_at":"2026-07-16T09:30:00Z","ai_generated":false,` +
		`"body":{"role":"user","parts":[{"type":"text","text":"x"}]}}`
	e, err := Decode([]byte(raw))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if e.To != "eve" {
		t.Errorf("duplicate field: To = %q, want last value %q", e.To, "eve")
	}
	// Canonicalizing the decoded struct is unambiguous (single value).
	c14n, err := Canonical(e)
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	if bytes.Count(c14n, []byte(`"to":`)) != 1 {
		t.Errorf("canonical form has ambiguous 'to' field: %s", c14n)
	}
}

func TestDecodeDeeplyNestedDoesNotOverflow(t *testing.T) {
	// A pathologically deep array must be rejected by the stdlib depth guard, not
	// blow the stack. Guards both Decode and the canonicalizer's parser.
	const depth = 200_000
	deep := strings.Repeat("[", depth) + strings.Repeat("]", depth)
	env := `{"v":1,"id":"a","thread":"b","to":"m","state":"submitted",` +
		`"sent_at":"2026-07-16T09:30:00Z","ai_generated":false,` +
		`"body":{"role":"user","parts":` + deep + `}}`

	if _, err := Decode([]byte(env)); err == nil {
		t.Errorf("Decode accepted pathologically deep input")
	}
	if _, err := jcs([]byte(deep)); err == nil {
		t.Errorf("jcs accepted pathologically deep input")
	}
}

// --- JCS edge cases: surrogate-pair key ordering, U+2028/2029 literals, and
// non-finite numbers. ---

// TestJCSSurrogatePairKeyOrder verifies RFC 8785 UTF-16 member ordering: a
// supplementary-plane key (U+10000, encoded as the surrogate pair 0xD800 0xDC00)
// sorts BEFORE a BMP key (U+FFFF = 0xFFFF), which naive code-point/byte ordering
// would get backwards.
func TestJCSSurrogatePairKeyOrder(t *testing.T) {
	supp := string(rune(0x10000)) // supplementary plane
	bmpMax := string(rune(0xFFFF))
	in := map[string]any{supp: 2, bmpMax: 1, "a": 3}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got, err := jcs(raw)
	if err != nil {
		t.Fatalf("jcs: %v", err)
	}
	want := `{"a":3,"` + supp + `":2,"` + bmpMax + `":1}`
	if string(got) != want {
		t.Errorf("surrogate ordering wrong (UTF-16 order required)\n got: %q\nwant: %q", got, want)
	}
}

func TestJCSLineParagraphSeparatorsLiteral(t *testing.T) {
	// JCS must emit U+2028/U+2029 as literal UTF-8, not as escapes.
	escLine := jsonEscapeOf(t, lineSep)
	escPara := jsonEscapeOf(t, paraSep)
	escapedIn := []byte(`{"k":"` + string(escLine) + string(escPara) + `"}`)
	literalIn := []byte(`{"k":"` + lineSep + paraSep + `"}`)

	esc, err := jcs(escapedIn)
	if err != nil {
		t.Fatalf("jcs(escaped): %v", err)
	}
	lit, err := jcs(literalIn)
	if err != nil {
		t.Fatalf("jcs(literal): %v", err)
	}
	if !bytes.Equal(esc, lit) {
		t.Errorf("escaped and literal U+2028/9 disagree:\n esc: %q\n lit: %q", esc, lit)
	}
	if bytes.Contains(esc, escLine) || bytes.Contains(esc, escPara) {
		t.Errorf("JCS output escaped U+2028/9 instead of emitting literal UTF-8: %q", esc)
	}
	if !bytes.Contains(esc, []byte(lineSep)) || !bytes.Contains(esc, []byte(paraSep)) {
		t.Errorf("JCS output missing literal U+2028/9 bytes: %q", esc)
	}
}

func TestJCSRejectsNonFiniteNumbers(t *testing.T) {
	for _, in := range []string{`1e400`, `-1e400`, `[1e999,2]`, `{"x":1e400}`, `1E308000`} {
		if _, err := jcs([]byte(in)); err == nil {
			t.Errorf("jcs(%q) accepted a non-finite number", in)
		}
	}
}

// --- Concurrency: New / Sign / Verify / Canonical from many goroutines under
// -race. New's UUIDv7 minting and the pure canonicalization path must be safe. ---

func TestConcurrentSignVerify(t *testing.T) {
	pub, priv := keypair(t)

	// A shared, read-only signed envelope for concurrent Verify/Canonical.
	shared := sampleEnvelope()
	if err := Sign(&shared, priv); err != nil {
		t.Fatalf("Sign shared: %v", err)
	}
	sharedCanon, err := Canonical(shared)
	if err != nil {
		t.Fatalf("Canonical shared: %v", err)
	}

	const workers = 64
	const iters = 50
	var wg sync.WaitGroup
	errCh := make(chan error, workers*2)

	// Group 1: each goroutine builds, signs, and verifies its OWN envelope.
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				body := Message{Role: "agent", Parts: []Part{{Type: "text", Text: "concurrent"}}}
				e := New(Party{Person: "edo", Device: "d", Agent: "cc"}, "marco", "", StateSubmitted, true, body)
				if err := Sign(&e, priv); err != nil {
					errCh <- err
					return
				}
				if err := Verify(e, pub); err != nil {
					errCh <- err
					return
				}
			}
		}()
	}
	// Group 2: concurrent read-only Verify + Canonical of the shared envelope.
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				if err := Verify(shared, pub); err != nil {
					errCh <- err
					return
				}
				c, err := Canonical(shared)
				if err != nil {
					errCh <- err
					return
				}
				if !bytes.Equal(c, sharedCanon) {
					errCh <- errors.New("concurrent Canonical produced divergent bytes")
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent op failed: %v", err)
	}
}
