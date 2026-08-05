// Package redact implements client-side secret redaction (D-12, T-12): a fixed
// set of high-signal patterns, each replaced with a visible ⟦redacted:<kind>⟧
// marker, plus a summary the caller surfaces to the sender. No entropy
// heuristics (a false-positive machine) and no ML — fixed patterns only. It
// runs client-side, before an envelope is signed; the relay is never involved.
package redact

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Kind identifies which pattern matched.
type Kind string

const (
	KindPrivateKey  Kind = "private-key"         // PEM PRIVATE KEY blocks
	KindGCPSAKey    Kind = "gcp-service-account" // "private_key": "..." in SA JSON
	KindJWT         Kind = "jwt"                 // eyJ….eyJ….sig
	KindAWSKey      Kind = "aws-access-key"      // AKIA/ASIA…
	KindGitHubToken Kind = "github-token"        // ghp_/gho_/…/github_pat_
	KindSlackToken  Kind = "slack-token"         // xox[baprs]-…
	KindEnvSecret   Kind = "env-secret"          // NAME=value where NAME ~ TOKEN|SECRET|KEY|PASSWORD
)

const markerFmt = "⟦redacted:%s⟧"

// rule replaces submatch group `group` of every match (0 = whole match) with the
// marker; keeping a group lets `KEY=secret` become `KEY=⟦redacted:env-secret⟧`.
type rule struct {
	kind  Kind
	re    *regexp.Regexp
	group int
}

// Order matters: unambiguous secret shapes first (so a GITHUB_TOKEN=… is
// labelled github-token, not env-secret), the broad env rule last. The gcp and
// env value groups exclude a leading ⟦ so an already-inserted marker is never
// re-redacted (which would double-count and relabel it).
//
// ponytail: fixed allow-listed shapes only, no entropy scan (T-12: entropy is a
// false-positive machine). The env rule matches any NAME containing KEY, so
// PUBLIC_KEY=… over-redacts — accepted (T-12), and over-redacting is the safe
// direction for a secret filter.
var rules = []rule{
	{KindPrivateKey, regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?-----END [A-Z0-9 ]*PRIVATE KEY-----`), 0},
	{KindGCPSAKey, regexp.MustCompile(`"private_key"\s*:\s*"([^⟦"]+)"`), 1},
	{KindJWT, regexp.MustCompile(`\beyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b`), 0},
	{KindAWSKey, regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`), 0},
	{KindGitHubToken, regexp.MustCompile(`\b(?:gh[pousr]_[A-Za-z0-9]{36}|github_pat_[A-Za-z0-9_]{40,})\b`), 0},
	{KindSlackToken, regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}\b`), 0},
	{KindEnvSecret, regexp.MustCompile(`(?im)^([ \t]*(?:export[ \t]+)?[A-Za-z0-9_]*(?:TOKEN|SECRET|KEY|PASSWORD)[A-Za-z0-9_]*[ \t]*=[ \t]*)([^⟦\s]\S*)`), 2},
}

// Result is redacted text plus a per-kind count of what was replaced.
type Result struct {
	Text   string
	Counts map[Kind]int
}

// Redacted reports whether anything was redacted.
func (r Result) Redacted() bool { return len(r.Counts) > 0 }

// Summary is a stable, human-readable warning line, e.g. "aws-access-key×1, jwt×2".
// Empty when nothing was redacted.
func (r Result) Summary() string {
	if len(r.Counts) == 0 {
		return ""
	}
	kinds := make([]string, 0, len(r.Counts))
	for k := range r.Counts {
		kinds = append(kinds, string(k))
	}
	sort.Strings(kinds)
	parts := make([]string, len(kinds))
	for i, k := range kinds {
		parts[i] = fmt.Sprintf("%s×%d", k, r.Counts[Kind(k)])
	}
	return strings.Join(parts, ", ")
}

// Redact replaces every T-12 pattern match in s with a visible marker and
// returns the rewritten text plus per-kind counts. Deterministic; safe on
// arbitrary input (RE2, no backtracking).
func Redact(s string) Result {
	counts := map[Kind]int{}
	for _, ru := range rules {
		var n int
		s, n = ru.apply(s)
		if n > 0 {
			counts[ru.kind] += n
		}
	}
	return Result{Text: s, Counts: counts}
}

// apply replaces group `group` of each match with the marker, returning the new
// string and the number of matches.
func (ru rule) apply(s string) (string, int) {
	locs := ru.re.FindAllStringSubmatchIndex(s, -1)
	if len(locs) == 0 {
		return s, 0
	}
	marker := fmt.Sprintf(markerFmt, ru.kind)
	var b strings.Builder
	last := 0
	for _, loc := range locs {
		gs, ge := loc[2*ru.group], loc[2*ru.group+1]
		if gs < 0 { // group didn't participate
			continue
		}
		b.WriteString(s[last:gs]) // untouched text (incl. any KEY= prefix)
		b.WriteString(marker)
		last = ge
	}
	b.WriteString(s[last:])
	return b.String(), len(locs)
}
