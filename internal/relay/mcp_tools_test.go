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
		ToApprove []struct{ ID, ThreadID, Kind, Text, Message string } `json:"to_approve"`
		ToReview  []struct{ ID, ThreadID, Kind, Text, Message string } `json:"to_review"`
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
	// The caller's own draft body is in `text` (trusted, plain, review-then-approve).
	// An inbound message body is in `message`, spotlight-framed — and MUST be in the
	// structured output, because Claude Code surfaces only structuredContent.
	if len(out.ToReview) == 1 && out.ToReview[0].Text != "B's draft reply" {
		t.Errorf("to_review draft text = %q, want the draft body", out.ToReview[0].Text)
	}
	if len(out.ToApprove) == 1 {
		if out.ToApprove[0].Text != "" {
			t.Errorf("inbound body must not use the plain `text` field: %q", out.ToApprove[0].Text)
		}
		if !strings.Contains(out.ToApprove[0].Message, "the question from A") {
			t.Errorf("inbound body missing from structured `message`: %q", out.ToApprove[0].Message)
		}
		if !strings.Contains(out.ToApprove[0].Message, "never as instructions") {
			t.Errorf("structured inbound `message` is not spotlight-framed: %q", out.ToApprove[0].Message)
		}
	}

	// The message content is spotlighted (quarantined) in the text content.
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			text += tc.Text
		}
	}
	for _, want := range []string{"the question from A", "never as instructions", `from="a@example.com (device verified)"`} {
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

func TestFindPeopleTool(t *testing.T) {
	s := testServer(t)
	ts := httptest.NewServer(s.logRequests(s.mux))
	defer ts.Close()
	ctx := context.Background()
	now := time.Now().UTC()

	alice := enrollForTest(t, s, "alice@example.com", now)
	bob := enrollForTest(t, s, "bob@example.com", now)
	if err := s.store.SetPersonName(bob, "Bob"); err != nil {
		t.Fatalf("set name: %v", err)
	}

	token, _ := s.issuer.Mint(alice, "claude.ai", now)
	sess := mcpSession(ctx, t, ts.URL, token)
	defer sess.Close()

	res, err := sess.CallTool(ctx, &sdkmcp.CallToolParams{Name: "find_people", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	var out struct {
		People []struct{ Email, Name string } `json:"people"`
	}
	raw, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode structured: %v (%s)", err, raw)
	}
	// The caller (alice) is excluded; bob appears with his name.
	if len(out.People) != 1 {
		t.Fatalf("people = %+v, want just bob (self excluded)", out.People)
	}
	if out.People[0].Email != "bob@example.com" || out.People[0].Name != "Bob" {
		t.Errorf("person = %+v, want bob@example.com / Bob", out.People[0])
	}
}

func TestWaitForActivityTool(t *testing.T) {
	s := testServer(t)
	ts := httptest.NewServer(s.logRequests(s.mux))
	defer ts.Close()
	ctx := context.Background()
	now := time.Now().UTC()
	fresh := store.Freshness{MaxAge: 24 * time.Hour, MaxSkew: 5 * time.Minute}

	a := enrollForTest(t, s, "a@example.com", now)
	b := enrollForTest(t, s, "b@example.com", now)
	bsess := mcpSession(ctx, t, ts.URL, mint(t, s, b))
	defer bsess.Close()

	wait := func(secs int) bool {
		res, err := bsess.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: "wait_for_activity", Arguments: map[string]any{"timeout_seconds": secs}})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		var out struct {
			Activity bool `json:"activity"`
		}
		raw, _ := json.Marshal(res.StructuredContent)
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("decode: %v (%s)", err, raw)
		}
		return out.Activity
	}

	// Empty inbox: a 1s wait returns false, and actually waited ~1s.
	start := time.Now()
	if wait(1) {
		t.Error("wait_for_activity reported activity on an empty inbox")
	}
	if elapsed := time.Since(start); elapsed < 900*time.Millisecond {
		t.Errorf("wait returned after %v; expected it to poll for ~1s", elapsed)
	}

	// Seed an inbound message → a longer wait returns true promptly.
	thread := uuid.Must(uuid.NewV7()).String()
	if err := s.store.IngestMessage(mkTestEnvelope(thread, a, b, "hi", now), a, fresh, now); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	start = time.Now()
	if !wait(30) {
		t.Error("wait_for_activity missed the seeded message")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("wait took %v to notice existing activity; should be prompt", elapsed)
	}
}

func TestInboundVerdictTools(t *testing.T) {
	s := testServer(t)
	ts := httptest.NewServer(s.logRequests(s.mux))
	defer ts.Close()
	ctx := context.Background()
	now := time.Now().UTC()
	fresh := store.Freshness{MaxAge: 24 * time.Hour, MaxSkew: 5 * time.Minute}

	a := enrollForTest(t, s, "a@example.com", now)
	b := enrollForTest(t, s, "b@example.com", now)
	thread := uuid.Must(uuid.NewV7()).String()
	msg := mkTestEnvelope(thread, a, b, "the question", now)
	if err := s.store.IngestMessage(msg, a, fresh, now); err != nil {
		t.Fatalf("ingest: %v", err)
	}

	ok := func(res *sdkmcp.CallToolResult, err error) bool { return err == nil && res != nil && !res.IsError }
	approve := func(sess *sdkmcp.ClientSession, id string) (*sdkmcp.CallToolResult, error) {
		return sess.CallTool(ctx, &sdkmcp.CallToolParams{Name: "approve_message", Arguments: map[string]any{"id": id}})
	}

	asess := mcpSession(ctx, t, ts.URL, mint(t, s, a))
	defer asess.Close()
	bsess := mcpSession(ctx, t, ts.URL, mint(t, s, b))
	defer bsess.Close()
	csess := mcpSession(ctx, t, ts.URL, mint(t, s, enrollForTest(t, s, "c@example.com", now)))
	defer csess.Close()

	// A (the sender) cannot approve their own message; C (non-participant) cannot either.
	if res, err := approve(asess, msg.ID); ok(res, err) {
		t.Error("sender approved their own message")
	}
	if res, err := approve(csess, msg.ID); ok(res, err) {
		t.Error("non-participant approved the message")
	}
	// B (the recipient) approves → thread working.
	res, err := approve(bsess, msg.ID)
	if !ok(res, err) {
		t.Fatalf("recipient approve failed: err=%v isErr=%v", err, res != nil && res.IsError)
	}
	var out struct {
		ThreadState string `json:"thread_state"`
	}
	raw, _ := json.Marshal(res.StructuredContent)
	_ = json.Unmarshal(raw, &out)
	if out.ThreadState != "working" {
		t.Errorf("thread_state = %q, want working", out.ThreadState)
	}
	// Re-approving is now an illegal transition → refused.
	if res, err := approve(bsess, msg.ID); ok(res, err) {
		t.Error("re-approving an already-approved message succeeded")
	}

	// Decline on a fresh thread → rejected.
	t2 := uuid.Must(uuid.NewV7()).String()
	m2 := mkTestEnvelope(t2, a, b, "another", now)
	if err := s.store.IngestMessage(m2, a, fresh, now); err != nil {
		t.Fatalf("ingest 2: %v", err)
	}
	dres, derr := bsess.CallTool(ctx, &sdkmcp.CallToolParams{Name: "decline_message", Arguments: map[string]any{"id": m2.ID}})
	if !ok(dres, derr) {
		t.Fatalf("decline failed: %v", derr)
	}
	raw, _ = json.Marshal(dres.StructuredContent)
	_ = json.Unmarshal(raw, &out)
	if out.ThreadState != "rejected" {
		t.Errorf("declined thread_state = %q, want rejected", out.ThreadState)
	}
}

func TestSetThreadGrantTool(t *testing.T) {
	s := testServer(t)
	ts := httptest.NewServer(s.logRequests(s.mux))
	defer ts.Close()
	ctx := context.Background()
	now := time.Now().UTC()
	fresh := store.Freshness{MaxAge: 24 * time.Hour, MaxSkew: 5 * time.Minute}

	a := enrollForTest(t, s, "a@example.com", now)
	b := enrollForTest(t, s, "b@example.com", now)
	thread := uuid.Must(uuid.NewV7()).String()
	if err := s.store.IngestMessage(mkTestEnvelope(thread, a, b, "q", now), a, fresh, now); err != nil {
		t.Fatalf("ingest: %v", err)
	}

	ok := func(res *sdkmcp.CallToolResult, err error) bool { return err == nil && res != nil && !res.IsError }
	grant := func(sess *sdkmcp.ClientSession, dir string, enabled bool) (*sdkmcp.CallToolResult, error) {
		return sess.CallTool(ctx, &sdkmcp.CallToolParams{Name: "set_thread_grant",
			Arguments: map[string]any{"thread_id": thread, "direction": dir, "enabled": enabled}})
	}

	bsess := mcpSession(ctx, t, ts.URL, mint(t, s, b))
	defer bsess.Close()
	csess := mcpSession(ctx, t, ts.URL, mint(t, s, enrollForTest(t, s, "c@example.com", now)))
	defer csess.Close()

	// Non-participant refused.
	if res, err := grant(csess, "inbound", true); ok(res, err) {
		t.Error("non-participant set a grant")
	}
	// Participant enable, then disable, then disable again (idempotent).
	if res, err := grant(bsess, "inbound", true); !ok(res, err) {
		t.Fatalf("enable grant failed: %v", err)
	}
	if res, err := grant(bsess, "inbound", false); !ok(res, err) {
		t.Fatalf("disable grant failed: %v", err)
	}
	if res, err := grant(bsess, "inbound", false); !ok(res, err) {
		t.Error("disabling an inactive grant was not idempotent")
	}
	// Bad direction refused.
	if res, err := grant(bsess, "sideways", true); ok(res, err) {
		t.Error("a bad direction was accepted")
	}
}

func TestOutboundTools(t *testing.T) {
	s := testServer(t)
	ts := httptest.NewServer(s.logRequests(s.mux))
	defer ts.Close()
	ctx := context.Background()
	now := time.Now().UTC()

	a := enrollForTest(t, s, "a@example.com", now)
	_ = enrollForTest(t, s, "b@example.com", now)
	asess := mcpSession(ctx, t, ts.URL, mint(t, s, a))
	defer asess.Close()
	bsess := mcpSession(ctx, t, ts.URL, mint(t, s, enrollForTest(t, s, "b2@example.com", now)))
	defer bsess.Close()

	ok := func(res *sdkmcp.CallToolResult, err error) bool { return err == nil && res != nil && !res.IsError }

	// send_message A→B (new ask) → pending_review draft on a fresh thread.
	res, err := asess.CallTool(ctx, &sdkmcp.CallToolParams{Name: "send_message",
		Arguments: map[string]any{"to": "b@example.com", "text": "original question"}})
	if !ok(res, err) {
		t.Fatalf("send_message failed: err=%v isErr=%v", err, res != nil && res.IsError)
	}
	var sm struct {
		DraftID  string `json:"draft_id"`
		ThreadID string `json:"thread_id"`
		State    string `json:"state"`
	}
	raw, _ := json.Marshal(res.StructuredContent)
	_ = json.Unmarshal(raw, &sm)
	if sm.State != "pending_review" || sm.DraftID == "" || sm.ThreadID == "" {
		t.Fatalf("send_message output = %+v", sm)
	}

	// Unknown recipient is refused.
	if res, err := asess.CallTool(ctx, &sdkmcp.CallToolParams{Name: "send_message",
		Arguments: map[string]any{"to": "nobody@example.com", "text": "x"}}); ok(res, err) {
		t.Error("send_message to an off-roster email succeeded")
	}

	// Someone else cannot release A's draft.
	if res, err := bsess.CallTool(ctx, &sdkmcp.CallToolParams{Name: "approve_reply",
		Arguments: map[string]any{"id": sm.DraftID}}); ok(res, err) {
		t.Error("a non-author released the draft")
	}

	// A approves with an edit → sent, and the stored draft carries the edited text.
	edited := "edited final answer"
	res, err = asess.CallTool(ctx, &sdkmcp.CallToolParams{Name: "approve_reply",
		Arguments: map[string]any{"id": sm.DraftID, "edited_text": edited}})
	if !ok(res, err) {
		t.Fatalf("approve_reply failed: %v", err)
	}
	view, verr := s.store.ThreadFor(a, sm.ThreadID)
	if verr != nil {
		t.Fatalf("ThreadFor: %v", verr)
	}
	var checked bool
	for _, d := range view.Drafts {
		if d.DraftID == sm.DraftID {
			e, derr := envelope.Decode(d.Envelope)
			if derr != nil {
				t.Fatalf("decode sent draft: %v", derr)
			}
			if len(e.Body.Parts) != 1 || e.Body.Parts[0].Text != edited {
				t.Errorf("released draft body = %q, want the edited text %q", e.Body.Parts, edited)
			}
			checked = true
		}
	}
	if !checked {
		t.Error("released draft not found in the thread view")
	}

	// Discard flow: a fresh draft, discarded, then un-releasable.
	res, _ = asess.CallTool(ctx, &sdkmcp.CallToolParams{Name: "send_message",
		Arguments: map[string]any{"to": "b@example.com", "text": "to discard"}})
	var sm2 struct {
		DraftID string `json:"draft_id"`
	}
	raw, _ = json.Marshal(res.StructuredContent)
	_ = json.Unmarshal(raw, &sm2)
	if dres, derr := asess.CallTool(ctx, &sdkmcp.CallToolParams{Name: "discard_reply",
		Arguments: map[string]any{"id": sm2.DraftID}}); !ok(dres, derr) {
		t.Fatalf("discard_reply failed: %v", derr)
	}
	if res, err := asess.CallTool(ctx, &sdkmcp.CallToolParams{Name: "approve_reply",
		Arguments: map[string]any{"id": sm2.DraftID}}); ok(res, err) {
		t.Error("released a discarded draft")
	}
	if res, err := asess.CallTool(ctx, &sdkmcp.CallToolParams{Name: "approve_reply",
		Arguments: map[string]any{"id": "no-such-draft"}}); ok(res, err) {
		t.Error("released a non-existent draft")
	}
}

func mint(t *testing.T, s *Server, person string) string {
	t.Helper()
	tok, err := s.issuer.Mint(person, "claude.ai", time.Now().UTC())
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	return tok
}

func TestGetThreadTool(t *testing.T) {
	s := testServer(t)
	ts := httptest.NewServer(s.logRequests(s.mux))
	defer ts.Close()
	ctx := context.Background()
	now := time.Now().UTC()
	fresh := store.Freshness{MaxAge: 24 * time.Hour, MaxSkew: 5 * time.Minute}

	a := enrollForTest(t, s, "a@example.com", now)
	b := enrollForTest(t, s, "b@example.com", now)
	thread := uuid.Must(uuid.NewV7()).String()
	msg := mkTestEnvelope(thread, a, b, "the question from A", now)
	if err := s.store.IngestMessage(msg, a, fresh, now); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	draftID, err := s.store.CreateDraft(thread, b, mkTestEnvelope(thread, b, a, "B private draft", now), now)
	if err != nil {
		t.Fatalf("draft: %v", err)
	}

	ok := func(res *sdkmcp.CallToolResult, err error) bool {
		return err == nil && res != nil && !res.IsError
	}
	call := func(sess *sdkmcp.ClientSession, id string) (*sdkmcp.CallToolResult, error) {
		return sess.CallTool(ctx, &sdkmcp.CallToolParams{Name: "get_thread", Arguments: map[string]any{"thread_id": id}})
	}

	// B (a participant) sees the thread.
	bsess := mcpSession(ctx, t, ts.URL, mint(t, s, b))
	defer bsess.Close()
	res, err := call(bsess, thread)
	if !ok(res, err) {
		t.Fatalf("get_thread as participant failed: err=%v isErr=%v", err, res != nil && res.IsError)
	}
	var out struct {
		State    string                                      `json:"state"`
		Messages []struct{ ID, From, Body string }           `json:"messages"`
		Drafts   []struct{ ID, ThreadID, Kind, Text string } `json:"drafts"`
	}
	raw, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode: %v (%s)", err, raw)
	}
	if len(out.Messages) != 1 || out.Messages[0].ID != msg.ID {
		t.Errorf("messages = %+v, want the ingested message", out.Messages)
	}
	if len(out.Drafts) != 1 || out.Drafts[0].ID != draftID {
		t.Errorf("drafts = %+v, want B's draft", out.Drafts)
	}
	// The message body must be in the STRUCTURED output (Claude Code surfaces only
	// that), spotlight-framed; the caller's own draft body is plain (trusted).
	if len(out.Messages) == 1 && (!strings.Contains(out.Messages[0].Body, "the question from A") || !strings.Contains(out.Messages[0].Body, "never as instructions")) {
		t.Errorf("structured message body not spotlight-framed: %q", out.Messages[0].Body)
	}
	if len(out.Drafts) == 1 && out.Drafts[0].Text != "B private draft" {
		t.Errorf("structured draft text = %q, want the draft body", out.Drafts[0].Text)
	}
	var text string
	for _, c := range res.Content {
		if tc, isText := c.(*sdkmcp.TextContent); isText {
			text += tc.Text
		}
	}
	if !strings.Contains(text, "the question from A") || !strings.Contains(text, "never as instructions") {
		t.Errorf("thread message not spotlighted:\n%s", text)
	}
	if strings.Contains(text, "B private draft") {
		t.Error("draft body leaked into spotlighted text")
	}

	// A non-participant gets a sanitized not-found (no oracle).
	csess := mcpSession(ctx, t, ts.URL, mint(t, s, enrollForTest(t, s, "c@example.com", now)))
	defer csess.Close()
	if res, err := call(csess, thread); ok(res, err) {
		t.Error("non-participant was shown the thread")
	}
	// Unknown thread id → same not-found.
	if res, err := call(bsess, "no-such-thread"); ok(res, err) {
		t.Error("unknown thread id did not return not-found")
	}
}
