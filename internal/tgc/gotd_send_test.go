package tgc

import (
	"context"
	"testing"

	"github.com/gotd/td/bin"
	"github.com/gotd/td/tg"
)

// captureInvoker records the last encoded request and returns success,
// standing in for the MTProto transport.
type captureInvoker struct {
	last bin.Encoder
}

func (c *captureInvoker) Invoke(ctx context.Context, input bin.Encoder, output bin.Decoder) error {
	c.last = input
	return nil
}

func TestSendSetsRandomID(t *testing.T) {
	inv := &captureInvoker{}
	ready := make(chan struct{})
	close(ready) // waitReady returns immediately
	c := &GotdClient{
		api:   tg.NewClient(inv),
		ready: ready,
	}
	err := c.Send(context.Background(), Dialog{
		peer: &tg.InputPeerUser{UserID: 1, AccessHash: 2},
	}, "hi")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	req, ok := inv.last.(*tg.MessagesSendMessageRequest)
	if !ok {
		t.Fatalf("expected MessagesSendMessageRequest, got %T", inv.last)
	}
	if req.RandomID == 0 {
		t.Fatal("RandomID must be nonzero — the server rejects 0 with RANDOM_ID_EMPTY")
	}
	if req.Message != "hi" {
		t.Fatalf("expected message %q, got %q", "hi", req.Message)
	}
	if req.Peer == nil {
		t.Fatal("expected a peer in the request")
	}
}
