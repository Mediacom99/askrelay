// Command askrelay is the askrelay CLI. WP-04 lands `serve` and `invite`; the
// remaining verbs (inbox, approve, device, status, …) arrive with WP-12.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Mediacom99/askrelay/internal/relay"
	"github.com/Mediacom99/askrelay/internal/relay/oauth"
	"github.com/Mediacom99/askrelay/internal/relay/store"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	verb, args := os.Args[1], os.Args[2:]

	var err error
	switch verb {
	case "serve":
		err = cmdServe(args)
	case "invite":
		err = cmdInvite(args)
	case "version":
		fmt.Println(relay.Version)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "askrelay: unknown command %q\n\n", verb)
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "askrelay: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `askrelay — async, approval-gated messaging between AI sessions

usage: askrelay <command> [flags]

commands:
  serve             run the relay HTTP server
  invite <email>    mint a single-use enrollment invite, print the invite URL
  version           print version
  help              show this help
`)
}

// cmdServe loads config, opens the store, and runs the server until a signal.
func cmdServe(args []string) error {
	cfg, err := relay.LoadConfig(args)
	if err != nil {
		return err
	}
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer st.Close()

	iss, err := oauth.NewIssuer(cfg.SigningKeyPath, cfg.BaseURL, time.Hour)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return relay.NewServer(cfg, st, iss, log).Run(ctx)
}

// cmdInvite mints an invite by opening the SQLite database directly — the
// operator runs it on the relay host; WAL + busy_timeout handle contention
// with a running `serve` (an admin socket would be needless mechanism in v1).
func cmdInvite(args []string) error {
	// Accept <email> before OR after the flags. Stdlib flag stops at the first
	// non-flag arg, so if the email leads we peel it off before parsing;
	// otherwise it's the trailing positional. Both forms are common, so
	// support both rather than force the Go-canonical flags-first order.
	var email string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		email, args = args[0], args[1:]
	}

	fs := flag.NewFlagSet("invite", flag.ContinueOnError)
	dbPath := fs.String("db", envOr("ASKRELAY_DB", "askrelay.db"), "sqlite database path")
	baseURL := fs.String("base-url", os.Getenv("ASKRELAY_BASE_URL"), "public base URL, used to build the invite link")
	ttl := fs.Duration("ttl", envDur("ASKRELAY_INVITE_TTL", 24*time.Hour), "invite lifetime")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if email == "" {
		if fs.NArg() != 1 {
			return fmt.Errorf("usage: askrelay invite <email> [-db path] [-base-url url] [-ttl dur]")
		}
		email = fs.Arg(0)
	} else if fs.NArg() != 0 {
		return fmt.Errorf("usage: askrelay invite <email> [-db path] [-base-url url] [-ttl dur]")
	}
	if *baseURL == "" {
		return fmt.Errorf("base-url is required (flag -base-url or ASKRELAY_BASE_URL) to build the invite link")
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	token, err := st.CreateInvite(email, *ttl, time.Now().UTC())
	if err != nil {
		return err
	}
	fmt.Printf("%s/enroll/%s\n", strings.TrimRight(*baseURL, "/"), token)
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
