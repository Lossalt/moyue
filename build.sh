#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

GO="${GO:-go}"
WINRES="${WINRES:-go-winres}"

if ! command -v "$GO" >/dev/null 2>&1; then
  echo "go not found; set GO=/path/to/go" >&2
  exit 1
fi
if ! command -v "$WINRES" >/dev/null 2>&1; then
  echo "go-winres not found; run: go install github.com/tc-hib/go-winres@latest" >&2
  exit 1
fi

echo "[1/4] tests"
GOOS= GOARCH= CGO_ENABLED=0 "$GO" test ./...

echo "[2/4] resources"
"$WINRES" make --arch amd64

echo "[3/4] build"
mkdir -p dist
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 "$GO" build -trimpath \
  -ldflags "-H windowsgui -s -w" -o "dist/墨阅.exe" .

echo "[4/4] checksum"
if command -v sha256sum >/dev/null 2>&1; then
  (cd dist && sha256sum "墨阅.exe" > SHA256SUMS.txt)
elif command -v shasum >/dev/null 2>&1; then
  (cd dist && shasum -a 256 "墨阅.exe" > SHA256SUMS.txt)
fi
ls -lh "dist/墨阅.exe"
