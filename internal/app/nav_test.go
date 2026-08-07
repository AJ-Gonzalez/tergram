package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"tergram/internal/tgc"
)

// key builds a KeyPressMsg whose Code matches what a real key press produces
// (and thus what keyName() returns), so the nav dispatch is exercised exactly
// as it would be live.
func key(tok string) tea.KeyPressMsg {
	switch tok {
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEsc}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	}
	return tea.KeyPressMsg{Code: rune(tok[0]), Text: tok}
}

// step runs msg through Update and returns the new concrete Model plus the
// resulting Cmd, exactly as the Bubble Tea runtime would.
func step(m Model, msg tea.Msg) (Model, tea.Cmd) {
	mm, cmd := m.Update(msg)
	return mm.(Model), cmd
}

// load runs a Cmd to completion and feeds its result back through Update.
func load(m Model, cmd tea.Cmd) (Model, tea.Cmd) {
	if cmd == nil {
		return m, nil
	}
	next, c2 := m.Update(cmd())
	if c2 != nil {
		return next.(Model), c2
	}
	return next.(Model), c2
}

func newLoaded(t *testing.T) Model {
	t.Helper()
	m := New(tgc.NewDemo(3))
	m, _ = load(m, m.loadDialogsCmd())
	if len(m.dialogs) != 3 {
		t.Fatalf("expected 3 dialogs, got %d", len(m.dialogs))
	}
	return m
}

func TestListVimMotion(t *testing.T) {
	m := newLoaded(t)

	steps := []struct {
		key  string
		want int
	}{
		{"j", 1},
		{"j", 2},
		{"k", 1},
		{"down", 2},
		{"up", 1},
		{"k", 0},
	}
	for _, s := range steps {
		m, _ = step(m, key(s.key))
		if m.listIdx != s.want {
			t.Fatalf("%s: expected listIdx %d, got %d", s.key, s.want, m.listIdx)
		}
	}
	// Clamp at the top and bottom.
	for i := 0; i < 5; i++ {
		m, _ = step(m, key("k"))
	}
	if m.listIdx != 0 {
		t.Fatalf("top clamp: expected 0, got %d", m.listIdx)
	}
	for i := 0; i < 5; i++ {
		m, _ = step(m, key("j"))
	}
	if m.listIdx != len(m.dialogs)-1 {
		t.Fatalf("bottom clamp: expected %d, got %d", len(m.dialogs)-1, m.listIdx)
	}
}

func TestOpenChatComposeSend(t *testing.T) {
	m := newLoaded(t)

	// Space selects/opens the chat (level 0 → 1).
	m, cmd := step(m, key("space"))
	if m.level() != 1 {
		t.Fatalf("space: expected level 1, got %d", m.level())
	}
	m, _ = load(m, cmd)
	if len(m.messages) == 0 {
		t.Fatalf("expected messages after opening chat")
	}
	// Newly opened chat is in browse mode (not inserting).
	if m.inserting {
		t.Fatalf("chat should open in browse mode")
	}

	// Enter enters compose mode.
	m, _ = step(m, key("enter"))
	if !m.inserting {
		t.Fatalf("enter should enter compose mode")
	}

	// Letters (including j/k/h/l/space) are typed, not navigation.
	for _, k := range []string{"h", "i", " ", "j"} {
		m, _ = step(m, key(k))
	}
	if got := string(m.composer); got != "hi j" {
		t.Fatalf("composer: expected %q, got %q", "hi j", got)
	}

	// Backspace deletes.
	m, _ = step(m, key("backspace"))
	if got := string(m.composer); got != "hi " {
		t.Fatalf("after backspace: expected %q, got %q", "hi ", got)
	}

	// Enter sends (compose mode) and clears.
	m, sendCmd := step(m, key("enter"))
	if got := string(m.composer); got != "" {
		t.Fatalf("composer not cleared after send: %q", got)
	}
	for i := 0; i < 3 && sendCmd != nil; i++ {
		m, sendCmd = load(m, sendCmd)
	}
	found := false
	for _, msg := range m.messages {
		if strings.Contains(msg.Text, "hi") {
			found = true
		}
	}
	if !found {
		t.Fatalf("sent message not present after send")
	}
}

func TestPasteIntoComposer(t *testing.T) {
	m := newLoaded(t)

	// Paste at level 0 (chat list) is ignored — there is no composer.
	m, _ = step(m, tea.PasteMsg{Content: "hello"})
	if m.level() != 0 || len(m.composer) != 0 {
		t.Fatalf("paste at level 0 should be ignored")
	}

	// Open a chat; it starts in browse mode.
	m, cmd := step(m, key("space"))
	m, _ = load(m, cmd)
	if m.inserting {
		t.Fatalf("chat should open in browse mode")
	}

	// Paste in browse mode starts composing and inserts the text,
	// collapsing line breaks to spaces (single-line composer).
	m, _ = step(m, tea.PasteMsg{Content: "line1\r\nline2\nline3"})
	if !m.inserting {
		t.Fatalf("paste in browse mode should enter compose mode")
	}
	if got := string(m.composer); got != "line1 line2 line3" {
		t.Fatalf("composer: expected %q, got %q", "line1 line2 line3", got)
	}
	if m.composerN != len(m.composer) {
		t.Fatalf("caret should sit after pasted text")
	}

	// Paste in insert mode appends at the caret.
	m, _ = step(m, tea.PasteMsg{Content: "!"})
	if got := string(m.composer); got != "line1 line2 line3!" {
		t.Fatalf("composer: expected %q, got %q", "line1 line2 line3!", got)
	}

	// Pasting with the caret in the middle inserts there.
	m.composerN = 5
	m, _ = step(m, tea.PasteMsg{Content: "X"})
	if got := string(m.composer); got != "line1X line2 line3!" {
		t.Fatalf("composer: expected %q, got %q", "line1X line2 line3!", got)
	}

	// Empty paste is a no-op.
	m, _ = step(m, tea.PasteMsg{Content: ""})
	if got := string(m.composer); got != "line1X line2 line3!" {
		t.Fatalf("empty paste should not change the composer")
	}
}

func TestEscAndQLevels(t *testing.T) {
	m := newLoaded(t)

	// q at top level quits (returns a command); stays at level 0.
	_, cmd := step(m, key("q"))
	if cmd == nil {
		t.Fatalf("q at top level should produce a quit command")
	}
	if m.level() != 0 {
		t.Fatalf("q at top should stay at level 0")
	}

	// Open a chat (browse mode).
	m, loadCmd := step(m, key("space"))
	m, _ = load(m, loadCmd)
	if m.level() != 1 {
		t.Fatalf("expected level 1")
	}

	// q inside a chat (browse) must NOT quit; it goes back one level.
	m, cmd = step(m, key("q"))
	if cmd != nil {
		t.Fatalf("q inside a chat must not quit (got a quit command)")
	}
	if m.level() != 0 {
		t.Fatalf("q inside chat should go back to level 0")
	}

	// Reopen; esc in browse goes back a level.
	m, loadCmd = step(m, key("space"))
	m, _ = load(m, loadCmd)
	if m.level() != 1 {
		t.Fatalf("expected level 1 again")
	}
	m, _ = step(m, key("esc"))
	if m.level() != 0 {
		t.Fatalf("esc inside chat (browse) should go back to level 0")
	}

	// esc at top level is a no-op.
	m, _ = step(m, key("esc"))
	if m.level() != 0 {
		t.Fatalf("esc at top should stay level 0")
	}

	// In compose mode, esc exits compose first, then backs out on a second esc.
	m, loadCmd = step(m, key("space"))
	m, _ = load(m, loadCmd)
	m, _ = step(m, key("enter")) // compose
	if !m.inserting {
		t.Fatalf("expected compose mode")
	}
	m, _ = step(m, key("esc")) // first esc → browse, still level 1
	if m.inserting {
		t.Fatalf("first esc should exit compose mode")
	}
	if m.level() != 1 {
		t.Fatalf("first esc should stay in the chat")
	}
	m, _ = step(m, key("esc")) // second esc → back to list
	if m.level() != 0 {
		t.Fatalf("second esc should go back to level 0")
	}
}
