# Tergram: Terminal Telegram Client

```
 _____                                  
|_   _|__ _ __ __ _ _ __ __ _ _ __ ___  
  | |/ _ \ '__/ _` | '__/ _` | '_ ` _ \ 
  | |  __/ | | (_| | | | (_| | | | | | |
  |_|\___|_|  \__, |_|  \__,_|_| |_| |_|
              |___/                     
```
What it says on the tin, minimal terminal client for TG. 

## Why? 

I keep hitting my terminal hotkey by force habit because I no a lot of things in the terminal, and telegram desktop is slow to load. 

I wanted a way to quickly send or read messages, without needing to switch context. I could open tmux or a new tab or terminal window. 

[Telegram CLI](https://github.com/vysheng/tg) exists yes, this is much smaller and much leaner.


## Built With

- [Deepseek V4 Flash 0731](https://huggingface.co/deepseek-ai/DeepSeek-V4-Flash-0731)
- [Oh My Pi harness](https://github.com/can1357/oh-my-pi)
- [Go](https://go.dev/)
- [Bubble Tea v2](https://github.com/charmbracelet/bubbletea)
- [Lip Gloss](https://github.com/charmbracelet/lipgloss)
- [gotd/td](https://github.com/gotd/td)
- [qrterminal](https://github.com/mdp/qrterminal)

## Scope

No images, no link preview, no interest in adding them. 

## Building

Requires Go 1.26+. Build locally:

```sh
go build -o tergram ./cmd/tergram
```

The app is identified by a Telegram **api_id / api_hash** pair (the app), while each
user logs in with their **own** account.

By default tergram reads these from the `APP_ID` / `APP_HASH` environment variables. To bake a pair in (e.g. to hand someone a
binary that just works), set them at build time:

```sh
go build -ldflags "-X main.bundleAppID=YOUR_API_ID -X main.bundleAppHash=YOUR_API_HASH" -o tergram ./cmd/tergram
```

Runtime precedence: `APP_ID` / `APP_HASH` env vars win; otherwise the bundled values
are used. Note: your api_id/api_hash identify your app to Telegram and can't be rotated
easily. 

Baking them into broad distribution is NOT recommended.

Notes:
- `tergram -demo` runs offline with synthetic data (no credentials needed).
- `tergram -version` prints the version.

## License

[Apache-2.0](https://www.apache.org/licenses/LICENSE-2.0)

