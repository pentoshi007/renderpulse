#!/usr/bin/env bash
# Install renderpulse from GitHub releases (private repo: needs gh or GITHUB_TOKEN).
# Usage: bash install.sh [version]   (default: latest)
set -euo pipefail

REPO="pentoshi007/renderpulse"
VERSION="${1:-latest}"

fail() { echo "install.sh: $*" >&2; exit 1; }

OS="$(uname -s)"
ARCH="$(uname -m)"
case "$OS" in
  Linux) os="linux" ;;
  Darwin) os="darwin" ;;
  *) fail "unsupported OS: $OS (download a release asset manually)" ;;
esac
case "$ARCH" in
  x86_64|amd64) arch="amd64" ;;
  aarch64|arm64) arch="arm64" ;;
  *) fail "unsupported architecture: $ARCH" ;;
esac

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

asset() { # version -> asset basename
  echo "renderpulse_${1}_${os}_${arch}"
}

if command -v gh >/dev/null 2>&1; then
  echo ">> downloading renderpulse $VERSION ($os/$arch) via gh"
  if [ "$VERSION" = "latest" ]; then
    gh release download --repo "$REPO" --dir "$TMP" --pattern "$(asset '*')*" --clobber
  else
    gh release download "$VERSION" --repo "$REPO" --dir "$TMP" --pattern "$(asset "$VERSION")*" --clobber
  fi
elif [ -n "${GITHUB_TOKEN:-}" ]; then
  echo ">> downloading renderpulse $VERSION ($os/$arch) via curl + GITHUB_TOKEN"
  if [ "$VERSION" = "latest" ]; then
    VERSION="$(curl -fsSL -H "Authorization: Bearer $GITHUB_TOKEN" \
      "https://api.github.com/repos/$REPO/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p')"
    [ -n "$VERSION" ] || fail "could not resolve latest release"
  fi
  base="$(asset "${VERSION#v}")"
  if [ "$os" = "windows" ]; then ext="zip"; else ext="tar.gz"; fi
  curl -fL -H "Authorization: Bearer $GITHUB_TOKEN" \
    -o "$TMP/$base.$ext" \
    "https://github.com/$REPO/releases/download/$VERSION/$base.$ext"
else
  fail "this repo is private: install gh and run 'gh auth login', or export GITHUB_TOKEN"
fi

cd "$TMP"
if ls ./*.tar.gz >/dev/null 2>&1; then
  tar xzf ./*.tar.gz
elif ls ./*.zip >/dev/null 2>&1; then
  unzip -q ./*.zip
else
  fail "no archive downloaded"
fi

BIN="$(find . -type f -name 'renderpulse' -perm -u+x | head -1)"
[ -n "$BIN" ] || fail "renderpulse binary not found in archive"

# verify checksum when checksums.txt is present
if [ -f checksums.txt ]; then
  (sha256sum -c checksums.txt --ignore-missing) || echo "!! checksum verification failed or incomplete" >&2
fi

DEST="${DESTDIR:-/usr/local/bin}"
if [ -w "$DEST" ] || [ "$(id -u)" = "0" ]; then
  install -m 0755 "$BIN" "$DEST/renderpulse"
else
  sudo install -m 0755 "$BIN" "$DEST/renderpulse"
fi

echo ">> installed: $DEST/renderpulse"
"$DEST/renderpulse" --version
