package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"syscall"
	"time"
)

// cimdFetchTimeout bounds the whole client-metadata fetch; it sits on the
// interactive /authorize path, so it must be short.
const cimdFetchTimeout = 5 * time.Second

// cimdMaxBody caps the metadata document — a client_id doc is a few hundred
// bytes; 16 KiB is generous and bounds a malicious oversized response.
const cimdMaxBody = 16 << 10

// clientMetadata is the subset of a CIMD document we trust. Only client_id
// (self-consistency) and redirect_uris (the security decision) are read; every
// other field is ignored display text.
type clientMetadata struct {
	ClientID     string   `json:"client_id"`
	RedirectURIs []string `json:"redirect_uris"`
}

// cimdClient is an SSRF-hardened HTTP client for fetching a Client ID Metadata
// Document from an untrusted https client_id. Dialer.Control inspects the
// *resolved* IP right before connecting — after DNS, closing the rebinding
// TOCTOU a pre-resolve check leaves open — and refuses any non-public address
// (loopback, private, link-local incl. cloud metadata 169.254.169.254,
// unspecified, multicast). Proxy is nil: a proxy would connect on our behalf and
// bypass the IP guard. Redirects are followed but re-dial through the same guard.
var cimdClient = &http.Client{
	Timeout: cimdFetchTimeout,
	Transport: &http.Transport{
		Proxy: nil,
		DialContext: (&net.Dialer{
			Timeout: cimdFetchTimeout,
			Control: guardDialToPublicIP,
		}).DialContext,
		DisableKeepAlives: true,
	},
}

// cgnatRange is RFC 6598 shared address space (100.64.0.0/10). net.IP.IsPrivate
// does not cover it, but Tailscale assigns from it to every mesh node — a real
// internal target on the self-host deploy askrelay documents, so the SSRF guard
// must refuse it (security review finding #2).
var cgnatRange = func() *net.IPNet {
	_, n, _ := net.ParseCIDR("100.64.0.0/10")
	return n
}()

// guardDialToPublicIP refuses a dial whose resolved address is not a public
// unicast IP (the SSRF guard for the CIMD fetch).
func guardDialToPublicIP(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("relay: cimd dial: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("relay: cimd dial: unresolved host %q", host)
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() ||
		cgnatRange.Contains(ip) {
		return fmt.Errorf("relay: cimd dial: refused non-public address")
	}
	return nil
}

// fetchClientMetadata GETs the CIMD document at clientID (an https URL) through
// the SSRF-guarded client. Deny-by-default: any transport, status, content-type,
// or size problem is an error, and the caller rejects the client on any error.
func fetchClientMetadata(ctx context.Context, clientID string) (*clientMetadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, clientID, nil)
	if err != nil {
		return nil, fmt.Errorf("relay: cimd request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := cimdClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("relay: cimd fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("relay: cimd fetch: status %d", resp.StatusCode)
	}
	mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if mediaType != "application/json" {
		return nil, fmt.Errorf("relay: cimd fetch: content-type %q", mediaType)
	}
	var meta clientMetadata
	if err := json.NewDecoder(io.LimitReader(resp.Body, cimdMaxBody)).Decode(&meta); err != nil {
		return nil, fmt.Errorf("relay: cimd decode: %w", err)
	}
	return &meta, nil
}

// redirectMatches reports whether a requested redirect_uri satisfies a registered
// one. Non-loopback URIs must match exactly; loopback URIs (http on
// localhost/127.0.0.1/[::1]) match ignoring the port, per RFC 8252 §7.3 — native
// apps register a port-less loopback and request an ephemeral one.
func redirectMatches(registered, requested string) bool {
	if registered == requested {
		return true
	}
	rg, err1 := url.Parse(registered)
	rq, err2 := url.Parse(requested)
	if err1 != nil || err2 != nil {
		return false
	}
	return isLoopbackRedirect(rg) && isLoopbackRedirect(rq) &&
		rg.Scheme == rq.Scheme && strings.EqualFold(rg.Hostname(), rq.Hostname()) && rg.Path == rq.Path
}

// isLoopbackRedirect reports whether u is an http loopback redirect — the only
// case RFC 8252 §7.3 lets vary by port. Host is matched case-insensitively
// (RFC 3986 hosts are; url.Parse lowercases the scheme but not the host).
func isLoopbackRedirect(u *url.URL) bool {
	if u.Scheme != "http" {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

// CIMD fetch throttle + cache (S-04 hardening, security review finding #1): the
// fetch runs on unauthenticated GET *and* POST /authorize with no client secret,
// so without limits it is an SSRF/DoS proxy. The semaphore bounds concurrent
// outbound fetches; the short TTL cache collapses the per-login double-fetch and
// repeated same-client_id hits into one. An operator rate limiter (R-10) still
// fronts /authorize.
const (
	cimdMaxConcurrent = 8
	cimdCacheTTL      = 60 * time.Second
	cimdCacheMax      = 1024 // bound so client_id rotation cannot grow the cache unboundedly
)

type cimdCacheEntry struct {
	meta *clientMetadata
	err  error
	exp  time.Time
}

// cimdFetcher wraps the raw fetch with a concurrency cap and a short bounded
// cache. It is the default Server.fetchCIMD; tests inject their own fake at that
// seam and bypass this. fetch/now are fields so the cache itself is testable.
type cimdFetcher struct {
	sem   chan struct{}
	fetch func(context.Context, string) (*clientMetadata, error)
	now   func() time.Time

	mu    sync.Mutex
	cache map[string]cimdCacheEntry
}

func newCIMDFetcher() *cimdFetcher {
	return &cimdFetcher{
		sem:   make(chan struct{}, cimdMaxConcurrent),
		fetch: fetchClientMetadata,
		now:   time.Now,
		cache: make(map[string]cimdCacheEntry),
	}
}

// get serves a fresh cache hit without a fetch, and otherwise fetches under the
// concurrency cap and caches the result — positive AND negative, so a hostile
// client_id cannot be replayed into a fetch storm.
func (f *cimdFetcher) get(ctx context.Context, clientID string) (*clientMetadata, error) {
	now := f.now()
	f.mu.Lock()
	if e, ok := f.cache[clientID]; ok && now.Before(e.exp) {
		f.mu.Unlock()
		return e.meta, e.err
	}
	f.mu.Unlock()

	select {
	case f.sem <- struct{}{}:
		defer func() { <-f.sem }()
	case <-ctx.Done():
		return nil, ctx.Err() // not cached: transient, per-request cancellation
	}

	meta, err := f.fetch(ctx, clientID)
	f.store(clientID, meta, err, now.Add(cimdCacheTTL))
	return meta, err
}

// store records an entry, sweeping expired ones first and refusing to grow past
// the cap so client_id rotation cannot exhaust memory.
// ponytail: crude cap (sweep-then-skip), not LRU — fine for a 60s TTL.
func (f *cimdFetcher) store(clientID string, meta *clientMetadata, err error, exp time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.cache) >= cimdCacheMax {
		now := f.now()
		for k, e := range f.cache {
			if !now.Before(e.exp) {
				delete(f.cache, k)
			}
		}
		if len(f.cache) >= cimdCacheMax {
			return // still full of live entries — skip caching rather than grow
		}
	}
	f.cache[clientID] = cimdCacheEntry{meta: meta, err: err, exp: exp}
}
