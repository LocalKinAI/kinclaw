#!/usr/bin/env bash
# install.sh — build kinclaw, ad-hoc codesign with a stable identifier,
# install into ~/.localkin/bin/ (user-writable, no sudo). KinClaw Mac's
# supervisor finds this binary first, ahead of the dev-repo binary.
#
# Why this exists: macOS TCC re-prompts for Accessibility / Screen
# Recording / Apple Events on every cdhash change. Daily `go build`
# cycles change cdhash, so the user has to re-authorize after every
# rebuild — a real productivity drag while building kinclaw.
#
# This script's mitigation:
#   1. Build kinclaw
#   2. Ad-hoc codesign with --identifier dev.localkin.kinclaw
#      → TCC may match by signed identifier in some cases, reducing
#        re-prompt frequency (full fix needs $99 Developer ID, M6).
#   3. Install to ~/.localkin/bin/kinclaw (stable path)
#      → KinClawSupervisor finds it first; user authorizes ONCE for
#        this path, dev rebuilds at the source dir don't affect this
#        installed copy.
#
# Workflow:
#   - Edit kinclaw source freely. `go build` produces dev binary.
#   - When you're happy: `./scripts/install.sh` promotes to installed.
#   - KinClaw Mac uses installed binary on next launch.
#   - Re-authorize ONCE, then re-running install.sh rarely re-prompts.
#
# Run with no args; safe to re-run.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(dirname "$SCRIPT_DIR")"
INSTALL_DIR="$HOME/.localkin/bin"
INSTALL_PATH="$INSTALL_DIR/kinclaw"
IDENTIFIER="dev.localkin.kinclaw"

echo "==> Building kinclaw..."
cd "$REPO_DIR"
go build -o kinclaw ./cmd/kinclaw/

echo "==> Ad-hoc codesigning (identifier: $IDENTIFIER)..."
# --force: replace existing signature
# --sign -: ad-hoc (no Apple cert needed)
# --identifier: stable signed identifier — TCC uses this in some matching paths
#
# NO --options=runtime here. Hardened runtime enables library
# validation, which requires every dlopen'd dylib to share the host's
# Team ID. Ad-hoc signatures don't have a real Team ID — macOS
# synthesizes one per file from the cdhash, so two separately
# ad-hoc-signed files look like "different teams" and the loader
# rejects with:
#
#   "mapping process and mapped file have different Team IDs"
#
# This bit us specifically with libkinrec_writer.dylib (the screen
# recording library kinclaw extracts to ~/Library/Caches/kinrec/
# at first use). With runtime hardening on kinclaw, the linker-signed
# kinrec dylib failed validation; without it, dlopen succeeds.
#
# We can't notarize ad-hoc anyway (need $99 Apple Developer cert,
# deferred to M6), so hardened runtime gains nothing here and only
# breaks dynamic loading.
codesign --force --sign - --identifier "$IDENTIFIER" ./kinclaw

echo "==> Installing to $INSTALL_PATH..."
mkdir -p "$INSTALL_DIR"
cp ./kinclaw "$INSTALL_PATH"

# Re-sign at install path so the signed binary's stored path attribute
# matches its actual location (some TCC verification paths check this).
codesign --force --sign - --identifier "$IDENTIFIER" "$INSTALL_PATH"

# Sync kinclaw-branded souls into the LocalKin family location
# (~/.localkin/souls/) so the installed binary can find pilot.soul.md
# / coder.soul.md / etc. without depending on the dev repo path.
# `cp -n` = no-clobber: user customizations to ~/.localkin/souls/<name>
# survive re-install.
SOULS_DIR="$HOME/.localkin/souls"
echo "==> Syncing kinclaw souls to $SOULS_DIR (no-clobber)..."
mkdir -p "$SOULS_DIR"
for soul in "$REPO_DIR"/souls/*.soul.md; do
    if [ -f "$soul" ]; then
        cp -n "$soul" "$SOULS_DIR/"
    fi
done

echo
echo "✓ Installed: $INSTALL_PATH"
echo "  $(file "$INSTALL_PATH" | sed 's|.*: ||')"
echo "  Signed: $(codesign -dv "$INSTALL_PATH" 2>&1 | grep '^Identifier=' | head -1)"
echo
echo "Next:"
echo "  1. Quit KinClaw Mac if running (⌘Q from menubar)."
echo "  2. Relaunch — supervisor will use $INSTALL_PATH instead of"
echo "     the dev binary at $REPO_DIR/kinclaw."
echo "  3. Authorize ONCE at System Settings → Privacy & Security →"
echo "     Accessibility (and Screen Recording when prompted)."
echo "  4. Future install.sh runs rarely re-prompt (signed identifier"
echo "     stays stable across rebuilds)."
echo
echo "Full fix (no re-prompts ever, signed for distribution): M6 with"
echo "Apple Developer cert (\$99/year)."
