package envelope

import (
	"encoding/json"
	"testing"
)

// FuzzCanonical checks that canonicalization never panics on arbitrary input,
// always yields valid JSON when it succeeds, and is idempotent (canonicalizing
// canonical bytes is a no-op). Idempotency is the property Sign/Verify rely on.
func FuzzCanonical(f *testing.F) {
	seeds := []string{
		`{}`,
		`[]`,
		`null`,
		`{"a":1,"b":[true,null,"x"]}`,
		`{"z":1,"a":2,"m":3}`,
		`"€ text with \n and \" and /"`,
		`123.456`,
		`1e-27`,
		`{"num":333333333.33333329,"e":1e30,"small":2e-3}`,
		`{"nested":{"deep":{"x":[1,2,3]}}}`,
		`{"dup":1}`,
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		out, err := jcs(data)
		if err != nil {
			return // not valid JSON — acceptable
		}
		if !json.Valid(out) {
			t.Fatalf("canonical output is not valid JSON: %q (from %q)", out, data)
		}
		again, err := jcs(out)
		if err != nil {
			t.Fatalf("re-canonicalizing failed: %v (out=%q)", err, out)
		}
		if string(again) != string(out) {
			t.Fatalf("not idempotent:\n once:  %q\n twice: %q", out, again)
		}
	})
}

// FuzzDecode checks that decoding never panics and that any envelope it accepts
// is at a supported version and can be canonicalized.
func FuzzDecode(f *testing.F) {
	sample, err := json.Marshal(sampleEnvelope())
	if err != nil {
		f.Fatalf("seed marshal: %v", err)
	}
	f.Add(sample)
	f.Add([]byte(`{"v":1,"id":"a","thread":"b","to":"m","state":"submitted","sent_at":"2026-07-16T09:30:00Z","ai_generated":false,"body":{"role":"user","parts":[]}}`))
	f.Add([]byte(`{"v":2}`))
	f.Add([]byte(`garbage`))
	f.Add([]byte(``))

	f.Fuzz(func(t *testing.T, data []byte) {
		e, err := Decode(data)
		if err != nil {
			return
		}
		if !versionSupported(e.V) {
			t.Fatalf("Decode accepted unsupported version %d", e.V)
		}
		if _, err := Canonical(e); err != nil {
			t.Fatalf("Canonical of decoded envelope failed: %v", err)
		}
	})
}

// FuzzDecodeCanonicalStable is the wire-stability property that Verify depends on
// (canon.go / sign.go): for any envelope Decode accepts, marshaling it back to
// the wire and decoding again must reproduce the identical canonical form. If it
// does not, a legitimately signed envelope could fail verification after a
// benign relay round trip — a correctness break, not just a fuzz crash.
func FuzzDecodeCanonicalStable(f *testing.F) {
	sample, err := json.Marshal(sampleEnvelope())
	if err != nil {
		f.Fatalf("seed marshal: %v", err)
	}
	f.Add(sample)
	f.Add([]byte(`{"v":1,"id":"a","thread":"b","to":"m","state":"submitted","sent_at":"2026-07-16T09:30:00.500Z","ai_generated":true,"body":{"role":"agent","parts":[{"type":"text","text":"€ "}]},"sig":"AAAA"}`))
	f.Add([]byte(`{"v":1,"id":"","thread":"","to":"","state":"","sent_at":"0001-01-01T00:00:00Z","ai_generated":false,"body":{"role":"","parts":null}}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		e1, err := Decode(data)
		if err != nil {
			return
		}
		c1, err := Canonical(e1)
		if err != nil {
			t.Fatalf("Canonical(e1): %v", err)
		}
		raw2, err := json.Marshal(e1)
		if err != nil {
			t.Fatalf("re-marshal accepted envelope: %v", err)
		}
		e2, err := Decode(raw2)
		if err != nil {
			t.Fatalf("re-decode of our own marshaled envelope failed: %v (raw=%q)", err, raw2)
		}
		c2, err := Canonical(e2)
		if err != nil {
			t.Fatalf("Canonical(e2): %v", err)
		}
		if string(c1) != string(c2) {
			t.Fatalf("canonical form not stable across a wire round trip:\n c1: %q\n c2: %q", c1, c2)
		}
	})
}
