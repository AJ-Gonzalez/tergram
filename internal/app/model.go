// Package app is the Bubble Tea application shell for tergram.
package app

import (
	"tergram/internal/tgc"
)

// Model holds the full UI state. Level drives navigation:
//
//	level 0 — chat list (top level: q quits)
//	level 1 — open chat (Esc/q go back one level)
//
// This matches the fixed navigation requirements (arrow keys + hjkl, Space to
// select, Esc up a level, q quits only at top level).
type Model struct {
	client tgc.Client

	width, height int

	loadingChat   bool
	err           string
	status        string // transient info (e.g. flood-retry progress)
	dialogsLoaded bool
	// level 0
	dialogs []tgc.Dialog
	listIdx int

	// level 1
	openID    int64 // dialog id currently open
	messages  []tgc.Message
	msgCur    int  // highlighted message index (0 oldest .. n-1 newest)
	inserting bool // composer insert mode (vim-style); false = browse mode
	composer  []rune
	composerN int // caret position within composer
}

// New returns the initial model.
func New(client tgc.Client) Model {
	return Model{client: client}
}

func (m Model) currentDialog() (tgc.Dialog, bool) {
	if m.listIdx < 0 || m.listIdx >= len(m.dialogs) {
		return tgc.Dialog{}, false
	}
	return m.dialogs[m.listIdx], true
}

func (m Model) chatVisible() int {
	if m.height < 6 {
		return 1
	}
	// header (1) + composer block (3) leaves the rest for messages.
	return m.height - 4
}
