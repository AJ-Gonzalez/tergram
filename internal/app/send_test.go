package app

import (
	"context"
	"strings"
	"testing"

	"github.com/gotd/td/tgerr"

	"tergram/internal/tgc"
)

// sendRPCClient wraps fakeClient but makes Send fail the first n calls with
// the given RPC error, then succeed. It records every send target.
type sendRPCClient struct {
	*fakeClient
	failWith error
	fails    int
	sends    int
	targets  []int64
}

func (s *sendRPCClient) Send(ctx context.Context, d tgc.Dialog, text string) error {
	s.sends++
	s.targets = append(s.targets, d.ID)
	if s.fails > 0 {
		s.fails--
		return s.failWith
	}
	return s.fakeClient.Send(ctx, d, text)
}

// openChatAt loads the dialog list, opens the first chat, and returns the
// model plus the open dialog.
func openChatAt(t *testing.T, c tgc.Client) (Model, tgc.Dialog) {
	t.Helper()
	m := New(c)
	m, _ = load(m, m.loadDialogsCmd())
	m, _ = step(m, key("space"))
	d, ok := m.currentDialog()
	if !ok {
		t.Fatalf("no dialog after opening chat")
	}
	return m, d
}

func TestSendPeerInvalidRefreshesAndRetries(t *testing.T) {
	sc := &sendRPCClient{
		fakeClient: &fakeClient{dialogs: []tgc.Dialog{{ID: 3, Title: "Chat"}}},
		failWith:   tgerr.New(400, "PEER_ID_INVALID"),
		fails:      1,
	}
	m, d := openChatAt(t, sc)

	// First send fails with a stale peer: a dialog refresh is issued, the
	// text is stashed, no error is shown yet.
	m, cmd := load(m, m.sendCmd(d, "hi"))
	if sc.sends != 1 {
		t.Fatalf("expected 1 send, got %d", sc.sends)
	}
	if m.err != "" {
		t.Fatalf("peer-invalid send should not surface an error yet, got %q", m.err)
	}
	if m.pendingSend != "hi" {
		t.Fatalf("expected stashed pending send %q, got %q", "hi", m.pendingSend)
	}
	if cmd == nil {
		t.Fatalf("expected a dialog refresh cmd after peer-invalid send")
	}

	// The refresh lands and consumes the stash; the retry send fires. The
	// peerRefreshed latch is what prevents a second refresh cycle.
	m, cmd = load(m, cmd)
	if m.pendingSend != "" {
		t.Fatalf("stash must be consumed when the retry fires, got %q", m.pendingSend)
	}
	if cmd == nil {
		t.Fatalf("expected the retry send cmd after refresh")
	}
	m, _ = load(m, cmd)
	if sc.sends != 2 {
		t.Fatalf("expected exactly one retry (2 sends total), got %d", sc.sends)
	}
	if sc.targets[1] != 3 {
		t.Fatalf("retry must target the same chat (id 3), got %d", sc.targets[1])
	}
	if m.err != "" {
		t.Fatalf("expected no error after successful retry, got %q", m.err)
	}
	if m.pendingSend != "" || m.peerRefreshed {
		t.Fatalf("retry state must clear after success (pending=%q refreshed=%v)", m.pendingSend, m.peerRefreshed)
	}
}

func TestSendPeerInvalidGivesUpAfterOneRetry(t *testing.T) {
	sc := &sendRPCClient{
		fakeClient: &fakeClient{dialogs: []tgc.Dialog{{ID: 3, Title: "Chat"}}},
		failWith:   tgerr.New(400, "PEER_ID_INVALID"),
		fails:      2, // original send + the retry both fail
	}
	m, d := openChatAt(t, sc)

	m, cmd := load(m, m.sendCmd(d, "hi"))
	m, cmd = load(m, cmd) // refresh lands, retry fires
	m, _ = load(m, cmd)   // retry fails again
	if sc.sends != 2 {
		t.Fatalf("retry must happen exactly once, got %d sends", sc.sends)
	}
	if !strings.Contains(m.err, "PEER_ID_INVALID") {
		t.Fatalf("expected the error name in the message, got %q", m.err)
	}
	if m.pendingSend != "" || m.peerRefreshed {
		t.Fatalf("retry state must clear after failure (pending=%q refreshed=%v)", m.pendingSend, m.peerRefreshed)
	}
}

func TestSendForbiddenNotRefreshed(t *testing.T) {
	sc := &sendRPCClient{
		fakeClient: &fakeClient{dialogs: []tgc.Dialog{{ID: 3, Title: "Chat"}}},
		failWith:   tgerr.New(400, "CHAT_WRITE_FORBIDDEN"),
		fails:      100,
	}
	m, d := openChatAt(t, sc)

	m, cmd := load(m, m.sendCmd(d, "hi"))
	if sc.sends != 1 {
		t.Fatalf("forbidden send must not retry, got %d sends", sc.sends)
	}
	if cmd != nil {
		t.Fatalf("forbidden send must not issue a refresh cmd")
	}
	if !strings.Contains(m.err, "no permission") || !strings.Contains(m.err, "CHAT_WRITE_FORBIDDEN") {
		t.Fatalf("expected permission hint + error name, got %q", m.err)
	}
	if m.pendingSend != "" {
		t.Fatalf("no pending send expected, got %q", m.pendingSend)
	}
}

func TestSendNonRPCErrorShownVerbatim(t *testing.T) {
	sc := &sendRPCClient{
		fakeClient: &fakeClient{dialogs: []tgc.Dialog{{ID: 3, Title: "Chat"}}},
		failWith:   context.DeadlineExceeded,
		fails:      1,
	}
	m, d := openChatAt(t, sc)

	m, cmd := load(m, m.sendCmd(d, "hi"))
	if cmd != nil {
		t.Fatalf("non-RPC send error must not issue a refresh cmd")
	}
	if m.err != context.DeadlineExceeded.Error() {
		t.Fatalf("expected the raw error verbatim, got %q", m.err)
	}
}
