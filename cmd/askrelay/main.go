// Command askrelay is the askrelay CLI. WP-04 lands `serve` and `invite`; the
// remaining verbs (inbox, approve, device, status, …) arrive with WP-12.
package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/Mediacom99/askrelay/internal/daemon"
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
	case "enroll":
		err = cmdEnroll(args)
	case "device":
		err = cmdDevice(args)
	case "daemon":
		err = cmdDaemon(args)
	case "mcp":
		err = cmdMCP(args)
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
  enroll <url>      enroll this machine as a device from an invite URL
  device list|revoke  list or revoke enrolled devices (relay host)
  daemon            run the local notifier (relay push → desktop notification)
  mcp               run the local stdio MCP server — configure THIS in your AI client
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

type enrollReq struct {
	PubKey string `json:"pubkey"`
	Label  string `json:"label"`
	Name   string `json:"name,omitempty"`
}

type enrollResp struct {
	PersonID         string `json:"person_id"`
	DeviceID         string `json:"device_id"`
	BaseURL          string `json:"base_url"`
	DeviceCredential string `json:"device_credential"`
}

// cmdEnroll consumes an invite URL, enrolls this machine as a device, writes the
// user config + private key (0600) under ~/.config/askrelay, and prints the
// device credential to paste when connecting an AI client. The credential is
// also stored in the config, because `askrelay daemon` authenticates /ws with
// it (WP-09 ST-2) — which is why that file is 0600 and never logged. Colleague-side: it
// talks to the relay over HTTP (unlike invite/device, which open the DB).
func cmdEnroll(args []string) error {
	var inviteURL string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		inviteURL, args = args[0], args[1:]
	}
	fs := flag.NewFlagSet("enroll", flag.ContinueOnError)
	label := fs.String("label", defaultLabel(), "device label (shown in `device list`)")
	name := fs.String("name", "", "your display name (shown to people who message you)")
	cfgPath := fs.String("config", "", "config file path (default ~/.config/askrelay/config.json)")
	force := fs.Bool("force", false, "replace an existing enrollment")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if inviteURL == "" {
		if fs.NArg() != 1 {
			return fmt.Errorf("usage: askrelay enroll <invite-url> [-label name] [-config path] [-force]")
		}
		inviteURL = fs.Arg(0)
	}

	path, keyPath, err := configPaths(*cfgPath)
	if err != nil {
		return err
	}
	if !*force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("already enrolled (%s exists); pass -force to replace", path)
		}
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("askrelay: generate device key: %w", err)
	}
	resp, err := postEnroll(inviteURL, pub, *label, *name)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("askrelay: create config dir: %w", err)
	}
	if err := os.WriteFile(keyPath, priv, 0o600); err != nil {
		return fmt.Errorf("askrelay: write device key: %w", err)
	}
	buf, _ := json.MarshalIndent(daemon.Config{
		RelayURL: resp.BaseURL, PersonID: resp.PersonID, DeviceID: resp.DeviceID, KeyPath: keyPath,
		DeviceCredential: resp.DeviceCredential, // the daemon's /ws bearer — 0600 below
	}, "", "  ")
	if err := os.WriteFile(path, buf, 0o600); err != nil {
		return fmt.Errorf("askrelay: write config: %w", err)
	}

	fmt.Printf("Enrolled as %s (device %s) on %s\n", resp.PersonID, resp.DeviceID, resp.BaseURL)
	fmt.Printf("Config: %s\nKey:    %s\n\n", path, keyPath)
	fmt.Printf("Device credential — paste this into the authorize page when you connect your AI client:\n\n%s\n", resp.DeviceCredential)
	return nil
}

// postEnroll POSTs the pubkey to the invite URL and returns the enroll response.
func postEnroll(inviteURL string, pub ed25519.PublicKey, label, name string) (enrollResp, error) {
	body, _ := json.Marshal(enrollReq{PubKey: base64.StdEncoding.EncodeToString(pub), Label: label, Name: name})
	req, err := http.NewRequest(http.MethodPost, inviteURL, bytes.NewReader(body))
	if err != nil {
		return enrollResp{}, fmt.Errorf("askrelay: build enroll request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return enrollResp{}, fmt.Errorf("askrelay: enroll request: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 8<<10))
	if res.StatusCode != http.StatusOK {
		return enrollResp{}, fmt.Errorf("askrelay: enroll failed (HTTP %d): %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}
	var er enrollResp
	if err := json.Unmarshal(raw, &er); err != nil {
		return enrollResp{}, fmt.Errorf("askrelay: decode enroll response: %w", err)
	}
	return er, nil
}

// configPaths resolves config.json + device.key (arch §7: ~/.config/askrelay,
// honoring XDG_CONFIG_HOME).
func configPaths(override string) (cfgPath, keyPath string, err error) {
	if override != "" {
		return override, filepath.Join(filepath.Dir(override), "device.key"), nil
	}
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return "", "", fmt.Errorf("askrelay: locate home dir: %w", herr)
		}
		dir = filepath.Join(home, ".config")
	}
	dir = filepath.Join(dir, "askrelay")
	return filepath.Join(dir, "config.json"), filepath.Join(dir, "device.key"), nil
}

func defaultLabel() string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return "device"
}

// cmdDevice manages enrolled devices on the relay host (opens the DB directly,
// like invite — WAL + busy_timeout handle contention with a running serve).
func cmdDevice(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: askrelay device <list|revoke> [args]")
	}
	switch args[0] {
	case "list":
		return cmdDeviceList(args[1:])
	case "revoke":
		return cmdDeviceRevoke(args[1:])
	default:
		return fmt.Errorf("askrelay device: unknown subcommand %q (want list|revoke)", args[0])
	}
}

func cmdDeviceList(args []string) error {
	fs := flag.NewFlagSet("device list", flag.ContinueOnError)
	dbPath := fs.String("db", envOr("ASKRELAY_DB", "askrelay.db"), "sqlite database path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	st, err := store.Open(*dbPath)
	if err != nil {
		return err
	}
	defer st.Close()
	devices, err := st.ListDevices()
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		fmt.Println("no devices enrolled")
		return nil
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "DEVICE ID\tPERSON\tLABEL\tENROLLED\tSTATUS")
	for _, d := range devices {
		status := "active"
		if !d.RevokedAt.IsZero() {
			status = "revoked " + d.RevokedAt.Format("2006-01-02")
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			d.DeviceID, d.Email, d.Label, d.CreatedAt.Format("2006-01-02"), status)
	}
	return tw.Flush()
}

func cmdDeviceRevoke(args []string) error {
	var deviceID string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		deviceID, args = args[0], args[1:]
	}
	fs := flag.NewFlagSet("device revoke", flag.ContinueOnError)
	dbPath := fs.String("db", envOr("ASKRELAY_DB", "askrelay.db"), "sqlite database path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if deviceID == "" {
		if fs.NArg() != 1 {
			return fmt.Errorf("usage: askrelay device revoke <device-id> [-db path]")
		}
		deviceID = fs.Arg(0)
	}
	st, err := store.Open(*dbPath)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := st.RevokeDevice(deviceID, time.Now().UTC()); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("no such device: %s", deviceID)
		}
		return err
	}
	fmt.Printf("revoked device %s\n", deviceID)
	return nil
}

// loadDaemon parses the -config flag shared by the two local roles and loads the
// daemon. Logs go to STDERR, never stdout: on the `mcp` path stdout is the MCP
// channel itself, and a stray log line there corrupts the protocol.
func loadDaemon(verb string, args []string) (*daemon.Daemon, error) {
	fs := flag.NewFlagSet(verb, flag.ContinueOnError)
	cfgPath := fs.String("config", "", "config file path (default ~/.config/askrelay/config.json)")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	path := *cfgPath
	if path == "" {
		p, _, err := configPaths("")
		if err != nil {
			return nil, err
		}
		path = p
	}
	return daemon.Load(path, relay.Version, slog.New(slog.NewJSONHandler(os.Stderr, nil)))
}

// cmdDaemon runs the background notifier (arch §6): it holds the relay push
// socket open and raises a desktop notification when mail arrives.
func cmdDaemon(args []string) error {
	d, err := loadDaemon("daemon", args)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return d.Run(ctx)
}

// cmdMCP runs the stdio MCP server an AI client spawns (arch §6). It proxies the
// relay's own tool surface, which is what puts the daemon in the send path —
// where redaction and signatures are meaningful (T-19 amendment).
func cmdMCP(args []string) error {
	d, err := loadDaemon("mcp", args)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return d.ServeStdio(ctx)
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
