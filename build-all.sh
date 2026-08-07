#!/usr/bin/env bash
# Build tergram for every release target locally. Mirrors the GitHub release
# workflow (.github/workflows/release.yml): same target matrix, same naming,
# same flags. CGO_ENABLED=0 keeps the binaries static and cross-compilable
# from any host (pure-Go deps only).
#
# Usage:
#   ./build-all.sh            # local dev version (0.1.0 baked in main.go)
#   ./build-all.sh v0.1.1     # inject a version like the release workflow
#
# Outputs land in dist/: tergram-<goos>-<goarch>[.exe]
set -euo pipefail

outdir="dist"
mkdir -p "$outdir"

version="${1:-}"
targets=(
  linux/amd64
  linux/arm64
  linux/386
  darwin/amd64
  darwin/arm64
  windows/amd64
  windows/arm64
  windows/386
  freebsd/amd64
  freebsd/386
)
for t in "${targets[@]}"; do
  goos="${t%/*}"
  goarch="${t#*/}"
  out="$outdir/tergram-$goos-$goarch"
  [ "$goos" = "windows" ] && out="$out.exe"

  ldflags=""
  if [ -n "$version" ]; then
    ldflags="-X main.version=$version"
  fi

  echo "== $t -> $out"
  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 go build -trimpath -ldflags "$ldflags" -o "$out" ./cmd/tergram
done

echo
echo "done: $(ls -1 "$outdir" | wc -l) binaries in $outdir/"
