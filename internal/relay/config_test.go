package relay

import (
	"strings"
	"testing"
	"time"
)

func TestLoadConfigDefaults(t *testing.T) {
	c, err := LoadConfig([]string{"-base-url", "https://relay.example.com"})
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if c.ListenAddr != "127.0.0.1:8080" || c.DBPath != "askrelay.db" {
		t.Errorf("defaults wrong: %+v", c)
	}
	if c.InviteTTL != 24*time.Hour || c.AckGrace != 72*time.Hour || c.HardTTL != 720*time.Hour {
		t.Errorf("duration defaults wrong: %+v", c)
	}
	// Signing key defaults beside the db.
	if c.SigningKeyPath != "signing.key" {
		t.Errorf("signing-key default = %q, want signing.key", c.SigningKeyPath)
	}
	c2, err := LoadConfig([]string{"-base-url", "https://r.example.com", "-db", "/data/relay.db"})
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if c2.SigningKeyPath != "/data/signing.key" {
		t.Errorf("signing-key beside db = %q, want /data/signing.key", c2.SigningKeyPath)
	}
}

func TestLoadConfigFlagBeatsEnv(t *testing.T) {
	t.Setenv("ASKRELAY_LISTEN", "0.0.0.0:9000")
	t.Setenv("ASKRELAY_DB", "/data/env.db")

	// Env fills an unset flag...
	c, err := LoadConfig([]string{"-base-url", "https://r.example.com"})
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if c.ListenAddr != "0.0.0.0:9000" || c.DBPath != "/data/env.db" {
		t.Errorf("env fallback not applied: %+v", c)
	}
	// ...but an explicit flag wins over env.
	c, err = LoadConfig([]string{"-base-url", "https://r.example.com", "-listen", "127.0.0.1:1"})
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if c.ListenAddr != "127.0.0.1:1" {
		t.Errorf("flag did not beat env: %q", c.ListenAddr)
	}
}

func TestLoadConfigBadEnvDurationFallsBack(t *testing.T) {
	t.Setenv("ASKRELAY_INVITE_TTL", "not-a-duration")
	c, err := LoadConfig([]string{"-base-url", "https://r.example.com"})
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if c.InviteTTL != 24*time.Hour {
		t.Errorf("bad env duration should fall back to default, got %s", c.InviteTTL)
	}
}

func TestConfigValidate(t *testing.T) {
	base := Config{
		ListenAddr: "127.0.0.1:8080", DBPath: "askrelay.db", BaseURL: "https://r.example.com",
		InviteTTL: 24 * time.Hour, AckGrace: 72 * time.Hour, HardTTL: 720 * time.Hour,
		MaxAge: 24 * time.Hour, MaxSkew: 5 * time.Minute,
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}

	cases := []struct {
		name string
		mut  func(*Config)
		want string
	}{
		{"empty base-url", func(c *Config) { c.BaseURL = "" }, "base-url"},
		{"relative base-url", func(c *Config) { c.BaseURL = "relay.example.com" }, "base-url"},
		{"non-http scheme", func(c *Config) { c.BaseURL = "ftp://r.example.com" }, "base-url"},
		{"sub-1h hard-ttl", func(c *Config) { c.HardTTL = 30 * time.Minute }, "hard-ttl"},
		{"sub-1h ack-grace", func(c *Config) { c.AckGrace = time.Minute }, "ack-grace"},
		{"zero skew", func(c *Config) { c.MaxSkew = 0 }, "fresh-max-skew"},
		{"huge skew", func(c *Config) { c.MaxSkew = 2 * time.Hour }, "fresh-max-skew"},
		{"empty listen", func(c *Config) { c.ListenAddr = "" }, "listen"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := base
			tc.mut(&c)
			err := c.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Validate() = %v, want an error mentioning %q", err, tc.want)
			}
		})
	}
}
