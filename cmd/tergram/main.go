// Command tergram is a cross-platform TUI Telegram client (Go + Bubble Tea).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"

	tea "charm.land/bubbletea/v2"

	"tergram/internal/app"
	"tergram/internal/store"
	"tergram/internal/tgc"
)

func main() {
	var demo bool
	flag.BoolVar(&demo, "demo", false, "run with synthetic demo data (no network/credentials)")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	var c tgc.Client
	if demo {
		c = tgc.NewDemo(6)
	} else {
		gc, err := tgc.Connect(ctx,
			envInt("APP_ID"),
			os.Getenv("APP_HASH"),
			store.SessionPath(),
		)
		if err != nil {
			fmt.Fprintln(os.Stderr, "tergram:", err)
			os.Exit(1)
		}
		c = gc
	}
	defer c.Close()

	p := tea.NewProgram(app.New(c))
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tergram:", err)
		os.Exit(1)
	}
}

func envInt(name string) int {
	v, _ := strconv.Atoi(os.Getenv(name))
	return v
}
