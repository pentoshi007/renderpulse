#!/usr/bin/env bash
# Build static release artifacts for all supported platforms into dist/.
# Usage: scripts/build.sh [version]   (default: 0.1.0)
set -euo pipefail

cd "$(dirname "$0")/.."
VERSION="${1:-0.1.0}"
DIST=dist

rm -rf "$DIST"
mkdir -p "$DIST"

TARGETS=(
  "linux amd64 tar.gz"
  "linux arm64 tar.gz"
  "darwin amd64 tar.gz"
  "darwin arm64 tar.gz"
  "windows amd64 zip"
  "windows arm64 zip"
)

for t in "${TARGETS[@]}"; do
  read -r os arch fmt <<<"$t"
  bin=renderpulse
  ext=""
  if [ "$os" = "windows" ]; then bin=renderpulse; ext=".exe"; fi
  name="renderpulse_${VERSION}_${os}_${arch}"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build \
    -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o "$DIST/${name}/${bin}${ext}" .
  if [ "$fmt" = "zip" ]; then
    (cd "$DIST" && zip -qr "${name}.zip" "${name}")
  else
    (cd "$DIST" && tar -czf "${name}.tar.gz" "${name}")
  fi
  rm -rf "$DIST/${name}"
  echo "built ${name}.${fmt}"
done

(cd "$DIST" && sha256sum ./* > "checksums.txt")

echo "--- artifacts ---"
ls -la "$DIST"
file "$DIST"/renderpulse_${VERSION}_linux_arm64/renderpulse 2>/dev/null || file "$DIST"/* | head -3
