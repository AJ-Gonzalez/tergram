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

// version is the semantic version. It defaults to 0.2.0 and can be overridden
// at build time with: -ldflags "-X main.version=<tag>".
var version = "0.2.0"

// Bundled app credentials, injected at build time (kept out of the repo).
// Default to empty so source builds always fail fast with a clear message
// rather than accidentally using a placeholder. Build with:
//
//	go build -ldflags "-X main.bundleAppID=12345 -X main.bundleAppHash=abc..."
//
// Env vars (APP_ID / APP_HASH) take precedence over these when set.
var (
	bundleAppID   = ""
	bundleAppHash = ""
)

func main() {
	var (
		demo        bool
		showVersion bool
	)
	flag.BoolVar(&demo, "demo", false, "run with synthetic demo data (no network/credentials)")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.Parse()

	if showVersion {
		fmt.Println("tergram", version)
		os.Exit(0)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	var c tgc.Client
	if demo {
		c = tgc.NewDemo(6)
	} else {
		appID := envInt("APP_ID")
		if appID == 0 {
			appID, _ = strconv.Atoi(bundleAppID)
		}
		appHash := os.Getenv("APP_HASH")
		if appHash == "" {
			appHash = bundleAppHash
		}
		gc, err := tgc.Connect(ctx, appID, appHash, store.SessionPath())
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
