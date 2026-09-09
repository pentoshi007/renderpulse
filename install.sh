#!/usr/bin/env bash
# Install renderpulse from GitHub releases (public repo, plain HTTPS, no auth, no gh).
#
# One command:
#   curl -fsSL https://raw.githubusercontent.com/pentoshi007/renderpulse/main/install.sh | bash
#
# Or with an explicit version:
#   bash install.sh v0.2.0
set -euo pipefail

REPO="pentoshi007/renderpulse"
VERSION="${1:-latest}"

fail() { echo "install.sh: $*" >&2; exit 1; }

OS="$(uname -s)"
ARCH="$(uname -m)"
case "$OS" in
  Linux) os="linux" ;;
  Darwin) os="darwin" ;;
  *) fail "unsupported OS '$OS' — download a binary from https://github.com/$REPO/releases" ;;
esac
case "$ARCH" in
  x86_64 | amd64) arch="amd64" ;;
  aarch64 | arm64) arch="arm64" ;;
  *) fail "unsupported architecture '$ARCH' — download a binary from https://github.com/$REPO/releases" ;;
esac

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# optional: raises API rate limits if set; never required for this public repo
AUTH=()
if [ -n "${GITHUB_TOKEN:-}" ]; then AUTH=(-H "Authorization: Bearer $GITHUB_TOKEN"); fi

if [ "$VERSION" = "latest" ]; then
  VERSION="$(curl -fsSL ${AUTH[@]+"${AUTH[@]}"} "https://api.github.com/repos/$REPO/releases/latest" |
    sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' || true)"
  [ -n "$VERSION" ] || fail "could not resolve the latest release — retry, or pass a version: bash install.sh v0.2.0"
fi

BASE="renderpulse_${VERSION#v}_${os}_${arch}"
if [ "$os" = "windows" ]; then EXT="zip"; else EXT="tar.gz"; fi

echo ">> downloading $BASE.$EXT from $REPO release $VERSION"
curl -fL --retry 3 ${AUTH[@]+"${AUTH[@]}"} -o "$TMP/$BASE.$EXT" \
  "https://github.com/$REPO/releases/download/$VERSION/$BASE.$EXT" ||
  fail "download failed — check available assets at https://github.com/$REPO/releases"

if curl -fsSL ${AUTH[@]+"${AUTH[@]}"} -o "$TMP/checksums.txt" \
  "https://github.com/$REPO/releases/download/$VERSION/checksums.txt"; then
  (cd "$TMP" && sha256sum -c checksums.txt --ignore-missing >/dev/null) ||
    fail "SHA-256 verification failed — the download may be corrupted"
  echo ">> checksum OK"
else
  echo "!! checksums.txt unavailable, skipping verification" >&2
fi

if [ "$EXT" = "zip" ]; then
  (cd "$TMP" && unzip -q "$BASE.zip")
else
  (cd "$TMP" && tar xzf "$BASE.tar.gz")
fi
BIN="$(find "$TMP" -type f -name 'renderpulse' -perm -u+x | head -1)"
[ -n "$BIN" ] || fail "renderpulse binary not found in the archive"

# Destination: DESTDIR if given; else /usr/local/bin when writable or via
# passwordless sudo; else ~/.local/bin so a plain `curl | bash` never gets
# stuck on a sudo password prompt.
if [ -n "${DESTDIR:-}" ]; then
  DEST="$DESTDIR"
elif [ -w /usr/local/bin ] || sudo -n true 2>/dev/null; then
  DEST=/usr/local/bin
else
  DEST="$HOME/.local/bin"
fi
mkdir -p "$DEST"
if [ -w "$DEST" ]; then
  install -m 0755 "$BIN" "$DEST/renderpulse"
else
  sudo install -m 0755 "$BIN" "$DEST/renderpulse"
fi

case ":$PATH:" in
  *":$DEST:"*) ;;
  *) echo ">> note: $DEST is not in your PATH yet — run: export PATH=\"$DEST:\$PATH\" (or log out/in)" ;;
esac

echo ">> installed: $DEST/renderpulse"
"$DEST/renderpulse" --version
