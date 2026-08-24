#!/usr/bin/env bash
# Reproducible-build check (F-Droid Verified route): two clean builds from
# the same commit must produce byte-identical APK SHA-256s.
#
# Prereqs: Go in PATH (or run after fetch-go.sh), JDK 17+, network for
# Gradle deps on first run.
#
# Usage: verify-reproducible.sh   (run from android/ subdir)
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

git -C .. diff --quiet --exit-code || { echo "FAIL: working tree is dirty"; exit 1; }

export SOURCE_DATE_EPOCH="$(git -C .. log -1 --format=%ct)"
echo "SOURCE_DATE_EPOCH=$SOURCE_DATE_EPOCH"

build_once() {
  local tag="$1"
  bash scripts/build-core.sh > "/tmp/rb-core-$tag.log" 2>&1 || {
    echo "FAIL: core build $tag failed"; tail -20 "/tmp/rb-core-$tag.log"; exit 1
  }
  ./gradlew clean assembleRelease --no-daemon > "/tmp/rb-build-$tag.log" 2>&1 || {
    echo "FAIL: build $tag failed"; tail -30 "/tmp/rb-build-$tag.log"; exit 1
  }
  # Compare the UNSIGNED APK: signing adds per-build randomness to the
  # signature block, so signed APKs are never byte-identical. F-Droid's
  # reproducible-build check works the same way (signature-stripped
  # comparison via apksigcopier).
  find app/build/outputs/apk -name '*unsigned*.apk' | sort | xargs sha256sum > "/tmp/rb-hash-$tag.txt"
}

build_once one
build_once two

if diff -u "/tmp/rb-hash-one.txt" "/tmp/rb-hash-two.txt"; then
  echo "OK: reproducible — both builds produce identical APKs:"
  cat "/tmp/rb-hash-one.txt"
else
  echo "FAIL: APK hashes differ (non-reproducible build)"
  exit 1
fi
