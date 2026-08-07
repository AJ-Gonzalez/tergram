package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"tergram/internal/tgc"
)

// Messages flowing between the client commands and the Update loop.
type (
	loadedDialogsMsg struct {
		dialogs []tgc.Dialog
		err     error
	}
	loadedMessagesMsg struct {
		id   int64
		msgs []tgc.Message
		err  error
	}
	sentMsg     struct{ err error }
	updateMsg   struct{ u tgc.Update }
	pollStopped struct{}
)

// Flood-aware fetch retry. Telegram rate limits (FLOOD_WAIT) surface as plain
// RPC errors; instead of showing a dead-end error screen we wait out the
// required duration and retry, reporting progress on the status line. Sends
// are never auto-retried (that could double-send); they get a friendly hint.
type fetchStep int

const (
	stepDialogs fetchStep = iota
	stepMessages
)

// maxFloodRetries bounds how many times a fetch retries after being
// throttled before giving up with a clear message (~6 minutes at 27s waits).
const maxFloodRetries = 12

// floodRetryMsg reports a throttled fetch; the Update loop waits out wait and
// re-issues the fetch via retryNowMsg.
type floodRetryMsg struct {
	what    string // label for the status line
	wait    time.Duration
	attempt int // 1-based attempt number that was throttled
	step    fetchStep
	d       tgc.Dialog // set for stepMessages
}

// retryNowMsg is emitted when a flood wait elapses; it re-issues the fetch.
type retryNowMsg struct {
	step    fetchStep
	attempt int
	d       tgc.Dialog
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadDialogsCmd(), m.waitUpdates())
}

func (m Model) loadDialogsCmd() tea.Cmd {
	return m.fetchDialogs(1)
}

func (m Model) fetchDialogs(attempt int) tea.Cmd {
	return func() tea.Msg {
		ds, err := m.client.Dialogs(context.Background())
		if err != nil {
			if wait, ok := tgc.FloodWait(err); ok {
				return floodRetryMsg{what: "chat list", wait: wait, attempt: attempt, step: stepDialogs}
			}
			return loadedDialogsMsg{err: err}
		}
		return loadedDialogsMsg{dialogs: ds}
	}
}

func (m Model) loadMessagesCmd(d tgc.Dialog) tea.Cmd {
	return m.fetchMessages(d, 1)
}

func (m Model) fetchMessages(d tgc.Dialog, attempt int) tea.Cmd {
	return func() tea.Msg {
		ms, err := m.client.Messages(context.Background(), d)
		if err != nil {
			if wait, ok := tgc.FloodWait(err); ok {
				return floodRetryMsg{what: "chat " + d.Title, wait: wait, attempt: attempt, step: stepMessages, d: d}
			}
			return loadedMessagesMsg{id: d.ID, err: err}
		}
		return loadedMessagesMsg{id: d.ID, msgs: ms}
	}
}

func (m Model) sendCmd(d tgc.Dialog, text string) tea.Cmd {
	return func() tea.Msg {
		err := m.client.Send(context.Background(), d, text)
		if err != nil {
			if wait, ok := tgc.FloodWait(err); ok {
				err = fmt.Errorf("send throttled (%s); press enter to retry", wait.Round(time.Second))
			}
		}
		return sentMsg{err: err}
	}
}

// waitUpdates blocks on the client's update stream and re-polls after each
// real update. It stops (returns pollStopped) when the stream is closed.
func (m Model) waitUpdates() tea.Cmd {
	return func() tea.Msg {
		u, ok := <-m.client.Updates()
		if !ok {
			return pollStopped{}
		}
		return updateMsg{u: u}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m.updateKey(msg)

	case loadedDialogsMsg:
		m.status = ""
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.dialogs = msg.dialogs
		m.dialogsLoaded = true
		m.listIdx = 0
		m.err = ""
		return m, nil

	case floodRetryMsg:
		if msg.attempt > maxFloodRetries {
			m.status = ""
			m.err = fmt.Sprintf("%s still throttled after %d retries; restart the app to try again", msg.what, maxFloodRetries)
			return m, nil
		}
		m.status = fmt.Sprintf("Telegram is throttling the %s (%s); retrying…", msg.what, msg.wait.Round(time.Second))
		return m, tea.Tick(msg.wait+time.Second, func(time.Time) tea.Msg {
			return retryNowMsg{step: msg.step, attempt: msg.attempt + 1, d: msg.d}
		})

	case retryNowMsg:
		switch msg.step {
		case stepDialogs:
			return m, m.fetchDialogs(msg.attempt)
		case stepMessages:
			return m, m.fetchMessages(msg.d, msg.attempt)
		}
		return m, nil

	case loadedMessagesMsg:
		m.loadingChat = false
		m.status = ""
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.messages = msg.msgs
		if len(m.messages) > 0 {
			m.msgCur = len(m.messages) - 1 // anchor to newest
		}
		return m, nil

	case sentMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		// Reload history so the sent message shows up (client echoes it in
		// the update stream, but reloading keeps ordering correct).
		if d, ok := m.currentDialog(); ok {
			return m, m.loadMessagesCmd(d)
		}
		return m, nil

	case updateMsg:
		return m.applyUpdate(msg.u), m.waitUpdates()

	case pollStopped:
		return m, nil
	}
	return m, nil
}

func (m Model) applyUpdate(u tgc.Update) Model {
	// Only surface updates for the currently open chat, appended at the end.
	if u.DialogID != m.openID {
		return m
	}
	msg := u.Message
	sender := msg.Sender
	if sender == "" {
		if msg.Outgoing {
			sender = "You"
		} else {
			sender = "?"
		}
	}
	msg.Sender = sender
	msg.Outgoing = false
	m.messages = append(m.messages, msg)
	if m.msgCur < len(m.messages)-1 {
		m.msgCur = len(m.messages) - 1 // keep newest anchored unless user scrolled up
	}
	return m
}

// keyName maps a key press to a stable token used by the navigation dispatch.
// It matches on the key Code (the type-safe way), not String(), because
// String() for space/enter returns surprising values ("space", "\r").
func keyName(k tea.KeyPressMsg) string {
	switch k.Code {
	case tea.KeyUp:
		return "up"
	case tea.KeyDown:
		return "down"
	case tea.KeyLeft:
		return "left"
	case tea.KeyRight:
		return "right"
	case tea.KeyEnter:
		return "enter"
	case tea.KeyBackspace:
		return "backspace"
	case tea.KeyEsc:
		return "esc"
	case tea.KeySpace:
		return "space"
	}
	if k.Code > 0 && k.Code < 128 {
		return string(k.Code)
	}
	return ""
}

func (m Model) updateKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	name := keyName(msg)
	switch m.level() {
	case 0:
		// Chat list: full vim navigation. Space/enter select (open a chat);
		// q quits only here (top level); esc is a no-op at the bottom.
		switch name {
		case "j", "down":
			m.moveList(+1)
		case "k", "up":
			m.moveList(-1)
		case "h", "l", "left", "right":
			// nothing horizontal to move in the list
		case "space", "enter":
			return m.openChat()
		case "q":
			return m, tea.Quit // top level: quit
		case "esc":
		}

	case 1:
		if m.inserting {
			// Compose mode: printable characters (incl. space, letters, q)
			// all go to the composer. Only a few control keys act as commands.
			if msg.Text != "" {
				m.insertComposer(msg.Text)
				return m, nil
			}
			switch name {
			case "backspace":
				m.composerBackspace()
			case "enter":
				return m.sendCurrent()
			case "esc":
				m.inserting = false // back to browse mode (one esc)
			case "up":
				m.moveMsg(-1)
			case "down":
				m.moveMsg(+1)
			}
			return m, nil
		}
		// Browse mode: hjkl/arrows navigate messages, enter/i/a start
		// composing, esc/q go back a level.
		switch name {
		case "j", "down":
			m.moveMsg(+1)
		case "k", "up":
			m.moveMsg(-1)
		case "h", "l", "left", "right":
			// no horizontal navigation in a chat; ignore
		case "space":
			// space = select; message is already selected by the cursor
		case "enter", "i", "a":
			m.inserting = true
		case "esc", "q":
			m.levelBack()
		default:
			// Unbound key: start composing and type it (vim-like).
			if msg.Text != "" {
				m.inserting = true
				m.insertComposer(msg.Text)
			}
		}
	}
	return m, nil
}

// level is a small helper keeping the nav logic readable.
func (m Model) level() int {
	if m.openID == 0 {
		return 0
	}
	return 1
}

func (m *Model) moveList(delta int) {
	n := len(m.dialogs)
	if n == 0 {
		return
	}
	m.listIdx += delta
	if m.listIdx < 0 {
		m.listIdx = 0
	}
	if m.listIdx >= n {
		m.listIdx = n - 1
	}
}

func (m Model) openChat() (tea.Model, tea.Cmd) {
	d, ok := m.currentDialog()
	if !ok {
		return m, nil
	}
	m.openID = d.ID
	m.messages = nil
	m.msgCur = 0
	m.inserting = false
	m.composer = nil
	m.composerN = 0
	m.loadingChat = true
	return m, m.loadMessagesCmd(d)
}

func (m *Model) levelBack() {
	m.openID = 0
	m.messages = nil
	m.loadingChat = false
	m.inserting = false
	m.composer = nil
	m.composerN = 0
}

func (m *Model) moveMsg(delta int) {
	n := len(m.messages)
	if n == 0 {
		return
	}
	m.msgCur += delta
	if m.msgCur < 0 {
		m.msgCur = 0
	}
	if m.msgCur >= n {
		m.msgCur = n - 1
	}
}

func (m *Model) insertComposer(s string) {
	runes := []rune(s)
	at := m.composerN
	m.composer = append(m.composer[:at], append(runes, m.composer[at:]...)...)
	m.composerN = at + len(runes)
}

func (m *Model) composerBackspace() {
	if m.composerN <= 0 || len(m.composer) == 0 {
		return
	}
	m.composer = append(m.composer[:m.composerN-1], m.composer[m.composerN:]...)
	m.composerN--
}

func (m *Model) composerMove(delta int) {
	n := m.composerN + delta
	if n < 0 {
		n = 0
	}
	if n > len(m.composer) {
		n = len(m.composer)
	}
	m.composerN = n
}

func (m Model) sendCurrent() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(string(m.composer))
	if text == "" {
		return m, nil
	}
	d, ok := m.currentDialog()
	if !ok {
		return m, nil
	}
	m.composer = nil
	m.composerN = 0
	return m, tea.Batch(m.sendCmd(d, text), m.loadMessagesCmd(d))
}
