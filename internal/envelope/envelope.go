package envelope

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"

	"github.com/Mediacom99/askrelay/internal/a2a"
)

// Version is the current envelope wire version. Evolution is append-only
// (docs/askrelay-architecture.md §12): unknown fields are ignored on decode,
// and an unknown version is rejected with ErrBadVersion. Breaking changes bump
// this and ship a translating relay first.
const Version = 1

// Whole-artifact size caps (T-16, docs/askrelay-architecture.md §3). All three
// are enforced in the Sign/Verify path (see check) so a signed envelope is
// bounded by construction and every consumer inherits the bound. Together they
// starve exfil-by-bulk, which the text-only cap alone did not: a 4 MB envelope
// with tiny body text was demonstrably signable before T-16.
const (
	// MaxBodyBytes caps the message-body payload: the sum of the UTF-8 bytes of
	// all part Text values (see bodySize). 32 KiB fits "question + code
	// snippet".
	MaxBodyBytes = 32 << 10
	// MaxWireBytes caps the whole canonical (sig-omitted) envelope, so metadata
	// or label bloat in non-body fields cannot smuggle bulk past MaxBodyBytes.
	MaxWireBytes = 64 << 10
	// MaxParts caps len(Body.Parts), so a flood of small/empty parts cannot
	// smuggle bulk past MaxBodyBytes either.
	MaxParts = 16
)

// Sentinel errors returned by Sign, Verify, and Decode. They are the only
// verdicts callers should branch on; other returned errors are unexpected
// serialization faults.
var (
	// ErrTooLarge means the envelope exceeds a byte cap: either the body-text
	// cap (MaxBodyBytes) or the whole canonical envelope cap (MaxWireBytes).
	ErrTooLarge = errors.New("envelope: exceeds size cap")
	// ErrTooManyParts means Body.Parts exceeds MaxParts.
	ErrTooManyParts = errors.New("envelope: too many parts")
	// ErrBadVersion means the envelope version is not supported by this build.
	ErrBadVersion = errors.New("envelope: unsupported version")
	// ErrBadSignature means the Ed25519 signature is missing, malformed, or
	// does not verify against the supplied public key over the canonical form.
	ErrBadSignature = errors.New("envelope: bad signature")
)

// Party identifies the sending endpoint: the person, the enrolled device whose
// key signs the envelope, and the originating agent/client.
type Party struct {
	Person string `json:"person"`
	Device string `json:"device"`
	Agent  string `json:"agent"`
}

// Part is one ordered piece of a Message. Only "text" is defined in v1
// (docs/askrelay-architecture.md §3, D-05: no arbitrary file parts); a "code"
// convention is carried as fenced text.
type Part struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Message is an A2A Message: a role ("user" | "agent") and ordered parts.
type Message struct {
	Role  string `json:"role"`
	Parts []Part `json:"parts"`
}

// Envelope wraps a Message for signed transport between AI sessions
// (docs/askrelay-architecture.md §3). The signature covers the RFC 8785 (JCS)
// canonical form of the envelope with the Sig field absent; a valid signature
// proves origin only — never safety or intent (D-10). Replay protection
// (id tombstones + sent_at freshness) is the store's job (WP-03); the envelope
// carries ID, Thread, and SentAt so that becomes possible.
type Envelope struct {
	V           int             `json:"v"`
	ID          string          `json:"id"`
	Thread      string          `json:"thread"`
	From        Party           `json:"from"`
	To          string          `json:"to"`
	State       a2a.ThreadState `json:"state"`
	SentAt      time.Time       `json:"sent_at"`
	AIGenerated bool            `json:"ai_generated"`
	Body        Message         `json:"body"`
	Sig         []byte          `json:"sig,omitempty"`
}

// New builds an unsigned envelope at the current Version with a fresh UUIDv7
// message id and SentAt set to the current UTC time. When thread is empty a new
// UUIDv7 thread id is minted (a new conversation); otherwise the message joins
// the named thread. The returned envelope has no signature — call Sign before
// transmitting.
//
// New panics only if the operating system CSPRNG is unavailable, which is
// unrecoverable (mirrors stdlib randomness helpers).
func New(from Party, to, thread string, state a2a.ThreadState, aiGenerated bool, body Message) Envelope {
	if thread == "" {
		thread = newID()
	}
	return Envelope{
		V:           Version,
		ID:          newID(),
		Thread:      thread,
		From:        from,
		To:          to,
		State:       state,
		SentAt:      time.Now().UTC(),
		AIGenerated: aiGenerated,
		Body:        body,
	}
}

// Decode parses a received envelope. Unknown fields are ignored for forward
// compatibility (§12); an unsupported version is rejected with ErrBadVersion.
// Decode validates structure and version only — call Verify to check the
// signature and caps.
func Decode(data []byte) (Envelope, error) {
	var e Envelope
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&e); err != nil {
		return Envelope{}, fmt.Errorf("envelope: decode: %w", err)
	}
	// Reject trailing data after the envelope value.
	var extra json.RawMessage
	if err := dec.Decode(&extra); err != io.EOF {
		return Envelope{}, errors.New("envelope: trailing data after envelope")
	}
	if !versionSupported(e.V) {
		return Envelope{}, ErrBadVersion
	}
	return e, nil
}

// newID returns a fresh UUIDv7 string. It panics only on CSPRNG failure.
func newID() string {
	return uuid.Must(uuid.NewV7()).String()
}

// versionSupported reports whether this build accepts envelope version v.
// Append-only: future versions extend the accepted set.
func versionSupported(v int) bool {
	return v == Version
}

// bodySize returns the total UTF-8 byte length of every part's text — the
// payload the MaxBodyBytes cap bounds. Structural JSON overhead is not counted;
// the cap is on human/AI text content.
func bodySize(m Message) int {
	n := 0
	for _, p := range m.Parts {
		n += len(p.Text)
	}
	return n
}

// check enforces the version and all size-cap invariants shared by Sign and
// Verify, and returns the canonical (sig-omitted) bytes so the caller can sign
// or verify against them without canonicalizing a second time.
//
// The caps are checked cheapest-first, which is also DoS-safest: version and
// the part-count / body-text length checks are O(parts) and reject a
// pathological envelope (e.g. 200k parts) before any canonicalization, while
// the whole-wire cap necessarily canonicalizes to measure. Precedence is
// therefore fixed as: version → part-count → body-text → whole-wire. Both
// byte caps report ErrTooLarge; the part-count cap reports ErrTooManyParts.
// Canonical never calls check, so there is no recursion.
func check(e Envelope) ([]byte, error) {
	if !versionSupported(e.V) {
		return nil, ErrBadVersion
	}
	if len(e.Body.Parts) > MaxParts {
		return nil, ErrTooManyParts
	}
	if bodySize(e.Body) > MaxBodyBytes {
		return nil, ErrTooLarge
	}
	canon, err := Canonical(e)
	if err != nil {
		return nil, err
	}
	if len(canon) > MaxWireBytes {
		return nil, ErrTooLarge
	}
	return canon, nil
}
