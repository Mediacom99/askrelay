package relay

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"time"
)

// Config is the relay's whole configuration — flags + env only (T-11); no
// config file. Precedence is flag > env > default, achieved by baking the env
// fallback into each flag's default value.
type Config struct {
	ListenAddr string        // -listen       / ASKRELAY_LISTEN        (default 127.0.0.1:8080)
	DBPath     string        // -db           / ASKRELAY_DB            (default askrelay.db)
	BaseURL    string        // -base-url      / ASKRELAY_BASE_URL      (public http(s) URL; required)
	InviteTTL  time.Duration // -invite-ttl    / ASKRELAY_INVITE_TTL    (default 24h)

	// Store knobs (T-09 retention + WP-01/03 freshness), 1h floor per T-09.
	AckGrace time.Duration // -ack-grace      / ASKRELAY_ACK_GRACE      (default 72h)
	HardTTL  time.Duration // -hard-ttl       / ASKRELAY_HARD_TTL       (default 720h / 30d)
	MaxAge   time.Duration // -fresh-max-age  / ASKRELAY_FRESH_MAX_AGE  (default 24h)
	MaxSkew  time.Duration // -fresh-max-skew / ASKRELAY_FRESH_MAX_SKEW (default 5m)
}

// LoadConfig parses argv for the `serve` command. The env fallback is baked
// into each flag's default, so flag > env > default falls out of flag.Parse.
// It uses ContinueOnError (not ExitOnError) so callers — and tests — get an
// error rather than a process exit.
func LoadConfig(args []string) (Config, error) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	var c Config
	fs.StringVar(&c.ListenAddr, "listen", envOr("ASKRELAY_LISTEN", "127.0.0.1:8080"), "listen address")
	fs.StringVar(&c.DBPath, "db", envOr("ASKRELAY_DB", "askrelay.db"), "sqlite database path")
	fs.StringVar(&c.BaseURL, "base-url", os.Getenv("ASKRELAY_BASE_URL"), "public base URL, e.g. https://relay.example.com (required)")
	fs.DurationVar(&c.InviteTTL, "invite-ttl", envDur("ASKRELAY_INVITE_TTL", 24*time.Hour), "invite lifetime")
	fs.DurationVar(&c.AckGrace, "ack-grace", envDur("ASKRELAY_ACK_GRACE", 72*time.Hour), "delete acked messages this long after their last ack")
	fs.DurationVar(&c.HardTTL, "hard-ttl", envDur("ASKRELAY_HARD_TTL", 720*time.Hour), "delete any message this long after receipt")
	fs.DurationVar(&c.MaxAge, "fresh-max-age", envDur("ASKRELAY_FRESH_MAX_AGE", 24*time.Hour), "reject/forget messages older than this")
	fs.DurationVar(&c.MaxSkew, "fresh-max-skew", envDur("ASKRELAY_FRESH_MAX_SKEW", 5*time.Minute), "reject messages more than this far in the future")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

// Validate rejects a config that would be unsafe or non-functional. The 1h
// floor on retention/freshness is the T-09 operator floor; BaseURL must be an
// absolute http(s) URL because enroll responses and (later) OAuth audiences
// are built from it.
func (c Config) Validate() error {
	if c.ListenAddr == "" {
		return fmt.Errorf("relay: listen address is required")
	}
	if c.DBPath == "" {
		return fmt.Errorf("relay: db path is required")
	}
	u, err := url.Parse(c.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("relay: base-url must be an absolute http(s) URL, got %q", c.BaseURL)
	}
	for _, d := range []struct {
		name string
		v    time.Duration
	}{
		{"invite-ttl", c.InviteTTL}, {"ack-grace", c.AckGrace},
		{"hard-ttl", c.HardTTL}, {"fresh-max-age", c.MaxAge},
	} {
		if d.v < time.Hour {
			return fmt.Errorf("relay: %s must be at least 1h (T-09 floor), got %s", d.name, d.v)
		}
	}
	if c.MaxSkew <= 0 || c.MaxSkew > time.Hour {
		return fmt.Errorf("relay: fresh-max-skew must be in (0, 1h], got %s", c.MaxSkew)
	}
	return nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envDur(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
