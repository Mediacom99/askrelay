package store

import (
	"database/sql"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"
)

// ---------------------------------------------------------------------------
// ADVERSARIAL (askrelay-test, WP-06 quality pass): concurrency races on the
// single-use guards, the replay-detection window across retention sweeps,
// family-revoke scope precision, schema defense-in-depth for the new 0002
// tables, and the actual 1->2 migration path. oauth_test.go covers the
// documented happy/error paths; this file is what an attacker (or a buggy
// retry) throws at it.
// ---------------------------------------------------------------------------

// TestConsumeAuthCodeConcurrentRaceExactlyOneWins fires N goroutines at the
// SAME code simultaneously. The writer mutex (T-03) + the conditional UPDATE
// must let exactly one succeed, whatever the interleaving.
func TestConsumeAuthCodeConcurrentRaceExactlyOneWins(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	pid, did := enrollDevice(t, s, "marco@example.com", now)
	b := AuthCode{ClientID: "c", RedirectURI: "u", CodeChallenge: "ch", PersonID: pid, DeviceID: did}
	if err := s.CreateAuthCode("race-code", b, now.Add(time.Minute), now); err != nil {
		t.Fatalf("CreateAuthCode: %v", err)
	}

	const n = 20
	var wg sync.WaitGroup
	var mu sync.Mutex
	oks, fails := 0, 0
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := s.ConsumeAuthCode("race-code", now)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				oks++
			case errors.Is(err, ErrCodeInvalid):
				fails++
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
	if oks != 1 || fails != n-1 {
		t.Errorf("race outcome: oks=%d fails=%d, want exactly 1 ok and %d ErrCodeInvalid", oks, fails, n-1)
	}
}

// TestConsumeRefreshTokenConcurrentRaceExactlyOneWins is the refresh-token
// analogue: concurrent FIRST use of the same live token.
func TestConsumeRefreshTokenConcurrentRaceExactlyOneWins(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	pid, did := enrollDevice(t, s, "marco@example.com", now)
	g := RefreshGrant{ClientID: "c", PersonID: pid, DeviceID: did}
	if err := s.CreateRefreshToken("race-rt", g, now.Add(time.Hour), now); err != nil {
		t.Fatalf("CreateRefreshToken: %v", err)
	}

	const n = 20
	var wg sync.WaitGroup
	var mu sync.Mutex
	oks, reused := 0, 0
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.ConsumeRefreshToken("race-rt", now)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				oks++
			case errors.Is(err, ErrRefreshReused):
				reused++
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
	// The loser(s) see the winner's row as already-consumed -> ErrRefreshReused
	// (indistinguishable from a real replay, by design), never ErrRefreshInvalid.
	if oks != 1 || reused != n-1 {
		t.Errorf("race outcome: oks=%d reused=%d, want exactly 1 ok and %d ErrRefreshReused", oks, reused, n-1)
	}
}

// TestConsumeRefreshTokenReplayRaceIsIdempotent: N goroutines all replaying an
// ALREADY-consumed token concurrently. Every one must observe ErrRefreshReused
// (no crash, no partial state) even though the family-revoke UPDATE they each
// attempt becomes a no-op after the first.
func TestConsumeRefreshTokenReplayRaceIsIdempotent(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	pid, did := enrollDevice(t, s, "marco@example.com", now)
	g := RefreshGrant{ClientID: "c", PersonID: pid, DeviceID: did}
	if err := s.CreateRefreshToken("consumed-rt", g, now.Add(time.Hour), now); err != nil {
		t.Fatalf("CreateRefreshToken: %v", err)
	}
	if _, err := s.ConsumeRefreshToken("consumed-rt", now); err != nil {
		t.Fatalf("initial consume: %v", err)
	}

	const n = 20
	var wg sync.WaitGroup
	var mu sync.Mutex
	reused := 0
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.ConsumeRefreshToken("consumed-rt", now)
			mu.Lock()
			defer mu.Unlock()
			if errors.Is(err, ErrRefreshReused) {
				reused++
			} else {
				t.Errorf("replay race: err = %v, want ErrRefreshReused", err)
			}
		}()
	}
	wg.Wait()
	if reused != n {
		t.Errorf("reused = %d, want all %d concurrent replays to report ErrRefreshReused", reused, n)
	}
}

// TestRefreshFamilyScopedPreciselyByPersonAndClient pins the blast radius of a
// reuse-triggered family revoke: it must be person X client A only, never
// bleeding into the same person's OTHER client, nor into another person on
// the SAME client.
func TestRefreshFamilyScopedPreciselyByPersonAndClient(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	pidX, didX := enrollDevice(t, s, "x@example.com", now)
	pidY, didY := enrollDevice(t, s, "y@example.com", now)

	gXA := RefreshGrant{ClientID: "A", PersonID: pidX, DeviceID: didX}
	gXB := RefreshGrant{ClientID: "B", PersonID: pidX, DeviceID: didX}
	gYA := RefreshGrant{ClientID: "A", PersonID: pidY, DeviceID: didY}
	if err := s.CreateRefreshToken("rt-xa", gXA, now.Add(time.Hour), now); err != nil {
		t.Fatalf("CreateRefreshToken rt-xa: %v", err)
	}
	if err := s.CreateRefreshToken("rt-xb", gXB, now.Add(time.Hour), now); err != nil {
		t.Fatalf("CreateRefreshToken rt-xb: %v", err)
	}
	if err := s.CreateRefreshToken("rt-ya", gYA, now.Add(time.Hour), now); err != nil {
		t.Fatalf("CreateRefreshToken rt-ya: %v", err)
	}

	if _, err := s.ConsumeRefreshToken("rt-xa", now); err != nil {
		t.Fatalf("consume rt-xa: %v", err)
	}
	if _, err := s.ConsumeRefreshToken("rt-xa", now); !errors.Is(err, ErrRefreshReused) {
		t.Fatalf("replay rt-xa: err = %v, want ErrRefreshReused", err)
	}

	if _, err := s.ConsumeRefreshToken("rt-xb", now); err != nil {
		t.Errorf("rt-xb (same person, DIFFERENT client) killed by the X/A family revoke: %v", err)
	}
	if _, err := s.ConsumeRefreshToken("rt-ya", now); err != nil {
		t.Errorf("rt-ya (same client, DIFFERENT person) killed by the X/A family revoke: %v", err)
	}
}

// TestSweepOAuthPreservesReplayDetectionWindow: a CONSUMED refresh token must
// stay in the table until ITS OWN expires_at, not be swept early just because
// it is consumed -- otherwise a stale replay would look "unknown" instead of
// "reused" and the family-revoke signal would be lost.
func TestSweepOAuthPreservesReplayDetectionWindow(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	pid, did := enrollDevice(t, s, "marco@example.com", now)
	g := RefreshGrant{ClientID: "c", PersonID: pid, DeviceID: did}
	if err := s.CreateRefreshToken("rt1", g, now.Add(30*24*time.Hour), now); err != nil {
		t.Fatalf("CreateRefreshToken: %v", err)
	}
	if _, err := s.ConsumeRefreshToken("rt1", now); err != nil {
		t.Fatalf("consume: %v", err)
	}

	// Sweep well after consumption but far short of the token's own expiry.
	if _, err := s.SweepOAuth(now.Add(time.Hour)); err != nil {
		t.Fatalf("SweepOAuth: %v", err)
	}

	// A replay must STILL read as reuse, not "never existed".
	if _, err := s.ConsumeRefreshToken("rt1", now.Add(time.Hour)); !errors.Is(err, ErrRefreshReused) {
		t.Errorf("replay after an early sweep: err = %v, want ErrRefreshReused (consumed rows must survive until their own expiry)", err)
	}
}

// TestClientByIDMalformedInputNoCrash: SQL-ish/binary/huge/unicode client_id
// lookups all collapse to ErrClientUnknown -- no crash, no injection, no oracle.
func TestClientByIDMalformedInputNoCrash(t *testing.T) {
	s := newStore(t)
	cases := []string{
		"",
		"'; DROP TABLE oauth_clients; --",
		strings.Repeat("a", 100_000),
		"日本語クライアント",
		"\x00\x01\x02",
		"%",
		"_",
	}
	for _, c := range cases {
		if _, err := s.ClientByID(c); !errors.Is(err, ErrClientUnknown) {
			t.Errorf("ClientByID(%q): err = %v, want ErrClientUnknown", c, err)
		}
	}
	// The table must still be intact and queryable (parameterized queries
	// make injection moot, but let's not just assume it).
	uris := []string{"https://good.example.com/cb"}
	if _, err := s.RegisterClient(uris, "still works", time.Now().UTC()); err != nil {
		t.Errorf("RegisterClient after malformed lookups: %v", err)
	}
}

// TestCreateAuthCodeStoresEmptyChallengeStoreLayerHasNoGuard characterizes a
// trust boundary: the store does NOT itself reject an empty CodeChallenge --
// that guard is /authorize's parseAuthz (code_challenge == "" is rejected
// there, oauth_authorize.go). Any future caller that bypasses the HTTP layer
// must not assume the store enforces this.
func TestCreateAuthCodeStoresEmptyChallengeStoreLayerHasNoGuard(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	pid, did := enrollDevice(t, s, "marco@example.com", now)
	b := AuthCode{ClientID: "c", RedirectURI: "u", CodeChallenge: "", PersonID: pid, DeviceID: did}
	if err := s.CreateAuthCode("empty-challenge", b, now.Add(time.Minute), now); err != nil {
		t.Fatalf("CreateAuthCode: %v", err)
	}
	got, err := s.AuthCodeByCode("empty-challenge", now)
	if err != nil {
		t.Fatalf("AuthCodeByCode: %v", err)
	}
	if got.CodeChallenge != "" {
		t.Errorf("challenge = %q, want empty (store round-trips whatever it is given, no validation)", got.CodeChallenge)
	}
}

// TestOAuthCodesSchemaRefusesBadRows: FK + STRICT + PK defense-in-depth on
// oauth_codes, mirroring schema_test.go's pattern for the 0001 tables.
func TestOAuthCodesSchemaRefusesBadRows(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	pid, did := enrollDevice(t, s, "marco@example.com", now)

	if _, err := s.db.Exec(
		`INSERT INTO oauth_codes (code_hash, client_id, redirect_uri, code_challenge, person_id, device_id, created_at, expires_at)
		 VALUES ('h1','c','u','ch','nobody', ?, 100, 200)`, did); err == nil || !strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
		t.Errorf("orphan person_id: err = %v, want FOREIGN KEY violation", err)
	}
	if _, err := s.db.Exec(
		`INSERT INTO oauth_codes (code_hash, client_id, redirect_uri, code_challenge, person_id, device_id, created_at, expires_at)
		 VALUES ('h2','c','u','ch', ?, 'nobody', 100, 200)`, pid); err == nil || !strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
		t.Errorf("orphan device_id: err = %v, want FOREIGN KEY violation", err)
	}
	if _, err := s.db.Exec(
		`INSERT INTO oauth_codes (code_hash, client_id, redirect_uri, code_challenge, person_id, device_id, created_at, expires_at)
		 VALUES ('h3','c','u','ch', ?, ?, 'not-a-time', 200)`, pid, did); err == nil || !strings.Contains(err.Error(), "cannot store TEXT value in INTEGER column") {
		t.Errorf("mistyped created_at: err = %v, want a STRICT type violation", err)
	}
	if _, err := s.db.Exec(
		`INSERT INTO oauth_codes (code_hash, client_id, redirect_uri, code_challenge, person_id, device_id, created_at, expires_at)
		 VALUES ('dup','c','u','ch', ?, ?, 100, 200)`, pid, did); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := s.db.Exec(
		`INSERT INTO oauth_codes (code_hash, client_id, redirect_uri, code_challenge, person_id, device_id, created_at, expires_at)
		 VALUES ('dup','c2','u2','ch2', ?, ?, 100, 200)`, pid, did); err == nil || !strings.Contains(err.Error(), "UNIQUE constraint failed") {
		t.Errorf("duplicate code_hash: err = %v, want UNIQUE violation", err)
	}
}

// TestOAuthRefreshTokensSchemaRefusesBadRows: same defense-in-depth checks for
// oauth_refresh_tokens.
func TestOAuthRefreshTokensSchemaRefusesBadRows(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	pid, did := enrollDevice(t, s, "marco@example.com", now)

	if _, err := s.db.Exec(
		`INSERT INTO oauth_refresh_tokens (token_hash, client_id, person_id, device_id, created_at, expires_at)
		 VALUES ('h1','c','nobody', ?, 100, 200)`, did); err == nil || !strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
		t.Errorf("orphan person_id: err = %v, want FOREIGN KEY violation", err)
	}
	if _, err := s.db.Exec(
		`INSERT INTO oauth_refresh_tokens (token_hash, client_id, person_id, device_id, created_at, expires_at)
		 VALUES ('h2','c', ?, 'nobody', 100, 200)`, pid); err == nil || !strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
		t.Errorf("orphan device_id: err = %v, want FOREIGN KEY violation", err)
	}
	if _, err := s.db.Exec(
		`INSERT INTO oauth_refresh_tokens (token_hash, client_id, person_id, device_id, created_at, expires_at)
		 VALUES ('dup','c', ?, ?, 100, 200)`, pid, did); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := s.db.Exec(
		`INSERT INTO oauth_refresh_tokens (token_hash, client_id, person_id, device_id, created_at, expires_at)
		 VALUES ('dup','c2', ?, ?, 100, 200)`, pid, did); err == nil || !strings.Contains(err.Error(), "UNIQUE constraint failed") {
		t.Errorf("duplicate token_hash: err = %v, want UNIQUE violation", err)
	}
}

// TestMigrationV1ToV2PreservesDataAndAddsOAuthTables replays the exact
// upgrade path a real relay binary takes: a database migrated only to v1
// (pre-WP-06), then opened by a binary that also embeds 0002_oauth.sql. Pins
// the plan's WP-06 test-plan item ("does 0002 apply cleanly on top of 0001,
// user_version 1->2") against the REAL embedded migration set, not a synthetic
// one, and checks existing data survives.
func TestMigrationV1ToV2PreservesDataAndAddsOAuthTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "relay.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	defer db.Close()

	data0001, err := fs.ReadFile(migrationFS, "migrations/0001_init.sql")
	if err != nil {
		t.Fatalf("read 0001: %v", err)
	}
	v1fs := fstest.MapFS{"migrations/0001_init.sql": {Data: data0001}}
	if err := migrate(db, v1fs); err != nil {
		t.Fatalf("migrate to v1: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO persons (id, email, label, created_at) VALUES ('p1','a@example.com','',100)`); err != nil {
		t.Fatalf("seed person: %v", err)
	}
	var v int
	if err := db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if v != 1 {
		t.Fatalf("pre-upgrade version = %d, want 1", v)
	}

	// Apply the REAL embedded set (0001+0002) -- exactly what store.Open does
	// when an upgraded binary opens a pre-WP-06 database.
	if err := migrate(db, migrationFS); err != nil {
		t.Fatalf("migrate to v2: %v", err)
	}
	if err := db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if v != 2 {
		t.Errorf("post-upgrade version = %d, want 2", v)
	}

	var email string
	if err := db.QueryRow(`SELECT email FROM persons WHERE id='p1'`).Scan(&email); err != nil {
		t.Fatalf("pre-existing person lost across the 1->2 migration: %v", err)
	}
	if email != "a@example.com" {
		t.Errorf("email = %q, want a@example.com", email)
	}
	for _, tbl := range []string{"oauth_clients", "oauth_codes", "oauth_refresh_tokens"} {
		var n int
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_schema WHERE type='table' AND name=?`, tbl).Scan(&n); err != nil {
			t.Fatalf("check table %s: %v", tbl, err)
		}
		if n != 1 {
			t.Errorf("table %s missing after the 1->2 migration", tbl)
		}
	}

	// Idempotent re-open: running migrate again against an already-current
	// database is a clean no-op.
	if err := migrate(db, migrationFS); err != nil {
		t.Errorf("idempotent re-migrate: %v", err)
	}
}

// TestMigrationV2RollsBackOnFailure: the generic TestBrokenMigrationRollsBack
// (store_test.go) proves the mechanism with synthetic migration content. This
// pins the SAME guarantee for 0002 specifically, on top of the REAL 0001
// schema: a corrupted 0002 must not leave partial oauth tables behind or bump
// user_version past 1.
func TestMigrationV2RollsBackOnFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "relay.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	defer db.Close()

	data0001, err := fs.ReadFile(migrationFS, "migrations/0001_init.sql")
	if err != nil {
		t.Fatalf("read 0001: %v", err)
	}
	data0002, err := fs.ReadFile(migrationFS, "migrations/0002_oauth.sql")
	if err != nil {
		t.Fatalf("read 0002: %v", err)
	}
	broken := string(data0002) + "\nTHIS IS NOT SQL;"
	fsys := fstest.MapFS{
		"migrations/0001_init.sql":  {Data: data0001},
		"migrations/0002_oauth.sql": {Data: []byte(broken)},
	}
	if err := migrate(db, fsys); err == nil {
		t.Fatal("migrate with a corrupted 0002 must fail")
	}

	var v int
	if err := db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if v != 1 {
		t.Errorf("user_version = %d, want 1 (the broken 0002 must not be stamped)", v)
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_schema WHERE type='table' AND name='oauth_clients'`).Scan(&n); err != nil {
		t.Fatalf("inspect schema: %v", err)
	}
	if n != 0 {
		t.Error("oauth_clients from the failed 0002 survived its rollback")
	}
}
