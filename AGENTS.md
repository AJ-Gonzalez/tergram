# AGENTS.md

Guidance for AI agents and contributors working in this repository. This file is
authoritative; when in doubt, follow it over inline convenience.

## HARD RULE — no forced-choice questions

DO NOT use the multiple-choice / question-widget tool (`ask`, the question widget, or
any equivalent that forces the user to pick from preset options).

- Ask questions in an open, free-text format instead — present the options or context in
  prose and let the user answer naturally.
- Never force a selection. The user must always be free to answer off-list or with their
  own phrasing.
- This applies to clarifying questions during implementation, design decisions, and any
  interactive prompt.

Rationale: forced multiple-choice hides viable off-list answers and biases the user
toward preset options. Open questions preserve the full decision space.

## Git Rules

Use the local `dumbpush.sh` script, give the commit message in quotes as an argument. 

Example

`$ ./dumbpush.sh "Commit message here"`

Only trigger the github actions workflow when there is a tag. 
Otherwise make a local build for the Linux target. 

## Version

- The current version is `0.1.0` (see `var version` in `cmd/tergram/main.go`).
- The user will explicitly indicate when the version number should increase. Do NOT
  bump the version on your own initiative.
- When the user says a new version, update both the `version` var in
  `cmd/tergram/main.go` and the matching release tag (git tags drive the release
  workflow).
- Release binaries get their version injected from the git tag via
  `-X main.version=...` (see `.github/workflows/release.yml`), so a tag like `v0.1.1`
  reports `tergram v0.1.1` from `tergram -version`.

## Build

Requires Go 1.26+. The module root is `tergram`.

Local Linux build:

```sh
go build -o tergram ./cmd/tergram
```

Telegram `api_id`/`api_hash` identify the app; each user logs in with their own account
(QR). They come from `APP_ID`/`APP_HASH` env vars by default, with a build-time fallback
that can be baked in so a binary works without env vars:

```sh
go build -ldflags "-X main.bundleAppID=API_ID -X main.bundleAppHash=API_HASH" -o tergram ./cmd/tergram
```

Runtime precedence: `APP_ID`/`APP_HASH` env → bundled values (`bundleAppID`/
`bundleAppHash` in `cmd/tergram/main.go`) → error prompting to set them.

Do NOT commit real api_id/api_hash to the repo. Prefer env vars or ldflags injection.

Cross-compile matrix (releases build these; local smoke test only needs linux amd64):
`GOOS` ∈ {linux, darwin, windows} × `GOARCH` ∈ {amd64, arm64}, `CGO_ENABLED=0`.
