package app

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/gotd/td/tgerr"

	"tergram/internal/tgc"
)

// fakeClient is a controllable tgc.Client for driving the flood-retry logic
// without a network. floodOnce makes the next Dialogs/Messages call fail with
// a FLOOD_WAIT; dialogsErr/messagesErr make every such call fail.
type fakeClient struct {
	dialogs     []tgc.Dialog
	dialogsErr  error
	messages    []tgc.Message
	messagesErr error
	floodOnce   bool
}

func (f *fakeClient) Dialogs(ctx context.Context) ([]tgc.Dialog, error) {
	if f.floodOnce {
		f.floodOnce = false
		return nil, tgerr.New(420, "FLOOD_WAIT_27")
	}
	return f.dialogs, f.dialogsErr
}

func (f *fakeClient) Messages(ctx context.Context, d tgc.Dialog) ([]tgc.Message, error) {
	if f.floodOnce {
		f.floodOnce = false
		return nil, tgerr.New(420, "FLOOD_WAIT_27")
	}
	return f.messages, f.messagesErr
}

func (f *fakeClient) Send(ctx context.Context, d tgc.Dialog, text string) error {
	return nil
}

func (f *fakeClient) Updates() <-chan tgc.Update { return nil }

func (f *fakeClient) Close() {}

// sendFloodClient wraps fakeClient but makes the first Send fail with a flood.
type sendFloodClient struct {
	*fakeClient
	sends int
}

func (s *sendFloodClient) Send(ctx context.Context, d tgc.Dialog, text string) error {
	s.sends++
	if s.sends == 1 {
		return tgerr.New(420, "FLOOD_WAIT_27")
	}
	return s.fakeClient.Send(ctx, d, text)
}

// floodStep runs a fetch cmd and feeds its result through Update, returning
// the model plus the follow-up cmd (a tea.Tick for floods). The tick's timer
// is bubbletea's own; its output (retryNowMsg) is fed in by the tests.
func floodStep(m Model, cmd func() tea.Msg) (Model, tea.Cmd) {
	next, c2 := m.Update(cmd())
	return next.(Model), c2
}

func TestDialogsFloodRetry(t *testing.T) {
	c := &fakeClient{
		floodOnce: true,
		dialogs:   []tgc.Dialog{{ID: 1, Title: "Alpha"}, {ID: 2, Title: "Beta"}},
	}
	m := New(c)

	// First fetch is throttled: expect a status line, not a dead-end error.
	m, tick := floodStep(m, m.loadDialogsCmd())
	if tick == nil {
		t.Fatal("expected a tick cmd after a flood")
	}
	if m.err != "" {
		t.Fatalf("flood must not surface as an error: %q", m.err)
	}
	if !strings.Contains(m.status, "retrying") {
		t.Fatalf("expected retry status, got %q", m.status)
	}

	// The wait elapses; the fetch is re-issued and succeeds.
	m, fetchCmd := step(m, retryNowMsg{step: stepDialogs, attempt: 2})
	if fetchCmd == nil {
		t.Fatal("expected a re-issued fetch cmd after the tick")
	}
	m, _ = load(m, fetchCmd)
	if len(m.dialogs) != 2 {
		t.Fatalf("expected 2 dialogs after retry, got %d", len(m.dialogs))
	}
	if m.status != "" || m.err != "" {
		t.Fatalf("status/err should be cleared after success: status=%q err=%q", m.status, m.err)
	}
	if !m.dialogsLoaded {
		t.Fatal("dialogsLoaded should be set after a successful fetch")
	}
}

func TestDialogsFloodGivesUp(t *testing.T) {
	c := &fakeClient{dialogsErr: tgerr.New(420, "FLOOD_WAIT_27")}
	m := New(c)

	attempt := 1
	var fetchCmd tea.Cmd
	m, _ = floodStep(m, m.loadDialogsCmd())
	for m.err == "" && attempt <= maxFloodRetries+1 {
		m, fetchCmd = step(m, retryNowMsg{step: stepDialogs, attempt: attempt + 1})
		attempt++
		if fetchCmd == nil {
			break
		}
		m, _ = floodStep(m, fetchCmd)
	}
	if m.err == "" {
		t.Fatalf("expected give-up error after %d retries, status=%q", maxFloodRetries, m.status)
	}
	if !strings.Contains(m.err, "throttled") {
		t.Fatalf("expected throttled give-up error, got %q", m.err)
	}
	if m.status != "" {
		t.Fatalf("status should clear when giving up, got %q", m.status)
	}
}

func TestMessagesFloodRetry(t *testing.T) {
	c := &fakeClient{
		dialogs:  []tgc.Dialog{{ID: 7, Title: "Chat"}},
		messages: []tgc.Message{{ID: 1, Text: "hello"}},
	}
	m := New(c)
	m, _ = load(m, m.loadDialogsCmd())

	// Open the chat; the history fetch floods once, then succeeds.
	m, fetchCmd := step(m, key("space"))
	c.floodOnce = true
	m, tick := floodStep(m, fetchCmd)
	if tick == nil {
		t.Fatal("expected a tick cmd after a flood")
	}
	if m.err != "" {
		t.Fatalf("flood must not surface as an error: %q", m.err)
	}
	if !strings.Contains(m.status, "throttling") {
		t.Fatalf("expected throttle status, got %q", m.status)
	}

	m, fetchCmd = step(m, retryNowMsg{step: stepMessages, attempt: 2, d: c.dialogs[0]})
	if fetchCmd == nil {
		t.Fatal("expected a re-issued fetch cmd after the tick")
	}
	m, _ = load(m, fetchCmd)
	if m.openID != 7 {
		t.Fatalf("expected chat 7 open, got %d", m.openID)
	}
	if len(m.messages) != 1 || m.messages[0].Text != "hello" {
		t.Fatalf("expected the retried history, got %+v", m.messages)
	}
	if m.status != "" || m.err != "" {
		t.Fatalf("status/err should be cleared: status=%q err=%q", m.status, m.err)
	}
}

func TestSendFloodIsNotAutoRetried(t *testing.T) {
	sc := &sendFloodClient{fakeClient: &fakeClient{dialogs: []tgc.Dialog{{ID: 3, Title: "Chat"}}}}
	m := New(sc)
	m, _ = load(m, m.loadDialogsCmd())
	m, _ = step(m, key("space"))
	d, _ := m.currentDialog()

	// A throttled send surfaces as a hint and is never re-fired automatically.
	m, _ = load(m, m.sendCmd(d, "hi"))
	if m.err == "" || !strings.Contains(m.err, "throttled") {
		t.Fatalf("expected throttled send hint, got %q", m.err)
	}
	if sc.sends != 1 {
		t.Fatalf("send must not auto-retry: %d sends", sc.sends)
	}
}
