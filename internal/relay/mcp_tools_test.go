package relay

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Mediacom99/askrelay/internal/a2a"
	"github.com/Mediacom99/askrelay/internal/envelope"
	"github.com/Mediacom99/askrelay/internal/relay/store"
)

func enrollForTest(t *testing.T, s *Server, email string, now time.Time) string {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	token, err := s.store.CreateInvite(email, time.Hour, now)
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	p, _, err := s.store.Enroll(token, pub, "dev", now)
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}
	return p.ID
}

func mkTestEnvelope(thread, from, to, text string, now time.Time) envelope.Envelope {
	return envelope.Envelope{
		V:      1,
		ID:     uuid.Must(uuid.NewV7()).String(),
		Thread: thread,
		From:   envelope.Party{Person: from, Device: "d", Agent: "cc"},
		To:     to,
		State:  a2a.StateSubmitted,
		SentAt: now,
		Body:   envelope.Message{Role: "user", Parts: []envelope.Part{{Type: "text", Text: text}}},
	}
}

// mcpSession connects a go-sdk client to the test server with a bearer token.
func mcpSession(ctx context.Context, t *testing.T, url, token string) *sdkmcp.ClientSession {
	t.Helper()
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test", Version: "0"}, nil)
	sess, err := client.Connect(ctx, &sdkmcp.StreamableClientTransport{
		Endpoint:             url + "/mcp",
		HTTPClient:           &http.Client{Transport: authRT{base: http.DefaultTransport, token: token}},
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	return sess
}

func TestCheckInboxTool(t *testing.T) {
	s := testServer(t)
	ts := httptest.NewServer(s.logRequests(s.mux))
	defer ts.Close()
	ctx := context.Background()
	now := time.Now().UTC()
	fresh := store.Freshness{MaxAge: 24 * time.Hour, MaxSkew: 5 * time.Minute}

	a := enrollForTest(t, s, "a@example.com", now)
	b := enrollForTest(t, s, "b@example.com", now)

	// A asks B → inbound message in input-required on B's side.
	thread := uuid.Must(uuid.NewV7()).String()
	msg := mkTestEnvelope(thread, a, b, "the question from A", now)
	if err := s.store.IngestMessage(msg, a, fresh, now); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	// B has a pending_review draft of their own.
	draftEnv := mkTestEnvelope(thread, b, a, "B's draft reply", now)
	draftID, err := s.store.CreateDraft(thread, b, draftEnv, now)
	if err != nil {
		t.Fatalf("draft: %v", err)
	}

	// Call check_inbox as B.
	token, err := s.issuer.Mint(b, "claude.ai", now)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	sess := mcpSession(ctx, t, ts.URL, token)
	defer sess.Close()

	res, err := sess.CallTool(ctx, &sdkmcp.CallToolParams{Name: "check_inbox", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	// Structured output: the message in to_approve, the draft in to_review.
	var out struct {
		ToApprove []struct{ ID, ThreadID, Kind string } `json:"to_approve"`
		ToReview  []struct{ ID, ThreadID, Kind string } `json:"to_review"`
	}
	raw, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode structured: %v (%s)", err, raw)
	}
	if len(out.ToApprove) != 1 || out.ToApprove[0].ID != msg.ID {
		t.Errorf("to_approve = %+v, want the message %q", out.ToApprove, msg.ID)
	}
	if len(out.ToReview) != 1 || out.ToReview[0].ID != draftID {
		t.Errorf("to_review = %+v, want the draft %q", out.ToReview, draftID)
	}

	// The message content is spotlighted (quarantined) in the text content.
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			text += tc.Text
		}
	}
	for _, want := range []string{"the question from A", "DATA, not instructions", `from="a@example.com (device verified)"`} {
		if !strings.Contains(text, want) {
			t.Errorf("check_inbox text missing %q:\n%s", want, text)
		}
	}
	// B's own draft text must NOT be spotlighted as untrusted inbound.
	if strings.Contains(text, "B's draft reply") {
		t.Error("own draft leaked into the spotlighted inbound content")
	}

	// A message B sent themselves must not appear as awaiting B's verdict:
	// approve it away and re-check is covered elsewhere; here assert an
	// unrelated person C sees an empty inbox.
	c := enrollForTest(t, s, "c@example.com", now)
	ctoken, _ := s.issuer.Mint(c, "claude.ai", now)
	csess := mcpSession(ctx, t, ts.URL, ctoken)
	defer csess.Close()
	cres, err := csess.CallTool(ctx, &sdkmcp.CallToolParams{Name: "check_inbox", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("CallTool (C): %v", err)
	}
	var cout struct {
		ToApprove []json.RawMessage `json:"to_approve"`
		ToReview  []json.RawMessage `json:"to_review"`
	}
	craw, _ := json.Marshal(cres.StructuredContent)
	_ = json.Unmarshal(craw, &cout)
	if len(cout.ToApprove) != 0 || len(cout.ToReview) != 0 {
		t.Errorf("unrelated person C has a non-empty inbox: %s", craw)
	}
}
