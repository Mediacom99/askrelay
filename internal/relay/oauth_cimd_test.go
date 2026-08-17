package relay

import (
	"context"
	"errors"
	"testing"
	"time"
)

// disabledCIMD is the default fetchCIMD in tests: it never touches the network,
// so a CIMD-path test that forgets to inject its own fake fails closed (a reject)
// rather than making a real outbound fetch. Tests that need a working document
// override s.fetchCIMD with their own fake.
func disabledCIMD(context.Context, string) (*clientMetadata, error) {
	return nil, errors.New("cimd fetch disabled in tests; inject a fake")
}

func TestGuardDialToPublicIP(t *testing.T) {
	cases := []struct {
		addr string
		ok   bool
	}{
		{"8.8.8.8:443", true},             // public
		{"127.0.0.1:443", false},          // loopback
		{"[::1]:443", false},              // loopback v6
		{"10.0.0.5:443", false},           // private
		{"192.168.1.1:443", false},        // private
		{"172.16.0.1:443", false},         // private
		{"169.254.169.254:80", false},     // link-local (cloud metadata)
		{"0.0.0.0:443", false},            // unspecified
		{"[fe80::1]:443", false},          // link-local v6
		{"[fc00::1]:443", false},          // ULA (private v6)
		{"[::ffff:127.0.0.1]:443", false}, // v4-mapped loopback
		{"100.64.0.1:443", false},         // CGNAT / Tailscale (RFC 6598)
		{"100.127.255.255:443", false},    // top of 100.64/10
		{"100.128.0.1:443", true},         // just outside 100.64/10 — still public
	}
	for _, c := range cases {
		if err := guardDialToPublicIP("tcp", c.addr, nil); (err == nil) != c.ok {
			t.Errorf("guardDialToPublicIP(%q): err=%v, want ok=%v", c.addr, err, c.ok)
		}
	}
}

func TestRedirectMatches(t *testing.T) {
	cases := []struct {
		reg, req string
		want     bool
	}{
		{"http://localhost/callback", "http://localhost:52341/callback", true}, // loopback: port ignored
		{"http://localhost/callback", "http://LOCALHOST:4444/callback", true},  // host case-insensitive
		{"http://127.0.0.1/callback", "http://127.0.0.1:5/callback", true},
		{"http://localhost/callback", "http://localhost/callback", true},    // exact
		{"https://claude.ai/cb", "https://claude.ai/cb", true},              // exact https
		{"http://localhost/callback", "http://127.0.0.1:5/callback", false}, // host must match (no cross)
		{"http://localhost/callback", "http://localhost:5/evil", false},     // path differs
		{"https://claude.ai/cb", "https://claude.ai:8443/cb", false},        // non-loopback: port matters
		{"http://localhost/callback", "https://localhost/callback", false},  // scheme differs
		{"http://evil.com/callback", "http://evil.com:9/callback", false},   // non-loopback http: exact only
	}
	for _, c := range cases {
		if got := redirectMatches(c.reg, c.req); got != c.want {
			t.Errorf("redirectMatches(%q, %q) = %v, want %v", c.reg, c.req, got, c.want)
		}
	}
}

func TestCIMDFetcherCachesAndExpires(t *testing.T) {
	var calls int
	base := time.Unix(1_700_000_000, 0)
	clock := base
	f := &cimdFetcher{
		sem:   make(chan struct{}, 2),
		now:   func() time.Time { return clock },
		cache: make(map[string]cimdCacheEntry),
		fetch: func(context.Context, string) (*clientMetadata, error) {
			calls++
			return &clientMetadata{ClientID: "x"}, nil
		},
	}
	ctx := context.Background()
	// Two gets within the TTL → a single underlying fetch (kills the per-login
	// GET+POST double-fetch and repeated-hit amplification).
	for range 2 {
		if _, err := f.get(ctx, "x"); err != nil {
			t.Fatalf("get: %v", err)
		}
	}
	if calls != 1 {
		t.Fatalf("within TTL: fetch calls = %d, want 1", calls)
	}
	// Past the TTL → refetch.
	clock = base.Add(cimdCacheTTL + time.Second)
	if _, err := f.get(ctx, "x"); err != nil {
		t.Fatalf("get after ttl: %v", err)
	}
	if calls != 2 {
		t.Fatalf("after TTL: fetch calls = %d, want 2", calls)
	}
}

func TestCIMDFetcherNegativeCache(t *testing.T) {
	var calls int
	clock := time.Unix(1_700_000_000, 0)
	f := &cimdFetcher{
		sem:   make(chan struct{}, 2),
		now:   func() time.Time { return clock },
		cache: make(map[string]cimdCacheEntry),
		fetch: func(context.Context, string) (*clientMetadata, error) {
			calls++
			return nil, errors.New("boom")
		},
	}
	ctx := context.Background()
	// A failure is cached too, so a hostile client_id can't be replayed into a
	// fetch storm within the TTL.
	for range 2 {
		if _, err := f.get(ctx, "bad"); err == nil {
			t.Fatal("want error")
		}
	}
	if calls != 1 {
		t.Fatalf("negative cache: fetch calls = %d, want 1", calls)
	}
}
