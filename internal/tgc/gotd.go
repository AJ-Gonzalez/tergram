// gotd.go implements the network-backed Client on top of gotd/td
// (github.com/gotd/td), a pure-Go MTProto user client.
package tgc

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"syscall"

	"go.uber.org/zap"
	"golang.org/x/term"

	"github.com/gotd/log/logzap"
	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/auth/qrlogin"
	"github.com/gotd/td/telegram/message/peer"
	"github.com/gotd/td/telegram/query"
	"github.com/gotd/td/telegram/query/dialogs"
	"github.com/gotd/td/telegram/query/messages"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
	"github.com/mdp/qrterminal/v3"
)

// GotdClient is the network-backed tgc.Client.
type GotdClient struct {
	client  *telegram.Client
	api     *tg.Client
	updates chan Update

	appID   int
	appHash string

	ready chan struct{} // closed once authenticated
	done  chan struct{} // closed once auth attempt finishes (success or failure)
	err   error

	finishOnce sync.Once
	close      sync.Once
}

// Connect creates a gotd client, authenticates if no stored session is present, and
// blocks until authentication completes — so the TUI only starts once we can actually
// talk to Telegram. Login is QR-first: an ASCII QR is printed to the terminal for you to
// scan with the Telegram app (Settings → Devices → Link Desktop Device). If QR fails it
// falls back to the phone/code/2FA prompt.
func Connect(ctx context.Context, appID int, appHash, sessionPath string) (*GotdClient, error) {
	if appID == 0 || appHash == "" {
		return nil, errors.New("set APP_ID and APP_HASH (see core.telegram.org/api/obtaining_api_id)")
	}

	c := &GotdClient{
		updates: make(chan Update, 256),
		appID:   appID,
		appHash: appHash,
		ready:   make(chan struct{}),
		done:    make(chan struct{}),
	}

	dispatcher := tg.NewUpdateDispatcher()
	gc := telegram.NewClient(appID, appHash, telegram.Options{
		Logger:         logzap.New(zap.NewNop()),
		SessionStorage: &session.FileStorage{Path: sessionPath},
		UpdateHandler:  dispatcher,
	})
	c.client = gc
	c.api = gc.API()

	// Live updates → UI channel (best-effort, non-blocking).
	dispatcher.OnNewMessage(func(ctx context.Context, entities tg.Entities, u *tg.UpdateNewMessage) error {
		if m, ok := u.Message.(*tg.Message); ok {
			c.push(m)
		}
		return nil
	})
	dispatcher.OnNewChannelMessage(func(ctx context.Context, entities tg.Entities, u *tg.UpdateNewChannelMessage) error {
		if m, ok := u.Message.(*tg.Message); ok {
			c.push(m)
		}
		return nil
	})

	phoneFlow := auth.NewFlow(termAuth{}, auth.SendCodeOptions{})
	go func() {
		_ = gc.Run(ctx, func(ctx context.Context) error {
			status, err := gc.Auth().Status(ctx)
			if err != nil {
				c.finish(err)
				return err
			}
			if !status.Authorized {
				err := c.loginQR(ctx, dispatcher)
				switch {
				case err == nil:
					// authorized via QR
				case tgerr.Is(err, "SESSION_PASSWORD_NEEDED"):
					// Account has 2FA: the QR scan links the device, but the
					// cloud password is still required to finish authorization.
					if err := c.password2FA(ctx, gc.Auth()); err != nil {
						c.finish(err)
						return err
					}
				default:
					fmt.Fprintln(os.Stderr, "\nQR login failed ("+err.Error()+"); using phone/code instead.")
					if err := gc.Auth().IfNecessary(ctx, phoneFlow); err != nil {
						c.finish(err)
						return err
					}
				}
			}
			c.finish(nil)
			<-ctx.Done()
			return nil
		})
	}()

	select {
	case <-c.done:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if c.err != nil {
		return nil, c.err
	}
	return c, nil
}

// loginQR runs the interactive QR login, waiting until the user scans it.
func (c *GotdClient) loginQR(ctx context.Context, dispatcher tg.UpdateDispatcher) error {
	loggedIn := qrlogin.OnLoginToken(dispatcher)
	// c.client.QR() is pre-wired with gotd's Migrate handler, so if the account
	// lives on a different data center (e.g. DC 1) the QR helper reconnects there
	// and retries automatically instead of failing with "migration to N needed".
	qr := c.client.QR()
	show := func(ctx context.Context, token qrlogin.Token) error {
		fmt.Fprintln(os.Stderr, "\nScan this QR code with Telegram (Settings → Devices → Link Desktop Device):")
		qrterminal.Generate(token.URL(), qrterminal.L, os.Stderr)
		fmt.Fprintln(os.Stderr, "\nOr open: "+token.URL()+"\n")
		return nil
	}
	_, err := qr.Auth(ctx, loggedIn, show)
	return err
}

// password2FA prompts for and submits the account's cloud password (2FA),
// retrying on an invalid password until accepted or input ends.
func (c *GotdClient) password2FA(ctx context.Context, a *auth.Client) error {
	for {
		fmt.Fprint(os.Stderr, "\nEnter 2FA password: ")
		pwd, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return err
		}
		if _, err := a.Password(ctx, strings.TrimSpace(string(pwd))); err != nil {
			if errors.Is(err, auth.ErrPasswordInvalid) {
				fmt.Fprintln(os.Stderr, "Wrong password, try again.")
				continue
			}
			return err
		}
		return nil
	}
}

// finish records the auth outcome and releases Connect (and Dialogs/Messages/Send).
func (c *GotdClient) finish(err error) {
	c.finishOnce.Do(func() {
		c.err = err
		close(c.done)
		if err == nil {
			close(c.ready)
		}
	})
}

func (c *GotdClient) push(m *tg.Message) {
	dialogID := peerClassID(m.PeerID)
	u := Update{
		DialogID: dialogID,
		Message: Message{
			ID:     m.ID,
			PeerID: dialogID,
			Text:   m.Message,
			At:     int64(m.Date),
		},
	}
	select {
	case c.updates <- u:
	default: // drop if the UI is saturated
	}
}

func (c *GotdClient) waitReady(ctx context.Context) error {
	select {
	case <-c.ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *GotdClient) Dialogs(ctx context.Context) ([]Dialog, error) {
	if err := c.waitReady(ctx); err != nil {
		return nil, err
	}
	var out []Dialog
	err := query.GetDialogs(c.api).ForEach(ctx, func(ctx context.Context, elem dialogs.Elem) error {
		if elem.Deleted() {
			return nil
		}
		var (
			title  string
			unread int
		)
		if dlg, ok := elem.Dialog.(*tg.Dialog); ok {
			switch p := dlg.Peer.(type) {
			case *tg.PeerUser:
				if u, ok := elem.Entities.User(p.UserID); ok {
					title = strings.TrimSpace(u.FirstName + " " + u.LastName)
				}
			case *tg.PeerChat:
				if ch, ok := elem.Entities.Chat(p.ChatID); ok {
					title = ch.Title
				}
			case *tg.PeerChannel:
				if ch, ok := elem.Entities.Channel(p.ChannelID); ok {
					title = ch.Title
				}
			}
			unread = dlg.UnreadCount
		}
		last := ""
		if lm, ok := elem.Last.(*tg.Message); ok {
			last = lm.Message
		}
		if title == "" {
			title = "?"
		}
		out = append(out, Dialog{
			ID:       inputPeerID(elem.Peer),
			Title:    title,
			LastText: last,
			Unread:   unread,
			peer:     elem.Peer,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *GotdClient) Messages(ctx context.Context, d Dialog) ([]Message, error) {
	if err := c.waitReady(ctx); err != nil {
		return nil, err
	}
	var out []Message
	err := query.Messages(c.api).GetHistory(d.peer).ForEach(ctx, func(ctx context.Context, elem messages.Elem) error {
		msg, ok := elem.Msg.(*tg.Message)
		if !ok || msg.Message == "" {
			return nil
		}
		out = append(out, Message{
			ID:       msg.ID,
			PeerID:   peerClassID(msg.PeerID),
			Sender:   senderName(elem.Entities, msg.FromID),
			Text:     msg.Message,
			At:       int64(msg.Date),
			Outgoing: msg.Out,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	// The iterator yields newest-first; reverse to chronological.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func (c *GotdClient) Send(ctx context.Context, d Dialog, text string) error {
	if err := c.waitReady(ctx); err != nil {
		return err
	}
	_, err := c.api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
		Peer:    d.peer,
		Message: text,
	})
	return err
}

func (c *GotdClient) Updates() <-chan Update {
	return c.updates
}

func (c *GotdClient) Close() {
	// The underlying gotd connection is torn down by cancelling the context
	// passed to Connect/Run (there is no telegram.Client.Close); this just
	// unblocks any waiter on the update stream.
	c.close.Do(func() { close(c.updates) })
}

// --- helpers -----------------------------------------------------------------

func senderName(entities peer.Entities, from tg.PeerClass) string {
	switch p := from.(type) {
	case *tg.PeerUser:
		if u, ok := entities.User(p.UserID); ok {
			return strings.TrimSpace(u.FirstName + " " + u.LastName)
		}
	}
	return "?"
}

func peerClassID(p tg.PeerClass) int64 {
	switch pp := p.(type) {
	case *tg.PeerUser:
		return pp.UserID
	case *tg.PeerChat:
		return pp.ChatID
	case *tg.PeerChannel:
		return pp.ChannelID
	}
	return 0
}

func inputPeerID(p tg.InputPeerClass) int64 {
	switch pp := p.(type) {
	case *tg.InputPeerUser:
		return pp.UserID
	case *tg.InputPeerChat:
		return pp.ChatID
	case *tg.InputPeerChannel:
		return pp.ChannelID
	}
	return 0
}

// termAuth prompts for phone / code / 2FA on stdin (mirrors gotd's example).
type termAuth struct{}

func (termAuth) SignUp(ctx context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, errors.New("sign-up not implemented in tergram")
}

func (termAuth) AcceptTermsOfService(ctx context.Context, tos tg.HelpTermsOfService) error {
	return &auth.SignUpRequired{TermsOfService: tos}
}

func (termAuth) Code(ctx context.Context, _ *tg.AuthSentCode) (string, error) {
	return promptLine("Enter the code sent to your phone: ")
}

func (termAuth) Phone(ctx context.Context) (string, error) {
	return promptLine("Phone (international, e.g. +1234567890): ")
}

func (termAuth) Password(ctx context.Context) (string, error) {
	fmt.Fprint(os.Stderr, "Enter 2FA password: ")
	pwd, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(pwd)), nil
}

func promptLine(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
