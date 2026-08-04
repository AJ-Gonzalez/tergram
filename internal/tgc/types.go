// Package tgc is the Telegram-layer abstraction used by the UI.
//
// It defines a small Client interface (typed shared with the UI: Dialog,
// Message, Update) with two implementations: a network-backed gotd client
// (gotd.go) and a synthetic demo client used for development and smoke tests
// without credentials (demo.go).
package tgc

import (
	"context"

	"github.com/gotd/td/tg"
)

// Dialog is a single conversation shown in the chat list.
type Dialog struct {
	ID       int64
	Title    string
	LastText string
	Unread   int

	// peer is the MTProto input peer used to fetch messages / send. Not copied
	// out of the package (UI only reads the exported fields).
	peer tg.InputPeerClass
}

// Message is a single text message in a conversation.
type Message struct {
	ID       int
	PeerID   int64
	Sender   string
	Text     string
	At       int64 // unix seconds
	Outgoing bool
}

// Update is a live event pushed from the Telegram client to the UI.
type Update struct {
	DialogID int64
	Message  Message
}

// Client is the interface the UI uses to talk to Telegram (or a demo).
type Client interface {
	// Dialogs returns the list of conversations (chat list).
	Dialogs(ctx context.Context) ([]Dialog, error)
	// Messages returns the message history for a dialog (oldest first).
	Messages(ctx context.Context, d Dialog) ([]Message, error)
	// Send sends a text message to a dialog.
	Send(ctx context.Context, d Dialog, text string) error
	// Updates returns the live update stream. It is closed on Close.
	Updates() <-chan Update
	// Close tears down the client and closes the Updates stream.
	Close()
}
