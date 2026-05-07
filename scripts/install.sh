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

# Souls live in the repo. The installed binary at $INSTALL_PATH is
# launched with -soul pointing directly at $REPO_DIR/souls/*.soul.md
# (kinclaw-mac's Makefile/Supervisor + KINCLAW_SOUL_DIRS env var
# both point there too). No copy step needed — and the old `cp -n`
# no-clobber sync silently caused multi-day stale-soul debugging
# sessions where dev edits never reached the running helper.
#
# Removed 2026-05-06. If a future kinclaw install loses access to
# $REPO_DIR (e.g. user installs the .app on a fresh machine without
# cloning the kinclaw repo), souls should ship inside the .app
# bundle (Bundle.main.url path in KinClawSupervisor.swift) rather
# than as a copied family-dir snapshot that drifts on every edit.
SOULS_DIR="$HOME/.localkin/souls"
if [ -d "$SOULS_DIR" ]; then
    echo "==> Removing legacy $SOULS_DIR (souls now read directly from $REPO_DIR/souls/) ..."
    rm -rf "$SOULS_DIR"
fi

# Register the dev repo's skills/ as a source for runtime discovery.
# kinclaw at boot reads ~/.localkin/skill-sources.txt and scans every
# listed dir for SKILL.md files. So skills stay where they are (no
# copy = no stale duplicates), and edits to the dev repo are
# immediately live without re-running install.sh.
#
# Same skill name in two dirs = the LATER dir wins. Order in
# kinclaw's skillSearchDirs() is: env > skill-sources.txt >
# ~/.localkin/skills > ./skills. So user customizations in
# ~/.localkin/skills/ override the dev repo's, but the dev repo's
# are available as the floor.
SKILL_SOURCES="$HOME/.localkin/skill-sources.txt"
DEV_SKILLS="$REPO_DIR/skills"
if [ -d "$DEV_SKILLS" ]; then
    if ! grep -qxF "$DEV_SKILLS" "$SKILL_SOURCES" 2>/dev/null; then
        echo "==> Registering $DEV_SKILLS as a skill source"
        mkdir -p "$(dirname "$SKILL_SOURCES")"
        printf '%s\n' "$DEV_SKILLS" >> "$SKILL_SOURCES"
    fi
fi

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
