package envelope

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Canonical returns the RFC 8785 (JCS) canonical serialization of e with the
// Sig field omitted. This is the exact byte sequence that Sign signs and Verify
// checks: object keys sorted by UTF-16 code unit, no insignificant whitespace,
// ECMAScript number formatting, and JCS string escaping.
func Canonical(e Envelope) ([]byte, error) {
	e.Sig = nil // signature is never part of the signed form (omitempty drops it)
	raw, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("envelope: marshal for canonicalization: %w", err)
	}
	return jcs(raw)
}

// jcs canonicalizes an arbitrary JSON document per RFC 8785. It parses with
// number fidelity preserved, then re-serializes in canonical form.
func jcs(data []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("envelope: parse JSON for canonicalization: %w", err)
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, errors.New("envelope: trailing data in JSON for canonicalization")
	}
	return appendCanonical(nil, v)
}

// appendCanonical writes the canonical form of a decoded JSON value to dst.
func appendCanonical(dst []byte, v any) ([]byte, error) {
	switch t := v.(type) {
	case nil:
		return append(dst, "null"...), nil
	case bool:
		if t {
			return append(dst, "true"...), nil
		}
		return append(dst, "false"...), nil
	case string:
		return appendString(dst, t), nil
	case json.Number:
		return appendNumber(dst, t)
	case []any:
		dst = append(dst, '[')
		for i, e := range t {
			if i > 0 {
				dst = append(dst, ',')
			}
			var err error
			if dst, err = appendCanonical(dst, e); err != nil {
				return dst, err
			}
		}
		return append(dst, ']'), nil
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		slices.SortFunc(keys, compareUTF16)
		dst = append(dst, '{')
		for i, k := range keys {
			if i > 0 {
				dst = append(dst, ',')
			}
			dst = appendString(dst, k)
			dst = append(dst, ':')
			var err error
			if dst, err = appendCanonical(dst, t[k]); err != nil {
				return dst, err
			}
		}
		return append(dst, '}'), nil
	default:
		return dst, fmt.Errorf("envelope: unsupported JSON type %T in canonicalization", v)
	}
}

const hexdigits = "0123456789abcdef"

// appendString writes s as a JCS-escaped JSON string (RFC 8785 §3.2.2.2):
// the seven short escapes, \u00XX (lowercase) for the remaining C0 controls,
// and every other character — including non-ASCII and solidus — as literal
// UTF-8. Notably it does not escape '/', '<', '>', '&', U+2028, or U+2029, so
// it must not be replaced by encoding/json for values.
func appendString(dst []byte, s string) []byte {
	dst = append(dst, '"')
	for _, r := range s {
		switch r {
		case '"':
			dst = append(dst, '\\', '"')
		case '\\':
			dst = append(dst, '\\', '\\')
		case '\b':
			dst = append(dst, '\\', 'b')
		case '\f':
			dst = append(dst, '\\', 'f')
		case '\n':
			dst = append(dst, '\\', 'n')
		case '\r':
			dst = append(dst, '\\', 'r')
		case '\t':
			dst = append(dst, '\\', 't')
		default:
			if r < 0x20 {
				dst = append(dst, '\\', 'u', '0', '0', hexdigits[r>>4], hexdigits[r&0xf])
			} else {
				dst = utf8.AppendRune(dst, r)
			}
		}
	}
	return append(dst, '"')
}

// appendNumber writes n using ECMAScript Number formatting, treating every JSON
// number as an IEEE-754 double as RFC 8785 requires.
func appendNumber(dst []byte, n json.Number) ([]byte, error) {
	f, err := strconv.ParseFloat(n.String(), 64)
	if err != nil {
		return dst, fmt.Errorf("envelope: invalid number %q: %w", n.String(), err)
	}
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return dst, fmt.Errorf("envelope: non-finite number %q", n.String())
	}
	s, err := formatFloatES(f)
	if err != nil {
		return dst, err
	}
	return append(dst, s...), nil
}

// formatFloatES renders f as ECMAScript's Number.prototype.toString would
// (the number production RFC 8785 defers to). It derives the shortest
// round-trip significant digits from strconv and applies the ES fixed/
// exponential thresholds (fixed for exponents in (-6, 21], exponential
// otherwise).
func formatFloatES(f float64) (string, error) {
	if f == 0 {
		return "0", nil // also collapses -0 to "0", matching ES
	}
	neg := f < 0
	af := f
	if neg {
		af = -f
	}
	// strconv 'e' with prec -1 yields the shortest "d[.ddd]e±dd" form.
	mant, expPart, _ := strings.Cut(strconv.FormatFloat(af, 'e', -1, 64), "e")
	exp, err := strconv.Atoi(expPart)
	if err != nil {
		return "", fmt.Errorf("envelope: parse exponent %q: %w", expPart, err)
	}
	digits := strings.Replace(mant, ".", "", 1) // significant digits, no point
	k := len(digits)                            // digit count
	n := exp + 1                                // position of the decimal point

	var out string
	switch {
	case n >= 1 && n <= 21:
		if k <= n {
			out = digits + strings.Repeat("0", n-k)
		} else {
			out = digits[:n] + "." + digits[n:]
		}
	case n <= 0 && n > -6:
		out = "0." + strings.Repeat("0", -n) + digits
	default: // n > 21 || n <= -6 → exponential
		m := digits
		if k > 1 {
			m = digits[:1] + "." + digits[1:]
		}
		e := n - 1
		sign := "+"
		if e < 0 {
			sign = "-"
			e = -e
		}
		out = m + "e" + sign + strconv.Itoa(e)
	}
	if neg {
		return "-" + out, nil
	}
	return out, nil
}

// compareUTF16 orders two strings by their UTF-16 code units, as RFC 8785
// requires for object member sorting. This differs from Go's byte/rune order
// for supplementary-plane characters (which UTF-16 encodes as surrogate pairs).
func compareUTF16(a, b string) int {
	ua := utf16.Encode([]rune(a))
	ub := utf16.Encode([]rune(b))
	n := min(len(ua), len(ub))
	for i := range n {
		if ua[i] != ub[i] {
			if ua[i] < ub[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(ua) < len(ub):
		return -1
	case len(ua) > len(ub):
		return 1
	default:
		return 0
	}
}
