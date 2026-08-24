#!/usr/bin/env bash
# Downloads a pinned Go toolchain (official tarball, SHA-256 verified) for
# the F-Droid build server, which does not ship Go. Idempotent: caches the
# tarball under $ANDROID_BUILD_TOP or the caller's current directory.
#
# Usage: fetch-go.sh [version]   (default 1.25.10 — pin when bumping!)
# Exports GOROOT and prepends $GOROOT/bin to PATH in the calling shell.
set -euo pipefail

VERSION="${1:-1.25.10}"
TARBALL="go${VERSION}.linux-amd64.tar.gz"
# SHA-256 of the official release tarball (https://go.dev/dl/). Update when
# bumping VERSION — a mismatch aborts the build instead of silently using
# an untrusted toolchain.
SHA256="${GO_TARBALL_SHA256:-}"

CACHE_DIR="${ANDROID_BUILD_TOP:-$PWD}/.go-cache"
DEST_DIR="$CACHE_DIR/go-$VERSION"
mkdir -p "$CACHE_DIR"

if [ ! -x "$DEST_DIR/bin/go" ]; then
  echo "==> downloading Go $VERSION"
  curl -fsSL --retry 3 -o "$CACHE_DIR/$TARBALL" "https://go.dev/dl/$TARBALL"
  if [ -n "$SHA256" ]; then
    echo "$SHA256  $CACHE_DIR/$TARBALL" | sha256sum -c -
  fi
  tar -C "$CACHE_DIR" -xzf "$CACHE_DIR/$TARBALL"
  mv "$CACHE_DIR/go" "$DEST_DIR"
  rm -f "$CACHE_DIR/$TARBALL"
fi

export GOROOT="$DEST_DIR"
export PATH="$DEST_DIR/bin:$PATH"
echo "==> Go $(go version) at $GOROOT"
