package tgc

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Demo is a synthetic Client that produces realistic-looking data with no
// network access. It is used for development and smoke tests (`tergram -demo`).
type Demo struct {
	dialogs  []Dialog
	messages map[int64][]Message

	updates chan Update
	close   sync.Once
}

// NewDemo returns a demo client with n synthetic dialogs.
func NewDemo(n int) *Demo {
	if n < 1 {
		n = 1
	}
	d := &Demo{
		messages: make(map[int64][]Message, n),
		updates:  make(chan Update, 64),
	}
	now := time.Now()
	for i := 0; i < n; i++ {
		id := int64(i + 10001)
		d.dialogs = append(d.dialogs, Dialog{
			ID:       id,
			Title:    fmt.Sprintf("Chat %d — %q", i+1, sampleNames[i%len(sampleNames)]),
			LastText: sampleLines[0],
			Unread:   i * 2,
		})
		msgs := make([]Message, 0, len(sampleLines))
		for j, line := range sampleLines {
			outgoing := j%3 == 0
			sender := "Alicia"
			if outgoing {
				sender = "You"
			}
			msgs = append(msgs, Message{
				ID:       j + 1,
				PeerID:   id,
				Sender:   sender,
				Text:     line,
				At:       now.Add(time.Duration(j) * time.Minute).Unix(),
				Outgoing: outgoing,
			})
		}
		d.messages[id] = msgs
	}
	return d
}

var sampleNames = []string{
	"Design Team", "weekend plans", "Intern Notes", "Groceries", "Reading Club", "Family",
}

var sampleLines = []string{
	"Hey, are we still on for the call?",
	"Just pushed the changes to the release branch.",
	"Can you review the migration before EOD?",
	"Looks good to me, ship it.",
	"Reminder: standup moved to 9:30 tomorrow.",
	"Thanks everyone, great session.",
}

func (d *Demo) Dialogs(ctx context.Context) ([]Dialog, error) {
	out := make([]Dialog, len(d.dialogs))
	copy(out, d.dialogs)
	return out, nil
}

func (d *Demo) Messages(ctx context.Context, dl Dialog) ([]Message, error) {
	msgs := d.messages[dl.ID]
	out := make([]Message, len(msgs))
	copy(out, msgs)
	return out, nil
}

func (d *Demo) Send(ctx context.Context, dl Dialog, text string) error {
	msgs := d.messages[dl.ID]
	msgs = append(msgs, Message{
		ID:       len(msgs) + 1,
		PeerID:   dl.ID,
		Sender:   "You",
		Text:     text,
		At:       time.Now().Unix(),
		Outgoing: true,
	})
	d.messages[dl.ID] = msgs
	return nil
}

func (d *Demo) Updates() <-chan Update {
	return d.updates
}

func (d *Demo) Close() {
	d.close.Do(func() { close(d.updates) })
}
