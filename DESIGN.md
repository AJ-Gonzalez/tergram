# Design

Living decision/requirements doc. Amend as decisions are made; never delete history,
append and note the date.

## Status

- **Stack locked: Go + Bubble Tea.** See Key decisions + Architecture below.
- Decisions are recorded here as they're locked in; open items under Open questions.

## Requirements (agreed, fixed)

1. No image/media rendering (no sixel/kitty/block-art images). Media beyond text is
   out of scope.
2. No URL previews.
3. Keyboard navigation — vim-style:
   - Arrow keys **and** `h`/`j`/`k`/`l` move the cursor.
   - `Space` selects/activates the current item.
   - `Esc` moves up one level.
   - `q` quits — but **only at the top level** (i.e. `q` at top level quits; inside a
     view `q` must not quit, it should behave as a back/cancel within the level).

## Key decisions

- **2026-08-03 — Stack locked: Go + Bubble Tea.** Full survey in the section below; the
  Go/Bubble Tea + gotd/td combination won on active framework momentum plus a first-class
  pure-Go MTProto user client in one compiled binary.
- **2026-08-03 — Cross-platform is first-class.** Target Linux/macOS/Windows. All deps stay
  pure-Go (`CGO_ENABLED=0`) so binaries are static and trivially cross-compiled with
  `GOOS`/`GOARCH`. Any future SQLite message store must use a pure-Go driver
  (`modernc.org/sqlite`), never a cgo binding, to preserve this.
- **2026-08-03 — Releases: GitHub Actions (free tier), tag-only builds.** A workflow runs
  ONLY when a git tag is pushed; it builds the cross-compile matrix and attaches native
  binaries to a GitHub Release. No builds on every push/PR (keeps free-tier minutes and
  signals a tagged release = a real artifact).
- **2026-08-03 — Framework/language survey done.** No images and no URL previews fixed out
  of scope; nav input model agreed (see Requirements).
- **2026-08-03 — Confirmed defaults (user):** `Enter` sends in the composer; in-chat
  media (never rendered) shows a one-line placeholder marker (e.g. 📎 Photo); session
  stored at `~/.config/tergram/session.json`.
- **2026-08-03 — Input model refinement (browse/insert mode):** hjkl/arrows navigating
  conflicts with typing those exact letters, so the chat view uses a vim-style two-mode
  split: **browse** (hjkl/arrows scroll messages, `Enter`/`i`/`a` start composing,
  `Esc`/`q` back up) and **insert** (all printable chars incl. space/letters type,
  `Enter` sends, `Esc` exits insert). This keeps hjkl working as motion where it doesn't
  collide with input, and lets the composer receive every letter.
- **2026-08-03 — Key dispatch matches key `Code`, not `String()`:** Bubble Tea's
  `KeyPressMsg.String()` returns surprising values for space/enter (`"space"`, `"\r"`),
  so the app normalizes keys by `Code` (see `keyName` in internal/app/update.go).
- **2026-08-03 — Cross-compile verified:** full matrix (linux/darwin/windows ×
  amd64/arm64) builds with `CGO_ENABLED=0`; smoke-tested the demo TUI end-to-end
  (list → open chat → compose → send → Esc up → q quit, clean exit 0).

## Framework/language survey (2026-08-03)

| Lang | TUI framework | Status | Telegram (MTProto user client) |
|---|---|---|---|
| Python | Textual | active | Telethon |
| TypeScript | Ink (React-style) | active | gramjs |
| Go | Bubble Tea (Elm-style) → tview → gocui | very active; **v2 shipped** | gotd/td (active), gotgproto wrapper |
| Rust | ratatui | very active (22k★; fork of tui-rs) | grammers (active; by Telethon author; now on Codeberg) |
| C#/.NET | Terminal.Gui | active (11k★, v2) | WTelegramClient (maintained) |

Constraints for "reliable": a maintained TUI framework **and** a maintained full MTProto
user-client library. Under that bar, beyond Python/TS/Go, **Rust** and **C#/.NET** are the
viable additions. Everything else (Nim, Zig, Elixir, Lua, C++) fails one or both sides.

- **Rust:** ratatui (fork of tui-rs, actively developed) + grammers (full MTProto user
  client, authored by the Telethon maintainer; repository moved to Codeberg — note the
  migration). Strong, but two new ecosystems (Rust + MTProto) to learn.
- **C#/.NET:** Terminal.Gui (rich widget kit, cross-platform) + WTelegramClient. Solid but
  the TUI ecosystem is less "momentum" than Go's Bubble Tea.

## Architecture & build plan (2026-08-03)

### Directory layout
```
tergram/
  cmd/tergram/main.go          # entry: flags, wiring, start Bubble Tea program
  internal/app/                # Bubble Tea model/update/view, keybindings, nav stack
  internal/ui/                 # lipgloss styles + screens (chat list, message view, composer)
  internal/tgc/                # gotd wrapper (tgc pkg): client lifecycle, auth, update bridge
  internal/store/              # persistence (session path, pure-Go SQLite later)
  .github/workflows/release.yml
```

### Event bridge (gotd ↔ Bubble Tea)
Bubble Tea runs a single-threaded Elm loop; gotd runs its own async loop. Bridge:
- Start the gotd client in a goroutine; its update handler pushes update-derived structs
  into a buffered `chan tea.Msg`.
- A `tea.Cmd` polls that channel (`func() tea.Msg { return <-updates }`) so incoming
  updates reach `Update` as ordinary messages.
- Outgoing work (send text, fetch history, mark read) runs as `tea.Cmd`s that call the
  gotd client and return a result message. No shared mutable state across the loop.

### Navigation model (drives the Requirements)
Navigation is a stack of screens; Esc pops one level (at the bottom it does nothing/back),
`q` quits only at the bottom (top-level) screen.
- **Level 1 — Chat list (top).** Rows = dialogs (name, last message, time). `j`/`k`/arrows
  move, `Space`/`Enter` opens a chat, `q` quits.
- **Level 2 — Chat view.** Message pane (sender, timestamp) + composer, with a vim-style
  **browse / insert** split (see Key decisions):
  - *Browse:* `j`/`k`/arrows scroll messages, `Enter`/`i`/`a` start composing,
    `Esc`/`q` go back one level.
  - *Insert:* all printable chars (incl. space, letters) type, `Enter` sends,
    `Esc` exits insert (a second `Esc` goes back up).
- Future levels: search, reply/selection, settings.

### Auth (QR-first)
- `Connect` blocks until authentication completes; the TUI starts only after we're
  authorized (no login/UI-interleave race).
- **QR login:** an ASCII QR (from gotd `qrlogin` + `qrterminal`) is printed to the
  terminal; scan it in Telegram → Settings → Devices → Link Desktop Device. Text-only,
  satisfies the no-images rule.
- **Fallback:** if QR fails (e.g. 2FA), falls back to phone/code/2FA prompt (`auth.NewFlow`).
- The gotd session persists to `~/.config/tergram/session.json`, so restarts skip login
  (reuses the stored session when `Auth().Status()` reports authorized).

### Messages
- History fetched via tg `MessagesGetHistory`, paginated upward (scroll to load older).
- Render text only; strip/ignore media and URL previews (per Requirements). Media shows a
  one-line placeholder marker (e.g. 📎 Photo), no rendering.
- Bubble/sender/timestamp layout still open (below).

### Release workflow (GitHub Actions, tag-only, free tier)
`.github/workflows/release.yml` triggers `on: push: tags: ['v*']`:
- `actions/checkout` + `actions/setup-go`.
- Build matrix `GOOS` ∈ {linux, darwin, windows} × `GOARCH` ∈ {amd64, arm64},
  `CGO_ENABLED=0`.
- Attach binaries to a GitHub Release (`gh release create` / action).

## Open questions

- Chat rendering: bubbles, timestamps, sender alignment/layout.
- Session/auth UX details (already stored at `~/.config/tergram/session.json`; prompt
  feel and QR-vs-phone flow).
- Message-pane scrolling details (windowed view, "load older" trigger point).

